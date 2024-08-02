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

package experimental

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestAggregateMessages(t *testing.T) {
	g := NewWithT(t)

	conditions := []ConditionWithOwnerInfo{
		{OwnerResource: ConditionOwnerInfo{Name: "obj1"}, Condition: metav1.Condition{Type: "A", Message: "Message-1", Status: metav1.ConditionFalse}},
		{OwnerResource: ConditionOwnerInfo{Name: "obj2"}, Condition: metav1.Condition{Type: "A", Message: "Message-1", Status: metav1.ConditionFalse}},
		{OwnerResource: ConditionOwnerInfo{Name: "obj3"}, Condition: metav1.Condition{Type: "A", Message: "Message-2", Status: metav1.ConditionFalse}},
		{OwnerResource: ConditionOwnerInfo{Name: "obj4"}, Condition: metav1.Condition{Type: "A", Message: "Message-2", Status: metav1.ConditionFalse}},
		{OwnerResource: ConditionOwnerInfo{Name: "obj5"}, Condition: metav1.Condition{Type: "A", Message: "Message-1", Status: metav1.ConditionFalse}},
		{OwnerResource: ConditionOwnerInfo{Name: "obj6"}, Condition: metav1.Condition{Type: "A", Message: "Message-3", Status: metav1.ConditionFalse}},
		{OwnerResource: ConditionOwnerInfo{Name: "obj7"}, Condition: metav1.Condition{Type: "A", Message: "Message-4", Status: metav1.ConditionFalse}},
		{OwnerResource: ConditionOwnerInfo{Name: "obj8"}, Condition: metav1.Condition{Type: "A", Message: "Message-1", Status: metav1.ConditionFalse}},
		{OwnerResource: ConditionOwnerInfo{Name: "obj9"}, Condition: metav1.Condition{Type: "A", Message: "Message-1", Status: metav1.ConditionFalse}},
	}

	n := 3
	messages, others := aggregateMessages(conditions, &n, "objects", false)

	g.Expect(n).To(Equal(0))
	g.Expect(messages).To(Equal([]string{
		"(False): Message-1 from obj1, obj2, obj5 and 2 other objects",
		"(False): Message-2 from obj3, obj4",
		"(False): Message-3 from obj6",
	}))
	g.Expect(others).To(Equal(1))
}

