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

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1beta1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/api/core/v1beta2/index"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	"sigs.k8s.io/cluster-api/controllers/external"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimeclient "sigs.k8s.io/cluster-api/exp/runtime/client"
	"sigs.k8s.io/cluster-api/exp/topology/desiredstate"
	"sigs.k8s.io/cluster-api/exp/topology/scope"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/internal/controllers/topology/cluster/structuredmerge"
	"sigs.k8s.io/cluster-api/internal/hooks"
	capicontrollerutil "sigs.k8s.io/cluster-api/internal/util/controller"
	"sigs.k8s.io/cluster-api/internal/util/ssa"
	"sigs.k8s.io/cluster-api/internal/webhooks"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/cache"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io;bootstrap.cluster.x-k8s.io;controlplane.cluster.x-k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusterclasses,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinehealthchecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;create;delete

// Reconciler reconciles a managed topology for a Cluster object.
type Reconciler struct {
	Client       client.Client
	ClusterCache clustercache.ClusterCache
	// APIReader is used to list MachineSets directly via the API server to avoid
	// race conditions caused by an outdated cache.
	APIReader client.Reader

	RuntimeClient runtimeclient.Client

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	externalTracker external.ObjectTracker
	recorder        record.EventRecorder

	hookCache cache.Cache[cache.HookEntry]

	// desiredStateGenerator is used to generate the desired state.
	desiredStateGenerator desiredstate.Generator

	ssaCache ssa.Cache
}

var kcpScheme = runtime.NewScheme()

func init() {
	_ = controlplanev1.AddToScheme(kcpScheme)
	_ = controlplanev1beta1.AddToScheme(kcpScheme)
}

func (r *Reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	if r.Client == nil || r.APIReader == nil || r.ClusterCache == nil {
		return errors.New("Client, APIReader and ClusterCache must not be nil")
	}

	if feature.Gates.Enabled(feature.RuntimeSDK) && r.RuntimeClient == nil {
		return errors.New("RuntimeClient must not be nil")
	}

	predicateLog := ctrl.LoggerFrom(ctx).WithValues("controller", "topology/cluster")
	c, err := capicontrollerutil.NewControllerManagedBy(mgr, predicateLog).
		For(&clusterv1.Cluster{}, builder.WithPredicates(
			// Only reconcile Cluster with topology and with changes relevant for this controller.
			predicates.ClusterHasTopology(mgr.GetScheme(), predicateLog),
			clusterChangeIsRelevant(mgr.GetScheme(), predicateLog),
		)).
		Named("topology/cluster").
		WatchesRawSource(r.ClusterCache.GetClusterSource("topology/cluster", func(_ context.Context, o client.Object) []ctrl.Request {
			return []ctrl.Request{{NamespacedName: client.ObjectKeyFromObject(o)}}
		})).
		Watches(
			&clusterv1.ClusterClass{},
			handler.EnqueueRequestsFromMapFunc(r.clusterClassToCluster),
		).
		Watches(
			&clusterv1.MachineDeployment{},
			handler.EnqueueRequestsFromMapFunc(r.machineDeploymentToCluster),
			// Only trigger Cluster reconciliation if the MachineDeployment is topology owned, the resource is changed, and the change is relevant.
			predicates.ResourceIsTopologyOwned(mgr.GetScheme(), predicateLog),
			machineDeploymentChangeIsRelevant(mgr.GetScheme(), predicateLog),
		).
		Watches(
			&clusterv1.MachinePool{},
			handler.EnqueueRequestsFromMapFunc(r.machinePoolToCluster),
			// Only trigger Cluster reconciliation if the MachinePool is topology owned, the resource is changed.
			predicates.ResourceIsTopologyOwned(mgr.GetScheme(), predicateLog),
		).
		WithOptions(options).
		WithEventFilter(predicates.ResourceHasFilterLabel(mgr.GetScheme(), predicateLog, r.WatchFilterValue)).
		Build(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}

	r.externalTracker = external.ObjectTracker{
		Controller:      c,
		Cache:           mgr.GetCache(),
		Scheme:          mgr.GetScheme(),
		PredicateLogger: &predicateLog,
	}
	r.hookCache = cache.New[cache.HookEntry](cache.HookCacheDefaultTTL)
	r.desiredStateGenerator, err = desiredstate.NewGenerator(
		r.Client,
		r.ClusterCache,
		r.RuntimeClient,
		r.hookCache,
		// Note: We are using 10m so that we are able to relatively quickly pick up changes to the
		// upgrade plan from the extension if necessary.
		cache.New[desiredstate.GenerateUpgradePlanCacheEntry](10*time.Minute),
	)
	if err != nil {
		return errors.Wrap(err, "failed creating desired state generator")
	}

	r.recorder = mgr.GetEventRecorderFor("topology/cluster-controller")
	r.ssaCache = ssa.NewCache("topology/cluster")
	return nil
}

