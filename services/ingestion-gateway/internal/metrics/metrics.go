// Package metrics exposes Prometheus metrics for the gateway.
package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/batcher"
)

// Metrics owns a private registry so tests and multiple instances never collide.
type Metrics struct {
	reg            *prometheus.Registry
	requests       *prometheus.CounterVec
	dropped        *prometheus.CounterVec
	batchBytes     *prometheus.HistogramVec
	batchItems     *prometheus.HistogramVec
	publishSeconds *prometheus.HistogramVec
}

// New creates and registers all collectors.
func New() *Metrics {
	m := &Metrics{
		reg: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "eoa_gateway_requests_total", Help: "Ingest requests by transport, signal and result code.",
		}, []string{"transport", "signal", "code"}),
		dropped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "eoa_gateway_dropped_items_total", Help: "Spans or log records dropped by validation.",
		}, []string{"signal"}),
		batchBytes: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "eoa_gateway_batch_bytes", Help: "Size of published queue messages.",
			Buckets: prometheus.ExponentialBuckets(1024, 4, 8),
		}, []string{"subject"}),
		batchItems: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "eoa_gateway_batch_requests", Help: "Client requests merged per queue message.",
			Buckets: prometheus.ExponentialBuckets(1, 2, 10),
		}, []string{"subject"}),
		publishSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "eoa_gateway_publish_seconds", Help: "Queue publish latency.",
			Buckets: prometheus.ExponentialBuckets(0.0005, 2, 14),
		}, []string{"subject", "result"}),
	}
	m.reg.MustRegister(
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.requests, m.dropped, m.batchBytes, m.batchItems, m.publishSeconds,
	)
	return m
}

// Request counts one finished ingest request.
func (m *Metrics) Request(transport, signal string, code int) {
	m.requests.WithLabelValues(transport, signal, strconv.Itoa(code)).Inc()
}

// Dropped counts items removed by validation.
func (m *Metrics) Dropped(signal string, n int64) { m.dropped.WithLabelValues(signal).Add(float64(n)) }

// BatchObserver feeds batcher results into the histograms.
func (m *Metrics) BatchObserver() batcher.Observer {
	return func(r batcher.Result) {
		result := "ok"
		if r.Err != nil {
			result = "error"
		}
		m.publishSeconds.WithLabelValues(r.Subject, result).Observe(r.Duration.Seconds())
		if r.Err == nil {
			m.batchBytes.WithLabelValues(r.Subject).Observe(float64(len(r.Payload)))
			m.batchItems.WithLabelValues(r.Subject).Observe(float64(r.Items))
		}
	}
}

// Inflight registers a gauge reporting bytes admitted but not yet acknowledged by the queue.
func (m *Metrics) Inflight(subject string, fn func() int64) {
	m.reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "eoa_gateway_inflight_bytes", Help: "Bytes waiting to be published.",
		ConstLabels: prometheus.Labels{"subject": subject},
	}, func() float64 { return float64(fn()) }))
}

// Handler serves the metrics in Prometheus text format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}
