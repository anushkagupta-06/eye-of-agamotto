package main

import (
	"testing"
	"time"
)

func TestExtractEdgeEvents(t *testing.T) {
	start := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)

	spans := []SpanEvent{
		{
			Service: "checkout", TraceID: "trace1", SpanID: "parent1",
			SpanKind: "SPAN_KIND_CLIENT", StartTime: start,
			DurationNs: 120_000_000,
		},
		{
			Service: "payment", TraceID: "trace1", SpanID: "child1",
			ParentSpanID: "parent1", SpanKind: "SPAN_KIND_SERVER",
			StartTime:  start.Add(10 * time.Millisecond),
			DurationNs: 80_000_000, HTTPStatusCode: 503,
		},
		{
			Service: "payment", TraceID: "trace1", SpanID: "internal1",
			ParentSpanID: "child1", SpanKind: "SPAN_KIND_SERVER",
			StartTime:  start.Add(20 * time.Millisecond),
			DurationNs: 20_000_000, HTTPStatusCode: 200,
		},
	}

	got := ExtractEdgeEvents("tenant-a", spans)

	if len(got) != 1 {
		t.Fatalf("expected 1 cross-service edge event, got %d", len(got))
	}

	event := got[0]
	if event.TenantID != "tenant-a" ||
		event.Caller != "checkout" ||
		event.Callee != "payment" ||
		event.DurationNs != 80_000_000 ||
		event.IsError != 1 ||
		!event.Timestamp.Equal(start.Add(10*time.Millisecond)) {
		t.Fatalf("unexpected edge event: %+v", event)
	}
}

func TestExtractEdgeEventsSkipsMissingParentAndEmptyTenant(t *testing.T) {
	spans := []SpanEvent{
		{
			Service: "payment", TraceID: "trace1", SpanID: "child1",
			ParentSpanID: "missing", SpanKind: "SPAN_KIND_SERVER",
			DurationNs: 10,
		},
	}

	if got := ExtractEdgeEvents("tenant-a", spans); len(got) != 0 {
		t.Fatalf("expected no event for missing parent, got %+v", got)
	}

	if got := ExtractEdgeEvents("", spans); len(got) != 0 {
		t.Fatalf("expected no events for empty tenant, got %+v", got)
	}
}
