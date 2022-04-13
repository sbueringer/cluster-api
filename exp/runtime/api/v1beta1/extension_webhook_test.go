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

package v1beta1

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/cluster-api/feature"
	utildefaulting "sigs.k8s.io/cluster-api/util/defaulting"
)

var (
	fakeScheme = runtime.NewScheme()
)

func init() {
	_ = AddToScheme(fakeScheme)
}

func TestExtensionDefault(t *testing.T) {
	g := NewWithT(t)
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.RuntimeSDK, true)()

	extension := &Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-extension",
		},
		Spec: ExtensionSpec{
			ClientConfig: ExtensionClientConfig{
				Service: &ServiceReference{
					Name:      "name",
					Namespace: "namespace",
				},
			},
		},
	}
	t.Run("for Extension", utildefaulting.DefaultValidateTest(extension))
	extension.Default()

	g.Expect(extension.Spec.NamespaceSelector).To(Equal(&metav1.LabelSelector{}))
	g.Expect(extension.Spec.ClientConfig.Service.Port).To(Equal(pointer.Int32(8443)))
}

func TestExtensionConfigValidate(t *testing.T) {
	extensionWithURL := &Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-extension",
		},
		Spec: ExtensionSpec{
			ClientConfig: ExtensionClientConfig{
				URL: pointer.String("https://extension-address.com"),
			},
		},
	}

	extensionWithService := &Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-extension",
		},
		Spec: ExtensionSpec{
			ClientConfig: ExtensionClientConfig{
				Service: &ServiceReference{
					Path:      pointer.StringPtr("/path/to/handler"),
					Port:      pointer.Int32(1),
					Name:      "foo",
					Namespace: "bar",
				}},
		},
	}

	// Valid updated Extension
	updatedExtension := extensionWithURL.DeepCopy()
	updatedExtension.Spec.ClientConfig.URL = pointer.StringPtr("https://a-in-extension-address.com")

	badURLExtension := extensionWithURL.DeepCopy()
	badURLExtension.Spec.ClientConfig.URL = pointer.String("https//extension-address.com")

	extensionWithoutURLOrService := extensionWithURL.DeepCopy()
	extensionWithoutURLOrService.Spec.ClientConfig.URL = nil

	extensionWithInvalidServicePath := extensionWithService.DeepCopy()
	extensionWithInvalidServicePath.Spec.ClientConfig.Service.Path = pointer.StringPtr("https://example.com")

	extensionWithInvalidServicePort := extensionWithService.DeepCopy()
	extensionWithInvalidServicePort.Spec.ClientConfig.Service.Port = pointer.Int32(90000)

	tests := []struct {
		name        string
		in          *Extension
		old         *Extension
		featureGate bool
		expectErr   bool
	}{
		{
			name:        "creation should fail if feature flag is disabled",
			in:          extensionWithURL,
			featureGate: false,
			expectErr:   true,
		},
		{
			name:        "update should fail if feature flag is disabled",
			old:         extensionWithURL,
			in:          updatedExtension,
			featureGate: false,
			expectErr:   true,
		},
		{
			name:        "update should fail if URL is invalid",
			old:         extensionWithURL,
			in:          badURLExtension,
			featureGate: true,
			expectErr:   true,
		},
		{
			name:        "update should fail if URL and Service are both nil",
			old:         extensionWithURL,
			in:          extensionWithoutURLOrService,
			featureGate: true,
			expectErr:   true,
		},

		{
			name:        "update should fail if Service Path is invalid",
			old:         extensionWithService,
			in:          extensionWithInvalidServicePath,
			featureGate: true,
			expectErr:   true,
		},

		{
			name:        "update should fail if Service Port is invalid",
			old:         extensionWithService,
			in:          extensionWithInvalidServicePort,
			featureGate: true,
			expectErr:   true,
		},
		{
			name:        "update should pass if Service Path is valid",
			old:         extensionWithService,
			in:          extensionWithService,
			featureGate: true,
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.RuntimeSDK, tt.featureGate)()
			g := NewWithT(t)

			// Default the objects so we're not handling defaulted cases.
			tt.in.Default()
			if tt.old != nil {
				tt.old.Default()
			}

			err := tt.in.validate(tt.old)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
		})
	}
}
