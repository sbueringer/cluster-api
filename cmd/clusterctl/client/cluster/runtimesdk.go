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

package cluster

import (
	"context"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1 "sigs.k8s.io/cluster-api/exp/runtime/api/v1alpha1"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	runtimeclient "sigs.k8s.io/cluster-api/internal/runtime/client"
	runtimeregistry "sigs.k8s.io/cluster-api/internal/runtime/registry"
)

func newClusterctlRuntimeClient(client client.Client) runtimeclient.Client {
	catalog := runtimecatalog.New()
	_ = runtimehooksv1.AddToCatalog(catalog)

	registry := runtimeregistry.New()

	_ = registry.WarmUp(&runtimev1.ExtensionConfigList{})

	return &clusterctlRuntimeClient{
		Client: runtimeclient.New(runtimeclient.Options{
			Catalog:  catalog,
			Registry: registry,
			Client:   client,
		}),
	}
}

type clusterctlRuntimeClient struct {
	runtimeclient.Client
}

func (c clusterctlRuntimeClient) CallExtension(ctx context.Context, hook runtimecatalog.Hook, forObject metav1.Object, name string, request runtime.Object, response runtimehooksv1.ResponseObject) error {
	return errors.Errorf("Topology Mutation Hook is not yet implemented for clusterctl")
}
