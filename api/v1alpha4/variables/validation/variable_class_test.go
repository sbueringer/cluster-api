package validation

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_ValidateVariableClasses(t *testing.T) {

	tests := []struct {
		name            string
		variableClasses string
		wantErr         bool
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			errList := ValidateVariableClasses(toVariableClasses(g, tt.variableClasses),
				field.NewPath("spec", "variables"))

			if tt.wantErr {
				g.Expect(errList).NotTo(BeEmpty())
			} else {
				g.Expect(errList).To(BeEmpty())
			}
		})
	}
}
