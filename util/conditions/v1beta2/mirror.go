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
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NotYetReportedReason is set on missing conditions generated during mirror, aggregate or summary operations.
// Missing conditions are generated during the above operations when an expected condition does not exist on a object.
// TODO: Move to the API package.
const NotYetReportedReason = "NotYetReported"

// MirrorOption is some configuration that modifies options for a mirror call.
type MirrorOption interface {
	// ApplyToMirror applies this configuration to the given mirror options.
	ApplyToMirror(*MirrorOptions)
}

// MirrorOptions allows to set options for the mirror operation.
type MirrorOptions struct {
	targetConditionType string
}

// ApplyOptions applies the given list options on these options,
// and then returns itself (for convenient chaining).
func (o *MirrorOptions) ApplyOptions(opts []MirrorOption) *MirrorOptions {
	for _, opt := range opts {
		opt.ApplyToMirror(o)
	}
	return o
}

// NewMirrorCondition create a mirror of the given condition from obj; if the given condition does not exist in the source obj,
// a new condition with status Unknown, reason NotYetReported is created.
//
// By default, the Mirror condition has the same type of the source condition, but this can be changed by using
// the TargetConditionType option.
func NewMirrorCondition(sourceObj runtime.Object, sourceConditionType string, opts ...MirrorOption) (*metav1.Condition, error) {
	mirrorOpt := &MirrorOptions{
		targetConditionType: sourceConditionType,
	}
	mirrorOpt.ApplyOptions(opts)

	conditionOwner := getConditionOwnerInfo(sourceObj)
	conditionOwnerString := fmt.Sprintf("%s %s", conditionOwner.Kind, conditionOwner.Name)

	condition, err := Get(sourceObj, sourceConditionType)
	if err != nil {
		return nil, err
	}
	if condition == nil {
		return &metav1.Condition{
			Type:               mirrorOpt.targetConditionType,
			Status:             metav1.ConditionUnknown,
			LastTransitionTime: metav1.Now(),
			Reason:             NotYetReportedReason,
			Message:            fmt.Sprintf("Condition %s not yet reported from %s", sourceConditionType, conditionOwnerString),
			// NOTE: LastTransitionTime and ObservedGeneration will be set when this condition is added to an object by calling Set.
		}, nil
	}

	return &metav1.Condition{
		Type:               mirrorOpt.targetConditionType,
		Status:             condition.Status,
		ObservedGeneration: condition.ObservedGeneration,
		LastTransitionTime: condition.LastTransitionTime,
		Reason:             condition.Reason,
		Message:            strings.TrimSpace(fmt.Sprintf("%s (from %s)", condition.Message, conditionOwnerString)),
		// NOTE: LastTransitionTime and ObservedGeneration will be set when this condition is added to an object by calling Set.
	}, nil
}

// SetMirrorCondition is a convenience method that calls NewMirrorCondition to create a mirror condition from the source object,
// and then calls Set to add the new condition to the target object.
func SetMirrorCondition(sourceObj, targetObj runtime.Object, sourceConditionType string, opts ...MirrorOption) error {
	mirrorCondition, err := NewMirrorCondition(sourceObj, sourceConditionType, opts...)
	if err != nil {
		return err
	}
	return Set(targetObj, *mirrorCondition)
}
