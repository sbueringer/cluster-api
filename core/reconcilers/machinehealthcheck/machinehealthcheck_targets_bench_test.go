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

package machinehealthcheck

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// benchmarkUnhealthyConditionExpressions returns 10 CEL expressions representative of
// real-world unhealthyConditions usage: a mix of node-only, machine-only and combined
// node+machine expressions, some with a lastTransitionTime-based timeout and some without.
func benchmarkUnhealthyConditionExpressions() []clusterv1.UnhealthyCondition {
	return []clusterv1.UnhealthyCondition{
		{Rule: `node.status.conditions.exists(c, c.type == 'Ready' && c.status == 'False' && duration(current_time - timestamp(c.lastTransitionTime)) > duration('5m'))`},
		{Rule: `node.status.conditions.exists(c, c.type == 'Ready' && c.status == 'Unknown' && duration(current_time - timestamp(c.lastTransitionTime)) > duration('5m'))`},
		{Rule: `node.status.conditions.exists(c, c.type == 'DiskPressure' && c.status == 'True')`},
		{Rule: `node.status.conditions.exists(c, c.type == 'MemoryPressure' && c.status == 'True')`},
		{Rule: `node.status.conditions.exists(c, c.type == 'PIDPressure' && c.status == 'True')`},
		{Rule: `node.status.conditions.exists(c, c.type == 'NetworkUnavailable' && c.status == 'True')`},
		{Rule: `machine.status.conditions.exists(c, c.type == 'NodeReady' && c.status == 'False' && duration(current_time - timestamp(c.lastTransitionTime)) > duration('5m'))`},
		{Rule: `machine.status.conditions.exists(c, c.type == 'HealthCheckSucceeded' && c.status == 'False')`},
		{Rule: `node.status.conditions.exists(c, c.type == 'Ready' && c.status == 'False') && machine.status.conditions.exists(c, c.type == 'NodeReady' && c.status == 'False')`},
		{Rule: `node.status.conditions.exists(c, c.type == 'Ready' && c.status == 'True') && !machine.status.conditions.exists(c, c.type == 'HealthCheckSucceeded' && c.status == 'True')`},
	}
}

// newBenchmarkTargets builds n healthCheckTargets representative of a fleet of Machines:
// most are healthy with a Node present, a fraction have no Node yet (e.g. still
// provisioning), and a fraction are genuinely unhealthy.
func newBenchmarkTargets(n int, mhc *clusterv1.MachineHealthCheck, now time.Time) []*healthCheckTarget {
	targets := make([]*healthCheckTarget, 0, n)
	for i := range n {
		machine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("machine-%d", i)},
			Status: clusterv1.MachineStatus{
				Conditions: []metav1.Condition{
					{Type: "HealthCheckSucceeded", Status: metav1.ConditionTrue, LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))},
					{Type: "NodeReady", Status: metav1.ConditionTrue, LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))},
				},
			},
		}

		var node *corev1.Node
		switch {
		case i%50 == 0:
			// ~2% of Machines don't have a Node yet (still provisioning).
			node = nil
		case i%37 == 0:
			// A small fraction are genuinely unhealthy.
			machine.Status.Conditions[0].Status = metav1.ConditionFalse
			machine.Status.Conditions[1].Status = metav1.ConditionFalse
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("node-%d", i)},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionFalse, LastTransitionTime: metav1.NewTime(now.Add(-10 * time.Minute))},
						{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse, LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))},
						{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse, LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))},
						{Type: corev1.NodePIDPressure, Status: corev1.ConditionFalse, LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))},
						{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse, LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))},
					},
				},
			}
		default:
			// The common case: healthy Machine with a healthy Node.
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("node-%d", i)},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))},
						{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse, LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))},
						{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse, LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))},
						{Type: corev1.NodePIDPressure, Status: corev1.ConditionFalse, LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))},
						{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse, LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))},
					},
				},
			}
		}

		targets = append(targets, &healthCheckTarget{
			MHC:     mhc,
			Machine: machine,
			Node:    node,
		})
	}
	return targets
}

// BenchmarkUnhealthyConditionsChecksAtScale models evaluating 10 unhealthyConditions CEL
// expressions (see benchmarkUnhealthyConditionExpressions) against a fleet of 27,000
// Machines, e.g. 1,000 Clusters averaging 27 Machines each. Run with -benchtime=1x to treat
// one iteration as one full reconcile pass across the fleet, e.g.:
//
//	go test ./core/reconcilers/machinehealthcheck/ -run=^$ \
//	  -bench=BenchmarkUnhealthyConditionsChecksAtScale -benchtime=1x -benchmem
//
// ns/op, B/op and allocs/op then directly report the cost of one pass over the whole fleet.
func BenchmarkUnhealthyConditionsChecksAtScale(b *testing.B) {
	const machineCount = 1000 * 27

	now := time.Now()
	mhc := &clusterv1.MachineHealthCheck{
		Spec: clusterv1.MachineHealthCheckSpec{
			Checks: clusterv1.MachineHealthCheckChecks{
				UnhealthyConditions: benchmarkUnhealthyConditionExpressions(),
			},
		},
	}
	targets := newBenchmarkTargets(machineCount, mhc, now)
	logger := logr.Discard()

	b.ReportAllocs()
	for b.Loop() {
		for _, target := range targets {
			_ = target.unhealthyConditionsChecks(logger, now)
		}
	}
}

// BenchmarkUnhealthyConditionsChecksAtScalePerMachine reports the average cost of
// evaluating all 10 expressions for a single Machine, useful for comparing against the
// per-reconcile budget of a single MachineHealthCheck reconcile.
func BenchmarkUnhealthyConditionsChecksAtScalePerMachine(b *testing.B) {
	now := time.Now()
	mhc := &clusterv1.MachineHealthCheck{
		Spec: clusterv1.MachineHealthCheckSpec{
			Checks: clusterv1.MachineHealthCheckChecks{
				UnhealthyConditions: benchmarkUnhealthyConditionExpressions(),
			},
		},
	}
	targets := newBenchmarkTargets(1000, mhc, now)
	logger := logr.Discard()

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		_ = targets[i%len(targets)].unhealthyConditionsChecks(logger, now)
		i++
	}
}
