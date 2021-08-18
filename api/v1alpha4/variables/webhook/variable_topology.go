package webhook

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/api/v1alpha4/variables/defaulting"
	"sigs.k8s.io/cluster-api/api/v1alpha4/variables/validation"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-variables-cluster-x-k8s-io-v1alpha4-cluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=clusters,versions=v1alpha4,name=validation-variables.cluster.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

type ClusterValidator struct {
	Client  ctrlclient.Reader
	decoder *admission.Decoder
}

// TODO(tbd): should we move all webhooks out of the API packages to reduce circular dependencies?
// Right now we cannot use envtest with webhooks because of that.

// Handle implements variable validation of a Cluster.
// Note: we cannot assume a specific order in which our webhooks are called, i.e. we cannot assume
// our "regular" validation webhook has already run before.
func (cv *ClusterValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	cluster := &clusterv1.Cluster{}

	if err := cv.decoder.Decode(req, cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if cluster.Spec.Topology == nil || len(cluster.Spec.Topology.Class) == 0 {
		return admission.Allowed("")
	}

	var clusterClass clusterv1.ClusterClass
	if err := cv.Client.Get(ctx, ctrlclient.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.Topology.Class,
	}, &clusterClass); err != nil {
		return admission.Errored(http.StatusInternalServerError, errors.Errorf("failed to get ClusterClass %s %s: %v", cluster.Namespace, cluster.Spec.Topology.Class, err.Error()))
	}

	allErrs := validation.ValidateVariableTopologies(cluster.Spec.Topology.Variables, clusterClass.Spec.Variables.Definitions, field.NewPath("spec", "topology", "variables"))

	if len(allErrs) == 0 {
		return admission.Allowed("")
	}

	result := apierrors.NewInvalid(clusterv1.GroupVersion.WithKind("Cluster").GroupKind(), cluster.Name, allErrs).ErrStatus
	return admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: false,
			Result:  &result,
		},
	}
}

// InjectDecoder injects the decoder.
func (cv *ClusterValidator) InjectDecoder(d *admission.Decoder) error {
	cv.decoder = d
	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/mutate-variables-cluster-x-k8s-io-v1alpha4-cluster,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=clusters,versions=v1alpha4,name=default-variables.cluster.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

type ClusterDefaulter struct {
	Client  ctrlclient.Reader
	decoder *admission.Decoder
}

// Handle implements variable defaulting of a Cluster.
func (cv *ClusterDefaulter) Handle(ctx context.Context, req admission.Request) admission.Response {
	cluster := &clusterv1.Cluster{}

	if err := cv.decoder.Decode(req, cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if cluster.Spec.Topology == nil || len(cluster.Spec.Topology.Class) == 0 {
		return admission.Allowed("")
	}

	var clusterClass clusterv1.ClusterClass
	if err := cv.Client.Get(ctx, ctrlclient.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.Topology.Class,
	}, &clusterClass); err != nil {
		return admission.Errored(http.StatusInternalServerError, errors.Errorf("failed to get ClusterClass %s %s: %v", cluster.Namespace, cluster.Spec.Topology.Class, err.Error()))
	}

	defaultedVariables, err := defaulting.DefaultVariableTopologies(cluster.Spec.Topology.Variables, clusterClass.Spec.Variables.Definitions)
	if err != nil {
		return admission.Denied(err.Error())
	}

	cluster.Spec.Topology.Variables = defaultedVariables

	marshaledCluster, err := json.Marshal(cluster)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledCluster)
}

// InjectDecoder injects the decoder.
func (cv *ClusterDefaulter) InjectDecoder(d *admission.Decoder) error {
	cv.decoder = d
	return nil
}
