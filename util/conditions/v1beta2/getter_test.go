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
	"k8s.io/apimachinery/pkg/runtime"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/internal/test/builder"
)

type ObjWithoutStatus struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (f *ObjWithoutStatus) DeepCopyObject() runtime.Object {
	panic("implement me")
}

type ObjWithStatusWithoutConditions struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Status struct {
	}
}

func (f *ObjWithStatusWithoutConditions) DeepCopyObject() runtime.Object {
	panic("implement me")
}

type ObjWithWrongConditionType struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Status struct {
		Conditions []string
	}
}

func (f *ObjWithWrongConditionType) DeepCopyObject() runtime.Object {
	panic("implement me")
}

type ObjWithWrongV1beta2ConditionType struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Status struct {
		Conditions clusterv1.Conditions
		V1Beta2    struct {
			Conditions []string
		}
	}
}

func (f *ObjWithWrongV1beta2ConditionType) DeepCopyObject() runtime.Object {
	panic("implement me")
}

func TestGetAll(t *testing.T) {
	now := metav1.Time{Time: metav1.Now().Rfc3339Copy().UTC()}

	t.Run("fails with nil", func(t *testing.T) {
		g := NewWithT(t)

		_, err := GetAll(nil)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("fails for nil object", func(t *testing.T) {
		g := NewWithT(t)
		var foo *builder.Phase0Obj

		_, err := GetAll(foo)
		g.Expect(err).To(HaveOccurred())

		var fooUnstructured *unstructured.Unstructured

		_, err = GetAll(fooUnstructured)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("fails for object without status", func(t *testing.T) {
		g := NewWithT(t)
		foo := &ObjWithoutStatus{}

		_, err := GetAll(foo)
		g.Expect(err).To(HaveOccurred())

		fooUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(foo)
		g.Expect(err).NotTo(HaveOccurred())

		_, err = GetAll(&unstructured.Unstructured{Object: fooUnstructured})
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("fails for object with status without conditions or v1beta2 conditions", func(t *testing.T) {
		g := NewWithT(t)
		foo := &ObjWithStatusWithoutConditions{}

		_, err := GetAll(foo)
		g.Expect(err).To(HaveOccurred())

		fooUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(foo)
		g.Expect(err).NotTo(HaveOccurred())

		_, err = GetAll(&unstructured.Unstructured{Object: fooUnstructured})
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("v1beta object with legacy conditions", func(t *testing.T) {
		g := NewWithT(t)
		foo := &builder.Phase0Obj{
			Status: builder.Phase0ObjStatus{
				Conditions: clusterv1.Conditions{
					{
						Type:               "barCondition",
						Status:             corev1.ConditionTrue,
						LastTransitionTime: now,
					},
					{
						Type:               "fooCondition",
						Status:             corev1.ConditionFalse,
						LastTransitionTime: now,
						Reason:             "FooReason",
						Message:            "FooMessage",
					},
				},
			},
		}

		expect := []metav1.Condition{
			{
				Type:               string(foo.Status.Conditions[0].Type),
				Status:             metav1.ConditionStatus(foo.Status.Conditions[0].Status),
				LastTransitionTime: foo.Status.Conditions[0].LastTransitionTime,
				Reason:             NoReasonReported,
			},
			{
				Type:               string(foo.Status.Conditions[1].Type),
				Status:             metav1.ConditionStatus(foo.Status.Conditions[1].Status),
				LastTransitionTime: foo.Status.Conditions[1].LastTransitionTime,
				Reason:             foo.Status.Conditions[1].Reason,
				Message:            foo.Status.Conditions[1].Message,
			},
		}

		got, err := GetAll(foo)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(MatchConditions(expect), cmp.Diff(got, expect))

		fooUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(foo)
		g.Expect(err).NotTo(HaveOccurred())

		got, err = GetAll(&unstructured.Unstructured{Object: fooUnstructured})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(MatchConditions(expect, IgnoreLastTransitionTime(true)), cmp.Diff(got, expect))
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
				V1Beta2: builder.Phase1ObjStatusV1beta2{
					Conditions: []metav1.Condition{
						{
							Type:               "fooCondition",
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 10,
							LastTransitionTime: now,
							Reason:             "FooReason",
							Message:            "FooMessage",
						},
					},
				},
			},
		}

		expect := []metav1.Condition{
			{
				Type:               foo.Status.V1Beta2.Conditions[0].Type,
				Status:             foo.Status.V1Beta2.Conditions[0].Status,
				LastTransitionTime: foo.Status.V1Beta2.Conditions[0].LastTransitionTime,
				ObservedGeneration: foo.Status.V1Beta2.Conditions[0].ObservedGeneration,
				Reason:             foo.Status.V1Beta2.Conditions[0].Reason,
				Message:            foo.Status.V1Beta2.Conditions[0].Message,
			},
		}

		got, err := GetAll(foo)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(MatchConditions(expect, IgnoreLastTransitionTime(true)), cmp.Diff(got, expect))

		fooUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(foo)
		g.Expect(err).NotTo(HaveOccurred())

		got, err = GetAll(&unstructured.Unstructured{Object: fooUnstructured})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(MatchConditions(expect, IgnoreLastTransitionTime(true)), cmp.Diff(got, expect))
	})

	t.Run("v1beta2 object with conditions and backward compatible conditions", func(t *testing.T) {
		g := NewWithT(t)
		foo := &builder.Phase2Obj{
			Status: builder.Phase2ObjStatus{
				Conditions: []metav1.Condition{
					{
						Type:               "fooCondition",
						Status:             metav1.ConditionTrue,
						LastTransitionTime: now,
						Reason:             "fooReason",
					},
				},
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

		expect := []metav1.Condition{
			{
				Type:               foo.Status.Conditions[0].Type,
				Status:             foo.Status.Conditions[0].Status,
				LastTransitionTime: foo.Status.Conditions[0].LastTransitionTime,
				Reason:             foo.Status.Conditions[0].Reason,
			},
		}

		got, err := GetAll(foo)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(MatchConditions(expect, IgnoreLastTransitionTime(true)), cmp.Diff(got, expect))

		fooUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(foo)
		g.Expect(err).NotTo(HaveOccurred())

		got, err = GetAll(&unstructured.Unstructured{Object: fooUnstructured})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(MatchConditions(expect, IgnoreLastTransitionTime(true)), cmp.Diff(got, expect))
	})

	t.Run("v1beta2 object with conditions (end state)", func(t *testing.T) {
		g := NewWithT(t)
		foo := &builder.Phase3Obj{
			Status: builder.Phase3ObjStatus{
				Conditions: []metav1.Condition{
					{
						Type:               "fooCondition",
						Status:             metav1.ConditionTrue,
						LastTransitionTime: now,
						Reason:             "fooReason",
					},
				},
			},
		}

		expect := []metav1.Condition{
			{
				Type:               foo.Status.Conditions[0].Type,
				Status:             foo.Status.Conditions[0].Status,
				LastTransitionTime: foo.Status.Conditions[0].LastTransitionTime,
				Reason:             foo.Status.Conditions[0].Reason,
			},
		}

		got, err := GetAll(foo)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(MatchConditions(expect, IgnoreLastTransitionTime(true)), cmp.Diff(got, expect))

		fooUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(foo)
		g.Expect(err).NotTo(HaveOccurred())

		got, err = GetAll(&unstructured.Unstructured{Object: fooUnstructured})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(MatchConditions(expect, IgnoreLastTransitionTime(true)), cmp.Diff(got, expect))
	})
}

func TestConvertFromV1Beta1Conditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions []clusterv1.Condition
		want       []metav1.Condition
		wantError  bool
	}{
		{
			name: "Fails if Type is missing",
			conditions: clusterv1.Conditions{
				clusterv1.Condition{Status: corev1.ConditionTrue},
			},
			wantError: true,
		},
		{
			name: "Fails if Status is missing",
			conditions: clusterv1.Conditions{
				clusterv1.Condition{Type: clusterv1.ConditionType("foo")},
			},
			wantError: true,
		},
		{
			name: "Defaults reason for positive polarity",
			conditions: clusterv1.Conditions{
				clusterv1.Condition{Type: clusterv1.ConditionType("foo"), Status: corev1.ConditionTrue},
			},
			wantError: false,
			want: []metav1.Condition{
				{
					Type:   "foo",
					Status: metav1.ConditionTrue,
					Reason: NoReasonReported,
				},
			},
		},
		{
			name: "Defaults reason for negative polarity",
			conditions: clusterv1.Conditions{
				clusterv1.Condition{Type: clusterv1.ConditionType("foo"), Status: corev1.ConditionFalse},
			},
			wantError: false,
			want: []metav1.Condition{
				{
					Type:   "foo",
					Status: metav1.ConditionFalse,
					Reason: NoReasonReported,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got, err := convertFromV1Beta1Conditions(tt.conditions)
			if tt.wantError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(got).To(Equal(tt.want), cmp.Diff(tt.want, got))
		})
		t.Run(tt.name+" - unstructured", func(t *testing.T) {
			g := NewWithT(t)

			unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&builder.Phase0Obj{Status: builder.Phase0ObjStatus{Conditions: tt.conditions}})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(unstructuredObj).To(HaveKey("status"))
			unstructuredStatusObj := unstructuredObj["status"].(map[string]interface{})
			g.Expect(unstructuredStatusObj).To(HaveKey("conditions"))

			got, err := convertFromUnstructuredConditions(unstructuredStatusObj["conditions"].([]interface{}))
			if tt.wantError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(got).To(Equal(tt.want), cmp.Diff(tt.want, got))
		})
	}
}