func clusterChangeIsRelevant(scheme *runtime.Scheme, logger logr.Logger) predicate.Funcs {
	dropNotRelevant := func(cluster *clusterv1.Cluster) *clusterv1.Cluster {
		c := cluster.DeepCopy()
		// Drop metadata fields which are impacted by not relevant changes.
		c.ManagedFields = nil
		c.ResourceVersion = ""
		return c
	}

	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := logger.WithValues("predicate", "ClusterChangeIsRelevant", "eventType", "update")
			if gvk, err := apiutil.GVKForObject(e.ObjectOld, scheme); err == nil {
				log = log.WithValues(gvk.Kind, klog.KObj(e.ObjectOld))
			}

			if e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
				log.V(6).Info("Cluster resync event, allowing further processing")
				return true
			}

			oldObj, ok := e.ObjectOld.(*clusterv1.Cluster)
			if !ok {
				log.V(4).Info("Expected Cluster", "type", fmt.Sprintf("%T", e.ObjectOld))
				return false
			}
			oldObj = dropNotRelevant(oldObj)

			newObj := e.ObjectNew.(*clusterv1.Cluster)
			if !ok {
				log.V(4).Info("Expected Cluster", "type", fmt.Sprintf("%T", e.ObjectNew))
				return false
			}
			newObj = dropNotRelevant(newObj)

			if reflect.DeepEqual(oldObj, newObj) {
				log.V(6).Info("Cluster does not have relevant changes, blocking further processing")
				return false
			}
			log.V(6).Info("Cluster has relevant changes, allowing further processing")
			return true
		},
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return true },
	}
}

func machineDeploymentChangeIsRelevant(scheme *runtime.Scheme, logger logr.Logger) predicate.Funcs {
	dropNotRelevant := func(machineDeployment *clusterv1.MachineDeployment) *clusterv1.MachineDeployment {
		md := machineDeployment.DeepCopy()
		// Drop metadata fields which are impacted by not relevant changes.
		md.ManagedFields = nil
		md.ResourceVersion = ""
		return md
	}

	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := logger.WithValues("predicate", "MachineDeploymentChangeIsRelevant", "eventType", "update")
			if gvk, err := apiutil.GVKForObject(e.ObjectOld, scheme); err == nil {
				log = log.WithValues(gvk.Kind, klog.KObj(e.ObjectOld))
			}

			oldObj, ok := e.ObjectOld.(*clusterv1.MachineDeployment)
			if !ok {
				log.V(4).Info("Expected MachineDeployment", "type", fmt.Sprintf("%T", e.ObjectOld))
				return false
			}
			oldObj = dropNotRelevant(oldObj)

			newObj := e.ObjectNew.(*clusterv1.MachineDeployment)
			if !ok {
				log.V(4).Info("Expected MachineDeployment", "type", fmt.Sprintf("%T", e.ObjectNew))
				return false
			}
			newObj = dropNotRelevant(newObj)

			if reflect.DeepEqual(oldObj, newObj) {
				log.V(6).Info("MachineDeployment does not have relevant changes, blocking further processing")
				return false
			}
			log.V(6).Info("MachineDeployment has relevant changes, allowing further processing")
			return true
		},
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return true },
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	// Fetch the Cluster instance.
	cluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	cluster.APIVersion = clusterv1.GroupVersion.String()
	cluster.Kind = "Cluster"

	// Return early, if the Cluster does not use a managed topology.
	// NOTE: We're already filtering events, but this is a safeguard for cases like e.g. when
	// there are MachineDeployments which have the topology owned label, but the corresponding
	// cluster is not topology owned.
	if !cluster.Spec.Topology.IsDefined() {
		return ctrl.Result{}, nil
	}

	patchHelper, err := patch.NewHelper(cluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create a scope initialized with only the cluster; during reconcile
	// additional information will be added about the Cluster blueprint, current state and desired state.
	s := scope.New(cluster)

	defer func() {
		if err := r.reconcileConditions(s, cluster, reterr); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, errors.Wrap(err, "failed to reconcile cluster topology conditions")})
			return
		}
		options := []patch.Option{
			patch.WithOwnedV1Beta1Conditions{Conditions: []clusterv1.ConditionType{
				clusterv1.TopologyReconciledV1Beta1Condition,
			}},
			patch.WithOwnedV1Beta1Conditions{Conditions: []clusterv1.ConditionType{
				clusterv1.ClusterTopologyReconciledCondition,
			}},
		}
		if err := patchHelper.Patch(ctx, cluster, options...); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
			return
		}
	}()

	// Return early if the Cluster is paused.
	//if ptr.Deref(cluster.Spec.Paused, false) || annotations.HasPaused(cluster) {
	//	return ctrl.Result{}, nil
	//}

	// In case the object is deleted, the managed topology stops to reconcile;
	// (the other controllers will take care of deletion).
	if !cluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, s)
	}

	// Handle normal reconciliation loop.
	return r.reconcile(ctx, s)
}

