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
	"reflect"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// TODO: Move to the API package.
const (
	// NoReasonReported identifies a clusterv1.Condition that reports no reason.
	NoReasonReported = "NoReasonReported"
)

// Get returns a condition from the sourceObj.
//
// Get supports retrieving conditions from objects at different stages of the transition to the metav1.Condition type:
//   - Objects with clusterv1.Condition in status.conditions; in this case a best effort conversion
//     to metav1.Condition is performed, just enough to allow surfacing a condition from a provider object with Mirror
//   - Objects with metav1.Condition in status.v1beta2.conditions
//   - Objects with metav1.Condition in status.conditions
//
// Please note that Get also supports reading conditions from unstructured objects; in this case, best effort
// conversion from a map is performed, just enough to allow surfacing a condition from a providers object with Mirror
//
// In case the object does not have metav1.Conditions, Get tries to read clusterv1.Conditions from status.conditions
// and convert them to metav1.Conditions.
func Get(sourceObj runtime.Object, sourceConditionType string) (*metav1.Condition, error) {
	conditions, err := GetAll(sourceObj)
	if err != nil {
		return nil, err
	}
	return meta.FindStatusCondition(conditions, sourceConditionType), nil
}

// GetAll returns all the conditions from the object.
//
// GetAll supports retrieving conditions from objects at different stages of the transition to the metav1.Condition type:
//   - Objects with clusterv1.Condition in status.conditions; in this case a best effort conversion
//     to metav1.Condition is performed, just enough to allow surfacing a condition from a providers object with Mirror
//   - Objects with metav1.Condition in status.v1beta2.conditions
//   - Objects with metav1.Condition in status.conditions
//
// Please note that GetAll also supports reading conditions from unstructured objects; in this case, best effort
// conversion from a map is performed, just enough to allow surfacing a condition from a providers object with Mirror
//
// In case the object does not have metav1.Conditions, GetAll tries to read clusterv1.Conditions from status.conditions
// and convert them to metav1.Conditions.
func GetAll(sourceObj runtime.Object) ([]metav1.Condition, error) {
	if sourceObj == nil {
		return nil, errors.New("sourceObj cannot be nil")
	}

	switch sourceObj.(type) {
	case runtime.Unstructured:
		return getFromUnstructuredObject(sourceObj)
	default:
		return getFromTypedObject(sourceObj)
	}
}

func getFromUnstructuredObject(obj runtime.Object) ([]metav1.Condition, error) {
	u, ok := obj.(runtime.Unstructured)
	if !ok {
		// NOTE: this should not happen due to the type assertion before calling this func
		return nil, errors.New("obj cannot be converted to runtime.Unstructured")
	}

	if !reflect.ValueOf(u).Elem().IsValid() {
		return nil, errors.New("obj cannot be nil")
	}

	ownerInfo := getConditionOwnerInfo(obj)

	value, exists, err := unstructured.NestedFieldNoCopy(u.UnstructuredContent(), "status", "v1beta2", "conditions")
	if exists && err == nil {
		if conditions, ok := value.([]interface{}); ok {
			r, err := convertFromUnstructuredConditions(conditions)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to convert %s.status.v1beta2.conditions to []metav1.Condition", ownerInfo)
			}
			return r, nil
		}
	}

	value, exists, err = unstructured.NestedFieldNoCopy(u.UnstructuredContent(), "status", "conditions")
	if exists && err == nil {
		if conditions, ok := value.([]interface{}); ok {
			r, err := convertFromUnstructuredConditions(conditions)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to convert %s.status.conditions to []metav1.Condition", ownerInfo)
			}
			return r, nil
		}
	}

	return nil, errors.Errorf("%s must have status with one of conditions or v1beta2.conditions", ownerInfo)
}

// convertFromUnstructuredConditions converts []interface{} to []metav1.Condition; this operation must account for
// objects which are not transitioning to metav1.Condition, or not yet fully transitioned, and thus a best
// effort conversion of values to metav1.Condition is performed.
func convertFromUnstructuredConditions(conditions []interface{}) ([]metav1.Condition, error) {
	if conditions == nil {
		return nil, nil
	}

	convertedConditions := make([]metav1.Condition, 0, len(conditions))
	for _, c := range conditions {
		cMap, ok := c.(map[string]interface{})
		if !ok || cMap == nil {
			continue
		}

		var conditionType string
		if v, ok := cMap["type"]; ok {
			conditionType = v.(string)
		}

		var status string
		if v, ok := cMap["status"]; ok {
			status = v.(string)
		}

		var observedGeneration int64
		if v, ok := cMap["observedGeneration"]; ok {
			observedGeneration = v.(int64)
		}

		var lastTransitionTime metav1.Time
		if v, ok := cMap["lastTransitionTime"]; ok && v != nil && v.(string) != "" {
			if err := lastTransitionTime.UnmarshalQueryParameter(v.(string)); err != nil {
				return nil, errors.Wrapf(err, "failed to unmarshal lastTransitionTime value: %s", v)
			}
		}

		var reason string
		if v, ok := cMap["reason"]; ok {
			reason = v.(string)
		}

		var message string
		if v, ok := cMap["message"]; ok {
			message = v.(string)
		}

		c := metav1.Condition{
			Type:               conditionType,
			Status:             metav1.ConditionStatus(status),
			ObservedGeneration: observedGeneration,
			LastTransitionTime: lastTransitionTime,
			Reason:             reason,
			Message:            message,
		}
		if err := validateAndFixConvertedCondition(&c); err != nil {
			return nil, err
		}

		convertedConditions = append(convertedConditions, c)
	}
	return convertedConditions, nil
}

