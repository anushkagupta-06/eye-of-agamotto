package main

import "testing"

func TestProcessSpans(t *testing.T) {
	spans := []SpanEvent{
		{
			Service:        "order-service",
			SpanKind:       "SPAN_KIND_SERVER",
			HTTPStatusCode: 200,
			DurationNs:     100_000_000,
		},
		{
			Service:        "order-service",
			SpanKind:       "SPAN_KIND_SERVER",
			HTTPStatusCode: 500,
			DurationNs:     300_000_000,
		},
		{
			// Client spans must not be counted as incoming requests.
			Service:        "order-service",
			SpanKind:       "SPAN_KIND_CLIENT",
			HTTPStatusCode: 500,
			DurationNs:     900_000_000,
		},
	}

	metrics := ProcessSpans(spans)
	got := metrics["order-service"]

	if got == nil {
		t.Fatal("expected order-service metrics")
	}

	if got.TotalRequests != 2 {
		t.Errorf("expected 2 requests, got %d", got.TotalRequests)
	}

	if got.ServerErrors != 1 {
		t.Errorf("expected 1 server error, got %d", got.ServerErrors)
	}

	if got.TotalDurationNs != 400_000_000 {
		t.Errorf("expected 400000000 ns total duration, got %d",
			got.TotalDurationNs)
	}

	if got.AverageDurationMs() != 200 {
		t.Errorf("expected 200 ms average duration, got %f",
			got.AverageDurationMs())
	}

	if got.ServerErrorRate() != 50 {
		t.Errorf("expected 50%% error rate, got %f",
			got.ServerErrorRate())
	}
}
