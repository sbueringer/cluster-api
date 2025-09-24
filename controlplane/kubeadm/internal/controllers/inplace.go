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
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/controlplane/kubeadm/internal"
	"sigs.k8s.io/cluster-api/controlplane/kubeadm/internal/desiredstate"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/internal/hooks"
	"sigs.k8s.io/cluster-api/internal/util/compare"
	"sigs.k8s.io/cluster-api/internal/util/ssa"
)

func (r *KubeadmControlPlaneReconciler) tryInPlaceUpdate(ctx context.Context, controlPlane *internal.ControlPlane, machineToInPlaceUpdate *clusterv1.Machine, machinesNeedingRolloutResult internal.NotUpToDateResult) (res ctrl.Result, err error) {
	if r.overrideTryInPlaceFunc != nil {
		return r.overrideTryInPlaceFunc(ctx, controlPlane)
	}

	log := ctrl.LoggerFrom(ctx)

	// Run preflight checks to ensure that the control plane is stable before proceeding with in-place update operation.
	if resultForAllMachines := r.preflightChecks(ctx, controlPlane); !resultForAllMachines.IsZero() {
		// FIXME(low-priority): figure out the details here:
		// * We also shouldn't block a scale down of unhealthy Machines that would otherwise work
		if result := r.preflightChecks(ctx, controlPlane, machineToInPlaceUpdate); result.IsZero() {
			// Fallback to scale down.
			return ctrl.Result{}, nil
		}

		return resultForAllMachines, err
	}

	// Compute current & desired including in-place updatable changes
	// All in-place updatable changes have been already applied in syncMachines.

	if !feature.Gates.Enabled(feature.InPlaceUpdates) {
		return ctrl.Result{}, nil
	}

	extensions, err := r.RuntimeClient.GetAllExtensions(ctx, runtimehooksv1.CanUpdateMachine, machineToInPlaceUpdate)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(extensions) == 0 {
		return ctrl.Result{}, nil
	}

	// Machine might have been changed during the reconcile, so we take the latest version of the Machine here
	// FIXME(low-priority): double check this is actually the latest version
	currentMachine := machineToInPlaceUpdate.DeepCopy()
	_, inPlaceInProgressAnnotationAlreadySet := currentMachine.Annotations[clusterv1.MachineInPlaceUpdateInProgressAnnotation]
	if !inPlaceInProgressAnnotationAlreadySet {
		if currentMachine.Annotations == nil {
			currentMachine.Annotations = map[string]string{}
		}
		// FIXME(trigger): ensure this is actually re-entrant, i.e. machine is part of machinesNeedingRollout with the right diff (? probably not needed, we shouldn't ask again once we have a candidate)
		// We have to ensure we hit this code path until the hook is marked as pending
		currentMachine.Annotations[clusterv1.MachineInPlaceUpdateInProgressAnnotation] = ""
	}

	// Compute the desiredMachine from the currentMachine here, because:
	// * spec.version is the only change that we are not rolling out in-place
	// * if we would have done this in matchesMachineSpec, currentMachine might have changed afterward
	// * if we compute the desiredMachine from scratch there could be differences like `controlplane.cluster.x-k8s.io/remediation-for`
	//   and we want to avoid that the RuntimeExtension has to account for differences like this.
	desiredMachine, err := desiredstate.ComputeDesiredMachine(controlPlane.KCP, controlPlane.Cluster, currentMachine.Spec.FailureDomain, currentMachine)
	if err != nil {
		return ctrl.Result{}, err
	}
	desiredMachineToSetInPlaceCandidateAnnotation := desiredMachine.DeepCopy()
	desiredMachine.Spec.Version = controlPlane.KCP.Spec.Version

	// CanUpdateMachine
	// Only ask until Machine is marked for in-place
	// Once we start patching KubeadmConfig / InfraMachine / Machine the diff won't be correct anymore.
	if !inPlaceInProgressAnnotationAlreadySet {
		// Sync in-place updatable changes of current/desired KubeadmConfig/InfraMachine
		// (desired KubeadmConfig/InfraMachine already contain the latest labels & annotations)
		machinesNeedingRolloutResult.CurrentKubeadmConfig.SetLabels(machinesNeedingRolloutResult.DesiredKubeadmConfig.GetLabels())
		machinesNeedingRolloutResult.CurrentKubeadmConfig.SetAnnotations(machinesNeedingRolloutResult.DesiredKubeadmConfig.GetAnnotations())
		machinesNeedingRolloutResult.CurrentInfraMachine.SetLabels(machinesNeedingRolloutResult.DesiredInfraMachine.GetLabels())
		machinesNeedingRolloutResult.CurrentInfraMachine.SetAnnotations(machinesNeedingRolloutResult.DesiredInfraMachine.GetAnnotations())

		// Apply defaulting to current/desired KubeadmConfig / InfraMachine / Machine (otherwise diff like dataSecretName + providerID + timeouts)
		// Note: This is also used to verify that we can apply the desired KubeadmConfig/InfraMachine later.
		// FIXME(trigger): implement
		// * figure out how exactly we are going to apply them later and do the same here, e.g. SSA (decide if the default results here should only be used for diff, but probably)
		// * What if we don't do this and leave this to the RuntimeExtension? (also consider the situation in MS), e.g.:
		//   * KubeadmConfig doesn't have a defaulting webhook and no API defaulting anymore (it's handled in PrepareKubeadmConfigsForDiff)
		//   * KubeadmConfigTemplate has a defaulting webhook that runs ApplyPreviousKubeadmConfigDefaults for topology dry-run requests
		//   * => Difference for KubeadmConfig is what controllers added later (so I guess we need it to so folks can diff current vs current-after-desired-is-applied and not current vs desired)
		//     * => check init vs joinconfiguration stuff
		// FIXME(trigger): See if we can use InfraMachine response as a signal of "can we actually InPlace update this InfraMachine later" (see if we can differentiate e.g. network issues form actual validation errors like immutability checks) => exponential back off on error

		desiredKubeadmConfigForDiff, currentKubeadmConfigForDiff := internal.PrepareKubeadmConfigsForDiff(machinesNeedingRolloutResult.DesiredKubeadmConfig.DeepCopy(), machinesNeedingRolloutResult.CurrentKubeadmConfig)
		fmt.Println("CanUpdateMachine", // FIXME(low-priority) cleanup status before "sending" (+ maybe more, e.g. managedFields, maybe keep only label & annotations metadata fields, check what we do for cluster in lifecycle hooks)
			"machineDiff", diff(currentMachine.Spec, desiredMachine.Spec),
			"kubeadmConfigDiff", diff(currentKubeadmConfigForDiff.Spec, desiredKubeadmConfigForDiff.Spec),
			"infraMachineDiff", diff(machinesNeedingRolloutResult.CurrentInfraMachine.Object["spec"], machinesNeedingRolloutResult.DesiredInfraMachine.Object["spec"]),
		)

		req := &runtimehooksv1.CanUpdateMachineRequest{
			Current:       runtimehooksv1.UpdateMachineObjects{
				Machine:               *currentMachine,
				BootstrapConfig:       &runtime.RawExtension{Object: currentKubeadmConfigForDiff},
				InfrastructureMachine: runtime.RawExtension{Object: machinesNeedingRolloutResult.CurrentInfraMachine},
			},
			Desired:       runtimehooksv1.UpdateMachineObjects{
				Machine: *desiredMachine,
				BootstrapConfig:       &runtime.RawExtension{Object: desiredKubeadmConfigForDiff},
				InfrastructureMachine: runtime.RawExtension{Object: machinesNeedingRolloutResult.DesiredInfraMachine},
			},
		}
		resp := &runtimehooksv1.CanUpdateMachineResponse{}
		// TODO: Has to be called in a loop etc
		// TODO: should the RX call use caching? Probably not, we're not calling it that often
		if err := r.RuntimeClient.CallExtension(ctx, runtimehooksv1.CanUpdateMachine, machineToInPlaceUpdate, extensions[0], req, resp); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to call CanUpdateMachine")
		}

		patchedMachine, err := applyPatchToObject(ctx, currentMachine, resp.MachinePatch)
		if err != nil {
			return ctrl.Result{}, err
		}
		desiredMachineMarshalled, err := json.Marshal(desiredMachine)
		if err != nil {
			return ctrl.Result{}, err
		}
		patchedKubeadmConfig, err := applyPatchToObject(ctx, currentKubeadmConfigForDiff, resp.MachinePatch) // run kubeadmconfig again through PrepareKubeadmConfigsForDiff before diff?
		if err != nil {
			return ctrl.Result{}, err
		}
		desiredKubeadmConfigMarshalled, err := json.Marshal(desiredKubeadmConfigForDiff)
		if err != nil {
			return ctrl.Result{}, err
		}
		patchedInfraMachine, err := applyPatchToObject(ctx, machinesNeedingRolloutResult.CurrentInfraMachine, resp.MachinePatch) // run kubeadmconfig again through PrepareKubeadmConfigsForDiff before diff?
		if err != nil {
			return ctrl.Result{}, err
		}
		desiredInfraMachineMarshalled, err := json.Marshal(machinesNeedingRolloutResult.DesiredInfraMachine)
		if err != nil {
			return ctrl.Result{}, err
		}
		var reasons []string
		match, _, err := compare.Diff(patchedMachine, desiredMachineMarshalled)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to match Machine")
		}
		if !match {
			reasons = append(reasons, "Machine cannot be updated in-place")
		}
		match, _, err = compare.Diff(patchedKubeadmConfig, desiredKubeadmConfigMarshalled)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to match KubeadmConfig")
		}
		if !match {
			reasons = append(reasons, "KubeadmConfig cannot be updated in-place")
		}
		match, _, err = compare.Diff(patchedInfraMachine, desiredInfraMachineMarshalled)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to match InfraMachine")
		}
		if !match {
			reasons = append(reasons, "InfraMachine cannot be updated in-place")
		}

		if len(reasons) > 0 {
			log.Info(fmt.Sprintf("Machine cannot be updated in-place, reasons: %s", strings.Join(reasons, ","), "Machine", klog.KObj(currentMachine)))
			return ctrl.Result{}, nil
		}

		// TODO: diff logic, compare: spec: yes, status: no, metadata? (probably no: labels & annotations are the same anyway)

		// Update Machine
		// Note: Once we reach this point and write MachineInPlaceUpdateInProgressAnnotation we will always continue with the in-place update.
		if err := ssa.Patch(ctx, r.Client, kcpManagerName, desiredMachineToSetInPlaceCandidateAnnotation, ssa.WithCachingProxy{Cache: r.ssaCache, Original: desiredMachineToSetInPlaceCandidateAnnotation}); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to apply Machine: failed to set %s annotation", clusterv1.MachineInPlaceUpdateInProgressAnnotation)
		}
	}

	// FIXME(Trigger) call this stuff from top-level and simplify accordingly (before the remediation func)

	// BootstrapConfig intentionally before InfraMachine (because patching the BootstrapConfig doesn't trigger any "real" changes)
	// Note: if there are no changes some of these Patch calls won't do anything

	// Write KubeadmConfig without labels & annotations.
	machinesNeedingRolloutResult.DesiredKubeadmConfig.Labels = nil
	machinesNeedingRolloutResult.DesiredKubeadmConfig.Annotations = map[string]string{
		clusterv1.MachineInPlaceUpdateInProgressAnnotation: "",
	}
	// FIXME(low-priority): figure out if the cache still works
	if err := ssa.Patch(ctx, r.Client, kcpManagerName2, machinesNeedingRolloutResult.DesiredKubeadmConfig, ssa.WithCachingProxy{Cache: r.ssaCache, Original: machinesNeedingRolloutResult.DesiredKubeadmConfig}); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to apply KubeadmConfig")
	}

	// Write InfraMachine without the labels & annotations that are written continuously by applyExternalObjectLabelsAnnotations.
	// TODO: Find a better way to remove labels & annotations that are written continuously by applyExternalObjectLabelsAnnotations so we don't miss anything.
	machinesNeedingRolloutResult.DesiredInfraMachine.SetLabels(nil)
	machinesNeedingRolloutResult.DesiredInfraMachine.SetAnnotations(map[string]string{
		clusterv1.TemplateClonedFromNameAnnotation:         machinesNeedingRolloutResult.DesiredInfraMachine.GetAnnotations()[clusterv1.TemplateClonedFromNameAnnotation],
		clusterv1.TemplateClonedFromGroupKindAnnotation:    machinesNeedingRolloutResult.DesiredInfraMachine.GetAnnotations()[clusterv1.TemplateClonedFromGroupKindAnnotation],
		clusterv1.MachineInPlaceUpdateInProgressAnnotation: "",
	})
	// FIXME(low-priority): figure out if the cache still works
	if err := ssa.Patch(ctx, r.Client, kcpManagerName2, machinesNeedingRolloutResult.DesiredInfraMachine, ssa.WithCachingProxy{Cache: r.ssaCache, Original: machinesNeedingRolloutResult.DesiredInfraMachine}); err != nil { // FIXME(tilt) figure out why this call picks up ownership of finalizers, ownerReferences and some spec fields => for ownerRefs stop setting KCP ownerRef after Machine controller took over ownership (ensure clean hand off to Machine controller, same for KubeadmConfig)
		return ctrl.Result{}, errors.Wrapf(err, "failed to apply %s", machinesNeedingRolloutResult.DesiredInfraMachine.GetKind())
	}

	if err := ssa.Patch(ctx, r.Client, kcpManagerName, desiredMachine, ssa.WithCachingProxy{Cache: r.ssaCache, Original: desiredMachine}); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to apply Machine")
	}

	// FIXME(low-priority): for now intentionally not using the SSA call above to make it easier to remove the annotation in the Machine controller.
	// !! If we don't merge this with above => only run above if necessary, but probably we should merge (also "manager Update" fieldManager is shared between KCP and Machine controller which is not great)
	// But probably fine to do it via SSA (just have to ensure that the annotation is preserved in ComputeDesiredMachine)
	// This requires more calls so we should probably prefer using SSA above (but we should use "Patch and wait")
	if err := hooks.MarkAsPending(ctx, r.Client, desiredMachine, runtimehooksv1.UpdateMachine); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to apply Machine: mark Machine as pending update")
	}

	return ctrl.Result{Requeue: true}, nil
}

