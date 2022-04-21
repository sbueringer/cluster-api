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
	hooksv1alpha1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	hooksv1alpha3 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha3"
	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
)

func TestClient_httpCall(t *testing.T) {
	g := NewWithT(t)

	ctx := context.TODO()

	discoveryHookHandler := func(w http.ResponseWriter, r *http.Request) {
		response := &hooksv1alpha1.DiscoveryHookResponse{
			TypeMeta: metav1.TypeMeta{
				Kind:       "DiscoveryHookResponse",
				APIVersion: hooksv1alpha1.GroupVersion.Identifier(),
			},
			Status: hooksv1alpha1.ResponseStatusSuccess,
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
		g.Expect(httpCall(ctx, &hooksv1alpha1.DiscoveryHookRequest{}, &hooksv1alpha1.DiscoveryHookResponse{}, &httpCallOptions{})).To(HaveOccurred())
	})
	t.Run("pass empty catalog", func(t *testing.T) {
		opts := &httpCallOptions{
			catalog: catalog.New(),
		}
		g.Expect(httpCall(ctx, &hooksv1alpha1.DiscoveryHookRequest{}, &hooksv1alpha1.DiscoveryHookResponse{}, opts)).To(HaveOccurred())
	})
	t.Run("ok, no conversion, discovery hook", func(t *testing.T) {
		// create catalog containing a DiscoveryHook
		c := catalog.New()
		hookFunc := func(req *hooksv1alpha1.DiscoveryHookRequest, resp *hooksv1alpha1.DiscoveryHookResponse) {}
		c.AddHook(
			hooksv1alpha1.GroupVersion,
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
		req := &hooksv1alpha1.DiscoveryHookRequest{
			TypeMeta: metav1.TypeMeta{
				Kind:       "DiscoveryHookRequest",
				APIVersion: hooksv1alpha1.GroupVersion.Identifier(),
			},
		}
		resp := &hooksv1alpha1.DiscoveryHookResponse{}

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
		// register hooksv1alpha1 to enable conversion
		g.Expect(hooksv1alpha1.AddToCatalog(c)).To(Succeed())
		hookFunc := func(req *hooksv1alpha1.DiscoveryHookRequest, resp *hooksv1alpha1.DiscoveryHookResponse) {}
		c.AddHook(
			hooksv1alpha1.GroupVersion,
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
		req := &hooksv1alpha3.DiscoveryHookRequest{
			TypeMeta: metav1.TypeMeta{
				Kind:       "DiscoveryHookRequest",
				APIVersion: hooksv1alpha3.GroupVersion.Identifier(),
			},
		}
		resp := &hooksv1alpha3.DiscoveryHookResponse{}

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
