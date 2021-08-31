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

package topology

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/internal/testtypes"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCalculateTemplatesInUse(t *testing.T) {
	t.Run("Calculate templates in use with regular MachineDeployment and MachineSet", func(t *testing.T) {
		g := NewWithT(t)

		md := testtypes.NewMachineDeploymentBuilder(metav1.NamespaceDefault, "md").
			WithBootstrapTemplate(testtypes.NewBootstrapTemplateBuilder(metav1.NamespaceDefault, "mdBT").Build()).
			WithInfrastructureTemplate(testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "mdIMT").Build()).
			Build()
		ms := testtypes.NewMachineSetBuilder(metav1.NamespaceDefault, "ms").
			WithBootstrapTemplate(testtypes.NewBootstrapTemplateBuilder(metav1.NamespaceDefault, "msBT").Build()).
			WithInfrastructureTemplate(testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "msIMT").Build()).
			Build()

		actual, err := calculateTemplatesInUse(md, []*clusterv1.MachineSet{ms})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(actual).To(HaveLen(4))

		g.Expect(actual).To(HaveKey(mustRefID(md.Spec.Template.Spec.Bootstrap.ConfigRef)))
		g.Expect(actual).To(HaveKey(mustRefID(&md.Spec.Template.Spec.InfrastructureRef)))

		g.Expect(actual).To(HaveKey(mustRefID(ms.Spec.Template.Spec.Bootstrap.ConfigRef)))
		g.Expect(actual).To(HaveKey(mustRefID(&ms.Spec.Template.Spec.InfrastructureRef)))
	})

	t.Run("Calculate templates in use with MachineDeployment and MachineSet without BootstrapTemplate", func(t *testing.T) {
		g := NewWithT(t)

		mdWithoutBootstrapTemplate := testtypes.NewMachineDeploymentBuilder(metav1.NamespaceDefault, "md").
			WithInfrastructureTemplate(testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "mdIMT").Build()).
			Build()
		msWithoutBootstrapTemplate := testtypes.NewMachineSetBuilder(metav1.NamespaceDefault, "ms").
			WithInfrastructureTemplate(testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "msIMT").Build()).
			Build()

		actual, err := calculateTemplatesInUse(mdWithoutBootstrapTemplate, []*clusterv1.MachineSet{msWithoutBootstrapTemplate})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(actual).To(HaveLen(2))

		g.Expect(actual).To(HaveKey(mustRefID(&mdWithoutBootstrapTemplate.Spec.Template.Spec.InfrastructureRef)))

		g.Expect(actual).To(HaveKey(mustRefID(&msWithoutBootstrapTemplate.Spec.Template.Spec.InfrastructureRef)))
	})

	t.Run("Calculate templates in use with MachineDeployment and MachineSet ignore templates when resources in deleting", func(t *testing.T) {
		g := NewWithT(t)

		deletionTimeStamp := metav1.Now()

		mdInDeleting := testtypes.NewMachineDeploymentBuilder(metav1.NamespaceDefault, "md").
			WithInfrastructureTemplate(testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "mdIMT").Build()).
			Build()
		mdInDeleting.SetDeletionTimestamp(&deletionTimeStamp)

		msInDeleting := testtypes.NewMachineSetBuilder(metav1.NamespaceDefault, "ms").
			WithInfrastructureTemplate(testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "msIMT").Build()).
			Build()
		msInDeleting.SetDeletionTimestamp(&deletionTimeStamp)

		actual, err := calculateTemplatesInUse(mdInDeleting, []*clusterv1.MachineSet{msInDeleting})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(actual).To(HaveLen(0))

		g.Expect(actual).ToNot(HaveKey(mustRefID(&mdInDeleting.Spec.Template.Spec.InfrastructureRef)))

		g.Expect(actual).ToNot(HaveKey(mustRefID(&msInDeleting.Spec.Template.Spec.InfrastructureRef)))
	})

	t.Run("Calculate templates in use without MachineDeployment and with MachineSet", func(t *testing.T) {
		g := NewWithT(t)

		ms := testtypes.NewMachineSetBuilder(metav1.NamespaceDefault, "ms").
			WithBootstrapTemplate(testtypes.NewBootstrapTemplateBuilder(metav1.NamespaceDefault, "msBT").Build()).
			WithInfrastructureTemplate(testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "msIMT").Build()).
			Build()

		actual, err := calculateTemplatesInUse(nil, []*clusterv1.MachineSet{ms})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(actual).To(HaveLen(2))

		g.Expect(actual).To(HaveKey(mustRefID(ms.Spec.Template.Spec.Bootstrap.ConfigRef)))
		g.Expect(actual).To(HaveKey(mustRefID(&ms.Spec.Template.Spec.InfrastructureRef)))
	})
}