func applyPatchToObject(ctx context.Context, obj client.Object, patch runtimehooksv1.Patch) (object []byte, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	defer func() {
		if r := recover(); r != nil {
			log.Info(fmt.Sprintf("Observed a panic when applying patch: %v\n%s", r, string(debug.Stack())))
			reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("observed a panic when applying patch: %v", r)})
		}
	}()

	patchedObject, err := json.Marshal(obj)
	if reterr != nil {
		return object, err
	}

	switch patch.PatchType {
	case runtimehooksv1.JSONPatchType:
		log.V(5).Info("Accumulating JSON patch", "patch", string(patch.Patch))
		jsonPatch, err := jsonpatch.DecodePatch(patch.Patch)
		if err != nil {
			return object, errors.Wrap(err, "failed to apply patch: error decoding json patch (RFC6902)")
		}

		if len(jsonPatch) == 0 {
			// Return if there are no patches, nothing to do.
			// If the requestItem.Object does not have a spec and we don't have a patch that adds one,
			// patchTemplateSpec below would fail, so let's return early.
			return object, nil
		}

		patchedObject, err = jsonPatch.Apply(patchedObject)
		if err != nil {
			return object, errors.Wrap(err, "failed to apply patch: error applying json patch (RFC6902)")
		}
	case runtimehooksv1.JSONMergePatchType:
		if len(patch.Patch) == 0 || bytes.Equal(patch.Patch, []byte("{}")) {
			// Return if there are no patches, nothing to do.
			// If the requestItem.Object does not have a spec and we don't have a patch that adds one,
			// patchTemplateSpec below would fail, so let's return early.
			return object, nil
		}

		log.V(5).Info("Accumulating JSON merge patch", "patch", string(patch.Patch))
		patchedObject, err = jsonpatch.MergePatch(patchedObject, patch.Patch)
		if err != nil {
			return object, errors.Wrap(err, "failed to apply patch: error applying json merge patch (RFC7386)")
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