func getFromTypedObject(obj runtime.Object) ([]metav1.Condition, error) {
	ptr := reflect.ValueOf(obj)
	if ptr.Kind() != reflect.Pointer {
		return nil, errors.New("obj must be a pointer")
	}

	elem := ptr.Elem()
	if !elem.IsValid() {
		return nil, errors.New("obj must be a valid value (non zero value of its type)")
	}

	ownerInfo := getConditionOwnerInfo(obj)

	statusField := elem.FieldByName("Status")
	if statusField == (reflect.Value{}) {
		return nil, errors.Errorf("%s must have a status field", ownerInfo)
	}

	// Get conditions.
	// NOTE: Given that we allow providers to migrate at different speed, it is required to support objects at the different stage of the transition from legacy conditions to metav1.Condition.
	// In order to handle this, first try to read Status.V1Beta2.Conditions, then Status.Conditions; for Status.Conditions, also support conversion from legacy conditions.
	// The V1Beta2 branch and the conversion from legacy conditions should be dropped when the v1beta1 API is removed.

	if v1beta2Field := statusField.FieldByName("V1Beta2"); v1beta2Field != (reflect.Value{}) {
		if conditionField := v1beta2Field.FieldByName("Conditions"); conditionField != (reflect.Value{}) {
			v1beta2Conditions, ok := conditionField.Interface().([]metav1.Condition)
			if !ok {
				return nil, errors.Errorf("%s.status.v1beta2.conditions must be of type []metav1.Condition", ownerInfo)
			}
			return v1beta2Conditions, nil
		}
	}

	if conditionField := statusField.FieldByName("Conditions"); conditionField != (reflect.Value{}) {
		conditions, ok := conditionField.Interface().([]metav1.Condition)
		if ok {
			return conditions, nil
		}

		v1betaConditions, ok := conditionField.Interface().(clusterv1.Conditions)
		if !ok {
			return nil, errors.Errorf("%s.status.conditions must be of type []metav1.Condition or clusterv1.Conditions", ownerInfo)
		}
		r, err := convertFromV1Beta1Conditions(v1betaConditions)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to convert %s.status.conditions to []metav1.Condition", ownerInfo)
		}
		return r, nil
	}

	return nil, errors.Errorf("%s.status must have one of conditions or v1beta2.conditions", ownerInfo)
}

// convertFromV1Beta1Conditions converts a clusterv1.Conditions to []metav1.Condition.
// NOTE: this operation is performed at best effort and assuming conditions have been set using Cluster API condition utils.
func convertFromV1Beta1Conditions(v1betaConditions clusterv1.Conditions) ([]metav1.Condition, error) {
	convertedConditions := make([]metav1.Condition, len(v1betaConditions))
	for i, c := range v1betaConditions {
		convertedConditions[i] = metav1.Condition{
			Type:               string(c.Type),
			Status:             metav1.ConditionStatus(c.Status),
			LastTransitionTime: c.LastTransitionTime,
			Reason:             c.Reason,
			Message:            c.Message,
		}

		if err := validateAndFixConvertedCondition(&convertedConditions[i]); err != nil {
			return nil, err
		}
	}
	return convertedConditions, nil
}

// validateAndFixConvertedCondition validates and fixes a clusterv1.Condition converted to a metav1.Condition.
// this operation assumes conditions have been set using Cluster API condition utils;
// also, only a few, minimal rules are enforced, just enough to allow surfacing a condition from a providers object with Mirror.
func validateAndFixConvertedCondition(c *metav1.Condition) error {
	if c.Type == "" {
		return errors.New("condition type must be set")
	}
	if c.Status == "" {
		return errors.New("condition status must be set")
	}
	if c.Reason == "" {
		switch c.Status {
		case metav1.ConditionFalse: // When using old Cluster API condition utils, for conditions with Status false, Reason can be empty only when a condition has negative polarity (means "good")
			c.Reason = NoReasonReported
		case metav1.ConditionTrue: // When using old Cluster API condition utils, for conditions with Status true, Reason can be empty only when a condition has positive polarity (means "good").
			c.Reason = NoReasonReported
		case metav1.ConditionUnknown:
			return errors.New("condition reason must be set when a condition is unknown")
		}
	}

	// NOTE: Empty LastTransitionTime is tolerated because it will be set when assigning the newly generated mirror condition to an object.
	// NOTE: Other metav1.Condition validations rules, e.g. regex, are not enforced at this stage; they will be enforced by the API server at a later stage.

	return nil
}
