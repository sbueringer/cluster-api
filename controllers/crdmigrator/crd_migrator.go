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

// Package crdmigrator contains the CRD migrator implementation.
package crdmigrator

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/cache"
	"sigs.k8s.io/cluster-api/util/contract"
	"sigs.k8s.io/cluster-api/util/predicates"
)

type Phase string

var (
	// AllPhase includes all phases.
	AllPhase Phase = "All"

	// StorageVersionMigrationPhase is the phase in which the storage version is migrated.
	StorageVersionMigrationPhase Phase = "StorageVersionMigration"

	// CleanupManagedFieldsPhase is the phase in which managedFields are cleaned up.
	CleanupManagedFieldsPhase Phase = "CleanupManagedFields"
)

// CRDMigrator migrates CRDs.
type CRDMigrator struct {
	Client    client.Client
	APIReader client.Reader

	// Comma-separated list of CRD migration phases to skip.
	// Valid values are: All, StorageVersionMigration, CleanupManagedFields.
	SkipCRDMigrationPhases  string
	crdMigrationPhasesToRun sets.Set[Phase]

	Config          map[client.Object]ByObjectConfig
	configByCRDName map[string]ByObjectConfig

	storageVersionMigrationCache cache.Cache[objectEntry]
}

// ByObjectConfig configures how an object should be migrated.
type ByObjectConfig struct {
	UseCache bool
}

func (r *CRDMigrator) SetupWithManager(ctx context.Context, mgr ctrl.Manager, controllerOptions controller.Options) error {
	if err := r.setup(mgr.GetScheme()); err != nil {
		return err
	}

	if len(r.crdMigrationPhasesToRun) == 0 {
		// Nothing to do
		return nil
	}

	predicateLog := ctrl.LoggerFrom(ctx).WithValues("controller", "crdmigrator")
	err := ctrl.NewControllerManagedBy(mgr).
		For(&apiextensionsv1.CustomResourceDefinition{},
			// This controller uses a PartialObjectMetadata watch/informer to avoid an informer for CRDs.
			// Core CAPI also already has an informer on PartialObjectMetadata for CRDs because it uses
			// conversion.UpdateReferenceAPIContract.
			builder.OnlyMetadata,
			builder.WithPredicates(
				predicates.ResourceIsChanged(mgr.GetScheme(), predicateLog),
			),
		).
		Named("crdmigrator").
		WithOptions(controllerOptions).
		Complete(r)
	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}

	return nil
}

func (r *CRDMigrator) setup(scheme *runtime.Scheme) error {
	if r.Client == nil || r.APIReader == nil || len(r.Config) == 0 {
		return errors.New("Client and APIReader must not be nil and Config must not be empty")
	}

	r.crdMigrationPhasesToRun = sets.Set[Phase]{}.Insert(StorageVersionMigrationPhase, CleanupManagedFieldsPhase)
	if r.SkipCRDMigrationPhases != "" {
		for _, skipPhase := range strings.Split(r.SkipCRDMigrationPhases, ",") {
			switch skipPhase {
			case string(AllPhase):
				r.crdMigrationPhasesToRun.Delete(StorageVersionMigrationPhase, CleanupManagedFieldsPhase)
			case string(StorageVersionMigrationPhase):
				r.crdMigrationPhasesToRun.Delete(StorageVersionMigrationPhase)
			case string(CleanupManagedFieldsPhase):
				r.crdMigrationPhasesToRun.Delete(CleanupManagedFieldsPhase)
			default:
				return errors.Errorf("Invalid phase %s specified in SkipCRDMigrationPhases", skipPhase)
			}
		}
	}

	r.configByCRDName = map[string]ByObjectConfig{}
	for obj, cfg := range r.Config {
		gvk, err := apiutil.GVKForObject(obj, scheme)
		if err != nil {
			return errors.Wrap(err, "failed to get GVK for object")
		}

		r.configByCRDName[contract.CalculateCRDName(gvk.Group, gvk.Kind)] = cfg
	}

	r.storageVersionMigrationCache = cache.New[objectEntry]()
	return nil
}

