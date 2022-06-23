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

// Package webhooks provides customized admission webhook interfaces to simplify
// the implemention of validating webhooks which fulfill the requirements of the
// topology controller.
package webhooks

import (
	"context"
	goerrors "errors"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// TopologyAwareValidator defines functions for validating an operation topology aware.
// This interface is the same as the admission.Validator interface except the skipImmutabilityChecks
// parameter for the ValidateUpdate method, which can be used by implementations to
// not block requests due to immutability of fields.
// Ref: https://github.com/kubernetes-sigs/controller-runtime/blob/c48baad70c539a2efb8dfe8850434ecc721c1ee1/pkg/webhook/admission/validator.go
// This is required for the topology controller for being able to successfully detect
// the upcoming changes for template rotation.
type TopologyAwareValidator interface {
	runtime.Object
	ValidateCreate() error
	ValidateUpdate(skipImmutabilityChecks bool, old runtime.Object) error
	ValidateDelete() error
}

// TopologyAwareValidatingWebhookFor creates a new Webhook for validating the provided type.
func TopologyAwareValidatingWebhookFor(validator TopologyAwareValidator) *admission.Webhook {
	return &admission.Webhook{
		Handler: &topologyAwareValidatingHandler{validator: validator},
	}
}

type topologyAwareValidatingHandler struct {
	validator TopologyAwareValidator
	decoder   *admission.Decoder
}

var _ admission.DecoderInjector = &topologyAwareValidatingHandler{}

// InjectDecoder injects the decoder into a validatingHandler.
func (h *topologyAwareValidatingHandler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

// Handle handles admission requests.
// The topology controller uses server-side apply dry-run requests to determine if
// an object and/or ownership of fields gets changed. It expects validating webhooks
// to not block Update requests due to immutability of fields to calculate the expected
// changes of an object.
// It passes the skipImmutabilityChecks bool to the ValidateUpdate method which is only
// true if the request is a dry run and to distinguish topology controller's dry-run
// from other actors it checks for the TopologyDryRunAnnotation on the object.
func (h *topologyAwareValidatingHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	if h.validator == nil {
		panic("validator should never be nil")
	}

	// Get the object in the request
	obj := h.validator.DeepCopyObject().(TopologyAwareValidator)
	if req.Operation == v1.Create {
		err := h.decoder.Decode(req, obj)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		err = obj.ValidateCreate()
		if err != nil {
			var apiStatus apierrors.APIStatus
			if goerrors.As(err, &apiStatus) {
				return validationResponseFromStatus(false, apiStatus.Status())
			}
			return admission.Denied(err.Error())
		}
	}

	if req.Operation == v1.Update {
		oldObj := obj.DeepCopyObject()

		err := h.decoder.DecodeRaw(req.Object, obj)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		err = h.decoder.DecodeRaw(req.OldObject, oldObj)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		// Determine if ValidateUpdate should skip immutability checks.
		skipImmutabilityChecks := shouldSkipImmutabilityChecks(req, obj)

		err = obj.ValidateUpdate(skipImmutabilityChecks, oldObj)
		if err != nil {
			var apiStatus apierrors.APIStatus
			if goerrors.As(err, &apiStatus) {
				return validationResponseFromStatus(false, apiStatus.Status())
			}
			return admission.Denied(err.Error())
		}
	}

	if req.Operation == v1.Delete {
		// In reference to PR: https://github.com/kubernetes/kubernetes/pull/76346
		// OldObject contains the object being deleted
		err := h.decoder.DecodeRaw(req.OldObject, obj)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		err = obj.ValidateDelete()
		if err != nil {
			var apiStatus apierrors.APIStatus
			if goerrors.As(err, &apiStatus) {
				return validationResponseFromStatus(false, apiStatus.Status())
			}
			return admission.Denied(err.Error())
		}
	}

	return admission.Allowed("")
}

// validationResponseFromStatus returns a response for admitting a request with provided Status object.
func validationResponseFromStatus(allowed bool, status metav1.Status) admission.Response {
	resp := admission.Response{
		AdmissionResponse: v1.AdmissionResponse{
			Allowed: allowed,
			Result:  &status,
		},
	}
	return resp
}

// shouldSkipImmutabilityChecks returns true if it is a dry-run request and the object has the
// TopologyDryRunAnnotation annotation set, false otherwise.
func shouldSkipImmutabilityChecks(req admission.Request, obj TopologyAwareValidator) bool {
	// Check if the request is a dry-run
	if req.DryRun == nil || !*req.DryRun {
		return false
	}

	// Cast the object to a metav1.Object to get access to the annotations.
	objMeta, ok := obj.(metav1.Object)
	if !ok {
		return false
	}

	// Check for the TopologyDryRunAnnotation
	annotations := objMeta.GetAnnotations()
	if annotations == nil {
		return false
	}
	if _, ok := annotations[clusterv1.TopologyDryRunAnnotation]; ok {
		return true
	}
	return false
}
