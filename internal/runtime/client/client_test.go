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

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
	fakev1alpha1 "sigs.k8s.io/cluster-api/internal/runtime/catalog/test/v1alpha1"
	fakev1alpha2 "sigs.k8s.io/cluster-api/internal/runtime/catalog/test/v1alpha2"
)

func TestClient_httpCall(t *testing.T) {
	g := NewWithT(t)

	tableTests := []struct {
		name     string
		request  runtime.Object
		response runtime.Object
		opts     *httpCallOptions
		wantErr  bool
	}{
		{
			"all nil, err",
			nil, nil,
			nil,
			true,
		},
		{
			"nil catalog, err",
			&fakev1alpha1.FakeRequest{}, &fakev1alpha1.FakeResponse{},
			nil,
			true,
		},
		{
			"empty catalog, err",
			&fakev1alpha1.FakeRequest{}, &fakev1alpha1.FakeResponse{},
			&httpCallOptions{
				catalog: catalog.New(),
			},
			true,
		},
		{
			"ok, no conversion",
			&fakev1alpha1.FakeRequest{
				TypeMeta: metav1.TypeMeta{
					Kind:       "FakeRequest",
					APIVersion: fakev1alpha1.GroupVersion.Identifier(),
				},
			},
			&fakev1alpha1.FakeResponse{},
			func() *httpCallOptions {
				c := catalog.New()
				hookFunc := func(req *fakev1alpha1.FakeRequest, resp *fakev1alpha1.FakeResponse) {}
				c.AddHook(
					fakev1alpha1.GroupVersion,
					hookFunc,
					&catalog.HookMeta{},
				)

				// get same gvh for hook by using the hookfunc and catalog
				gvh, err := c.GroupVersionHook(hookFunc)
				g.Expect(err).To(Succeed())

				return &httpCallOptions{
					catalog: c,
					gvh:     gvh,
				}
			}(),
			false,
		},
		{
			"ok, conversion",
			&fakev1alpha2.FakeRequest{
				TypeMeta: metav1.TypeMeta{
					Kind:       "FakeRequest",
					APIVersion: fakev1alpha2.GroupVersion.Identifier(),
				},
			},
			&fakev1alpha2.FakeResponse{},
			func() *httpCallOptions {
				c := catalog.New()
				// register fakev1alpha1 to enable conversion
				g.Expect(fakev1alpha1.AddToCatalog(c)).To(Succeed())
				hookFunc := func(req *fakev1alpha1.FakeRequest, resp *fakev1alpha1.FakeResponse) {}
				c.AddHook(
					fakev1alpha1.GroupVersion,
					hookFunc,
					&catalog.HookMeta{},
				)

				// get same gvh for hook by using the hookfunc and catalog
				gvh, err := c.GroupVersionHook(hookFunc)
				g.Expect(err).To(Succeed())

				return &httpCallOptions{
					catalog: c,
					gvh:     gvh,
				}
			}(),
			false,
		},
	}
	for _, tt := range tableTests {
		t.Run(tt.name, func(t *testing.T) {
			// a http server is only required if we have a valid catalog, otherwise httpCall will not reach out to the server
			if tt.opts != nil && tt.opts.catalog != nil {
				// create http server with fakeHookHandler
				mux := http.NewServeMux()
				mux.HandleFunc("/", fakeHookHandler)
				srv := httptest.NewServer(mux)
				defer srv.Close()

				// set url to srv for in tt.opts
				tt.opts.config.URL = pointer.String(srv.URL)
			}

			assert := g.Expect(httpCall(context.TODO(), tt.request, tt.response, tt.opts))
			if tt.wantErr {
				assert.To(HaveOccurred())
			} else {
				assert.To(Succeed())
			}
		})
	}
}

func fakeHookHandler(w http.ResponseWriter, r *http.Request) {
	response := &fakev1alpha1.FakeResponse{
		TypeMeta: metav1.TypeMeta{
			Kind:       "FakeHookResponse",
			APIVersion: fakev1alpha1.GroupVersion.Identifier(),
		},
		Second: "",
		First:  1,
	}
	respBody, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBody)
}
