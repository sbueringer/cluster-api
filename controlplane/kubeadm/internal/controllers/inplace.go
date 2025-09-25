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

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
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
	"sigs.k8s.io/cluster-api/internal/util/ssa"
)

func (r *KubeadmControlPlaneReconciler) tryInPlaceUpdate(ctx context.Context, controlPlane *internal.ControlPlane, machineToInPlaceUpdate *clusterv1.Machine, machinesNeedingRolloutResult internal.NotUpToDateResult) (fallbackToScaleDown bool, _ ctrl.Result, _ error) {
	if r.overrideTryInPlaceFunc != nil {
		return r.overrideTryInPlaceFunc(ctx, controlPlane)
	}

	// Run preflight checks to ensure that the control plane is stable before proceeding with in-place update operation.
	if resultForAllMachines := r.preflightChecks(ctx, controlPlane); !resultForAllMachines.IsZero() {
		// FIXME(low-priority): figure out the details here:
		// * We also shouldn't block a scale down of unhealthy Machines that would otherwise work
		if result := r.preflightChecks(ctx, controlPlane, machineToInPlaceUpdate); result.IsZero() {
			// Fallback to scale down.
			return true, ctrl.Result{}, nil
		}

		return false, resultForAllMachines, nil
	}

	// Machine might have been changed during the reconcile, so we take the latest version of the Machine here
	// FIXME(low-priority): double check this is actually the latest version
	currentMachine := machineToInPlaceUpdate.DeepCopy()

	canUpdate, err := r.canUpdateMachine(ctx, currentMachine, machinesNeedingRolloutResult)
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
	// TODO: wait until cache is updated!!
	if _, ok := currentMachine.Annotations[clusterv1.MachineInPlaceUpdateInProgressAnnotation]; !ok {
		orig := currentMachine.DeepCopy()
		currentMachine.Annotations[clusterv1.MachineInPlaceUpdateInProgressAnnotation] = ""
		if err := r.Client.Patch(ctx, currentMachine, client.MergeFrom(orig)); err != nil {
			return false, ctrl.Result{}, err
		}
	}

	return false, ctrl.Result{}, r.completeTriggerInPlaceUpdate(ctx, machinesNeedingRolloutResult)
}

