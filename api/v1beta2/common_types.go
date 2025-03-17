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

package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ObjectMeta is metadata that all persisted resources must have, which includes all objects
// users must create. This is a copy of customizable fields from metav1.ObjectMeta.
//
// ObjectMeta is embedded in `Machine.Spec`, `MachineDeployment.Template` and `MachineSet.Template`,
// which are not top-level Kubernetes objects. Given that metav1.ObjectMeta has lots of special cases
// and read-only fields which end up in the generated CRD validation, having it as a subset simplifies
// the API and some issues that can impact user experience.
//
// During the [upgrade to controller-tools@v2](https://github.com/kubernetes-sigs/cluster-api/pull/1054)
// for v1alpha2, we noticed a failure would occur running Cluster API test suite against the new CRDs,
// specifically `spec.metadata.creationTimestamp in body must be of type string: "null"`.
// The investigation showed that `controller-tools@v2` behaves differently than its previous version
// when handling types from [metav1](k8s.io/apimachinery/pkg/apis/meta/v1) package.
//
// In more details, we found that embedded (non-top level) types that embedded `metav1.ObjectMeta`
// had validation properties, including for `creationTimestamp` (metav1.Time).
// The `metav1.Time` type specifies a custom json marshaller that, when IsZero() is true, returns `null`
// which breaks validation because the field isn't marked as nullable.
//
// In future versions, controller-tools@v2 might allow overriding the type and validation for embedded
// types. When that happens, this hack should be revisited.
type ObjectMeta struct {
	// labels is a map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Validate validates the labels and annotations in ObjectMeta.
func (metadata *ObjectMeta) Validate(parent *field.Path) field.ErrorList {
	allErrs := metav1validation.ValidateLabels(
		metadata.Labels,
		parent.Child("labels"),
	)
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(
		metadata.Annotations,
		parent.Child("annotations"),
	)...)
	return allErrs
}

// MachineReadinessGate contains the type of a Machine condition to be used as a readiness gate.
type MachineReadinessGate struct {
	// conditionType refers to a condition with matching type in the Machine's condition list.
	// If the conditions doesn't exist, it will be treated as unknown.
	// Note: Both Cluster API conditions or conditions added by 3rd party controllers can be used as readiness gates.
	// +required
	// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=316
	ConditionType string `json:"conditionType"`

	// polarity of the conditionType specified in this readinessGate.
	// Valid values are Positive, Negative and omitted.
	// When omitted, the default behaviour will be Positive.
	// A positive polarity means that the condition should report a true status under normal conditions.
	// A negative polarity means that the condition should report a false status under normal conditions.
	// +kubebuilder:validation:Enum=Positive;Negative
	// +optional
	Polarity ConditionPolarity `json:"polarity,omitempty"`
}

// MachineDeploymentStrategy describes how to replace existing machines
// with new ones.
type MachineDeploymentStrategy struct {
	// type of deployment. Allowed values are RollingUpdate and OnDelete.
	// The default is RollingUpdate.
	// +kubebuilder:validation:Enum=RollingUpdate;OnDelete
	// +optional
	Type MachineDeploymentStrategyType `json:"type,omitempty"`

	// rollingUpdate is the rolling update config params. Present only if
	// MachineDeploymentStrategyType = RollingUpdate.
	// +optional
	RollingUpdate *MachineRollingUpdateDeployment `json:"rollingUpdate,omitempty"`

	// remediation controls the strategy of remediating unhealthy machines
	// and how remediating operations should occur during the lifecycle of the dependant MachineSets.
	// +optional
	Remediation *RemediationStrategy `json:"remediation,omitempty"`
}

// MachineDeploymentStrategyType defines the type of MachineDeployment rollout strategies.
type MachineDeploymentStrategyType string

// MachineRollingUpdateDeployment is used to control the desired behavior of rolling update.
type MachineRollingUpdateDeployment struct {
	// maxUnavailable is the maximum number of machines that can be unavailable during the update.
	// Value can be an absolute number (ex: 5) or a percentage of desired
	// machines (ex: 10%).
	// Absolute number is calculated from percentage by rounding down.
	// This can not be 0 if MaxSurge is 0.
	// Defaults to 0.
	// Example: when this is set to 30%, the old MachineSet can be scaled
	// down to 70% of desired machines immediately when the rolling update
	// starts. Once new machines are ready, old MachineSet can be scaled
	// down further, followed by scaling up the new MachineSet, ensuring
	// that the total number of machines available at all times
	// during the update is at least 70% of desired machines.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// maxSurge is the maximum number of machines that can be scheduled above the
	// desired number of machines.
	// Value can be an absolute number (ex: 5) or a percentage of
	// desired machines (ex: 10%).
	// This can not be 0 if MaxUnavailable is 0.
	// Absolute number is calculated from percentage by rounding up.
	// Defaults to 1.
	// Example: when this is set to 30%, the new MachineSet can be scaled
	// up immediately when the rolling update starts, such that the total
	// number of old and new machines do not exceed 130% of desired
	// machines. Once old machines have been killed, new MachineSet can
	// be scaled up further, ensuring that total number of machines running
	// at any time during the update is at most 130% of desired machines.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`

	// deletePolicy defines the policy used by the MachineDeployment to identify nodes to delete when downscaling.
	// Valid values are "Random, "Newest", "Oldest"
	// When no value is supplied, the default DeletePolicy of MachineSet is used
	// +kubebuilder:validation:Enum=Random;Newest;Oldest
	// +optional
	DeletePolicy *string `json:"deletePolicy,omitempty"`
}

