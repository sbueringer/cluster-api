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

package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/controlplane/kubeadm/internal"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/internal/hooks"
	"sigs.k8s.io/cluster-api/internal/util/compare"
	patchutil "sigs.k8s.io/cluster-api/internal/util/patch"
	"sigs.k8s.io/cluster-api/internal/util/ssa"
)

func (r *KubeadmControlPlaneReconciler) tryInPlaceUpdate(
	ctx context.Context,
	controlPlane *internal.ControlPlane,
	machineToInPlaceUpdate *clusterv1.Machine,
	machineUpToDateResult internal.UpToDateResult,
) (fallbackToScaleDown bool, _ ctrl.Result, _ error) {
	if r.overrideTryInPlaceUpdateFunc != nil {
		return r.overrideTryInPlaceUpdateFunc(ctx, controlPlane, machineToInPlaceUpdate, machineUpToDateResult)
	}

	// Run preflight checks to ensure that the control plane is stable before proceeding with in-place update operation.
	if resultForAllMachines := r.preflightChecks(ctx, controlPlane); !resultForAllMachines.IsZero() {
		// We should not block a scale down of an unhealthy Machine that would work.
		if result := r.preflightChecks(ctx, controlPlane, machineToInPlaceUpdate); result.IsZero() {
			// Fallback to scale down.
			return true, ctrl.Result{}, nil
		}

		return false, resultForAllMachines, nil
	}

	// Machine might have been changed during the reconcile (e.g. through syncMachines) so we take the latest
	// version of the Machine here instead of storing it in UpToDate in a UpToDateResult.
	currentMachine := machineToInPlaceUpdate.DeepCopy()

	canUpdate, err := r.canUpdateMachine(ctx, currentMachine, machineUpToDateResult)
	if err != nil {
		return false, ctrl.Result{}, errors.Wrapf(err, "failed to determine if Machine can be updated in-place")
	}

	if !canUpdate {
		return true, ctrl.Result{}, nil
	}

	// Mark Machine for in-place update
	// Note: Once we reach this point and write MachineInPlaceUpdateInProgressAnnotation we will always continue with the in-place update.
	// Note: Intentionally using client.Patch instead of SSA. Otherwise we would have race-conditions when the Machine controller
	// tries to remove the annotation and KCP adds it back.
	if _, ok := currentMachine.Annotations[clusterv1.MachineInPlaceUpdateInProgressAnnotation]; !ok {
		orig := currentMachine.DeepCopy()
		currentMachine.Annotations[clusterv1.MachineInPlaceUpdateInProgressAnnotation] = ""
		if err := r.Client.Patch(ctx, currentMachine, client.MergeFrom(orig)); err != nil {
			return false, ctrl.Result{}, err
		}
		// Wait until the cache observed the change to ensure subsequent reconciles will observe it as well.
		if err := wait.PollUntilContextTimeout(ctx, 5*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
			m := &clusterv1.Machine{}
			if err := r.Client.Get(ctx, client.ObjectKeyFromObject(currentMachine), m); err != nil {
				return false, err
			}
			_, annotationSet := m.Annotations[clusterv1.MachineInPlaceUpdateInProgressAnnotation]
			return annotationSet, nil
		}); err != nil {
			return false, ctrl.Result{}, errors.Wrapf(err, "failed waiting for Machine %s to be updated in the cache after setting the %s annotation", klog.KObj(currentMachine), clusterv1.MachineInPlaceUpdateInProgressAnnotation)
		}
	}

	return false, ctrl.Result{}, r.completeTriggerInPlaceUpdate(ctx, machineUpToDateResult)
}

