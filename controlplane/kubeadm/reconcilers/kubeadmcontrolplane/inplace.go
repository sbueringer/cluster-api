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

package kubeadmcontrolplane

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controlplane/kubeadm/pkg"
)

func (r *Reconciler) tryInPlaceUpdate(
	ctx context.Context,
	controlPlane *pkg.ControlPlane,
	machineToInPlaceUpdate *clusterv1.Machine,
	machineUpToDateResult pkg.UpToDateResult,
) (onlyMachineToInPlaceUpdateFailsPreflightChecks bool, _ ctrl.Result, _ error) {
	if r.overrideTryInPlaceUpdateFunc != nil {
		return r.overrideTryInPlaceUpdateFunc(ctx, controlPlane, machineToInPlaceUpdate, machineUpToDateResult)
	}

	// Run preflight checks to ensure that the control plane is stable before proceeding with in-place update operation.
	//
	// Important! preflight checks play an important role in ensuring that KCP performs "one operation at time", by forcing
	// the system to wait for the previous operation to complete and the control plane to become stable before starting the next one.
	//
	// Note: before considering in-place updates, KCP first takes care of completing
	// ongoing delete operations, completing in-place transitions, remediating unhealthy machines.
	if resultForAllMachines := r.preflightChecks(ctx, controlPlane, false); !resultForAllMachines.IsZero() {
		// If the control plane is not stable, check if the issues are only for machineToInPlaceUpdate.
		if result := r.preflightChecks(ctx, controlPlane, false, machineToInPlaceUpdate); result.IsZero() {
			// The issues are only for machineToInPlaceUpdate, fallback to scale down.
			// Note: The consequence of this is that a Machine with issues is scaled down and not in-place updated.
			return true, ctrl.Result{}, nil
		}

		return false, resultForAllMachines, nil
	}

	return false, ctrl.Result{}, r.triggerInPlaceUpdate(ctx, controlPlane, machineToInPlaceUpdate, machineUpToDateResult)
}