func (r *CRDMigrator) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	migrationConfig, ok := r.configByCRDName[req.Name]
	if !ok {
		// If there is no migrationConfig for this CRD, do nothing.
		return ctrl.Result{}, nil
	}

	crdPartial := &metav1.PartialObjectMetadata{}
	crdPartial.SetGroupVersionKind(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
	if err := r.Client.Get(ctx, req.NamespacedName, crdPartial); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	currentGeneration := strconv.FormatInt(crdPartial.GetGeneration(), 10)
	if observedGeneration, ok := crdPartial.Annotations[clusterv1.CRDMigrationObservedGenerationAnnotation]; ok &&
		currentGeneration == observedGeneration {
		// If the current generation was already observed, do nothing.
		return ctrl.Result{}, nil
	}

	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := r.APIReader.Get(ctx, req.NamespacedName, crd); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Note: Somehow it's possible that PartialObjectMeta above got a newer generation
	// than then CRD after the live call here. Requeuing here until we get the same generation.
	if crdPartial.Generation != crd.Generation {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	var storageVersion string
	for _, v := range crd.Spec.Versions {
		if v.Storage {
			storageVersion = v.Name
			break
		}
	}
	if storageVersion == "" {
		return ctrl.Result{}, errors.New("could not find storage version for CRD")
	}

	originalCRD := crd.DeepCopy()
	defer func() {
		if reterr == nil {
			if crd.Annotations == nil {
				crd.Annotations = map[string]string{}
			}
			crd.Annotations[clusterv1.CRDMigrationObservedGenerationAnnotation] = currentGeneration
			if err := r.Client.Patch(ctx, crd, client.MergeFrom(originalCRD)); err != nil {
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	var customResourceObjects []client.Object
	if r.crdMigrationPhasesToRun.Has(StorageVersionMigrationPhase) && storageVersionMigrationRequired(crd, storageVersion) ||
		r.crdMigrationPhasesToRun.Has(CleanupManagedFieldsPhase) {
		// Get CustomResources only if we actually are going to run one of the phases.
		var err error
		customResourceObjects, err = r.getCustomResources(ctx, crd, migrationConfig, storageVersion)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// If .status.storedVersions != [storageVersion], run storage version migration.
	if r.crdMigrationPhasesToRun.Has(StorageVersionMigrationPhase) && storageVersionMigrationRequired(crd, storageVersion) {
		if err := r.reconcileStorageVersionMigration(ctx, crd, customResourceObjects, storageVersion); err != nil {
			return ctrl.Result{}, err
		}

		crd.Status.StoredVersions = []string{storageVersion}
		if err := r.Client.Status().Patch(ctx, crd, client.MergeFrom(originalCRD)); err != nil {
			return ctrl.Result{}, err
		}
	}

	if r.crdMigrationPhasesToRun.Has(CleanupManagedFieldsPhase) {
		return ctrl.Result{}, r.reconcileCleanupManagedFields(ctx, crd, customResourceObjects, migrationConfig, storageVersion)
	}

	return ctrl.Result{}, nil
}

func storageVersionMigrationRequired(crd *apiextensionsv1.CustomResourceDefinition, storageVersion string) bool {
	if len(crd.Status.StoredVersions) == 1 && crd.Status.StoredVersions[0] == storageVersion {
		return false
	}
	return true
}

func (r *CRDMigrator) getCustomResources(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, migrationConfig ByObjectConfig, storageVersion string) ([]client.Object, error) {
	var objs []client.Object

	listGVK := schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: storageVersion,
		Kind:    crd.Spec.Names.ListKind,
	}

	if migrationConfig.UseCache {
		// Note: We should only use the cached client with a typed object list.
		// Otherwise we would create an additional informer for an UnstructuredList/PartialObjectMetadataList.
		objList, err := r.Client.Scheme().New(listGVK)
		if err != nil {
			return nil, errors.Wrapf(err, "TODO")
		}
		objectList, ok := objList.(client.ObjectList)
		if !ok {
			return nil, errors.Wrapf(err, "TODO")
		}
		objects, err := listObjectsFromCachedClient(ctx, r.Client, objectList)
		if err != nil {
			return nil, err
		}
		objs = append(objs, objects...)
	} else {
		// Note: As the APIReader won't create an informer we can use PartialObjectMetadataList.
		// Note: PartialObjectMetadataList is used instead of UnstructuredList to avoid retrieving
		// data that we don't need.
		objectList := &metav1.PartialObjectMetadataList{}
		objectList.SetGroupVersionKind(listGVK)
		objects, err := listObjectsFromAPIReader(ctx, r.APIReader, objectList)
		if err != nil {
			return nil, err
		}
		objs = append(objs, objects...)
	}

	return objs, nil
}

func listObjectsFromCachedClient(ctx context.Context, c client.Client, objectList client.ObjectList) ([]client.Object, error) {
	objs := []client.Object{}

	if err := c.List(ctx, objectList); err != nil {
		return nil, err
	}

	objectListItems, err := meta.ExtractList(objectList)
	if err != nil {
		return nil, errors.Wrapf(err, "TODO")
	}
	for _, obj := range objectListItems {
		objs = append(objs, obj.(client.Object))
	}

	return objs, nil
}

func listObjectsFromAPIReader(ctx context.Context, c client.Reader, objectList client.ObjectList) ([]client.Object, error) {
	var objs []client.Object

	for {
		listOpts := []client.ListOption{
			client.Continue(objectList.GetContinue()),
			client.Limit(500),
		}
		if err := c.List(ctx, objectList, listOpts...); err != nil {
			return nil, err
		}

		objectListItems, err := meta.ExtractList(objectList)
		if err != nil {
			return nil, errors.Wrapf(err, "TODO")
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

func (r *CRDMigrator) reconcileStorageVersionMigration(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, customResourceObjects []client.Object, storageVersion string) error {
	if len(customResourceObjects) == 0 {
		return nil
	}

	gvk := schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: storageVersion,
		Kind:    crd.Spec.Names.Kind,
	}

	for _, obj := range customResourceObjects {
		e := objectEntry{
			Kind:      gvk.Kind,
			ObjectKey: client.ObjectKeyFromObject(obj),
		}

		if _, alreadyMigrated := r.storageVersionMigrationCache.Has(e.Key()); alreadyMigrated {
			continue
		}

		// Based on: https://github.com/kubernetes/kubernetes/blob/v1.32.0/pkg/controller/storageversionmigrator/storageversionmigrator.go#L275-L284
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		u.SetNamespace(obj.GetNamespace())
		u.SetName(obj.GetName())
		// Set UID so that when a resource gets deleted, we get an "uid mismatch"
		// conflict error instead of trying to create it.
		u.SetUID(obj.GetUID())
		// Set RV so that when a resources gets updated or deleted+recreated, we get an "object has been modified"
		// conflict error. We do not actually need to do anything special for the updated case because if RV
		// was not set, it would just result in no-op request. But for the deleted+recreated case, if RV is
		// not set but UID is set, we would get an immutable field validation error. Hence we must set both.
		u.SetResourceVersion(obj.GetResourceVersion())

		err := r.Client.Patch(ctx, u, client.Apply, client.FieldOwner("crdmigrator"))
		// If we got a NotFound error, the object no longer exists so no need to update it.
		// If we got a Conflict error, another client wrote the object already so no need to update it.
		if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			return err
		}

		r.storageVersionMigrationCache.Add(e)
	}

	return nil
}

func (r *CRDMigrator) reconcileCleanupManagedFields(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, customResourceObjects []client.Object, migrationConfig ByObjectConfig, storageVersion string) error {
	if len(customResourceObjects) == 0 {
		return nil
	}

	for _, obj := range customResourceObjects {
		var getErr error
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			managedFields, removed := removeManagedFieldsWithNotServedGroupVersion(obj, crd)
			if !removed {
				return nil
			}

			if len(managedFields) == 0 {
				// Add a seeding managedFieldEntry for SSA to prevent SSA to create/infer a default managedFieldEntry when the first SSA is applied.
				// More specifically, if an existing object doesn't have managedFields when applying the first SSA the API server
				// creates an entry with operation=Update (kind of guessing where the object comes from), but this entry ends up
				// acting as a co-ownership and we want to prevent this.
				// NOTE: fieldV1Map cannot be empty, so we add metadata.name which will be cleaned up at the first SSA patch.
				fieldV1Map := map[string]interface{}{
					"f:metadata": map[string]interface{}{
						"f:name": map[string]interface{}{},
					},
				}
				fieldV1, err := json.Marshal(fieldV1Map)
				if err != nil {
					return errors.Wrap(err, "failed to create seeding fieldV1Map for cleaning up managed fields of old apiVersions")
				}
				now := metav1.Now()
				managedFields = append(managedFields, metav1.ManagedFieldsEntry{
					Manager:    "crd-migrator",
					Operation:  metav1.ManagedFieldsOperationApply,
					APIVersion: schema.GroupVersion{Group: crd.Spec.Group, Version: storageVersion}.String(),
					Time:       &now,
					FieldsType: "FieldsV1",
					FieldsV1:   &metav1.FieldsV1{Raw: fieldV1},
				})
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
				if migrationConfig.UseCache {
					getErr = r.Client.Get(ctx, client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()}, obj)
				} else {
					getErr = r.APIReader.Get(ctx, client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()}, obj)
				}
			}
			// Note: We always have to return the conflict error directly (instead of an aggregate) so retry on conflict works.
			return err
		}); err != nil {
			return errors.Wrapf(kerrors.NewAggregate([]error{err, getErr}), "failed to migrate managedFields of %s %s/%s", crd.Spec.Names.Kind, obj.GetNamespace(), obj.GetName())
		}
	}

	return nil
}

func removeManagedFieldsWithNotServedGroupVersion(obj client.Object, crd *apiextensionsv1.CustomResourceDefinition) ([]metav1.ManagedFieldsEntry, bool) {
	servedGroupVersions := sets.Set[string]{}
	for _, v := range crd.Spec.Versions {
		if v.Served {
			servedGroupVersions.Insert(fmt.Sprintf("%s/%s", crd.Spec.Group, v.Name))
		}
	}

	notServedGroupVersion := func(entry metav1.ManagedFieldsEntry) bool {
		return !servedGroupVersions.Has(entry.APIVersion)
	}

	if !slices.ContainsFunc(obj.GetManagedFields(), notServedGroupVersion) {
		return nil, false
	}

	return slices.DeleteFunc(obj.GetManagedFields(), notServedGroupVersion), true
}

type objectEntry struct {
	Kind string
	client.ObjectKey
}

func (r objectEntry) Key() string {
	return fmt.Sprintf("%s %s", r.Kind, r.ObjectKey.String())
}