func (r *KubeadmControlPlaneReconciler) canUpdateMachine(ctx context.Context, currentMachine *clusterv1.Machine, machinesNeedingRolloutResult internal.UpToDateResult) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	// Compute current & desired including in-place updatable changes
	// All in-place updatable changes have been already applied in syncMachines.

	if !feature.Gates.Enabled(feature.InPlaceUpdates) {
		return false, nil
	}

	// Machine cannot be updated in-place if the UpToDate func was not able to provide all objects,
	// e.g. if the InfraMachine or KubeadmConfig was deleted.
	if machinesNeedingRolloutResult.CurrentInfraMachine == nil ||
		machinesNeedingRolloutResult.DesiredInfraMachine == nil ||
		machinesNeedingRolloutResult.CurrentKubeadmConfig == nil ||
		machinesNeedingRolloutResult.DesiredKubeadmConfig == nil {
		return false, nil
	}

	extensionHandlers, err := r.RuntimeClient.GetAllExtensions(ctx, runtimehooksv1.CanUpdateMachine, currentMachine)
	if err != nil {
		return false, err
	}
	if len(extensionHandlers) == 0 {
		// No CanUpdateMachine extensionHandlers registered.
		return false, nil
	}

	// Only ask until Machine is marked for in-place because once we start patching
	// KubeadmConfig / InfraMachine / Machine the diff won't be correct anymore.

	currentMachineForDiff, currentKubeadmConfigForDiff, currentInfraMachineForDiff,
		desiredMachineForDiff, desiredKubeadmConfigForDiff, desiredInfraMachineForDiff, err := r.prepareObjectsForDiff(ctx, currentMachine, machinesNeedingRolloutResult)
	if err != nil {
		return false, err
	}

	// Create request and response.
	req := &runtimehooksv1.CanUpdateMachineRequest{
		Current: runtimehooksv1.CanUpdateMachineRequestObjects{
			Machine:               *currentMachineForDiff,
			BootstrapConfig:       runtime.RawExtension{Object: currentKubeadmConfigForDiff},
			InfrastructureMachine: runtime.RawExtension{Object: currentInfraMachineForDiff},
		},
		Desired: runtimehooksv1.CanUpdateMachineRequestObjects{
			Machine:               *desiredMachineForDiff,
			BootstrapConfig:       runtime.RawExtension{Object: desiredKubeadmConfigForDiff},
			InfrastructureMachine: runtime.RawExtension{Object: desiredInfraMachineForDiff},
		},
	}

	log.V(5).Info("Calling CanUpdateMachine hooks",
		"machineDiff", diff(currentMachineForDiff, desiredMachineForDiff),
		"kubeadmConfigDiff", diff(currentKubeadmConfigForDiff, desiredKubeadmConfigForDiff),
		"infraMachineDiff", diff(currentInfraMachineForDiff, desiredInfraMachineForDiff),
	)

	for _, extensionHandler := range extensionHandlers {
		resp := &runtimehooksv1.CanUpdateMachineResponse{}
		if err := r.RuntimeClient.CallExtension(ctx, runtimehooksv1.CanUpdateMachine, currentMachine, extensionHandler, req, resp); err != nil {
			return false, errors.Wrap(err, "failed to call CanUpdateMachine")
		}

		// Apply patches to the request.
		if err := applyPatchesToRequest(ctx, req, resp); err != nil {
			return false, errors.Wrapf(err, "failed to apply patches from extension %s", extensionHandler)
		}
	}

	var reasons []string
	match, diff, err := diffMachine(&req.Current.Machine, desiredMachineForDiff)
	if err != nil {
		return false, errors.Wrapf(err, "failed to match Machine")
	}
	if !match {
		reasons = append(reasons, fmt.Sprintf("Machine cannot be updated in-place: %s", diff))
	}
	match, diff, err = diffKubeadmConfig(req.Current.BootstrapConfig, desiredKubeadmConfigForDiff)
	if err != nil {
		return false, errors.Wrapf(err, "failed to match KubeadmConfig")
	}
	if !match {
		reasons = append(reasons, fmt.Sprintf("KubeadmConfig cannot be updated in-place: %s", diff))
	}
	match, diff, err = diffUnstructured(req.Current.InfrastructureMachine, desiredInfraMachineForDiff)
	if err != nil {
		return false, errors.Wrapf(err, "failed to match InfraMachine")
	}
	if !match {
		reasons = append(reasons, fmt.Sprintf("InfraMachine cannot be updated in-place: %s", diff))
	}

	if len(reasons) > 0 {
		log.Info(fmt.Sprintf("Machine cannot be updated in-place, remaining diff that cannot be updated in-place: %s", strings.Join(reasons, ",")), "Machine", klog.KObj(currentMachine))
		return false, nil
	}

	return true, nil
}

