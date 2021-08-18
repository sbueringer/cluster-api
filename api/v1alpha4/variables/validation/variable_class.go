package validation

import (
	"encoding/json"
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	structuraldefaulting "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/api/v1alpha4/variables"
)

func ValidateVariableClasses(variableClasses []variables.VariableDefinitionClass, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateVariableClassNamesUnique(variableClasses, fldPath)...)

	opts := validationOptions{
		allowDefaults:            true,
		disallowDefaultsReason:   "",
		requireValidPropertyType: true,
		requireStructuralSchema:  true,
		requirePrunedDefaults:    true,
	}

	for i, variableClass := range variableClasses {
		schema := &apiextensions.JSONSchemaProps{}
		if err := json.Unmarshal(variableClass.Schema.OpenAPIV3Schema.Raw, schema); err != nil {
			return field.ErrorList{field.Invalid(fldPath, string(variableClass.Schema.OpenAPIV3Schema.Raw),
				fmt.Sprintf("variable %q schema cannot be unmarshalled: %v", variableClass.Name, err))}
		}
		allErrs = append(allErrs, validateVariableClass(variableClass.Name, schema, opts, fldPath.Index(i))...)
	}

	return allErrs
}

func validateVariableClassNamesUnique(variableClasses []variables.VariableDefinitionClass, pathPrefix *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	variableNames := sets.NewString()
	for i, variableClass := range variableClasses {
		if variableNames.Has(variableClass.Name) {
			allErrs = append(allErrs,
				field.Invalid(
					pathPrefix.Index(i).Child("name"),
					variableClass.Name,
					fmt.Sprintf("variableClass name must be unique"),
				),
			)
		}
		variableNames.Insert(variableClass.Name)
	}

	return allErrs
}

// Everything below is initially copied from https://github.com/kubernetes/kubernetes/blob/5570a81040ead32b943facbb809a59a5d919287c/staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation/validation.go#L663
// Some parts that don't apply to our case were dropped.

var (
	openapiV3Types = sets.NewString("string", "number", "integer", "boolean", "array", "object")
)

// validationOptions groups several validation options, to avoid passing multiple bool parameters to methods
type validationOptions struct {
	// allowDefaults permits the validation schema to contain default attributes
	allowDefaults bool
	// disallowDefaultsReason gives a reason as to why allowDefaults is false (for better user feedback)
	disallowDefaultsReason string
	// requireValidPropertyType requires property types specified in the validation schema to be valid openapi v3 types
	requireValidPropertyType bool
	// requireStructuralSchema indicates that any schemas present must be structural
	requireStructuralSchema bool
	// requirePrunedDefaults indicates that defaults must be pruned
	requirePrunedDefaults bool
	// requireAtomicSetType indicates that the items type for a x-kubernetes-list-type=set list must be atomic.
	requireAtomicSetType bool
	// requireMapListKeysMapSetValidation indicates that:
	// 1. For x-kubernetes-list-type=map list, key fields are not nullable, and are required or have a default
	// 2. For x-kubernetes-list-type=map or x-kubernetes-list-type=set list, the whole item must not be nullable.
	requireMapListKeysMapSetValidation bool
}

func validateVariableClass(variableName string, schema *apiextensions.JSONSchemaProps, opts validationOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if schema == nil {
		return allErrs
	}

	// TODO(tbd-proposal): I think we should allow nullable for variables (even though it's not allowed on the root-level of CRDs)
	if schema.Nullable {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("schema.nullable"), fmt.Sprintf(`nullable cannot be true at the root`)))
	}

	openAPIV3Schema := &specStandardValidatorV3{
		allowDefaults:            opts.allowDefaults,
		disallowDefaultsReason:   "",
		requireValidPropertyType: true,
	}

	allErrs = append(allErrs, ValidateCustomResourceDefinitionOpenAPISchema(schema, fldPath.Child("schema"), openAPIV3Schema, true, &opts)...)

	if opts.requireStructuralSchema {
		// TODO(tbd): structural schema only allows type: object on the root level,
		// so we wrap the schema:
		// type: object
		// properties:
		//   <variable-name>: <variable-schema>
		wrappedSchema := &apiextensions.JSONSchemaProps{
			Type: "object",
			Properties: map[string]apiextensions.JSONSchemaProps{
				variableName: *schema,
			},
		}

		if ss, err := structuralschema.NewStructural(wrappedSchema); err != nil {
			// if the generic schema validation did its job, we should never get an error here. Hence, we hide it if there are validation errors already.
			if len(allErrs) == 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("schema"), "", err.Error()))
			}
		} else if validationErrors := structuralschema.ValidateStructural(fldPath.Child("schema"), ss); len(validationErrors) > 0 {
			allErrs = append(allErrs, validationErrors...)
		} else if validationErrors, err := structuraldefaulting.ValidateDefaults(fldPath.Child("schema"), ss, true, opts.requirePrunedDefaults); err != nil {
			// this should never happen
			allErrs = append(allErrs, field.Invalid(fldPath.Child("schema"), "", err.Error()))
		} else {
			allErrs = append(allErrs, validationErrors...)
		}
	}

	// if validation passed otherwise, make sure we can actually construct a schema validator.
	if len(allErrs) == 0 {
		if _, _, err := validation.NewSchemaValidator(&apiextensions.CustomResourceValidation{
			OpenAPIV3Schema: schema,
		}); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, "", fmt.Sprintf("error building validator: %v", err)))
		}
	}
	return allErrs
}

