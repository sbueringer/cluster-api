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
	"encoding/json"
	"fmt"

	pkgerrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NodeDrainConditionEvent identifies an observation made by the Machine controller
// while draining a Node and waiting for its volumes to be detached.
// See KEP-5683: https://kep.k8s.io/5683.
type NodeDrainConditionEvent string

const (
	// NodeDrainStarted is reported while the Machine controller is draining the Node.
	NodeDrainStarted NodeDrainConditionEvent = "DrainStarted"

	// NodeWaitingForVolumeDetach is reported while the Machine controller is waiting for
	// volumes to be detached from the Node.
	NodeWaitingForVolumeDetach NodeDrainConditionEvent = "WaitingForVolumeDetach"

	// NodeDrainCompleted is reported after drain and wait for volume detach completed.
	NodeDrainCompleted NodeDrainConditionEvent = "DrainCompleted"

	// NodeDrainAborted is reported if drain or wait for volume detach have been started, but the Machine controller
	// stopped them before they completed. This happens if:
	// * nodeDrainTimeoutSeconds or nodeVolumeDetachTimeoutSeconds was exceeded
	// * the corresponding exclude annotation was added mid-drain
	// * the InfraMachine was deleted
	// * the pre-terminate hook started.
	NodeDrainAborted NodeDrainConditionEvent = "DrainAborted"
)

// Reasons used by the Machine controller for the DrainInProgress and Drained Node conditions.
// Note: The reasons are prefixed with ClusterAPI, aligned to the Kubectl prefix used by kubectl drain.
const (
	// ClusterAPIDrainStartedReason surfaces that the Machine controller is draining the Node.
	ClusterAPIDrainStartedReason = "ClusterAPIDrainStarted"

	// ClusterAPIWaitingForVolumeDetachReason surfaces that the Machine controller is waiting for
	// volumes to be detached from the Node.
	ClusterAPIWaitingForVolumeDetachReason = "ClusterAPIWaitingForVolumeDetach"

	// ClusterAPIDrainCompletedReason surfaces that the Machine controller completed drain and wait for volume detach.
	ClusterAPIDrainCompletedReason = "ClusterAPIDrainCompleted"

	// ClusterAPIDrainAbortedReason surfaces that the Machine controller stopped drain or wait for volume detach
	// before completion (see NodeDrainAborted).
	ClusterAPIDrainAbortedReason = "ClusterAPIDrainAborted"
)

// SetNodeDrainConditions sets the DrainInProgress and Drained conditions on the Node according to event.
// The Node is not patched if both conditions already have the desired status, reason and message,
// which avoids patching the Node on every reconcile while drain or wait for volume detach are in progress.
func SetNodeDrainConditions(ctx context.Context, c client.Client, node *corev1.Node, event NodeDrainConditionEvent) error {
	desired, err := nodeDrainConditionsForEvent(event)
	if err != nil {
		return err
	}

	if nodeConditionsUpToDate(node.Status.Conditions, desired) {
		return nil
	}

	now := metav1.Now()
	for i := range desired {
		desired[i] = withNodeConditionTimes(node.Status.Conditions, desired[i], now)
	}

	// Note: A strategic merge patch is used, which merges conditions by type. This ensures only the
	// DrainInProgress and Drained conditions are changed, while other conditions (e.g. the ones owned by the kubelet)
	// are preserved even if they are concurrently updated. A JSON merge patch would replace the entire conditions list.
	patch := nodeConditionsPatch{}
	patch.Status.Conditions = desired
	patchData, err := json.Marshal(patch)
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to marshal patch to set drain conditions on Node %s", node.Name)
	}
	if err := c.Status().Patch(ctx, node, client.RawPatch(types.StrategicMergePatchType, patchData)); err != nil {
		return pkgerrors.Wrapf(err, "failed to set drain conditions on Node %s", node.Name)
	}
	return nil
}

type nodeConditionsPatch struct {
	Status struct {
		Conditions []corev1.NodeCondition `json:"conditions"`
	} `json:"status"`
}

func nodeDrainConditionsForEvent(event NodeDrainConditionEvent) ([]corev1.NodeCondition, error) {
	var inProgress, drained corev1.ConditionStatus
	var reason, message string
	switch event {
	case NodeDrainStarted:
		inProgress, drained = corev1.ConditionTrue, corev1.ConditionFalse
		reason, message = ClusterAPIDrainStartedReason, "Cluster API is draining the Node"
	case NodeWaitingForVolumeDetach:
		inProgress, drained = corev1.ConditionTrue, corev1.ConditionFalse
		reason, message = ClusterAPIWaitingForVolumeDetachReason, "Cluster API is waiting for volumes to be detached from the Node"
	case NodeDrainCompleted:
		inProgress, drained = corev1.ConditionFalse, corev1.ConditionTrue
		reason, message = ClusterAPIDrainCompletedReason, "Cluster API drain and wait for volume detach completed"
	case NodeDrainAborted:
		inProgress, drained = corev1.ConditionFalse, corev1.ConditionFalse
		reason, message = ClusterAPIDrainAbortedReason, "Cluster API stopped draining the Node before completion (e.g. because a timeout was exceeded)"
	default:
		return nil, fmt.Errorf("unknown Node drain condition event %q", event)
	}
	return []corev1.NodeCondition{
		{Type: corev1.NodeDrainInProgress, Status: inProgress, Reason: reason, Message: message},
		{Type: corev1.NodeDrained, Status: drained, Reason: reason, Message: message},
	}, nil
}

// nodeConditionsUpToDate returns true if all desired conditions exist with the same status, reason and message.
func nodeConditionsUpToDate(existing, desired []corev1.NodeCondition) bool {
	for _, d := range desired {
		e := getNodeCondition(existing, d.Type)
		if e == nil || e.Status != d.Status || e.Reason != d.Reason || e.Message != d.Message {
			return false
		}
	}
	return true
}

// withNodeConditionTimes sets lastHeartbeatTime to now and preserves lastTransitionTime if the status didn't change.
func withNodeConditionTimes(existing []corev1.NodeCondition, condition corev1.NodeCondition, now metav1.Time) corev1.NodeCondition {
	condition.LastHeartbeatTime = now
	condition.LastTransitionTime = now
	if e := getNodeCondition(existing, condition.Type); e != nil && e.Status == condition.Status && !e.LastTransitionTime.IsZero() {
		condition.LastTransitionTime = e.LastTransitionTime
	}
	return condition
}

func getNodeCondition(conditions []corev1.NodeCondition, conditionType corev1.NodeConditionType) *corev1.NodeCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
