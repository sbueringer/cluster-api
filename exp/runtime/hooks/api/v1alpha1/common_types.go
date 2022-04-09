/*
Copyright 2022 The Kubernetes Authors.

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

package v1alpha1

import (
	"fmt"
)

// ResponseStatus represents the status of the hook response.
// +enum
type ResponseStatus string

const (
	ResponseStatusSuccess ResponseStatus = "Success"

	ResponseStatusFailure ResponseStatus = "Failure"
)

var _ error = &ErrExtensionFailure{}

type ErrExtensionFailure struct {
	ExtensionName string
	Message       string
}

func (e *ErrExtensionFailure) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("extension %q failed: %s", e.ExtensionName, e.Message)
	}
	return fmt.Sprintf("extension %q failed", e.ExtensionName)
}