// reconcile handles cluster reconciliation.
func (r *Reconciler) reconcile(ctx context.Context, s *scope.Scope) (ctrl.Result, error) {
	var err error

	// Get ClusterClass.
	clusterClass := &clusterv1.ClusterClass{}
	key := s.Current.Cluster.GetClassKey()
	if err := r.Client.Get(ctx, key, clusterClass); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to retrieve ClusterClass %s", key)
	}

	s.Blueprint.ClusterClass = clusterClass
	// If the ClusterClass `metadata.Generation` doesn't match the `status.ObservedGeneration` return as the ClusterClass
	// is not up to date.
	// Note: This doesn't require requeue as a change to ClusterClass observedGeneration will cause an additional reconcile
	// in the Cluster.
	if !conditions.Has(clusterClass, clusterv1.ClusterClassVariablesReadyCondition) ||
		conditions.IsFalse(clusterClass, clusterv1.ClusterClassVariablesReadyCondition) {
		return ctrl.Result{}, errors.Errorf("ClusterClass is not successfully reconciled: status of %s condition on ClusterClass must be \"True\"", clusterv1.ClusterClassVariablesReadyCondition)
	}
	if clusterClass.GetGeneration() != clusterClass.Status.ObservedGeneration {
		return ctrl.Result{}, errors.Errorf("ClusterClass is not successfully reconciled: ClusterClass.status.observedGeneration must be %d, but is %d", clusterClass.GetGeneration(), clusterClass.Status.ObservedGeneration)
	}

	// Default and Validate the Cluster variables based on information from the ClusterClass.
	// This step is needed as if the ClusterClass does not exist at Cluster creation some fields may not be defaulted or
	// validated in the webhook.
	if errs := webhooks.DefaultAndValidateVariables(ctx, s.Current.Cluster, nil, clusterClass); len(errs) > 0 {
		return ctrl.Result{}, apierrors.NewInvalid(clusterv1.GroupVersion.WithKind("Cluster").GroupKind(), s.Current.Cluster.Name, errs)
	}

	// Gets the blueprint with the ClusterClass and the referenced templates
	// and store it in the request scope.
	s.Blueprint, err = r.getBlueprint(ctx, s.Current.Cluster, s.Blueprint.ClusterClass)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "error reading the ClusterClass")
	}

	// Gets the current state of the Cluster and store it in the request scope.
	s.Current, err = r.getCurrentState(ctx, s)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "error reading current state of the Cluster topology")
	}

	// The cluster topology is yet to be created. Call the BeforeClusterCreate hook before proceeding.
	if feature.Gates.Enabled(feature.RuntimeSDK) {
		res, err := r.callBeforeClusterCreateHook(ctx, s)
		if err != nil {
			return reconcile.Result{}, err
		}
		if !res.IsZero() {
			return res, nil
		}
	}

	// Setup watches for InfrastructureCluster and ControlPlane CRs when they exist.
	if err := r.setupDynamicWatches(ctx, s); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "error creating dynamic watch")
	}

	if s.Current.ControlPlane != nil && s.Current.ControlPlane.Object != nil && s.Current.ControlPlane.Object.GroupVersionKind().Kind == "KubeadmControlPlane" {
		// Note: We want to return the Reconcile when we fixed up managedFields
		// Note: We want to fixup managedFields as soon as possible
		// Note: It might be good to fixup managedFields in this controller as we can then ensure that this controller is not writing these objects concurrently
		// FIXME: move all into mitigateManagedFieldIssue and go through all objects that are written with SSA
		changed, err := r.mitigateManagedFieldIssue(ctx, s.Current.ControlPlane.Object)
		if err != nil {
			return ctrl.Result{}, err
		}
		if changed {
			return ctrl.Result{}, nil // No requeue needed, change will trigger another reconcile.
		}
	}

	// Computes the desired state of the Cluster and store it in the request scope.
	s.Desired, err = r.desiredStateGenerator.Generate(ctx, s)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "error computing the desired state of the Cluster topology")
	}

	// Reconciles current and desired state of the Cluster
	if err := r.reconcileState(ctx, s); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "error reconciling the Cluster topology")
	}

	// requeueAfter will not be 0 if any of the runtime hooks returns a blocking response.
	requeueAfter := s.HookResponseTracker.AggregateRetryAfter()
	if requeueAfter != 0 {
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	return ctrl.Result{}, nil
}