var metaFields = sets.NewString("metadata", "kind", "apiVersion")

// Note: everything below is copied from: https://github.com/kubernetes/kubernetes/blob/5570a81040ead32b943facbb809a59a5d919287c/staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation/validation.go

// specStandardValidator applies validations for different OpenAPI specification versions.
type specStandardValidator interface {
	validate(spec *apiextensions.JSONSchemaProps, fldPath *field.Path) field.ErrorList
	withForbiddenDefaults(reason string) specStandardValidator

	// insideResourceMeta returns true when validating either TypeMeta or ObjectMeta, from an embedded resource or on the top-level.
	insideResourceMeta() bool
	withInsideResourceMeta() specStandardValidator
}

// ValidateCustomResourceDefinitionOpenAPISchema statically validates
func ValidateCustomResourceDefinitionOpenAPISchema(schema *apiextensions.JSONSchemaProps, fldPath *field.Path, ssv specStandardValidator, isRoot bool, opts *validationOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	if schema == nil {
		return allErrs
	}

	allErrs = append(allErrs, ssv.validate(schema, fldPath)...)

	if schema.UniqueItems == true {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("uniqueItems"), "uniqueItems cannot be set to true since the runtime complexity becomes quadratic"))
	}

	// additionalProperties and properties are mutual exclusive because otherwise they
	// contradict Kubernetes' API convention to ignore unknown fields.
	//
	// In other words:
	// - properties are for structs,
	// - additionalProperties are for map[string]interface{}
	//
	// Note: when patternProperties is added to OpenAPI some day, this will have to be
	//       restricted like additionalProperties.
	if schema.AdditionalProperties != nil {
		if len(schema.Properties) != 0 {
			if schema.AdditionalProperties.Allows == false || schema.AdditionalProperties.Schema != nil {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("additionalProperties"), "additionalProperties and properties are mutual exclusive"))
			}
		}
		// Note: we forbid additionalProperties at resource root, both embedded and top-level.
		//       But further inside, additionalProperites is possible, e.g. for labels or annotations.
		subSsv := ssv
		if ssv.insideResourceMeta() {
			// we have to forbid defaults inside additionalProperties because pruning without actual value is ambiguous
			subSsv = ssv.withForbiddenDefaults("inside additionalProperties applying to object metadata")
		}
		allErrs = append(allErrs, ValidateCustomResourceDefinitionOpenAPISchema(schema.AdditionalProperties.Schema, fldPath.Child("additionalProperties"), subSsv, false, opts)...)
	}

	if len(schema.Properties) != 0 {
		for property, jsonSchema := range schema.Properties {
			subSsv := ssv

			if (isRoot || schema.XEmbeddedResource) && metaFields.Has(property) {
				// we recurse into the schema that applies to ObjectMeta.
				subSsv = ssv.withInsideResourceMeta()
				if isRoot {
					subSsv = subSsv.withForbiddenDefaults(fmt.Sprintf("in top-level %s", property))
				}
			}
			allErrs = append(allErrs, ValidateCustomResourceDefinitionOpenAPISchema(&jsonSchema, fldPath.Child("properties").Key(property), subSsv, false, opts)...)
		}
	}

	allErrs = append(allErrs, ValidateCustomResourceDefinitionOpenAPISchema(schema.Not, fldPath.Child("not"), ssv, false, opts)...)

	if len(schema.AllOf) != 0 {
		for i, jsonSchema := range schema.AllOf {
			allErrs = append(allErrs, ValidateCustomResourceDefinitionOpenAPISchema(&jsonSchema, fldPath.Child("allOf").Index(i), ssv, false, opts)...)
		}
	}

	if len(schema.OneOf) != 0 {
		for i, jsonSchema := range schema.OneOf {
			allErrs = append(allErrs, ValidateCustomResourceDefinitionOpenAPISchema(&jsonSchema, fldPath.Child("oneOf").Index(i), ssv, false, opts)...)
		}
	}

	if len(schema.AnyOf) != 0 {
		for i, jsonSchema := range schema.AnyOf {
			allErrs = append(allErrs, ValidateCustomResourceDefinitionOpenAPISchema(&jsonSchema, fldPath.Child("anyOf").Index(i), ssv, false, opts)...)
		}
	}

	if len(schema.Definitions) != 0 {
		for definition, jsonSchema := range schema.Definitions {
			allErrs = append(allErrs, ValidateCustomResourceDefinitionOpenAPISchema(&jsonSchema, fldPath.Child("definitions").Key(definition), ssv, false, opts)...)
		}
	}

	if schema.Items != nil {
		allErrs = append(allErrs, ValidateCustomResourceDefinitionOpenAPISchema(schema.Items.Schema, fldPath.Child("items"), ssv, false, opts)...)
		if len(schema.Items.JSONSchemas) != 0 {
			for i, jsonSchema := range schema.Items.JSONSchemas {
				allErrs = append(allErrs, ValidateCustomResourceDefinitionOpenAPISchema(&jsonSchema, fldPath.Child("items").Index(i), ssv, false, opts)...)
			}
		}
	}

	if schema.Dependencies != nil {
		for dependency, jsonSchemaPropsOrStringArray := range schema.Dependencies {
			allErrs = append(allErrs, ValidateCustomResourceDefinitionOpenAPISchema(jsonSchemaPropsOrStringArray.Schema, fldPath.Child("dependencies").Key(dependency), ssv, false, opts)...)
		}
	}

	if schema.XPreserveUnknownFields != nil && *schema.XPreserveUnknownFields == false {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("x-kubernetes-preserve-unknown-fields"), *schema.XPreserveUnknownFields, "must be true or undefined"))
	}

	if schema.XMapType != nil && schema.Type != "object" {
		if len(schema.Type) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must be object if x-kubernetes-map-type is specified"))
		} else {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), schema.Type, "must be object if x-kubernetes-map-type is specified"))
		}
	}

	if schema.XMapType != nil && *schema.XMapType != "atomic" && *schema.XMapType != "granular" {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("x-kubernetes-map-type"), *schema.XMapType, []string{"atomic", "granular"}))
	}

	if schema.XListType != nil && schema.Type != "array" {
		if len(schema.Type) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must be array if x-kubernetes-list-type is specified"))
		} else {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), schema.Type, "must be array if x-kubernetes-list-type is specified"))
		}
	} else if opts.requireAtomicSetType && schema.XListType != nil && *schema.XListType == "set" && schema.Items != nil && schema.Items.Schema != nil { // by structural schema items are present
		is := schema.Items.Schema
		switch is.Type {
		case "array":
			if is.XListType != nil && *is.XListType != "atomic" { // atomic is the implicit default behaviour if unset, hence != atomic is wrong
				allErrs = append(allErrs, field.Invalid(fldPath.Child("items").Child("x-kubernetes-list-type"), is.XListType, "must be atomic as item of a list with x-kubernetes-list-type=set"))
			}
		case "object":
			if is.XMapType == nil || *is.XMapType != "atomic" { // granular is the implicit default behaviour if unset, hence nil and != atomic are wrong
				allErrs = append(allErrs, field.Invalid(fldPath.Child("items").Child("x-kubernetes-map-type"), is.XListType, "must be atomic as item of a list with x-kubernetes-list-type=set"))
			}
		}
	}

	if schema.XListType != nil && *schema.XListType != "atomic" && *schema.XListType != "set" && *schema.XListType != "map" {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("x-kubernetes-list-type"), *schema.XListType, []string{"atomic", "set", "map"}))
	}

	if len(schema.XListMapKeys) > 0 {
		if schema.XListType == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("x-kubernetes-list-type"), "must be map if x-kubernetes-list-map-keys is non-empty"))
		} else if *schema.XListType != "map" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("x-kubernetes-list-type"), *schema.XListType, "must be map if x-kubernetes-list-map-keys is non-empty"))
		}
	}

	if schema.XListType != nil && *schema.XListType == "map" {
		if len(schema.XListMapKeys) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("x-kubernetes-list-map-keys"), "must not be empty if x-kubernetes-list-type is map"))
		}

		if schema.Items == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("items"), "must have a schema if x-kubernetes-list-type is map"))
		}

		if schema.Items != nil && schema.Items.Schema == nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("items"), schema.Items, "must only have a single schema if x-kubernetes-list-type is map"))
		}

		if schema.Items != nil && schema.Items.Schema != nil && schema.Items.Schema.Type != "object" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("items").Child("type"), schema.Items.Schema.Type, "must be object if parent array's x-kubernetes-list-type is map"))
		}

		if schema.Items != nil && schema.Items.Schema != nil && schema.Items.Schema.Type == "object" {
			keys := map[string]struct{}{}
			for _, k := range schema.XListMapKeys {
				if s, ok := schema.Items.Schema.Properties[k]; ok {
					if s.Type == "array" || s.Type == "object" {
						allErrs = append(allErrs, field.Invalid(fldPath.Child("items").Child("properties").Key(k).Child("type"), schema.Items.Schema.Type, "must be a scalar type if parent array's x-kubernetes-list-type is map"))
					}
				} else {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("x-kubernetes-list-map-keys"), schema.XListMapKeys, "entries must all be names of item properties"))
				}
				if _, ok := keys[k]; ok {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("x-kubernetes-list-map-keys"), schema.XListMapKeys, "must not contain duplicate entries"))
				}
				keys[k] = struct{}{}
			}
		}
	}

	if opts.requireMapListKeysMapSetValidation {
		allErrs = append(allErrs, validateMapListKeysMapSet(schema, fldPath)...)
	}

	return allErrs
}