func (r *KubeadmControlPlaneReconciler) canUpdateMachine(ctx context.Context, currentMachine *clusterv1.Machine, machinesNeedingRolloutResult internal.NotUpToDateResult) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	// Compute current & desired including in-place updatable changes
	// All in-place updatable changes have been already applied in syncMachines.

	if !feature.Gates.Enabled(feature.InPlaceUpdates) {
		return false, nil
	}

	extensions, err := r.RuntimeClient.GetAllExtensions(ctx, runtimehooksv1.CanUpdateMachine, currentMachine)
	if err != nil {
		return false, err
	}
	if len(extensions) == 0 {
		// No CanUpdateMachine extensions registered.
		return false, nil
	}

	// Only ask until Machine is marked for in-place because once we start patching
	// KubeadmConfig / InfraMachine / Machine the diff won't be correct anymore.

	currentMachineForDiff, currentKubeadmConfigForDiff, currentInfraMachineForDiff,
		desiredMachineForDiff, desiredKubeadmConfigForDiff, desiredInfraMachineForDiff, err := r.prepareObjectsForDiff(ctx, currentMachine, machinesNeedingRolloutResult)
	if err != nil {
		return false, err
	}

	fmt.Println("CanUpdateMachine",
		"machineDiff", diff(currentMachineForDiff, desiredMachineForDiff),
		"kubeadmConfigDiff", diff(currentKubeadmConfigForDiff, desiredKubeadmConfigForDiff),
		"infraMachineDiff", diff(currentInfraMachineForDiff, desiredInfraMachineForDiff),
	)
	req := &runtimehooksv1.CanUpdateMachineRequest{
		Current: runtimehooksv1.UpdateMachineObjects{
			Machine:               *currentMachineForDiff,
			BootstrapConfig:       &runtime.RawExtension{Object: currentKubeadmConfigForDiff},
			InfrastructureMachine: runtime.RawExtension{Object: currentInfraMachineForDiff},
		},
		Desired: runtimehooksv1.UpdateMachineObjects{
			Machine:               *desiredMachineForDiff,
			BootstrapConfig:       &runtime.RawExtension{Object: desiredKubeadmConfigForDiff},
			InfrastructureMachine: runtime.RawExtension{Object: desiredInfraMachineForDiff},
		},
	}

	resp := &runtimehooksv1.CanUpdateMachineResponse{}
	// FIXME: Has to be called in a loop etc
	if err := r.RuntimeClient.CallExtension(ctx, runtimehooksv1.CanUpdateMachine, currentMachine, extensions[0], req, resp); err != nil {
		return false, errors.Wrap(err, "failed to call CanUpdateMachine")
	}

	patchedMachine, err := applyPatchToObject(ctx, currentMachineForDiff, resp.MachinePatch)
	if err != nil {
		return false, err
	}
	desiredMachineMarshalled, err := json.Marshal(desiredMachineForDiff)
	if err != nil {
		return false, err
	}
	patchedKubeadmConfig, err := applyPatchToObject(ctx, currentKubeadmConfigForDiff, resp.BootstrapConfigPatch) // run kubeadmconfig again through PrepareKubeadmConfigsForDiff before diff?
	if err != nil {
		return false, err
	}
	desiredKubeadmConfigMarshalled, err := json.Marshal(desiredKubeadmConfigForDiff)
	if err != nil {
		return false, err
	}
	patchedInfraMachine, err := applyPatchToObject(ctx, currentInfraMachineForDiff, resp.InfrastructureMachinePatch) // run kubeadmconfig again through PrepareKubeadmConfigsForDiff before diff?
	if err != nil {
		return false, err
	}
	desiredInfraMachineMarshalled, err := json.Marshal(desiredInfraMachineForDiff)
	if err != nil {
		return false, err
	}

	var reasons []string
	match, diff, err := diffObject(patchedMachine, desiredMachineMarshalled)
	if err != nil {
		return false, errors.Wrapf(err, "failed to match Machine")
	}
	if !match {
		reasons = append(reasons, fmt.Sprintf("Machine cannot be updated in-place: %s", diff))
	}
	match, diff, err = diffObject(patchedKubeadmConfig, desiredKubeadmConfigMarshalled)
	if err != nil {
		return false, errors.Wrapf(err, "failed to match KubeadmConfig")
	}
	if !match {
		reasons = append(reasons, fmt.Sprintf("KubeadmConfig cannot be updated in-place: %s", diff))
	}
	match, diff, err = diffObject(patchedInfraMachine, desiredInfraMachineMarshalled)
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

func (r *KubeadmControlPlaneReconciler) prepareObjectsForDiff(ctx context.Context, currentMachine *clusterv1.Machine, machinesNeedingRolloutResult internal.NotUpToDateResult) (*clusterv1.Machine, *bootstrapv1.KubeadmConfig, *unstructured.Unstructured, *clusterv1.Machine, *bootstrapv1.KubeadmConfig, *unstructured.Unstructured, error) {
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
	desiredKubeadmConfigForDiff, currentKubeadmConfigForDiff = internal.PrepareKubeadmConfigsForDiff(desiredKubeadmConfigForDiff, currentKubeadmConfigForDiff)
	// Set GVK because object is later marshalled with json.Marshal when CanUpdateMachine request is sent.
	currentKubeadmConfigForDiff.SetGroupVersionKind(bootstrapv1.GroupVersion.WithKind("KubeadmConfig"))
	desiredKubeadmConfigForDiff.SetGroupVersionKind(bootstrapv1.GroupVersion.WithKind("KubeadmConfig"))

	// Cleanup objects
	currentMachineForDiff = cleanupMachine(currentMachineForDiff)
	currentKubeadmConfigForDiff = cleanupKubeadmConfig(currentKubeadmConfigForDiff)
	currentInfraMachineForDiff = cleanupInfraMachine(currentInfraMachineForDiff)

	desiredMachineForDiff = cleanupMachine(desiredMachineForDiff)
	desiredKubeadmConfigForDiff = cleanupKubeadmConfig(desiredKubeadmConfigForDiff)
	desiredInfraMachineForDiff = cleanupInfraMachine(desiredInfraMachineForDiff)

	return currentMachineForDiff, currentKubeadmConfigForDiff, currentInfraMachineForDiff,
		desiredMachineForDiff, desiredKubeadmConfigForDiff, desiredInfraMachineForDiff, nil
}

