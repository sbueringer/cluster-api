package client

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func init() {
	// Register the metrics at the controller-runtime registry metrics registry
	// to expose them.
	ctrlmetrics.Registry.MustRegister(RequestsTotal.metric)
	ctrlmetrics.Registry.MustRegister(RequestLatency.metric)
}

// Metrics subsystem and all of the keys used by the runtime sdk.
const (
	runtimeSDKSubsystem = "runtime_sdk"
	requestLatencyKey   = "request_latency_seconds"
	requestsTotalKey    = "requests_total"
)

var (
	// RequestsTotal reports request result by return code, method, host and hook.
	RequestsTotal = requestsTotalObserver{
		prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: runtimeSDKSubsystem,
			Name:      requestsTotalKey,
			Help:      "Number of HTTP requests, partitioned by status code, method, and host.",
		}, []string{"code", "method", "host", "hook"}),
	}
	// RequestLatency reports the request latency in seconds per hook and host.
	RequestLatency = latencyMetricObserver{
		prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: runtimeSDKSubsystem,
			Name:      requestLatencyKey,
			Help:      "Request latency in seconds. Broken down by verb and host.",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		}, []string{"hook", "host"}),
	}
)

type requestsTotalObserver struct {
	metric *prometheus.CounterVec
}

type latencyMetricObserver struct {
	metric *prometheus.HistogramVec
}

func (m *requestsTotalObserver) increment(code, method, host, hook string) {
	m.metric.WithLabelValues(code, method, host, hook).Inc()
}

// Observe observes a http request result and increments the metric for the given
// error code, mehtod, host and hook.
func (m *requestsTotalObserver) Observe(req *http.Request, resp *http.Response, hook string, err error) {
	host := "none"
	if req.URL != nil {
		host = req.URL.Host
	}

	// Errors can be arbitrary strings. Unbound label cardinality is not suitable for a metric
	// system so they are reported as `<error>`.
	if err != nil {
		m.increment("<error>", req.Method, host, hook)
	} else {
		// Metrics for failure codes.
		m.increment(strconv.Itoa(resp.StatusCode), req.Method, host, hook)
	}
}

// Observe increments the request latency metric for the given verb and host.
func (m *latencyMetricObserver) Observe(hook string, u url.URL, latency time.Duration) {
	m.metric.WithLabelValues(hook, u.Host).Observe(latency.Seconds())
}
