/*
Copyright 2018 The Kubernetes Authors.

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

package machineset

import (
	"encoding/json"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/internal/test/builder"
	"sigs.k8s.io/cluster-api/internal/util/ssa"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
)

var _ reconcile.Reconciler = &Reconciler{}

func TestMachineSetReconciler(t *testing.T) {
	setup := func(t *testing.T, g *WithT) (*corev1.Namespace, *clusterv1.Cluster) {
		t.Helper()

		t.Log("Creating the namespace")
		ns, err := env.CreateNamespace(ctx, "test-machine-set-reconciler")
		g.Expect(err).To(BeNil())

		t.Log("Creating the Cluster")
		cluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: ns.Name, Name: testClusterName}}
		g.Expect(env.Create(ctx, cluster)).To(Succeed())

		t.Log("Creating the Cluster Kubeconfig Secret")
		g.Expect(env.CreateKubeconfigSecret(ctx, cluster)).To(Succeed())

		return ns, cluster
	}

	teardown := func(t *testing.T, g *WithT, ns *corev1.Namespace, cluster *clusterv1.Cluster) {
		t.Helper()

		t.Log("Deleting the Cluster")
		g.Expect(env.Delete(ctx, cluster)).To(Succeed())
		t.Log("Deleting the namespace")
		g.Expect(env.Delete(ctx, ns)).To(Succeed())
	}

	t.Run("Should reconcile a MachineSet", func(t *testing.T) {
		g := NewWithT(t)
		namespace, testCluster := setup(t, g)
		defer teardown(t, g, namespace, testCluster)

		duration10m := &metav1.Duration{Duration: 10 * time.Minute}
		duration5m := &metav1.Duration{Duration: 5 * time.Minute}
		replicas := int32(2)
		version := "v1.14.2"
		instance := &clusterv1.MachineSet{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ms-",
				Namespace:    namespace.Name,
				Labels: map[string]string{
					"label-1": "true",
				},
			},
			Spec: clusterv1.MachineSetSpec{
				ClusterName: testCluster.Name,
				Replicas:    &replicas,
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"label-1": "true",
					},
				},
				Template: clusterv1.MachineTemplateSpec{
					ObjectMeta: clusterv1.ObjectMeta{
						Labels: map[string]string{
							"label-1": "true",
						},
						Annotations: map[string]string{
							"annotation-1": "true",
							"precedence":   "MachineSet",
						},
					},
					Spec: clusterv1.MachineSpec{
						ClusterName: testCluster.Name,
						Version:     &version,
						Bootstrap: clusterv1.Bootstrap{
							ConfigRef: &corev1.ObjectReference{
								APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
								Kind:       "GenericBootstrapConfigTemplate",
								Name:       "ms-template",
							},
						},
						InfrastructureRef: corev1.ObjectReference{
							APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
							Kind:       "GenericInfrastructureMachineTemplate",
							Name:       "ms-template",
						},
						NodeDrainTimeout:        duration10m,
						NodeDeletionTimeout:     duration10m,
						NodeVolumeDetachTimeout: duration10m,
					},
				},
			},
		}

		// Create bootstrap template resource.
		bootstrapResource := map[string]interface{}{
			"kind":       "GenericBootstrapConfig",
			"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta1",
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"precedence": "GenericBootstrapConfig",
				},
			},
		}
		bootstrapTmpl := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": bootstrapResource,
				},
			},
		}
		bootstrapTmpl.SetKind("GenericBootstrapConfigTemplate")
		bootstrapTmpl.SetAPIVersion("bootstrap.cluster.x-k8s.io/v1beta1")
		bootstrapTmpl.SetName("ms-template")
		bootstrapTmpl.SetNamespace(namespace.Name)
		g.Expect(env.Create(ctx, bootstrapTmpl)).To(Succeed())

		// Create infrastructure template resource.
		infraResource := map[string]interface{}{
			"kind":       "GenericInfrastructureMachine",
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"precedence": "GenericInfrastructureMachineTemplate",
				},
			},
			"spec": map[string]interface{}{
				"size": "3xlarge",
			},
		}
		infraTmpl := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": infraResource,
				},
			},
		}
		infraTmpl.SetKind("GenericInfrastructureMachineTemplate")
		infraTmpl.SetAPIVersion("infrastructure.cluster.x-k8s.io/v1beta1")
		infraTmpl.SetName("ms-template")
		infraTmpl.SetNamespace(namespace.Name)
		g.Expect(env.Create(ctx, infraTmpl)).To(Succeed())

		// Create the MachineSet.
		g.Expect(env.Create(ctx, instance)).To(Succeed())
		defer func() {
			g.Expect(env.Delete(ctx, instance)).To(Succeed())
		}()

		t.Log("Verifying the linked bootstrap template has a cluster owner reference")
		g.Eventually(func() bool {
			obj, err := external.Get(ctx, env, instance.Spec.Template.Spec.Bootstrap.ConfigRef, instance.Namespace)
			if err != nil {
				return false
			}

			return util.HasOwnerRef(obj.GetOwnerReferences(), metav1.OwnerReference{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Cluster",
				Name:       testCluster.Name,
				UID:        testCluster.UID,
			})
		}, timeout).Should(BeTrue())

		t.Log("Verifying the linked infrastructure template has a cluster owner reference")
		g.Eventually(func() bool {
			obj, err := external.Get(ctx, env, &instance.Spec.Template.Spec.InfrastructureRef, instance.Namespace)
			if err != nil {
				return false
			}

			return util.HasOwnerRef(obj.GetOwnerReferences(), metav1.OwnerReference{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Cluster",
				Name:       testCluster.Name,
				UID:        testCluster.UID,
			})
		}, timeout).Should(BeTrue())

		machines := &clusterv1.MachineList{}

		// Verify that we have 2 replicas.
		g.Eventually(func() int {
			if err := env.List(ctx, machines, client.InNamespace(namespace.Name)); err != nil {
				return -1
			}
			return len(machines.Items)
		}, timeout).Should(BeEquivalentTo(replicas))

		t.Log("Creating a InfrastructureMachine for each Machine")
		infraMachines := &unstructured.UnstructuredList{}
		infraMachines.SetAPIVersion("infrastructure.cluster.x-k8s.io/v1beta1")
		infraMachines.SetKind("GenericInfrastructureMachine")
		g.Eventually(func() int {
			if err := env.List(ctx, infraMachines, client.InNamespace(namespace.Name)); err != nil {
				return -1
			}
			return len(machines.Items)
		}, timeout).Should(BeEquivalentTo(replicas))
		for _, im := range infraMachines.Items {
			g.Expect(im.GetAnnotations()).To(HaveKeyWithValue("annotation-1", "true"), "have annotations of MachineTemplate applied")
			g.Expect(im.GetAnnotations()).To(HaveKeyWithValue("precedence", "MachineSet"), "the annotations from the MachineSpec template to overwrite the infrastructure template ones")
			g.Expect(im.GetLabels()).To(HaveKeyWithValue("label-1", "true"), "have labels of MachineTemplate applied")
		}
		g.Eventually(func() bool {
			g.Expect(env.List(ctx, infraMachines, client.InNamespace(namespace.Name))).To(Succeed())
			// The Machine reconciler should remove the ownerReference to the MachineSet on the InfrastructureMachine.
			hasMSOwnerRef := false
			hasMachineOwnerRef := false
			for _, im := range infraMachines.Items {
				for _, o := range im.GetOwnerReferences() {
					if o.Kind == machineSetKind.Kind {
						hasMSOwnerRef = true
					}
					if o.Kind == "Machine" {
						hasMachineOwnerRef = true
					}
				}
			}
			return !hasMSOwnerRef && hasMachineOwnerRef
		}, timeout).Should(BeTrue(), "infraMachine should not have ownerRef to MachineSet")

		t.Log("Creating a BootstrapConfig for each Machine")
		bootstrapConfigs := &unstructured.UnstructuredList{}
		bootstrapConfigs.SetAPIVersion("bootstrap.cluster.x-k8s.io/v1beta1")
		bootstrapConfigs.SetKind("GenericBootstrapConfig")
		g.Eventually(func() int {
			if err := env.List(ctx, bootstrapConfigs, client.InNamespace(namespace.Name)); err != nil {
				return -1
			}
			return len(machines.Items)
		}, timeout).Should(BeEquivalentTo(replicas))
		for _, im := range bootstrapConfigs.Items {
			g.Expect(im.GetAnnotations()).To(HaveKeyWithValue("annotation-1", "true"), "have annotations of MachineTemplate applied")
			g.Expect(im.GetAnnotations()).To(HaveKeyWithValue("precedence", "MachineSet"), "the annotations from the MachineSpec template to overwrite the bootstrap config template ones")
			g.Expect(im.GetLabels()).To(HaveKeyWithValue("label-1", "true"), "have labels of MachineTemplate applied")
		}
		g.Eventually(func() bool {
			g.Expect(env.List(ctx, bootstrapConfigs, client.InNamespace(namespace.Name))).To(Succeed())
			// The Machine reconciler should remove the ownerReference to the MachineSet on the Bootstrap object.
			hasMSOwnerRef := false
			hasMachineOwnerRef := false
			for _, im := range bootstrapConfigs.Items {
				for _, o := range im.GetOwnerReferences() {
					if o.Kind == machineSetKind.Kind {
						hasMSOwnerRef = true
					}
					if o.Kind == "Machine" {
						hasMachineOwnerRef = true
					}
				}
			}
			return !hasMSOwnerRef && hasMachineOwnerRef
		}, timeout).Should(BeTrue(), "bootstrap should not have ownerRef to MachineSet")

		// Set the infrastructure reference as ready.
		for _, m := range machines.Items {
			fakeBootstrapRefReady(*m.Spec.Bootstrap.ConfigRef, bootstrapResource, g)
			fakeInfrastructureRefReady(m.Spec.InfrastructureRef, infraResource, g)
		}

		// Verify that in-place mutable fields propagate form MachineSet to Machines.
		t.Log("Updating NodeDrainTimeout on MachineSet")
		patchHelper, err := patch.NewHelper(instance, env)
		g.Expect(err).Should(BeNil())
		instance.Spec.Template.Spec.NodeDrainTimeout = duration5m
		g.Expect(patchHelper.Patch(ctx, instance)).Should(Succeed())

		t.Log("Verifying new NodeDrainTimeout value is set on Machines")
		g.Eventually(func() bool {
			if err := env.List(ctx, machines, client.InNamespace(namespace.Name)); err != nil {
				return false
			}
			// All the machines should have the new NodeDrainTimeoutValue
			for _, m := range machines.Items {
				if m.Spec.NodeDrainTimeout == nil {
					return false
				}
				if m.Spec.NodeDrainTimeout.Duration != duration5m.Duration {
					return false
				}
			}
			return true
		}, timeout).Should(BeTrue(), "machine should have the updated NodeDrainTimeout value")

		// Try to delete 1 machine and check the MachineSet scales back up.
		machineToBeDeleted := machines.Items[0]
		g.Expect(env.Delete(ctx, &machineToBeDeleted)).To(Succeed())

		// Verify that the Machine has been deleted.
		g.Eventually(func() bool {
			key := client.ObjectKey{Name: machineToBeDeleted.Name, Namespace: machineToBeDeleted.Namespace}
			if err := env.Get(ctx, key, &machineToBeDeleted); apierrors.IsNotFound(err) || !machineToBeDeleted.DeletionTimestamp.IsZero() {
				return true
			}
			return false
		}, timeout).Should(BeTrue())

		// Verify that we have 2 replicas.
		g.Eventually(func() (ready int) {
			if err := env.List(ctx, machines, client.InNamespace(namespace.Name)); err != nil {
				return -1
			}
			for _, m := range machines.Items {
				if !m.DeletionTimestamp.IsZero() {
					continue
				}
				ready++
			}
			return
		}, timeout*3).Should(BeEquivalentTo(replicas))

		// Verify that each machine has the desired kubelet version,
		// create a fake node in Ready state, update NodeRef, and wait for a reconciliation request.
		for i := 0; i < len(machines.Items); i++ {
			m := machines.Items[i]
			if !m.DeletionTimestamp.IsZero() {
				// Skip deleted Machines
				continue
			}

			g.Expect(m.Spec.Version).ToNot(BeNil())
			g.Expect(*m.Spec.Version).To(BeEquivalentTo("v1.14.2"))
			fakeBootstrapRefReady(*m.Spec.Bootstrap.ConfigRef, bootstrapResource, g)
			providerID := fakeInfrastructureRefReady(m.Spec.InfrastructureRef, infraResource, g)
			fakeMachineNodeRef(&m, providerID, g)
		}

		// Verify that all Machines are Ready.
		g.Eventually(func() int32 {
			key := client.ObjectKey{Name: instance.Name, Namespace: instance.Namespace}
			if err := env.Get(ctx, key, instance); err != nil {
				return -1
			}
			return instance.Status.AvailableReplicas
		}, timeout).Should(BeEquivalentTo(replicas))

		t.Log("Verifying MachineSet has MachinesCreatedCondition")
		g.Eventually(func() bool {
			key := client.ObjectKey{Name: instance.Name, Namespace: instance.Namespace}
			if err := env.Get(ctx, key, instance); err != nil {
				return false
			}
			return conditions.IsTrue(instance, clusterv1.MachinesCreatedCondition)
		}, timeout).Should(BeTrue())

		t.Log("Verifying MachineSet has ResizedCondition")
		g.Eventually(func() bool {
			key := client.ObjectKey{Name: instance.Name, Namespace: instance.Namespace}
			if err := env.Get(ctx, key, instance); err != nil {
				return false
			}
			return conditions.IsTrue(instance, clusterv1.ResizedCondition)
		}, timeout).Should(BeTrue())

		t.Log("Verifying MachineSet has MachinesReadyCondition")
		g.Eventually(func() bool {
			key := client.ObjectKey{Name: instance.Name, Namespace: instance.Namespace}
			if err := env.Get(ctx, key, instance); err != nil {
				return false
			}
			return conditions.IsTrue(instance, clusterv1.MachinesReadyCondition)
		}, timeout).Should(BeTrue())

		// Validate that the controller set the cluster name label in selector.
		g.Expect(instance.Status.Selector).To(ContainSubstring(testCluster.Name))
	})
}

func TestMachineSetOwnerReference(t *testing.T) {
	testCluster := &clusterv1.Cluster{
		TypeMeta:   metav1.TypeMeta{Kind: "Cluster", APIVersion: clusterv1.GroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: testClusterName},
	}

	ms1 := newMachineSet("machineset1", "valid-cluster", int32(0))
	ms2 := newMachineSet("machineset2", "invalid-cluster", int32(0))
	ms3 := newMachineSet("machineset3", "valid-cluster", int32(0))
	ms3.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
			Name:       "valid-machinedeployment",
		},
	}

	testCases := []struct {
		name               string
		request            reconcile.Request
		ms                 *clusterv1.MachineSet
		expectReconcileErr bool
		expectedOR         []metav1.OwnerReference
	}{
		{
			name: "should add cluster owner reference to machine set",
			request: reconcile.Request{
				NamespacedName: util.ObjectKey(ms1),
			},
			ms: ms1,
			expectedOR: []metav1.OwnerReference{
				{
					APIVersion: testCluster.APIVersion,
					Kind:       testCluster.Kind,
					Name:       testCluster.Name,
					UID:        testCluster.UID,
				},
			},
		},
		{
			name: "should not add cluster owner reference if machine is owned by a machine deployment",
			request: reconcile.Request{
				NamespacedName: util.ObjectKey(ms3),
			},
			ms: ms3,
			expectedOR: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "MachineDeployment",
					Name:       "valid-machinedeployment",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			msr := &Reconciler{
				Client: fake.NewClientBuilder().WithObjects(
					testCluster,
					ms1,
					ms2,
					ms3,
				).Build(),
				recorder: record.NewFakeRecorder(32),
			}

			_, err := msr.Reconcile(ctx, tc.request)
			if tc.expectReconcileErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			key := client.ObjectKey{Namespace: tc.ms.Namespace, Name: tc.ms.Name}
			var actual clusterv1.MachineSet
			if len(tc.expectedOR) > 0 {
				g.Expect(msr.Client.Get(ctx, key, &actual)).To(Succeed())
				g.Expect(actual.OwnerReferences).To(Equal(tc.expectedOR))
			} else {
				g.Expect(actual.OwnerReferences).To(BeEmpty())
			}
		})
	}
}

func TestMachineSetReconcile(t *testing.T) {
	testCluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: testClusterName},
	}

	t.Run("ignore machine sets marked for deletion", func(t *testing.T) {
		g := NewWithT(t)

		dt := metav1.Now()
		ms := &clusterv1.MachineSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "machineset1",
				Namespace:         metav1.NamespaceDefault,
				DeletionTimestamp: &dt,
			},
			Spec: clusterv1.MachineSetSpec{
				ClusterName: testClusterName,
			},
		}
		request := reconcile.Request{
			NamespacedName: util.ObjectKey(ms),
		}

		msr := &Reconciler{
			Client:   fake.NewClientBuilder().WithObjects(testCluster, ms).Build(),
			recorder: record.NewFakeRecorder(32),
		}
		result, err := msr.Reconcile(ctx, request)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(reconcile.Result{}))
	})

	t.Run("records event if reconcile fails", func(t *testing.T) {
		g := NewWithT(t)

		ms := newMachineSet("machineset1", testClusterName, int32(0))
		ms.Spec.Selector.MatchLabels = map[string]string{
			"--$-invalid": "true",
		}

		request := reconcile.Request{
			NamespacedName: util.ObjectKey(ms),
		}

		rec := record.NewFakeRecorder(32)
		msr := &Reconciler{
			Client:   fake.NewClientBuilder().WithObjects(testCluster, ms).Build(),
			recorder: rec,
		}
		_, _ = msr.Reconcile(ctx, request)
		g.Eventually(rec.Events).Should(Receive())
	})

	t.Run("reconcile successfully when labels are missing", func(t *testing.T) {
		g := NewWithT(t)

		ms := newMachineSet("machineset1", testClusterName, int32(0))
		ms.Labels = nil
		ms.Spec.Selector.MatchLabels = nil
		ms.Spec.Template.Labels = nil

		request := reconcile.Request{
			NamespacedName: util.ObjectKey(ms),
		}

		rec := record.NewFakeRecorder(32)
		msr := &Reconciler{
			Client:   fake.NewClientBuilder().WithObjects(testCluster, ms).Build(),
			recorder: rec,
		}
		_, err := msr.Reconcile(ctx, request)
		g.Expect(err).NotTo(HaveOccurred())
	})
}

func TestMachineSetToMachines(t *testing.T) {
	machineSetList := []client.Object{
		&clusterv1.MachineSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "withMatchingLabels",
				Namespace: metav1.NamespaceDefault,
			},
			Spec: clusterv1.MachineSetSpec{
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"foo":                      "bar",
						clusterv1.ClusterNameLabel: testClusterName,
					},
				},
			},
		},
	}
	controller := true
	m := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "withOwnerRef",
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: testClusterName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       "Owner",
					Kind:       machineSetKind.Kind,
					Controller: &controller,
				},
			},
		},
	}
	m2 := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "noOwnerRefNoLabels",
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: testClusterName,
			},
		},
	}
	m3 := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "withMatchingLabels",
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				"foo":                      "bar",
				clusterv1.ClusterNameLabel: testClusterName,
			},
		},
	}
	testsCases := []struct {
		name      string
		mapObject client.Object
		expected  []reconcile.Request
	}{
		{
			name:      "should return empty request when controller is set",
			mapObject: &m,
			expected:  []reconcile.Request{},
		},
		{
			name:      "should return nil if machine has no owner reference",
			mapObject: &m2,
			expected:  nil,
		},
		{
			name:      "should return request if machine set's labels matches machine's labels",
			mapObject: &m3,
			expected: []reconcile.Request{
				{NamespacedName: client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "withMatchingLabels"}},
			},
		},
	}

	r := &Reconciler{
		Client: fake.NewClientBuilder().WithObjects(append(machineSetList, &m, &m2, &m3)...).Build(),
	}

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			gs := NewWithT(t)

			got := r.MachineToMachineSets(tc.mapObject)
			gs.Expect(got).To(Equal(tc.expected))
		})
	}
}

func TestShouldExcludeMachine(t *testing.T) {
	controller := true
	testCases := []struct {
		machineSet clusterv1.MachineSet
		machine    clusterv1.Machine
		expected   bool
	}{
		{
			machineSet: clusterv1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{UID: "1"},
			},
			machine: clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "withNoMatchingOwnerRef",
					Namespace: metav1.NamespaceDefault,
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       "Owner",
							Kind:       machineSetKind.Kind,
							Controller: &controller,
							UID:        "not-1",
						},
					},
				},
			},
			expected: true,
		},
		{
			machineSet: clusterv1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{UID: "1"},
			},
			machine: clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "withMatchingOwnerRef",
					Namespace: metav1.NamespaceDefault,
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       "Owner",
							Kind:       machineSetKind.Kind,
							Controller: &controller,
							UID:        "1",
						},
					},
				},
			},
			expected: false,
		},
		{
			machineSet: clusterv1.MachineSet{
				Spec: clusterv1.MachineSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			machine: clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "withMatchingLabels",
					Namespace: metav1.NamespaceDefault,
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			expected: false,
		},
		{
			machineSet: clusterv1.MachineSet{},
			machine: clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "withDeletionTimestamp",
					Namespace:         metav1.NamespaceDefault,
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)

		got := shouldExcludeMachine(&tc.machineSet, &tc.machine)

		g.Expect(got).To(Equal(tc.expected))
	}
}

func TestAdoptOrphan(t *testing.T) {
	g := NewWithT(t)

	m := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "orphanMachine",
		},
	}
	ms := clusterv1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "adoptOrphanMachine",
		},
	}
	controller := true
	blockOwnerDeletion := true
	testCases := []struct {
		machineSet clusterv1.MachineSet
		machine    clusterv1.Machine
		expected   []metav1.OwnerReference
	}{
		{
			machine:    m,
			machineSet: ms,
			expected: []metav1.OwnerReference{
				{
					APIVersion:         clusterv1.GroupVersion.String(),
					Kind:               machineSetKind.Kind,
					Name:               "adoptOrphanMachine",
					UID:                "",
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
	}

	r := &Reconciler{
		Client: fake.NewClientBuilder().WithObjects(&m).Build(),
	}
	for _, tc := range testCases {
		g.Expect(r.adoptOrphan(ctx, tc.machineSet.DeepCopy(), tc.machine.DeepCopy())).To(Succeed())

		key := client.ObjectKey{Namespace: tc.machine.Namespace, Name: tc.machine.Name}
		g.Expect(r.Client.Get(ctx, key, &tc.machine)).To(Succeed())

		got := tc.machine.GetOwnerReferences()
		g.Expect(got).To(Equal(tc.expected))
	}
}

func newMachineSet(name, cluster string, replicas int32) *clusterv1.MachineSet {
	return &clusterv1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: cluster,
			},
		},
		Spec: clusterv1.MachineSetSpec{
			ClusterName: testClusterName,
			Replicas:    &replicas,
			Template: clusterv1.MachineTemplateSpec{
				ObjectMeta: clusterv1.ObjectMeta{
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: cluster,
					},
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					clusterv1.ClusterNameLabel: cluster,
				},
			},
		},
	}
}

func TestMachineSetReconcile_MachinesCreatedConditionFalseOnBadInfraRef(t *testing.T) {
	g := NewWithT(t)
	replicas := int32(1)
	version := "v1.21.0"
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
	}

	ms := &clusterv1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ms-foo",
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: cluster.Name,
			},
		},
		Spec: clusterv1.MachineSetSpec{
			ClusterName: cluster.ObjectMeta.Name,
			Replicas:    &replicas,
			Template: clusterv1.MachineTemplateSpec{
				ObjectMeta: clusterv1.ObjectMeta{
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: cluster.Name,
					},
				},
				Spec: clusterv1.MachineSpec{
					InfrastructureRef: corev1.ObjectReference{
						Kind:       builder.GenericInfrastructureMachineTemplateCRD.Kind,
						APIVersion: builder.GenericInfrastructureMachineTemplateCRD.APIVersion,
						// Try to break Infra Cloning
						Name:      "something_invalid",
						Namespace: cluster.Namespace,
					},
					Version: &version,
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					clusterv1.ClusterNameLabel: cluster.Name,
				},
			},
		},
	}

	key := util.ObjectKey(ms)
	request := reconcile.Request{
		NamespacedName: key,
	}
	fakeClient := fake.NewClientBuilder().WithObjects(cluster, ms, builder.GenericInfrastructureMachineTemplateCRD.DeepCopy()).Build()

	msr := &Reconciler{
		Client:   fakeClient,
		recorder: record.NewFakeRecorder(32),
	}
	_, err := msr.Reconcile(ctx, request)
	g.Expect(err).To(HaveOccurred())
	g.Expect(fakeClient.Get(ctx, key, ms)).To(Succeed())
	gotCond := conditions.Get(ms, clusterv1.MachinesCreatedCondition)
	g.Expect(gotCond).ToNot(BeNil())
	g.Expect(gotCond.Status).To(Equal(corev1.ConditionFalse))
	g.Expect(gotCond.Reason).To(Equal(clusterv1.InfrastructureTemplateCloningFailedReason))
}

func TestMachineSetReconciler_updateStatusResizedCondition(t *testing.T) {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
	}

	testCases := []struct {
		name            string
		machineSet      *clusterv1.MachineSet
		machines        []*clusterv1.Machine
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "MachineSet should have ResizedCondition=false on scale up",
			machineSet:      newMachineSet("ms-scale-up", cluster.Name, int32(1)),
			machines:        []*clusterv1.Machine{},
			expectedReason:  clusterv1.ScalingUpReason,
			expectedMessage: "Scaling up MachineSet to 1 replicas (actual 0)",
		},
		{
			name:       "MachineSet should have ResizedCondition=false on scale down",
			machineSet: newMachineSet("ms-scale-down", cluster.Name, int32(0)),
			machines: []*clusterv1.Machine{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-a",
					Namespace: metav1.NamespaceDefault,
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: cluster.Name,
					},
				},
			},
			},
			expectedReason:  clusterv1.ScalingDownReason,
			expectedMessage: "Scaling down MachineSet to 0 replicas (actual 1)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			msr := &Reconciler{
				Client:   fake.NewClientBuilder().WithObjects().Build(),
				recorder: record.NewFakeRecorder(32),
			}
			err := msr.updateStatus(ctx, cluster, tc.machineSet, tc.machines)
			g.Expect(err).NotTo(HaveOccurred())
			gotCond := conditions.Get(tc.machineSet, clusterv1.ResizedCondition)
			g.Expect(gotCond).ToNot(BeNil())
			g.Expect(gotCond.Status).To(Equal(corev1.ConditionFalse))
			g.Expect(gotCond.Reason).To(Equal(tc.expectedReason))
			g.Expect(gotCond.Message).To(Equal(tc.expectedMessage))
		})
	}
}

func TestMachineSetReconciler_syncMachines(t *testing.T) {
	setup := func(t *testing.T, g *WithT) (*corev1.Namespace, *clusterv1.Cluster) {
		t.Helper()

		t.Log("Creating the namespace")
		ns, err := env.CreateNamespace(ctx, "test-machine-set-reconciler-sync-machines")
		g.Expect(err).To(BeNil())

		t.Log("Creating the Cluster")
		cluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: ns.Name, Name: testClusterName}}
		g.Expect(env.Create(ctx, cluster)).To(Succeed())

		t.Log("Creating the Cluster Kubeconfig Secret")
		g.Expect(env.CreateKubeconfigSecret(ctx, cluster)).To(Succeed())

		return ns, cluster
	}

	teardown := func(t *testing.T, g *WithT, ns *corev1.Namespace, cluster *clusterv1.Cluster) {
		t.Helper()

		t.Log("Deleting the Cluster")
		g.Expect(env.Delete(ctx, cluster)).To(Succeed())
		t.Log("Deleting the namespace")
		g.Expect(env.Delete(ctx, ns)).To(Succeed())
	}

	t.Run("Should sync machines with the in-place mutable changes", func(t *testing.T) {
		g := NewWithT(t)
		namespace, testCluster := setup(t, g)
		defer teardown(t, g, namespace, testCluster)

		replicas := int32(2)
		version := "v1.25.3"
		duration10s := &metav1.Duration{Duration: 10 * time.Second}
		ms := &clusterv1.MachineSet{
			ObjectMeta: metav1.ObjectMeta{
				UID:       "abc-123-ms-uid",
				Name:      "ms-1",
				Namespace: namespace.Name,
				Labels: map[string]string{
					"label-1": "true",
				},
			},
			Spec: clusterv1.MachineSetSpec{
				ClusterName: testCluster.Name,
				Replicas:    &replicas,
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"machine-set-matching-label": "true",
					},
				},
				Template: clusterv1.MachineTemplateSpec{
					ObjectMeta: clusterv1.ObjectMeta{
						Labels: map[string]string{
							"machine-set-matching-label": "true",
						},
						Annotations: map[string]string{
							"annotation-1": "true",
							"precedence":   "MachineSet",
						},
					},
					Spec: clusterv1.MachineSpec{
						ClusterName: testCluster.Name,
						Version:     &version,
						Bootstrap: clusterv1.Bootstrap{
							ConfigRef: &corev1.ObjectReference{
								APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
								Kind:       "GenericBootstrapConfigTemplate",
								Name:       "ms-template",
							},
						},
						InfrastructureRef: corev1.ObjectReference{
							APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
							Kind:       "GenericInfrastructureMachineTemplate",
							Name:       "ms-template",
						},
						NodeDrainTimeout:        duration10s,
						NodeDeletionTimeout:     duration10s,
						NodeVolumeDetachTimeout: duration10s,
					},
				},
			},
		}

		fieldV1Map := map[string]interface{}{
			"f:metadata": map[string]interface{}{
				"f:name": map[string]interface{}{},
			},
		}
		fieldV1, err := json.Marshal(fieldV1Map)
		g.Expect(err).To(BeNil())
		inPlaceMutatingMachine := &clusterv1.Machine{
			TypeMeta: metav1.TypeMeta{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Machine",
			},
			ObjectMeta: metav1.ObjectMeta{
				UID:         "abc-123-uid",
				Name:        "in-place-mutating-machine",
				Namespace:   namespace.Name,
				Labels:      map[string]string{},
				Annotations: map[string]string{},
				ManagedFields: []metav1.ManagedFieldsEntry{
					{
						Manager:    "manager",
						Operation:  metav1.ManagedFieldsOperationUpdate,
						FieldsType: "FieldsV1",
						FieldsV1:   &metav1.FieldsV1{Raw: fieldV1},
					},
				},
			},
			Spec: clusterv1.MachineSpec{
				ClusterName: testClusterName,
				InfrastructureRef: corev1.ObjectReference{
					Namespace: namespace.Name,
				},
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: pointer.String("machine-bootstrap-secret"),
				},
			},
		}
		g.Expect(env.Create(ctx, inPlaceMutatingMachine)).To(Succeed())

		deletingMachine := &clusterv1.Machine{
			TypeMeta: metav1.TypeMeta{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Machine",
			},
			ObjectMeta: metav1.ObjectMeta{
				UID:         "abc-123-uid",
				Name:        "deleting-machine",
				Namespace:   namespace.Name,
				Labels:      map[string]string{},
				Annotations: map[string]string{},
				ManagedFields: []metav1.ManagedFieldsEntry{
					{
						Manager:    "manager",
						Operation:  metav1.ManagedFieldsOperationUpdate,
						FieldsType: "FieldsV1",
						FieldsV1:   &metav1.FieldsV1{Raw: fieldV1},
					},
				},
				Finalizers: []string{"testing-finalizer"},
			},
			Spec: clusterv1.MachineSpec{
				ClusterName: testClusterName,
				InfrastructureRef: corev1.ObjectReference{
					Namespace: namespace.Name,
				},
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: pointer.String("machine-bootstrap-secret"),
				},
			},
		}
		g.Expect(env.Create(ctx, deletingMachine)).To(Succeed())
		// Delete the machine to put it in the deleting state
		g.Expect(env.Delete(ctx, deletingMachine)).To(Succeed())
		// Wait till the machine is marked for deletion
		g.Eventually(func() bool {
			if err := env.Get(ctx, client.ObjectKeyFromObject(deletingMachine), deletingMachine); err != nil {
				return false
			}
			return !deletingMachine.DeletionTimestamp.IsZero()
		}, timeout).Should(BeTrue())

		machines := []*clusterv1.Machine{inPlaceMutatingMachine, deletingMachine}

		// Sync the Machines.
		reconciler := &Reconciler{Client: env}
		g.Expect(reconciler.syncMachines(ctx, ms, machines)).To(Succeed())

		// The in-place mutating machine should have:
		// - clean-up managed fields
		// - updated in-place propagating values
		updatedInPlaceMutatingMachine := machines[0]
		// Verify ManagedFields
		g.Expect(updatedInPlaceMutatingMachine.ManagedFields).Should(
			ContainElement(ssa.MatchManagedField(machineSetManagerName, metav1.ManagedFieldsOperationApply)),
			"in-place mutable machine should contain an entry for SSA manager",
		)
		g.Expect(updatedInPlaceMutatingMachine.ManagedFields).ShouldNot(
			ContainElement(ssa.MatchManagedField("manager", metav1.ManagedFieldsOperationUpdate)),
			"in-place mutable machine should not contain an entry for old manager",
		)
		// Verify Labels
		g.Expect(updatedInPlaceMutatingMachine.Labels).Should(HaveKeyWithValue("machine-set-matching-label", "true"))
		// Verify Annotations
		g.Expect(updatedInPlaceMutatingMachine.Annotations).Should(HaveKeyWithValue("annotation-1", "true"))
		g.Expect(updatedInPlaceMutatingMachine.Annotations).Should(HaveKeyWithValue("precedence", "MachineSet"))
		// Verify Node timeout values
		g.Expect(updatedInPlaceMutatingMachine.Spec.NodeDrainTimeout).Should(And(
			Not(BeNil()),
			HaveValue(Equal(*ms.Spec.Template.Spec.NodeDrainTimeout)),
		))
		g.Expect(updatedInPlaceMutatingMachine.Spec.NodeDeletionTimeout).Should(And(
			Not(BeNil()),
			HaveValue(Equal(*ms.Spec.Template.Spec.NodeDeletionTimeout)),
		))
		g.Expect(updatedInPlaceMutatingMachine.Spec.NodeVolumeDetachTimeout).Should(And(
			Not(BeNil()),
			HaveValue(Equal(*ms.Spec.Template.Spec.NodeDrainTimeout)),
		))

		// The deleting machine should not change.
		updatedDeletingMachine := machines[1]
		g.Expect(updatedDeletingMachine.ManagedFields).ShouldNot(
			ContainElement(ssa.MatchManagedField(machineSetManagerName, metav1.ManagedFieldsOperationApply)),
			"deleting machine should not contain an entry for SSA manager",
		)
		g.Expect(updatedDeletingMachine.Labels).Should(Equal(deletingMachine.Labels))
		g.Expect(updatedDeletingMachine.Annotations).Should(Equal(deletingMachine.Annotations))
		g.Expect(updatedDeletingMachine.Spec.NodeDrainTimeout).Should(Equal(deletingMachine.Spec.NodeDrainTimeout))
		g.Expect(updatedDeletingMachine.Spec.NodeDeletionTimeout).Should(Equal(deletingMachine.Spec.NodeDeletionTimeout))
		g.Expect(updatedDeletingMachine.Spec.NodeVolumeDetachTimeout).Should(Equal(deletingMachine.Spec.NodeVolumeDetachTimeout))
	})
}

func TestComputeDesiredMachine(t *testing.T) {
	duration5s := &metav1.Duration{Duration: 5 * time.Second}
	duration10s := &metav1.Duration{Duration: 10 * time.Second}

	infraRef := corev1.ObjectReference{
		Kind:       "GenericInfrastructureMachineTemplate",
		Name:       "infra-template-1",
		APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
	}
	bootstrapRef := corev1.ObjectReference{
		Kind:       "GenericBootstrapConfigTemplate",
		Name:       "bootstrap-template-1",
		APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
	}

	ms := &clusterv1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "ms1",
			Labels: map[string]string{
				clusterv1.MachineDeploymentNameLabel: "md1",
			},
		},
		Spec: clusterv1.MachineSetSpec{
			ClusterName:     "test-cluster",
			Replicas:        pointer.Int32(3),
			MinReadySeconds: 10,
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"k1": "v1"},
			},
			Template: clusterv1.MachineTemplateSpec{
				ObjectMeta: clusterv1.ObjectMeta{
					Labels:      map[string]string{"machine-label1": "machine-value1"},
					Annotations: map[string]string{"machine-annotation1": "machine-value1"},
				},
				Spec: clusterv1.MachineSpec{
					Version:           pointer.String("v1.25.3"),
					InfrastructureRef: infraRef,
					Bootstrap: clusterv1.Bootstrap{
						ConfigRef: &bootstrapRef,
					},
					NodeDrainTimeout:        duration10s,
					NodeVolumeDetachTimeout: duration10s,
					NodeDeletionTimeout:     duration10s,
				},
			},
		},
	}

	skeletonMachine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Labels: map[string]string{
				"machine-label1":                     "machine-value1",
				clusterv1.MachineSetNameLabel:        "ms1",
				clusterv1.MachineDeploymentNameLabel: "md1",
			},
			Annotations: map[string]string{"machine-annotation1": "machine-value1"},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName:             "test-cluster",
			Version:                 pointer.String("v1.25.3"),
			NodeDrainTimeout:        duration10s,
			NodeVolumeDetachTimeout: duration10s,
			NodeDeletionTimeout:     duration10s,
		},
	}

	// Creating a new Machine
	expectedNewMachine := skeletonMachine.DeepCopy()

	// Updating an existing Machine
	existingMachine := skeletonMachine.DeepCopy()
	existingMachine.Name = "exiting-machine-1"
	existingMachine.UID = "abc-123-existing-machine-1"
	existingMachine.Labels = nil
	existingMachine.Annotations = nil
	existingMachine.Spec.InfrastructureRef = corev1.ObjectReference{
		Kind:       "GenericInfrastructureMachine",
		Name:       "infra-machine-1",
		APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
	}
	existingMachine.Spec.Bootstrap.ConfigRef = &corev1.ObjectReference{
		Kind:       "GenericBootstrapConfig",
		Name:       "bootstrap-config-1",
		APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
	}
	existingMachine.Spec.NodeDrainTimeout = duration5s
	existingMachine.Spec.NodeDeletionTimeout = duration5s
	existingMachine.Spec.NodeVolumeDetachTimeout = duration5s

	expectedUpdatedMachine := skeletonMachine.DeepCopy()
	expectedUpdatedMachine.Name = existingMachine.Name
	expectedUpdatedMachine.UID = existingMachine.UID
	expectedUpdatedMachine.Spec.InfrastructureRef = *existingMachine.Spec.InfrastructureRef.DeepCopy()
	expectedUpdatedMachine.Spec.Bootstrap.ConfigRef = existingMachine.Spec.Bootstrap.ConfigRef.DeepCopy()

	tests := []struct {
		name            string
		existingMachine *clusterv1.Machine
		want            *clusterv1.Machine
	}{
		{
			name:            "creating a new Machine",
			existingMachine: nil,
			want:            expectedNewMachine,
		},
		{
			name:            "updating an existing Machine",
			existingMachine: existingMachine,
			want:            expectedUpdatedMachine,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got := (&Reconciler{}).computeDesiredMachine(ms, tt.existingMachine)
			assertMachine(g, got, tt.want)
		})
	}
}

func assertMachine(g *WithT, actualMachine *clusterv1.Machine, expectedMachine *clusterv1.Machine) {
	// Check Name
	if expectedMachine.Name != "" {
		g.Expect(actualMachine.Name).Should(Equal(expectedMachine.Name))
	}
	// Check UID
	if expectedMachine.UID != "" {
		g.Expect(actualMachine.UID).Should(Equal(expectedMachine.UID))
	}
	// Check Namespace
	g.Expect(actualMachine.Namespace).Should(Equal(expectedMachine.Namespace))
	// Check Labels
	for k, v := range expectedMachine.Labels {
		g.Expect(actualMachine.Labels).Should(HaveKeyWithValue(k, v))
	}
	// Check Annotations
	for k, v := range expectedMachine.Annotations {
		g.Expect(actualMachine.Annotations).Should(HaveKeyWithValue(k, v))
	}
	// Check Spec
	g.Expect(actualMachine.Spec).Should(Equal(expectedMachine.Spec))
}
