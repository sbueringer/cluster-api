/*
Copyright 2021 The Kubernetes Authors.

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

package topology

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io;bootstrap.cluster.x-k8s.io;resources=*,verbs=delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments;machinedeployments/finalizers,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinesets;machinesets/finalizers,verbs=get;list;watch;update;patch;delete

// MachineDeploymentReconciler deletes referenced templates during deletion of topology-owned MachineDeployments and MachineSets.
// The templates are only deleted, if they are not used in other MachineDeployments or MachineSets which are not in deleting,
// i.e. the templates would otherwise be orphaned after the MachineDeployment or MachineSet deletion completes.
// Note: To achieve this the reconciler adds a finalizer, to hook into the MachineDeployment and MachineSet deletions.
type MachineDeploymentReconciler struct {
	Client           client.Client
	WatchFilterValue string
}

func (r *MachineDeploymentReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	_, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.MachineDeployment{}).
		Named("machinedeployment/topology").
		Watches(
			&source.Kind{Type: &clusterv1.MachineSet{}},
			handler.EnqueueRequestsFromMapFunc(r.machineSetToDeployments),
		).
		WithOptions(options).
		WithEventFilter(predicates.All(ctrl.LoggerFrom(ctx),
			predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue),
			// Note: Event filters are run for incoming events (i.e. MachineDeployment and MachineSet), not the
			// results of EnqueueRequestsFromMapFunc. Thus, it's important that topology-owned MachineDeployments *and*
			// MachineSets have the topology owned label to be filtered correctly.
			predicates.ResourceIsTopologyOwned(ctrl.LoggerFrom(ctx)),
		)).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}

	return nil
}

// Reconcile deletes referenced templates during deletion of topology-owned MachineDeployments and MachineSets.
// The templates are only deleted, if they are not used in other MachineDeployments or MachineSets which are not in deleting,
// i.e. the templates would otherwise be orphaned after the MachineDeployment or MachineSet deletion completes.
//
// This is achieved by:
// * adding finalizers to MachineDeployments and their corresponding MachineSets to hook into the deletion
// * calculating which templates are still used by MachineDeployments or MachineSets not in deleting
// * deleting templates of all MachineDeployments and MachineSets in deleting if they are not still used
//
// Some additional context:
// * MachineDeployment deletion:
//   * MachineDeployments are deleted and garbage collected first (without waiting until all MachineSets are also deleted)
//   * After that deletion of MachineSets is automatically triggered by Kubernetes based on owner references
// * MachineSet deletion:
//   * MachineSets are deleted and garbage collected first (without waiting until all Machines are also deleted)
//   * After that deletion of Machines is automatically triggered by Kubernetes based on owner references
//
// Note: We intentionally also reconcile the MachineSets by reconciling their MachineDeployment to avoid:
//       * duplicating the logic
//       * race conditions when deleting multiple MachineSets of the same MachineDeployment in parallel
//
// Note: We assume templates are not reused by different MachineDeployments, which is (only) true for topology-owned
//       MachineDeployments.
func (r *MachineDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the MachineDeployment instance (if it still exists).
	// Note: After a MachineDeployment has already been deleted, we will still "reconcile"
	// the MachineDeployment to clean up its corresponding MachineSets. Thus, it's valid in that case
	// that the MachineDeployment doesn't exist anymore.
	md := &clusterv1.MachineDeployment{}
	if err := r.Client.Get(ctx, req.NamespacedName, md); err != nil {
		if !apierrors.IsNotFound(err) {
			// Error reading the object - requeue the request.
			return ctrl.Result{}, errors.Wrapf(err, "failed to get MachineDeployment/%s", req.NamespacedName.Name)
		}

		// If the MachineDeployment doesn't exist anymore, set md to nil, so we can handle that case correctly below.
		md = nil
	}

	// Get the corresponding MachineSets.
	msList, err := r.getMachineSetsForDeployment(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Return early if the Cluster is paused.
	clusterPaused, err := r.isClusterPaused(ctx, md, msList)
	if err != nil {
		return ctrl.Result{}, err
	}
	if clusterPaused {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// Add finalizers to the MachineDeployment and the corresponding MachineSets.
	if err := r.addFinalizers(ctx, md, msList); err != nil {
		return ctrl.Result{}, err
	}

	// Calculate which templates are still in use by MachineDeployments or MachineSets which are not in deleting.
	templatesInUse, err := calculateTemplatesInUse(md, msList)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Delete unused templates of MachineSets which are in deleting.
	if err := r.deleteMachineSetsTemplatesIfNecessary(ctx, templatesInUse, msList); err != nil {
		return ctrl.Result{}, err
	}

	// Delete unused templates of the MachineDeployment if it is in deleting.
	if err := r.deleteMachineDeploymentTemplatesIfNecessary(ctx, templatesInUse, md); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// getMachineSetsForDeployment returns a list of MachineSets associated with a MachineDeployment.
func (r *MachineDeploymentReconciler) getMachineSetsForDeployment(ctx context.Context, md types.NamespacedName) ([]*clusterv1.MachineSet, error) {
	// List MachineSets based on the MachineDeployment label.
	msList := &clusterv1.MachineSetList{}
	if err := r.Client.List(ctx, msList,
		client.InNamespace(md.Namespace), client.MatchingLabels{clusterv1.MachineDeploymentLabelName: md.Name}); err != nil {
		return nil, errors.Wrapf(err, "failed to list MachineSets for MachineDeployment/%s", md.Name)
	}

	// Copy the MachineSets to an array of MachineSet pointers, to avoid MachineSet copying later.
	res := make([]*clusterv1.MachineSet, 0, len(msList.Items))
	for i := range msList.Items {
		res = append(res, &msList.Items[i])
	}
	return res, nil
}

func (r *MachineDeploymentReconciler) isClusterPaused(ctx context.Context, md *clusterv1.MachineDeployment, msList []*clusterv1.MachineSet) (bool, error) {
	var cluster *clusterv1.Cluster
	var err error

	// If the MachineDeployment still exists, get the Cluster from the MachineDeployment.
	if md != nil {
		// Fetch the corresponding Cluster.
		cluster, err = util.GetClusterByName(ctx, r.Client, md.ObjectMeta.Namespace, md.Spec.ClusterName)
		if err != nil {
			return false, errors.Wrapf(err, "failed to check if Cluster is paused")
		}
	}

	// Otherwise, get the Cluster from the first MachineSet, if possible.
	if len(msList) > 0 {
		cluster, err = util.GetClusterByName(ctx, r.Client, msList[0].ObjectMeta.Namespace, msList[0].Spec.ClusterName)
		if err != nil {
			return false, errors.Wrapf(err, "failed to check if Cluster is paused")
		}
	}

	if cluster == nil {
		// This should never happen as at least one MachineDeployment or MachineSet should
		// always exist during reconciliation.
		return false, errors.Errorf("couldn't find a Cluster to check if it is paused")
	}

	return annotations.IsPaused(cluster, md), nil
}

// calculateTemplatesInUse returns all templates referenced in non-deleting MachineDeployment and MachineSets.
func calculateTemplatesInUse(md *clusterv1.MachineDeployment, msList []*clusterv1.MachineSet) (map[string]bool, error) {
	templatesInUse := map[string]bool{}

	// Templates of the MachineSet are still in use if the MachineSet is not in deleting.
	for _, ms := range msList {
		if !ms.DeletionTimestamp.IsZero() {
			continue
		}

		bootstrapRef := ms.Spec.Template.Spec.Bootstrap.ConfigRef
		infrastructureRef := &ms.Spec.Template.Spec.InfrastructureRef
		if err := addRef(templatesInUse, bootstrapRef, infrastructureRef); err != nil {
			return nil, errors.Wrapf(err, "failed to add templates of %s to templatesInUse", KRef{Obj: ms})
		}
	}

	// MachineDeployment has already been deleted or still exists and is in deleting. In both cases there
	// are no templates referenced in the MachineDeployment which are still in use, so let's return here.
	if md == nil || !md.DeletionTimestamp.IsZero() {
		return templatesInUse, nil
	}

	// Templates of the MachineDeployment are still in use if the MachineDeployment is not in deleting.
	bootstrapRef := md.Spec.Template.Spec.Bootstrap.ConfigRef
	infrastructureRef := &md.Spec.Template.Spec.InfrastructureRef
	if err := addRef(templatesInUse, bootstrapRef, infrastructureRef); err != nil {
		return nil, errors.Wrapf(err, "failed to add templates of %s to templatesInUse", KRef{Obj: md})
	}
	return templatesInUse, nil
}

// addFinalizers adds finalizers to the MachineDeployment and the MachineSets, if they aren't already set.
func (r MachineDeploymentReconciler) addFinalizers(ctx context.Context, md *clusterv1.MachineDeployment, msList []*clusterv1.MachineSet) error {
	// Add finalizer to the MachineSets.
	for _, ms := range msList {
		if !controllerutil.ContainsFinalizer(ms, clusterv1.MachineSetTopologyFinalizer) {
			patchHelper, err := patch.NewHelper(ms, r.Client)
			if err != nil {
				return errors.Wrapf(err, "failed to create patch helper for %s", KRef{Obj: ms})
			}
			controllerutil.AddFinalizer(ms, clusterv1.MachineSetTopologyFinalizer)
			if err := patchHelper.Patch(ctx, ms); err != nil {
				return errors.Wrapf(err, "failed to patch %s", KRef{Obj: ms})
			}
		}
	}

	// If MachineDeployment has already been deleted, we don't have to add the finalizer to it.
	if md == nil {
		return nil
	}

	// Add finalizer to the MachineDeployment.
	if !controllerutil.ContainsFinalizer(md, clusterv1.MachineDeploymentTopologyFinalizer) {
		patchHelper, err := patch.NewHelper(md, r.Client)
		if err != nil {
			return errors.Wrapf(err, "failed to create patch helper for %s", KRef{Obj: md})
		}
		controllerutil.AddFinalizer(md, clusterv1.MachineDeploymentTopologyFinalizer)
		if err := patchHelper.Patch(ctx, md); err != nil {
			return errors.Wrapf(err, "failed to patch %s", KRef{Obj: md})
		}
	}
	return nil
}

// deleteMachineSetsTemplatesIfNecessary deletes templates referenced in MachineSets, if:
// * the MachineSet is in deleting
// * the MachineSet is not paused
// * the templates are not used by other MachineDeployments or MachineSets (i.e. they are not in templatesInUse).
//
// Note: We don't care about Machines, because the Machine deletion is triggered based
//       on owner references by Kubernetes *after* we remove the finalizer from the
//       MachineSet and the MachineSet has been garbage collected by Kubernetes.
func (r *MachineDeploymentReconciler) deleteMachineSetsTemplatesIfNecessary(ctx context.Context, templatesInUse map[string]bool, msList []*clusterv1.MachineSet) error {
	for _, ms := range msList {
		// MachineSet is not in deleting, do nothing.
		if ms.DeletionTimestamp.IsZero() {
			continue
		}
		// MachineSet is paused, do nothing.
		if annotations.HasPausedAnnotation(ms) {
			continue
		}

		// Delete the templates referenced in the MachineSet, if they are not used.
		ref := ms.Spec.Template.Spec.Bootstrap.ConfigRef
		if err := r.deleteTemplateIfNotUsed(ctx, templatesInUse, ref); err != nil {
			return errors.Wrapf(err, "failed to delete bootstrap template for %s", KRef{Obj: ms})
		}
		ref = &ms.Spec.Template.Spec.InfrastructureRef
		if err := r.deleteTemplateIfNotUsed(ctx, templatesInUse, ref); err != nil {
			return errors.Wrapf(err, "failed to delete infrastructure template for %s", KRef{Obj: ms})
		}

		// Remove the finalizer so the MachineSet can be deleted.
		patchHelper, err := patch.NewHelper(ms, r.Client)
		if err != nil {
			return errors.Wrapf(err, "failed to create patch helper for %s", KRef{Obj: ms})
		}
		controllerutil.RemoveFinalizer(ms, clusterv1.MachineSetTopologyFinalizer)
		if err := patchHelper.Patch(ctx, ms); err != nil {
			return errors.Wrapf(err, "failed to patch %s", KRef{Obj: ms})
		}
	}
	return nil
}

// deleteMachineDeploymentTemplatesIfNecessary deletes templates referenced in MachineDeployments, if:
// * the MachineDeployment exist and is in deleting
// * the MachineDeployment is not paused
// * the templates are not used by other MachineDeployments or MachineSets (i.e. they are not in templatesInUse).
//
// Note: We don't care about MachineSets, because the MachineSet deletion is triggered based
//       on owner references by Kubernetes *after* we remove the finalizer from the
//       MachineDeployment and the MachineDeployment has been garbage collected by Kubernetes.
func (r *MachineDeploymentReconciler) deleteMachineDeploymentTemplatesIfNecessary(ctx context.Context, templatesInUse map[string]bool, md *clusterv1.MachineDeployment) error {
	// MachineDeployment has already been deleted or still exists and is not in deleting, do nothing.
	if md == nil || md.DeletionTimestamp.IsZero() {
		return nil
	}
	// MachineDeployment is paused, do nothing.
	if annotations.HasPausedAnnotation(md) {
		return nil
	}

	// Delete the templates referenced in the MachineDeployment, if they are not used.
	ref := md.Spec.Template.Spec.Bootstrap.ConfigRef
	if err := r.deleteTemplateIfNotUsed(ctx, templatesInUse, ref); err != nil {
		return errors.Wrapf(err, "failed to delete bootstrap template for %s", KRef{Obj: md})
	}
	ref = &md.Spec.Template.Spec.InfrastructureRef
	if err := r.deleteTemplateIfNotUsed(ctx, templatesInUse, ref); err != nil {
		return errors.Wrapf(err, "failed to delete infrastructure template for %s", KRef{Obj: md})
	}

	// Remove the finalizer so the MachineDeployment can be deleted.
	patchHelper, err := patch.NewHelper(md, r.Client)
	if err != nil {
		return errors.Wrapf(err, "failed to create patch helper for %s", KRef{Obj: md})
	}
	controllerutil.RemoveFinalizer(md, clusterv1.MachineDeploymentTopologyFinalizer)
	if err := patchHelper.Patch(ctx, md); err != nil {
		return errors.Wrapf(err, "failed to patch %s", KRef{Obj: md})
	}
	return nil
}

// deleteTemplateIfNotUsed deletes the template (ref), if it is not in use (i.e. in templatesInUse).
func (r *MachineDeploymentReconciler) deleteTemplateIfNotUsed(ctx context.Context, templatesInUse map[string]bool, ref *corev1.ObjectReference) error {
	// If ref is nil, do nothing (this can happen, because bootstrap templates are optional).
	if ref == nil {
		return nil
	}

	log := loggerFrom(ctx).WithRef(ref)

	refID, err := refID(ref)
	if err != nil {
		return errors.Wrapf(err, "failed to calculate refID")
	}

	// If the template is still in use, do nothing.
	if templatesInUse[refID] {
		return nil
	}

	// TODO(sbueringer) use KRef after 5153 has been merged
	log.Infof("Deleting %s/%s", ref.Kind, ref.Name)
	if err := external.Delete(ctx, r.Client, ref); err != nil && !apierrors.IsNotFound(err) {
		// TODO(sbueringer) use KRef after 5153 has been merged
		return errors.Wrapf(err, "failed to delete %s/%s", ref.Kind, ref.Name)
	}
	return nil
}

// machineSetToDeployments is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// for MachineDeployments after changes to their MachineSets.
func (r *MachineDeploymentReconciler) machineSetToDeployments(o client.Object) []ctrl.Request {
	ms, ok := o.(*clusterv1.MachineSet)
	if !ok {
		panic(fmt.Sprintf("Expected a MachineSet but got a %T", o))
	}

	// Note: When deleting MachineDeployments, MachineDeployments are deleted before MachineSets.
	// So it can happen that we reconcile a MachineSet when the corresponding MachineDeployment
	// doesn't exist anymore. That's the reason why we intentionally directly return the NamespacedName,
	// instead of getting the resource first, as we do in similar funcs in other controllers.
	for _, ref := range ms.OwnerReferences {
		if ref.Kind != "MachineDeployment" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			// This should never happen.
			panic(fmt.Sprintf("Expected a valid groupVersion but got %q: %v", ref.APIVersion, err))
		}
		if gv.Group == clusterv1.GroupVersion.Group {
			return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: ms.Namespace, Name: ref.Name}}}
		}
	}
	return nil
}

// addRef adds the refs to the refMap with the refID as key.
func addRef(refMap map[string]bool, refs ...*corev1.ObjectReference) error {
	for _, ref := range refs {
		if ref != nil {
			refID, err := refID(ref)
			if err != nil {
				return errors.Wrapf(err, "failed to calculate refID")
			}
			refMap[refID] = true
		}
	}
	return nil
}

// refID returns the refID of a ObjectReference in the format: g/k/name.
// Note: We don't include the version as references with different versions should be treated as equal.
func refID(ref *corev1.ObjectReference) (string, error) {
	if ref == nil {
		return "", nil
	}

	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse apiVersion %q", ref.APIVersion)
	}

	return fmt.Sprintf("%s/%s/%s", gv.Group, ref.Kind, ref.Name), nil
}
