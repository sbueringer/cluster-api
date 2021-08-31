package variables

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// VariableDefinitionClass defines a variable which can
// be configured in the Cluster topology and used in patches.
// +k8s:deepcopy-gen=true
type VariableDefinitionClass struct {
	// Name of the variable.
	Name string `json:"name"`

	// Required specifies if the variable is required.
	// Note: this applies to the variable as a whole and thus the
	// top-level object defined in the schema. If nested fields are
	// required, this will be specified inside the schema.
	Required bool `json:"required"`

	// Schema defines the schema of the variable.

	// For this PoC, we use runtime.RawExtension which is just unmarshaled into apiextensionsv1.JSONSchemaProps.
	//
	// TODO(tbd-proposal): Considerations regarding embedding JSONSchemaProps:
	// * We want to embed our own "custom" copy of JSONSchemaProps, because:
	//   * We cannot embed apiextensionsv1.JSONSchemaProps directly because there are fields (like "JSONSchemaPropsOrArray")
	//     which don't have JSON tags, which breaks the manifest generation.
	//   * We don't want to use runtime.RawExtension, because we're able to define the type so we don't want to use an "untyped" struct.
	//
	// * Our custom copy could be either identical to apiextensionsv1.JSONSchemaProps or customized. I think we should customize because:
	//   * the apiextensionsv1.JSONSchemaProps contain a lot of fields which are not even supported by Kubernetes which is not especially user-friendly.
	//   * this gets even worse when we only implement a subset of the Kubernetes JSONSchemaProps feature set.
	//
	//   Note: For usage with structural schema we will need apiextensionsv1.JSONSchemaProps eventually. So if we introduce our own
	//         structs we will have to convert from our JSONSchemaProps to apiextensionsv1.JSONSchemaProps. Structural schema is important for us
	//         for a more strict schema validation, (possibly) pruning and defaulting.
	//   Note: apiextensionsv1.JSONSchemaProps contains *float64, to generate manifests via controller-gen we have to either:
	//         * use: "crd:crdVersions=v1,allowDangerousTypes=true"
	//         * use our own type with custom serialization

	// TODO(tbd-proposal): We might want to drop support for some of the fields of the Kubernetes JSONSchemaProps. This could be
	//                     done either by blocking them in the webhook or dropping the fields in case we're using our own JSONSchemaProps struct.
	//                     Fields to drop: (TBD)
	// * Fields which are already blocked by Kubernetes:
	//   * validation.go (via CRD and OpenAPI schema v3 validation):
	//     * ID, AdditionalItems, PatternProperties, Definitions, Dependencies, Ref, UniqueItems, Items.JSONSchemas
	//   * convert.go (NewStructural / validateUnsupportedFields)
	//     * ID, AdditionalItems, PatternProperties, Definitions, Dependencies, Ref, Schema
	//
	//   * To keep it as simple as possible, we might want to drop metadata/documentation fields which are not strictly necessary:
	//     * Description
	//     * Title
	//     * External docs
	//     * Example
	//
	//   * Kubernetes specific “features”
	//     * XPreserveUnknownFields (pruning): could be useful to support unspecified objects, Should we support unknown fields, what about pruning?
	//     * XEmbeddedResource: doesn't make sense imho
	//     * XIntOrString: would require anyOf, allOf in our struct. So I would only support it when we support anyOf, allOf, cleanly validated via structural schema, otherwise probably hard to validate
	//     * XListMapKeys: doesn't make sense in our case
	//     * XListType: doesn't make sense in our case
	//     * XMapType: doesn't make sense in our case
	//
	// * So, what is left?
	//     * Basic types: configured via Type
	//
	//     * Complex Types:
	//       * Object: via Properties
	//       * Array: via Items
	//       * Map: via AdditionalProperties:
	//         * They are intended for map[string]interface{}. While maps with structs as values are against the Kubernetes
	//         * API conventions we also would need them to support e.g. map[string]string (for labels/annotations)
	//         * See comment in variable_class.go:175-180
	//       * Composition: via OneOf, AnyOf
	//         * Imho we can only make this safe via structural schema (we shouldn't reinvent it)
	//         * Unlikely that we already need them / have use cases or them.
	//
	//     * Defaulting: via Default
	//
	//     * Basic validation via:
	//       * Format, Maximum, ExclusiveMaximum, Minimum, ExclusiveMinimum, MaxLength, MinLength
	//       * Pattern, MaxItems, MinItems, MultipleOf, Enum, MaxProperties, MinProperties
	//       * Required
	//
	//     * AllOf, Not: not sure for what we would need them
	//
	//     * Nullable: we would need Nullable to be able to set null values (otherwise they are pruned, if we prune)
	//       * https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#defaulting-and-nullable
	//
	Schema VariableDefinitionSchemaClass `json:"schema"`
}

