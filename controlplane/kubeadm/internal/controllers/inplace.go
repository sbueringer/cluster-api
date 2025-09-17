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
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controlplane/kubeadm/internal"
	"sigs.k8s.io/cluster-api/internal/util/compare"
)

func (r *KubeadmControlPlaneReconciler) tryInPlaceUpdate(ctx context.Context, controlPlane *internal.ControlPlane, machineToInPlaceUpdate *clusterv1.Machine, upToDateResult internal.UpToDateResult) (res ctrl.Result, err error) {
	if r.overrideTryInPlaceFunc != nil {
		return r.overrideTryInPlaceFunc(ctx, controlPlane)
	}
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

	// Machine might have been changed during the reconcile, so we take the latest version of the Machine here
	// FIXME(low-priority): double check this is actually the latest version
	currentMachine := machineToInPlaceUpdate
	// Compute the desiredMachine from the currentMachine here, because:
	// * spec.version is the only change that we are not rolling out in-place
	// * if we would have done this in matchesMachineSpec, currentMachine might have changed afterward
	// * if we compute the desiredMachine from scratch there could be differences like `controlplane.cluster.x-k8s.io/remediation-for`
	//   and we want to avoid that the RuntimeExtension has to account for differences like this.
	desiredMachine := currentMachine.DeepCopy()
	desiredMachine.Spec.Version = controlPlane.KCP.Spec.Version
	// Sync in-place updatable changes of current/desired KubeadmConfig/InfraMachine
	// (desired KubeadmConfig/InfraMachine already contain the latest labels & annotations) // FIXME apply in place func
	upToDateResult.CurrentKubeadmConfig.SetLabels(upToDateResult.DesiredKubeadmConfig.GetLabels())
	upToDateResult.CurrentKubeadmConfig.SetAnnotations(upToDateResult.DesiredKubeadmConfig.GetAnnotations())
	upToDateResult.CurrentInfraMachine.SetLabels(upToDateResult.DesiredInfraMachine.GetLabels())
	upToDateResult.CurrentInfraMachine.SetAnnotations(upToDateResult.DesiredInfraMachine.GetAnnotations())

	// Apply defaulting to current/desired KubeadmConfig / InfraMachine
	// Note: This is also used to verify that we can apply the desired KubeadmConfig/InfraMachine later.
	// FIXME: implement (TODO: figure out how exactly we are going to apply them later and do the same here, e.g. SSA) // FIXME(trigger)

	// CanUpdateMachine
	desiredKubeadmConfigForDiff, currentKubeadmConfigForDiff := internal.PrepareKubeadmConfigsForDiff(upToDateResult.DesiredKubeadmConfig.DeepCopy(), upToDateResult.CurrentKubeadmConfig)
	fmt.Println("CanUpdateMachine", // FIXME(low-priority) cleanup status before "sending"
		"machineDiff", diff(currentMachine, desiredMachine),
		"kubeadmConfigDiff", diff(currentKubeadmConfigForDiff.Spec, desiredKubeadmConfigForDiff.Spec),
		"infraMachineDiff", diff(upToDateResult.CurrentInfraMachine.Object["spec"], upToDateResult.DesiredInfraMachine.Object["spec"]),
	)

	// TODO: diff: spec: yes, status: no, metadata?

	// Update Machine
	fmt.Println("Update Machine",
		currentMachine,
		desiredMachine,
		upToDateResult.CurrentKubeadmConfig,
		upToDateResult.DesiredKubeadmConfig,
		upToDateResult.CurrentInfraMachine,
		upToDateResult.DesiredInfraMachine,
	)

	return ctrl.Result{}, nil
}

func diff(current, desired any) string {
	_, d, err := compare.Diff(current, desired)
	if err != nil {
		return err.Error()
	}
	return d
}