// setupDynamicWatches create watches for InfrastructureCluster and ControlPlane CRs when they exist.
func (r *Reconciler) setupDynamicWatches(ctx context.Context, s *scope.Scope) error {
	scheme := r.Client.Scheme()
	if s.Current.InfrastructureCluster != nil {
		if err := r.externalTracker.Watch(ctrl.LoggerFrom(ctx), s.Current.InfrastructureCluster,
			handler.EnqueueRequestForOwner(scheme, r.Client.RESTMapper(), &clusterv1.Cluster{}),
			// Only trigger Cluster reconciliation if the InfrastructureCluster is topology owned.
			predicates.ResourceIsChanged(scheme, *r.externalTracker.PredicateLogger),
			predicates.ResourceIsTopologyOwned(scheme, *r.externalTracker.PredicateLogger),
		); err != nil {
			return errors.Wrap(err, "error watching Infrastructure CR")
		}
	}
	if s.Current.ControlPlane.Object != nil {
		if err := r.externalTracker.Watch(ctrl.LoggerFrom(ctx), s.Current.ControlPlane.Object,
			handler.EnqueueRequestForOwner(scheme, r.Client.RESTMapper(), &clusterv1.Cluster{}),
			// Only trigger Cluster reconciliation if the ControlPlane is topology owned.
			predicates.ResourceIsChanged(scheme, *r.externalTracker.PredicateLogger),
			predicates.ResourceIsTopologyOwned(scheme, *r.externalTracker.PredicateLogger),
		); err != nil {
			return errors.Wrap(err, "error watching ControlPlane CR")
		}
	}
	return nil
}

