/*
Copyright 2025 The Kubernetes Authors.

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

package patches

import (
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cluster-api/internal/contract"
)

// PatchSpec overwrites spec in object with spec of patchedObject.
func PatchSpec(object *runtime.RawExtension, patchedObject []byte) error {
	// Call MarshalJSON to handle the case where object.Raw is not set but object.Object is set.
	objectBytes, err := object.MarshalJSON()
	if err != nil {
		return errors.Wrap(err, "failed to marshal object")
	}
	objectUnstructured, err := bytesToUnstructured(objectBytes)
	if err != nil {
		return errors.Wrap(err, "failed to convert object to Unstructured")
	}
	patchedObjectUnstructured, err := bytesToUnstructured(patchedObject)
	if err != nil {
		return errors.Wrap(err, "failed to convert patched object to Unstructured")
	}

	// Copy spec from patchedObjectUnstructured to objectUnstructured.
	if err := CopySpec(CopySpecInput{
		Src:          patchedObjectUnstructured,
		Dest:         objectUnstructured,
		SrcSpecPath:  "spec",
		DestSpecPath: "spec",
	}); err != nil {
		return errors.Wrap(err, "failed to apply patch to object")
	}

	// Marshal objectUnstructured and store it in object.
	objectBytes, err = objectUnstructured.MarshalJSON()
	if err != nil {
		return errors.Wrapf(err, "failed to marshal patched object")
	}
	object.Object = objectUnstructured
	object.Raw = objectBytes
	return nil
}

type CopySpecInput struct {
	Src              *unstructured.Unstructured
	Dest             *unstructured.Unstructured
	SrcSpecPath      string
	DestSpecPath     string
	FieldsToPreserve []contract.Path
}

// CopySpec copies a field from a srcSpecPath in src to a destSpecPath in dest,
// while preserving fieldsToPreserve.
func CopySpec(in CopySpecInput) error {
	// Backup fields that should be preserved from dest.
	preservedFields := map[string]interface{}{}
	for _, field := range in.FieldsToPreserve {
		value, found, err := unstructured.NestedFieldNoCopy(in.Dest.Object, field...)
		if !found {
			// Continue if the field does not exist in src. fieldsToPreserve don't have to exist.
			continue
		} else if err != nil {
			return errors.Wrapf(err, "failed to get field %q from %s %s", strings.Join(field, "."), in.Dest.GetKind(), klog.KObj(in.Dest))
		}
		preservedFields[strings.Join(field, ".")] = value
	}

	// Get spec from src.
	srcSpec, found, err := unstructured.NestedFieldNoCopy(in.Src.Object, strings.Split(in.SrcSpecPath, ".")...)
	if !found {
		// Return if srcSpecPath does not exist in src, nothing to do.
		return nil
	} else if err != nil {
		return errors.Wrapf(err, "failed to get field %q from %s %s", in.SrcSpecPath, in.Src.GetKind(), klog.KObj(in.Src))
	}

	// Set spec in dest.
	if err := unstructured.SetNestedField(in.Dest.Object, srcSpec, strings.Split(in.DestSpecPath, ".")...); err != nil {
		return errors.Wrapf(err, "failed to set field %q on %s %s", in.DestSpecPath, in.Dest.GetKind(), klog.KObj(in.Dest))
	}

	// Restore preserved fields.
	for path, value := range preservedFields {
		if err := unstructured.SetNestedField(in.Dest.Object, value, strings.Split(path, ".")...); err != nil {
			return errors.Wrapf(err, "failed to set field %q on %s %s", path, in.Dest.GetKind(), klog.KObj(in.Dest))
		}
	}
	return nil
}

// unstructuredDecoder is used to decode byte arrays into Unstructured objects.
var unstructuredDecoder = serializer.NewCodecFactory(nil).UniversalDeserializer()

// bytesToUnstructured provides a utility method that converts a (JSON) byte array into an Unstructured object.
func bytesToUnstructured(b []byte) (*unstructured.Unstructured, error) {
	// Unmarshal the JSON.
	u := &unstructured.Unstructured{}
	if _, _, err := unstructuredDecoder.Decode(b, nil, u); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal object from json")
	}

	return u, nil
}
