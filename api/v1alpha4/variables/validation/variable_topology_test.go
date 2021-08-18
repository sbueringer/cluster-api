package validation

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/api/v1alpha4/variables"
)

func Test_ValidateVariableTopologies(t *testing.T) {
	tests := []struct {
		name               string
		variableClasses    string
		variableTopologies string
		wantErr            bool
	}{
		{
			name: "integer",
			variableClasses: `[{
				"name": "cpu",
				"required": true,
				"schema": {
					"openAPIV3Schema": {
    					"type": "integer",
						"minimum": 1
					}
				}
			}]`,
			variableTopologies: `[{
				"name": "cpu",
				"value": 1
			}]`,
		},
		{
			name: "integer, but got string",
			variableClasses: `[{
				"name": "cpu",
				"required": true,
				"schema": {
					"openAPIV3Schema": {
						"type": "integer",
						"minimum": 1
					}
				}
			}]`,
			variableTopologies: `[{
				"name": "cpu",
				"value": "1"
			}]`,
			wantErr: true,
		},
		{
			name: "string",
			variableClasses: `[{
				"name": "location",
				"required": true,
				"schema": {
					"openAPIV3Schema": {
						"type": "string",
						"minLength": 1
					}
				}
			}]`,
			variableTopologies: `[{
				"name": "location",
				"value": "us-east"
			}]`,
		},
		{
			name: "object",
			variableClasses: `[{
				"name": "machineType",
				"required": true,
				"schema": {
					"openAPIV3Schema": {
						"type": "object",
						"properties": {
							"cpu": {
								"type": "integer",
								"minimum": 1
							},
							"gpu": {
								"type": "integer",
								"minimum": 1
							}
						}
					}
				}
			}]`,
			variableTopologies: `[{
				"name": "machineType",
			    "value": {
					"cpu": 8,
					"gpu": 1
				}
			}]`,
		},
		{
			name: "object, but got nested string instead of integer",
			variableClasses: `[{
				"name": "machineType",
				"required": true,
				"schema": {
					"openAPIV3Schema": {
						"type": "object",
						"properties": {
							"cpu": {
								"type": "integer",
								"minimum": 1
							},
							"gpu": {
								"type": "integer",
								"minimum": 1
							}
						}
					}
				}
			}]`,
			variableTopologies: `[{
				"name": "machineType",
				"value": {
					"cpu": 8,
					"gpu": "1"
				}
			}]`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			errList := ValidateVariableTopologies(toVariableTopologies(g, tt.variableTopologies), toVariableClasses(g, tt.variableClasses),
				field.NewPath("spec", "topology", "variables"))

			if tt.wantErr {
				g.Expect(errList).NotTo(BeEmpty())
			} else {
				g.Expect(errList).To(BeEmpty())
			}
		})
	}
}

func toVariableClasses(g *WithT, rawJSON string) []variables.VariableDefinitionClass {
	var ret []variables.VariableDefinitionClass
	g.Expect(json.Unmarshal([]byte(rawJSON), &ret)).To(Succeed())
	return ret
}

func toVariableTopologies(g *WithT, rawJSON string) []variables.VariableTopology {
	var ret []variables.VariableTopology
	g.Expect(json.Unmarshal([]byte(rawJSON), &ret)).To(Succeed())
	return ret
}
