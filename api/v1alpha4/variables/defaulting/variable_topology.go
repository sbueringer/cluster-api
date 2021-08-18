package defaulting

import (
	"encoding/json"
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	structuraldefaulting "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	structuralpruning "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	"sigs.k8s.io/cluster-api/api/v1alpha4/variables"
)

func DefaultVariableTopologies(variableTopologies []variables.VariableTopology, variableClasses []variables.VariableDefinitionClass) ([]variables.VariableTopology, error) {
	variableValues := map[string]interface{}{}
	for _, variableTopology := range variableTopologies {
		var value interface{}
		if err := json.Unmarshal(variableTopology.Value.Raw, &value); err != nil {
			return nil, err
		}
		variableValues[variableTopology.Name] = value
	}

	variableSchemas := map[string]*apiextensions.JSONSchemaProps{}
	for _, variableClass := range variableClasses {
		// Parse schema in variableClass.
		schema := &apiextensions.JSONSchemaProps{}
		if err := json.Unmarshal(variableClass.Schema.OpenAPIV3Schema.Raw, schema); err != nil {
			return nil, err
		}
		variableSchemas[variableClass.Name] = schema

		// Add a variable to variables if:
		// * we found a schema for a variable which does not exist in the Cluster and
		// * the schema has defaulting on the root-level.
		if _, ok := variableValues[variableClass.Name]; !ok && schema.Default != nil {
			variableValues[variableClass.Name] = nil
		}
	}

	// Verify that all variables from the Cluster exist in the CLusterClass and default the variables.
	var defaultedVariableTopologies []variables.VariableTopology
	for variableName, variableValue := range variableValues {
		// Get schema.
		schema, ok := variableSchemas[variableName]
		if !ok {
			return nil, fmt.Errorf("variable %q does not have a schema", variableName)
		}

		// TODO(tbd): structural schema defaulting does not work with scalar values,
		// so we wrap the schema and the variable in an object.
		// type: object
		// properties:
		//   <variable-name>: <variable-schema>
		wrappedSchema := &apiextensions.JSONSchemaProps{
			Type: "object",
			Properties: map[string]apiextensions.JSONSchemaProps{
				variableName: *schema,
			},
		}
		wrappedVariable := map[string]interface{}{
			variableName: variableValue,
		}

		// Run structural schema defaulting.
		ss, err := structuralschema.NewStructural(wrappedSchema)
		if err != nil {
			return nil, err
		}

		// TODO(tbd-proposal): just added pruning here for now, do we want it, the validation does not care otherwise
		structuralpruning.Prune(wrappedVariable, ss, false)
		structuraldefaulting.PruneNonNullableNullsWithoutDefaults(wrappedVariable, ss)

		structuraldefaulting.Default(wrappedVariable, ss)

		// Marshal the defaulted value..
		defaultedVariableValue := wrappedVariable[variableName]
		res, err := json.Marshal(defaultedVariableValue)
		if err != nil {
			return nil, err
		}

		// append the defaulted variable instance.
		defaultedVariableTopologies = append(defaultedVariableTopologies, variables.VariableTopology{
			Name: variableName,
			Value: apiextensionsv1.JSON{
				Raw: res,
			},
		})
	}

	return defaultedVariableTopologies, nil
}
