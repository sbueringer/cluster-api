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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"sigs.k8s.io/cluster-api/util/cache"
	predicatesutil "sigs.k8s.io/cluster-api/util/predicates"
)

// Builder is a wrapper around controller-runtime's builder.Builder.
type Builder struct {
	builder        *builder.Builder
	mgr            manager.Manager
	predicateLog   logr.Logger
	options        controller.TypedOptions[reconcile.Request]
	forObject      client.Object
	controllerName string
}

func ControllerManagedBy(m manager.Manager, predicateLog logr.Logger) *Builder {
	return &Builder{
		builder:      builder.ControllerManagedBy(m),
		mgr:          m,
		predicateLog: predicateLog,
	}
}

func (blder *Builder) For(object client.Object, opts ...builder.ForOption) *Builder {
	blder.forObject = object
	blder.builder.For(object, opts...)
	return blder
}

func (blder *Builder) Owns(object client.Object, predicates ...predicate.Predicate) *Builder {
	predicates = append(predicates, predicatesutil.ResourceIsChanged(blder.mgr.GetScheme(), blder.predicateLog))
	blder.builder.Owns(object, builder.WithPredicates(predicates...))
	return blder
}

func (blder *Builder) Watches(object client.Object, eventHandler handler.TypedEventHandler[client.Object, reconcile.Request], predicates ...predicate.Predicate) *Builder {
	predicates = append(predicates, predicatesutil.ResourceIsChanged(blder.mgr.GetScheme(), blder.predicateLog))
	blder.builder.Watches(object, eventHandler, builder.WithPredicates(predicates...))
	return blder
}

func (blder *Builder) WatchesRawSource(src source.TypedSource[reconcile.Request]) *Builder {
	blder.builder.WatchesRawSource(src)
	return blder
}

func (blder *Builder) WithOptions(options controller.TypedOptions[reconcile.Request]) *Builder {
	blder.options = options
	return blder
}

func (blder *Builder) WithEventFilter(p predicate.Predicate) *Builder {
	blder.builder.WithEventFilter(p)
	return blder
}

func (blder *Builder) Named(name string) *Builder {
	blder.controllerName = name
	blder.builder.Named(name)
	return blder
}

func (blder *Builder) Build(r reconcile.TypedReconciler[reconcile.Request]) (cache.Cache[cache.ReconcileEntry], controller.TypedController[reconcile.Request], error) {
	// Get GVK of the for object.
	var gvk schema.GroupVersionKind
	hasGVK := blder.forObject != nil
	if hasGVK {
		var err error
		gvk, err = apiutil.GVKForObject(blder.forObject, blder.mgr.GetScheme())
		if err != nil {
			return nil, nil, err
		}
	}

	// Get controllerName.
	controllerName := blder.controllerName
	if controllerName == "" {
		controllerName = strings.ToLower(gvk.Kind)
	}

	// Default ReconciliationTimeout.
	if blder.options.ReconciliationTimeout == 0 {
		blder.options.ReconciliationTimeout = 30 * time.Second
	}

	// Default LogConstructor.
	if blder.options.LogConstructor == nil {
		// Build and set LogConstructor.
		log := blder.mgr.GetLogger().WithValues(
			"controller", controllerName,
		)
		if hasGVK {
			log = log.WithValues(
				"controllerGroup", gvk.Group,
				"controllerKind", gvk.Kind,
			)
		}
		blder.options.LogConstructor = func(req *reconcile.Request) logr.Logger {
			// Note: This logic has to be inside the LogConstructor as this k/v pair
			// is different for every single reconcile.Request
			log := log
			if req != nil && hasGVK {
				log = log.WithValues(gvk.Kind, klog.KRef(req.Namespace, req.Name))
				// Note: Not setting additional name & namespace k/v pairs like controller-runtime
				// as they are redundant.
			}
			return log
		}
	}

	// Create reconcileCache.
	reconcileCache := cache.New[cache.ReconcileEntry](cache.DefaultTTL)

	c, err := blder.builder.Build(reconciler{reconciler: r, reconcileCache: reconcileCache})
	return reconcileCache, c, err
}
