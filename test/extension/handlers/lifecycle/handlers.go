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

// Package lifecycle contains the handlers for the lifecycle hooks.
package lifecycle

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
)

// Handler is the handler for the lifecycle hooks.
type Handler struct {
	Client client.Client
}

// DoBeforeClusterCreate implements the BeforeClusterCreate hook.
func (h *Handler) DoBeforeClusterCreate(ctx context.Context, request *runtimehooksv1.BeforeClusterCreateRequest, response *runtimehooksv1.BeforeClusterCreateResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("BeforeClusterCreate is called")

	if err := h.readResponseFromConfigMap(ctx, &request.Cluster, runtimehooksv1.BeforeClusterCreate, response); err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
		return
	}
	if err := h.recordCallInConfigMap(ctx, &request.Cluster, runtimehooksv1.BeforeClusterCreate, response); err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
	}
}

// DoBeforeClusterUpgrade implements the BeforeClusterUpgrade hook.
func (h *Handler) DoBeforeClusterUpgrade(ctx context.Context, request *runtimehooksv1.BeforeClusterUpgradeRequest, response *runtimehooksv1.BeforeClusterUpgradeResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("BeforeClusterUpgrade is called")

	if err := h.readResponseFromConfigMap(ctx, &request.Cluster, runtimehooksv1.BeforeClusterUpgrade, response); err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
		return
	}

	if err := h.recordCallInConfigMap(ctx, &request.Cluster, runtimehooksv1.BeforeClusterUpgrade, response); err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
	}
}

// DoAfterControlPlaneInitialized implements the AfterControlPlaneInitialized hook.
func (h *Handler) DoAfterControlPlaneInitialized(ctx context.Context, request *runtimehooksv1.AfterControlPlaneInitializedRequest, response *runtimehooksv1.AfterControlPlaneInitializedResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("AfterControlPlaneInitialized is called")

	if err := h.readResponseFromConfigMap(ctx, &request.Cluster, runtimehooksv1.AfterControlPlaneInitialized, response); err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
		return
	}

	if err := h.recordCallInConfigMap(ctx, &request.Cluster, runtimehooksv1.AfterControlPlaneInitialized, response); err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
	}
}

// DoAfterControlPlaneUpgrade implements the AfterControlPlaneUpgrade hook.
func (h *Handler) DoAfterControlPlaneUpgrade(ctx context.Context, request *runtimehooksv1.AfterControlPlaneUpgradeRequest, response *runtimehooksv1.AfterControlPlaneUpgradeResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("AfterControlPlaneUpgrade is called")

	if err := h.readResponseFromConfigMap(ctx, &request.Cluster, runtimehooksv1.AfterControlPlaneUpgrade, response); err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
		return
	}

	if err := h.recordCallInConfigMap(ctx, &request.Cluster, runtimehooksv1.AfterControlPlaneUpgrade, response); err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
	}
}

// DoAfterClusterUpgrade implements the AfterClusterUpgrade hook.
func (h *Handler) DoAfterClusterUpgrade(ctx context.Context, request *runtimehooksv1.AfterClusterUpgradeRequest, response *runtimehooksv1.AfterClusterUpgradeResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("AfterClusterUpgrade is called")

	if err := h.readResponseFromConfigMap(ctx, &request.Cluster, runtimehooksv1.AfterClusterUpgrade, response); err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
		return
	}

	if err := h.recordCallInConfigMap(ctx, &request.Cluster, runtimehooksv1.AfterClusterUpgrade, response); err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
	}
}

// DoBeforeClusterDelete implements the BeforeClusterDelete hook.
func (h *Handler) DoBeforeClusterDelete(ctx context.Context, request *runtimehooksv1.BeforeClusterDeleteRequest, response *runtimehooksv1.BeforeClusterDeleteResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("BeforeClusterDelete is called")

	if err := h.readResponseFromConfigMap(ctx, &request.Cluster, runtimehooksv1.BeforeClusterDelete, response); err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
		return
	}
	if err := h.recordCallInConfigMap(ctx, &request.Cluster, runtimehooksv1.BeforeClusterDelete, response); err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
	}

	// TODO: consider if to cleanup the ConfigMap after gating Cluster deletion.
}