func cleanupMachine(machine *clusterv1.Machine) *clusterv1.Machine {
	// Delete metadata apart from name, namespace, labels and annotations
	machine.SetUID("")
	machine.SetResourceVersion("")
	machine.SetGeneration(0)
	machine.SetGenerateName("")
	machine.SetCreationTimestamp(metav1.Time{})
	machine.SetDeletionTimestamp(nil)
	machine.SetDeletionGracePeriodSeconds(nil)
	machine.SetOwnerReferences(nil)
	machine.SetFinalizers(nil)
	machine.SetManagedFields(nil)
	machine.Status = clusterv1.MachineStatus{}
	return machine
}

func cleanupKubeadmConfig(kubeadmConfig *bootstrapv1.KubeadmConfig) *bootstrapv1.KubeadmConfig {
	// Delete metadata apart from name, namespace, labels and annotations
	kubeadmConfig.SetUID("")
	kubeadmConfig.SetResourceVersion("")
	kubeadmConfig.SetGeneration(0)
	kubeadmConfig.SetGenerateName("")
	kubeadmConfig.SetCreationTimestamp(metav1.Time{})
	kubeadmConfig.SetDeletionTimestamp(nil)
	kubeadmConfig.SetDeletionGracePeriodSeconds(nil)
	kubeadmConfig.SetOwnerReferences(nil)
	kubeadmConfig.SetFinalizers(nil)
	kubeadmConfig.SetManagedFields(nil)
	kubeadmConfig.Status = bootstrapv1.KubeadmConfigStatus{}
	return kubeadmConfig
}

func cleanupInfraMachine(u *unstructured.Unstructured) *unstructured.Unstructured {
	// Delete metadata apart from name, namespace, labels and annotations
	u.SetUID("")
	u.SetResourceVersion("")
	u.SetGeneration(0)
	u.SetGenerateName("")
	u.SetCreationTimestamp(metav1.Time{})
	u.SetDeletionTimestamp(nil)
	u.SetDeletionGracePeriodSeconds(nil)
	u.SetOwnerReferences(nil)
	u.SetFinalizers(nil)
	u.SetManagedFields(nil)
	delete(u.Object, "status")

	return u
}

func (r *KubeadmControlPlaneReconciler) completeTriggerInPlaceUpdate(ctx context.Context, machinesNeedingRolloutResult internal.NotUpToDateResult) error {
	// TODO: If this func fails "in the middle" we are going to reconcile again, if KCP changed in the mean time
	// desired objects might change and then we would use different desired objects for UpdateMachine compared to
	// what we used in CanUpdateMachine.
	// If we want to account for that we could consider writing desired InfraMachine/KubeadmConfig/Machine with the in-progress annotation
	// on the Machine and use it if necessary (and clean it up if necessary when we set the pending annotation).

	// BootstrapConfig intentionally before InfraMachine (because patching the BootstrapConfig doesn't trigger any "real" changes)
	// Note: if there are no changes some of these Patch calls won't do anything

	// FIXME(trigger): implement: fixup these applies until the changes are only as intended

	// Write InfraMachine without the labels & annotations that are written continuously by applyExternalObjectLabelsAnnotations.
	// Note: Let's update InfraMachine first because that is the call that is most likely to fail.
	// TODO: Find a better way to remove labels & annotations that are written continuously by applyExternalObjectLabelsAnnotations so we don't miss anything.
	machinesNeedingRolloutResult.DesiredInfraMachine.SetLabels(nil)
	machinesNeedingRolloutResult.DesiredInfraMachine.SetAnnotations(map[string]string{
		clusterv1.TemplateClonedFromNameAnnotation:         machinesNeedingRolloutResult.DesiredInfraMachine.GetAnnotations()[clusterv1.TemplateClonedFromNameAnnotation],
		clusterv1.TemplateClonedFromGroupKindAnnotation:    machinesNeedingRolloutResult.DesiredInfraMachine.GetAnnotations()[clusterv1.TemplateClonedFromGroupKindAnnotation],
		clusterv1.MachineInPlaceUpdateInProgressAnnotation: "",
	})
	if err := ssa.Patch(ctx, r.Client, kcpManagerName2, machinesNeedingRolloutResult.DesiredInfraMachine); err != nil {
		// FIXME(tilt) figure out why this call picks up ownership of finalizers, ownerReferences and some spec fields => for ownerRefs stop setting KCP ownerRef after Machine controller took over ownership (ensure clean hand off to Machine controller, same for KubeadmConfig)
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

	// FIXME(now): for now intentionally not using the SSA call above to make it easier to remove the annotation in the Machine controller.
	// !! If we don't merge this with above => only run above if necessary, but probably we should merge (also "manager Update" fieldManager is shared between KCP and Machine controller which is not great)
	// But probably fine to do it via SSA (just have to ensure that the annotation is preserved in ComputeDesiredMachine)
	// This requires more calls so we should probably prefer using SSA above (but we should use "Patch and wait")
	if err := hooks.MarkAsPending(ctx, r.Client, machinesNeedingRolloutResult.DesiredMachine, runtimehooksv1.UpdateMachine); err != nil {
		return errors.Wrap(err, "failed to apply Machine: mark Machine as pending update")
	}
	// FIXME: wait for cache

	return nil
}

func diffObject(patched, desired []byte) (equal bool, diff string, matchErr error) {
	// FIXME: optimize applyPatchToObject + diff below so we end up with better diffs (not just string diffs, probably should use Machine & KubeadmConfig where possible)
	// TODO: diff logic, compare: spec: yes, status: no, metadata? (probably no: labels & annotations are the same anyway)
	patchedUnstructured, err := bytesToUnstructured(patched)
	if err != nil {
		return false, "", err
	}
	desiredUnstructured, err := bytesToUnstructured(desired)
	if err != nil {
		return false, "", err
	}
	for k := range patchedUnstructured.Object {
		if k != "spec" {
			delete(patchedUnstructured.Object, k)
		}
	}
	for k := range desiredUnstructured.Object {
		if k != "spec" {
			delete(desiredUnstructured.Object, k)
		}
	}

	return compare.Diff(patchedUnstructured, desiredUnstructured)
}

// unstructuredDecoder is used to decode byte arrays into Unstructured objects.
var unstructuredDecoder = serializer.NewCodecFactory(nil).UniversalDeserializer()

// bytesToUnstructured provides a utility method that converts a (JSON) byte array into an Unstructured object.
func bytesToUnstructured(b []byte) (*unstructured.Unstructured, error) {
	// Unmarshal the JSON.
	u := &unstructured.Unstructured{}
	if _, _, err := unstructuredDecoder.Decode(b, nil, u); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal object from json")
	}

	return u, nil
}

