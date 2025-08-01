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

// Package desiredstate contains cluster topology utils, e.g. to compute the desired state.
package desiredstate

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	"sigs.k8s.io/cluster-api/controllers/external"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimeclient "sigs.k8s.io/cluster-api/exp/runtime/client"
	"sigs.k8s.io/cluster-api/exp/topology/scope"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/internal/contract"
	"sigs.k8s.io/cluster-api/internal/controllers/topology/cluster/patches"
	"sigs.k8s.io/cluster-api/internal/hooks"
	"sigs.k8s.io/cluster-api/internal/topology/clustershim"
	topologynames "sigs.k8s.io/cluster-api/internal/topology/names"
	"sigs.k8s.io/cluster-api/internal/topology/ownerrefs"
	"sigs.k8s.io/cluster-api/internal/topology/selectors"
	"sigs.k8s.io/cluster-api/internal/webhooks"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conversion"
)

// Generator is a generator to generate the desired state.
type Generator interface {
	Generate(ctx context.Context, s *scope.Scope) (*scope.ClusterState, error)
}

// NewGenerator creates a new generator to generate desired state.
func NewGenerator(client client.Client, clusterCache clustercache.ClusterCache, runtimeClient runtimeclient.Client) Generator {
	return &generator{
		Client:        client,
		ClusterCache:  clusterCache,
		RuntimeClient: runtimeClient,
		patchEngine:   patches.NewEngine(runtimeClient),
	}
}

// generator is a generator to generate desired state.
// It is used in the cluster topology controller, but it can also be used for testing.
type generator struct {
	Client client.Client

	ClusterCache clustercache.ClusterCache

	RuntimeClient runtimeclient.Client

	// patchEngine is used to apply patches during computeDesiredState.
	patchEngine patches.Engine
}

// Generate computes the desired state of the cluster topology.
// NOTE: We are assuming all the required objects are provided as input; also, in case of any error,
// the entire compute operation will fail. This might be improved in the future if support for reconciling
// subset of a topology will be implemented.
func (g *generator) Generate(ctx context.Context, s *scope.Scope) (*scope.ClusterState, error) {
	var err error
	desiredState := &scope.ClusterState{
		ControlPlane: &scope.ControlPlaneState{},
	}

	// Compute the desired state of the InfrastructureCluster object.
	if desiredState.InfrastructureCluster, err = computeInfrastructureCluster(ctx, s); err != nil {
		return nil, errors.Wrapf(err, "failed to compute InfrastructureCluster")
	}

	// If the clusterClass mandates the controlPlane has infrastructureMachines, compute the InfrastructureMachineTemplate for the ControlPlane.
	if s.Blueprint.HasControlPlaneInfrastructureMachine() {
		if desiredState.ControlPlane.InfrastructureMachineTemplate, err = g.computeControlPlaneInfrastructureMachineTemplate(ctx, s); err != nil {
			return nil, errors.Wrapf(err, "failed to compute ControlPlane InfrastructureMachineTemplate")
		}
	}

	// Mark all the MachineDeployments that are currently upgrading.
	// This captured information is used for:
	// - Building the TopologyReconciled condition.
	// - Make upgrade decisions on the control plane.
	// - Making upgrade decisions on machine deployments.
	mdUpgradingNames, err := s.Current.MachineDeployments.Upgrading(ctx, g.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if any MachineDeployment is upgrading")
	}
	s.UpgradeTracker.MachineDeployments.MarkUpgrading(mdUpgradingNames...)

	// Mark all the MachinePools that are currently upgrading.
	// This captured information is used for:
	// - Building the TopologyReconciled condition.
	// - Make upgrade decisions on the control plane.
	// - Making upgrade decisions on machine pools.
	if len(s.Current.MachinePools) > 0 {
		client, err := g.ClusterCache.GetClient(ctx, client.ObjectKeyFromObject(s.Current.Cluster))
		if err != nil {
			return nil, errors.Wrap(err, "failed to check if any MachinePool is upgrading")
		}
		// Mark all the MachinePools that are currently upgrading.
		mpUpgradingNames, err := s.Current.MachinePools.Upgrading(ctx, client)
		if err != nil {
			return nil, errors.Wrap(err, "failed to check if any MachinePool is upgrading")
		}
		s.UpgradeTracker.MachinePools.MarkUpgrading(mpUpgradingNames...)
	}

	// Compute the desired state of the ControlPlane object, eventually adding a reference to the
	// InfrastructureMachineTemplate generated by the previous step.
	if desiredState.ControlPlane.Object, err = g.computeControlPlane(ctx, s, desiredState.ControlPlane.InfrastructureMachineTemplate); err != nil {
		return nil, errors.Wrapf(err, "failed to compute ControlPlane")
	}

	// Compute the desired state of the ControlPlane MachineHealthCheck if defined.
	// The MachineHealthCheck will have the same name as the ControlPlane Object and a selector for the ControlPlane InfrastructureMachines.
	if s.Blueprint.IsControlPlaneMachineHealthCheckEnabled() {
		checks, remediation := s.Blueprint.ControlPlaneMachineHealthCheckClass()
		desiredState.ControlPlane.MachineHealthCheck = computeMachineHealthCheck(
			ctx,
			desiredState.ControlPlane.Object,
			selectors.ForControlPlaneMHC(),
			s.Current.Cluster,
			checks, remediation)
	}

	// Compute the desired state for the Cluster object adding a reference to the
	// InfrastructureCluster and the ControlPlane objects generated by the previous step.
	desiredState.Cluster = computeCluster(ctx, s, desiredState.InfrastructureCluster, desiredState.ControlPlane.Object)

	// If required, compute the desired state of the MachineDeployments from the list of MachineDeploymentTopologies
	// defined in the cluster.
	if s.Blueprint.HasMachineDeployments() {
		desiredState.MachineDeployments, err = g.computeMachineDeployments(ctx, s)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to compute MachineDeployments")
		}
	}

	// If required, compute the desired state of the MachinePools from the list of MachinePoolTopologies
	// defined in the cluster.
	if s.Blueprint.HasMachinePools() {
		desiredState.MachinePools, err = g.computeMachinePools(ctx, s)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to compute MachinePools")
		}
	}

	// Apply patches the desired state according to the patches from the ClusterClass, variables from the Cluster
	// and builtin variables.
	// NOTE: We have to make sure all spec fields that were explicitly set in desired objects during the computation above
	// are preserved during patching. When desired objects are computed their spec is copied from a template, in some cases
	// further modifications to the spec are made afterwards. In those cases we have to make sure those fields are not overwritten
	// in apply patches. Some examples are .spec.machineTemplate and .spec.version in control planes.
	if err := g.patchEngine.Apply(ctx, s.Blueprint, desiredState); err != nil {
		return nil, errors.Wrap(err, "failed to apply patches")
	}

	return desiredState, nil
}

// computeInfrastructureCluster computes the desired state for the InfrastructureCluster object starting from the
// corresponding template defined in the blueprint.
func computeInfrastructureCluster(_ context.Context, s *scope.Scope) (*unstructured.Unstructured, error) {
	template := s.Blueprint.InfrastructureClusterTemplate
	templateClonedFromRef := s.Blueprint.ClusterClass.Spec.Infrastructure.TemplateRef.ToObjectReference(s.Blueprint.ClusterClass.Namespace)
	cluster := s.Current.Cluster
	currentRef := cluster.Spec.InfrastructureRef

	nameTemplate := "{{ .cluster.name }}-{{ .random }}"
	if s.Blueprint.ClusterClass.Spec.Infrastructure.Naming.Template != "" {
		nameTemplate = s.Blueprint.ClusterClass.Spec.Infrastructure.Naming.Template
	}

	infrastructureCluster, err := templateToObject(templateToInput{
		template:              template,
		templateClonedFromRef: templateClonedFromRef,
		cluster:               cluster,
		nameGenerator:         topologynames.InfraClusterNameGenerator(nameTemplate, cluster.Name),
		currentObjectName:     currentRef.Name,
		// Note: It is not possible to add an ownerRef to Cluster at this stage, otherwise the provisioning
		// of the infrastructure cluster starts no matter of the object being actually referenced by the Cluster itself.
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate the InfrastructureCluster object from the %s", template.GetKind())
	}

	// Carry over shim owner reference if any.
	// NOTE: this prevents to the ownerRef to be deleted by server side apply.
	if s.Current.InfrastructureCluster != nil {
		shim := clustershim.New(s.Current.Cluster)
		if ref := getOwnerReferenceFrom(s.Current.InfrastructureCluster, shim); ref != nil {
			infrastructureCluster.SetOwnerReferences([]metav1.OwnerReference{*ref})
		}
	}

	return infrastructureCluster, nil
}

// computeControlPlaneInfrastructureMachineTemplate computes the desired state for InfrastructureMachineTemplate
// that should be referenced by the ControlPlane object.
func (g *generator) computeControlPlaneInfrastructureMachineTemplate(ctx context.Context, s *scope.Scope) (*unstructured.Unstructured, error) {
	template := s.Blueprint.ControlPlane.InfrastructureMachineTemplate
	templateClonedFromRef := s.Blueprint.ClusterClass.Spec.ControlPlane.MachineInfrastructure.TemplateRef.ToObjectReference(s.Blueprint.ClusterClass.Namespace)
	cluster := s.Current.Cluster

	// Check if the current control plane object has a machineTemplate.infrastructureRef already defined.
	// TODO: Move the next few lines into a method on scope.ControlPlaneState
	var currentObjectName string
	if s.Current.ControlPlane != nil && s.Current.ControlPlane.Object != nil {
		// Determine contract version used by the ControlPlane.
		contractVersion, err := contract.GetContractVersionForVersion(ctx, g.Client, s.Current.ControlPlane.Object.GroupVersionKind().GroupKind(), s.Current.ControlPlane.Object.GroupVersionKind().Version)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get contract version for the ControlPlane object")
		}

		if contractVersion == "v1beta1" {
			currentRef, err := contract.ControlPlane().MachineTemplate().InfrastructureV1Beta1Ref().Get(s.Current.ControlPlane.Object)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get spec.machineTemplate.infrastructureRef for the current ControlPlane object")
			}
			currentObjectName = currentRef.Name
		} else {
			currentRef, err := contract.ControlPlane().MachineTemplate().InfrastructureRef().Get(s.Current.ControlPlane.Object)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get spec.machineTemplate.spec.infrastructureRef for the current ControlPlane object")
			}
			currentObjectName = currentRef.Name
		}
	}

	return templateToTemplate(templateToInput{
		template:              template,
		templateClonedFromRef: templateClonedFromRef,
		cluster:               cluster,
		nameGenerator:         topologynames.SimpleNameGenerator(topologynames.ControlPlaneInfrastructureMachineTemplateNamePrefix(cluster.Name)),
		currentObjectName:     currentObjectName,
		// Note: we are adding an ownerRef to Cluster so the template will be automatically garbage collected
		// in case of errors in between creating this template and updating the Cluster object
		// with the reference to the ControlPlane object using this template.
		ownerRef: ownerrefs.OwnerReferenceTo(s.Current.Cluster, clusterv1.GroupVersion.WithKind("Cluster")),
	})
}

