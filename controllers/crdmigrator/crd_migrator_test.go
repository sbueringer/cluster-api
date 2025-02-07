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

package crdmigrator

import (
	"path"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	t1v1beta1 "sigs.k8s.io/cluster-api/controllers/crdmigrator/test/t1/v1beta1"
	t2v1beta2 "sigs.k8s.io/cluster-api/controllers/crdmigrator/test/t2/v1beta2"
	t3v1beta2 "sigs.k8s.io/cluster-api/controllers/crdmigrator/test/t3/v1beta2"
	t4v1beta2 "sigs.k8s.io/cluster-api/controllers/crdmigrator/test/t4/v1beta2"
)

func TestReconcile(t *testing.T) {
	mgr := env.Manager

	crdName := "testclusters.test.cluster.x-k8s.io"
	crdObjectKey := client.ObjectKey{Name: crdName}
	crdRequest := ctrl.Request{NamespacedName: crdObjectKey}

	tests := []struct {
		name                   string
		skipCRDMigrationPhases sets.Set[string]
	}{
		{
			name:                   "run both StorageVersionMigration and CleanupManagedFields",
			skipCRDMigrationPhases: nil,
		},
		{
			name:                   "run only CleanupManagedFields",
			skipCRDMigrationPhases: sets.Set[string]{}.Insert(string(StorageVersionMigrationPhase)),
		},
		{
			name:                   "run only StorageVersionMigration",
			skipCRDMigrationPhases: sets.Set[string]{}.Insert(string(CleanupManagedFieldsPhase)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			skipCRDMigrationPhases := strings.Join(tt.skipCRDMigrationPhases.UnsortedList(), ",")
			crdMigratorT1, err := createCRDMigrator(mgr, skipCRDMigrationPhases, map[client.Object]ByObjectConfig{
				&t1v1beta1.TestCluster{}: {UseCache: true},
			}, t1v1beta1.AddToScheme)
			g.Expect(err).ToNot(HaveOccurred())
			crdMigratorT2, err := createCRDMigrator(mgr, skipCRDMigrationPhases, map[client.Object]ByObjectConfig{
				&t2v1beta2.TestCluster{}: {UseCache: true},
			}, t2v1beta2.AddToScheme)
			g.Expect(err).ToNot(HaveOccurred())
			crdMigratorT3, err := createCRDMigrator(mgr, skipCRDMigrationPhases, map[client.Object]ByObjectConfig{
				&t3v1beta2.TestCluster{}: {UseCache: true},
			}, t3v1beta2.AddToScheme)
			g.Expect(err).ToNot(HaveOccurred())
			crdMigratorT4, err := createCRDMigrator(mgr, skipCRDMigrationPhases, map[client.Object]ByObjectConfig{
				&t4v1beta2.TestCluster{}: {UseCache: true},
			}, t4v1beta2.AddToScheme)
			g.Expect(err).ToNot(HaveOccurred())

			defer func() {
				crd := &apiextensionsv1.CustomResourceDefinition{}
				crd.SetName(crdName)
				g.Expect(env.CleanupAndWait(ctx, crd)).To(Succeed())
			}()

			// T1:
			// * v1beta1: served: true, storage: true
			g.Expect(installCRDs("test/t1/crd")).To(Succeed())
			expectedStoredVersion := []string{"v1beta1"}
			g.Eventually(func(g Gomega) {
				crd := &apiextensionsv1.CustomResourceDefinition{}
				g.Expect(env.GetAPIReader().Get(ctx, crdObjectKey, crd)).To(Succeed())
				g.Expect(crd.Status.StoredVersions).To(ConsistOf(expectedStoredVersion))
			}).WithTimeout(1 * time.Second).Should(Succeed())
			// Reconcile should:
			// * do nothing
			// * set the annotation
			_, err = crdMigratorT1.Reconcile(ctx, crdRequest)
			g.Expect(err).ToNot(HaveOccurred())
			crd := &apiextensionsv1.CustomResourceDefinition{}
			g.Expect(env.GetAPIReader().Get(ctx, crdObjectKey, crd)).To(Succeed())
			g.Expect(crd.Status.StoredVersions).To(ConsistOf(expectedStoredVersion))
			g.Expect(crd.Annotations[clusterv1.CRDMigrationObservedGenerationAnnotation]).To(Equal("1"))

			// After T1:
			// * Deploy test-cluster CR and verify managed fields use v1beta1
			testClusterT1 := &t1v1beta1.TestCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "test-cluster",
				},
				Spec: t1v1beta1.TestClusterSpec{
					Foo: "foo-value",
				},
			}
			g.Expect(crdMigratorT1.Client.Create(ctx, testClusterT1)).To(Succeed())
			expectedManagedFieldAPIVersion := "test.cluster.x-k8s.io/v1beta1"
			g.Expect(crdMigratorT1.Client.Get(ctx, client.ObjectKeyFromObject(testClusterT1), testClusterT1)).To(Succeed())
			g.Expect(testClusterT1.ManagedFields).To(HaveLen(1))
			g.Expect(testClusterT1.ManagedFields[0].APIVersion).To(Equal(expectedManagedFieldAPIVersion))

			// T2:
			// * v1beta1: served: true, storage: false
			// * v1beta2: served: true, storage: true
			g.Expect(installCRDs("test/t2/crd")).To(Succeed())
			expectedStoredVersion = []string{"v1beta1", "v1beta2"}
			g.Eventually(func(g Gomega) {
				crd := &apiextensionsv1.CustomResourceDefinition{}
				g.Expect(env.GetAPIReader().Get(ctx, crdObjectKey, crd)).To(Succeed())
				g.Expect(crd.Status.StoredVersions).To(ConsistOf(expectedStoredVersion))
			}).WithTimeout(1 * time.Second).Should(Succeed())
			// Reconcile should:
			// * run storage version migration (if not skipped)
			// * set .status.storedVersion = [storageVersion]
			// * set the annotation
			_, err = crdMigratorT2.Reconcile(ctx, crdRequest)
			g.Expect(err).ToNot(HaveOccurred())
			if !tt.skipCRDMigrationPhases.Has(string(StorageVersionMigrationPhase)) {
				expectedStoredVersion = []string{"v1beta2"}
			}
			crd = &apiextensionsv1.CustomResourceDefinition{}
			g.Expect(env.GetAPIReader().Get(ctx, crdObjectKey, crd)).To(Succeed())
			g.Expect(crd.Annotations[clusterv1.CRDMigrationObservedGenerationAnnotation]).To(Equal("2"))

			// T3:
			// * v1beta1: served: false, storage: false
			// * v1beta2: served: true, storage: true
			g.Expect(installCRDs("test/t3/crd")).To(Succeed())
			g.Eventually(func(g Gomega) {
				crd := &apiextensionsv1.CustomResourceDefinition{}
				g.Expect(env.GetAPIReader().Get(ctx, crdObjectKey, crd)).To(Succeed())
				g.Expect(crd.Status.StoredVersions).To(ConsistOf(expectedStoredVersion))
			}).WithTimeout(1 * time.Second).Should(Succeed())
			// Reconcile should:
			// * run managed field cleanup (if not skipped)
			// * set the annotation
			_, err = crdMigratorT3.Reconcile(ctx, crdRequest)
			g.Expect(err).ToNot(HaveOccurred())
			if !tt.skipCRDMigrationPhases.Has(string(CleanupManagedFieldsPhase)) {
				expectedManagedFieldAPIVersion = "test.cluster.x-k8s.io/v1beta2"
			}
			crd = &apiextensionsv1.CustomResourceDefinition{}
			g.Expect(env.GetAPIReader().Get(ctx, crdObjectKey, crd)).To(Succeed())
			g.Expect(crd.Status.StoredVersions).To(ConsistOf(expectedStoredVersion))
			g.Expect(crd.Annotations[clusterv1.CRDMigrationObservedGenerationAnnotation]).To(Equal("3"))

			// After T3:
			// * Verify managed fields
			testClusterT3 := &t3v1beta2.TestCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "test-cluster",
				},
			}
			g.Expect(crdMigratorT3.Client.Get(ctx, client.ObjectKeyFromObject(testClusterT3), testClusterT3)).To(Succeed())
			g.Expect(testClusterT3.ManagedFields).To(HaveLen(1))
			g.Expect(testClusterT3.ManagedFields[0].APIVersion).To(Equal(expectedManagedFieldAPIVersion))

			// T4:
			// * v1beta2: served: true, storage: true
			err = installCRDs("test/t4/crd")
			if tt.skipCRDMigrationPhases.Has(string(StorageVersionMigrationPhase)) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("status.storedVersions[0]: Invalid value: \"v1beta1\": must appear in spec.versions"))
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Eventually(func(g Gomega) {
				crd := &apiextensionsv1.CustomResourceDefinition{}
				g.Expect(env.GetAPIReader().Get(ctx, crdObjectKey, crd)).To(Succeed())
				g.Expect(crd.Status.StoredVersions).To(ConsistOf(expectedStoredVersion))
			}).WithTimeout(1 * time.Second).Should(Succeed())
			// Reconcile should:
			// * do nothing
			// * set the annotation
			_, err = crdMigratorT4.Reconcile(ctx, crdRequest)
			g.Expect(err).ToNot(HaveOccurred())
			crd = &apiextensionsv1.CustomResourceDefinition{}
			g.Expect(env.GetAPIReader().Get(ctx, crdObjectKey, crd)).To(Succeed())
			g.Expect(crd.Status.StoredVersions).To(ConsistOf(expectedStoredVersion))
			g.Expect(crd.Annotations[clusterv1.CRDMigrationObservedGenerationAnnotation]).To(Equal("4"))

			// After T4:
			// * Verify we can still read the test-cluster CR (this verifies it is stored as v1beta2)
			testClusterT4 := &t4v1beta2.TestCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "test-cluster",
				},
			}
			g.Expect(crdMigratorT4.Client.Get(ctx, client.ObjectKeyFromObject(testClusterT4), testClusterT4)).To(Succeed())
			g.Expect(testClusterT4.ManagedFields).To(HaveLen(1))
			g.Expect(testClusterT4.ManagedFields[0].APIVersion).To(Equal(expectedManagedFieldAPIVersion))

			testClusterT4SSAUpdate := &unstructured.Unstructured{}
			testClusterT4SSAUpdate.SetGroupVersionKind(t4v1beta2.GroupVersion.WithKind("TestCluster"))
			testClusterT4SSAUpdate.SetNamespace(testClusterT4.Namespace)
			testClusterT4SSAUpdate.SetName(testClusterT4.Name)
			g.Expect(unstructured.SetNestedField(testClusterT4SSAUpdate.Object, "new-foo-value", "spec", "foo")).To(Succeed())
			err = crdMigratorT4.Client.Patch(ctx, testClusterT4SSAUpdate, client.Apply, client.FieldOwner("unit-test-client"))
			if tt.skipCRDMigrationPhases.Has(string(CleanupManagedFieldsPhase)) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("request to convert CR to an invalid group/version: test.cluster.x-k8s.io/v1beta1"))
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
		})
	}
}

