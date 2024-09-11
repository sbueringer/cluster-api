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
	"sort"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SetOption is some configuration that modifies options for a Set request.
type SetOption interface {
	// ApplyToSet applies this configuration to the given Set options.
	ApplyToSet(option *SetOptions)
}

// SetOptions allows to define options for the set operation.
type SetOptions struct {
	conditionSortFunc ConditionSortFunc
}

// ApplyOptions applies the given list options on these options,
// and then returns itself (for convenient chaining).
func (o *SetOptions) ApplyOptions(opts []SetOption) *SetOptions {
	for _, opt := range opts {
		opt.ApplyToSet(o)
	}
	return o
}

// Set a condition on the given object.
//
// Set support adding conditions to objects at different stages of the transition to metav1.condition type:
// - Objects with metav1.condition in status.v1beta2.conditions conditions
// - Objects with metav1.condition in status.conditions
//
// When setting a condition:
// - condition.ObservedGeneration will be set to object.Metadata.Generation.
// - If the condition does not exist and condition.LastTransitionTime is not set, time.Now is used.
// - If the condition already exist, condition.Status is changing and condition.LastTransitionTime is not set, time.Now is used.
// - If the condition already exist, condition.Status is NOT changing, all the fields can be changed except for condition.LastTransitionTime.
//
// Set can't be used with unstructured objects.
//
// Additionally, Set enforce the a default condition order (Available and Ready fist, everything else in alphabetical order),
// but this can be changed by using the ConditionSortFunc option.
func Set(targetObj runtime.Object, condition metav1.Condition, opts ...SetOption) error {
	conditions, err := GetAll(targetObj)
	if err != nil {
		return err
	}

	if objMeta, ok := targetObj.(metav1.Object); ok {
		condition.ObservedGeneration = objMeta.GetGeneration()
	}

	if changed := meta.SetStatusCondition(&conditions, condition); !changed {
		return nil
	}

	return SetAll(targetObj, conditions, opts...)
}

// SetAll the conditions on the given object.
//
// SetAll support adding conditions to objects at different stages of the transition to metav1.condition type:
// - Objects with metav1.condition in status.v1beta2.conditions conditions
// - Objects with metav1.condition in status.conditions
//
// SetAll can't be used with unstructured objects.
//
// Additionally, SetAll enforce a default condition order (Available and Ready fist, everything else in alphabetical order),
// but this can be changed by using the ConditionSortFunc option.
func SetAll(targetObj runtime.Object, conditions []metav1.Condition, opts ...SetOption) error {
	setOpt := &SetOptions{
		// By default sort condition by the default condition order (first available, then ready, then the other conditions if alphabetical order.
		conditionSortFunc: defaultSortLessFunc,
	}
	setOpt.ApplyOptions(opts)

	if setOpt.conditionSortFunc != nil {
		sort.SliceStable(conditions, func(i, j int) bool {
			return setOpt.conditionSortFunc(conditions[i], conditions[j])
		})
	}

	switch targetObj.(type) {
	case runtime.Unstructured:
		return errors.New("cannot set conditions on unstructured objects")
	default:
		return setToTypedObject(targetObj, conditions)
	}
}

var metav1ConditionsType = reflect.TypeOf([]metav1.Condition{})

func setToTypedObject(obj runtime.Object, conditions []metav1.Condition) error {
	if obj == nil {
		return errors.New("cannot set conditions on a nil object")
	}

	ptr := reflect.ValueOf(obj)
	if ptr.Kind() != reflect.Pointer {
		return errors.New("cannot set conditions on a object that is not a pointer")
	}

	elem := ptr.Elem()
	if !elem.IsValid() {
		return errors.New("obj must be a valid value (non zero value of its type)")
	}

	ownerInfo := getConditionOwnerInfo(obj)

	statusField := elem.FieldByName("Status")
	if statusField == (reflect.Value{}) {
		return errors.Errorf("cannot set conditions on %s, status field is missing", ownerInfo)
	}

	// Set conditions.
	// NOTE: Given that we allow providers to migrate at different speed, it is required to support objects at the different stage of the transition from legacy conditions to metav1.conditions.
	// In order to handle this, first try to set Status.V1Beta2.Conditions, then Status.Conditions.
	// The V1Beta2 branch should be dropped when v1beta1 API are removed.

	if v1beta2Field := statusField.FieldByName("V1Beta2"); v1beta2Field != (reflect.Value{}) {
		if conditionField := v1beta2Field.FieldByName("Conditions"); conditionField != (reflect.Value{}) {
			if conditionField.Type() != metav1ConditionsType {
				return errors.Errorf("cannot set conditions on %s.v1beta2.conditions, the field doesn't have []metav1.Condition type: %s type detected", ownerInfo, reflect.TypeOf(conditionField.Interface()).String())
			}

			setToTypedField(conditionField, conditions)
			return nil
		}
	}

	if conditionField := statusField.FieldByName("Conditions"); conditionField != (reflect.Value{}) {
		if conditionField.Type() != metav1ConditionsType {
			return errors.Errorf("cannot set conditions on %s.conditions, the field doesn't have []metav1.Condition type []metav1.Condition: %s type detected", ownerInfo, reflect.TypeOf(conditionField.Interface()).String())
		}

		setToTypedField(conditionField, conditions)
		return nil
	}

	return errors.Errorf("cannot set conditions on %s both status.v1beta2.conditions and status.conditions fields are missing", ownerInfo)
}

func setToTypedField(conditionField reflect.Value, conditions []metav1.Condition) {
	n := len(conditions)
	conditionField.Set(reflect.MakeSlice(conditionField.Type(), n, n))
	for i := range n {
		itemField := conditionField.Index(i)
		itemField.Set(reflect.ValueOf(conditions[i]))
	}
}