func (r *Reconciler) mitigateManagedFieldIssue(ctx context.Context, controlPlane *unstructured.Unstructured) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	if len(controlPlane.GetManagedFields()) == 0 || slices.ContainsFunc(controlPlane.GetManagedFields(), func(entry metav1.ManagedFieldsEntry) bool { // FIXME: flip this if
		return entry.Manager == "before-first-apply"
	}) {
		// Case 1: no managedFields                          => [capi-topology]
		// Case 2: before-first-apply entry
		//   * a: [before-first-apply]                       => [capi-topology]
		//   * b: [before-first-apply,manager]               => [capi-topology,manager]
		//   * c: [before-first-apply,capi-topology,manager] => [capi-topology,manager]
		managedFields := slices.DeleteFunc(controlPlane.GetManagedFields(), func(entry metav1.ManagedFieldsEntry) bool {
			return entry.Manager == "before-first-apply"
		})

		if !slices.ContainsFunc(controlPlane.GetManagedFields(), func(entry metav1.ManagedFieldsEntry) bool {
			return entry.Manager == structuredmerge.TopologyManagerName
		}) {
			// Add a seeding managedFieldEntry for SSA to prevent SSA to create/infer a default managedFieldEntry
			// when the first SSA is applied.
			// More specifically, if an existing object doesn't have managedFields when applying the first SSA
			// the API server creates an entry with operation=Update (kind of guessing where the object comes from),
			// but this entry ends up acting as a co-ownership and we want to prevent this.
			// NOTE: fieldV1Map cannot be empty, so we add metadata.name which will be cleaned up at the first
			//       SSA patch of the same fieldManager.
			// NOTE: We use the fieldManager and operation from the managedFields that we remove to increase the
			//       chances that the managedField entry gets cleaned up. In any case having a minimal entry only
			//       for metadata.name is better than leaving the old entry that uses an apiVersion that is not
			//       served anymore (see: https://github.com/kubernetes/kubernetes/issues/111937).
			managedFieldSet := fieldpath.NewSet()
			switch controlPlane.GroupVersionKind().Version {
			case controlplanev1.GroupVersion.Version:
				kcp := &controlplanev1.KubeadmControlPlane{}
				if err := kcpScheme.Convert(controlPlane, kcp, nil); err != nil {
					return false, errors.Wrap(err, "failed to convert original object to Unstructured")
				}

				addArgs(managedFieldSet, kcp.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs,
					"spec", "kubeadmConfigSpec", "clusterConfiguration", "apiServer", "extraArgs")
				addArgs(managedFieldSet, kcp.Spec.KubeadmConfigSpec.ClusterConfiguration.ControllerManager.ExtraArgs,
					"spec", "kubeadmConfigSpec", "clusterConfiguration", "controllerManager", "extraArgs")
				addArgs(managedFieldSet, kcp.Spec.KubeadmConfigSpec.ClusterConfiguration.Scheduler.ExtraArgs,
					"spec", "kubeadmConfigSpec", "clusterConfiguration", "scheduler", "extraArgs")
				addArgs(managedFieldSet, kcp.Spec.KubeadmConfigSpec.ClusterConfiguration.Etcd.Local.ExtraArgs,
					"spec", "kubeadmConfigSpec", "clusterConfiguration", "etcd", "local", "extraArgs")
				addArgs(managedFieldSet, kcp.Spec.KubeadmConfigSpec.InitConfiguration.NodeRegistration.KubeletExtraArgs,
					"spec", "kubeadmConfigSpec", "initConfiguration", "nodeRegistration", "kubeletExtraArgs")
				addArgs(managedFieldSet, kcp.Spec.KubeadmConfigSpec.JoinConfiguration.NodeRegistration.KubeletExtraArgs,
					"spec", "kubeadmConfigSpec", "joinConfiguration", "nodeRegistration", "kubeletExtraArgs")

			case controlplanev1beta1.GroupVersion.Version:
				kcp := &controlplanev1beta1.KubeadmControlPlane{}
				if err := kcpScheme.Convert(controlPlane, kcp, nil); err != nil {
					return false, errors.Wrap(err, "failed to convert original object to Unstructured")
				}
				if kcp.Spec.KubeadmConfigSpec.ClusterConfiguration != nil {
					addArgsV1Beta1(managedFieldSet, kcp.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs,
						"spec", "kubeadmConfigSpec", "clusterConfiguration", "apiServer", "extraArgs")
					addArgsV1Beta1(managedFieldSet, kcp.Spec.KubeadmConfigSpec.ClusterConfiguration.ControllerManager.ExtraArgs,
						"spec", "kubeadmConfigSpec", "clusterConfiguration", "controllerManager", "extraArgs")
					addArgsV1Beta1(managedFieldSet, kcp.Spec.KubeadmConfigSpec.ClusterConfiguration.Scheduler.ExtraArgs,
						"spec", "kubeadmConfigSpec", "clusterConfiguration", "scheduler", "extraArgs")
					if kcp.Spec.KubeadmConfigSpec.ClusterConfiguration.Etcd.Local != nil {
						addArgsV1Beta1(managedFieldSet, kcp.Spec.KubeadmConfigSpec.ClusterConfiguration.Etcd.Local.ExtraArgs,
							"spec", "kubeadmConfigSpec", "clusterConfiguration", "etcd", "local", "extraArgs")
					}
				}
				if kcp.Spec.KubeadmConfigSpec.InitConfiguration != nil {
					addArgsV1Beta1(managedFieldSet, kcp.Spec.KubeadmConfigSpec.InitConfiguration.NodeRegistration.KubeletExtraArgs,
						"spec", "kubeadmConfigSpec", "initConfiguration", "nodeRegistration", "kubeletExtraArgs")
				}
				if kcp.Spec.KubeadmConfigSpec.JoinConfiguration != nil {
					addArgsV1Beta1(managedFieldSet, kcp.Spec.KubeadmConfigSpec.JoinConfiguration.NodeRegistration.KubeletExtraArgs,
						"spec", "kubeadmConfigSpec", "joinConfiguration", "nodeRegistration", "kubeletExtraArgs")
				}
			}

			if managedFieldSet.Empty() {
				managedFieldSet.Insert(fieldpath.MakePathOrDie("metadata", "name"))
			}

			fieldV1, err := managedFieldSet.ToJSON()
			if err != nil {
				return false, errors.Wrap(err, "failed to create seeding managedField entry")
			}
			managedFields = append(managedFields, metav1.ManagedFieldsEntry{
				Manager:    structuredmerge.TopologyManagerName,
				Operation:  "Apply",
				APIVersion: controlPlane.GetAPIVersion(),
				Time:       ptr.To(metav1.Now()),
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
				"op":    "replace",
				"path":  "/metadata/resourceVersion",
				"value": controlPlane.GetResourceVersion(),
			},
		}
		patch, err := json.Marshal(jsonPatch)
		if err != nil {
			return false, errors.Wrap(err, "failed to marshal patch")
		}

		log.V(4).Info("Fixing up managedFields", controlPlane.GetKind(), klog.KObj(controlPlane))
		if err := r.Client.Patch(ctx, controlPlane, client.RawPatch(types.JSONPatchType, patch)); err != nil {
			return false, errors.Wrap(err, "failed to fixup managedFields")
		}

		return true, nil
	}

	return false, nil
}

