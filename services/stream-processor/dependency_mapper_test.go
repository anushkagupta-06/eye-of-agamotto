package main

import "testing"

func TestExtractServiceEdges(t *testing.T) {
	spans := []SpanEvent{
		{
			Service:    "order-service",
			TraceID:    "trace-1",
			SpanID:     "client-1",
			SpanKind:   "SPAN_KIND_CLIENT",
			DurationNs: 200_000_000,
		},
		{
			Service:        "payment-service",
			TraceID:        "trace-1",
			SpanID:         "server-1",
			ParentSpanID:   "client-1",
			SpanKind:       "SPAN_KIND_SERVER",
			HTTPStatusCode: 500,
			DurationNs:     150_000_000,
		},
		{
			Service:        "order-service",
			TraceID:        "trace-2",
			SpanID:         "client-2",
			SpanKind:       "SPAN_KIND_CLIENT",
			HTTPStatusCode: 200,
			DurationNs:     100_000_000,
		},
		{
			Service:        "payment-service",
			TraceID:        "trace-2",
			SpanID:         "server-2",
			ParentSpanID:   "client-2",
			SpanKind:       "SPAN_KIND_SERVER",
			HTTPStatusCode: 200,
			DurationNs:     80_000_000,
		},
	}

	edges := ExtractServiceEdges(spans)
	got := edges["order-service->payment-service"]

	if got == nil {
		t.Fatal("expected order-service -> payment-service edge")
	}

	if got.TotalCalls != 2 {
		t.Errorf("expected 2 calls, got %d", got.TotalCalls)
	}

	if got.Errors != 1 {
		t.Errorf("expected 1 error, got %d", got.Errors)
	}

	if got.TotalDurationNs != 300_000_000 {
		t.Errorf("expected 300000000 ns total duration, got %d",
			got.TotalDurationNs)
	}
}