// VariableDefinitionSchemaClass defines the schema of a variable.
// +k8s:deepcopy-gen=true
type VariableDefinitionSchemaClass struct{
	// OpenAPIV3Schema defines the schema of a variable via OpenAPI v3
	// schema. The schema is a subset of the schema used in
	// Kubernetes CRDs.
	// +kubebuilder:pruning:PreserveUnknownFields
	OpenAPIV3Schema runtime.RawExtension `json:"openAPIV3Schema"`
}

// VariableTopology can be used to customize the Cluster through
// patches. It must comply to the corresponding
// VariableDefinitionClass defined in the ClusterClass.
// +k8s:deepcopy-gen=true
type VariableTopology struct {
	// Name of the variable.
	Name string `json:"name"`

	// Value of the variable.
	// Note: the value will be validated against the schema
	// of the corresponding VariableDefinitionClass from the ClusterClass.
	Value apiextensionsv1.JSON `json:"value"`
}

// PatchDefinitionClass defines a patch which is applied to customize the referenced templates.
// +k8s:deepcopy-gen=true
type PatchDefinitionClass struct {
	// Target defines on which templates the patch should be applied.
	Target PatchDefinitionTargetClass `json:"target"`

	// JSONPatches defines the patches which should be applied on the target.
	// Note: patches will be applied in the order of the array.
	JSONPatches []PatchDefinitionJSONPatchClass `json:"jsonPatches"`
}

// PatchDefinitionTargetClass defines on which templates the patch should be applied.
type PatchDefinitionTargetClass struct {
	// APIVersion filters templates by apiVersion.
	APIVersion string `json:"apiVersion"`

	// Kind filters templates by kind.
	Kind string `json:"kind"`

	// Selector filters templates based on where they are referenced.
	// Note: If selector is not set, all templates matching APIVersion and Kind are patched.
	Selector *PatchDefinitionTargetSelectorClass `json:"selector"`
}

// PatchDefinitionTargetSelectorClass selects templates based on where they are referenced.
type PatchDefinitionTargetSelectorClass struct {
	// MatchResources selects templates based on where they are referenced.
	MatchResources PatchDefinitionTargetSelectorResourcesClass `json:"matchResources"`
}

// PatchDefinitionTargetSelectorResourcesClass selects templates based on where they are referenced.
// Note: The results of controlPlane and machineDeploymentClass are ORed.
type PatchDefinitionTargetSelectorResourcesClass struct {
	// ControlPlane selects templates referenced in .spec.ControlPlane.
	ControlPlane bool `json:"controlPlane"`

	// MachineDeploymentClass selects templates referenced in specific MachineDeploymentClasses in
	// .spec.workers.machineDeployments.
	MachineDeploymentClass PatchDefinitionTargetSelectorResourcesMachineDeploymentClass `json:"machineDeploymentClass"`
}

// PatchDefinitionTargetSelectorResourcesMachineDeploymentClass selects templates referenced
// in specific MachineDeploymentClasses in .spec.workers.machineDeployments.
type PatchDefinitionTargetSelectorResourcesMachineDeploymentClass struct {
	// Names selects templates by class names.
	Names []string `json:"names"`
}

// PatchDefinitionJSONPatchClass defines a JSON patch.
type PatchDefinitionJSONPatchClass struct {
	// Op defines the operation of the patch.
	// Note: valid values are: add, replace and remove.
	Op string `json:"op"`

	// Path defines the path of the patch.
	Path string `json:"path"`

	// Value defines the value of the patch.
	// Note: Either Value or ValueFrom is required for add and replace
	// operations. Only one of them is allowed to be set at the same time.
	Value apiextensionsv1.JSON `json:"value"`

	// ValueFrom defines the value of the patch.
	// Note: Either Value or ValueFrom is required for add and replace
	// operations. Only one of them is allowed to be set at the same time.
	ValueFrom PatchDefinitionJSONPatchValueClass `json:"valueFrom"`
}

// PatchDefinitionJSONPatchValueClass defines the value of a patch.
// Note: only one of the fields is allowed to be set at the same time.
type PatchDefinitionJSONPatchValueClass struct {
	// Variable is the variable to be used as value.
	// Variable can be one of the variables defined in .spec.variables or a builtin variable.
	// Note: builtin variables are described further in the CAPI book.
	Variable string `json:"variable"`

	// Template is the Go template to be used to calculate the value.
	// A template can reference variables defined in .spec.variables and builtin variables.
	// Note: builtin variables are described further in the CAPI book.
	// Note: the template must evaluate to a valid YAML or JSON value.
	Template string `json:"template"`
}
