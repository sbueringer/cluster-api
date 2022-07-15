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
	"encoding/json"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (c *DockerCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	mgr.GetWebhookServer().Register("/mutate-infrastructure-cluster-x-k8s-io-v1beta1-dockercluster", &webhook.Admission{
		Handler: &DockerClusterMutator{},
	})
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-dockercluster,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=dockerclusters,versions=v1beta1,name=default.dockercluster.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// DockerClusterMutator validates KCP for replicas.
// +kubebuilder:object:generate=false
type DockerClusterMutator struct {
	decoder *admission.Decoder
}

// Handle will validate for number of replicas.
func (v *DockerClusterMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	dct := &DockerCluster{}

	err := v.decoder.DecodeRaw(req.Object, dct)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, errors.Wrapf(err, "failed to decode DockerCluster resource"))
	}

	oldDct := &DockerCluster{}
	if req.Operation == admissionv1.Update {
		if err := v.decoder.DecodeRaw(req.OldObject, oldDct); err != nil {
			return admission.Errored(http.StatusBadRequest, errors.Wrapf(err, "failed to decode DockerCluster resource"))
		}
	}

	for i, _ := range dct.Spec.Subnets1 {
		if dct.Spec.Subnets1[i].UUID == nil || *dct.Spec.Subnets1[i].UUID == "dummy" {
			dct.Spec.Subnets1[i].UUID = pointer.String(string(uuid.NewUUID()))
		}
	}
	for i, _ := range dct.Spec.Subnets2 {
		if dct.Spec.Subnets2[i].UUID == nil || *dct.Spec.Subnets2[i].UUID == "dummy" {
			dct.Spec.Subnets2[i].UUID = pointer.String(string(uuid.NewUUID()))
		}
	}
	for i, _ := range dct.Spec.Subnets3 {
		if dct.Spec.Subnets3[i].UUID == nil || *dct.Spec.Subnets3[i].UUID == "dummy" {
			if req.Operation == admissionv1.Create {
				dct.Spec.Subnets3[i].UUID = pointer.String(string(uuid.NewUUID()))
			}
			if req.Operation == admissionv1.Update {
				oldSubnet := oldDct.Spec.Subnets3.FindEqual(&dct.Spec.Subnets3[i])
				if oldSubnet != nil {
					dct.Spec.Subnets3[i].UUID = oldSubnet.UUID
				} else {
					dct.Spec.Subnets3[i].UUID = pointer.String(string(uuid.NewUUID()))
				}
			}
		}
	}

	if req.Operation == admissionv1.Update {
		updateOpts := &metav1.UpdateOptions{}
		if err := v.decoder.DecodeRaw(req.Options, updateOpts); err != nil {
			return admission.Errored(http.StatusBadRequest, errors.Wrapf(err, "failed to decode UpdateOptions resource"))
		}

		// Variant 1: custom merge logic.
		// Assumptions about CAPA behavior:
		// * If subnets are empty => CAPA creates default subnets and adds them to subnets spec
		//   => Fine as in that case topology controller doesn't set subnets at all.
		// * If SecondaryCIDR block is set => CAPA adds a corresponding subnet, if there is no subnet with that CIDR yet
		//   => 1. We have to preserve additional subnets with the SecondaryCIDR block.
		// * CAPA queries AWS for the subnet and syncs fields from AWS to subnet spec
		//   => 2. We have to carry over fields from the current AWSCluster
		// * If subnet.ID == "" => create subnet in AWS and sync fields from new subnet from AWS to subnet spec
		//   => 2. We have to carry over fields from the current AWSCluster
		if updateOpts.FieldManager == "capi-topology" {
			for i, _ := range dct.Spec.Subnets4 {
				oldSubnet := oldDct.Spec.Subnets4.FindEqual(&dct.Spec.Subnets4[i])
				if oldSubnet == nil {
					// Subnet has been newly added => nothing to carry-over.
					continue
				}

				// Subnet has been updated => 2. carry-over fields from old subnet, if they are not overwritten.
				if oldSubnet != nil {
					if dct.Spec.Subnets4[i].ID == "" {
						dct.Spec.Subnets4[i].ID = oldSubnet.ID
					}
					if dct.Spec.Subnets4[i].CidrBlock == "" {
						dct.Spec.Subnets4[i].CidrBlock = oldSubnet.CidrBlock
					}
					if dct.Spec.Subnets4[i].TopologyField == "" {
						dct.Spec.Subnets4[i].TopologyField = oldSubnet.TopologyField
					}
					if dct.Spec.Subnets4[i].DockerClusterField == "" {
						dct.Spec.Subnets4[i].DockerClusterField = oldSubnet.DockerClusterField
					}
				}
			}
		}
		// 1. We have to carry over additional subnets of the SecondaryCIDR block.
		for i, _ := range oldDct.Spec.Subnets4 {
			oldSubnet := oldDct.Spec.Subnets4[i]
			newSubnet := dct.Spec.Subnets4.FindEqual(&oldSubnet)
			if newSubnet != nil {
				// Subnet already exists in new => nothing to carry over.
				continue
			}

			// Subnet does not exist in new => carry-over subnet if its of the SecondaryCIDR block.
			// Note: That check is actually slightly more complicated (see CAPA subnets.go + cidr.SplitIntoSubnetsIPv4)
			if dct.Spec.SecondaryCidrBlock != nil && oldSubnet.CidrBlock == *dct.Spec.SecondaryCidrBlock {
				dct.Spec.Subnets4 = append(dct.Spec.Subnets4, oldSubnet)
			}
		}
	}

	// Background info:
	// Two subnets are defined equal to each other if their id is equal
	// or if they are in the same vpc and the cidr block is the same.

	// Alternative Variant: make it immutable
	//if updateOpts.FieldManager == "capi-topology" {
	//	dct.Spec.Subnets4 = oldDct.Spec.Subnets4
	//}

	// Create the patch
	marshalled, err := json.Marshal(dct)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshalled)
}

// InjectDecoder injects the decoder.
// DockerClusterMutator implements admission.DecoderInjector.
// A decoder will be automatically injected.
func (v *DockerClusterMutator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-dockercluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=dockerclusters,versions=v1beta1,name=validation.dockercluster.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &DockerCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (c *DockerCluster) ValidateCreate() error {
	if allErrs := validateDockerClusterSpec(c.Spec); len(allErrs) > 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("DockerCluster").GroupKind(), c.Name, allErrs)
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (c *DockerCluster) ValidateUpdate(old runtime.Object) error {
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (c *DockerCluster) ValidateDelete() error {
	return nil
}

func validateDockerClusterSpec(s DockerClusterSpec) field.ErrorList {
	return nil
}