// computeControlPlane computes the desired state for the ControlPlane object starting from the
// corresponding template defined in the blueprint.
func (g *generator) computeControlPlane(ctx context.Context, s *scope.Scope, infrastructureMachineTemplate *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	template := s.Blueprint.ControlPlane.Template
	templateClonedFromRef := s.Blueprint.ClusterClass.Spec.ControlPlane.TemplateRef.ToObjectReference(s.Blueprint.ClusterClass.Namespace)
	cluster := s.Current.Cluster
	currentRef := cluster.Spec.ControlPlaneRef

	// Compute the labels and annotations to be applied to ControlPlane metadata and ControlPlane machines.
	// We merge the labels and annotations from topology and ClusterClass.
	// We also add the cluster-name and the topology owned labels, so they are propagated down.
	topologyMetadata := s.Blueprint.Topology.ControlPlane.Metadata
	clusterClassMetadata := s.Blueprint.ClusterClass.Spec.ControlPlane.Metadata

	controlPlaneLabels := util.MergeMap(topologyMetadata.Labels, clusterClassMetadata.Labels)
	if controlPlaneLabels == nil {
		controlPlaneLabels = map[string]string{}
	}
	controlPlaneLabels[clusterv1.ClusterNameLabel] = cluster.Name
	controlPlaneLabels[clusterv1.ClusterTopologyOwnedLabel] = ""

	controlPlaneAnnotations := util.MergeMap(topologyMetadata.Annotations, clusterClassMetadata.Annotations)

	nameTemplate := "{{ .cluster.name }}-{{ .random }}"
	if s.Blueprint.ClusterClass.Spec.ControlPlane.Naming.Template != "" {
		nameTemplate = s.Blueprint.ClusterClass.Spec.ControlPlane.Naming.Template
	}

	controlPlane, err := templateToObject(templateToInput{
		template:              template,
		templateClonedFromRef: templateClonedFromRef,
		cluster:               cluster,
		nameGenerator:         topologynames.ControlPlaneNameGenerator(nameTemplate, cluster.Name),
		currentObjectName:     currentRef.Name,
		labels:                controlPlaneLabels,
		annotations:           controlPlaneAnnotations,
		// Note: It is not possible to add an ownerRef to Cluster at this stage, otherwise the provisioning
		// of the ControlPlane starts no matter of the object being actually referenced by the Cluster itself.
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate the ControlPlane object from the %s", template.GetKind())
	}

	// Carry over shim owner reference if any.
	// NOTE: this prevents to the ownerRef to be deleted by server side apply.
	if s.Current.ControlPlane != nil && s.Current.ControlPlane.Object != nil {
		shim := clustershim.New(s.Current.Cluster)
		if ref := getOwnerReferenceFrom(s.Current.ControlPlane.Object, shim); ref != nil {
			controlPlane.SetOwnerReferences([]metav1.OwnerReference{*ref})
		}
	}

	// Determine contract version used by the ControlPlane.
	contractVersion, err := contract.GetContractVersionForVersion(ctx, g.Client, controlPlane.GroupVersionKind().GroupKind(), controlPlane.GroupVersionKind().Version)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get contract version for the ControlPlane object")
	}

	// If the ClusterClass mandates the controlPlane has infrastructureMachines, add a reference to InfrastructureMachine
	// template and metadata to be used for the control plane machines.
	if s.Blueprint.HasControlPlaneInfrastructureMachine() {
		// We have to copy the template to avoid modifying the one from desired state.
		desiredRef := contract.ObjToRef(infrastructureMachineTemplate)

		if contractVersion == "v1beta1" {
			// If contract v1beta1 is used and the ControlPlane already exists, avoid downgrading the version if it was bumped
			// by the control plane controller in the meantime.
			if s.Current.ControlPlane.Object != nil {
				currentRef, err := contract.ControlPlane().MachineTemplate().InfrastructureV1Beta1Ref().Get(s.Current.ControlPlane.Object)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to get %s from the ControlPlane object", contract.ControlPlane().MachineTemplate().InfrastructureV1Beta1Ref().Path())
				}
				desiredRef, err = calculateRefDesiredAPIVersion(currentRef, infrastructureMachineTemplate)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to calculate desired %s", contract.ControlPlane().MachineTemplate().InfrastructureV1Beta1Ref().Path())
				}
			}
			if err := contract.ControlPlane().MachineTemplate().InfrastructureV1Beta1Ref().Set(controlPlane, desiredRef); err != nil {
				return nil, errors.Wrapf(err, "failed to set %s in the ControlPlane object", contract.ControlPlane().MachineTemplate().InfrastructureV1Beta1Ref().Path())
			}
		} else {
			if err := contract.ControlPlane().MachineTemplate().InfrastructureRef().Set(controlPlane, &clusterv1.ContractVersionedObjectReference{
				APIGroup: desiredRef.GroupVersionKind().Group,
				Kind:     desiredRef.Kind,
				Name:     desiredRef.Name,
			}); err != nil {
				return nil, errors.Wrapf(err, "failed to set %s in the ControlPlane object", contract.ControlPlane().MachineTemplate().InfrastructureRef().Path())
			}
		}

		// Add the ControlPlane labels and annotations to the ControlPlane machines as well.
		// Note: We have to ensure the machine template metadata copied from the control plane template is not overwritten.
		controlPlaneMachineTemplateMetadata, err := contract.ControlPlane().MachineTemplate().Metadata().Get(controlPlane)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get %s from the ControlPlane object", contract.ControlPlane().MachineTemplate().Metadata().Path())
		}

		controlPlaneMachineTemplateMetadata.Labels = util.MergeMap(controlPlaneLabels, controlPlaneMachineTemplateMetadata.Labels)
		controlPlaneMachineTemplateMetadata.Annotations = util.MergeMap(controlPlaneAnnotations, controlPlaneMachineTemplateMetadata.Annotations)

		if err := contract.ControlPlane().MachineTemplate().Metadata().Set(controlPlane,
			&clusterv1.ObjectMeta{
				Labels:      controlPlaneMachineTemplateMetadata.Labels,
				Annotations: controlPlaneMachineTemplateMetadata.Annotations,
			}); err != nil {
			return nil, errors.Wrapf(err, "failed to set %s in the ControlPlane object", contract.ControlPlane().MachineTemplate().Metadata().Path())
		}
	}

	// If it is required to manage the number of replicas for the control plane, set the corresponding field.
	// NOTE: If the Topology.ControlPlane.replicas value is nil, it is assumed that the control plane controller
	// does not implement support for this field and the ControlPlane object is generated without the number of Replicas.
	if s.Blueprint.Topology.ControlPlane.Replicas != nil {
		if err := contract.ControlPlane().Replicas().Set(controlPlane, *s.Blueprint.Topology.ControlPlane.Replicas); err != nil {
			return nil, errors.Wrapf(err, "failed to set %s in the ControlPlane object", contract.ControlPlane().Replicas().Path())
		}
	}

	// If it is required to manage the readinessGates for the control plane, set the corresponding field.
	// NOTE: If readinessGates value from both Cluster and ClusterClass is nil, it is assumed that the control plane controller
	// does not implement support for this field and the ControlPlane object is generated without readinessGates.
	if s.Blueprint.Topology.ControlPlane.ReadinessGates != nil {
		if err := contract.ControlPlane().MachineTemplate().ReadinessGates(contractVersion).Set(controlPlane, s.Blueprint.Topology.ControlPlane.ReadinessGates); err != nil {
			return nil, errors.Wrapf(err, "failed to set %s in the ControlPlane object", contract.ControlPlane().MachineTemplate().ReadinessGates(contractVersion).Path())
		}
	} else if s.Blueprint.ClusterClass.Spec.ControlPlane.ReadinessGates != nil {
		if err := contract.ControlPlane().MachineTemplate().ReadinessGates(contractVersion).Set(controlPlane, s.Blueprint.ClusterClass.Spec.ControlPlane.ReadinessGates); err != nil {
			return nil, errors.Wrapf(err, "failed to set %s in the ControlPlane object", contract.ControlPlane().MachineTemplate().ReadinessGates(contractVersion).Path())
		}
	}

	// If it is required to manage the NodeDrainTimeoutSeconds for the control plane, set the corresponding field.
	nodeDrainTimeout := s.Blueprint.ClusterClass.Spec.ControlPlane.Deletion.NodeDrainTimeoutSeconds
	if s.Blueprint.Topology.ControlPlane.Deletion.NodeDrainTimeoutSeconds != nil {
		nodeDrainTimeout = s.Blueprint.Topology.ControlPlane.Deletion.NodeDrainTimeoutSeconds
	}
	if nodeDrainTimeout != nil {
		if contractVersion == "v1beta1" {
			if err := contract.ControlPlane().MachineTemplate().NodeDrainTimeout().Set(controlPlane, metav1.Duration{Duration: time.Duration(*nodeDrainTimeout) * time.Second}); err != nil {
				return nil, errors.Wrapf(err, "failed to set %s in the ControlPlane object", contract.ControlPlane().MachineTemplate().NodeDrainTimeout().Path())
			}
		} else {
			if err := contract.ControlPlane().MachineTemplate().NodeDrainTimeoutSeconds().Set(controlPlane, *nodeDrainTimeout); err != nil {
				return nil, errors.Wrapf(err, "failed to set %s in the ControlPlane object", contract.ControlPlane().MachineTemplate().NodeDrainTimeoutSeconds().Path())
			}
		}
	}

	// If it is required to manage the NodeVolumeDetachTimeoutSeconds for the control plane, set the corresponding field.
	nodeVolumeDetachTimeout := s.Blueprint.ClusterClass.Spec.ControlPlane.Deletion.NodeVolumeDetachTimeoutSeconds
	if s.Blueprint.Topology.ControlPlane.Deletion.NodeVolumeDetachTimeoutSeconds != nil {
		nodeVolumeDetachTimeout = s.Blueprint.Topology.ControlPlane.Deletion.NodeVolumeDetachTimeoutSeconds
	}
	if nodeVolumeDetachTimeout != nil {
		if contractVersion == "v1beta1" {
			if err := contract.ControlPlane().MachineTemplate().NodeVolumeDetachTimeout().Set(controlPlane, metav1.Duration{Duration: time.Duration(*nodeVolumeDetachTimeout) * time.Second}); err != nil {
				return nil, errors.Wrapf(err, "failed to set %s in the ControlPlane object", contract.ControlPlane().MachineTemplate().NodeVolumeDetachTimeout().Path())
			}
		} else {
			if err := contract.ControlPlane().MachineTemplate().NodeVolumeDetachTimeoutSeconds().Set(controlPlane, *nodeVolumeDetachTimeout); err != nil {
				return nil, errors.Wrapf(err, "failed to set %s in the ControlPlane object", contract.ControlPlane().MachineTemplate().NodeVolumeDetachTimeoutSeconds().Path())
			}
		}
	}

	// If it is required to manage the NodeDeletionTimeoutSeconds for the control plane, set the corresponding field.
	nodeDeletionTimeout := s.Blueprint.ClusterClass.Spec.ControlPlane.Deletion.NodeDeletionTimeoutSeconds
	if s.Blueprint.Topology.ControlPlane.Deletion.NodeDeletionTimeoutSeconds != nil {
		nodeDeletionTimeout = s.Blueprint.Topology.ControlPlane.Deletion.NodeDeletionTimeoutSeconds
	}
	if nodeDeletionTimeout != nil {
		if contractVersion == "v1beta1" {
			if err := contract.ControlPlane().MachineTemplate().NodeDeletionTimeout().Set(controlPlane, metav1.Duration{Duration: time.Duration(*nodeDeletionTimeout) * time.Second}); err != nil {
				return nil, errors.Wrapf(err, "failed to set %s in the ControlPlane object", contract.ControlPlane().MachineTemplate().NodeDeletionTimeout().Path())
			}
		} else {
			if err := contract.ControlPlane().MachineTemplate().NodeDeletionTimeoutSeconds().Set(controlPlane, *nodeDeletionTimeout); err != nil {
				return nil, errors.Wrapf(err, "failed to set %s in the ControlPlane object", contract.ControlPlane().MachineTemplate().NodeDeletionTimeoutSeconds().Path())
			}
		}
	}

	// Sets the desired Kubernetes version for the control plane.
	version, err := g.computeControlPlaneVersion(ctx, s)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compute version of control plane")
	}
	if err := contract.ControlPlane().Version().Set(controlPlane, version); err != nil {
		return nil, errors.Wrapf(err, "failed to set %s in the ControlPlane object", contract.ControlPlane().Version().Path())
	}

	return controlPlane, nil
}

