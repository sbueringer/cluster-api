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

package v1alpha4

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha4 "sigs.k8s.io/cluster-api/internal/api/core/v1alpha4"
)

const (
	// ClusterResourceSetSecretType is the only accepted type of secret in resources.
	ClusterResourceSetSecretType corev1.SecretType = "addons.cluster.x-k8s.io/resource-set" //nolint:gosec

	// ClusterResourceSetFinalizer is added to the ClusterResourceSet object for additional cleanup logic on deletion.
	ClusterResourceSetFinalizer = "addons.cluster.x-k8s.io"
)

// ANCHOR: ClusterResourceSetSpec

// ClusterResourceSetSpec defines the desired state of ClusterResourceSet.
type ClusterResourceSetSpec struct {
	// clusterSelector is the label selector for Clusters. The Clusters that are
	// selected by this will be the ones affected by this ClusterResourceSet.
	// It must match the Cluster labels. This field is immutable.
	// Label selector cannot be empty.
	ClusterSelector metav1.LabelSelector `json:"clusterSelector"`

	// resources is a list of Secrets/ConfigMaps where each contains 1 or more resources to be applied to remote clusters.
	Resources []ResourceRef `json:"resources,omitempty"`

	// strategy is the strategy to be used during applying resources. Defaults to ApplyOnce. This field is immutable.
	// +kubebuilder:validation:Enum=ApplyOnce
	// +optional
	Strategy string `json:"strategy,omitempty"`
}

// ANCHOR_END: ClusterResourceSetSpec

// ClusterResourceSetResourceKind is a string representation of a ClusterResourceSet resource kind.
type ClusterResourceSetResourceKind string

// Define the ClusterResourceSetResourceKind constants.
const (
	SecretClusterResourceSetResourceKind    ClusterResourceSetResourceKind = "Secret"
	ConfigMapClusterResourceSetResourceKind ClusterResourceSetResourceKind = "ConfigMap"
)

// ResourceRef specifies a resource.
type ResourceRef struct {
	// name of the resource that is in the same namespace with ClusterResourceSet object.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// kind of the resource. Supported kinds are: Secrets and ConfigMaps.
	// +kubebuilder:validation:Enum=Secret;ConfigMap
	Kind string `json:"kind"`
}

// ClusterResourceSetStrategy is a string representation of a ClusterResourceSet Strategy.
type ClusterResourceSetStrategy string

const (
	// ClusterResourceSetStrategyApplyOnce is the default strategy a ClusterResourceSet strategy is assigned by
	// ClusterResourceSet controller after being created if not specified by user.
	ClusterResourceSetStrategyApplyOnce ClusterResourceSetStrategy = "ApplyOnce"
)

// SetTypedStrategy sets the Strategy field to the string representation of ClusterResourceSetStrategy.
func (c *ClusterResourceSetSpec) SetTypedStrategy(p ClusterResourceSetStrategy) {
	c.Strategy = string(p)
}

// ANCHOR: ClusterResourceSetStatus

// ClusterResourceSetStatus defines the observed state of ClusterResourceSet.
type ClusterResourceSetStatus struct {
	// observedGeneration reflects the generation of the most recently observed ClusterResourceSet.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// conditions defines current state of the ClusterResourceSet.
	// +optional
	Conditions clusterv1alpha4.Conditions `json:"conditions,omitempty"`
}

// ANCHOR_END: ClusterResourceSetStatus

// GetConditions returns the set of conditions for this object.
func (m *ClusterResourceSet) GetConditions() clusterv1alpha4.Conditions {
	return m.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (m *ClusterResourceSet) SetConditions(conditions clusterv1alpha4.Conditions) {
	m.Status.Conditions = conditions
}

// +kubebuilder:object:root=true
// +kubebuilder:unservedversion
// +kubebuilder:deprecatedversion
// +kubebuilder:resource:path=clusterresourcesets,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of ClusterResourceSet"

// ClusterResourceSet is the Schema for the clusterresourcesets API.
//
// Deprecated: This type will be removed in one of the next releases.
type ClusterResourceSet struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of ClusterResourceSet.
	Spec ClusterResourceSetSpec `json:"spec,omitempty"`
	// status is the observed state of ClusterResourceSet.
	Status ClusterResourceSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterResourceSetList contains a list of ClusterResourceSet.
//
// Deprecated: This type will be removed in one of the next releases.
type ClusterResourceSetList struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#lists-and-simple-kinds
	metav1.ListMeta `json:"metadata,omitempty"`
	// items is the list of ClusterResourceSets.
	Items []ClusterResourceSet `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &ClusterResourceSet{}, &ClusterResourceSetList{})
}
