//go:build !race

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
	"reflect"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/randfill"

	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
)

const (
	fakeID     = "abcdef"
	fakeSecret = "abcdef0123456789"
)

// Test is disabled when the race detector is enabled (via "//go:build !race" above) because otherwise the fuzz tests would just time out.

func TestFuzzyConversion(t *testing.T) {
	t.Run("for KubeadmConfig", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Hub:         &bootstrapv1.KubeadmConfig{},
		Spoke:       &KubeadmConfig{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{KubeadmConfigFuzzFuncs},
	}))
	t.Run("for KubeadmConfigTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Hub:         &bootstrapv1.KubeadmConfigTemplate{},
		Spoke:       &KubeadmConfigTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{KubeadmConfigTemplateFuzzFuncs},
	}))
}

func KubeadmConfigFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubKubeadmConfigStatus,
		spokeKubeadmConfigSpec,
		spokeKubeadmConfigStatus,
		spokeClusterConfiguration,
		hubBootstrapTokenString,
		spokeBootstrapTokenString,
		spokeAPIServer,
		spokeDiscovery,
		hubKubeadmConfigSpec,
		hubNodeRegistrationOptions,
		spokeBootstrapToken,
	}
}

func KubeadmConfigTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		spokeKubeadmConfigSpec,
		spokeClusterConfiguration,
		hubBootstrapTokenString,
		spokeBootstrapTokenString,
		spokeAPIServer,
		spokeDiscovery,
		hubKubeadmConfigSpec,
		hubNodeRegistrationOptions,
		spokeBootstrapToken,
	}
}

func hubKubeadmConfigSpec(in *bootstrapv1.KubeadmConfigSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// enforce ControlPlaneComponentHealthCheckSeconds to be equal on init and join configuration
	initControlPlaneComponentHealthCheckSeconds := in.InitConfiguration.Timeouts.ControlPlaneComponentHealthCheckSeconds
	if in.JoinConfiguration.IsDefined() || initControlPlaneComponentHealthCheckSeconds != nil {
		in.JoinConfiguration.Timeouts.ControlPlaneComponentHealthCheckSeconds = initControlPlaneComponentHealthCheckSeconds
	}

	if in.ClusterConfiguration.APIServer.ExtraEnvs != nil && *in.ClusterConfiguration.APIServer.ExtraEnvs == nil {
		in.ClusterConfiguration.APIServer.ExtraEnvs = nil
	}
	if in.ClusterConfiguration.ControllerManager.ExtraEnvs != nil && *in.ClusterConfiguration.ControllerManager.ExtraEnvs == nil {
		in.ClusterConfiguration.ControllerManager.ExtraEnvs = nil
	}
	if in.ClusterConfiguration.Scheduler.ExtraEnvs != nil && *in.ClusterConfiguration.Scheduler.ExtraEnvs == nil {
		in.ClusterConfiguration.Scheduler.ExtraEnvs = nil
	}
	if in.ClusterConfiguration.Etcd.Local.ExtraEnvs != nil && *in.ClusterConfiguration.Etcd.Local.ExtraEnvs == nil {
		in.ClusterConfiguration.Etcd.Local.ExtraEnvs = nil
	}
}

func hubNodeRegistrationOptions(in *bootstrapv1.NodeRegistrationOptions, c randfill.Continue) {
	c.FillNoCustom(in)

	if in.Taints != nil && *in.Taints == nil {
		in.Taints = nil
	}
}

func hubKubeadmConfigStatus(in *bootstrapv1.KubeadmConfigStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Always create struct with at least one mandatory fields.
	if in.Deprecated == nil {
		in.Deprecated = &bootstrapv1.KubeadmConfigDeprecatedStatus{}
	}
	if in.Deprecated.V1Beta1 == nil {
		in.Deprecated.V1Beta1 = &bootstrapv1.KubeadmConfigV1Beta1DeprecatedStatus{}
	}
}

func spokeKubeadmConfigSpec(in *KubeadmConfigSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Drop UseExperimentalRetryJoin as we intentionally don't preserve it.
	in.UseExperimentalRetryJoin = false

	dropEmptyStringsKubeadmConfigSpec(in)

	if in.DiskSetup != nil && reflect.DeepEqual(in.DiskSetup, &DiskSetup{}) {
		in.DiskSetup = nil
	}
	if in.NTP != nil && reflect.DeepEqual(in.NTP, &NTP{}) {
		in.NTP = nil
	}
	for i, file := range in.Files {
		if file.ContentFrom != nil && reflect.DeepEqual(file.ContentFrom, &FileSource{}) {
			file.ContentFrom = nil
		}
		in.Files[i] = file
	}
}

func spokeKubeadmConfigStatus(obj *KubeadmConfigStatus, c randfill.Continue) {
	c.FillNoCustom(obj)

	dropEmptyStringsKubeadmConfigStatus(obj)
}

func spokeClusterConfiguration(in *ClusterConfiguration, c randfill.Continue) {
	c.FillNoCustom(in)

	// Drop the following fields as they have been removed in v1beta2, so we don't have to preserve them.
	in.Networking.ServiceSubnet = ""
	in.Networking.PodSubnet = ""
	in.Networking.DNSDomain = ""
	in.KubernetesVersion = ""
	in.ClusterName = ""

	if in.Etcd.Local != nil && reflect.DeepEqual(in.Etcd.Local, &LocalEtcd{}) {
		in.Etcd.Local = nil
	}
	if in.Etcd.External != nil && reflect.DeepEqual(in.Etcd.External, &ExternalEtcd{}) {
		in.Etcd.External = nil
	}
}

func hubBootstrapTokenString(in *bootstrapv1.BootstrapTokenString, _ randfill.Continue) {
	in.ID = fakeID
	in.Secret = fakeSecret
}

func spokeBootstrapTokenString(in *BootstrapTokenString, _ randfill.Continue) {
	in.ID = fakeID
	in.Secret = fakeSecret
}

func spokeAPIServer(in *APIServer, c randfill.Continue) {
	c.FillNoCustom(in)

	if in.TimeoutForControlPlane != nil {
		in.TimeoutForControlPlane = ptr.To[metav1.Duration](metav1.Duration{Duration: time.Duration(c.Int31()) * time.Second})
	}
}

func spokeBootstrapToken(in *BootstrapToken, c randfill.Continue) {
	c.FillNoCustom(in)

	if in.TTL != nil {
		in.TTL = ptr.To[metav1.Duration](metav1.Duration{Duration: time.Duration(c.Int31()) * time.Second})
	}
	if reflect.DeepEqual(in.Expires, &metav1.Time{}) {
		in.Expires = nil
	}
}

func spokeDiscovery(in *Discovery, c randfill.Continue) {
	c.FillNoCustom(in)

	if in.Timeout != nil {
		in.Timeout = ptr.To[metav1.Duration](metav1.Duration{Duration: time.Duration(c.Int31()) * time.Second})
	}
	if in.File != nil {
		if reflect.DeepEqual(in.File, &FileDiscovery{}) {
			in.File = nil
		}
	}
	if in.BootstrapToken != nil && reflect.DeepEqual(in.BootstrapToken, &BootstrapTokenDiscovery{}) {
		in.BootstrapToken = nil
	}
}
