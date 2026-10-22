package main

import (
	"testing"
	"time"
)

func TestProcessSpanWindow(t *testing.T) {
	start := time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)

	spans := []SpanEvent{
		{
			Service:        "order-service",
			SpanKind:       "SPAN_KIND_SERVER",
			StartTime:      start,
			DurationNs:     100_000_000,
			HTTPStatusCode: 200,
		},
		{
			Service:        "order-service",
			SpanKind:       "SPAN_KIND_SERVER",
			StartTime:      start.Add(2 * time.Second),
			DurationNs:     200_000_000,
			HTTPStatusCode: 200,
		},
		{
			Service:        "order-service",
			SpanKind:       "SPAN_KIND_SERVER",
			StartTime:      start.Add(4 * time.Second),
			DurationNs:     300_000_000,
			HTTPStatusCode: 500,
		},
		{
			Service:        "order-service",
			SpanKind:       "SPAN_KIND_SERVER",
			StartTime:      start.Add(6 * time.Second),
			DurationNs:     900_000_000,
			HTTPStatusCode: 200,
		},
		{
			// Outgoing calls must not count as incoming requests.
			Service:        "order-service",
			SpanKind:       "SPAN_KIND_CLIENT",
			StartTime:      start.Add(3 * time.Second),
			DurationNs:     800_000_000,
			HTTPStatusCode: 500,
		},
		{
			// The end of the window is exclusive.
			Service:        "order-service",
			SpanKind:       "SPAN_KIND_SERVER",
			StartTime:      start.Add(10 * time.Second),
			DurationNs:     500_000_000,
			HTTPStatusCode: 500,
		},
	}

	metrics := ProcessSpanWindow(spans, start, 10*time.Second)
	got := metrics["order-service"]

	if got == nil {
		t.Fatal("expected order-service metrics")
	}

	if got.TotalRequests != 4 {
		t.Errorf("expected 4 requests, got %d", got.TotalRequests)
	}
	if got.Errors != 1 {
		t.Errorf("expected 1 error, got %d", got.Errors)
	}
	if got.RequestRate != 0.4 {
		t.Errorf("expected rate 0.4, got %f", got.RequestRate)
	}
	if got.ErrorRate != 25 {
		t.Errorf("expected error rate 25%%, got %f", got.ErrorRate)
	}
	if got.AverageDurationMs != 375 {
		t.Errorf("expected average 375 ms, got %f", got.AverageDurationMs)
	}
	if got.P50Ms != 200 {
		t.Errorf("expected p50 200 ms, got %f", got.P50Ms)
	}
	if got.P95Ms != 900 {
		t.Errorf("expected p95 900 ms, got %f", got.P95Ms)
	}
	if got.P99Ms != 900 {
		t.Errorf("expected p99 900 ms, got %f", got.P99Ms)
	}
}
