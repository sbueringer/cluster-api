/*
Copyright 2026 The Kubernetes Authors.

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

package cel

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestCompile(t *testing.T) {
	t.Run("compiles a valid expression", func(t *testing.T) {
		g := NewWithT(t)
		_, err := Compile("node.status.conditions.exists(c, c.type == 'Ready' && c.status == 'False')")
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("returns an error for an invalid expression", func(t *testing.T) {
		g := NewWithT(t)
		_, err := Compile("node.status.conditions.exists(")
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("returns an error if the expression does not evaluate to a bool", func(t *testing.T) {
		g := NewWithT(t)
		_, err := Compile("node.status.conditions")
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("returns an error for fields that are not exposed to CEL", func(t *testing.T) {
		g := NewWithT(t)

		_, err := Compile("node.metadata.name == 'foo'")
		g.Expect(err).To(HaveOccurred())

		_, err = Compile("node.spec.providerID == 'foo'")
		g.Expect(err).To(HaveOccurred())

		_, err = Compile("machine.spec.clusterName == 'foo'")
		g.Expect(err).To(HaveOccurred())

		_, err = Compile("node.status.conditions.exists(c, c.bogusField == 'Ready')")
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("caches compiled programs by expression", func(t *testing.T) {
		g := NewWithT(t)

		expression := "node.status.conditions.exists(c, c.type == 'CacheTestCondition')"
		lenBefore := programs.Len()

		prg1, err := Compile(expression)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(programs.Len()).To(Equal(lenBefore + 1))

		prg2, err := Compile(expression)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(programs.Len()).To(Equal(lenBefore+1), "compiling the same expression again must not grow the cache")

		entry, ok := programs.Has(expression)
		g.Expect(ok).To(BeTrue())
		g.Expect(entry.program).To(BeIdenticalTo(prg1))
		g.Expect(entry.program).To(BeIdenticalTo(prg2))
	})
}

func TestReferencesNode(t *testing.T) {
	t.Run("detects a direct field access on node", func(t *testing.T) {
		g := NewWithT(t)
		entry, err := compile("node.status.conditions.exists(c, c.type == 'Ready')")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(entry.usesNode).To(BeTrue())
	})

	t.Run("does not flag an expression that only references machine", func(t *testing.T) {
		g := NewWithT(t)
		entry, err := compile("machine.status.conditions.exists(c, c.type == 'HealthCheckSucceeded')")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(entry.usesNode).To(BeFalse())
	})
}

func TestEvaluate(t *testing.T) {
	now := time.Now()

	node := &corev1.Node{
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:               corev1.NodeReady,
					Status:             corev1.ConditionFalse,
					LastTransitionTime: metav1.NewTime(now.Add(-10 * time.Minute)),
				},
				{
					Type:   "InfrastructureReady",
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	machine := &clusterv1.Machine{}

	expression := `
node.status.conditions.exists(c,
  c.type == 'Ready' &&
  c.status == 'False' &&
  duration(current_time - timestamp(c.lastTransitionTime)) > duration('5m')
) &&
!node.status.conditions.exists(c,
  c.type == 'InfrastructureReady' &&
  c.status == 'False'
)
`

	t.Run("matches when the Ready condition has been False for longer than the timeout", func(t *testing.T) {
		g := NewWithT(t)
		matched, err := Evaluate(expression, NewInput(node, machine, now))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(matched).To(BeTrue())
	})

	t.Run("does not match when the Ready condition became False recently", func(t *testing.T) {
		g := NewWithT(t)
		recentNode := node.DeepCopy()
		recentNode.Status.Conditions[0].LastTransitionTime = metav1.NewTime(now.Add(-1 * time.Minute))
		matched, err := Evaluate(expression, NewInput(recentNode, machine, now))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(matched).To(BeFalse())
	})

	t.Run("returns an error for an invalid expression", func(t *testing.T) {
		g := NewWithT(t)
		_, err := Evaluate("node.doesNotExist", NewInput(node, machine, now))
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("matches using the machine variable", func(t *testing.T) {
		g := NewWithT(t)
		unhealthyMachine := &clusterv1.Machine{
			Status: clusterv1.MachineStatus{
				Conditions: []metav1.Condition{
					{
						Type:   "HealthCheckSucceeded",
						Status: metav1.ConditionFalse,
					},
				},
			},
		}
		matched, err := Evaluate("machine.status.conditions.exists(c, c.type == 'HealthCheckSucceeded' && c.status == 'False')", NewInput(node, unhealthyMachine, now))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(matched).To(BeTrue())
	})

	t.Run("automatically treats an expression referencing node as not matched when the node is nil, without erroring", func(t *testing.T) {
		g := NewWithT(t)
		matched, err := Evaluate(expression, NewInput(nil, machine, now))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(matched).To(BeFalse())
	})

	t.Run("matches using only the machine variable when the node is nil, e.g. before the node has been created", func(t *testing.T) {
		g := NewWithT(t)
		unhealthyMachine := &clusterv1.Machine{
			Status: clusterv1.MachineStatus{
				Conditions: []metav1.Condition{
					{
						Type:   "HealthCheckSucceeded",
						Status: metav1.ConditionFalse,
					},
				},
			},
		}
		matched, err := Evaluate("machine.status.conditions.exists(c, c.type == 'HealthCheckSucceeded' && c.status == 'False')", NewInput(nil, unhealthyMachine, now))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(matched).To(BeTrue())
	})

	t.Run("reuses the same Input across multiple Evaluate calls without recomputing the node/machine CEL values", func(t *testing.T) {
		g := NewWithT(t)
		input := NewInput(node, machine, now)

		matched, err := Evaluate("node.status.conditions.exists(c, c.type == 'Ready' && c.status == 'False')", input)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(matched).To(BeTrue())
		g.Expect(input.node).ToNot(BeNil())
		g.Expect(input.machineVal).To(BeNil(), "machine value should not be computed for an expression that never references machine")

		matched, err = Evaluate("machine.status.conditions.exists(c, c.type == 'HealthCheckSucceeded')", input)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(matched).To(BeFalse())
		g.Expect(input.machineVal).ToNot(BeNil())
	})
}
