/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package crdmigrator

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/cache"
)

// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;update;patch

// CRDMigrator migrates CRDs.
type CRDMigrator struct {
	Client    client.Client
	APIReader client.Reader

	Migrate map[client.Object]MigrationConfig

	migrateByGK              map[schema.GroupKind]MigrationConfig
	scheme                   *runtime.Scheme
	storageMigrationCache    cache.Cache[objectEntry]
	managedFieldCleanupCache cache.Cache[objectEntry]
}

// MigrationConfig configures how an object should be migrated
type MigrationConfig struct {
	UseAPIReader bool
}

func (r *CRDMigrator) SetupWithManager(_ context.Context, mgr ctrl.Manager, options controller.Options) error {
	if r.Client == nil || r.APIReader == nil {
		return errors.New("Client and APIReader must not be nil")
	}

	err := ctrl.NewControllerManagedBy(mgr).
		For(&apiextensionsv1.CustomResourceDefinition{}).
		Named("crd_migrator").
		WithOptions(options).
		Complete(r)
	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}

	for obj, cfg := range r.Migrate {
		gvk, err := apiutil.GVKForObject(obj, mgr.GetScheme())
		if err != nil {
			return errors.Wrap(err, "failed to get GVK for object")
		}
		r.migrateByGK[schema.GroupKind{
			Group: gvk.Group,
			Kind:  gvk.Kind,
		}] = cfg
	}

	r.scheme = mgr.GetScheme()
	r.storageMigrationCache = cache.New[objectEntry]()
	r.managedFieldCleanupCache = cache.New[objectEntry]()
	return nil
}

func (r *CRDMigrator) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := r.Client.Get(ctx, req.NamespacedName, crd); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	migrationConfig, ok := r.migrateByGK[schema.GroupKind{
		Group: crd.Spec.Group,
		Kind:  crd.Spec.Names.Kind,
	}]
	if !ok {
		// If there is no migrationConfig for this CRD, do nothing.
		return ctrl.Result{}, nil
	}

	if err := r.reconcileStorageVersionMigration(ctx, crd, migrationConfig); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileCleanupManagedFields(ctx, crd, migrationConfig); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *CRDMigrator) reconcileStorageVersionMigration(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, migrationConfig MigrationConfig) error {
	log := ctrl.LoggerFrom(ctx)

	storageVersion, err := storageVersionForCRD(crd)
	if err != nil {
		return err
	}

	// If .status.storedVersions is equal to [storageVersion], there is nothing to do
	// as all objects are already stored in the storageVersion.
	if len(crd.Status.StoredVersions) == 1 && crd.Status.StoredVersions[0] == storageVersion {
		log.V(6).Info(".status.storedVersions only contains current storage version, nothing to do")
		return nil
	}

	if err := r.migrateStorageVersion(ctx, crd, storageVersion, migrationConfig); err != nil {
		return err
	}

	originalCRD := crd.DeepCopy()
	crd.Status.StoredVersions = []string{storageVersion}
	if err := r.Client.Status().Patch(ctx, crd, client.MergeFrom(originalCRD)); err != nil { // FIXME verify the changes propagates into the managed field func
		return errors.Wrap(err, "failed to update status.storedVersions for CRD")
	}

	return nil
}

// storageVersionForCRD discovers the storage version for a given CRD.
func storageVersionForCRD(crd *apiextensionsv1.CustomResourceDefinition) (string, error) {
	for _, v := range crd.Spec.Versions {
		if v.Storage {
			return v.Name, nil
		}
	}
	return "", errors.Errorf("could not find storage version for CRD %q", crd.Name)
}

func (r *CRDMigrator) migrateStorageVersion(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, storageVersion string, migrationConfig MigrationConfig) error {
	log := ctrl.LoggerFrom(ctx)

	var objs []client.Object
	if migrationConfig.UseAPIReader {
		// Note: As the APIReader won't create an informer we can use UnstructuredList
		// and thus it's not required that the type has been registered with our scheme.
		objectList := &unstructured.UnstructuredList{}
		objectList.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: storageVersion,
			Kind:    crd.Spec.Names.ListKind,
		})
		objects, err := listObjectsWithPagination(ctx, r.APIReader, objectList)
		if err != nil {
			return err
		}
		objs = append(objs, objects...)
	} else {
		objects, err := listObjectsFromCache(ctx, r.Client, crd, storageVersion)
		if err != nil {
			return err
		}
		objs = append(objs, objects...)
	}

	for i, obj := range objs {
		e := objectEntry(client.ObjectKeyFromObject(obj))

		if _, alreadyDone := r.storageMigrationCache.Has(e.Key()); alreadyDone {
			continue
		}

		if err := retryWithExponentialBackoff(ctx, newCRDMigrationBackoff(), func(ctx context.Context) error {
			err := r.Client.Update(ctx, obj)
			// If we got a NotFound error, the resource no longer exists so no need to update it.
			// If we got a Conflict error, another client wrote the object already so no need to update it.
			if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
				return err
			}

			return nil
		}); err != nil {
			return errors.Wrapf(err, "failed to migrate %s/%s", obj.GetNamespace(), obj.GetName())
		}

		r.storageMigrationCache.Add(e)

		// Add some random delays to avoid pressure on the API server.
		i++
		if i%10 == 0 {
			log.V(2).Info(fmt.Sprintf("%d objects migrated", i))
			time.Sleep(time.Duration(rand.IntnRange(50*int(time.Millisecond), 250*int(time.Millisecond))))
		}
	}

	return nil
}

