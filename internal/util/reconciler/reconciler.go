/*
Copyright 2025 The Kubernetes Authors.

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

// Package reconciler provides utils for usage with the controller-runtime client.
package reconciler

import (
	"context"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/cluster-api/util/cache"
)

type reconciler struct {
	name           string
	reconcileCache cache.Cache[cache.ReconcileEntry]
	reconciler     reconcile.Reconciler
}

func (r reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	reconcileStartTime := time.Now()

	// Check reconcileCache to ensure we won't run reconcile too frequently.
	if cacheEntry, ok := r.reconcileCache.Has(cache.NewReconcileEntryKey(req)); ok {
		if requeueAfter, requeue := cacheEntry.ShouldRequeue(reconcileStartTime); requeue {
			return ctrl.Result{RequeueAfter: requeueAfter}, nil
		}
	}

	// Add entry to the reconcileCache so we won't run Reconcile more than once per second.
	// Under certain circumstances the ReconcileAfter time will be set to a later time, e.g. when we're waiting
	// for Pods to terminate or volumes to detach.
	// This is done to ensure we're not spamming the workload cluster API server.
	// Note: This is overwritten for example for drain or wait for volume detach.
	r.reconcileCache.Add(cache.NewReconcileEntry(req, reconcileStartTime.Add(1*time.Second)))

	// Update metrics after processing each item
	defer func() {
		reconcileTime.WithLabelValues(r.name).Observe(time.Since(reconcileStartTime).Seconds())
	}()

	result, err := r.reconciler.Reconcile(ctx, req)
	if err != nil {
		// Note: controller-runtime logs a warning if an error is returned in combination with
		// RequeueAfter / Requeue. Dropping RequeueAfter and Requeue here to avoid this warning
		// (while preserving Priority).
		result.RequeueAfter = 0
		result.Requeue = false //nolint:staticcheck // We have to handle Requeue until it is removed
	}
	switch {
	case err != nil:
		reconcileTotal.WithLabelValues(r.name, labelError).Inc()
	case result.RequeueAfter > 0:
		reconcileTotal.WithLabelValues(r.name, labelRequeueAfter).Inc()
	case result.Requeue: //nolint: staticcheck // We have to handle Requeue until it is removed
		reconcileTotal.WithLabelValues(r.name, labelRequeue).Inc()
	default:
		reconcileTotal.WithLabelValues(r.name, labelSuccess).Inc()
	}

	return result, err
}