func (r *KubeadmControlPlaneReconciler) completeTriggerInPlaceUpdate(ctx context.Context, machinesNeedingRolloutResult internal.UpToDateResult) error {
	// TODO: If this func fails "in the middle" we are going to reconcile again, if KCP changed in the mean time
	// desired objects might change and then we would use different desired objects for UpdateMachine compared to
	// what we used in CanUpdateMachine.
	// If we want to account for that we could consider writing desired InfraMachine/KubeadmConfig/Machine with the in-progress annotation
	// on the Machine and use it if necessary (and clean it up if necessary when we set the pending annotation).

	// Machine cannot be updated in-place if the UpToDate func was not able to provide all objects,
	// e.g. if the InfraMachine or KubeadmConfig was deleted.
	if machinesNeedingRolloutResult.DesiredInfraMachine == nil {
		return errors.Errorf("failed to complete triggering in-place update, could not compute desired InfraMachine")
	}
	if machinesNeedingRolloutResult.DesiredKubeadmConfig == nil {
		return errors.Errorf("failed to complete triggering in-place update, could not compute desired KubeadmConfig")
	}

	// Write InfraMachine without the labels & annotations that are written continuously by applyExternalObjectLabelsAnnotations.
	// Note: Let's update InfraMachine first because that is the call that is most likely to fail.
	machinesNeedingRolloutResult.DesiredInfraMachine.SetLabels(nil)
	machinesNeedingRolloutResult.DesiredInfraMachine.SetAnnotations(map[string]string{
		// ClonedFrom annotations are written by createInfraMachine so we have to send them here again to not unset them.
		// They also have to be updated here if the InfraMachineTemplate was rotated.
		clusterv1.TemplateClonedFromNameAnnotation:         machinesNeedingRolloutResult.DesiredInfraMachine.GetAnnotations()[clusterv1.TemplateClonedFromNameAnnotation],
		clusterv1.TemplateClonedFromGroupKindAnnotation:    machinesNeedingRolloutResult.DesiredInfraMachine.GetAnnotations()[clusterv1.TemplateClonedFromGroupKindAnnotation],
		clusterv1.MachineInPlaceUpdateInProgressAnnotation: "",
	})
	if err := ssa.Patch(ctx, r.Client, kcpManagerName2, machinesNeedingRolloutResult.DesiredInfraMachine); err != nil {
		return errors.Wrapf(err, "failed to apply %s", machinesNeedingRolloutResult.DesiredInfraMachine.GetKind())
	}

	// Write KubeadmConfig without labels & annotations.
	machinesNeedingRolloutResult.DesiredKubeadmConfig.Labels = nil
	machinesNeedingRolloutResult.DesiredKubeadmConfig.Annotations = map[string]string{
		clusterv1.MachineInPlaceUpdateInProgressAnnotation: "",
	}
	if err := ssa.Patch(ctx, r.Client, kcpManagerName2, machinesNeedingRolloutResult.DesiredKubeadmConfig); err != nil {
		return errors.Wrapf(err, "failed to apply KubeadmConfig")
	}

	if err := ssa.Patch(ctx, r.Client, kcpManagerName, machinesNeedingRolloutResult.DesiredMachine); err != nil {
		return errors.Wrap(err, "failed to apply Machine")
	}

	// Note: We set this annotation intentionally in a separate call with the "manager" fieldManager.
	// If we combine it with the SSA call above we would have to preserve it in ComputeDesiredMachine.
	// If we do that we can encounter race conditions where the Machine controller removes the pending annotation
	// and KCP that is running concurrently is re-adding it.
	if err := hooks.MarkAsPending(ctx, r.Client, machinesNeedingRolloutResult.DesiredMachine, runtimehooksv1.UpdateMachine); err != nil {
		return errors.Wrap(err, "failed to apply Machine: mark Machine as pending update")
	}
	// Wait until the cache observed the change to ensure subsequent reconciles will observe it as well.
	if err := wait.PollUntilContextTimeout(ctx, 5*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		m := &clusterv1.Machine{}
		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(machinesNeedingRolloutResult.DesiredMachine), m); err != nil {
			return false, err
		}
		return hooks.IsPending(runtimehooksv1.UpdateMachine, m), nil
	}); err != nil {
		return errors.Wrapf(err, "failed waiting for Machine %s to be updated in the cache after marking the UpdateMachine hook as pending", klog.KObj(machinesNeedingRolloutResult.DesiredInfraMachine))
	}

	return nil
}