// RemediationStrategy allows to define how the MachineSet can control scaling operations.
type RemediationStrategy struct {
	// maxInFlight determines how many in flight remediations should happen at the same time.
	//
	// Remediation only happens on the MachineSet with the most current revision, while
	// older MachineSets (usually present during rollout operations) aren't allowed to remediate.
	//
	// Note: In general (independent of remediations), unhealthy machines are always
	// prioritized during scale down operations over healthy ones.
	//
	// MaxInFlight can be set to a fixed number or a percentage.
	// Example: when this is set to 20%, the MachineSet controller deletes at most 20% of
	// the desired replicas.
	//
	// If not set, remediation is limited to all machines (bounded by replicas)
	// under the active MachineSet's management.
	//
	// +optional
	MaxInFlight *intstr.IntOrString `json:"maxInFlight,omitempty"`
}

// MachineHealthCheckClass defines a MachineHealthCheck for a group of Machines.
type MachineHealthCheckClass struct {
	// unhealthyConditions contains a list of the conditions that determine
	// whether a node is considered unhealthy. The conditions are combined in a
	// logical OR, i.e. if any of the conditions is met, the node is unhealthy.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=100
	UnhealthyConditions []UnhealthyCondition `json:"unhealthyConditions,omitempty"`

	// maxUnhealthy specifies the maximum number of unhealthy machines allowed.
	// Any further remediation is only allowed if at most "maxUnhealthy" machines selected by
	// "selector" are not healthy.
	// +optional
	MaxUnhealthy *intstr.IntOrString `json:"maxUnhealthy,omitempty"`

	// unhealthyRange specifies the range of unhealthy machines allowed.
	// Any further remediation is only allowed if the number of machines selected by "selector" as not healthy
	// is within the range of "unhealthyRange". Takes precedence over maxUnhealthy.
	// Eg. "[3-5]" - This means that remediation will be allowed only when:
	// (a) there are at least 3 unhealthy machines (and)
	// (b) there are at most 5 unhealthy machines
	// +optional
	// +kubebuilder:validation:Pattern=^\[[0-9]+-[0-9]+\]$
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	UnhealthyRange *string `json:"unhealthyRange,omitempty"`

	// nodeStartupTimeout allows to set the maximum time for MachineHealthCheck
	// to consider a Machine unhealthy if a corresponding Node isn't associated
	// through a `Spec.ProviderID` field.
	//
	// The duration set in this field is compared to the greatest of:
	// - Cluster's infrastructure ready condition timestamp (if and when available)
	// - Control Plane's initialized condition timestamp (if and when available)
	// - Machine's infrastructure ready condition timestamp (if and when available)
	// - Machine's metadata creation timestamp
	//
	// Defaults to 10 minutes.
	// If you wish to disable this feature, set the value explicitly to 0.
	// +optional
	NodeStartupTimeout *metav1.Duration `json:"nodeStartupTimeout,omitempty"`

	// remediationTemplate is a reference to a remediation template
	// provided by an infrastructure provider.
	//
	// This field is completely optional, when filled, the MachineHealthCheck controller
	// creates a new object from the template referenced and hands off remediation of the machine to
	// a controller that lives outside of Cluster API.
	// +optional
	RemediationTemplate *corev1.ObjectReference `json:"remediationTemplate,omitempty"`
}

// UnhealthyCondition represents a Node condition type and value with a timeout
// specified as a duration.  When the named condition has been in the given
// status for at least the timeout value, a node is considered unhealthy.
type UnhealthyCondition struct {
	// type of Node condition
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MinLength=1
	// +required
	Type corev1.NodeConditionType `json:"type"`

	// status of the condition, one of True, False, Unknown.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MinLength=1
	// +required
	Status corev1.ConditionStatus `json:"status"`

	// timeout is the duration that a node must be in a given status for,
	// after which the node is considered unhealthy.
	// For example, with a value of "1h", the node must match the status
	// for at least 1 hour before being considered unhealthy.
	// +required
	Timeout metav1.Duration `json:"timeout"`
}
