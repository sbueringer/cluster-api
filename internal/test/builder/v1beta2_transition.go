/*
Copyright 2024 The Kubernetes Authors.

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

package builder

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var (
	// TransitionV1beta2GroupVersion is group version used for test CRDs used for validating the v1beta2 transition.
	TransitionV1beta2GroupVersion = schema.GroupVersion{Group: "transition.v1beta2.cluster.x-k8s.io", Version: "v1beta1"}

	// Phase0ObjKind is the kind for the Phase0Obj type.
	Phase0ObjKind = "Phase0Obj"
	// Phase0ObjCRD is a Phase0Obj CRD.
	Phase0ObjCRD = phase0ObjCRD(TransitionV1beta2GroupVersion.WithKind(Phase0ObjKind))

	// Phase1ObjKind is the kind for the Phase1Obj type.
	Phase1ObjKind = "Phase1Obj"
	// Phase1ObjCRD is a Phase1Obj CRD.
	Phase1ObjCRD = phase1ObjCRD(TransitionV1beta2GroupVersion.WithKind(Phase1ObjKind))

	// Phase2ObjKind is the kind for the Phase2Obj type.
	Phase2ObjKind = "Phase2Obj"
	// Phase2ObjCRD is a Phase2Obj CRD.
	Phase2ObjCRD = phase2ObjCRD(TransitionV1beta2GroupVersion.WithKind(Phase2ObjKind))

	// Phase3ObjKind is the kind for the Phase3Obj type.
	Phase3ObjKind = "Phase3Obj"
	// Phase3ObjCRD is a Phase3Obj CRD.
	Phase3ObjCRD = phase3ObjCRD(TransitionV1beta2GroupVersion.WithKind(Phase3ObjKind))

	// schemeBuilder is used to add go types to the GroupVersionKind scheme.
	schemeBuilder = runtime.NewSchemeBuilder(addTransitionV1beta2Types)

	// AddTransitionV1beta2ToScheme adds the types for validating the transition to v1Beta2 in this group-version to the given scheme.
	AddTransitionV1beta2ToScheme = schemeBuilder.AddToScheme
)

func addTransitionV1beta2Types(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(TransitionV1beta2GroupVersion,
		&Phase0Obj{}, &Phase0ObjList{},
		&Phase1Obj{}, &Phase1ObjList{},
		&Phase2Obj{}, &Phase2ObjList{},
		&Phase3Obj{}, &Phase3ObjList{},
	)
	metav1.AddToGroupVersion(scheme, TransitionV1beta2GroupVersion)
	return nil
}

// Phase0ObjList is a list of Phase0Obj.
// +kubebuilder:object:root=true
type Phase0ObjList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Phase0Obj `json:"items"`
}

// Phase0Obj defines an object with clusterv1.Conditions.
// +kubebuilder:object:root=true
type Phase0Obj struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              Phase0ObjSpec   `json:"spec,omitempty"`
	Status            Phase0ObjStatus `json:"status,omitempty"`
}

// Phase0ObjSpec defines the spec of a Phase0Obj.
type Phase0ObjSpec struct {
	Foo string `json:"foo,omitempty"`
}

// Phase0ObjStatus defines the status of a Phase0Obj.
type Phase0ObjStatus struct {
	Bar        string               `json:"bar,omitempty"`
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (o *Phase0Obj) GetConditions() clusterv1.Conditions {
	return o.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (o *Phase0Obj) SetConditions(conditions clusterv1.Conditions) {
	o.Status.Conditions = conditions
}

func phase0ObjCRD(gvk schema.GroupVersionKind) *apiextensionsv1.CustomResourceDefinition {
	return generateCRD(gvk, map[string]apiextensionsv1.JSONSchemaProps{
		"metadata": {
			// NOTE: in CRD there is only a partial definition of metadata schema.
			// Ref https://github.com/kubernetes-sigs/controller-tools/blob/59485af1c1f6a664655dad49543c474bb4a0d2a2/pkg/crd/gen.go#L185
			Type: "object",
		},
		"spec": {
			Type: "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{
				"foo": {Type: "string"},
			},
		},
		"status": {
			Type: "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{
				"bar": {Type: "string"},
				"conditions": {
					Type: "array",
					Items: &apiextensionsv1.JSONSchemaPropsOrArray{
						Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"type":               {Type: "string"},
								"status":             {Type: "string"},
								"severity":           {Type: "string"},
								"reason":             {Type: "string"},
								"message":            {Type: "string"},
								"lastTransitionTime": {Type: "string"},
							},
							Required: []string{"type", "status"},
						},
					},
				},
			},
		},
	})
}

// Phase1ObjList is a list of Phase1Obj.
// +kubebuilder:object:root=true
type Phase1ObjList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Phase1Obj `json:"items"`
}

// Phase1Obj defines an object with conditions and experimental conditions.
// +kubebuilder:object:root=true
type Phase1Obj struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              Phase1ObjSpec   `json:"spec,omitempty"`
	Status            Phase1ObjStatus `json:"status,omitempty"`
}

// Phase1ObjSpec defines the spec of a Phase1Obj.
type Phase1ObjSpec struct {
	Foo string `json:"foo,omitempty"`
}

// Phase1ObjStatus defines the status of a Phase1Obj.
type Phase1ObjStatus struct {
	Bar                    string               `json:"bar,omitempty"`
	Conditions             clusterv1.Conditions `json:"conditions,omitempty"`
	ExperimentalConditions []metav1.Condition   `json:"experimentalConditions,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (o *Phase1Obj) GetConditions() clusterv1.Conditions {
	return o.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (o *Phase1Obj) SetConditions(conditions clusterv1.Conditions) {
	o.Status.Conditions = conditions
}

func phase1ObjCRD(gvk schema.GroupVersionKind) *apiextensionsv1.CustomResourceDefinition {
	return generateCRD(gvk, map[string]apiextensionsv1.JSONSchemaProps{
		"metadata": {
			// NOTE: in CRD there is only a partial definition of metadata schema.
			// Ref https://github.com/kubernetes-sigs/controller-tools/blob/59485af1c1f6a664655dad49543c474bb4a0d2a2/pkg/crd/gen.go#L185
			Type: "object",
		},
		"spec": {
			Type: "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{
				"foo": {Type: "string"},
			},
		},
		"status": {
			Type: "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{
				"bar": {Type: "string"},
				"conditions": {
					Type: "array",
					Items: &apiextensionsv1.JSONSchemaPropsOrArray{
						Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"type":               {Type: "string"},
								"status":             {Type: "string"},
								"severity":           {Type: "string"},
								"reason":             {Type: "string"},
								"message":            {Type: "string"},
								"lastTransitionTime": {Type: "string"},
							},
							Required: []string{"type", "status"},
						},
					},
				},
				"experimentalConditions": {
					Type: "array",
					Items: &apiextensionsv1.JSONSchemaPropsOrArray{
						Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"type":               {Type: "string"},
								"status":             {Type: "string"},
								"observedGeneration": {Type: "integer"},
								"reason":             {Type: "string"},
								"message":            {Type: "string"},
								"lastTransitionTime": {Type: "string"},
							},
							Required: []string{"type", "status"},
						},
					},
				},
			},
		},
	})
}

// Phase2ObjList is a list of Phase2Obj.
// +kubebuilder:object:root=true
type Phase2ObjList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Phase2Obj `json:"items"`
}

// Phase2Obj defines an object with conditions and back compatibility conditions.
// +kubebuilder:object:root=true
type Phase2Obj struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              Phase2ObjSpec   `json:"spec,omitempty"`
	Status            Phase2ObjStatus `json:"status,omitempty"`
}

// Phase2ObjSpec defines the spec of a Phase2Obj.
type Phase2ObjSpec struct {
	Foo string `json:"foo,omitempty"`
}

// Phase2ObjStatus defines the status of a Phase2Obj.
type Phase2ObjStatus struct {
	Bar               string                           `json:"bar,omitempty"`
	Conditions        []metav1.Condition               `json:"conditions,omitempty"`
	BackCompatibility Phase2ObjStatusBackCompatibility `json:"backCompatibility,omitempty"`
}

// Phase2ObjStatusBackCompatibility defines the status.BackCompatibility of a Phase2Obj.
type Phase2ObjStatusBackCompatibility struct {
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (o *Phase2Obj) GetConditions() clusterv1.Conditions {
	return o.Status.BackCompatibility.Conditions
}

// SetConditions sets the conditions on this object.
func (o *Phase2Obj) SetConditions(conditions clusterv1.Conditions) {
	o.Status.BackCompatibility.Conditions = conditions
}

func phase2ObjCRD(gvk schema.GroupVersionKind) *apiextensionsv1.CustomResourceDefinition {
	return generateCRD(gvk, map[string]apiextensionsv1.JSONSchemaProps{
		"metadata": {
			// NOTE: in CRD there is only a partial definition of metadata schema.
			// Ref https://github.com/kubernetes-sigs/controller-tools/blob/59485af1c1f6a664655dad49543c474bb4a0d2a2/pkg/crd/gen.go#L185
			Type: "object",
		},
		"spec": {
			Type: "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{
				"foo": {Type: "string"},
			},
		},
		"status": {
			Type: "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{
				"bar": {Type: "string"},
				"conditions": {
					Type: "array",
					Items: &apiextensionsv1.JSONSchemaPropsOrArray{
						Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"type":               {Type: "string"},
								"status":             {Type: "string"},
								"observedGeneration": {Type: "integer"},
								"reason":             {Type: "string"},
								"message":            {Type: "string"},
								"lastTransitionTime": {Type: "string"},
							},
							Required: []string{"type", "status"},
						},
					},
				},
				"backCompatibility": {
					Type: "object",
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"conditions": {
							Type: "array",
							Items: &apiextensionsv1.JSONSchemaPropsOrArray{
								Schema: &apiextensionsv1.JSONSchemaProps{
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"type":               {Type: "string"},
										"status":             {Type: "string"},
										"severity":           {Type: "string"},
										"reason":             {Type: "string"},
										"message":            {Type: "string"},
										"lastTransitionTime": {Type: "string"},
									},
									Required: []string{"type", "status"},
								},
							},
						},
					},
				},
			},
		},
	})
}

// Phase3ObjList is a list of Phase3Obj.
// +kubebuilder:object:root=true
type Phase3ObjList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Phase3Obj `json:"items"`
}

// Phase3Obj defines an object with metav1.conditions.
// +kubebuilder:object:root=true
type Phase3Obj struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              Phase3ObjSpec   `json:"spec,omitempty"`
	Status            Phase3ObjStatus `json:"status,omitempty"`
}

// Phase3ObjSpec defines the spec of a Phase3Obj.
type Phase3ObjSpec struct {
	Foo string `json:"foo,omitempty"`
}

// Phase3ObjStatus defines the status of a Phase3Obj.
type Phase3ObjStatus struct {
	Bar        string             `json:"bar,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func phase3ObjCRD(gvk schema.GroupVersionKind) *apiextensionsv1.CustomResourceDefinition {
	return generateCRD(gvk, map[string]apiextensionsv1.JSONSchemaProps{
		"metadata": {
			// NOTE: in CRD there is only a partial definition of metadata schema.
			// Ref https://github.com/kubernetes-sigs/controller-tools/blob/59485af1c1f6a664655dad49543c474bb4a0d2a2/pkg/crd/gen.go#L185
			Type: "object",
		},
		"spec": {
			Type: "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{
				"foo": {Type: "string"},
			},
		},
		"status": {
			Type: "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{
				"bar": {Type: "string"},
				"conditions": {
					Type: "array",
					Items: &apiextensionsv1.JSONSchemaPropsOrArray{
						Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"type":               {Type: "string"},
								"status":             {Type: "string"},
								"observedGeneration": {Type: "integer"},
								"reason":             {Type: "string"},
								"message":            {Type: "string"},
								"lastTransitionTime": {Type: "string"},
							},
							Required: []string{"type", "status"},
						},
					},
				},
			},
		},
	})
}
