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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
	fakev1alpha1 "sigs.k8s.io/cluster-api/internal/runtime/catalog/test/v1alpha1"
	fakev1alpha2 "sigs.k8s.io/cluster-api/internal/runtime/catalog/test/v1alpha2"
)

func TestClient_httpCall(t *testing.T) {
	g := NewWithT(t)

	ctx := context.TODO()

	discoveryHookHandler := func(w http.ResponseWriter, r *http.Request) {
		response := &fakev1alpha1.FakeResponse{
			TypeMeta: metav1.TypeMeta{
				Kind:       "DiscoveryHookResponse",
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
		w.Write(respBody)
	}

	t.Run("pass all nil", func(t *testing.T) {
		g.Expect(httpCall(ctx, nil, nil, nil)).To(HaveOccurred())
	})
	t.Run("pass nil catalog", func(t *testing.T) {
		g.Expect(httpCall(ctx, &fakev1alpha1.FakeRequest{}, &fakev1alpha1.FakeResponse{}, &httpCallOptions{})).To(HaveOccurred())
	})
	t.Run("pass empty catalog", func(t *testing.T) {
		opts := &httpCallOptions{
			catalog: catalog.New(),
		}
		g.Expect(httpCall(ctx, &fakev1alpha1.FakeRequest{}, &fakev1alpha1.FakeResponse{}, opts)).To(HaveOccurred())
	})
	t.Run("ok, no conversion, discovery hook", func(t *testing.T) {
		// create catalog containing a DiscoveryHook
		c := catalog.New()
		hookFunc := func(req *fakev1alpha1.FakeRequest, resp *fakev1alpha1.FakeResponse) {}
		c.AddHook(
			fakev1alpha1.GroupVersion,
			hookFunc,
			&catalog.HookMeta{},
		)

		// get same gvh for hook by using the hookfunc
		gvh, err := c.GroupVersionHook(hookFunc)
		g.Expect(err).To(Succeed())

		// create opts having gvh same as discovery hook
		opts := &httpCallOptions{
			catalog: c,
			gvh:     gvh,
		}
		// use same gvh for req as for hook in catalog
		req := &fakev1alpha1.FakeRequest{
			TypeMeta: metav1.TypeMeta{
				Kind:       "DiscoveryHookRequest",
				APIVersion: fakev1alpha1.GroupVersion.Identifier(),
			},
		}
		resp := &fakev1alpha1.FakeResponse{}

		// create http server with hook
		mux := http.NewServeMux()
		mux.HandleFunc("/", discoveryHookHandler)
		srv := httptest.NewServer(mux)
		defer srv.Close()

		// set hook url
		opts.config.URL = pointer.String(srv.URL)

		g.Expect(httpCall(ctx, req, resp, opts)).To(Succeed())
	})
	t.Run("ok, conversion, discovery hook", func(t *testing.T) {
		// create catalog containing a DiscoveryHook
		c := catalog.New()
		// register fakev1alpha2 to enable conversion
		g.Expect(fakev1alpha2.AddToCatalog(c)).To(Succeed())
		hookFunc := func(req *fakev1alpha2.FakeRequest, resp *fakev1alpha2.FakeResponse) {}
		c.AddHook(
			fakev1alpha2.GroupVersion,
			hookFunc,
			&catalog.HookMeta{},
		)

		// get same gvh for hook by using the hookfunc
		gvh, err := c.GroupVersionHook(hookFunc)
		g.Expect(err).To(Succeed())

		// create opts having gvh same as discovery hook
		opts := &httpCallOptions{
			catalog: c,
			gvh:     gvh,
		}
		// use same gvh for req as for hook in catalog
		req := &fakev1alpha1.FakeRequest{
			TypeMeta: metav1.TypeMeta{
				Kind:       "FakeRequest",
				APIVersion: fakev1alpha1.GroupVersion.Identifier(),
			},
		}
		resp := &fakev1alpha1.FakeResponse{}

		// create http server with hook
		mux := http.NewServeMux()
		mux.HandleFunc("/", discoveryHookHandler)
		srv := httptest.NewServer(mux)
		defer srv.Close()

		// set hook url
		opts.config.URL = pointer.String(srv.URL)

		g.Expect(httpCall(ctx, req, resp, opts)).To(Succeed())
	})
}
