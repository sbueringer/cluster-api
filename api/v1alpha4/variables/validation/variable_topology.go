package validation

import (
	"encoding/json"
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/api/v1alpha4/variables"
)

func ValidateVariableTopologies(variableTopologies []variables.VariableTopology, variableClasses []variables.VariableDefinitionClass, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	variableValues := map[string]interface{}{}
	for _, variableTopology := range variableTopologies {
		var value interface{}
		if err := json.Unmarshal(variableTopology.Value.Raw, &value); err != nil {
			return field.ErrorList{field.Invalid(fldPath, string(variableTopology.Value.Raw),
				fmt.Sprintf("variables could not be parsed: %v", err))}
		}
		variableValues[variableTopology.Name] = value
	}

	variableSchemas := map[string]*apiextensions.JSONSchemaProps{}
	for _, variableClass := range variableClasses {
		// Parse schema in variableClass.
		schema := &apiextensions.JSONSchemaProps{}
		if err := json.Unmarshal(variableClass.Schema.OpenAPIV3Schema.Raw, schema); err != nil {
			return field.ErrorList{field.Invalid(fldPath, string(variableClass.Schema.OpenAPIV3Schema.Raw),
				fmt.Sprintf("variable %q schema cannot be unmarshalled: %v", variableClass.Name, err))}
		}
		variableSchemas[variableClass.Name] = schema
	}

	// Check that all required variables exist.
	for _, variableClass := range variableClasses {
		if _, ok := variableValues[variableClass.Name]; !ok && variableClass.Required {
			allErrs = append(allErrs, field.Invalid(fldPath, variableTopologies,
				fmt.Sprintf("required variable %q does not exist", variableClass.Name)))
		}
	}

	// Verify that all variables from the Cluster exist in the CLusterClass and match the schema.
	for variableName, variableValue := range variableValues {
		// Get schema.
		schema, ok := variableSchemas[variableName]
		if !ok {
			return field.ErrorList{field.Invalid(fldPath, variableTopologies,
				fmt.Sprintf("variable %q does not exist in the ClusterClass", variableName))}
		}

		// Create validator for schema.
		validator, _, err := validation.NewSchemaValidator(&apiextensions.CustomResourceValidation{
			OpenAPIV3Schema: schema,
		})
		if err != nil {
			return field.ErrorList{field.Invalid(fldPath, schema,
				fmt.Sprintf("variable %q does not have a valid schema in the ClusterClass: %v", variableName, err))}
		}

		// Validate variable.
		allErrs = append(allErrs, validation.ValidateCustomResource(fldPath, variableValue, validator)...)
	}
	return allErrs
}