func TestSortConditions(t *testing.T) {
	g := NewWithT(t)

	t0 := metav1.Now()
	t1 := metav1.Time{Time: t0.Add(10 * time.Minute)}
	t2 := metav1.Time{Time: t0.Add(20 * time.Minute)}

	conditions := []ConditionWithOwnerInfo{
		{OwnerResource: ConditionOwnerInfo{Name: "baz"}, Condition: metav1.Condition{Type: "B", Status: metav1.ConditionFalse, LastTransitionTime: t0}},
		{OwnerResource: ConditionOwnerInfo{Name: "baz"}, Condition: metav1.Condition{Type: "A", Status: metav1.ConditionFalse, LastTransitionTime: t1}},
		{OwnerResource: ConditionOwnerInfo{Name: "foo"}, Condition: metav1.Condition{Type: "!C", Status: metav1.ConditionTrue, LastTransitionTime: t2}},
		{OwnerResource: ConditionOwnerInfo{Name: "bar"}, Condition: metav1.Condition{Type: "A", Status: metav1.ConditionUnknown, LastTransitionTime: t0}},
		{OwnerResource: ConditionOwnerInfo{Name: "foo"}, Condition: metav1.Condition{Type: "A", Status: metav1.ConditionTrue, LastTransitionTime: t2}},
		{OwnerResource: ConditionOwnerInfo{Name: "bar"}, Condition: metav1.Condition{Type: "!C", Status: metav1.ConditionUnknown, LastTransitionTime: t0}},
		{OwnerResource: ConditionOwnerInfo{Name: "bar"}, Condition: metav1.Condition{Type: "B", Status: metav1.ConditionUnknown, LastTransitionTime: t2}},
		{OwnerResource: ConditionOwnerInfo{Name: "foo"}, Condition: metav1.Condition{Type: "B", Status: metav1.ConditionTrue, LastTransitionTime: t1}},
		{OwnerResource: ConditionOwnerInfo{Name: "baz"}, Condition: metav1.Condition{Type: "!C", Status: metav1.ConditionFalse, LastTransitionTime: t1}},
	}

	orderedConditionTypes := []string{"A", "B", "!C"}
	sortConditions(conditions, orderedConditionTypes)

	// Check conditions are sorted by orderedConditionTypes and by LastTransitionTime

	g.Expect(conditions).To(Equal([]ConditionWithOwnerInfo{
		{OwnerResource: ConditionOwnerInfo{Name: "bar"}, Condition: metav1.Condition{Type: "A", Status: metav1.ConditionUnknown, LastTransitionTime: t0}},
		{OwnerResource: ConditionOwnerInfo{Name: "baz"}, Condition: metav1.Condition{Type: "A", Status: metav1.ConditionFalse, LastTransitionTime: t1}},
		{OwnerResource: ConditionOwnerInfo{Name: "foo"}, Condition: metav1.Condition{Type: "A", Status: metav1.ConditionTrue, LastTransitionTime: t2}},
		{OwnerResource: ConditionOwnerInfo{Name: "baz"}, Condition: metav1.Condition{Type: "B", Status: metav1.ConditionFalse, LastTransitionTime: t0}},
		{OwnerResource: ConditionOwnerInfo{Name: "foo"}, Condition: metav1.Condition{Type: "B", Status: metav1.ConditionTrue, LastTransitionTime: t1}},
		{OwnerResource: ConditionOwnerInfo{Name: "bar"}, Condition: metav1.Condition{Type: "B", Status: metav1.ConditionUnknown, LastTransitionTime: t2}},
		{OwnerResource: ConditionOwnerInfo{Name: "bar"}, Condition: metav1.Condition{Type: "!C", Status: metav1.ConditionUnknown, LastTransitionTime: t0}},
		{OwnerResource: ConditionOwnerInfo{Name: "baz"}, Condition: metav1.Condition{Type: "!C", Status: metav1.ConditionFalse, LastTransitionTime: t1}},
		{OwnerResource: ConditionOwnerInfo{Name: "foo"}, Condition: metav1.Condition{Type: "!C", Status: metav1.ConditionTrue, LastTransitionTime: t2}},
	}))
}

