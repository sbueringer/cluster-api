//go:build e2e
// +build e2e

/*
Copyright 2020 The Kubernetes Authors.

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

package e2e

import (
	. "github.com/onsi/ginkgo"
)

var _ = Describe("When testing clusterctl upgrades [clusterctl-Upgrade]", func() {
	ClusterctlUpgradeSpec(ctx, func() ClusterctlUpgradeSpecInput {
		return ClusterctlUpgradeSpecInput{
			E2EConfig:             e2eConfig,
			ClusterctlConfigPath:  clusterctlConfigPath,
			BootstrapClusterProxy: bootstrapClusterProxy,
			ArtifactFolder:        artifactFolder,
			SkipCleanup:           skipCleanup,
			// Also copy cluster-template + ClusterClass to v1.0.x folder in local clusterctl repository + drop patches/variable from both, ref:
			// /Users/buringerst/code/src/sigs.k8s.io/cluster-api/_artifacts/repository/infrastructure-docker/v1.1.99/cluster-template-topology.yaml
			// Deploy ClusterClass somewhere in the middle (when deploying the workload cluster) because clusterctl v1.0.x doesn't do it automatically.
			WorkloadFlavor:        "topology",
		}
	})
})