func (r *KubeadmControlPlaneReconciler) prepareObjectsForDiff(ctx context.Context, currentMachine *clusterv1.Machine, machinesNeedingRolloutResult internal.UpToDateResult) (*clusterv1.Machine, *bootstrapv1.KubeadmConfig, *unstructured.Unstructured, *clusterv1.Machine, *bootstrapv1.KubeadmConfig, *unstructured.Unstructured, error) {
	currentMachineForDiff := currentMachine.DeepCopy()
	currentKubeadmConfigForDiff := machinesNeedingRolloutResult.CurrentKubeadmConfig.DeepCopy()
	currentInfraMachineForDiff := machinesNeedingRolloutResult.CurrentInfraMachine.DeepCopy()

	desiredMachineForDiff := machinesNeedingRolloutResult.DesiredMachine.DeepCopy()
	desiredKubeadmConfigForDiff := machinesNeedingRolloutResult.DesiredKubeadmConfig.DeepCopy()
	desiredInfraMachineForDiff := machinesNeedingRolloutResult.DesiredInfraMachine.DeepCopy()

	// Sync in-place updatable changes of current/desired KubeadmConfig/InfraMachine (handled by syncMachines)
	// (desired KubeadmConfig/InfraMachine already contain the latest labels & annotations)
	currentKubeadmConfigForDiff.SetLabels(desiredKubeadmConfigForDiff.GetLabels())
	currentKubeadmConfigForDiff.SetAnnotations(desiredKubeadmConfigForDiff.GetAnnotations())
	currentInfraMachineForDiff.SetLabels(desiredInfraMachineForDiff.GetLabels())
	currentInfraMachineForDiff.SetAnnotations(desiredInfraMachineForDiff.GetAnnotations())

	// Apply defaulting to current/desired KubeadmConfig / InfraMachine / Machine
	// This is necessary because otherwise we have unintended diffs like e.g.
	// dataSecretName, providerID and nodeDeletionTimeout on the Machine.

	// Machine
	// Note: We assume currentMachineForDiff doesn't need a dry-run as it is regularly written
	if err := ssa.Patch(ctx, r.Client, kcpManagerName, desiredMachineForDiff, ssa.WithDryRun{}); err != nil {
		return nil, nil, nil, nil, nil, nil, errors.Wrap(err, "server side apply dry-run failed for desired Machine")
	}

	// InfraMachine
	if err := ssa.Patch(ctx, r.Client, kcpManagerName2, currentInfraMachineForDiff, ssa.WithDryRun{}); err != nil {
		return nil, nil, nil, nil, nil, nil, errors.Wrap(err, "server side apply dry-run failed for current InfraMachine")
	}
	if err := ssa.Patch(ctx, r.Client, kcpManagerName2, desiredInfraMachineForDiff, ssa.WithDryRun{}); err != nil {
		// TODO: We need a contract similar to `topology.IsDryRunRequest` that InfraMachine webhooks should not
		// run immutability validation on dry-runs for in-place (similar for InfraMachineTemplate webhooks)
		return nil, nil, nil, nil, nil, nil, errors.Wrap(err, "server side apply dry-run failed for desired InfraMachine")
	}

	// KubeadmConfig
	// We don't dry-run here as PrepareKubeadmConfigsForDiff already has to perfectly handle differences between
	// current & desired KubeadmConfig. Otherwise the regular rollout logic would not detect correctly if a Machine
	// needs rollout.
	// Notes:
	// * KubeadmConfig doesn't have a defaulting webhook and no API defaulting anymore (pre-existing objects are handled correctly in PrepareKubeadmConfigsForDiff)
	//   * KubeadmConfigTemplate in the MD controller has different considerations as the defaulting is in the webhook
	desiredKubeadmConfigForDiff, currentKubeadmConfigForDiff = internal.PrepareKubeadmConfigsForDiff(desiredKubeadmConfigForDiff, currentKubeadmConfigForDiff, true)

	// Cleanup objects
	currentMachineForDiff = cleanupMachine(currentMachineForDiff)
	currentKubeadmConfigForDiff = cleanupKubeadmConfig(currentKubeadmConfigForDiff)
	currentInfraMachineForDiff = cleanupUnstructured(currentInfraMachineForDiff)

	desiredMachineForDiff = cleanupMachine(desiredMachineForDiff)
	desiredKubeadmConfigForDiff = cleanupKubeadmConfig(desiredKubeadmConfigForDiff)
	desiredInfraMachineForDiff = cleanupUnstructured(desiredInfraMachineForDiff)

	return currentMachineForDiff, currentKubeadmConfigForDiff, currentInfraMachineForDiff,
		desiredMachineForDiff, desiredKubeadmConfigForDiff, desiredInfraMachineForDiff, nil
}

