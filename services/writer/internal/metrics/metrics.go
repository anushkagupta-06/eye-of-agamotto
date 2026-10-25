// Package metrics exposes Prometheus metrics for the writer and implements pipeline.Stats.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics owns a private registry.
type Metrics struct {
	reg           *prometheus.Registry
	rows          *prometheus.CounterVec
	messages      *prometheus.CounterVec
	insertSeconds *prometheus.HistogramVec
	batchRows     *prometheus.HistogramVec
}

// New creates and registers all collectors.
func New() *Metrics {
	m := &Metrics{
		reg: prometheus.NewRegistry(),
		rows: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "eoa_writer_rows_inserted_total", Help: "Rows successfully inserted.",
		}, []string{"signal"}),
		messages: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "eoa_writer_messages_total", Help: "Queue messages by outcome: ack, nak (retry) or dead (dead-lettered).",
		}, []string{"signal", "result"}),
		insertSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "eoa_writer_insert_seconds", Help: "Bulk insert latency.",
			Buckets: prometheus.ExponentialBuckets(0.005, 2, 12),
		}, []string{"signal", "result"}),
		batchRows: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "eoa_writer_batch_rows", Help: "Rows per successful insert.",
			Buckets: prometheus.ExponentialBuckets(10, 4, 8),
		}, []string{"signal"}),
	}
	m.reg.MustRegister(
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.rows, m.messages, m.insertSeconds, m.batchRows,
	)
	return m
}

// Batch implements pipeline.Stats.
func (m *Metrics) Batch(signal string, _, rows int, d time.Duration, err error) {
	result := "ok"
	if err != nil {
		result = "error"
	}
	m.insertSeconds.WithLabelValues(signal, result).Observe(d.Seconds())
	if err == nil {
		m.rows.WithLabelValues(signal).Add(float64(rows))
		m.batchRows.WithLabelValues(signal).Observe(float64(rows))
	}
}

// Messages implements pipeline.Stats.
func (m *Metrics) Messages(signal, result string, n int) {
	m.messages.WithLabelValues(signal, result).Add(float64(n))
}

// Handler serves the metrics in Prometheus text format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}