func createCRDMigrator(mgr manager.Manager, skipCRDMigrationPhases string, crdMigratorConfig map[client.Object]ByObjectConfig, AddToSchemeFuncs ...func(s *runtime.Scheme) error) (*CRDMigrator, error) {
	scheme := runtime.NewScheme()
	_ = apiextensionsv1.AddToScheme(scheme)
	for _, f := range AddToSchemeFuncs {
		if err := f(scheme); err != nil {
			return nil, err
		}
	}

	// Note: We have to use a scheme that has the correct types for the client.
	c, err := client.New(mgr.GetConfig(), client.Options{
		HTTPClient: mgr.GetHTTPClient(),
		Scheme:     scheme,
		Mapper:     mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, err
	}

	m := &CRDMigrator{
		Client:                 c,
		APIReader:              c,
		SkipCRDMigrationPhases: skipCRDMigrationPhases,
		Config:                 crdMigratorConfig,
	}

	if err := m.setup(scheme); err != nil {
		return nil, err
	}
	return m, nil
}

func installCRDs(crdPath string) error {
	// Get the root of the current file to use in CRD paths.
	_, filename, _, _ := goruntime.Caller(0)                                  //nolint:dogsled
	_, err := envtest.InstallCRDs(env.GetConfig(), envtest.CRDInstallOptions{ // FIXME: update drops the annotation!
		Scheme: env.GetScheme(),
		Paths: []string{
			filepath.Join(path.Dir(filename), "..", "..", "controllers", "crdmigrator", crdPath),
		},
		ErrorIfPathMissing: true,
	})
	return err
}