func listObjectsWithPagination(ctx context.Context, c client.Reader, objectList client.ObjectList) ([]client.Object, error) {
	var objs []client.Object
	for {
		listOpts := []client.ListOption{
			client.Continue(objectList.GetContinue()),
			client.Limit(100),
		}
		if err := c.List(ctx, objectList, listOpts...); err != nil {
			return nil, err
		}

		objectListItems, err := meta.ExtractList(objectList)
		if err != nil {
			return nil, errors.Errorf("TODO")
		}
		for _, obj := range objectListItems {
			objs = append(objs, obj.(client.Object))
		}

		if objectList.GetContinue() == "" {
			break
		}
	}
	return objs, nil
}

func listObjectsFromCache(ctx context.Context, c client.Client, crd *apiextensionsv1.CustomResourceDefinition, storageVersion string) ([]client.Object, error) {
	var objs []client.Object
	// Note: We should only use the cached client with a typed object list.
	// Otherwise we would create an additional informer for a UnstructuredList/PartialObjectMetadataList.
	objList, err := c.Scheme().New(schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: storageVersion,
		Kind:    crd.Spec.Names.ListKind,
	})
	if err != nil {
		return nil, err
	}
	objectList, ok := objList.(client.ObjectList)
	if !ok {
		return nil, errors.Errorf("TODO")
	}

	if err := c.List(ctx, objectList); err != nil {
		return nil, err
	}
	objectListItems, err := meta.ExtractList(objectList)
	if err != nil {
		return nil, err
	}
	for _, obj := range objectListItems {
		objs = append(objs, obj.(client.Object))
	}
	return objs, nil
}

func (r *CRDMigrator) reconcileCleanupManagedFields(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, migrationConfig MigrationConfig) error {
	log := ctrl.LoggerFrom(ctx)

	storageVersion, err := storageVersionForCRD(crd)
	if err != nil {
		return err
	}

	servedGroupVersions := sets.Set[string]{}
	servedVersions := sets.Set[string]{}
	unservedVersions := sets.Set[string]{}
	for _, v := range crd.Spec.Versions {
		if v.Served {
			servedVersions.Insert(v.Name)
			servedGroupVersions.Insert(fmt.Sprintf("%s/%s", crd.Spec.Group, v.Name))
		} else {
			unservedVersions.Insert(v.Name)
		}
	}

	if v, ok := crd.Annotations[clusterv1.CRDMigrationManagedFieldsCleanupCompletedAnnotation]; ok && v != "" {
		cleanedUpVersions := strings.Split(v, ",")
		if unservedVersions.Difference(sets.New[string](cleanedUpVersions...)).Len() == 0 {
			// All unserved versions have been cleaned up, return.

			if unservedVersions.Len() != len(cleanedUpVersions) {
				originalCRD := crd.DeepCopy()
				sortedUnservedVersions := unservedVersions.UnsortedList()
				sort.Strings(sortedUnservedVersions)
				crd.Annotations[clusterv1.CRDMigrationManagedFieldsCleanupCompletedAnnotation] = strings.Join(sortedUnservedVersions, ",")
				if err := r.Client.Status().Patch(ctx, crd, client.MergeFrom(originalCRD)); err != nil {
					return errors.Wrap(err, "failed to update annotation TODO")
				}
			}
			return nil
		}
	}

	var objs []client.Object
	if migrationConfig.UseAPIReader {
		// Note: As the APIReader won't create an informer we can use PartialObjectMetadataList
		// and thus it's not required that the type has been registered with our scheme.
		// Note: PartialObjectMetadataList is used instead of UnstructuredList to avoid retrieving
		// data that we don't need.
		objectList := &metav1.PartialObjectMetadataList{}
		objectList.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: storageVersion,
			Kind:    crd.Spec.Names.ListKind,
		})
		objects, err := listObjectsWithPagination(ctx, r.APIReader, objectList)
		if err != nil {
			return err
		}
		objs = append(objs, objects...)
	} else {
		objects, err := listObjectsFromCache(ctx, r.Client, crd, storageVersion)
		if err != nil {
			return err
		}
		objs = append(objs, objects...)
	}

	for i, obj := range objs {
		e := objectEntry(client.ObjectKeyFromObject(obj))

		if _, alreadyDone := r.managedFieldCleanupCache.Has(e.Key()); alreadyDone {
			continue
		}

		if err := retryWithExponentialBackoff(ctx, newCRDMigrationBackoff(), func(ctx context.Context) error {
			managedFields, removed := removeManagedFieldsWithUnservedGroupVersion(obj, servedGroupVersions)
			if !removed {
				return nil
			}

			// Create a patch to update only managedFields.
			// Include resourceVersion to avoid race conditions.
			jsonPatch := []map[string]interface{}{
				{
					"op":    "replace",
					"path":  "/metadata/managedFields",
					"value": managedFields,
				},
				{
					// Use "replace" instead of "test" operation so that etcd rejects with
					// 409 conflict instead of apiserver with an invalid request
					"op":    "replace",
					"path":  "/metadata/resourceVersion",
					"value": obj.GetResourceVersion(),
				},
			}
			patch, err := json.Marshal(jsonPatch)
			if err != nil {
				return err
			}

			err = r.Client.Patch(ctx, obj, client.RawPatch(types.JSONPatchType, patch))
			// If the resource no longer exists, the managedFields don't have to be cleaned up anymore.
			if err == nil || apierrors.IsNotFound(err) {
				return nil
			}

			if apierrors.IsConflict(err) {
				var getErr error
				if migrationConfig.UseAPIReader {
					getErr = r.APIReader.Get(ctx, client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()}, obj)
				} else {
					getErr = r.Client.Get(ctx, client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()}, obj)
				}
				return kerrors.NewAggregate([]error{err, getErr})
			}
			return err
		}); err != nil {
			return errors.Wrapf(err, "failed to migrate managedFields of %s/%s", obj.GetNamespace(), obj.GetName())
		}

		r.managedFieldCleanupCache.Add(e)

		// Add some random delays to avoid pressure on the API server.
		i++
		if i%10 == 0 {
			log.V(2).Info(fmt.Sprintf("%d objects migrated", i))
			time.Sleep(time.Duration(rand.IntnRange(50*int(time.Millisecond), 250*int(time.Millisecond))))
		}
	}

	originalCRD := crd.DeepCopy()
	sortedUnservedVersions := unservedVersions.UnsortedList()
	sort.Strings(sortedUnservedVersions)
	crd.Annotations[clusterv1.CRDMigrationManagedFieldsCleanupCompletedAnnotation] = strings.Join(sortedUnservedVersions, ",")
	if err := r.Client.Status().Patch(ctx, crd, client.MergeFrom(originalCRD)); err != nil {
		return errors.Wrap(err, "failed to update annotation TODO")
	}

	return nil
}