// computeControlPlaneVersion calculates the version of the desired control plane.
// The version is calculated using the state of the current machine deployments, the current control plane
// and the version defined in the topology.
func (g *generator) computeControlPlaneVersion(ctx context.Context, s *scope.Scope) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	desiredVersion := s.Blueprint.Topology.Version
	// If we are creating the control plane object (current control plane is nil), use version from topology.
	if s.Current.ControlPlane == nil || s.Current.ControlPlane.Object == nil {
		return desiredVersion, nil
	}

	// Get the current currentVersion of the control plane.
	currentVersion, err := contract.ControlPlane().Version().Get(s.Current.ControlPlane.Object)
	if err != nil {
		return "", errors.Wrap(err, "failed to get the version from control plane spec")
	}

	s.UpgradeTracker.ControlPlane.IsPendingUpgrade = true
	if *currentVersion == desiredVersion {
		// Mark that the control plane spec is already at the desired version.
		// This information is used to show the appropriate message for the TopologyReconciled
		// condition.
		s.UpgradeTracker.ControlPlane.IsPendingUpgrade = false
	}

	// Check if the control plane is being created for the first time.
	cpProvisioning, err := contract.ControlPlane().IsProvisioning(s.Current.ControlPlane.Object)
	if err != nil {
		return "", errors.Wrap(err, "failed to check if the control plane is being provisioned")
	}
	// If the control plane is being provisioned (being craeted for the first time), then do not
	// pick up the desiredVersion yet.
	// Return the current version of the control plane. We will pick up the new version after the
	// control plane is provisioned.
	if cpProvisioning {
		s.UpgradeTracker.ControlPlane.IsProvisioning = true
		return *currentVersion, nil
	}

	// Check if the current control plane is upgrading
	cpUpgrading, err := contract.ControlPlane().IsUpgrading(s.Current.ControlPlane.Object)
	if err != nil {
		return "", errors.Wrap(err, "failed to check if control plane is upgrading")
	}
	// If the current control plane is upgrading  (still completing a previous upgrade),
	// then do not pick up the desiredVersion yet.
	// Return the current version of the control plane. We will pick up the new version
	// after the control plane is stable.
	if cpUpgrading {
		s.UpgradeTracker.ControlPlane.IsUpgrading = true
		return *currentVersion, nil
	}

	// Return here if the control plane is already at the desired version
	if !s.UpgradeTracker.ControlPlane.IsPendingUpgrade {
		// At this stage the control plane is not upgrading and is already at the desired version.
		// We can return.
		// Nb. We do not return early in the function if the control plane is already at the desired version so as
		// to know if the control plane is being upgraded. This information
		// is required when updating the TopologyReconciled condition on the cluster.

		// Call the AfterControlPlaneUpgrade now that the control plane is upgraded.
		if feature.Gates.Enabled(feature.RuntimeSDK) {
			// Call the hook only if we are tracking the intent to do so. If it is not tracked it means we don't need to call the
			// hook because we didn't go through an upgrade or we already called the hook after the upgrade.
			if hooks.IsPending(runtimehooksv1.AfterControlPlaneUpgrade, s.Current.Cluster) {
				v1beta1Cluster := &clusterv1beta1.Cluster{}
				if err := v1beta1Cluster.ConvertFrom(s.Current.Cluster); err != nil {
					return "", errors.Wrap(err, "error converting Cluster to v1beta1 Cluster")
				}

				// Call all the registered extension for the hook.
				hookRequest := &runtimehooksv1.AfterControlPlaneUpgradeRequest{
					Cluster:           *cleanupCluster(v1beta1Cluster),
					KubernetesVersion: desiredVersion,
				}
				hookResponse := &runtimehooksv1.AfterControlPlaneUpgradeResponse{}
				if err := g.RuntimeClient.CallAllExtensions(ctx, runtimehooksv1.AfterControlPlaneUpgrade, s.Current.Cluster, hookRequest, hookResponse); err != nil {
					return "", err
				}
				// Add the response to the tracker so we can later update condition or requeue when required.
				s.HookResponseTracker.Add(runtimehooksv1.AfterControlPlaneUpgrade, hookResponse)

				// If the extension responds to hold off on starting Machine deployments upgrades,
				// change the UpgradeTracker accordingly, otherwise the hook call is completed and we
				// can remove this hook from the list of pending-hooks.
				if hookResponse.RetryAfterSeconds != 0 {
					log.Info(fmt.Sprintf("MachineDeployments/MachinePools upgrade to version %q are blocked by %q hook", desiredVersion, runtimecatalog.HookName(runtimehooksv1.AfterControlPlaneUpgrade)))
				} else {
					if err := hooks.MarkAsDone(ctx, g.Client, s.Current.Cluster, runtimehooksv1.AfterControlPlaneUpgrade); err != nil {
						return "", err
					}
				}
			}
		}

		return *currentVersion, nil
	}

	// If the control plane is not upgrading or scaling, we can assume the control plane is stable.
	// However, we should also check for the MachineDeployments/MachinePools upgrading.
	// If the MachineDeployments/MachinePools are upgrading, then do not pick up the desiredVersion yet.
	// We will pick up the new version after the MachineDeployments/MachinePools finish upgrading.
	if len(s.UpgradeTracker.MachineDeployments.UpgradingNames()) > 0 ||
		len(s.UpgradeTracker.MachinePools.UpgradingNames()) > 0 {
		return *currentVersion, nil
	}

	if feature.Gates.Enabled(feature.RuntimeSDK) {
		var hookAnnotations []string
		for key := range s.Current.Cluster.Annotations {
			if strings.HasPrefix(key, clusterv1.BeforeClusterUpgradeHookAnnotationPrefix) {
				hookAnnotations = append(hookAnnotations, key)
			}
		}
		if len(hookAnnotations) > 0 {
			slices.Sort(hookAnnotations)
			message := fmt.Sprintf("annotations [%s] are set", strings.Join(hookAnnotations, ", "))
			if len(hookAnnotations) == 1 {
				message = fmt.Sprintf("annotation [%s] is set", strings.Join(hookAnnotations, ", "))
			}
			// Add the hook with a response to the tracker so we can later update the condition.
			s.HookResponseTracker.Add(runtimehooksv1.BeforeClusterUpgrade, &runtimehooksv1.BeforeClusterUpgradeResponse{
				CommonRetryResponse: runtimehooksv1.CommonRetryResponse{
					// RetryAfterSeconds needs to be set because having only hooks without RetryAfterSeconds
					// would lead to not updating the condition. We can rely on getting an event when the
					// annotation gets removed so we set twice of the default sync-period to not cause additional reconciles.
					RetryAfterSeconds: 20 * 60,
					CommonResponse: runtimehooksv1.CommonResponse{
						Message: message,
					},
				},
			})

			log.Info(fmt.Sprintf("Cluster upgrade to version %q is blocked by %q hook (via annotations)", desiredVersion, runtimecatalog.HookName(runtimehooksv1.BeforeClusterUpgrade)), "hooks", strings.Join(hookAnnotations, ","))
			return *currentVersion, nil
		}

		// At this point the control plane and the machine deployments are stable and we are almost ready to pick
		// up the desiredVersion. Call the BeforeClusterUpgrade hook before picking up the desired version.
		v1beta1Cluster := &clusterv1beta1.Cluster{}
		if err := v1beta1Cluster.ConvertFrom(s.Current.Cluster); err != nil {
			return "", errors.Wrap(err, "error converting Cluster to v1beta1 Cluster")
		}

		hookRequest := &runtimehooksv1.BeforeClusterUpgradeRequest{
			Cluster:               *cleanupCluster(v1beta1Cluster),
			FromKubernetesVersion: *currentVersion,
			ToKubernetesVersion:   desiredVersion,
		}
		hookResponse := &runtimehooksv1.BeforeClusterUpgradeResponse{}
		if err := g.RuntimeClient.CallAllExtensions(ctx, runtimehooksv1.BeforeClusterUpgrade, s.Current.Cluster, hookRequest, hookResponse); err != nil {
			return "", err
		}
		// Add the response to the tracker so we can later update condition or requeue when required.
		s.HookResponseTracker.Add(runtimehooksv1.BeforeClusterUpgrade, hookResponse)
		if hookResponse.RetryAfterSeconds != 0 {
			// Cannot pickup the new version right now. Need to try again later.
			log.Info(fmt.Sprintf("Cluster upgrade to version %q is blocked by %q hook", desiredVersion, runtimecatalog.HookName(runtimehooksv1.BeforeClusterUpgrade)))
			return *currentVersion, nil
		}

		// We are picking up the new version here.
		// Track the intent of calling the AfterControlPlaneUpgrade and the AfterClusterUpgrade hooks once we are done with the upgrade.
		if err := hooks.MarkAsPending(ctx, g.Client, s.Current.Cluster, runtimehooksv1.AfterControlPlaneUpgrade, runtimehooksv1.AfterClusterUpgrade); err != nil {
			return "", err
		}
	}

	// Control plane and machine deployments are stable. All the required hook are called.
	// Ready to pick up the topology version.
	s.UpgradeTracker.ControlPlane.IsPendingUpgrade = false
	s.UpgradeTracker.ControlPlane.IsStartingUpgrade = true
	return desiredVersion, nil
}

