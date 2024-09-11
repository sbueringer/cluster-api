/*
Copyright 2024 The Kubernetes Authors.

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

package v1beta2

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/internal/test/builder"
)

func TestSetAll(t *testing.T) {
	now := metav1.Now().Rfc3339Copy()

	conditions := []metav1.Condition{
		{
			Type:               "fooCondition",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: 10,
			LastTransitionTime: now,
			Reason:             "FooReason",
			Message:            "FooMessage",
		},
	}

	cloneConditions := func() []metav1.Condition {
		ret := make([]metav1.Condition, len(conditions))
		copy(ret, conditions)
		return ret
	}

	t.Run("fails with nil", func(t *testing.T) {
		g := NewWithT(t)

		conditions := cloneConditions()
		err := SetAll(nil, conditions)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("fails for nil object", func(t *testing.T) {
		g := NewWithT(t)
		var foo *builder.Phase0Obj

		conditions := cloneConditions()
		err := SetAll(foo, conditions)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("fails for Unstructured", func(t *testing.T) {
		g := NewWithT(t)
		fooUnstructured := &unstructured.Unstructured{}

		conditions := cloneConditions()
		err := SetAll(fooUnstructured, conditions)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("v1beta object with legacy conditions", func(t *testing.T) {
		g := NewWithT(t)
		foo := &builder.Phase0Obj{
			Status: builder.Phase0ObjStatus{Conditions: nil},
		}

		conditions := cloneConditions()
		err := SetAll(foo, conditions)
		g.Expect(err).To(HaveOccurred()) // Can't set legacy conditions.
	})

	t.Run("v1beta1 object with both legacy and v1beta2 conditions", func(t *testing.T) {
		g := NewWithT(t)
		foo := &builder.Phase1Obj{
			Status: builder.Phase1ObjStatus{
				Conditions: clusterv1.Conditions{
					{
						Type:               "barCondition",
						Status:             corev1.ConditionFalse,
						LastTransitionTime: now,
					},
				},
				V1Beta2: builder.Phase1ObjStatusV1beta2{Conditions: nil},
			},
		}

		conditions := cloneConditions()
		err := SetAll(foo, conditions)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(foo.Status.V1Beta2.Conditions).To(Equal(conditions), cmp.Diff(foo.Status.V1Beta2.Conditions, conditions))
	})

	t.Run("v1beta2 object with conditions and backward compatible conditions", func(t *testing.T) {
		g := NewWithT(t)
		foo := &builder.Phase2Obj{
			Status: builder.Phase2ObjStatus{
				Conditions: nil,
				Deprecated: builder.Phase2ObjStatusDeprecated{
					V1Beta1: builder.Phase2ObjStatusDeprecatedV1Beta2{
						Conditions: clusterv1.Conditions{
							{
								Type:               "barCondition",
								Status:             corev1.ConditionFalse,
								LastTransitionTime: now,
							},
						},
					},
				},
			},
		}

		conditions := cloneConditions()
		err := SetAll(foo, conditions)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(foo.Status.Conditions).To(MatchConditions(conditions), cmp.Diff(foo.Status.Conditions, conditions))
	})

	t.Run("v1beta2 object with conditions (end state)", func(t *testing.T) {
		g := NewWithT(t)
		foo := &builder.Phase3Obj{
			Status: builder.Phase3ObjStatus{
				Conditions: nil,
			},
		}

		conditions := cloneConditions()
		err := SetAll(foo, conditions)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(foo.Status.Conditions).To(Equal(conditions), cmp.Diff(foo.Status.Conditions, conditions))
	})

	t.Run("Set infers ObservedGeneration", func(t *testing.T) {
		g := NewWithT(t)
		foo := &builder.Phase3Obj{
			ObjectMeta: metav1.ObjectMeta{Generation: 123},
			Status: builder.Phase3ObjStatus{
				Conditions: nil,
			},
		}

		condition := metav1.Condition{
			Type:               "fooCondition",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: now,
			Reason:             "FooReason",
			Message:            "FooMessage",
		}

		err := Set(foo, condition)
		g.Expect(err).ToNot(HaveOccurred())

		condition.ObservedGeneration = foo.Generation
		conditions := []metav1.Condition{condition}
		g.Expect(foo.Status.Conditions).To(Equal(conditions), cmp.Diff(foo.Status.Conditions, conditions))
	})
}