func addArgs(managedFieldSet *fieldpath.Set, args []bootstrapv1.Arg, fields ...any) {
	if len(args) == 0 { // FIXME: verify if we got the managedFields right for the empty case
		return
	}

	for _, arg := range args {
		managedFieldSet.Insert(fieldpath.MakePathOrDie(append(fields, fieldpath.KeyByFields("name", arg.Name, "value", ptr.Deref(arg.Value, "")))...))
		managedFieldSet.Insert(fieldpath.MakePathOrDie(append(fields, fieldpath.KeyByFields("name", arg.Name, "value", ptr.Deref(arg.Value, "")), "name")...))
		managedFieldSet.Insert(fieldpath.MakePathOrDie(append(fields, fieldpath.KeyByFields("name", arg.Name, "value", ptr.Deref(arg.Value, "")), "value")...))
	}
}

func addArgsV1Beta1(managedFieldSet *fieldpath.Set, args map[string]string, fields ...any) {
	if len(args) == 0 { // FIXME: verify if we got the managedFields right for the empty case
		return
	}

	for argName := range args {
		managedFieldSet.Insert(fieldpath.MakePathOrDie(append(fields, argName)...))
	}
}

func (r *Reconciler) callBeforeClusterCreateHook(ctx context.Context, s *scope.Scope) (reconcile.Result, error) {
	// If the cluster objects (InfraCluster, ControlPlane, etc) are not yet created we are in the creation phase.
	// Call the BeforeClusterCreate hook before proceeding.
	log := ctrl.LoggerFrom(ctx)

	if !s.Current.Cluster.Spec.InfrastructureRef.IsDefined() && !s.Current.Cluster.Spec.ControlPlaneRef.IsDefined() {
		// Return quickly if the hook is not defined.
		extensionHandlers, err := r.RuntimeClient.GetAllExtensions(ctx, runtimehooksv1.BeforeClusterCreate, s.Current.Cluster)
		if err != nil {
			return ctrl.Result{}, err
		}
		if len(extensionHandlers) == 0 {
			return ctrl.Result{}, nil
		}

		if cacheEntry, ok := r.hookCache.Has(cache.NewHookEntryKey(s.Current.Cluster, runtimehooksv1.BeforeClusterCreate)); ok {
			if requeueAfter, requeue := cacheEntry.ShouldRequeue(time.Now()); requeue {
				log.V(5).Info(fmt.Sprintf("Skip calling BeforeClusterCreate hook, retry after %s", requeueAfter))
				s.HookResponseTracker.Add(runtimehooksv1.BeforeClusterCreate, cacheEntry.ToResponse(&runtimehooksv1.BeforeClusterCreateResponse{}, requeueAfter))
				return ctrl.Result{RequeueAfter: requeueAfter}, nil
			}
		}

		hookRequest := &runtimehooksv1.BeforeClusterCreateRequest{
			Cluster: *cleanupCluster(s.Current.Cluster),
		}
		hookResponse := &runtimehooksv1.BeforeClusterCreateResponse{}
		if err := r.RuntimeClient.CallAllExtensions(ctx, runtimehooksv1.BeforeClusterCreate, s.Current.Cluster, hookRequest, hookResponse); err != nil {
			return ctrl.Result{}, err
		}
		s.HookResponseTracker.Add(runtimehooksv1.BeforeClusterCreate, hookResponse)

		if hookResponse.RetryAfterSeconds != 0 {
			r.hookCache.Add(cache.NewHookEntry(s.Current.Cluster, runtimehooksv1.BeforeClusterCreate, time.Now().Add(time.Duration(hookResponse.RetryAfterSeconds)*time.Second), hookResponse.GetMessage()))
			log.Info(fmt.Sprintf("Creation of Cluster topology is blocked by %s hook, retry after %ds", runtimecatalog.HookName(runtimehooksv1.BeforeClusterCreate), hookResponse.RetryAfterSeconds))
			return ctrl.Result{RequeueAfter: time.Duration(hookResponse.RetryAfterSeconds) * time.Second}, nil
		}

		log.Info(fmt.Sprintf("Creation of Cluster topology unblocked by %s hook", runtimecatalog.HookName(runtimehooksv1.BeforeClusterCreate)))
	}
	return ctrl.Result{}, nil
}

