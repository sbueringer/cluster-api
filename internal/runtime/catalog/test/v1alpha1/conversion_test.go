/*
Copyright 2022 The Kubernetes Authors.

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

package v1alpha1

import (
	"context"
	"testing"

	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
	"sigs.k8s.io/cluster-api/internal/runtime/catalog/test/v1alpha2"
)

func TestConversion(t *testing.T) {
	// NOTE: this is only a very simple conversion test
	// We need something more once we merge multiple versions of a type
	// and actually use conversion.

	var c = catalog.New()
	_ = AddToCatalog(c)
	_ = v1alpha2.AddToCatalog(c)

	c1 := &v1alpha2.FakeRequest{}
	c2 := &FakeRequest{}

	err := c.Convert(c1, c2, context.Background())
	if err != nil {
		t.Fatal(err)
	}
}