func validateMapListKeysMapSet(schema *apiextensions.JSONSchemaProps, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if schema.Items == nil || schema.Items.Schema == nil {
		return nil
	}
	if schema.XListType == nil {
		return nil
	}
	if *schema.XListType != "set" && *schema.XListType != "map" {
		return nil
	}

	// set and map list items cannot be nullable
	if schema.Items.Schema.Nullable {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("items").Child("nullable"), "cannot be nullable when x-kubernetes-list-type is "+*schema.XListType))
	}

	switch *schema.XListType {
	case "map":
		// ensure all map keys are required or have a default
		isRequired := make(map[string]bool, len(schema.Items.Schema.Required))
		for _, required := range schema.Items.Schema.Required {
			isRequired[required] = true
		}

		for _, k := range schema.XListMapKeys {
			obj, ok := schema.Items.Schema.Properties[k]
			if !ok {
				// we validate that all XListMapKeys are existing properties in ValidateCustomResourceDefinitionOpenAPISchema, so skipping here is ok
				continue
			}

			if isRequired[k] == false && obj.Default == nil {
				allErrs = append(allErrs, field.Required(fldPath.Child("items").Child("properties").Key(k).Child("default"), "this property is in x-kubernetes-list-map-keys, so it must have a default or be a required property"))
			}

			if obj.Nullable {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("items").Child("properties").Key(k).Child("nullable"), "this property is in x-kubernetes-list-map-keys, so it cannot be nullable"))
			}
		}
	case "set":
		// no other set-specific validation
	}

	return allErrs
}

