/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1

import (
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

func SetupWebhooksWithManager(mgr ctrl.Manager) error {
	if err := (&Machine{}).SetupWebhookWithManager(mgr); err != nil {
		return errors.Wrapf(err, "failed to create Machine webhook")
	}

	if err := (&MachineSet{}).SetupWebhookWithManager(mgr); err != nil {
		return errors.Wrapf(err, "failed to create MachineSet webhook")
	}

	if err := (&MachineDeployment{}).SetupWebhookWithManager(mgr); err != nil {
		return errors.Wrapf(err, "failed to create MachineDeployment webhook")
	}

	if err := (&MachineHealthCheck{}).SetupWebhookWithManager(mgr); err != nil {
		return errors.Wrapf(err, "failed to create MachineHealthCheck webhook")
	}

	return nil
}
