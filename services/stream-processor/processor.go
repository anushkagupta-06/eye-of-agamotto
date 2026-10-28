package main

import (
	"sort"
	"time"
)

// TelemetryEvent represents one recorded request.
type TelemetryEvent struct {
	Service    string    `json:"service"`
	StatusCode int       `json:"status_code"`
	DurationMs int       `json:"duration_ms"`
	Timestamp  time.Time `json:"timestamp"`
}

// ServiceMetrics stores aggregated metrics for one service.
type ServiceMetrics struct {
	TotalRequests int
	ServerErrors  int
	TotalDuration int
}

// ProcessEvents calculates metrics separately for each service.
func ProcessEvents(events []TelemetryEvent) map[string]*ServiceMetrics {
	metrics := make(map[string]*ServiceMetrics)

	for _, event := range events {
		// Ignore invalid telemetry.
		if event.Service == "" ||
			event.StatusCode < 100 ||
			event.StatusCode > 599 ||
			event.DurationMs < 0 {
			continue
		}

		// Create a metrics entry when a service appears for the first time.
		if metrics[event.Service] == nil {
			metrics[event.Service] = &ServiceMetrics{}
		}

		m := metrics[event.Service]

		m.TotalRequests++
		m.TotalDuration += event.DurationMs

		if event.StatusCode >= 500 {
			m.ServerErrors++
		}
	}

	return metrics
}

func FilterEventsByWindow(
	events []TelemetryEvent,
	start time.Time,
	window time.Duration,
) []TelemetryEvent {
	var filtered []TelemetryEvent
	end := start.Add(window)

	for _, event := range events {
		// Include start; exclude end.
		if !event.Timestamp.Before(start) &&
			event.Timestamp.Before(end) {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

type SpanMetrics struct {
	TotalRequests   int
	ServerErrors    int
	TotalDurationNs int64
}

// ProcessSpans calculates request metrics from server spans only.
func ProcessSpans(spans []SpanEvent) map[string]*SpanMetrics {
	metrics := make(map[string]*SpanMetrics)

	for _, span := range spans {
		// Only server spans represent incoming server-side requests.
		if span.SpanKind != "SPAN_KIND_SERVER" {
			continue
		}

		if span.Service == "" ||
			span.DurationNs < 0 ||
			span.HTTPStatusCode < 0 ||
			span.HTTPStatusCode > 599 {
			continue
		}

		if metrics[span.Service] == nil {
			metrics[span.Service] = &SpanMetrics{}
		}

		m := metrics[span.Service]
		m.TotalRequests++
		m.TotalDurationNs += span.DurationNs

		if span.HTTPStatusCode >= 500 {
			m.ServerErrors++
		}
	}

	return metrics
}

// AverageDurationMs converts the average span duration to milliseconds.
func (m *SpanMetrics) AverageDurationMs() float64 {
	if m.TotalRequests == 0 {
		return 0
	}

	return float64(m.TotalDurationNs) /
		float64(m.TotalRequests) / 1_000_000
}

// ServerErrorRate returns the percentage of requests with HTTP 5xx status.
func (m *SpanMetrics) ServerErrorRate() float64 {
	if m.TotalRequests == 0 {
		return 0
	}

	return float64(m.ServerErrors) / float64(m.TotalRequests) * 100
}

// ServiceNames returns sorted service names for deterministic output.
func ServiceNames(metrics map[string]*SpanMetrics) []string {
	names := make([]string, 0, len(metrics))
	for name := range metrics {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}
