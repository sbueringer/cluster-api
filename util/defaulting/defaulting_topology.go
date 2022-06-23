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

package defaulting

import (
	"testing"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// DefaultingTopologyAwareValidator interface is for objects that define both topology
// aware defaulting and validating webhooks.
type DefaultingTopologyAwareValidator interface { //nolint:revive
	admission.Defaulter
	// These functions should be kept in sync with the sigs.k8s.io/cluster-api/util/webhooks.TopologyAwareValidator interface.
	// It cannot be imported here because it would cause a cyclic dependency.
	ValidateCreate() error
	ValidateUpdate(skipImmutabilityChecks bool, old runtime.Object) error
	ValidateDelete() error
}

// DefaultTopologyAwareValidateTest returns a new testing function to be used in tests to
// make sure topology aware defaulting webhooks also pass validation tests on create,
// update and delete.
func DefaultTopologyAwareValidateTest(object DefaultingTopologyAwareValidator) func(*testing.T) {
	return func(t *testing.T) {
		t.Helper()

		createCopy := object.DeepCopyObject().(DefaultingTopologyAwareValidator)
		updateCopy := object.DeepCopyObject().(DefaultingTopologyAwareValidator)
		deleteCopy := object.DeepCopyObject().(DefaultingTopologyAwareValidator)
		defaultingUpdateCopy := updateCopy.DeepCopyObject().(DefaultingTopologyAwareValidator)

		t.Run("validate-on-create", func(t *testing.T) {
			g := gomega.NewWithT(t)
			createCopy.Default()
			g.Expect(createCopy.ValidateCreate()).To(gomega.Succeed())
		})
		t.Run("validate-on-update", func(t *testing.T) {
			g := gomega.NewWithT(t)
			defaultingUpdateCopy.Default()
			updateCopy.Default()
			g.Expect(defaultingUpdateCopy.ValidateUpdate(false, updateCopy)).To(gomega.Succeed())
		})
		t.Run("validate-on-delete", func(t *testing.T) {
			g := gomega.NewWithT(t)
			deleteCopy.Default()
			g.Expect(deleteCopy.ValidateDelete()).To(gomega.Succeed())
		})
	}
}
