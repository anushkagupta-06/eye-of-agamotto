package main

import (
	"math"
	"sort"
	"time"
)

// REDMetrics stores metrics for one service in one time window.
type REDMetrics struct {
	Service           string
	TotalRequests     int
	Errors            int
	TotalDurationNs   int64
	RequestRate       float64
	ErrorRate         float64
	AverageDurationMs float64
	P50Ms             float64
	P95Ms             float64
	P99Ms             float64
}

// ProcessSpanWindow calculates RED metrics for each service
// using server spans whose start times fall inside the window.
func ProcessSpanWindow(
	spans []SpanEvent,
	start time.Time,
	window time.Duration,
) map[string]*REDMetrics {
	metrics := make(map[string]*REDMetrics)

	// A non-positive window cannot produce a meaningful rate.
	if window <= 0 {
		return metrics
	}

	end := start.Add(window)

	// Keep individual durations to calculate percentiles later.
	durations := make(map[string][]int64)

	for _, span := range spans {
		// Count incoming requests, not outgoing client calls.
		if span.SpanKind != "SPAN_KIND_SERVER" {
			continue
		}

		if span.Service == "" || span.DurationNs < 0 {
			continue
		}

		// Window start is inclusive; window end is exclusive.
		if span.StartTime.Before(start) || !span.StartTime.Before(end) {
			continue
		}

		if metrics[span.Service] == nil {
			metrics[span.Service] = &REDMetrics{
				Service: span.Service,
			}
		}

		m := metrics[span.Service]
		m.TotalRequests++
		m.TotalDurationNs += span.DurationNs
		durations[span.Service] = append(
			durations[span.Service],
			span.DurationNs,
		)

		if span.HTTPStatusCode >= 500 &&
			span.HTTPStatusCode <= 599 {
			m.Errors++
		}
	}

	windowSeconds := window.Seconds()

	for service, m := range metrics {
		m.RequestRate = float64(m.TotalRequests) / windowSeconds
		m.ErrorRate = float64(m.Errors) /
			float64(m.TotalRequests) * 100

		m.AverageDurationMs = float64(m.TotalDurationNs) /
			float64(m.TotalRequests) / 1_000_000

		sort.Slice(durations[service], func(i, j int) bool {
			return durations[service][i] < durations[service][j]
		})

		m.P50Ms = percentileMs(durations[service], 0.50)
		m.P95Ms = percentileMs(durations[service], 0.95)
		m.P99Ms = percentileMs(durations[service], 0.99)
	}

	return metrics
}

// percentileMs uses the nearest-rank method.
func percentileMs(durations []int64, percentile float64) float64 {
	if len(durations) == 0 {
		return 0
	}

	index := int(math.Ceil(percentile*float64(len(durations)))) - 1

	if index < 0 {
		index = 0
	}

	return float64(durations[index]) / 1_000_000
}
