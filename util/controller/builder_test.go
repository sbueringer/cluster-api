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

package controller

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/predicates"
)

func TestRateLimiter(t *testing.T) {
	rateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](5*time.Millisecond, 1000*time.Second)

	rateLimiter2 := NewTypedItemExponentialFailureRateLimiter[reconcile.Request](5*time.Millisecond, 1000*time.Second)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "default",
			Name:      "cluster",
		},
	}

	total := time.Duration(0)
	total2 := time.Duration(0)
	for i := range 100 {
		backoff := rateLimiter.When(req)
		total += backoff + 1*time.Second

		backoff2 := rateLimiter2.When(req)
		total2 += backoff2

		fmt.Printf("%3d: %s %s, %s %s\n", i+1, total, backoff+1*time.Second, total2, backoff2)
	}
}

func NewTypedItemExponentialFailureRateLimiter[T comparable](baseDelay time.Duration, maxDelay time.Duration) workqueue.TypedRateLimiter[T] {
	return &TypedItemExponentialFailureRateLimiter[T]{
		failures:  map[T]int{},
		baseDelay: baseDelay,
		maxDelay:  maxDelay,
	}
}

type TypedItemExponentialFailureRateLimiter[T comparable] struct {
	failuresLock sync.Mutex
	failures     map[T]int

	baseDelay time.Duration
	maxDelay  time.Duration
}

// Deprecated: DefaultItemBasedRateLimiter is deprecated, use DefaultTypedItemBasedRateLimiter instead.
func DefaultItemBasedRateLimiter() workqueue.RateLimiter {
	return DefaultTypedItemBasedRateLimiter[any]()
}

func DefaultTypedItemBasedRateLimiter[T comparable]() workqueue.TypedRateLimiter[T] {
	return NewTypedItemExponentialFailureRateLimiter[T](time.Millisecond, 1000*time.Second)
}

func (r *TypedItemExponentialFailureRateLimiter[T]) When(item T) time.Duration {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()

	r.failures[item] = r.failures[item] + 1

	failures := r.failures[item]
	switch {
	case failures >= 1 && failures <= 5:
		return 1 * time.Second
	case failures >= 6 && failures <= 10:
		return 5 * time.Second
	case failures >= 11 && failures <= 15:
		return 10 * time.Second
	default:
		return 10 * time.Minute
	}
}

func (r *TypedItemExponentialFailureRateLimiter[T]) NumRequeues(item T) int {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()

	return r.failures[item]
}

func (r *TypedItemExponentialFailureRateLimiter[T]) Forget(item T) {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()

	delete(r.failures, item)
}

func TestBuilder(t *testing.T) {
	g := NewWithT(t)

	predicateLog := ctrl.LoggerFrom(t.Context()).WithValues("controller", "test/cluster")

	scheme := runtime.NewScheme()
	g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())

	realManager, err := manager.New(&rest.Config{}, manager.Options{})
	g.Expect(err).ToNot(HaveOccurred())
	fakeSink := &fakeSink{}
	fakeManager := &fakeManager{
		Manager: realManager,
		Scheme:  scheme,
		Sink:    fakeSink,
	}

	fakeSource := source.Channel(nil, handler.EnqueueRequestsFromMapFunc(nil))

	reconciler := reconcile.Func(func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
		ctrl.LoggerFrom(ctx).Info("Reconciling", "request", req)
		return reconcile.Result{}, nil
	})

	c, err := NewControllerManagedBy(fakeManager, predicateLog).
		For(&clusterv1.Cluster{}).
		Named("test/cluster").
		WatchesRawSource(fakeSource).
		Watches(&clusterv1.ClusterClass{}, handler.EnqueueRequestsFromMapFunc(nil)).
		Watches(&clusterv1.MachineDeployment{}, handler.EnqueueRequestsFromMapFunc(nil), predicates.ResourceIsTopologyOwned(fakeManager.GetScheme(), predicateLog)).
		Watches(&clusterv1.MachinePool{}, handler.EnqueueRequestsFromMapFunc(nil)).
		WithOptions(controller.Options{}).
		WithEventFilter(predicates.ResourceHasFilterLabel(fakeManager.GetScheme(), predicateLog, "labelValue")).
		Build(reconciler)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(c).ToNot(BeNil())

	typedController := (c.(*controllerWrapper)).TypedController

	// Verify ReconciliationTimeout.
	var timeDurationType time.Duration
	reconciliationTimeout := reflect.NewAt(
		reflect.TypeOf(timeDurationType),
		unsafe.Pointer(reflect.ValueOf(typedController).Elem().FieldByName("ReconciliationTimeout").UnsafeAddr()), //nolint:gosec // Using unsafe here is ~ fine.
	).Elem().Interface().(time.Duration)
	g.Expect(reconciliationTimeout).To(Equal(defaultReconciliationTimeout))

	// Verify LogConstructor.
	g.Expect(fakeSink.keysAndValues).To(Equal([]string{
		"controller", "test/cluster",
		"controllerGroup", "cluster.x-k8s.io",
		"controllerKind", "Cluster",
	}))
}

type fakeManager struct {
	manager.Manager
	Scheme *runtime.Scheme
	Sink   *fakeSink
}

func (m *fakeManager) Add(_ manager.Runnable) error {
	return nil
}

func (m *fakeManager) GetScheme() *runtime.Scheme {
	return m.Scheme
}

func (m *fakeManager) GetLogger() logr.Logger {
	return logr.New(m.Sink)
}

func (m *fakeManager) GetControllerOptions() config.Controller {
	return config.Controller{}
}

type fakeSink struct {
	logr.LogSink
	keysAndValues []string
}

func (s *fakeSink) Init(_ logr.RuntimeInfo) {
}

func (s *fakeSink) WithValues(keysAndValues ...any) logr.LogSink {
	for _, item := range keysAndValues {
		s.keysAndValues = append(s.keysAndValues, item.(string))
	}
	return s
}
