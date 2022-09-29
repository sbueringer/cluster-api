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

package v1alpha4

import (
	"testing"

	infrav1 "sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
)

func TestFuzzyConversion(t *testing.T) {
	t.Run("for DockerCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Hub:   &infrav1.DockerCluster{},
		Spoke: &DockerCluster{},
	}))

	t.Run("for DockerClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Hub:   &infrav1.DockerClusterTemplate{},
		Spoke: &DockerClusterTemplate{},
	}))

	t.Run("for DockerMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Hub:   &infrav1.DockerMachine{},
		Spoke: &DockerMachine{},
	}))

	t.Run("for DockerMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Hub:   &infrav1.DockerMachineTemplate{},
		Spoke: &DockerMachineTemplate{},
	}))
}