// computeCluster computes the desired state for the Cluster object.
// NOTE: Some fields of the Cluster’s fields contribute to defining the Cluster blueprint (e.g. Cluster.Spec.Topology),
// while some other fields should be managed as part of the actual Cluster (e.g. Cluster.Spec.ControlPlaneRef); in this func
// we are concerned only about the latest group of fields.
func computeCluster(_ context.Context, s *scope.Scope, infrastructureCluster, controlPlane *unstructured.Unstructured) *clusterv1.Cluster {
	cluster := s.Current.Cluster.DeepCopy()

	// Enforce the topology labels.
	// NOTE: The cluster label is added at creation time so this object could be read by the ClusterTopology
	// controller immediately after creation, even before other controllers are going to add the label (if missing).
	if cluster.Labels == nil {
		cluster.Labels = map[string]string{}
	}
	cluster.Labels[clusterv1.ClusterNameLabel] = cluster.Name
	cluster.Labels[clusterv1.ClusterTopologyOwnedLabel] = ""

	// Set the references to the infrastructureCluster and controlPlane objects.
	// NOTE: Once set for the first time, the references are not expected to change.
	cluster.Spec.InfrastructureRef = contract.ObjToContractVersionedObjectReference(infrastructureCluster)
	cluster.Spec.ControlPlaneRef = contract.ObjToContractVersionedObjectReference(controlPlane)

	return cluster
}

// calculateRefDesiredAPIVersion returns the desired ref calculated from desiredReferencedObject
// so it doesn't override the version in apiVersion stored in the currentRef, if any.
// This is required because the apiVersion in the desired ref is aligned to the apiVersion used
// in ClusterClass when reading the current state. If the currentRef is nil or group or kind
// doesn't match, no changes are applied to desired ref.
func calculateRefDesiredAPIVersion(currentRef *corev1.ObjectReference, desiredReferencedObject *unstructured.Unstructured) (*corev1.ObjectReference, error) {
	desiredRef := contract.ObjToRef(desiredReferencedObject)
	// If ref is not set yet, just set a ref to the desired referenced object.
	if currentRef == nil {
		return desiredRef, nil
	}

	currentGV, err := schema.ParseGroupVersion(currentRef.APIVersion)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse apiVersion %q of current ref", currentRef.APIVersion)
	}
	desiredGK := desiredReferencedObject.GroupVersionKind().GroupKind()

	// Keep the apiVersion of the current ref if the group and kind is already correct.
	// We only want to change the apiVersion to update the group, as it should be possible
	// for other controllers to bump the version if necessary (i.e. if there is a newer
	// version of the CRD compared to the one that the topology controller is working on).
	if currentGV.Group == desiredGK.Group && currentRef.Kind == desiredGK.Kind {
		desiredRef.APIVersion = currentRef.APIVersion
	}
	return desiredRef, nil
}

// computeMachineDeployments computes the desired state of the list of MachineDeployments.
func (g *generator) computeMachineDeployments(ctx context.Context, s *scope.Scope) (scope.MachineDeploymentsStateMap, error) {
	machineDeploymentsStateMap := make(scope.MachineDeploymentsStateMap)
	for _, mdTopology := range s.Blueprint.Topology.Workers.MachineDeployments {
		desiredMachineDeployment, err := g.computeMachineDeployment(ctx, s, mdTopology)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to compute MachineDepoyment for topology %q", mdTopology.Name)
		}
		machineDeploymentsStateMap[mdTopology.Name] = desiredMachineDeployment
	}
	return machineDeploymentsStateMap, nil
}