func (h *Handler) readResponseFromConfigMap(ctx context.Context, cluster *clusterv1.Cluster, hook runtimecatalog.Hook, response runtimehooksv1.ResponseObject) error {
	hookName := runtimecatalog.HookName(hook)
	configMap := &corev1.ConfigMap{}
	configMapName := fmt.Sprintf("%s-test-extension-hookresponses", cluster.Name)
	if err := h.Client.Get(ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: configMapName}, configMap); err != nil {
		if apierrors.IsNotFound(err) {
			configMap = responsesConfigMap(cluster)
			if err := h.Client.Create(ctx, configMap); err != nil {
				return errors.Wrapf(err, "failed to create the ConfigMap %s", klog.KRef(cluster.Namespace, configMapName))
			}
		} else {
			return errors.Wrapf(err, "failed to read the ConfigMap %s", klog.KRef(cluster.Namespace, configMapName))
		}
	}
	if err := yaml.Unmarshal([]byte(configMap.Data[hookName+"-preloadedResponse"]), response); err != nil {
		return errors.Wrapf(err, "failed to read %q response information from ConfigMap", hook)
	}
	if r, ok := response.(runtimehooksv1.RetryResponseObject); ok {
		log := ctrl.LoggerFrom(ctx)
		log.Info(fmt.Sprintf("%s response is %s. retry: %v", hookName, r.GetStatus(), r.GetRetryAfterSeconds()))
	}
	return nil
}

// responsesConfigMap generates a ConfigMap with preloaded responses for the test extension.
func responsesConfigMap(cluster *clusterv1.Cluster) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-test-extension-hookresponses", cluster.Name),
			Namespace: cluster.Namespace,
		},
		// Set the initial preloadedResponses for each of the tested hooks.
		Data: map[string]string{
			// Blocking hooks are set to return RetryAfterSeconds initially. These will be changed during the test.
			"BeforeClusterCreate-preloadedResponse":      `{"Status": "Success", "RetryAfterSeconds": 0}`,
			"BeforeClusterUpgrade-preloadedResponse":     `{"Status": "Success", "RetryAfterSeconds": 0}`,
			"AfterControlPlaneUpgrade-preloadedResponse": `{"Status": "Success", "RetryAfterSeconds": 0}`,
			"BeforeClusterDelete-preloadedResponse":      `{"Status": "Success", "RetryAfterSeconds": 0}`,

			// Non-blocking hooks are set to Status:Success.
			"AfterControlPlaneInitialized-preloadedResponse": `{"Status": "Success"}`,
			"AfterClusterUpgrade-preloadedResponse":          `{"Status": "Success"}`,
		},
	}
}

func (h *Handler) recordCallInConfigMap(ctx context.Context, cluster *clusterv1.Cluster, hook runtimecatalog.Hook, response runtimehooksv1.ResponseObject) error {
	hookName := runtimecatalog.HookName(hook)
	configMap := &corev1.ConfigMap{}
	configMapName := fmt.Sprintf("%s-test-extension-hookresponses", cluster.Name)
	if err := h.Client.Get(ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: configMapName}, configMap); err != nil {
		return errors.Wrapf(err, "failed to read the ConfigMap %s", klog.KRef(cluster.Namespace, configMapName))
	}
	var patch client.Patch
	if r, ok := response.(runtimehooksv1.RetryResponseObject); ok {
		patch = client.RawPatch(types.MergePatchType,
			[]byte(fmt.Sprintf(`{"data":{"%s-actualResponseStatus": "Status: %s, RetryAfterSeconds: %v"}}`, hookName, r.GetStatus(), r.GetRetryAfterSeconds())))
	} else {
		// Patch the actualResponseStatus with the returned value
		patch = client.RawPatch(types.MergePatchType,
			[]byte(fmt.Sprintf(`{"data":{"%s-actualResponseStatus":"%s"}}`, hookName, response.GetStatus()))) //nolint:gocritic
	}
	if err := h.Client.Patch(ctx, configMap, patch); err != nil {
		return errors.Wrapf(err, "failed to update the ConfigMap %s", klog.KRef(cluster.Namespace, configMapName))
	}
	return nil
}
