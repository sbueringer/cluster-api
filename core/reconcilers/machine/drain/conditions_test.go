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

package drain

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// statusPatchCounter counts status patches sent through the client.
type statusPatchCounter struct {
	count int
}

func (s *statusPatchCounter) subResourcePatch(ctx context.Context, c client.Client, subResourceName string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	s.count++
	return c.SubResource(subResourceName).Patch(ctx, obj, patch, opts...)
}

func TestSetNodeDrainConditions(t *testing.T) {
	oldTransition := metav1.NewTime(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC))

	tests := []struct {
		name                 string
		event                NodeDrainConditionEvent
		existing             []corev1.NodeCondition
		wantInProgress       corev1.ConditionStatus
		wantDrained          corev1.ConditionStatus
		wantReason           string
		wantMessage          string
		wantInProgressOldLTT bool
		wantDrainedOldLTT    bool
		wantPatch            bool
	}{
		{
			name:           "DrainStarted without existing conditions",
			event:          NodeDrainStarted,
			wantInProgress: corev1.ConditionTrue,
			wantDrained:    corev1.ConditionFalse,
			wantReason:     ClusterAPIDrainStartedReason,
			wantMessage:    "Cluster API is draining the Node",
			wantPatch:      true,
		},
		{
			name:  "WaitingForVolumeDetach after DrainStarted preserves transition times",
			event: NodeWaitingForVolumeDetach,
			existing: []corev1.NodeCondition{
				{Type: corev1.NodeDrainInProgress, Status: corev1.ConditionTrue, Reason: ClusterAPIDrainStartedReason, LastTransitionTime: oldTransition},
				{Type: corev1.NodeDrained, Status: corev1.ConditionFalse, Reason: ClusterAPIDrainStartedReason, LastTransitionTime: oldTransition},
			},
			wantInProgress:       corev1.ConditionTrue,
			wantDrained:          corev1.ConditionFalse,
			wantReason:           ClusterAPIWaitingForVolumeDetachReason,
			wantMessage:          "Cluster API is waiting for volumes to be detached from the Node",
			wantInProgressOldLTT: true,
			wantDrainedOldLTT:    true,
			wantPatch:            true,
		},
		{
			name:  "DrainCompleted flips both conditions",
			event: NodeDrainCompleted,
			existing: []corev1.NodeCondition{
				{Type: corev1.NodeDrainInProgress, Status: corev1.ConditionTrue, Reason: ClusterAPIWaitingForVolumeDetachReason, LastTransitionTime: oldTransition},
				{Type: corev1.NodeDrained, Status: corev1.ConditionFalse, Reason: ClusterAPIWaitingForVolumeDetachReason, LastTransitionTime: oldTransition},
			},
			wantInProgress: corev1.ConditionFalse,
			wantDrained:    corev1.ConditionTrue,
			wantReason:     ClusterAPIDrainCompletedReason,
			wantMessage:    "Cluster API drain and wait for volume detach completed",
			wantPatch:      true,
		},
		{
			name:  "DrainAborted only flips DrainInProgress",
			event: NodeDrainAborted,
			existing: []corev1.NodeCondition{
				{Type: corev1.NodeDrainInProgress, Status: corev1.ConditionTrue, Reason: ClusterAPIDrainStartedReason, LastTransitionTime: oldTransition},
				{Type: corev1.NodeDrained, Status: corev1.ConditionFalse, Reason: ClusterAPIDrainStartedReason, LastTransitionTime: oldTransition},
			},
			wantInProgress:    corev1.ConditionFalse,
			wantDrained:       corev1.ConditionFalse,
			wantReason:        ClusterAPIDrainAbortedReason,
			wantMessage:       "Cluster API stopped draining the Node before completion (e.g. because a timeout was exceeded)",
			wantDrainedOldLTT: true,
			wantPatch:         true,
		},
		{
			name:  "No patch if conditions are already up to date",
			event: NodeDrainStarted,
			existing: []corev1.NodeCondition{
				{Type: corev1.NodeDrainInProgress, Status: corev1.ConditionTrue, Reason: ClusterAPIDrainStartedReason, Message: "Cluster API is draining the Node", LastTransitionTime: oldTransition},
				{Type: corev1.NodeDrained, Status: corev1.ConditionFalse, Reason: ClusterAPIDrainStartedReason, Message: "Cluster API is draining the Node", LastTransitionTime: oldTransition},
			},
			wantInProgress:       corev1.ConditionTrue,
			wantDrained:          corev1.ConditionFalse,
			wantReason:           ClusterAPIDrainStartedReason,
			wantMessage:          "Cluster API is draining the Node",
			wantInProgressOldLTT: true,
			wantDrainedOldLTT:    true,
			wantPatch:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				Status: corev1.NodeStatus{
					Conditions: append([]corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue, Reason: "KubeletReady", LastTransitionTime: oldTransition},
					}, tt.existing...),
				},
			}
			fakeClient := fake.NewClientBuilder().WithObjects(node).WithStatusSubresource(&corev1.Node{}).Build()
			counter := &statusPatchCounter{}
			c := interceptor.NewClient(fakeClient, interceptor.Funcs{SubResourcePatch: counter.subResourcePatch})

			nodeToPatch := &corev1.Node{}
			g.Expect(fakeClient.Get(t.Context(), client.ObjectKeyFromObject(node), nodeToPatch)).To(Succeed())
			g.Expect(SetNodeDrainConditions(t.Context(), c, nodeToPatch, tt.event)).To(Succeed())
			if tt.wantPatch {
				g.Expect(counter.count).To(Equal(1))
			} else {
				g.Expect(counter.count).To(Equal(0))
			}

			gotNode := &corev1.Node{}
			g.Expect(fakeClient.Get(t.Context(), client.ObjectKeyFromObject(node), gotNode)).To(Succeed())

			ready := getNodeCondition(gotNode.Status.Conditions, corev1.NodeReady)
			g.Expect(ready).ToNot(BeNil())
			g.Expect(ready.Status).To(Equal(corev1.ConditionTrue))
			g.Expect(ready.Reason).To(Equal("KubeletReady"))

			assertNodeCondition(g, gotNode, corev1.NodeDrainInProgress, tt.wantInProgress, tt.wantReason, tt.wantMessage, tt.wantInProgressOldLTT, oldTransition)
			assertNodeCondition(g, gotNode, corev1.NodeDrained, tt.wantDrained, tt.wantReason, tt.wantMessage, tt.wantDrainedOldLTT, oldTransition)
		})
	}
}

func TestSetNodeDrainConditionsUnknownEvent(t *testing.T) {
	g := NewWithT(t)

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
	c := fake.NewClientBuilder().WithObjects(node).WithStatusSubresource(&corev1.Node{}).Build()
	g.Expect(SetNodeDrainConditions(t.Context(), c, node, "Unknown")).ToNot(Succeed())
}

func assertNodeCondition(g *WithT, node *corev1.Node, conditionType corev1.NodeConditionType, status corev1.ConditionStatus, reason, message string, wantOldLTT bool, oldTransition metav1.Time) {
	condition := getNodeCondition(node.Status.Conditions, conditionType)
	g.Expect(condition).ToNot(BeNil())
	g.Expect(condition.Status).To(Equal(status))
	g.Expect(condition.Reason).To(Equal(reason))
	g.Expect(condition.Message).To(Equal(message))
	if wantOldLTT {
		g.Expect(condition.LastTransitionTime.Time).To(BeTemporally("==", oldTransition.Time))
	} else {
		g.Expect(condition.LastTransitionTime.Time).To(BeTemporally("~", time.Now(), 5*time.Second))
		g.Expect(condition.LastHeartbeatTime.Time).To(BeTemporally("~", time.Now(), 5*time.Second))
	}
}