// computeMachineDeployment computes the desired state for a MachineDeploymentTopology.
// The generated machineDeployment object is calculated using the values from the machineDeploymentTopology and
// the machineDeployment class.
func (g *generator) computeMachineDeployment(ctx context.Context, s *scope.Scope, machineDeploymentTopology clusterv1.MachineDeploymentTopology) (*scope.MachineDeploymentState, error) {
	desiredMachineDeployment := &scope.MachineDeploymentState{}

	// Gets the blueprint for the MachineDeployment class.
	className := machineDeploymentTopology.Class
	machineDeploymentBlueprint, ok := s.Blueprint.MachineDeployments[className]
	if !ok {
		return nil, errors.Errorf("MachineDeployment class %s not found in ClusterClass %s", className, klog.KObj(s.Blueprint.ClusterClass))
	}

	var machineDeploymentClass *clusterv1.MachineDeploymentClass
	for _, mdClass := range s.Blueprint.ClusterClass.Spec.Workers.MachineDeployments {
		if mdClass.Class == className {
			machineDeploymentClass = &mdClass
			break
		}
	}
	if machineDeploymentClass == nil {
		return nil, errors.Errorf("MachineDeployment class %s not found in ClusterClass %s", className, klog.KObj(s.Blueprint.ClusterClass))
	}

	// Compute the bootstrap template.
	currentMachineDeployment := s.Current.MachineDeployments[machineDeploymentTopology.Name]
	var currentBootstrapTemplateRef clusterv1.ContractVersionedObjectReference
	if currentMachineDeployment != nil && currentMachineDeployment.BootstrapTemplate != nil {
		currentBootstrapTemplateRef = currentMachineDeployment.Object.Spec.Template.Spec.Bootstrap.ConfigRef
	}
	var err error
	desiredMachineDeployment.BootstrapTemplate, err = templateToTemplate(templateToInput{
		template:              machineDeploymentBlueprint.BootstrapTemplate,
		templateClonedFromRef: contract.ObjToRef(machineDeploymentBlueprint.BootstrapTemplate),
		cluster:               s.Current.Cluster,
		nameGenerator:         topologynames.SimpleNameGenerator(topologynames.BootstrapTemplateNamePrefix(s.Current.Cluster.Name, machineDeploymentTopology.Name)),
		currentObjectName:     currentBootstrapTemplateRef.Name,
		// Note: we are adding an ownerRef to Cluster so the template will be automatically garbage collected
		// in case of errors in between creating this template and creating/updating the MachineDeployment object
		// with the reference to this template.
		ownerRef: ownerrefs.OwnerReferenceTo(s.Current.Cluster, clusterv1.GroupVersion.WithKind("Cluster")),
	})
	if err != nil {
		return nil, err
	}

	bootstrapTemplateLabels := desiredMachineDeployment.BootstrapTemplate.GetLabels()
	if bootstrapTemplateLabels == nil {
		bootstrapTemplateLabels = map[string]string{}
	}
	// Add ClusterTopologyMachineDeploymentLabel to the generated Bootstrap template
	bootstrapTemplateLabels[clusterv1.ClusterTopologyMachineDeploymentNameLabel] = machineDeploymentTopology.Name
	desiredMachineDeployment.BootstrapTemplate.SetLabels(bootstrapTemplateLabels)

	// Compute the Infrastructure template.
	var currentInfraMachineTemplateRef *clusterv1.ContractVersionedObjectReference
	if currentMachineDeployment != nil && currentMachineDeployment.InfrastructureMachineTemplate != nil {
		currentInfraMachineTemplateRef = &currentMachineDeployment.Object.Spec.Template.Spec.InfrastructureRef
	}
	desiredMachineDeployment.InfrastructureMachineTemplate, err = templateToTemplate(templateToInput{
		template:              machineDeploymentBlueprint.InfrastructureMachineTemplate,
		templateClonedFromRef: contract.ObjToRef(machineDeploymentBlueprint.InfrastructureMachineTemplate),
		cluster:               s.Current.Cluster,
		nameGenerator:         topologynames.SimpleNameGenerator(topologynames.InfrastructureMachineTemplateNamePrefix(s.Current.Cluster.Name, machineDeploymentTopology.Name)),
		currentObjectName:     ptr.Deref(currentInfraMachineTemplateRef, clusterv1.ContractVersionedObjectReference{}).Name,
		// Note: we are adding an ownerRef to Cluster so the template will be automatically garbage collected
		// in case of errors in between creating this template and creating/updating the MachineDeployment object
		// with the reference to this template.
		ownerRef: ownerrefs.OwnerReferenceTo(s.Current.Cluster, clusterv1.GroupVersion.WithKind("Cluster")),
	})
	if err != nil {
		return nil, err
	}

	infraMachineTemplateLabels := desiredMachineDeployment.InfrastructureMachineTemplate.GetLabels()
	if infraMachineTemplateLabels == nil {
		infraMachineTemplateLabels = map[string]string{}
	}
	// Add ClusterTopologyMachineDeploymentLabel to the generated InfrastructureMachine template
	infraMachineTemplateLabels[clusterv1.ClusterTopologyMachineDeploymentNameLabel] = machineDeploymentTopology.Name
	desiredMachineDeployment.InfrastructureMachineTemplate.SetLabels(infraMachineTemplateLabels)
	version := g.computeMachineDeploymentVersion(s, machineDeploymentTopology, currentMachineDeployment)

	// Compute values that can be set both in the MachineDeploymentClass and in the MachineDeploymentTopology
	minReadySeconds := machineDeploymentClass.MinReadySeconds
	if machineDeploymentTopology.MinReadySeconds != nil {
		minReadySeconds = machineDeploymentTopology.MinReadySeconds
	}

	var rollout clusterv1.MachineDeploymentRolloutSpec
	if !reflect.DeepEqual(machineDeploymentClass.Rollout, clusterv1.MachineDeploymentRolloutSpec{}) {
		rollout = clusterv1.MachineDeploymentRolloutSpec{
			Strategy: clusterv1.MachineDeploymentRolloutStrategy{
				Type: machineDeploymentClass.Rollout.Strategy.Type,
				RollingUpdate: clusterv1.MachineDeploymentRolloutStrategyRollingUpdate{
					MaxUnavailable: machineDeploymentClass.Rollout.Strategy.RollingUpdate.MaxUnavailable,
					MaxSurge:       machineDeploymentClass.Rollout.Strategy.RollingUpdate.MaxSurge,
				},
			},
		}
	}
	if !reflect.DeepEqual(machineDeploymentTopology.Rollout, clusterv1.MachineDeploymentTopologyRolloutSpec{}) {
		rollout = clusterv1.MachineDeploymentRolloutSpec{
			Strategy: clusterv1.MachineDeploymentRolloutStrategy{
				Type: machineDeploymentTopology.Rollout.Strategy.Type,
				RollingUpdate: clusterv1.MachineDeploymentRolloutStrategyRollingUpdate{
					MaxUnavailable: machineDeploymentTopology.Rollout.Strategy.RollingUpdate.MaxUnavailable,
					MaxSurge:       machineDeploymentTopology.Rollout.Strategy.RollingUpdate.MaxSurge,
				},
			},
		}
	}

	remediationMaxInFlight := machineDeploymentClass.HealthCheck.Remediation.MaxInFlight
	if machineDeploymentTopology.HealthCheck.Remediation.MaxInFlight != nil {
		remediationMaxInFlight = machineDeploymentTopology.HealthCheck.Remediation.MaxInFlight
	}

	failureDomain := machineDeploymentClass.FailureDomain
	if machineDeploymentTopology.FailureDomain != "" {
		failureDomain = machineDeploymentTopology.FailureDomain
	}

	deletionOrder := machineDeploymentClass.Deletion.Order
	if machineDeploymentTopology.Deletion.Order != "" {
		deletionOrder = machineDeploymentTopology.Deletion.Order
	}

	nodeDrainTimeout := machineDeploymentClass.Deletion.NodeDrainTimeoutSeconds
	if machineDeploymentTopology.Deletion.NodeDrainTimeoutSeconds != nil {
		nodeDrainTimeout = machineDeploymentTopology.Deletion.NodeDrainTimeoutSeconds
	}

	nodeVolumeDetachTimeout := machineDeploymentClass.Deletion.NodeVolumeDetachTimeoutSeconds
	if machineDeploymentTopology.Deletion.NodeVolumeDetachTimeoutSeconds != nil {
		nodeVolumeDetachTimeout = machineDeploymentTopology.Deletion.NodeVolumeDetachTimeoutSeconds
	}

	nodeDeletionTimeout := machineDeploymentClass.Deletion.NodeDeletionTimeoutSeconds
	if machineDeploymentTopology.Deletion.NodeDeletionTimeoutSeconds != nil {
		nodeDeletionTimeout = machineDeploymentTopology.Deletion.NodeDeletionTimeoutSeconds
	}

	readinessGates := machineDeploymentClass.ReadinessGates
	if machineDeploymentTopology.ReadinessGates != nil {
		readinessGates = machineDeploymentTopology.ReadinessGates
	}

	// Compute the MachineDeployment object.
	desiredBootstrapTemplateRef := contract.ObjToContractVersionedObjectReference(desiredMachineDeployment.BootstrapTemplate)
	desiredInfraMachineTemplateRef := contract.ObjToContractVersionedObjectReference(desiredMachineDeployment.InfrastructureMachineTemplate)

	nameTemplate := "{{ .cluster.name }}-{{ .machineDeployment.topologyName }}-{{ .random }}"
	if machineDeploymentClass.Naming.Template != "" {
		nameTemplate = machineDeploymentClass.Naming.Template
	}

	name, err := topologynames.MachineDeploymentNameGenerator(nameTemplate, s.Current.Cluster.Name, machineDeploymentTopology.Name).GenerateName()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate name for MachineDeployment")
	}

	desiredMachineDeploymentObj := &clusterv1.MachineDeployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.Current.Cluster.Namespace,
		},
		Spec: clusterv1.MachineDeploymentSpec{
			ClusterName: s.Current.Cluster.Name,
			Rollout:     rollout,
			Deletion: clusterv1.MachineDeploymentDeletionSpec{
				Order: deletionOrder,
			},
			Remediation: clusterv1.MachineDeploymentRemediationSpec{
				MaxInFlight: remediationMaxInFlight,
			},
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					ClusterName:       s.Current.Cluster.Name,
					Version:           version,
					Bootstrap:         clusterv1.Bootstrap{ConfigRef: desiredBootstrapTemplateRef},
					InfrastructureRef: desiredInfraMachineTemplateRef,
					FailureDomain:     failureDomain,
					Deletion: clusterv1.MachineDeletionSpec{
						NodeDrainTimeoutSeconds:        nodeDrainTimeout,
						NodeVolumeDetachTimeoutSeconds: nodeVolumeDetachTimeout,
						NodeDeletionTimeoutSeconds:     nodeDeletionTimeout,
					},
					ReadinessGates:  readinessGates,
					MinReadySeconds: minReadySeconds,
				},
			},
		},
	}

	// If an existing MachineDeployment is present, override the MachineDeployment generate name
	// re-using the existing name (this will help in reconcile).
	if currentMachineDeployment != nil && currentMachineDeployment.Object != nil {
		desiredMachineDeploymentObj.SetName(currentMachineDeployment.Object.Name)
	}

	// Apply annotations
	machineDeploymentAnnotations := util.MergeMap(machineDeploymentTopology.Metadata.Annotations, machineDeploymentBlueprint.Metadata.Annotations)
	// Ensure the annotations used to control the upgrade sequence are never propagated.
	delete(machineDeploymentAnnotations, clusterv1.ClusterTopologyHoldUpgradeSequenceAnnotation)
	delete(machineDeploymentAnnotations, clusterv1.ClusterTopologyDeferUpgradeAnnotation)
	desiredMachineDeploymentObj.SetAnnotations(machineDeploymentAnnotations)
	desiredMachineDeploymentObj.Spec.Template.Annotations = machineDeploymentAnnotations

	// Apply Labels
	// NOTE: On top of all the labels applied to managed objects we are applying the ClusterTopologyMachineDeploymentLabel
	// keeping track of the MachineDeployment name from the Topology; this will be used to identify the object in next reconcile loops.
	machineDeploymentLabels := util.MergeMap(machineDeploymentTopology.Metadata.Labels, machineDeploymentBlueprint.Metadata.Labels)
	if machineDeploymentLabels == nil {
		machineDeploymentLabels = map[string]string{}
	}
	machineDeploymentLabels[clusterv1.ClusterNameLabel] = s.Current.Cluster.Name
	machineDeploymentLabels[clusterv1.ClusterTopologyOwnedLabel] = ""
	machineDeploymentLabels[clusterv1.ClusterTopologyMachineDeploymentNameLabel] = machineDeploymentTopology.Name
	desiredMachineDeploymentObj.SetLabels(machineDeploymentLabels)

	// Also set the labels in .spec.template.labels so that they are propagated to
	// MachineSet.labels and MachineSet.spec.template.labels and thus to Machine.labels.
	// Note: the labels in MachineSet are used to properly cleanup templates when the MachineSet is deleted.
	desiredMachineDeploymentObj.Spec.Template.Labels = machineDeploymentLabels

	// Set the selector with the subset of labels identifying controlled machines.
	// NOTE: this prevents the web hook to add cluster.x-k8s.io/deployment-name label, that is
	// redundant for managed MachineDeployments given that we already have topology.cluster.x-k8s.io/deployment-name.
	desiredMachineDeploymentObj.Spec.Selector.MatchLabels = map[string]string{}
	desiredMachineDeploymentObj.Spec.Selector.MatchLabels[clusterv1.ClusterNameLabel] = s.Current.Cluster.Name
	desiredMachineDeploymentObj.Spec.Selector.MatchLabels[clusterv1.ClusterTopologyOwnedLabel] = ""
	desiredMachineDeploymentObj.Spec.Selector.MatchLabels[clusterv1.ClusterTopologyMachineDeploymentNameLabel] = machineDeploymentTopology.Name

	// Set the desired replicas.
	desiredMachineDeploymentObj.Spec.Replicas = machineDeploymentTopology.Replicas

	desiredMachineDeployment.Object = desiredMachineDeploymentObj

	// If the ClusterClass defines a MachineHealthCheck for the MachineDeployment add it to the desired state.
	if s.Blueprint.IsMachineDeploymentMachineHealthCheckEnabled(&machineDeploymentTopology) {
		// Note: The MHC is going to use a selector that provides a minimal set of labels which are common to all MachineSets belonging to the MachineDeployment.
		checks, remediation := s.Blueprint.MachineDeploymentMachineHealthCheckClass(&machineDeploymentTopology)
		desiredMachineDeployment.MachineHealthCheck = computeMachineHealthCheck(
			ctx,
			desiredMachineDeploymentObj,
			selectors.ForMachineDeploymentMHC(desiredMachineDeploymentObj),
			s.Current.Cluster,
			checks, remediation)
	}
	return desiredMachineDeployment, nil
}