func cleanupMachine(machine *clusterv1.Machine) *clusterv1.Machine {
	return &clusterv1.Machine{
		// Set GVK because object is later marshalled with json.Marshal when the hook request is sent.
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Machine",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        machine.Name,
			Namespace:   machine.Namespace,
			Labels:      machine.Labels,
			Annotations: machine.Annotations,
		},
		Spec: *machine.Spec.DeepCopy(),
	}
}

func cleanupKubeadmConfig(kubeadmConfig *bootstrapv1.KubeadmConfig) *bootstrapv1.KubeadmConfig {
	return &bootstrapv1.KubeadmConfig{
		// Set GVK because object is later marshalled with json.Marshal when the hook request is sent.
		TypeMeta: metav1.TypeMeta{
			APIVersion: bootstrapv1.GroupVersion.String(),
			Kind:       "KubeadmConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        kubeadmConfig.Name,
			Namespace:   kubeadmConfig.Namespace,
			Labels:      kubeadmConfig.Labels,
			Annotations: kubeadmConfig.Annotations,
		},
		Spec: *kubeadmConfig.Spec.DeepCopy(),
	}
}

func cleanupUnstructured(u *unstructured.Unstructured) *unstructured.Unstructured {
	cleanedUpU := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": u.GetAPIVersion(),
			"kind":       u.GetKind(),
			"spec":       u.Object["spec"],
		},
	}
	cleanedUpU.SetName(u.GetName())
	cleanedUpU.SetNamespace(u.GetNamespace())
	cleanedUpU.SetLabels(u.GetLabels())
	cleanedUpU.SetAnnotations(u.GetAnnotations())
	return cleanedUpU
}

func applyPatchesToRequest(ctx context.Context, req *runtimehooksv1.CanUpdateMachineRequest, resp *runtimehooksv1.CanUpdateMachineResponse) error {
	if err := applyPatchToMachine(ctx, &req.Current.Machine, resp.MachinePatch); err != nil {
		return err
	}

	if err := applyPatchToObject(ctx, &req.Current.BootstrapConfig, resp.BootstrapConfigPatch); err != nil {
		return err
	}

	if err := applyPatchToObject(ctx, &req.Current.InfrastructureMachine, resp.InfrastructureMachinePatch); err != nil {
		return err
	}

	return nil
}

func applyPatchToMachine(ctx context.Context, currentMachine *clusterv1.Machine, patch runtimehooksv1.Patch) error {
	currentMachineRaw := &runtime.RawExtension{
		Object: currentMachine,
	}
	if err := applyPatchToObject(ctx, currentMachineRaw, patch); err != nil {
		return err
	}
	m := &clusterv1.Machine{}

	// If applyPatchToObject is a no-op currentMachineRaw.Raw will stay unset.
	if currentMachineRaw.Raw == nil {
		return nil
	}

	if err := json.Unmarshal(currentMachineRaw.Raw, m); err != nil {
		return err
	}
	*currentMachine = *m
	return nil
}

