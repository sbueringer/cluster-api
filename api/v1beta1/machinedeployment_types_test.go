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
	"context"
	"fmt"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var testScheme = runtime.NewScheme()

func init() {
	_ = clientgoscheme.AddToScheme(testScheme)
	_ = apiextensionsv1.AddToScheme(testScheme)
	_ = AddToScheme(testScheme)
}

func TestList(t *testing.T) {
	cfg := ctrl.GetConfigOrDie()

	c, err := client.New(cfg, client.Options{
		Scheme: testScheme,
	})
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	list := &MachineDeploymentList{}
	if err := c.List(ctx, list); err != nil {
		panic(err)
	}

	md := &MachineDeployment{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: "quick-start-hr8mgd", Name: "quick-start-bcd1q5-md-0"}, md); err != nil {
		panic(err)
	}

	md.Status.TestReplicasPointer = nil
	md.Status.TestReplicasPointerOmitEmpty = nil
	if err := c.Status().Update(ctx, md); err != nil {
		panic(err)
	}

	mdfinal := &MachineDeployment{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: "quick-start-hr8mgd", Name: "quick-start-bcd1q5-md-0"}, mdfinal); err != nil {
		panic(err)
	}

	fmt.Println(list)
}
