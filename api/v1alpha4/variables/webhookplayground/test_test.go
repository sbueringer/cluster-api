package webhookplayground_test

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

var tpl = `
- contentFrom:
    secret:
      key: control-plane-azure.json
      name: {{ .builtin.cluster.name }}-control-plane-azure-json
`

func TestPatchTemplating(t *testing.T) {

	template := template.Must(template.New("test").Parse(tpl))

	data := map[string]interface{} {
		"builtin": map[string]interface{} {
			"cluster": map[string]string{
				"name": "Cluster1",
			},
		},
	}

	var buf bytes.Buffer
	err := template.ExecuteTemplate(&buf, "test", data)
	fmt.Println(err)
	fmt.Println(buf.String())

	var value apiextensionsv1.JSON

	err = yaml.Unmarshal(buf.Bytes(), &value)
	fmt.Println(err)
	fmt.Println(string(value.Raw))


}
