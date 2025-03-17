//go:build !ignore_autogenerated

/*
Copyright The Kubernetes Authors.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1beta2

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/cluster-api/errors"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIEndpoint) DeepCopyInto(out *APIEndpoint) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIEndpoint.
func (in *APIEndpoint) DeepCopy() *APIEndpoint {
	if in == nil {
		return nil
	}
	out := new(APIEndpoint)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Cluster) DeepCopyInto(out *Cluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Cluster.
func (in *Cluster) DeepCopy() *Cluster {
	if in == nil {
		return nil
	}
	out := new(Cluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Cluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterAvailabilityGate) DeepCopyInto(out *ClusterAvailabilityGate) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterAvailabilityGate.
func (in *ClusterAvailabilityGate) DeepCopy() *ClusterAvailabilityGate {
	if in == nil {
		return nil
	}
	out := new(ClusterAvailabilityGate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterControlPlaneStatus) DeepCopyInto(out *ClusterControlPlaneStatus) {
	*out = *in
	if in.DesiredReplicas != nil {
		in, out := &in.DesiredReplicas, &out.DesiredReplicas
		*out = new(int32)
		**out = **in
	}
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.UpToDateReplicas != nil {
		in, out := &in.UpToDateReplicas, &out.UpToDateReplicas
		*out = new(int32)
		**out = **in
	}
	if in.ReadyReplicas != nil {
		in, out := &in.ReadyReplicas, &out.ReadyReplicas
		*out = new(int32)
		**out = **in
	}
	if in.AvailableReplicas != nil {
		in, out := &in.AvailableReplicas, &out.AvailableReplicas
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterControlPlaneStatus.
func (in *ClusterControlPlaneStatus) DeepCopy() *ClusterControlPlaneStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterControlPlaneStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterList) DeepCopyInto(out *ClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Cluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterList.
func (in *ClusterList) DeepCopy() *ClusterList {
	if in == nil {
		return nil
	}
	out := new(ClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterNetwork) DeepCopyInto(out *ClusterNetwork) {
	*out = *in
	if in.APIServerPort != nil {
		in, out := &in.APIServerPort, &out.APIServerPort
		*out = new(int32)
		**out = **in
	}
	if in.Services != nil {
		in, out := &in.Services, &out.Services
		*out = new(NetworkRanges)
		(*in).DeepCopyInto(*out)
	}
	if in.Pods != nil {
		in, out := &in.Pods, &out.Pods
		*out = new(NetworkRanges)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterNetwork.
func (in *ClusterNetwork) DeepCopy() *ClusterNetwork {
	if in == nil {
		return nil
	}
	out := new(ClusterNetwork)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSpec) DeepCopyInto(out *ClusterSpec) {
	*out = *in
	if in.ClusterNetwork != nil {
		in, out := &in.ClusterNetwork, &out.ClusterNetwork
		*out = new(ClusterNetwork)
		(*in).DeepCopyInto(*out)
	}
	out.ControlPlaneEndpoint = in.ControlPlaneEndpoint
	if in.ControlPlaneRef != nil {
		in, out := &in.ControlPlaneRef, &out.ControlPlaneRef
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.InfrastructureRef != nil {
		in, out := &in.InfrastructureRef, &out.InfrastructureRef
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.Topology != nil {
		in, out := &in.Topology, &out.Topology
		*out = new(Topology)
		(*in).DeepCopyInto(*out)
	}
	if in.AvailabilityGates != nil {
		in, out := &in.AvailabilityGates, &out.AvailabilityGates
		*out = make([]ClusterAvailabilityGate, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSpec.
func (in *ClusterSpec) DeepCopy() *ClusterSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterStatus) DeepCopyInto(out *ClusterStatus) {
	*out = *in
	if in.FailureDomains != nil {
		in, out := &in.FailureDomains, &out.FailureDomains
		*out = make(FailureDomains, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.FailureReason != nil {
		in, out := &in.FailureReason, &out.FailureReason
		*out = new(errors.ClusterStatusError)
		**out = **in
	}
	if in.FailureMessage != nil {
		in, out := &in.FailureMessage, &out.FailureMessage
		*out = new(string)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.V1Beta2 != nil {
		in, out := &in.V1Beta2, &out.V1Beta2
		*out = new(ClusterV1Beta2Status)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterStatus.
func (in *ClusterStatus) DeepCopy() *ClusterStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterV1Beta2Status) DeepCopyInto(out *ClusterV1Beta2Status) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ControlPlane != nil {
		in, out := &in.ControlPlane, &out.ControlPlane
		*out = new(ClusterControlPlaneStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.Workers != nil {
		in, out := &in.Workers, &out.Workers
		*out = new(WorkersStatus)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterV1Beta2Status.
func (in *ClusterV1Beta2Status) DeepCopy() *ClusterV1Beta2Status {
	if in == nil {
		return nil
	}
	out := new(ClusterV1Beta2Status)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterVariable) DeepCopyInto(out *ClusterVariable) {
	*out = *in
	in.Value.DeepCopyInto(&out.Value)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterVariable.
func (in *ClusterVariable) DeepCopy() *ClusterVariable {
	if in == nil {
		return nil
	}
	out := new(ClusterVariable)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Condition) DeepCopyInto(out *Condition) {
	*out = *in
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Condition.
func (in *Condition) DeepCopy() *Condition {
	if in == nil {
		return nil
	}
	out := new(Condition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in Conditions) DeepCopyInto(out *Conditions) {
	{
		in := &in
		*out = make(Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Conditions.
func (in Conditions) DeepCopy() Conditions {
	if in == nil {
		return nil
	}
	out := new(Conditions)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControlPlaneTopology) DeepCopyInto(out *ControlPlaneTopology) {
	*out = *in
	in.Metadata.DeepCopyInto(&out.Metadata)
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.MachineHealthCheck != nil {
		in, out := &in.MachineHealthCheck, &out.MachineHealthCheck
		*out = new(MachineHealthCheckTopology)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeDrainTimeout != nil {
		in, out := &in.NodeDrainTimeout, &out.NodeDrainTimeout
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.NodeVolumeDetachTimeout != nil {
		in, out := &in.NodeVolumeDetachTimeout, &out.NodeVolumeDetachTimeout
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.NodeDeletionTimeout != nil {
		in, out := &in.NodeDeletionTimeout, &out.NodeDeletionTimeout
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.ReadinessGates != nil {
		in, out := &in.ReadinessGates, &out.ReadinessGates
		*out = make([]MachineReadinessGate, len(*in))
		copy(*out, *in)
	}
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = new(ControlPlaneVariables)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControlPlaneTopology.
func (in *ControlPlaneTopology) DeepCopy() *ControlPlaneTopology {
	if in == nil {
		return nil
	}
	out := new(ControlPlaneTopology)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControlPlaneVariables) DeepCopyInto(out *ControlPlaneVariables) {
	*out = *in
	if in.Overrides != nil {
		in, out := &in.Overrides, &out.Overrides
		*out = make([]ClusterVariable, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControlPlaneVariables.
func (in *ControlPlaneVariables) DeepCopy() *ControlPlaneVariables {
	if in == nil {
		return nil
	}
	out := new(ControlPlaneVariables)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FailureDomainSpec) DeepCopyInto(out *FailureDomainSpec) {
	*out = *in
	if in.Attributes != nil {
		in, out := &in.Attributes, &out.Attributes
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FailureDomainSpec.
func (in *FailureDomainSpec) DeepCopy() *FailureDomainSpec {
	if in == nil {
		return nil
	}
	out := new(FailureDomainSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in FailureDomains) DeepCopyInto(out *FailureDomains) {
	{
		in := &in
		*out = make(FailureDomains, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FailureDomains.
func (in FailureDomains) DeepCopy() FailureDomains {
	if in == nil {
		return nil
	}
	out := new(FailureDomains)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineDeploymentStrategy) DeepCopyInto(out *MachineDeploymentStrategy) {
	*out = *in
	if in.RollingUpdate != nil {
		in, out := &in.RollingUpdate, &out.RollingUpdate
		*out = new(MachineRollingUpdateDeployment)
		(*in).DeepCopyInto(*out)
	}
	if in.Remediation != nil {
		in, out := &in.Remediation, &out.Remediation
		*out = new(RemediationStrategy)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineDeploymentStrategy.
func (in *MachineDeploymentStrategy) DeepCopy() *MachineDeploymentStrategy {
	if in == nil {
		return nil
	}
	out := new(MachineDeploymentStrategy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineDeploymentTopology) DeepCopyInto(out *MachineDeploymentTopology) {
	*out = *in
	in.Metadata.DeepCopyInto(&out.Metadata)
	if in.FailureDomain != nil {
		in, out := &in.FailureDomain, &out.FailureDomain
		*out = new(string)
		**out = **in
	}
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.MachineHealthCheck != nil {
		in, out := &in.MachineHealthCheck, &out.MachineHealthCheck
		*out = new(MachineHealthCheckTopology)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeDrainTimeout != nil {
		in, out := &in.NodeDrainTimeout, &out.NodeDrainTimeout
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.NodeVolumeDetachTimeout != nil {
		in, out := &in.NodeVolumeDetachTimeout, &out.NodeVolumeDetachTimeout
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.NodeDeletionTimeout != nil {
		in, out := &in.NodeDeletionTimeout, &out.NodeDeletionTimeout
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.MinReadySeconds != nil {
		in, out := &in.MinReadySeconds, &out.MinReadySeconds
		*out = new(int32)
		**out = **in
	}
	if in.ReadinessGates != nil {
		in, out := &in.ReadinessGates, &out.ReadinessGates
		*out = make([]MachineReadinessGate, len(*in))
		copy(*out, *in)
	}
	if in.Strategy != nil {
		in, out := &in.Strategy, &out.Strategy
		*out = new(MachineDeploymentStrategy)
		(*in).DeepCopyInto(*out)
	}
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = new(MachineDeploymentVariables)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineDeploymentTopology.
func (in *MachineDeploymentTopology) DeepCopy() *MachineDeploymentTopology {
	if in == nil {
		return nil
	}
	out := new(MachineDeploymentTopology)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineDeploymentVariables) DeepCopyInto(out *MachineDeploymentVariables) {
	*out = *in
	if in.Overrides != nil {
		in, out := &in.Overrides, &out.Overrides
		*out = make([]ClusterVariable, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineDeploymentVariables.
func (in *MachineDeploymentVariables) DeepCopy() *MachineDeploymentVariables {
	if in == nil {
		return nil
	}
	out := new(MachineDeploymentVariables)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineHealthCheckClass) DeepCopyInto(out *MachineHealthCheckClass) {
	*out = *in
	if in.UnhealthyConditions != nil {
		in, out := &in.UnhealthyConditions, &out.UnhealthyConditions
		*out = make([]UnhealthyCondition, len(*in))
		copy(*out, *in)
	}
	if in.MaxUnhealthy != nil {
		in, out := &in.MaxUnhealthy, &out.MaxUnhealthy
		*out = new(intstr.IntOrString)
		**out = **in
	}
	if in.UnhealthyRange != nil {
		in, out := &in.UnhealthyRange, &out.UnhealthyRange
		*out = new(string)
		**out = **in
	}
	if in.NodeStartupTimeout != nil {
		in, out := &in.NodeStartupTimeout, &out.NodeStartupTimeout
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.RemediationTemplate != nil {
		in, out := &in.RemediationTemplate, &out.RemediationTemplate
		*out = new(v1.ObjectReference)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineHealthCheckClass.
func (in *MachineHealthCheckClass) DeepCopy() *MachineHealthCheckClass {
	if in == nil {
		return nil
	}
	out := new(MachineHealthCheckClass)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineHealthCheckTopology) DeepCopyInto(out *MachineHealthCheckTopology) {
	*out = *in
	if in.Enable != nil {
		in, out := &in.Enable, &out.Enable
		*out = new(bool)
		**out = **in
	}
	in.MachineHealthCheckClass.DeepCopyInto(&out.MachineHealthCheckClass)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineHealthCheckTopology.
func (in *MachineHealthCheckTopology) DeepCopy() *MachineHealthCheckTopology {
	if in == nil {
		return nil
	}
	out := new(MachineHealthCheckTopology)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachinePoolTopology) DeepCopyInto(out *MachinePoolTopology) {
	*out = *in
	in.Metadata.DeepCopyInto(&out.Metadata)
	if in.FailureDomains != nil {
		in, out := &in.FailureDomains, &out.FailureDomains
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NodeDrainTimeout != nil {
		in, out := &in.NodeDrainTimeout, &out.NodeDrainTimeout
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.NodeVolumeDetachTimeout != nil {
		in, out := &in.NodeVolumeDetachTimeout, &out.NodeVolumeDetachTimeout
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.NodeDeletionTimeout != nil {
		in, out := &in.NodeDeletionTimeout, &out.NodeDeletionTimeout
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.MinReadySeconds != nil {
		in, out := &in.MinReadySeconds, &out.MinReadySeconds
		*out = new(int32)
		**out = **in
	}
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = new(MachinePoolVariables)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachinePoolTopology.
func (in *MachinePoolTopology) DeepCopy() *MachinePoolTopology {
	if in == nil {
		return nil
	}
	out := new(MachinePoolTopology)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachinePoolVariables) DeepCopyInto(out *MachinePoolVariables) {
	*out = *in
	if in.Overrides != nil {
		in, out := &in.Overrides, &out.Overrides
		*out = make([]ClusterVariable, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachinePoolVariables.
func (in *MachinePoolVariables) DeepCopy() *MachinePoolVariables {
	if in == nil {
		return nil
	}
	out := new(MachinePoolVariables)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineReadinessGate) DeepCopyInto(out *MachineReadinessGate) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineReadinessGate.
func (in *MachineReadinessGate) DeepCopy() *MachineReadinessGate {
	if in == nil {
		return nil
	}
	out := new(MachineReadinessGate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineRollingUpdateDeployment) DeepCopyInto(out *MachineRollingUpdateDeployment) {
	*out = *in
	if in.MaxUnavailable != nil {
		in, out := &in.MaxUnavailable, &out.MaxUnavailable
		*out = new(intstr.IntOrString)
		**out = **in
	}
	if in.MaxSurge != nil {
		in, out := &in.MaxSurge, &out.MaxSurge
		*out = new(intstr.IntOrString)
		**out = **in
	}
	if in.DeletePolicy != nil {
		in, out := &in.DeletePolicy, &out.DeletePolicy
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineRollingUpdateDeployment.
func (in *MachineRollingUpdateDeployment) DeepCopy() *MachineRollingUpdateDeployment {
	if in == nil {
		return nil
	}
	out := new(MachineRollingUpdateDeployment)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkRanges) DeepCopyInto(out *NetworkRanges) {
	*out = *in
	if in.CIDRBlocks != nil {
		in, out := &in.CIDRBlocks, &out.CIDRBlocks
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkRanges.
func (in *NetworkRanges) DeepCopy() *NetworkRanges {
	if in == nil {
		return nil
	}
	out := new(NetworkRanges)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ObjectMeta) DeepCopyInto(out *ObjectMeta) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ObjectMeta.
func (in *ObjectMeta) DeepCopy() *ObjectMeta {
	if in == nil {
		return nil
	}
	out := new(ObjectMeta)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RemediationStrategy) DeepCopyInto(out *RemediationStrategy) {
	*out = *in
	if in.MaxInFlight != nil {
		in, out := &in.MaxInFlight, &out.MaxInFlight
		*out = new(intstr.IntOrString)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RemediationStrategy.
func (in *RemediationStrategy) DeepCopy() *RemediationStrategy {
	if in == nil {
		return nil
	}
	out := new(RemediationStrategy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Topology) DeepCopyInto(out *Topology) {
	*out = *in
	if in.RolloutAfter != nil {
		in, out := &in.RolloutAfter, &out.RolloutAfter
		*out = (*in).DeepCopy()
	}
	in.ControlPlane.DeepCopyInto(&out.ControlPlane)
	if in.Workers != nil {
		in, out := &in.Workers, &out.Workers
		*out = new(WorkersTopology)
		(*in).DeepCopyInto(*out)
	}
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = make([]ClusterVariable, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Topology.
func (in *Topology) DeepCopy() *Topology {
	if in == nil {
		return nil
	}
	out := new(Topology)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UnhealthyCondition) DeepCopyInto(out *UnhealthyCondition) {
	*out = *in
	out.Timeout = in.Timeout
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UnhealthyCondition.
func (in *UnhealthyCondition) DeepCopy() *UnhealthyCondition {
	if in == nil {
		return nil
	}
	out := new(UnhealthyCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkersStatus) DeepCopyInto(out *WorkersStatus) {
	*out = *in
	if in.DesiredReplicas != nil {
		in, out := &in.DesiredReplicas, &out.DesiredReplicas
		*out = new(int32)
		**out = **in
	}
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.UpToDateReplicas != nil {
		in, out := &in.UpToDateReplicas, &out.UpToDateReplicas
		*out = new(int32)
		**out = **in
	}
	if in.ReadyReplicas != nil {
		in, out := &in.ReadyReplicas, &out.ReadyReplicas
		*out = new(int32)
		**out = **in
	}
	if in.AvailableReplicas != nil {
		in, out := &in.AvailableReplicas, &out.AvailableReplicas
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkersStatus.
func (in *WorkersStatus) DeepCopy() *WorkersStatus {
	if in == nil {
		return nil
	}
	out := new(WorkersStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkersTopology) DeepCopyInto(out *WorkersTopology) {
	*out = *in
	if in.MachineDeployments != nil {
		in, out := &in.MachineDeployments, &out.MachineDeployments
		*out = make([]MachineDeploymentTopology, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.MachinePools != nil {
		in, out := &in.MachinePools, &out.MachinePools
		*out = make([]MachinePoolTopology, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkersTopology.
func (in *WorkersTopology) DeepCopy() *WorkersTopology {
	if in == nil {
		return nil
	}
	out := new(WorkersTopology)
	in.DeepCopyInto(out)
	return out
}
