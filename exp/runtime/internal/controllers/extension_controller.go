/*
Copyright 2022 The Kubernetes Authors.

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

package controllers

import (
	"context"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimev1 "sigs.k8s.io/cluster-api/exp/runtime/api/v1beta1"
	runtimeclient "sigs.k8s.io/cluster-api/internal/runtime/client"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
)

// FIXME: Replace all direct calls to registry with Client equivalents when available.

// +kubebuilder:rbac:groups=runtime.cluster.x-k8s.io,resources=extensions,verbs=get;list;watch;create;update;patch;delete

// Reconciler reconciles an Extension object.
type Reconciler struct {
	Client        client.Client
	APIReader     client.Reader
	RuntimeClient runtimeclient.Client
	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

func (r *Reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&runtimev1.Extension{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}
	return nil
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	if !r.RuntimeClient.IsReady() {
		err := r.syncRegistry(ctx)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "extensions controller not initialized")
		}
		// After the reconciler is warmed up requeue the request straight away.
		return ctrl.Result{Requeue: true}, nil
	}

	extension := &runtimev1.Extension{}
	err := r.Client.Get(ctx, req.NamespacedName, extension)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Extension not found. Remove from registry.
			err = r.RuntimeClient.Extension(extension).Unregister()
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	original := extension.DeepCopy()
	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(original, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		// Always attempt to patch the object and status after each reconciliation.
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{}
		patchOpts = append(patchOpts, patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			runtimev1.RuntimeExtensionDiscovered,
		}})
		if err := patchExtension(ctx, patchHelper, extension, patchOpts...); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deletion reconciliation loop.
	if !extension.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, extension)
	}

	if extension, err = r.discoverRuntimeExtensions(ctx, extension); err != nil {
		extension = original
		conditions.MarkFalse(extension, runtimev1.RuntimeExtensionDiscovered, "DiscoveryFailed", clusterv1.ConditionSeverityError, "error in discovery: %v", err)
		return ctrl.Result{}, err
	}
	conditions.MarkTrue(extension, runtimev1.RuntimeExtensionDiscovered)
	return ctrl.Result{}, nil
}

func patchExtension(ctx context.Context, patchHelper *patch.Helper, ext *runtimev1.Extension, options ...patch.Option) error {
	// Can add errors and waits here to the Extension object.
	return patchHelper.Patch(ctx, ext, options...)
}

func (r *Reconciler) reconcileDelete(_ context.Context, extension *runtimev1.Extension) (ctrl.Result, error) {
	// FIXME: This isn't implemented on the client/registry.
	if err := r.RuntimeClient.Extension(extension).Unregister(); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// syncRegistry only does one full, error-free, read of all Extensions and re-registers them in the Registry through
// the client. It does not discover any new extensions.
func (r *Reconciler) syncRegistry(ctx context.Context) (reterr error) {
	extensionList := runtimev1.ExtensionList{}
	if err := r.APIReader.List(ctx, &extensionList); err != nil {
		return err
	}

	// A mix of calls that mutate and calls that return (e.g. WarmUp vs Discovery) make this more difficult to understand.
	// IMO it would be better to always return the updated object or list.
	// The below means that any failure in WarmUp will result in no extensions being updated - no partial WarmUp.
	originalList := extensionList.DeepCopy()
	if err := r.RuntimeClient.WarmUp(&extensionList); err != nil {
		return err
	}

	var errs []error
	for i, ext := range originalList.Items {
		// Initialize the patch helper for this extension.
		extension := extensionList.Items[i].DeepCopy()
		patchHelper, err := patch.NewHelper(ext.DeepCopy(), r.Client)
		if err != nil {
			errs = append(errs, err)
		}

		// At this point all extensions in the list have been discovered correctly.
		conditions.MarkTrue(extension, runtimev1.RuntimeExtensionDiscovered)

		defer func(extension runtimev1.Extension) { //nolint:gocritic
			// Always attempt to patch the object and status after each reconciliation.
			patchOpts := []patch.Option{}
			patchOpts = append(patchOpts, patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
				runtimev1.RuntimeExtensionDiscovered,
			}})
			if err = patchExtension(ctx, patchHelper, &extension, patchOpts...); err != nil {
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}(*extension)
	}

	if len(errs) != 0 {
		return kerrors.NewAggregate(errs)
	}
	// If all extensions are listed and discovered successfully the reconciler is now ready.
	return nil
}

func (r Reconciler) discoverRuntimeExtensions(ctx context.Context, extension *runtimev1.Extension) (*runtimev1.Extension, error) {
	var err error
	extension, err = r.RuntimeClient.Extension(extension).Discover(ctx)
	if err != nil {
		return nil, err
	}
	if err = validateRuntimeExtensionDiscovery(extension); err != nil {
		return nil, err
	}
	if err = r.RuntimeClient.Extension(extension).Register(); err != nil {
		return nil, errors.Wrap(err, "failed to register extension")
	}
	return extension, nil
}

// validateRuntimeExtensionDiscovery runs a set of validations on the data returned from an Extension's discovery call.
// if any of these checks fails the response is invalid and an error is returned. Extensions with previously valid
// RuntimeExtension registrations are not removed from the registry or the object's status.
func validateRuntimeExtensionDiscovery(ext *runtimev1.Extension) error {
	for _, runtimeExtension := range ext.Status.RuntimeExtensions {
		// simple dummy check to show how and where RuntimeExtension validation works.
		if len(runtimeExtension.Name) > 63 {
			return errors.New("RuntimeExtension name must be less than 64 characters")
		}
	}
	return nil
}
