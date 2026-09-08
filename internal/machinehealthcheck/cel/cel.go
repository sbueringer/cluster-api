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

// Package cel provides a CEL environment used to evaluate MachineHealthCheck
// unhealthyConditions expressions.
package cel

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	celgo "github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/ext"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/cel/environment"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/cache"
)

// NodeVariableName is the name of the CEL variable that the Node is bound to.
const NodeVariableName = "node"

// MachineVariableName is the name of the CEL variable that the Machine is bound to.
const MachineVariableName = "machine"

// CurrentTimeVariableName is the name of the CEL variable that the current time is bound to.
const CurrentTimeVariableName = "current_time"

// nodeForCEL, machineForCEL and their nested types define the exact shape of the "node" and
// "machine" CEL variables. Using dedicated, strictly-typed structs (registered with cel-go's
// native types support, see getEnv below) rather than cel.DynType means that expressions
// referencing fields outside of this shape (e.g. node.spec.providerID) fail to compile,
// instead of failing at evaluation time.
type nodeForCEL struct {
	Status nodeStatusForCEL `cel:"status"`
}

type nodeStatusForCEL struct {
	Conditions []nodeConditionForCEL `cel:"conditions"`
}

type nodeConditionForCEL struct {
	Type    string `cel:"type"`
	Status  string `cel:"status"`
	Reason  string `cel:"reason"`
	Message string `cel:"message"`
	// FIXME: should this really be of type string instead of e.g. timestamp? (same for machineConditionForCEL)
	LastTransitionTime string `cel:"lastTransitionTime"`
}

type machineForCEL struct {
	Status machineStatusForCEL `cel:"status"`
}

type machineStatusForCEL struct {
	Conditions []machineConditionForCEL `cel:"conditions"`
}

type machineConditionForCEL struct {
	Type               string `cel:"type"`
	Status             string `cel:"status"`
	Reason             string `cel:"reason"`
	Message            string `cel:"message"`
	LastTransitionTime string `cel:"lastTransitionTime"`
}

var (
	envOnce sync.Once
	env     *celgo.Env
	envErr  error

	// programs caches compiled programs by expression, so that identical
	// expressions across MachineHealthCheck reconciles are only compiled once.
	// Entries expire after programsCacheTTL so that expressions belonging to
	// MachineHealthChecks that have since been deleted or modified don't stay
	// cached forever.
	programs = cache.New[programEntry](context.Background(), 1*time.Hour)
)

// programEntry is a cache.Entry caching a compiled CEL program under its source expression.
type programEntry struct {
	expression string
	program    celgo.Program
	// usesNode is true if the expression references the "node" variable. Expressions that
	// don't reference "node" at all are unaffected by whether a Node exists; expressions
	// that do are automatically skipped (treated as not matched) when node is nil, see Evaluate.
	usesNode bool
	// usesMachine is true if the expression references the "machine" variable. Used to avoid
	// building the machine CEL value for expressions that don't need it, see Evaluate.
	usesMachine bool
}

// Key implements cache.Entry.
func (e programEntry) Key() string {
	return e.expression
}

// nodeForCELType and machineForCELType are the CEL object types that the native types
// extension (registered in getEnv below) derives for nodeForCEL and machineForCEL: the
// package alias (last segment of the package path) followed by the Go type name.
var (
	nodeForCELType    = celgo.ObjectType("cel.nodeForCEL")
	machineForCELType = celgo.ObjectType("cel.machineForCEL")
)

// getEnv returns the CEL environment used to compile and run unhealthyConditions
// expressions. The environment is based on the Kubernetes CEL base environment
// (the same function libraries used by e.g. CRD x-kubernetes-validations rules),
// extended with the "node", "machine" and "current_time" variables.
func getEnv() (*celgo.Env, error) {
	envOnce.Do(func() {
		envSet, err := environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion()).Extend(
			environment.VersionedOptions{
				IntroducedVersion: version.MajorMinor(1, 0),
				EnvOptions: []celgo.EnvOption{
					ext.NativeTypes(
						ext.ParseStructTags(true),
						reflect.TypeFor[nodeForCEL](),
						reflect.TypeFor[machineForCEL](),
					),
					celgo.Variable(NodeVariableName, nodeForCELType),
					celgo.Variable(MachineVariableName, machineForCELType),
					celgo.Variable(CurrentTimeVariableName, celgo.TimestampType),
				},
			},
		)
		if err != nil {
			envErr = fmt.Errorf("failed to build CEL environment: %w", err)
			return
		}
		env, envErr = envSet.Env(environment.StoredExpressions)
	})
	return env, envErr
}

// compile compiles expression, returning a runnable CEL program along with whether the
// expression references the "node" variable. Compiled programs are cached, so repeated
// calls with the same expression are cheap.
func compile(expression string) (programEntry, error) {
	if entry, ok := programs.Has(expression); ok {
		return entry, nil
	}

	env, err := getEnv()
	if err != nil {
		return programEntry{}, err
	}

	ast, iss := env.Compile(expression)
	if iss.Err() != nil {
		return programEntry{}, iss.Err()
	}
	if ast.OutputType() != celgo.BoolType {
		return programEntry{}, fmt.Errorf("expression must evaluate to a bool, got %s", ast.OutputType())
	}

	prg, err := env.Program(ast)
	if err != nil {
		return programEntry{}, err
	}

	entry := programEntry{
		expression:  expression,
		program:     prg,
		usesNode:    referencesIdent(ast.NativeRep(), NodeVariableName),
		usesMachine: referencesIdent(ast.NativeRep(), MachineVariableName),
	}
	programs.Add(entry)
	return entry, nil
}

