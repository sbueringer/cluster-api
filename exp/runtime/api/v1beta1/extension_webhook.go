/*
Copyright 2022 The Kubernetes Authors.

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
	"fmt"
	"net/url"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/feature"
)

func (e *Extension) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(e).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-runtime-cluster-x-k8s-io-v1beta1-extension,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=runtime.cluster.x-k8s.io,resources=extensions,versions=v1beta1,name=validation.extensions.runtime.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-runtime-cluster-x-k8s-io-v1beta1-extension,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=runtime.cluster.x-k8s.io,resources=extensions,versions=v1beta1,name=default.extension.runtime.addons.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &Extension{}
var _ webhook.Defaulter = &Extension{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (e *Extension) Default() {
	// Default NamespaceSelector to an empty LabelSelector, which matches everything, if not set.
	if e.Spec.NamespaceSelector == nil {
		e.Spec.NamespaceSelector = &metav1.LabelSelector{}
	}

	// If a Service is defined default an unset port.
	if e.Spec.ClientConfig.Service != nil {
		if e.Spec.ClientConfig.Service.Port == nil {
			e.Spec.ClientConfig.Service.Port = pointer.Int32(8443)
		}
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (e *Extension) ValidateCreate() error {
	return e.validate(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (e *Extension) ValidateUpdate(old runtime.Object) error {
	newExtension, ok := old.(*Extension)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an Extension but got a %T", old))
	}
	return e.validate(newExtension)
}

func (e *Extension) validate(old *Extension) error {
	specPath := field.NewPath("spec")
	var allErrs field.ErrorList
	// NOTE: ExtensionConfig is behind the RuntimeSDK feature gate flag; the web hook
	// must prevent creating and updating objects old case the feature flag is disabled.
	if !feature.Gates.Enabled(feature.RuntimeSDK) {
		allErrs = append(allErrs, field.Forbidden(
			specPath,
			"can be set only if the RuntimeSDK feature flag is enabled",
		))
	}

	allErrs = append(allErrs, validateExtensionSpec(e)...)

	// ValidateUpdate if old is not nil.
	if old != nil {
		allErrs = append(allErrs, validateExtensionSpec(old)...)
	}
	if len(allErrs) > 0 {
		return apierrors.NewInvalid(clusterv1.GroupVersion.WithKind("Extension").GroupKind(), e.Name, allErrs)
	}
	return nil
}

func validateExtensionSpec(e *Extension) field.ErrorList {
	var allErrs field.ErrorList

	specPath := field.NewPath("spec")

	if e.Spec.ClientConfig.URL == nil && e.Spec.ClientConfig.Service == nil {
		allErrs = append(allErrs, field.Required(
			specPath.Child("clientConfig"),
			"either URL or Service must be defined",
		))
	}

	// Validate URl
	if e.Spec.ClientConfig.URL != nil {
		if _, err := url.ParseRequestURI(*e.Spec.ClientConfig.URL); err != nil {
			allErrs = append(allErrs, field.Invalid(
				specPath.Child("clientConfig", "url"),
				*e.Spec.ClientConfig.URL,
				"must be a valid URL e.g. https://example.com",
			))
		}
		// TODO: Decide handling of http/https prefixes.
	}

	// Validate Service if defined
	if e.Spec.ClientConfig.Service != nil {
		// Validate that the name is not empty and is a Valid RFC1123 name.
		if e.Spec.ClientConfig.Service.Name == "" {
			allErrs = append(allErrs, field.Required(
				specPath.Child("clientConfig", "service", "name"),
				"must not be empty",
			))
		}

		if errs := validation.IsDNS1123Subdomain(e.Spec.ClientConfig.Service.Name); len(errs) != 0 {
			allErrs = append(allErrs, field.Invalid(
				specPath.Child("clientConfig", "service", "name"),
				e.Spec.ClientConfig.Service.Name,
				"invalid name",
			))
		}

		// TODO: Decide if we need to validate Namespace (should it be equal to some other Namespace?)
		if e.Spec.ClientConfig.Service.Namespace == "" {
			allErrs = append(allErrs, field.Required(
				specPath.Child("clientConfig", "service", "namespace"),
				"must not be empty",
			))
		}

		if e.Spec.ClientConfig.Service.Path != nil {
			// TODO: Decide if we should be this strict on the path.
			path := *e.Spec.ClientConfig.Service.Path
			if _, err := url.ParseRequestURI(path); err != nil {
				allErrs = append(allErrs, field.Invalid(
					specPath.Child("clientConfig", "service", "path"),
					path,
					"must be a valid URL path e.g. /path/to/hook",
				))
			}
			if path[0:1] != "/" {
				allErrs = append(allErrs, field.Invalid(
					specPath.Child("clientConfig", "service", "path"),
					path,
					"must be a valid URL path e.g. /path/to/hook",
				))
			}
		}
		if e.Spec.ClientConfig.Service.Port != nil {
			if errs := validation.IsValidPortNum(int(*e.Spec.ClientConfig.Service.Port)); len(errs) != 0 {
				allErrs = append(allErrs, field.Invalid(
					specPath.Child("clientConfig", "service", "port"),
					*e.Spec.ClientConfig.Service.Port,
					"must be in range 1-65535 inclusive",
				))
			}
		}
	}
	if e.Spec.NamespaceSelector == nil {
		allErrs = append(allErrs, field.Required(
			specPath.Child("NamespaceSelector"),
			"must be defined",
		))
	}
	return allErrs
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (e *Extension) ValidateDelete() error {
	return nil
}
