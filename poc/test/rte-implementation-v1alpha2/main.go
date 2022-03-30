package main

import (
	"context"
	"fmt"
	"net"
	"net/http"

	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha2"
	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
	catalogHTTP "sigs.k8s.io/cluster-api/internal/runtime/server"
)

// go run rte/test/rte-implementation-v1alpha2/main.go

var c = catalog.New()

func init() {
	_ = v1alpha2.AddToCatalog(c)
}

func main() {
	ctx := ctrl.SetupSignalHandler()

	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", "8082"))
	if err != nil {
		panic(err)
	}

	fmt.Println("Server started")

	operation1Handler, err := catalogHTTP.NewHandlerBuilder().
		WithCatalog(c).
		AddService(v1alpha2.Discovery, doOperation1). // TODO: this is not strongly typed, but there are type checks when the service starts
		// TODO: test with more services
		Build()
	if err != nil {
		panic(err)
	}

	srv := &http.Server{
		Handler: operation1Handler,
	}

	go func() {
		<-ctx.Done()

		// TODO: use a context with reasonable timeout
		if err := srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout
			panic("error shutting down the HTTP server")
		}
	}()

	if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

func doOperation1(in *v1alpha2.DiscoveryHookRequest, out *v1alpha2.DiscoveryHookResponse) error {
	fmt.Println("Discovery/v1alpha2 called")
	out.Message = fmt.Sprintf("Discovery implementation version v1alpha2 - first: %d, second: %s", in.First, in.Second)
	return nil
}