func removeManagedFieldsWithUnservedGroupVersion(obj client.Object, servedGroupVersions sets.Set[string]) ([]metav1.ManagedFieldsEntry, bool) {
	usesUnservedGroupVersion := func(entry metav1.ManagedFieldsEntry) bool {
		return !servedGroupVersions.Has(entry.APIVersion)
	}

	if !slices.ContainsFunc(obj.GetManagedFields(), usesUnservedGroupVersion) {
		return nil, false
	}

	return slices.DeleteFunc(obj.GetManagedFields(), usesUnservedGroupVersion), true
}

type objectEntry client.ObjectKey

func (r objectEntry) Key() string {
	return r.Key()
}

// FIXME: maybe get rid of funcs below

// newCRDMigrationBackoff creates a new API Machinery backoff parameter set suitable for use with crd migration operations.
// Clusterctl upgrades cert-manager right before doing CRD migration. This may lead to rollout of new certificates.
// The time between new certificate creation + injection into objects (CRD, Webhooks) and the new secrets getting propagated
// to the controller can be 60-90s, because the kubelet only periodically syncs secret contents to pods.
// During this timespan conversion, validating- or mutating-webhooks may be unavailable and cause a failure.
func newCRDMigrationBackoff() wait.Backoff {
	// Return a exponential backoff configuration which returns durations for a total time of ~1m30s + some buffer.
	// Example: 0, .25s, .6s, 1.1s, 1.8s, 2.7s, 4s, 6s, 9s, 12s, 17s, 25s, 35s, 49s, 69s, 97s, 135s
	// Jitter is added as a random fraction of the duration multiplied by the jitter factor.
	return wait.Backoff{
		Duration: 250 * time.Millisecond,
		Factor:   1.4,
		Steps:    17,
		Jitter:   0.1,
	}
}

// retryWithExponentialBackoff repeats an operation until it passes or the exponential backoff times out.
func retryWithExponentialBackoff(ctx context.Context, opts wait.Backoff, operation func(ctx context.Context) error) error {
	log := ctrl.LoggerFrom(ctx)

	i := 0
	err := wait.ExponentialBackoffWithContext(ctx, opts, func(ctx context.Context) (bool, error) {
		i++
		if err := operation(ctx); err != nil {
			if i < opts.Steps {
				log.V(5).Info("Retrying with backoff", "cause", err.Error())
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return errors.Wrapf(err, "action failed after %d attempts", i)
	}
	return nil
}