func TestDeleteMachineSetsTemplatesIfNecessary(t *testing.T) {
	deletionTimeStamp := metav1.Now()

	msBT := testtypes.NewBootstrapTemplateBuilder(metav1.NamespaceDefault, "msBT").Build()
	msIMT := testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "msIMT").Build()
	ms := testtypes.NewMachineSetBuilder(metav1.NamespaceDefault, "ms").
		WithBootstrapTemplate(msBT).
		WithInfrastructureTemplate(msIMT).
		Build()

	msPausedIMT := testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "msInDeletingIMT").Build()
	msPaused := testtypes.NewMachineSetBuilder(metav1.NamespaceDefault, "msInDeleting").
		WithInfrastructureTemplate(msPausedIMT).
		Build()
	annotations.AddAnnotations(msPaused, map[string]string{clusterv1.PausedAnnotation: ""})
	msPaused.SetDeletionTimestamp(&deletionTimeStamp)

	msInDeletingBT := testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "msInDeletingBT").Build()
	msInDeletingIMT := testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "msInDeletingIMT").Build()
	msInDeleting := testtypes.NewMachineSetBuilder(metav1.NamespaceDefault, "msInDeleting").
		WithBootstrapTemplate(msInDeletingBT).
		WithInfrastructureTemplate(msInDeletingIMT).
		Build()
	msInDeleting.SetDeletionTimestamp(&deletionTimeStamp)

	msWithoutBootstrapTemplateInDeletingIMT := testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "msWithoutBootstrapTemplateInDeletingIMT").Build()
	msWithoutBootstrapTemplateInDeleting := testtypes.NewMachineSetBuilder(metav1.NamespaceDefault, "msWithoutBootstrapTemplateInDeleting").
		WithInfrastructureTemplate(msWithoutBootstrapTemplateInDeletingIMT).
		Build()
	msWithoutBootstrapTemplateInDeleting.SetDeletionTimestamp(&deletionTimeStamp)

	t.Run("Should not delete templates of a MachineSet not in deleting", func(t *testing.T) {
		g := NewWithT(t)

		fakeClient := fake.NewClientBuilder().
			WithScheme(fakeScheme).
			WithObjects(ms, msBT, msIMT).
			Build()

		r := &MachineDeploymentReconciler{
			Client: fakeClient,
		}
		err := r.deleteMachineSetsTemplatesIfNecessary(ctx, map[string]bool{}, []*clusterv1.MachineSet{ms})
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(templateExists(fakeClient, msBT)).To(BeTrue())
		g.Expect(templateExists(fakeClient, msIMT)).To(BeTrue())
	})

	t.Run("Should not delete templates of a paused MachineSet", func(t *testing.T) {
		g := NewWithT(t)

		fakeClient := fake.NewClientBuilder().
			WithScheme(fakeScheme).
			WithObjects(msPaused, msPausedIMT).
			Build()

		r := &MachineDeploymentReconciler{
			Client: fakeClient,
		}
		err := r.deleteMachineSetsTemplatesIfNecessary(ctx, map[string]bool{}, []*clusterv1.MachineSet{msPaused})
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(templateExists(fakeClient, msPausedIMT)).To(BeTrue())
	})

	t.Run("Should delete templates of a MachineSet in deleting", func(t *testing.T) {
		g := NewWithT(t)

		fakeClient := fake.NewClientBuilder().
			WithScheme(fakeScheme).
			WithObjects(msInDeleting, msInDeletingBT, msInDeletingIMT).
			Build()

		r := &MachineDeploymentReconciler{
			Client: fakeClient,
		}
		err := r.deleteMachineSetsTemplatesIfNecessary(ctx, map[string]bool{}, []*clusterv1.MachineSet{msInDeleting})
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(templateExists(fakeClient, msInDeletingBT)).To(BeFalse())
		g.Expect(templateExists(fakeClient, msInDeletingIMT)).To(BeFalse())
	})

	t.Run("Should not delete templates of a MachineSet in deleting when they are still in use", func(t *testing.T) {
		g := NewWithT(t)

		fakeClient := fake.NewClientBuilder().
			WithScheme(fakeScheme).
			WithObjects(msInDeleting, msInDeletingBT, msInDeletingIMT).
			Build()

		templatesInUse := map[string]bool{
			mustRefID(msInDeleting.Spec.Template.Spec.Bootstrap.ConfigRef): true,
			mustRefID(&msInDeleting.Spec.Template.Spec.InfrastructureRef):  true,
		}

		r := &MachineDeploymentReconciler{
			Client: fakeClient,
		}
		err := r.deleteMachineSetsTemplatesIfNecessary(ctx, templatesInUse, []*clusterv1.MachineSet{msInDeleting})
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(templateExists(fakeClient, msInDeletingBT)).To(BeTrue())
		g.Expect(templateExists(fakeClient, msInDeletingIMT)).To(BeTrue())
	})

	t.Run("Should delete infra template of a MachineSet without a bootstrap template in deleting", func(t *testing.T) {
		g := NewWithT(t)

		fakeClient := fake.NewClientBuilder().
			WithScheme(fakeScheme).
			WithObjects(msWithoutBootstrapTemplateInDeleting, msWithoutBootstrapTemplateInDeletingIMT).
			Build()

		r := &MachineDeploymentReconciler{
			Client: fakeClient,
		}
		err := r.deleteMachineSetsTemplatesIfNecessary(ctx, map[string]bool{}, []*clusterv1.MachineSet{msWithoutBootstrapTemplateInDeleting})
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(templateExists(fakeClient, msWithoutBootstrapTemplateInDeletingIMT)).To(BeFalse())
	})
}