// referencesIdent returns true if a references the given identifier anywhere in the
// expression tree, e.g. NodeVariableName matches both node.status.conditions and a
// standalone node.
func referencesIdent(a *celast.AST, name string) bool {
	found := false
	celast.PreOrderVisit(a.Expr(), celast.NewExprVisitor(func(e celast.Expr) {
		if e.Kind() == celast.IdentKind && e.AsIdent() == name {
			found = true
		}
	}))
	return found
}

// Compile compiles expression and returns a runnable CEL program.
// The expression must evaluate to a bool, and has access to a "node" variable, bound to
// node.status.conditions, a "machine" variable, bound to machine.status.conditions, and a
// "current_time" variable, bound to the time of evaluation.
// Compiled programs are cached, so repeated calls with the same expression are cheap.
func Compile(expression string) (celgo.Program, error) {
	entry, err := compile(expression)
	if err != nil {
		return nil, err
	}
	return entry.program, nil
}

// Input holds the Node, Machine and evaluation time that a set of unhealthyConditions
// expressions are evaluated against, e.g. all expressions configured on one
// MachineHealthCheck for one target. Use NewInput to create one, and reuse it across all
// Evaluate calls for that target: the "node" and "machine" CEL values are computed at most
// once per Input (lazily, only the first time an expression actually references them), so
// evaluating multiple expressions against the same target does not redo the underlying
// condition conversion (including per-condition time formatting) for every expression.
type Input struct {
	node    *corev1.Node
	machine *clusterv1.Machine
	now     time.Time

	nodeVal    *nodeForCEL
	machineVal *machineForCEL
}

// NewInput returns an Input for evaluating unhealthyConditions expressions against node,
// machine and now. node may be nil, e.g. if the Machine does not have a Node yet.
func NewInput(node *corev1.Node, machine *clusterv1.Machine, now time.Time) *Input {
	return &Input{node: node, machine: machine, now: now}
}

func (in *Input) nodeCELValue() nodeForCEL {
	if in.nodeVal == nil {
		in.nodeVal = &nodeForCEL{Status: nodeStatusForCEL{Conditions: nodeConditionsToCEL(in.node.Status.Conditions)}}
	}
	return *in.nodeVal
}

func (in *Input) machineCELValue() machineForCEL {
	if in.machineVal == nil {
		in.machineVal = &machineForCEL{Status: machineStatusForCEL{Conditions: machineConditionsToCEL(in.machine.Status.Conditions)}}
	}
	return *in.machineVal
}

// Evaluate compiles (or reuses a cached compilation of) expression and evaluates it
// against input, returning whether the expression matched.
// input.node may be nil, e.g. if the Machine does not have a Node yet. If the expression
// references the "node" variable, it is automatically treated as not matched (without
// error) in that case, so authors don't need to guard node access themselves; expressions
// that don't reference "node" at all are evaluated normally regardless of whether a Node
// exists. Expressions that don't reference "machine" never pay the cost of converting the
// Machine's conditions.
// Only node.status.conditions and machine.status.conditions are made available to the
// expression, built by hand from the respective condition lists rather than by reflecting
// over the (much larger) Node/Machine objects.
func Evaluate(expression string, input *Input) (bool, error) {
	entry, err := compile(expression)
	if err != nil {
		return false, err
	}

	if entry.usesNode && input.node == nil {
		return false, nil
	}

	vars := map[string]any{
		CurrentTimeVariableName: input.now,
	}
	if entry.usesNode {
		vars[NodeVariableName] = input.nodeCELValue()
	}
	if entry.usesMachine {
		vars[MachineVariableName] = input.machineCELValue()
	}

	out, _, err := entry.program.Eval(vars)
	if err != nil {
		return false, err
	}

	result, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("expression did not evaluate to a bool")
	}
	return result, nil
}

// nodeConditionsToCEL converts conditions to the shape exposed to CEL expressions as
// node.status.conditions, matching the field names Kubernetes uses when serializing a
// Node to JSON.
func nodeConditionsToCEL(conditions []corev1.NodeCondition) []nodeConditionForCEL {
	out := make([]nodeConditionForCEL, 0, len(conditions))
	for _, c := range conditions {
		out = append(out, nodeConditionForCEL{
			Type:               string(c.Type),
			Status:             string(c.Status),
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime.UTC().Format(time.RFC3339),
		})
	}
	return out
}

// machineConditionsToCEL converts conditions to the shape exposed to CEL expressions as
// machine.status.conditions, matching the field names Kubernetes uses when serializing a
// metav1.Condition to JSON.
func machineConditionsToCEL(conditions []metav1.Condition) []machineConditionForCEL {
	out := make([]machineConditionForCEL, 0, len(conditions))
	for _, c := range conditions {
		out = append(out, machineConditionForCEL{
			Type:               c.Type,
			Status:             string(c.Status),
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime.UTC().Format(time.RFC3339),
		})
	}
	return out
}