// clusterClassToCluster is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// for Cluster to update when its own ClusterClass gets updated.
func (r *Reconciler) clusterClassToCluster(ctx context.Context, o client.Object) []ctrl.Request {
	clusterClass, ok := o.(*clusterv1.ClusterClass)
	if !ok {
		panic(fmt.Sprintf("Expected a ClusterClass but got a %T", o))
	}

	clusterList := &clusterv1.ClusterList{}
	if err := r.Client.List(
		ctx,
		clusterList,
		client.MatchingFields{
			index.ClusterClassRefPath: index.ClusterClassRef(clusterClass),
		},
	); err != nil {
		return nil
	}

	// There can be more than one cluster using the same cluster class.
	// create a request for each of the clusters.
	requests := []ctrl.Request{}
	for i := range clusterList.Items {
		requests = append(requests, ctrl.Request{NamespacedName: util.ObjectKey(&clusterList.Items[i])})
	}
	return requests
}

// machineDeploymentToCluster is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// for Cluster to update when one of its own MachineDeployments gets updated.
func (r *Reconciler) machineDeploymentToCluster(_ context.Context, o client.Object) []ctrl.Request {
	md, ok := o.(*clusterv1.MachineDeployment)
	if !ok {
		panic(fmt.Sprintf("Expected a MachineDeployment but got a %T", o))
	}
	if md.Spec.ClusterName == "" {
		return nil
	}

	return []ctrl.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: md.Namespace,
			Name:      md.Spec.ClusterName,
		},
	}}
}

// machinePoolToCluster is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// for Cluster to update when one of its own MachinePools gets updated.
func (r *Reconciler) machinePoolToCluster(_ context.Context, o client.Object) []ctrl.Request {
	mp, ok := o.(*clusterv1.MachinePool)
	if !ok {
		panic(fmt.Sprintf("Expected a MachinePool but got a %T", o))
	}
	if mp.Spec.ClusterName == "" {
		return nil
	}

	return []ctrl.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: mp.Namespace,
			Name:      mp.Spec.ClusterName,
		},
	}}
}