// computeMachineDeploymentVersion calculates the version of the desired machine deployment.
// The version is calculated using the state of the current machine deployments,
// the current control plane and the version defined in the topology.
func (g *generator) computeMachineDeploymentVersion(s *scope.Scope, machineDeploymentTopology clusterv1.MachineDeploymentTopology, currentMDState *scope.MachineDeploymentState) string {
	desiredVersion := s.Blueprint.Topology.Version
	// If creating a new machine deployment, mark it as pending if the control plane is not
	// yet stable. Creating a new MD while the control plane is upgrading can lead to unexpected race conditions.
	// Example: join could fail if the load balancers are slow in detecting when CP machines are
	// being deleted.
	if currentMDState == nil || currentMDState.Object == nil {
		if !s.UpgradeTracker.ControlPlane.IsControlPlaneStable() || s.HookResponseTracker.IsBlocking(runtimehooksv1.AfterControlPlaneUpgrade) {
			s.UpgradeTracker.MachineDeployments.MarkPendingCreate(machineDeploymentTopology.Name)
		}
		return desiredVersion
	}

	// Get the current version of the machine deployment.
	currentVersion := currentMDState.Object.Spec.Template.Spec.Version

	// Return early if the currentVersion is already equal to the desiredVersion
	// no further checks required.
	if currentVersion == desiredVersion {
		return currentVersion
	}

	// Return early if the upgrade for the MachineDeployment is deferred.
	if isMachineDeploymentDeferred(s.Blueprint.Topology, machineDeploymentTopology) {
		s.UpgradeTracker.MachineDeployments.MarkDeferredUpgrade(currentMDState.Object.Name)
		s.UpgradeTracker.MachineDeployments.MarkPendingUpgrade(currentMDState.Object.Name)
		return currentVersion
	}

	// Return early if the AfterControlPlaneUpgrade hook returns a blocking response.
	if s.HookResponseTracker.IsBlocking(runtimehooksv1.AfterControlPlaneUpgrade) {
		s.UpgradeTracker.MachineDeployments.MarkPendingUpgrade(currentMDState.Object.Name)
		return currentVersion
	}

	// Return early if the upgrade concurrency is reached.
	if s.UpgradeTracker.MachineDeployments.UpgradeConcurrencyReached() {
		s.UpgradeTracker.MachineDeployments.MarkPendingUpgrade(currentMDState.Object.Name)
		return currentVersion
	}

	// Return early if the Control Plane is not stable. Do not pick up the desiredVersion yet.
	// Return the current version of the machine deployment. We will pick up the new version after the control
	// plane is stable.
	if !s.UpgradeTracker.ControlPlane.IsControlPlaneStable() {
		s.UpgradeTracker.MachineDeployments.MarkPendingUpgrade(currentMDState.Object.Name)
		return currentVersion
	}

	// Control plane and machine deployments are stable.
	// Ready to pick up the topology version.
	s.UpgradeTracker.MachineDeployments.MarkUpgrading(currentMDState.Object.Name)
	return desiredVersion
}

// isMachineDeploymentDeferred returns true if the upgrade for the mdTopology is deferred.
// This is the case when either:
//   - the mdTopology has the ClusterTopologyDeferUpgradeAnnotation annotation.
//   - the mdTopology has the ClusterTopologyHoldUpgradeSequenceAnnotation annotation.
//   - another md topology which is before mdTopology in the workers.machineDeployments list has the
//     ClusterTopologyHoldUpgradeSequenceAnnotation annotation.
func isMachineDeploymentDeferred(clusterTopology clusterv1.Topology, mdTopology clusterv1.MachineDeploymentTopology) bool {
	// If mdTopology has the ClusterTopologyDeferUpgradeAnnotation annotation => md is deferred.
	if _, ok := mdTopology.Metadata.Annotations[clusterv1.ClusterTopologyDeferUpgradeAnnotation]; ok {
		return true
	}

	// If mdTopology has the ClusterTopologyHoldUpgradeSequenceAnnotation annotation => md is deferred.
	if _, ok := mdTopology.Metadata.Annotations[clusterv1.ClusterTopologyHoldUpgradeSequenceAnnotation]; ok {
		return true
	}

	for _, md := range clusterTopology.Workers.MachineDeployments {
		// If another md topology with the ClusterTopologyHoldUpgradeSequenceAnnotation annotation
		// is found before the mdTopology => md is deferred.
		if _, ok := md.Metadata.Annotations[clusterv1.ClusterTopologyHoldUpgradeSequenceAnnotation]; ok {
			return true
		}

		// If mdTopology is found before a md topology with the ClusterTopologyHoldUpgradeSequenceAnnotation
		// annotation => md is not deferred.
		if md.Name == mdTopology.Name {
			return false
		}
	}

	// This case should be impossible as mdTopology should have been found in workers.machineDeployments.
	return false
}

// computeMachinePools computes the desired state of the list of MachinePools.
func (g *generator) computeMachinePools(ctx context.Context, s *scope.Scope) (scope.MachinePoolsStateMap, error) {
	machinePoolsStateMap := make(scope.MachinePoolsStateMap)
	for _, mpTopology := range s.Blueprint.Topology.Workers.MachinePools {
		desiredMachinePool, err := g.computeMachinePool(ctx, s, mpTopology)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to compute MachinePool for topology %q", mpTopology.Name)
		}
		machinePoolsStateMap[mpTopology.Name] = desiredMachinePool
	}
	return machinePoolsStateMap, nil
}

