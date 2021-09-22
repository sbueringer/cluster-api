module sigs.k8s.io/cluster-api/hack/tools/linters

// Same as in golangci-lint
// Note main is using 1.16 already
go 1.15

// https://github.com/golangci/golangci-lint/issues/1276

// All overlapping dependencies must be the same as in
// go version -m ./hack/tools/bin/golangci-lint
require (
	github.com/dbraley/example-linter v0.0.0-20191111190908-06456a94dfad
	golang.org/x/tools v0.1.5
)
