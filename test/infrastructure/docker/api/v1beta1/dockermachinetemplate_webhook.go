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
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/cluster-api/util/webhooks"
)

const dockerMachineTemplateImmutableMsg = "DockerMachineTemplate spec.template.spec field is immutable. Please create a new resource instead."

func (m *DockerMachineTemplateWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&DockerMachineTemplate{}).
		WithValidator(m).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-dockermachinetemplate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=dockermachinetemplates,versions=v1beta1,name=validation.dockermachinetemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// DockerMachineTemplateWebhook implements a validating webhook for DockerMachineTemplate.
type DockerMachineTemplateWebhook struct{}

var _ webhook.CustomValidator = &DockerMachineTemplateWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (m *DockerMachineTemplateWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (m *DockerMachineTemplateWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	var allErrs field.ErrorList

	newDockerMachineTemplate, ok := newObj.(*DockerMachineTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a Cluster but got a %T", newObj))
	}
	oldDockerMachineTemplate, ok := oldObj.(*DockerMachineTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a DockerMachineTemplate but got a %T", oldObj))
	}

	req, _ := admission.RequestFromContext(ctx)

	if webhooks.ShouldSkipImmutabilityChecks(req, newDockerMachineTemplate) &&
		!reflect.DeepEqual(newDockerMachineTemplate.Spec.Template.Spec, oldDockerMachineTemplate.Spec.Template.Spec) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "template", "spec"), m, dockerMachineTemplateImmutableMsg))
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("DockerMachineTemplate").GroupKind(), newDockerMachineTemplate.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (m *DockerMachineTemplateWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