// computeMachinePool computes the desired state for a MachinePoolTopology.
// The generated machinePool object is calculated using the values from the machinePoolTopology and
// the machinePool class.
func (g *generator) computeMachinePool(_ context.Context, s *scope.Scope, machinePoolTopology clusterv1.MachinePoolTopology) (*scope.MachinePoolState, error) {
	desiredMachinePool := &scope.MachinePoolState{}

	// Gets the blueprint for the MachinePool class.
	className := machinePoolTopology.Class
	machinePoolBlueprint, ok := s.Blueprint.MachinePools[className]
	if !ok {
		return nil, errors.Errorf("MachinePool class %s not found in ClusterClass %s", className, klog.KObj(s.Blueprint.ClusterClass))
	}

	var machinePoolClass *clusterv1.MachinePoolClass
	for _, mpClass := range s.Blueprint.ClusterClass.Spec.Workers.MachinePools {
		if mpClass.Class == className {
			machinePoolClass = &mpClass
			break
		}
	}
	if machinePoolClass == nil {
		return nil, errors.Errorf("MachinePool class %s not found in ClusterClass %s", className, klog.KObj(s.Blueprint.ClusterClass))
	}

	// Compute the bootstrap config.
	currentMachinePool := s.Current.MachinePools[machinePoolTopology.Name]
	var currentBootstrapConfigRef clusterv1.ContractVersionedObjectReference
	if currentMachinePool != nil && currentMachinePool.BootstrapObject != nil {
		currentBootstrapConfigRef = currentMachinePool.Object.Spec.Template.Spec.Bootstrap.ConfigRef
	}
	var err error
	desiredMachinePool.BootstrapObject, err = templateToObject(templateToInput{
		template:              machinePoolBlueprint.BootstrapTemplate,
		templateClonedFromRef: contract.ObjToRef(machinePoolBlueprint.BootstrapTemplate),
		cluster:               s.Current.Cluster,
		nameGenerator:         topologynames.SimpleNameGenerator(topologynames.BootstrapConfigNamePrefix(s.Current.Cluster.Name, machinePoolTopology.Name)),
		currentObjectName:     currentBootstrapConfigRef.Name,
		// Note: we are adding an ownerRef to Cluster so the template will be automatically garbage collected
		// in case of errors in between creating this template and creating/updating the MachinePool object
		// with the reference to this template.
		ownerRef: ownerrefs.OwnerReferenceTo(s.Current.Cluster, clusterv1.GroupVersion.WithKind("Cluster")),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to compute bootstrap object for topology %q", machinePoolTopology.Name)
	}

	bootstrapObjectLabels := desiredMachinePool.BootstrapObject.GetLabels()
	if bootstrapObjectLabels == nil {
		bootstrapObjectLabels = map[string]string{}
	}
	// Add ClusterTopologyMachinePoolLabel to the generated Bootstrap config
	bootstrapObjectLabels[clusterv1.ClusterTopologyMachinePoolNameLabel] = machinePoolTopology.Name
	desiredMachinePool.BootstrapObject.SetLabels(bootstrapObjectLabels)

	// Compute the InfrastructureMachinePool.
	var currentInfraMachinePoolRef *clusterv1.ContractVersionedObjectReference
	if currentMachinePool != nil && currentMachinePool.InfrastructureMachinePoolObject != nil {
		currentInfraMachinePoolRef = &currentMachinePool.Object.Spec.Template.Spec.InfrastructureRef
	}
	desiredMachinePool.InfrastructureMachinePoolObject, err = templateToObject(templateToInput{
		template:              machinePoolBlueprint.InfrastructureMachinePoolTemplate,
		templateClonedFromRef: contract.ObjToRef(machinePoolBlueprint.InfrastructureMachinePoolTemplate),
		cluster:               s.Current.Cluster,
		nameGenerator:         topologynames.SimpleNameGenerator(topologynames.InfrastructureMachinePoolNamePrefix(s.Current.Cluster.Name, machinePoolTopology.Name)),
		currentObjectName:     ptr.Deref(currentInfraMachinePoolRef, clusterv1.ContractVersionedObjectReference{}).Name,
		// Note: we are adding an ownerRef to Cluster so the template will be automatically garbage collected
		// in case of errors in between creating this template and creating/updating the MachinePool object
		// with the reference to this template.
		ownerRef: ownerrefs.OwnerReferenceTo(s.Current.Cluster, clusterv1.GroupVersion.WithKind("Cluster")),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to compute infrastructure object for topology %q", machinePoolTopology.Name)
	}

	infraMachinePoolObjectLabels := desiredMachinePool.InfrastructureMachinePoolObject.GetLabels()
	if infraMachinePoolObjectLabels == nil {
		infraMachinePoolObjectLabels = map[string]string{}
	}
	// Add ClusterTopologyMachinePoolLabel to the generated InfrastructureMachinePool object
	infraMachinePoolObjectLabels[clusterv1.ClusterTopologyMachinePoolNameLabel] = machinePoolTopology.Name
	desiredMachinePool.InfrastructureMachinePoolObject.SetLabels(infraMachinePoolObjectLabels)
	version := g.computeMachinePoolVersion(s, machinePoolTopology, currentMachinePool)

	// Compute values that can be set both in the MachinePoolClass and in the MachinePoolTopology
	minReadySeconds := machinePoolClass.MinReadySeconds
	if machinePoolTopology.MinReadySeconds != nil {
		minReadySeconds = machinePoolTopology.MinReadySeconds
	}

	failureDomains := machinePoolClass.FailureDomains
	if machinePoolTopology.FailureDomains != nil {
		failureDomains = machinePoolTopology.FailureDomains
	}

	nodeDrainTimeout := machinePoolClass.Deletion.NodeDrainTimeoutSeconds
	if machinePoolTopology.Deletion.NodeDrainTimeoutSeconds != nil {
		nodeDrainTimeout = machinePoolTopology.Deletion.NodeDrainTimeoutSeconds
	}

	nodeVolumeDetachTimeout := machinePoolClass.Deletion.NodeVolumeDetachTimeoutSeconds
	if machinePoolTopology.Deletion.NodeVolumeDetachTimeoutSeconds != nil {
		nodeVolumeDetachTimeout = machinePoolTopology.Deletion.NodeVolumeDetachTimeoutSeconds
	}

	nodeDeletionTimeout := machinePoolClass.Deletion.NodeDeletionTimeoutSeconds
	if machinePoolTopology.Deletion.NodeDeletionTimeoutSeconds != nil {
		nodeDeletionTimeout = machinePoolTopology.Deletion.NodeDeletionTimeoutSeconds
	}

	// Compute the MachinePool object.
	desiredBootstrapConfigRef := contract.ObjToContractVersionedObjectReference(desiredMachinePool.BootstrapObject)
	desiredInfraMachinePoolRef := contract.ObjToContractVersionedObjectReference(desiredMachinePool.InfrastructureMachinePoolObject)

	nameTemplate := "{{ .cluster.name }}-{{ .machinePool.topologyName }}-{{ .random }}"
	if machinePoolClass.Naming.Template != "" {
		nameTemplate = machinePoolClass.Naming.Template
	}

	name, err := topologynames.MachinePoolNameGenerator(nameTemplate, s.Current.Cluster.Name, machinePoolTopology.Name).GenerateName()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate name for MachinePool")
	}

	desiredMachinePoolObj := &clusterv1.MachinePool{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachinePool",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.Current.Cluster.Namespace,
		},
		Spec: clusterv1.MachinePoolSpec{
			ClusterName:    s.Current.Cluster.Name,
			FailureDomains: failureDomains,
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					ClusterName:       s.Current.Cluster.Name,
					Version:           version,
					Bootstrap:         clusterv1.Bootstrap{ConfigRef: desiredBootstrapConfigRef},
					InfrastructureRef: desiredInfraMachinePoolRef,
					Deletion: clusterv1.MachineDeletionSpec{
						NodeDrainTimeoutSeconds:        nodeDrainTimeout,
						NodeVolumeDetachTimeoutSeconds: nodeVolumeDetachTimeout,
						NodeDeletionTimeoutSeconds:     nodeDeletionTimeout,
					},
					MinReadySeconds: minReadySeconds,
				},
			},
		},
	}

	// If an existing MachinePool is present, override the MachinePool generate name
	// re-using the existing name (this will help in reconcile).
	if currentMachinePool != nil && currentMachinePool.Object != nil {
		desiredMachinePoolObj.SetName(currentMachinePool.Object.Name)
	}

	// Apply annotations
	machinePoolAnnotations := util.MergeMap(machinePoolTopology.Metadata.Annotations, machinePoolBlueprint.Metadata.Annotations)
	// Ensure the annotations used to control the upgrade sequence are never propagated.
	delete(machinePoolAnnotations, clusterv1.ClusterTopologyHoldUpgradeSequenceAnnotation)
	delete(machinePoolAnnotations, clusterv1.ClusterTopologyDeferUpgradeAnnotation)
	desiredMachinePoolObj.SetAnnotations(machinePoolAnnotations)
	desiredMachinePoolObj.Spec.Template.Annotations = machinePoolAnnotations

	// Apply Labels
	// NOTE: On top of all the labels applied to managed objects we are applying the ClusterTopologyMachinePoolLabel
	// keeping track of the MachinePool name from the Topology; this will be used to identify the object in next reconcile loops.
	machinePoolLabels := util.MergeMap(machinePoolTopology.Metadata.Labels, machinePoolBlueprint.Metadata.Labels)
	if machinePoolLabels == nil {
		machinePoolLabels = map[string]string{}
	}
	machinePoolLabels[clusterv1.ClusterNameLabel] = s.Current.Cluster.Name
	machinePoolLabels[clusterv1.ClusterTopologyOwnedLabel] = ""
	machinePoolLabels[clusterv1.ClusterTopologyMachinePoolNameLabel] = machinePoolTopology.Name
	desiredMachinePoolObj.SetLabels(machinePoolLabels)

	// Also set the labels in .spec.template.labels so that they are propagated to
	// MachineSet.labels and MachineSet.spec.template.labels and thus to Machine.labels.
	// Note: the labels in MachineSet are used to properly cleanup templates when the MachineSet is deleted.
	desiredMachinePoolObj.Spec.Template.Labels = machinePoolLabels

	// Set the desired replicas.
	desiredMachinePoolObj.Spec.Replicas = machinePoolTopology.Replicas

	desiredMachinePool.Object = desiredMachinePoolObj

	return desiredMachinePool, nil
}