func applyPatchToObject(ctx context.Context, obj client.Object, patch runtimehooksv1.Patch) (_ []byte, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	defer func() {
		if r := recover(); r != nil {
			log.Info(fmt.Sprintf("Observed a panic when applying patch: %v\n%s", r, string(debug.Stack())))
			reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("observed a panic when applying patch: %v", r)})
		}
	}()

	patchedObject, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	switch patch.PatchType {
	case runtimehooksv1.JSONPatchType:
		log.V(5).Info("Accumulating JSON patch", "patch", string(patch.Patch))
		jsonPatch, err := jsonpatch.DecodePatch(patch.Patch)
		if err != nil {
			return nil, errors.Wrap(err, "failed to apply patch: error decoding json patch (RFC6902)")
		}

		if len(jsonPatch) == 0 {
			// Return if there are no patches, nothing to do.
			// If the requestItem.Object does not have a spec and we don't have a patch that adds one,
			// patchTemplateSpec below would fail, so let's return early.
			return patchedObject, nil
		}

		patchedObject, err = jsonPatch.Apply(patchedObject)
		if err != nil {
			return nil, errors.Wrap(err, "failed to apply patch: error applying json patch (RFC6902)")
		}
	case runtimehooksv1.JSONMergePatchType:
		if len(patch.Patch) == 0 || bytes.Equal(patch.Patch, []byte("{}")) {
			// Return if there are no patches, nothing to do.
			// If the requestItem.Object does not have a spec and we don't have a patch that adds one,
			// patchTemplateSpec below would fail, so let's return early.
			return patchedObject, nil
		}

		log.V(5).Info("Accumulating JSON merge patch", "patch", string(patch.Patch))
		patchedObject, err = jsonpatch.MergePatch(patchedObject, patch.Patch)
		if err != nil {
			return nil, errors.Wrap(err, "failed to apply patch: error applying json merge patch (RFC7386)")
		}
	}

	// FIXME: should we ensure that we only patch spec?? (probably)

	return patchedObject, nil
}

func diff(current, desired any) string {
	_, d, err := compare.Diff(current, desired)
	if err != nil {
		return err.Error()
	}
	return d
}
