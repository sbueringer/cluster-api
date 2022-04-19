package main

import (
	"flag"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	runtimev1 "sigs.k8s.io/cluster-api/exp/runtime/api/v1beta1"
	"sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
	catalogHTTP "sigs.k8s.io/cluster-api/internal/runtime/server"
)

// go run rte/test/rte-implementation-v1alpha2/main.go

var c = catalog.New()
var certDir = flag.String("certDir", "/tmp/rte-implementation-secure/", "path to directory containing tls.crt and tls.key")

func init() {
	_ = v1alpha1.AddToCatalog(c)
}

func main() {
	ctx := ctrl.SetupSignalHandler()

	srv := webhook.Server{
		Host:          "127.0.0.1",
		Port:          8083,
		CertDir:       *certDir,
		CertName:      "tls.crt",
		KeyName:       "tls.key",
		WebhookMux:    http.NewServeMux(),
		TLSMinVersion: "1.2",
	}

	operation1Handler, err := catalogHTTP.NewHandlerBuilder().
		WithCatalog(c).
		AddDiscovery(v1alpha1.Discovery, doDiscovery). // TODO: this is not strongly typed, but there are type checks when the service starts
		AddExtension(v1alpha1.BeforeClusterUpgrade, "install-metrics-database", doInstallMetricsDatabase).
		// TODO: test with more services
		Build()
	if err != nil {
		panic(err)
	}

	srv.WebhookMux.Handle("/", operation1Handler)

	if err := srv.StartStandalone(ctx, nil); err != nil {
		panic(err)
	}
}

// TODO: consider registering extensions with all required data and then auto-generating the discovery func based on that.
// If we want folks to write it manually, make it nicer to do.
func doDiscovery(request *v1alpha1.DiscoveryHookRequest, response *v1alpha1.DiscoveryHookResponse) error {
	fmt.Println("Discovery/v1alpha1 called")

	response.Status = v1alpha1.ResponseStatusSuccess
	response.Extensions = append(response.Extensions, runtimev1.RuntimeExtension{
		Name: "install-metrics-database",
		Hook: runtimev1.Hook{
			APIVersion: v1alpha1.GroupVersion.String(),
			Name:       "BeforeClusterUpgrade",
		},
		TimeoutSeconds: pointer.Int32(10),
		FailurePolicy:  toPtr(runtimev1.FailurePolicyFail),
	})

	return nil
}

func doInstallMetricsDatabase(request *v1alpha1.BeforeClusterUpgradeRequest, response *v1alpha1.BlockingResponse) error {
	fmt.Println("BeforeClusterUpgrade/v1alpha1 called", "cluster", klog.KObj(&request.Cluster))

	response.Status = v1alpha1.ResponseStatusSuccess
	response.RetryAfterSeconds = 10

	return nil
}

func toPtr(f runtimev1.FailurePolicy) *runtimev1.FailurePolicy {
	return &f
}
