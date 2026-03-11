package admin

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus counters and histograms for the S3 API.
type Metrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	BytesIn         prometheus.Counter
	BytesOut        prometheus.Counter
}

// NewMetrics registers and returns Prometheus metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		RequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "sangraha_requests_total",
			Help: "Total number of S3 API requests.",
		}, []string{"method", "operation", "status_code"}),

		RequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sangraha_request_duration_seconds",
			Help:    "Histogram of S3 API request durations.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "operation"}),

		BytesIn: promauto.NewCounter(prometheus.CounterOpts{
			Name: "sangraha_bytes_received_total",
			Help: "Total bytes received (PUT/POST bodies).",
		}),

		BytesOut: promauto.NewCounter(prometheus.CounterOpts{
			Name: "sangraha_bytes_sent_total",
			Help: "Total bytes sent (GET response bodies).",
		}),
	}
}

// Handler returns the Prometheus HTTP handler for /admin/v1/metrics.
func metricsHandler() http.Handler {
	return promhttp.Handler()
}
