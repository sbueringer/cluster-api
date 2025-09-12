/*
Copyright 2020 The Kubernetes Authors.

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

	"github.com/blang/semver/v4"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	"sigs.k8s.io/cluster-api/controlplane/kubeadm/internal"
	"sigs.k8s.io/cluster-api/util/collections"
)

func (r *KubeadmControlPlaneReconciler) upgradeControlPlane(
	ctx context.Context,
	controlPlane *internal.ControlPlane,
	machinesRequireUpgrade collections.Machines,
) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// TODO: handle reconciliation of etcd members and kubeadm config in case they get out of sync with cluster

	workloadCluster, err := controlPlane.GetWorkloadCluster(ctx)
	if err != nil {
		log.Error(err, "failed to get remote client for workload cluster", "Cluster", klog.KObj(controlPlane.Cluster))
		return ctrl.Result{}, err
	}

	parsedVersion, err := semver.ParseTolerant(controlPlane.KCP.Spec.Version)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to parse kubernetes version %q", controlPlane.KCP.Spec.Version)
	}

	// Ensure kubeadm clusterRoleBinding for v1.29+ as per https://github.com/kubernetes/kubernetes/pull/121305
	if err := workloadCluster.AllowClusterAdminPermissions(ctx, parsedVersion); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to set cluster-admin ClusterRoleBinding for kubeadm")
	}

	kubeadmCMMutators := make([]func(*bootstrapv1.ClusterConfiguration), 0)

	if controlPlane.KCP.Spec.KubeadmConfigSpec.ClusterConfiguration.IsDefined() {
		// Get the imageRepository or the correct value if nothing is set and a migration is necessary.
		imageRepository := internal.ImageRepositoryFromClusterConfig(controlPlane.KCP.Spec.KubeadmConfigSpec.ClusterConfiguration)

		kubeadmCMMutators = append(kubeadmCMMutators,
			workloadCluster.UpdateImageRepositoryInKubeadmConfigMap(imageRepository),
			workloadCluster.UpdateFeatureGatesInKubeadmConfigMap(controlPlane.KCP.Spec.KubeadmConfigSpec, parsedVersion),
			workloadCluster.UpdateAPIServerInKubeadmConfigMap(controlPlane.KCP.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer),
			workloadCluster.UpdateControllerManagerInKubeadmConfigMap(controlPlane.KCP.Spec.KubeadmConfigSpec.ClusterConfiguration.ControllerManager),
			workloadCluster.UpdateSchedulerInKubeadmConfigMap(controlPlane.KCP.Spec.KubeadmConfigSpec.ClusterConfiguration.Scheduler),
			workloadCluster.UpdateCertificateValidityPeriodDays(controlPlane.KCP.Spec.KubeadmConfigSpec.ClusterConfiguration.CertificateValidityPeriodDays))

		// Etcd local and external are mutually exclusive and they cannot be switched, once set.
		if controlPlane.IsEtcdManaged() {
			kubeadmCMMutators = append(kubeadmCMMutators,
				workloadCluster.UpdateEtcdLocalInKubeadmConfigMap(controlPlane.KCP.Spec.KubeadmConfigSpec.ClusterConfiguration.Etcd.Local))
		} else {
			kubeadmCMMutators = append(kubeadmCMMutators,
				workloadCluster.UpdateEtcdExternalInKubeadmConfigMap(controlPlane.KCP.Spec.KubeadmConfigSpec.ClusterConfiguration.Etcd.External))
		}
	}

	// collectively update Kubeadm config map
	if err = workloadCluster.UpdateClusterConfiguration(ctx, parsedVersion, kubeadmCMMutators...); err != nil {
		return ctrl.Result{}, err
	}

	switch controlPlane.KCP.Spec.Rollout.Strategy.Type {
	case controlplanev1.RollingUpdateStrategyType:
		// RolloutStrategy is currently defaulted and validated to always be RollingUpdate.
		return r.rollingUpdate(ctx, controlPlane, machinesRequireUpgrade)
	default:
		log.Info("RolloutStrategy type is not set to RollingUpdate, unable to determine the strategy for rolling out machines")
		return ctrl.Result{}, nil
	}
}

func (r *KubeadmControlPlaneReconciler) rollingUpdate(ctx context.Context, controlPlane *internal.ControlPlane, machinesNeedingRollout collections.Machines) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// FIXME(in-place): if in place in progress => return (ensure that while an in-place update is ongoing we wait for it to complete)
	// Alternative: modify preflightChecks to add controlPlane.HasInPlaceUpdatingMachine

	maxSurge := int32(controlPlane.KCP.Spec.Rollout.Strategy.RollingUpdate.MaxSurge.IntValue())
	var maxUnavailable int32 // maxUnavailable is a bit too confusing here as KCP doesn't have the same concept here as MD
	switch {
	case maxSurge == 0:
		maxUnavailable = 1
	case maxSurge == 1:
		maxUnavailable = 0
	}

	// We don't have to consider Available replicas because we are enforcing health checks before we get here.
	currentReplicas := int32(controlPlane.Machines.Len())
	currentUpToDateReplicas := int32(len(controlPlane.UpToDateMachines()))
	desiredReplicas := *controlPlane.KCP.Spec.Replicas
	desiredMaxReplicas := desiredReplicas + maxSurge
	desiredMinReplicas := desiredReplicas - maxUnavailable

	// Depending on maxSurge/maxUnavailable the [desiredMinReplicas, desiredMaxReplicas] interval will be:
	// * maxSurge: 1 (maxUnavailable: 0) =>     [desiredReplicas   , desiredReplicas+1]
	// * maxSurge: 0 (maxUnavailable: 1) =>     [desiredReplicas-1 , desiredReplicas  ]
	switch {
	case desiredMinReplicas < currentReplicas:
		// Pick the Machine that we should in-place update or scale down.
		machineToInPlaceUpdateOrScaleDown, err := selectMachineForInPlaceUpdateOrScaleDown(ctx, controlPlane, machinesNeedingRollout)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to select machine for scale down")
		}

		if currentUpToDateReplicas < desiredReplicas {
			// If we need more upToDate replicas, try in-place
			res, err := r.tryInPlaceUpdate(ctx, controlPlane, machineToInPlaceUpdateOrScaleDown)
			if err != nil {
				// If error => return
				return ctrl.Result{}, err
			}
			if !res.IsZero() {
				// If preflightChecks error or in-place update triggered / in-progress
				return res, nil
			}
			// Otherwise => scaleDown
			// * preflightChecks error only on machineToInPlaceUpdateOrScaleDown
			// * CanUpdateMachine == false
		}
		return r.scaleDownControlPlane(ctx, controlPlane, machineToInPlaceUpdateOrScaleDown)
	case currentReplicas < desiredMaxReplicas:
		// scaleUp ensures that we don't continue scale up while waiting for Machines to have NodeRefs
		return r.scaleUpControlPlane(ctx, controlPlane)
	default:
		log.Info("FIXME: should never happen")
		return ctrl.Result{}, nil
	}
}