func TestDeleteMachineDeploymentTemplatesIfNecessary(t *testing.T) {
	deletionTimeStamp := metav1.Now()

	mdBT := testtypes.NewBootstrapTemplateBuilder(metav1.NamespaceDefault, "mdBT").Build()
	mdIMT := testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "mdIMT").Build()
	md := testtypes.NewMachineDeploymentBuilder(metav1.NamespaceDefault, "md").
		WithBootstrapTemplate(mdBT).
		WithInfrastructureTemplate(mdIMT).
		Build()

	mdPausedIMT := testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "mdInDeletingIMT").Build()
	mdPaused := testtypes.NewMachineDeploymentBuilder(metav1.NamespaceDefault, "mdInDeleting").
		WithInfrastructureTemplate(mdPausedIMT).
		Build()
	annotations.AddAnnotations(mdPaused, map[string]string{clusterv1.PausedAnnotation: ""})
	mdPaused.SetDeletionTimestamp(&deletionTimeStamp)

	mdInDeletingBT := testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "mdInDeletingBT").Build()
	mdInDeletingIMT := testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "mdInDeletingIMT").Build()
	mdInDeleting := testtypes.NewMachineDeploymentBuilder(metav1.NamespaceDefault, "mdInDeleting").
		WithBootstrapTemplate(mdInDeletingBT).
		WithInfrastructureTemplate(mdInDeletingIMT).
		Build()
	mdInDeleting.SetDeletionTimestamp(&deletionTimeStamp)

	mdWithoutBootstrapTemplateInDeletingIMT := testtypes.NewInfrastructureMachineTemplateBuilder(metav1.NamespaceDefault, "mdWithoutBootstrapTemplateInDeletingIMT").Build()
	mdWithoutBootstrapTemplateInDeleting := testtypes.NewMachineDeploymentBuilder(metav1.NamespaceDefault, "mdWithoutBootstrapTemplateInDeleting").
		WithInfrastructureTemplate(mdWithoutBootstrapTemplateInDeletingIMT).
		Build()
	mdWithoutBootstrapTemplateInDeleting.SetDeletionTimestamp(&deletionTimeStamp)

	t.Run("Should not fail if MachineDeployment does not exist", func(t *testing.T) {
		g := NewWithT(t)

		fakeClient := fake.NewClientBuilder().
			WithScheme(fakeScheme).
			WithObjects().
			Build()

		r := &MachineDeploymentReconciler{
			Client: fakeClient,
		}
		err := r.deleteMachineDeploymentTemplatesIfNecessary(ctx, map[string]bool{}, nil)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("Should not delete templates of a MachineDeployment not in deleting", func(t *testing.T) {
		g := NewWithT(t)

		fakeClient := fake.NewClientBuilder().
			WithScheme(fakeScheme).
			WithObjects(md, mdBT, mdIMT).
			Build()

		r := &MachineDeploymentReconciler{
			Client: fakeClient,
		}
		err := r.deleteMachineDeploymentTemplatesIfNecessary(ctx, map[string]bool{}, md)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(templateExists(fakeClient, mdBT)).To(BeTrue())
		g.Expect(templateExists(fakeClient, mdIMT)).To(BeTrue())
	})

	t.Run("Should not delete templates of a paused MachineDeployment", func(t *testing.T) {
		g := NewWithT(t)

		fakeClient := fake.NewClientBuilder().
			WithScheme(fakeScheme).
			WithObjects(mdPaused, mdPausedIMT).
			Build()

		r := &MachineDeploymentReconciler{
			Client: fakeClient,
		}
		err := r.deleteMachineDeploymentTemplatesIfNecessary(ctx, map[string]bool{}, mdPaused)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(templateExists(fakeClient, mdPausedIMT)).To(BeTrue())
	})

	t.Run("Should delete templates of a MachineDeployment in deleting", func(t *testing.T) {
		g := NewWithT(t)

		fakeClient := fake.NewClientBuilder().
			WithScheme(fakeScheme).
			WithObjects(mdInDeleting, mdInDeletingBT, mdInDeletingIMT).
			Build()

		r := &MachineDeploymentReconciler{
			Client: fakeClient,
		}
		err := r.deleteMachineDeploymentTemplatesIfNecessary(ctx, map[string]bool{}, mdInDeleting)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(templateExists(fakeClient, mdInDeletingBT)).To(BeFalse())
		g.Expect(templateExists(fakeClient, mdInDeletingIMT)).To(BeFalse())
	})

	t.Run("Should not delete templates of a MachineDeployment in deleting when they are still in use", func(t *testing.T) {
		g := NewWithT(t)

		fakeClient := fake.NewClientBuilder().
			WithScheme(fakeScheme).
			WithObjects(mdInDeleting, mdInDeletingBT, mdInDeletingIMT).
			Build()

		templatesInUse := map[string]bool{
			mustRefID(mdInDeleting.Spec.Template.Spec.Bootstrap.ConfigRef): true,
			mustRefID(&mdInDeleting.Spec.Template.Spec.InfrastructureRef):  true,
		}

		r := &MachineDeploymentReconciler{
			Client: fakeClient,
		}
		err := r.deleteMachineDeploymentTemplatesIfNecessary(ctx, templatesInUse, mdInDeleting)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(templateExists(fakeClient, mdInDeletingBT)).To(BeTrue())
		g.Expect(templateExists(fakeClient, mdInDeletingIMT)).To(BeTrue())
	})

	t.Run("Should delete infra template of a MachineDeployment without a bootstrap template in deleting", func(t *testing.T) {
		g := NewWithT(t)

		fakeClient := fake.NewClientBuilder().
			WithScheme(fakeScheme).
			WithObjects(mdWithoutBootstrapTemplateInDeleting, mdWithoutBootstrapTemplateInDeletingIMT).
			Build()

		r := &MachineDeploymentReconciler{
			Client: fakeClient,
		}
		err := r.deleteMachineDeploymentTemplatesIfNecessary(ctx, map[string]bool{}, mdWithoutBootstrapTemplateInDeleting)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(templateExists(fakeClient, mdWithoutBootstrapTemplateInDeletingIMT)).To(BeFalse())
	})
}

func templateExists(fakeClient client.Reader, template *unstructured.Unstructured) bool {
	obj := &unstructured.Unstructured{}
	obj.SetKind(template.GetKind())
	obj.SetAPIVersion(template.GetAPIVersion())

	err := fakeClient.Get(ctx, client.ObjectKeyFromObject(template), obj)
	if err != nil && !apierrors.IsNotFound(err) {
		// This should never happen.
		panic(errors.Wrapf(err, "failed to get %s/%s", template.GetKind(), template.GetName()))
	}
	return err == nil
}

func mustRefID(ref *corev1.ObjectReference) string {
	refID, err := refID(ref)
	if err != nil {
		panic(errors.Wrap(err, "failed to calculate refID"))
	}
	return refID
}
