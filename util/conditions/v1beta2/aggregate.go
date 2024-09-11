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

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// AggregateOption is some configuration that modifies options for a aggregate call.
type AggregateOption interface {
	// ApplyToAggregate applies this configuration to the given aggregate options.
	ApplyToAggregate(option *AggregateOptions)
}

// AggregateOptions allows to set options for the aggregate operation.
type AggregateOptions struct {
	mergeStrategy       MergeStrategy
	targetConditionType string
}

// ApplyOptions applies the given list options on these options,
// and then returns itself (for convenient chaining).
func (o *AggregateOptions) ApplyOptions(opts []AggregateOption) *AggregateOptions {
	for _, opt := range opts {
		opt.ApplyToAggregate(o)
	}
	return o
}

// NewAggregateCondition aggregates a condition from a list of objects; the given condition must have positive polarity;
// if the given condition does not exist in one of the source objects, missing conditions are considered Unknown, reason NotYetReported.
//
// By default, the Aggregate condition has the same type of the source condition, but this can be changed by using
// the TargetConditionType option.
//
// Additionally, it is possible to inject custom merge strategies using the WithMergeStrategy option.
func NewAggregateCondition(sourceObjs []runtime.Object, sourceConditionType string, opts ...AggregateOption) (*metav1.Condition, error) {
	if len(sourceObjs) == 0 {
		return nil, errors.New("sourceObjs can't be empty")
	}

	aggregateOpt := &AggregateOptions{
		mergeStrategy:       newDefaultMergeStrategy(),
		targetConditionType: sourceConditionType,
	}
	aggregateOpt.ApplyOptions(opts)

	conditionsInScope := make([]ConditionWithOwnerInfo, 0, len(sourceObjs))
	for _, obj := range sourceObjs {
		conditions, err := getConditionsWithOwnerInfo(obj)
		if err != nil {
			// Note: considering all sourceObjs are usually of the same type (and thus getConditionsWithOwnerInfo will either pass or fail for all sourceObjs), we are returning at the first error.
			// This also avoid to implement fancy error aggregation, which is required to manage a potentially high number of sourceObjs/errors.
			return nil, err
		}

		// Drops all the conditions not in scope for the merge operation
		hasConditionType := false
		for _, condition := range conditions {
			if condition.Type != sourceConditionType {
				continue
			}
			conditionsInScope = append(conditionsInScope, condition)
			hasConditionType = true
			break
		}

		// Add the expected conditions if it does not exist, so we are compliant with K8s guidelines
		// (all missing conditions should be considered unknown).
		if !hasConditionType {
			conditionOwner := getConditionOwnerInfo(obj)

			conditionsInScope = append(conditionsInScope, ConditionWithOwnerInfo{
				OwnerResource: conditionOwner,
				Condition: metav1.Condition{
					Type:    aggregateOpt.targetConditionType,
					Status:  metav1.ConditionUnknown,
					Reason:  NotYetReportedReason,
					Message: fmt.Sprintf("Condition %s not yet reported from %s", sourceConditionType, conditionOwner.Kind),
					// NOTE: LastTransitionTime and ObservedGeneration are not relevant for merge.
				},
			})
		}
	}

	status, reason, message, err := aggregateOpt.mergeStrategy.Merge(
		conditionsInScope,
		[]string{sourceConditionType},
		nil,   // negative conditions
		false, // step counter
	)
	if err != nil {
		return nil, err
	}

	c := &metav1.Condition{
		Type:    aggregateOpt.targetConditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
		// NOTE: LastTransitionTime and ObservedGeneration will be set when this condition is added to an object by calling Set.
	}

	return c, err
}

// SetAggregateCondition is a convenience method that calls NewAggregateCondition to create an aggregate condition from the source objects,
// and then calls Set to add the new condition to the target object.
func SetAggregateCondition(sourceObjs []runtime.Object, targetObj runtime.Object, conditionType string, opts ...AggregateOption) error {
	aggregateCondition, err := NewAggregateCondition(sourceObjs, conditionType, opts...)
	if err != nil {
		return err
	}
	return Set(targetObj, *aggregateCondition)
}