func TestSplitConditionsByPriority(t *testing.T) {
	g := NewWithT(t)

	conditions := []ConditionWithOwnerInfo{
		{OwnerResource: ConditionOwnerInfo{Name: "baz"}, Condition: metav1.Condition{Type: "B", Status: metav1.ConditionFalse}},    // issue
		{OwnerResource: ConditionOwnerInfo{Name: "foo"}, Condition: metav1.Condition{Type: "A", Status: metav1.ConditionTrue}},     // info
		{OwnerResource: ConditionOwnerInfo{Name: "baz"}, Condition: metav1.Condition{Type: "A", Status: metav1.ConditionFalse}},    // issue
		{OwnerResource: ConditionOwnerInfo{Name: "foo"}, Condition: metav1.Condition{Type: "!C", Status: metav1.ConditionTrue}},    // issue
		{OwnerResource: ConditionOwnerInfo{Name: "bar"}, Condition: metav1.Condition{Type: "A", Status: metav1.ConditionUnknown}},  // unknown
		{OwnerResource: ConditionOwnerInfo{Name: "bar"}, Condition: metav1.Condition{Type: "!C", Status: metav1.ConditionUnknown}}, // unknown
		{OwnerResource: ConditionOwnerInfo{Name: "bar"}, Condition: metav1.Condition{Type: "B", Status: metav1.ConditionUnknown}},  // unknown
		{OwnerResource: ConditionOwnerInfo{Name: "foo"}, Condition: metav1.Condition{Type: "B", Status: metav1.ConditionTrue}},     // info
		{OwnerResource: ConditionOwnerInfo{Name: "baz"}, Condition: metav1.Condition{Type: "!C", Status: metav1.ConditionFalse}},   // info
	}

	issueConditions, unknownConditions, infoConditions := splitConditionsByPriority(conditions, sets.New[string]("!C"))

	// Check condition are grouped as expected and order is preserved.

	g.Expect(issueConditions).To(Equal([]ConditionWithOwnerInfo{
		{OwnerResource: ConditionOwnerInfo{Name: "baz"}, Condition: metav1.Condition{Type: "B", Status: metav1.ConditionFalse}},
		{OwnerResource: ConditionOwnerInfo{Name: "baz"}, Condition: metav1.Condition{Type: "A", Status: metav1.ConditionFalse}},
		{OwnerResource: ConditionOwnerInfo{Name: "foo"}, Condition: metav1.Condition{Type: "!C", Status: metav1.ConditionTrue}},
	}))

	g.Expect(unknownConditions).To(Equal([]ConditionWithOwnerInfo{
		{OwnerResource: ConditionOwnerInfo{Name: "bar"}, Condition: metav1.Condition{Type: "A", Status: metav1.ConditionUnknown}},
		{OwnerResource: ConditionOwnerInfo{Name: "bar"}, Condition: metav1.Condition{Type: "!C", Status: metav1.ConditionUnknown}},
		{OwnerResource: ConditionOwnerInfo{Name: "bar"}, Condition: metav1.Condition{Type: "B", Status: metav1.ConditionUnknown}},
	}))

	g.Expect(infoConditions).To(Equal([]ConditionWithOwnerInfo{
		{OwnerResource: ConditionOwnerInfo{Name: "foo"}, Condition: metav1.Condition{Type: "A", Status: metav1.ConditionTrue}},
		{OwnerResource: ConditionOwnerInfo{Name: "foo"}, Condition: metav1.Condition{Type: "B", Status: metav1.ConditionTrue}},
		{OwnerResource: ConditionOwnerInfo{Name: "baz"}, Condition: metav1.Condition{Type: "!C", Status: metav1.ConditionFalse}},
	}))
}

func TestGetPriority(t *testing.T) {
	tests := []struct {
		name             string
		condition        metav1.Condition
		negativePolarity bool
		wantPriority     mergePriority
	}{
		{
			name:             "Issue (PositivePolarity)",
			condition:        metav1.Condition{Type: "foo", Status: metav1.ConditionFalse},
			negativePolarity: false,
			wantPriority:     issueMergePriority,
		},
		{
			name:             "Unknown (PositivePolarity)",
			condition:        metav1.Condition{Type: "foo", Status: metav1.ConditionUnknown},
			negativePolarity: false,
			wantPriority:     unknownMergePriority,
		},
		{
			name:             "Info (PositivePolarity)",
			condition:        metav1.Condition{Type: "foo", Status: metav1.ConditionTrue},
			negativePolarity: false,
			wantPriority:     infoMergePriority,
		},
		{
			name:             "NoStatus (PositivePolarity)",
			condition:        metav1.Condition{Type: "foo"},
			negativePolarity: false,
			wantPriority:     unknownMergePriority,
		},
		{
			name:             "Issue (NegativePolarity)",
			condition:        metav1.Condition{Type: "foo", Status: metav1.ConditionTrue},
			negativePolarity: true,
			wantPriority:     issueMergePriority,
		},
		{
			name:             "Unknown (NegativePolarity)",
			condition:        metav1.Condition{Type: "foo", Status: metav1.ConditionUnknown},
			negativePolarity: true,
			wantPriority:     unknownMergePriority,
		},
		{
			name:             "Info (NegativePolarity)",
			condition:        metav1.Condition{Type: "foo", Status: metav1.ConditionFalse},
			negativePolarity: true,
			wantPriority:     infoMergePriority,
		},
		{
			name:             "NoStatus (NegativePolarity)",
			condition:        metav1.Condition{Type: "foo"},
			negativePolarity: true,
			wantPriority:     unknownMergePriority,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			negativePolarityConditionTypes := sets.New[string]()
			if tt.negativePolarity {
				negativePolarityConditionTypes.Insert(tt.condition.Type)
			}
			gotPriority := getPriority(tt.condition, negativePolarityConditionTypes)

			g.Expect(gotPriority).To(Equal(tt.wantPriority))
		})
	}
}