func applyPatchToObject(ctx context.Context, obj *runtime.RawExtension, patch runtimehooksv1.Patch) (reterr error) {
	log := ctrl.LoggerFrom(ctx)

	defer func() {
		if r := recover(); r != nil {
			log.Info(fmt.Sprintf("Observed a panic when applying patch: %v\n%s", r, string(debug.Stack())))
			reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("observed a panic when applying patch: %v", r)})
		}
	}()

	patchedObject, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	switch patch.PatchType {
	case runtimehooksv1.JSONPatchType:
		log.V(5).Info("Accumulating JSON patch", "patch", string(patch.Patch))
		jsonPatch, err := jsonpatch.DecodePatch(patch.Patch)
		if err != nil {
			return errors.Wrap(err, "failed to apply patch: error decoding json patch (RFC6902)")
		}

		if len(jsonPatch) == 0 {
			// Return if there are no patches, nothing to do.
			// If the requestItem.Object does not have a spec and we don't have a patch that adds one,
			// patchTemplateSpec below would fail, so let's return early.
			return nil
		}

		patchedObject, err = jsonPatch.Apply(patchedObject)
		if err != nil {
			return errors.Wrap(err, "failed to apply patch: error applying json patch (RFC6902)")
		}
	case runtimehooksv1.JSONMergePatchType:
		if len(patch.Patch) == 0 || bytes.Equal(patch.Patch, []byte("{}")) {
			// Return if there are no patches, nothing to do.
			// If the requestItem.Object does not have a spec and we don't have a patch that adds one,
			// patchTemplateSpec below would fail, so let's return early.
			return nil
		}

		log.V(5).Info("Accumulating JSON merge patch", "patch", string(patch.Patch))
		patchedObject, err = jsonpatch.MergePatch(patchedObject, patch.Patch)
		if err != nil {
			return errors.Wrap(err, "failed to apply patch: error applying json merge patch (RFC7386)")
		}
	}

	// Overwrite the spec of obj with the spec of the patchedTemplate,
	// to ensure that we only pick up changes to the spec.
	if err := patchutil.PatchSpec(obj, patchedObject); err != nil {
		return errors.Wrap(err, "failed to apply patch to object")
	}

	return nil
}

func diffMachine(patched, desired *clusterv1.Machine) (equal bool, diff string, matchErr error) {
	patchedSpecOnly := &clusterv1.Machine{
		Spec: patched.Spec,
	}
	desiredSpecOnly := &clusterv1.Machine{
		Spec: desired.Spec,
	}
	return compare.Diff(patchedSpecOnly, desiredSpecOnly)
}

func diffKubeadmConfig(patched runtime.RawExtension, desired *bootstrapv1.KubeadmConfig) (equal bool, diff string, matchErr error) {
	patchedKubeadmConfig, ok := patched.Object.(*bootstrapv1.KubeadmConfig)
	if !ok {
		// Call MarshalJSON to handle the case where object.Raw is not set but object.Object is set.
		patchedBytes, err := patched.MarshalJSON()
		if err != nil {
			return false, "", err
		}
		patchedKubeadmConfig = &bootstrapv1.KubeadmConfig{}
		if err := json.Unmarshal(patchedBytes, patchedKubeadmConfig); err != nil {
			return false, "", err
		}
	}
	patchedSpecOnly := &bootstrapv1.KubeadmConfig{
		Spec: patchedKubeadmConfig.Spec,
	}
	desiredSpecOnly := &bootstrapv1.KubeadmConfig{
		Spec: desired.Spec,
	}
	return compare.Diff(patchedSpecOnly, desiredSpecOnly)
}

func diffUnstructured(patched runtime.RawExtension, desired *unstructured.Unstructured) (equal bool, diff string, matchErr error) {
	patchedUnstructured, ok := patched.Object.(*unstructured.Unstructured)
	if !ok {
		// Call MarshalJSON to handle the case where object.Raw is not set but object.Object is set.
		patchedBytes, err := patched.MarshalJSON()
		if err != nil {
			return false, "", err
		}
		patchedUnstructured = &unstructured.Unstructured{}
		if err := json.Unmarshal(patchedBytes, patchedUnstructured); err != nil {
			return false, "", err
		}
	}
	patchedSpecOnly := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": patchedUnstructured.Object["spec"],
		},
	}
	desiredSpecOnly := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": desired.Object["spec"],
		},
	}
	return compare.Diff(patchedSpecOnly, desiredSpecOnly)
}

func diff(current, desired any) string {
	_, d, err := compare.Diff(current, desired)
	if err != nil {
		return err.Error()
	}
	return d
}