// computeMachinePoolVersion calculates the version of the desired machine pool.
// The version is calculated using the state of the current machine pools,
// the current control plane and the version defined in the topology.
func (g *generator) computeMachinePoolVersion(s *scope.Scope, machinePoolTopology clusterv1.MachinePoolTopology, currentMPState *scope.MachinePoolState) string {
	desiredVersion := s.Blueprint.Topology.Version
	// If creating a new machine pool, mark it as pending if the control plane is not
	// yet stable. Creating a new MP while the control plane is upgrading can lead to unexpected race conditions.
	// Example: join could fail if the load balancers are slow in detecting when CP machines are
	// being deleted.
	if currentMPState == nil || currentMPState.Object == nil {
		if !s.UpgradeTracker.ControlPlane.IsControlPlaneStable() || s.HookResponseTracker.IsBlocking(runtimehooksv1.AfterControlPlaneUpgrade) {
			s.UpgradeTracker.MachinePools.MarkPendingCreate(machinePoolTopology.Name)
		}
		return desiredVersion
	}

	// Get the current version of the machine pool.
	currentVersion := currentMPState.Object.Spec.Template.Spec.Version

	// Return early if the currentVersion is already equal to the desiredVersion
	// no further checks required.
	if currentVersion == desiredVersion {
		return currentVersion
	}

	// Return early if the upgrade for the MachinePool is deferred.
	if isMachinePoolDeferred(s.Blueprint.Topology, machinePoolTopology) {
		s.UpgradeTracker.MachinePools.MarkDeferredUpgrade(currentMPState.Object.Name)
		s.UpgradeTracker.MachinePools.MarkPendingUpgrade(currentMPState.Object.Name)
		return currentVersion
	}

	// Return early if the AfterControlPlaneUpgrade hook returns a blocking response.
	if s.HookResponseTracker.IsBlocking(runtimehooksv1.AfterControlPlaneUpgrade) {
		s.UpgradeTracker.MachinePools.MarkPendingUpgrade(currentMPState.Object.Name)
		return currentVersion
	}

	// Return early if the upgrade concurrency is reached.
	if s.UpgradeTracker.MachinePools.UpgradeConcurrencyReached() {
		s.UpgradeTracker.MachinePools.MarkPendingUpgrade(currentMPState.Object.Name)
		return currentVersion
	}

	// Return early if the Control Plane is not stable. Do not pick up the desiredVersion yet.
	// Return the current version of the machine pool. We will pick up the new version after the control
	// plane is stable.
	if !s.UpgradeTracker.ControlPlane.IsControlPlaneStable() {
		s.UpgradeTracker.MachinePools.MarkPendingUpgrade(currentMPState.Object.Name)
		return currentVersion
	}

	// Control plane and machine pools are stable.
	// Ready to pick up the topology version.
	s.UpgradeTracker.MachinePools.MarkUpgrading(currentMPState.Object.Name)
	return desiredVersion
}

// isMachinePoolDeferred returns true if the upgrade for the mpTopology is deferred.
// This is the case when either:
//   - the mpTopology has the ClusterTopologyDeferUpgradeAnnotation annotation.
//   - the mpTopology has the ClusterTopologyHoldUpgradeSequenceAnnotation annotation.
//   - another mp topology which is before mpTopology in the workers.machinePools list has the
//     ClusterTopologyHoldUpgradeSequenceAnnotation annotation.
func isMachinePoolDeferred(clusterTopology clusterv1.Topology, mpTopology clusterv1.MachinePoolTopology) bool {
	// If mpTopology has the ClusterTopologyDeferUpgradeAnnotation annotation => mp is deferred.
	if _, ok := mpTopology.Metadata.Annotations[clusterv1.ClusterTopologyDeferUpgradeAnnotation]; ok {
		return true
	}

	// If mpTopology has the ClusterTopologyHoldUpgradeSequenceAnnotation annotation => mp is deferred.
	if _, ok := mpTopology.Metadata.Annotations[clusterv1.ClusterTopologyHoldUpgradeSequenceAnnotation]; ok {
		return true
	}

	for _, mp := range clusterTopology.Workers.MachinePools {
		// If another mp topology with the ClusterTopologyHoldUpgradeSequenceAnnotation annotation
		// is found before the mpTopology => mp is deferred.
		if _, ok := mp.Metadata.Annotations[clusterv1.ClusterTopologyHoldUpgradeSequenceAnnotation]; ok {
			return true
		}

		// If mpTopology is found before a mp topology with the ClusterTopologyHoldUpgradeSequenceAnnotation
		// annotation => mp is not deferred.
		if mp.Name == mpTopology.Name {
			return false
		}
	}

	// This case should be impossible as mpTopology should have been found in workers.machinePools.
	return false
}

type templateToInput struct {
	template              *unstructured.Unstructured
	templateClonedFromRef *corev1.ObjectReference
	cluster               *clusterv1.Cluster
	nameGenerator         topologynames.NameGenerator
	currentObjectName     string
	labels                map[string]string
	annotations           map[string]string
	// OwnerRef is an optional OwnerReference to attach to the cloned object.
	ownerRef *metav1.OwnerReference
}

// templateToObject generates an object from a template, taking care
// of adding required labels (cluster, topology), annotations (clonedFrom)
// and assigning a meaningful name (or reusing current reference name).
func templateToObject(in templateToInput) (*unstructured.Unstructured, error) {
	// NOTE: The cluster label is added at creation time so this object could be read by the ClusterTopology
	// controller immediately after creation, even before other controllers are going to add the label (if missing).
	labels := map[string]string{}
	for k, v := range in.labels {
		labels[k] = v
	}
	labels[clusterv1.ClusterNameLabel] = in.cluster.Name
	labels[clusterv1.ClusterTopologyOwnedLabel] = ""

	// Generate the object from the template.
	// NOTE: OwnerRef can't be set at this stage; other controllers are going to add OwnerReferences when
	// the object is actually created.
	object, err := external.GenerateTemplate(&external.GenerateTemplateInput{
		Template:    in.template,
		TemplateRef: in.templateClonedFromRef,
		Namespace:   in.cluster.Namespace,
		Labels:      labels,
		Annotations: in.annotations,
		ClusterName: in.cluster.Name,
		OwnerRef:    in.ownerRef,
	})
	if err != nil {
		return nil, err
	}

	// Ensure the generated objects have a meaningful name.
	// NOTE: In case there is already a ref to this object in the Cluster, re-use the same name
	// in order to simplify comparison at later stages of the reconcile process.
	name, err := in.nameGenerator.GenerateName()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate name for %s", object.GetKind())
	}
	object.SetName(name)
	if in.currentObjectName != "" {
		object.SetName(in.currentObjectName)
	}

	return object, nil
}

// templateToTemplate generates a template from an existing template, taking care
// of adding required labels (cluster, topology), annotations (clonedFrom)
// and assigning a meaningful name (or reusing current reference name).
// NOTE: We are creating a copy of the ClusterClass template for each cluster so
// it is possible to add cluster specific information without affecting the original object.
func templateToTemplate(in templateToInput) (*unstructured.Unstructured, error) {
	template := &unstructured.Unstructured{}
	in.template.DeepCopyInto(template)

	// Remove all the info automatically assigned by the API server and not relevant from
	// the copy of the template.
	template.SetResourceVersion("")
	template.SetFinalizers(nil)
	template.SetUID("")
	template.SetSelfLink("")

	// Enforce the topology labels into the provided label set.
	// NOTE: The cluster label is added at creation time so this object could be read by the ClusterTopology
	// controller immediately after creation, even before other controllers are going to add the label (if missing).
	labels := template.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	for k, v := range in.labels {
		labels[k] = v
	}
	labels[clusterv1.ClusterNameLabel] = in.cluster.Name
	labels[clusterv1.ClusterTopologyOwnedLabel] = ""
	template.SetLabels(labels)

	// Enforce cloned from annotations and removes the kubectl last-applied-configuration annotation
	// because we don't want to propagate it to the cloned template objects.
	annotations := template.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	for k, v := range in.annotations {
		annotations[k] = v
	}
	annotations[clusterv1.TemplateClonedFromNameAnnotation] = in.templateClonedFromRef.Name
	annotations[clusterv1.TemplateClonedFromGroupKindAnnotation] = in.templateClonedFromRef.GroupVersionKind().GroupKind().String()
	delete(annotations, corev1.LastAppliedConfigAnnotation)
	template.SetAnnotations(annotations)

	// Set the owner reference.
	if in.ownerRef != nil {
		template.SetOwnerReferences([]metav1.OwnerReference{*in.ownerRef})
	}

	// Ensure the generated template gets a meaningful name.
	// NOTE: In case there is already an object ref to this template, it is required to re-use the same name
	// in order to simplify comparison at later stages of the reconcile process.
	name, err := in.nameGenerator.GenerateName()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate name for %s", template.GetKind())
	}
	template.SetName(name)
	if in.currentObjectName != "" {
		template.SetName(in.currentObjectName)
	}
	template.SetNamespace(in.cluster.Namespace)

	return template, nil
}

func computeMachineHealthCheck(ctx context.Context, healthCheckTarget client.Object, selector *metav1.LabelSelector, cluster *clusterv1.Cluster, mhcChecks clusterv1.MachineHealthCheckChecks, mhcRemediation clusterv1.MachineHealthCheckRemediation) *clusterv1.MachineHealthCheck {
	// Create a MachineHealthCheck with the spec given in the ClusterClass.
	mhc := &clusterv1.MachineHealthCheck{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineHealthCheck",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      healthCheckTarget.GetName(),
			Namespace: healthCheckTarget.GetNamespace(),
			Labels: map[string]string{
				clusterv1.ClusterTopologyOwnedLabel: "",
			},
			// Note: we are adding an ownerRef to Cluster so the MHC will be automatically garbage collected
			// in case deletion is triggered before an object reconcile happens.
			OwnerReferences: []metav1.OwnerReference{
				*ownerrefs.OwnerReferenceTo(cluster, clusterv1.GroupVersion.WithKind("Cluster")),
			},
		},
		Spec: clusterv1.MachineHealthCheckSpec{
			ClusterName: cluster.Name,
			Selector:    *selector,
			Checks:      mhcChecks,
			Remediation: mhcRemediation,
		},
	}

	// Default all fields in the MachineHealthCheck using the same function called in the webhook. This ensures the desired
	// state of the object won't be different from the current state due to webhook Defaulting.
	if err := (&webhooks.MachineHealthCheck{}).Default(ctx, mhc); err != nil {
		panic(err)
	}

	return mhc
}

func getOwnerReferenceFrom(obj, owner client.Object) *metav1.OwnerReference {
	for _, o := range obj.GetOwnerReferences() {
		if o.Kind == owner.GetObjectKind().GroupVersionKind().Kind && o.Name == owner.GetName() {
			return &o
		}
	}
	return nil
}

func cleanupCluster(cluster *clusterv1beta1.Cluster) *clusterv1beta1.Cluster {
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
	cluster.Status = clusterv1beta1.ClusterStatus{}
	return cluster
}
