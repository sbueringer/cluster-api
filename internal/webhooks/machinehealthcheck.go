package webhooks

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var (
	// Minimum time allowed for a node to start up.
	minNodeStartupTimeout = metav1.Duration{Duration: 30 * time.Second}
	// We allow users to disable the nodeStartupTimeout by setting the duration to 0.
	disabledNodeStartupTimeout = clusterv1.ZeroDuration
)

// SetMinNodeStartupTimeout allows users to optionally set a custom timeout
// for the validation webhook.
//
// This function is mostly used within envtest (integration tests), and should
// never be used in a production environment.
func SetMinNodeStartupTimeout(d metav1.Duration) {
	minNodeStartupTimeout = d
}

// ValidateCommonMachineHealthCheckFields validates UnhealthyConditions NodeStartupTimeout, MaxUnhealthy, and RemediationTemplate of the MHC.
// It is also used to help validate other types which define MachineHealthChecks such as MachineHealthCheckClass and MachineHealthCheckTopology.
func ValidateCommonMachineHealthCheckFields(m *clusterv1.MachineHealthCheck, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if m.Spec.NodeStartupTimeout != nil &&
		m.Spec.NodeStartupTimeout.Seconds() != disabledNodeStartupTimeout.Seconds() &&
		m.Spec.NodeStartupTimeout.Seconds() < minNodeStartupTimeout.Seconds() {
		allErrs = append(
			allErrs,
			field.Invalid(fldPath.Child("nodeStartupTimeout"), m.Spec.NodeStartupTimeout.String(), "must be at least 30s"),
		)
	}
	if m.Spec.MaxUnhealthy != nil {
		if _, err := intstr.GetScaledValueFromIntOrPercent(m.Spec.MaxUnhealthy, 0, false); err != nil {
			allErrs = append(
				allErrs,
				field.Invalid(fldPath.Child("maxUnhealthy"), m.Spec.MaxUnhealthy, fmt.Sprintf("must be either an int or a percentage: %v", err.Error())),
			)
		}
	}
	if m.Spec.RemediationTemplate != nil && m.Spec.RemediationTemplate.Namespace != m.Namespace {
		allErrs = append(
			allErrs,
			field.Invalid(
				fldPath.Child("remediationTemplate", "namespace"),
				m.Spec.RemediationTemplate.Namespace,
				"must match metadata.namespace",
			),
		)
	}

	if len(m.Spec.UnhealthyConditions) == 0 {
		allErrs = append(allErrs, field.Forbidden(
			fldPath.Child("unhealthyConditions"),
			"must have at least one entry",
		))
	}

	return allErrs
}
