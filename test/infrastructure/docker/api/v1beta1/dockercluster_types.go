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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ClusterFinalizer allows DockerClusterReconciler to clean up resources associated with DockerCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "dockercluster.infrastructure.cluster.x-k8s.io"
)

// DockerClusterSpec defines the desired state of DockerCluster.
type DockerClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint APIEndpoint `json:"controlPlaneEndpoint"`

	// FailureDomains are usually not defined in the spec.
	// The docker provider is special since failure domains don't mean anything in a local docker environment.
	// Instead, the docker cluster controller will simply copy these into the Status and allow the Cluster API
	// controllers to do what they will with the defined failure domains.
	// +optional
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`

	// LoadBalancer allows defining configurations for the cluster load balancer.
	// +optional
	LoadBalancer DockerLoadBalancer `json:"loadBalancer,omitempty"`

	// SecondaryCidrBlock is the additional CIDR range to use for pod IPs.
	// Must be within the 100.64.0.0/10 or 198.19.0.0/16 range.
	// +optional
	SecondaryCidrBlock *string `json:"secondaryCidrBlock,omitempty"`

	Subnets1 DockerClusterSubnets1 `json:"subnets1,omitempty"`
	Subnets2 DockerClusterSubnets2 `json:"subnets2,omitempty"`
	Subnets3 DockerClusterSubnets3 `json:"subnets3,omitempty"`
	Subnets4 DockerClusterSubnets4 `json:"subnets4,omitempty"`
}

// DockerClusterSubnets1 .
// +listType=map
// +listMapKey=uuid
type DockerClusterSubnets1 []DockerClusterSubnets1Spec

// DockerClusterSubnets1Spec .
type DockerClusterSubnets1Spec struct {
	// +kubebuilder:default="dummy"
	UUID *string `json:"uuid,omitempty"`

	// ID defines a unique identifier to reference this resource.
	ID string `json:"id,omitempty"`

	// CidrBlock is the CIDR block to be used when the provider creates a managed VPC.
	CidrBlock string `json:"cidrBlock,omitempty"`

	TopologyField      string `json:"topologyField,omitempty"`
	DockerClusterField string `json:"dockerClusterField,omitempty"`
}

// DockerClusterSubnets2 .
// +listType=map
// +listMapKey=uuid
type DockerClusterSubnets2 []DockerClusterSubnets2Spec

// DockerClusterSubnets2Spec .
type DockerClusterSubnets2Spec struct {
	// +kubebuilder:default="dummy"
	UUID *string `json:"uuid,omitempty"`

	// ID defines a unique identifier to reference this resource.
	ID string `json:"id,omitempty"`

	// CidrBlock is the CIDR block to be used when the provider creates a managed VPC.
	CidrBlock string `json:"cidrBlock,omitempty"`

	TopologyField      string `json:"topologyField,omitempty"`
	DockerClusterField string `json:"dockerClusterField,omitempty"`
}

// DockerClusterSubnets3 .
// +listType=map
// +listMapKey=uuid
type DockerClusterSubnets3 []DockerClusterSubnets3Spec

// DockerClusterSubnets3Spec .
type DockerClusterSubnets3Spec struct {
	// +kubebuilder:default="dummy"
	UUID *string `json:"uuid,omitempty"`

	// ID defines a unique identifier to reference this resource.
	ID string `json:"id,omitempty"`

	// CidrBlock is the CIDR block to be used when the provider creates a managed VPC.
	CidrBlock string `json:"cidrBlock,omitempty"`

	TopologyField      string `json:"topologyField,omitempty"`
	DockerClusterField string `json:"dockerClusterField,omitempty"`
}

// DockerClusterSubnets4 .
type DockerClusterSubnets4 []DockerClusterSubnets4Spec

// DockerClusterSubnets4Spec .
type DockerClusterSubnets4Spec struct {
	// ID defines a unique identifier to reference this resource.
	ID string `json:"id,omitempty"`

	// CidrBlock is the CIDR block to be used when the provider creates a managed VPC.
	CidrBlock string `json:"cidrBlock,omitempty"`

	TopologyField      string `json:"topologyField,omitempty"`
	DockerClusterField string `json:"dockerClusterField,omitempty"`
}

func (s DockerClusterSubnets1) FindEqual(spec *DockerClusterSubnets1Spec) *DockerClusterSubnets1Spec {
	for _, x := range s {
		if (spec.ID != "" && x.ID == spec.ID) || (spec.CidrBlock == x.CidrBlock) {
			return &x
		}
	}
	return nil
}
func (s DockerClusterSubnets2) FindEqual(spec *DockerClusterSubnets2Spec) *DockerClusterSubnets2Spec {
	for _, x := range s {
		if (spec.ID != "" && x.ID == spec.ID) || (spec.CidrBlock == x.CidrBlock) {
			return &x
		}
	}
	return nil
}
func (s DockerClusterSubnets3) FindEqual(spec *DockerClusterSubnets3Spec) *DockerClusterSubnets3Spec {
	for _, x := range s {
		if (spec.ID != "" && x.ID == spec.ID) || (spec.CidrBlock == x.CidrBlock) {
			return &x
		}
	}
	return nil
}
func (s DockerClusterSubnets4) FindEqual(spec *DockerClusterSubnets4Spec) *DockerClusterSubnets4Spec {
	// FIXME: Note added spec.CidrBlock != "" here to avoid finding "equal" subnet with empty CIDRs.
	for _, x := range s {
		if (spec.ID != "" && x.ID == spec.ID) || (spec.CidrBlock != "" && spec.CidrBlock == x.CidrBlock) {
			return &x
		}
	}
	return nil
}

// DockerLoadBalancer allows defining configurations for the cluster load balancer.
type DockerLoadBalancer struct {
	// ImageMeta allows customizing the image used for the cluster load balancer.
	ImageMeta `json:",inline"`
}

// ImageMeta allows customizing the image used for components that are not
// originated from the Kubernetes/Kubernetes release process.
type ImageMeta struct {
	// ImageRepository sets the container registry to pull the haproxy image from.
	// if not set, "kindest" will be used instead.
	// +optional
	ImageRepository string `json:"imageRepository,omitempty"`

	// ImageTag allows to specify a tag for the haproxy image.
	// if not set, "v20210715-a6da3463" will be used instead.
	// +optional
	ImageTag string `json:"imageTag,omitempty"`
}

// DockerClusterStatus defines the observed state of DockerCluster.
type DockerClusterStatus struct {
	// Ready denotes that the docker cluster (infrastructure) is ready.
	// +optional
	Ready bool `json:"ready"`

	// FailureDomains don't mean much in CAPD since it's all local, but we can see how the rest of cluster API
	// will use this if we populate it.
	// +optional
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`

	// Conditions defines current service state of the DockerCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// APIEndpoint represents a reachable Kubernetes API endpoint.
type APIEndpoint struct {
	// Host is the hostname on which the API server is serving.
	Host string `json:"host"`

	// Port is the port on which the API server is serving.
	Port int `json:"port"`
}

// +kubebuilder:resource:path=dockerclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of DockerCluster"

// DockerCluster is the Schema for the dockerclusters API.
type DockerCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DockerClusterSpec   `json:"spec,omitempty"`
	Status DockerClusterStatus `json:"status,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (c *DockerCluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (c *DockerCluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// DockerClusterList contains a list of DockerCluster.
type DockerClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DockerCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DockerCluster{}, &DockerClusterList{})
}