func (r *Reconciler) reconcileDelete(ctx context.Context, s *scope.Scope) (ctrl.Result, error) {
	cluster := s.Current.Cluster

	// Call the BeforeClusterDelete hook if the 'ok-to-delete' annotation is not set
	// and add the annotation to the cluster after receiving a successful non-blocking response.
	log := ctrl.LoggerFrom(ctx)
	if feature.Gates.Enabled(feature.RuntimeSDK) {
		if !hooks.IsOkToDelete(cluster) {
			// Return quickly if the hook is not defined.
			extensionHandlers, err := r.RuntimeClient.GetAllExtensions(ctx, runtimehooksv1.BeforeClusterDelete, s.Current.Cluster)
			if err != nil {
				return ctrl.Result{}, err
			}
			if len(extensionHandlers) == 0 {
				if err := hooks.MarkAsOkToDelete(ctx, r.Client, cluster, false); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}

			if cacheEntry, ok := r.hookCache.Has(cache.NewHookEntryKey(s.Current.Cluster, runtimehooksv1.BeforeClusterDelete)); ok {
				if requeueAfter, requeue := cacheEntry.ShouldRequeue(time.Now()); requeue {
					log.V(5).Info(fmt.Sprintf("Skip calling BeforeClusterDelete hook, retry after %s", requeueAfter))
					s.HookResponseTracker.Add(runtimehooksv1.BeforeClusterDelete, cacheEntry.ToResponse(&runtimehooksv1.BeforeClusterDeleteResponse{}, requeueAfter))
					return ctrl.Result{RequeueAfter: requeueAfter}, nil
				}
			}

			hookRequest := &runtimehooksv1.BeforeClusterDeleteRequest{
				Cluster: *cleanupCluster(cluster),
			}
			hookResponse := &runtimehooksv1.BeforeClusterDeleteResponse{}
			if err := r.RuntimeClient.CallAllExtensions(ctx, runtimehooksv1.BeforeClusterDelete, cluster, hookRequest, hookResponse); err != nil {
				return ctrl.Result{}, err
			}
			// Add the response to the tracker so we can later update condition or requeue when required.
			s.HookResponseTracker.Add(runtimehooksv1.BeforeClusterDelete, hookResponse)

			if hookResponse.RetryAfterSeconds != 0 {
				r.hookCache.Add(cache.NewHookEntry(s.Current.Cluster, runtimehooksv1.BeforeClusterDelete, time.Now().Add(time.Duration(hookResponse.RetryAfterSeconds)*time.Second), hookResponse.GetMessage()))
				log.Info(fmt.Sprintf("Cluster deletion is blocked by %q hook, retry after %ds", runtimecatalog.HookName(runtimehooksv1.BeforeClusterDelete), hookResponse.RetryAfterSeconds))
				return ctrl.Result{RequeueAfter: time.Duration(hookResponse.RetryAfterSeconds) * time.Second}, nil
			}
			// The BeforeClusterDelete hook returned a non-blocking response. Now the cluster is ready to be deleted.
			// Lets mark the cluster as `ok-to-delete`
			if err := hooks.MarkAsOkToDelete(ctx, r.Client, cluster, false); err != nil {
				return ctrl.Result{}, err
			}
			log.Info(fmt.Sprintf("Cluster deletion is unblocked by %s hook", runtimecatalog.HookName(runtimehooksv1.BeforeClusterDelete)))
		}
	}
	return ctrl.Result{}, nil
}

func cleanupCluster(cluster *clusterv1.Cluster) *clusterv1.Cluster {
	cluster = cluster.DeepCopy()

	// Optimize size of Cluster by not sending status, the managedFields and some specific annotations.
	cluster.SetManagedFields(nil)

	// The conversion that we run before calling cleanupCluster does not clone annotations
	// So we have to do it here to not modify the original Cluster.
	if cluster.Annotations != nil {
		annotations := maps.Clone(cluster.Annotations)
		delete(annotations, corev1.LastAppliedConfigAnnotation)
		delete(annotations, conversion.DataAnnotation)
		cluster.Annotations = annotations
	}
	cluster.Status = clusterv1.ClusterStatus{}
	return cluster
}
