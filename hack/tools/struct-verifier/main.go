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

package main

import (
	"bytes"
	"fmt"
	"go/printer"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

var (
	optionalMarker = markers.Must(markers.MakeDefinition("optional", markers.DescribesField, struct{}{}))
	omitEmpty      = "omitempty"
)

type fieldInfo struct {
	field       markers.FieldInfo
	structName  string
	typeName    string
	omitEmpty   bool
	optional    bool
	pointer     bool
	pointerType bool
}

func main() {
	// Define the marker collector.
	col := &markers.Collector{
		Registry: &markers.Registry{},
	}
	// Register the markers.
	if err := col.Registry.Register(optionalMarker); err != nil {
		klog.Fatal(err)
	}

	// Load all packages.
	packages, err := loader.LoadRoots("./...")
	if err != nil {
		klog.Fatal(err)
	}

	var fieldInfos []fieldInfo

	for i := range packages {
		root := packages[i]

		if !strings.HasSuffix(root.PkgPath, "/api/v1beta1") {
			// We're only interested in api/v1beta1 folders for now, exclude everything
			// else that's not an API package.
			continue
		}

		// Populate type information and loop through every type.
		root.NeedTypesInfo()
		if err := markers.EachType(col, root, func(info *markers.TypeInfo) {
			// Check if the type is registered with storage version.
			for _, field := range info.Fields {
				var foundOptionalMarker bool
				var foundOmitEmpty bool
				var foundPointer bool
				var foundPointerType bool

				if m := field.Markers.Get(optionalMarker.Name); m != nil {
					foundOptionalMarker = true
				}
				if strings.Contains(string(field.Tag), omitEmpty) {
					foundOmitEmpty = true
				}

				var typeNameBuf bytes.Buffer
				if err := printer.Fprint(&typeNameBuf, root.Fset, field.RawField.Type); err != nil {
					klog.Errorf(err.Error())
				}
				typeName := typeNameBuf.String()

				if strings.HasPrefix(typeName, "*") {
					foundPointer = true
				}

				if strings.HasPrefix(typeName, "*") ||
					strings.HasPrefix(typeName, "map") ||
					strings.HasPrefix(typeName, "[]") {
					foundPointerType = true
				}

				// Let's ignore Spec and Status
				if field.Name == "Spec" || field.Name == "Status" {
					continue
				}

				// Let's ignore top-level ListMeta and ObjectMeta
				if field.Name == "" && (typeName == "metav1.ListMeta" || typeName == "metav1.ObjectMeta") {
					continue
				}

				fieldInfos = append(fieldInfos, fieldInfo{
					field:       field,
					structName:  info.Name,
					typeName:    typeName,
					omitEmpty:   foundOmitEmpty,
					optional:    foundOptionalMarker,
					pointer:     foundPointer,
					pointerType: foundPointerType,
				})

			}
		}); err != nil {
			klog.Fatal(err)
		}
	}

	// Overview contains all fields which have one of optional, omitEmpty and pointer, but not all
	printTable("### Overview", fieldInfos, func(field fieldInfo) bool {
		return (field.optional || field.omitEmpty || field.pointer) &&
			(!(field.optional && field.omitEmpty && field.pointer))
	})

	printTable("### Optional without omitempty", fieldInfos, func(field fieldInfo) bool { return field.optional && !field.omitEmpty })

	printTable("### Omitempty without optional", fieldInfos, func(field fieldInfo) bool { return !field.optional && field.omitEmpty })
}

func printTable(title string, fieldInfos []fieldInfo, pred func(field fieldInfo) bool) {
	fmt.Printf("%s\n\n", title)
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Type", "Optional", "Omitempty", "Pointer", "PointerType"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	for _, field := range fieldInfos {
		if !pred(field) {
			continue
		}
		table.Append(calculateRow(field))
	}
	table.Render()
	fmt.Println()
}

func calculateRow(field fieldInfo) []string {
	return []string{fmt.Sprintf("%s.%s", field.structName, field.field.Name), field.typeName, boolToX(field.optional), boolToX(field.omitEmpty), boolToX(field.pointer), boolToX(field.pointerType)}
}

func boolToX(b bool) string {
	if b {
		return "x"
	}
	return ""
}