type specStandardValidatorV3 struct {
	allowDefaults            bool
	disallowDefaultsReason   string
	isInsideResourceMeta     bool
	requireValidPropertyType bool
}

func (v *specStandardValidatorV3) withForbiddenDefaults(reason string) specStandardValidator {
	clone := *v
	clone.disallowDefaultsReason = reason
	clone.allowDefaults = false
	return &clone
}

func (v *specStandardValidatorV3) withInsideResourceMeta() specStandardValidator {
	clone := *v
	clone.isInsideResourceMeta = true
	return &clone
}

func (v *specStandardValidatorV3) insideResourceMeta() bool {
	return v.isInsideResourceMeta
}

// validate validates against OpenAPI Schema v3.
func (v *specStandardValidatorV3) validate(schema *apiextensions.JSONSchemaProps, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if schema == nil {
		return allErrs
	}

	//
	// WARNING: if anything new is allowed below, NewStructural must be adapted to support it.
	//

	if v.requireValidPropertyType && len(schema.Type) > 0 && !openapiV3Types.Has(schema.Type) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), schema.Type, openapiV3Types.List()))
	}

	if schema.Default != nil && !v.allowDefaults {
		detail := "must not be set"
		if len(v.disallowDefaultsReason) > 0 {
			detail += " " + v.disallowDefaultsReason
		}
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("default"), detail))
	}

	if schema.ID != "" {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("id"), "id is not supported"))
	}

	if schema.AdditionalItems != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("additionalItems"), "additionalItems is not supported"))
	}

	if len(schema.PatternProperties) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("patternProperties"), "patternProperties is not supported"))
	}

	if len(schema.Definitions) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("definitions"), "definitions is not supported"))
	}

	if schema.Dependencies != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("dependencies"), "dependencies is not supported"))
	}

	if schema.Ref != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("$ref"), "$ref is not supported"))
	}

	if schema.Type == "null" {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("type"), "type cannot be set to null, use nullable as an alternative"))
	}

	if schema.Items != nil && len(schema.Items.JSONSchemas) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("items"), "items must be a schema object and not an array"))
	}

	if v.isInsideResourceMeta && schema.XEmbeddedResource {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("x-kubernetes-embedded-resource"), "must not be used inside of resource meta"))
	}

	return allErrs
}
