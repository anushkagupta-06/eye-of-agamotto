package main

import (
	"testing"
	"time"
)

func TestEdgeCorrelator_SameMessageSpans(t *testing.T) {
	c := NewEdgeCorrelator(time.Minute)
	now := time.Now()

	parent := SpanEvent{
		Service:   "frontend",
		TraceID:   "t1",
		SpanID:    "s1",
		SpanName:  "GET /checkout",
		SpanKind:  "SPAN_KIND_CLIENT",
		StartTime: now,
	}
	child := SpanEvent{
		Service:      "payment",
		TraceID:      "t1",
		SpanID:       "s2",
		ParentSpanID: "s1",
		SpanName:     "POST /charge",
		SpanKind:     "SPAN_KIND_SERVER",
		StartTime:    now.Add(10 * time.Millisecond),
		DurationNs:   50000000, // 50ms
	}

	edges := c.Correlate("demo", []SpanEvent{parent, child})
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	edge := edges[0]
	if edge.TenantID != "demo" || edge.Caller != "frontend" || edge.Callee != "payment" {
		t.Errorf("unexpected edge: %+v", edge)
	}
	if edge.DurationNs != 50000000 {
		t.Errorf("expected 50ms duration, got %d", edge.DurationNs)
	}
}

func TestEdgeCorrelator_SplitMessages_ParentFirst(t *testing.T) {
	c := NewEdgeCorrelator(time.Minute)
	now := time.Now()

	parent := SpanEvent{
		Service:   "api-gateway",
		TraceID:   "t2",
		SpanID:    "s10",
		SpanName:  "HTTP GET",
		SpanKind:  "SPAN_KIND_CLIENT",
		StartTime: now,
	}
	child := SpanEvent{
		Service:      "auth-service",
		TraceID:      "t2",
		SpanID:       "s11",
		ParentSpanID: "s10",
		SpanName:     "Authenticate",
		SpanKind:     "SPAN_KIND_SERVER",
		StartTime:    now.Add(5 * time.Millisecond),
		DurationNs:   20000000,
	}

	// Message 1 contains parent
	edges1 := c.Correlate("demo", []SpanEvent{parent})
	if len(edges1) != 0 {
		t.Fatalf("expected 0 edges for parent only, got %d", len(edges1))
	}

	// Message 2 contains child
	edges2 := c.Correlate("demo", []SpanEvent{child})
	if len(edges2) != 1 {
		t.Fatalf("expected 1 edge when child arrives, got %d", len(edges2))
	}
	if edges2[0].Caller != "api-gateway" || edges2[0].Callee != "auth-service" {
		t.Errorf("unexpected edge: %+v", edges2[0])
	}
}

func TestEdgeCorrelator_SplitMessages_ChildFirst(t *testing.T) {
	c := NewEdgeCorrelator(time.Minute)
	now := time.Now()

	parent := SpanEvent{
		Service:   "order-service",
		TraceID:   "t3",
		SpanID:    "s20",
		SpanName:  "CreateOrder",
		SpanKind:  "SPAN_KIND_CLIENT",
		StartTime: now,
	}
	child := SpanEvent{
		Service:      "inventory-service",
		TraceID:      "t3",
		SpanID:       "s21",
		ParentSpanID: "s20",
		SpanName:     "ReserveStock",
		SpanKind:     "SPAN_KIND_SERVER",
		StartTime:    now.Add(15 * time.Millisecond),
		DurationNs:   30000000,
	}

	// Message 1 arrives with child first
	edges1 := c.Correlate("demo", []SpanEvent{child})
	if len(edges1) != 0 {
		t.Fatalf("expected 0 edges before parent arrives, got %d", len(edges1))
	}

	// Message 2 arrives later with parent
	edges2 := c.Correlate("demo", []SpanEvent{parent})
	if len(edges2) != 1 {
		t.Fatalf("expected 1 edge when parent arrives late, got %d", len(edges2))
	}
	if edges2[0].Caller != "order-service" || edges2[0].Callee != "inventory-service" {
		t.Errorf("unexpected edge: %+v", edges2[0])
	}
}

func TestEdgeCorrelator_DuplicateRedelivery(t *testing.T) {
	c := NewEdgeCorrelator(time.Minute)
	now := time.Now()

	parent := SpanEvent{
		Service:   "web",
		TraceID:   "t4",
		SpanID:    "s30",
		StartTime: now,
	}
	child := SpanEvent{
		Service:      "db",
		TraceID:      "t4",
		SpanID:       "s31",
		ParentSpanID: "s30",
		SpanKind:     "SPAN_KIND_SERVER",
		StartTime:    now.Add(5 * time.Millisecond),
	}

	// First delivery
	edges1 := c.Correlate("demo", []SpanEvent{parent, child})
	if len(edges1) != 1 {
		t.Fatalf("expected 1 edge on first delivery, got %d", len(edges1))
	}

	// Redelivery of exact same spans
	edges2 := c.Correlate("demo", []SpanEvent{parent, child})
	if len(edges2) != 0 {
		t.Fatalf("expected 0 edges on redelivery due to deduplication, got %d", len(edges2))
	}
}

func TestEdgeCorrelator_SameServiceIgnored(t *testing.T) {
	c := NewEdgeCorrelator(time.Minute)
	now := time.Now()

	parent := SpanEvent{
		Service:   "billing",
		TraceID:   "t5",
		SpanID:    "s40",
		StartTime: now,
	}
	child := SpanEvent{
		Service:      "billing", // Same service
		TraceID:      "t5",
		SpanID:       "s41",
		ParentSpanID: "s40",
		SpanKind:     "SPAN_KIND_SERVER",
		StartTime:    now.Add(2 * time.Millisecond),
	}

	edges := c.Correlate("demo", []SpanEvent{parent, child})
	if len(edges) != 0 {
		t.Fatalf("expected 0 edges for intra-service calls, got %d", len(edges))
	}
}

func TestEdgeCorrelator_MissingParent_TTLExpiration(t *testing.T) {
	c := NewEdgeCorrelator(50 * time.Millisecond)
	now := time.Now()

	child := SpanEvent{
		Service:      "orphan-service",
		TraceID:      "t6",
		SpanID:       "s50",
		ParentSpanID: "s-nonexistent",
		SpanKind:     "SPAN_KIND_SERVER",
		StartTime:    now,
	}

	edges := c.Correlate("demo", []SpanEvent{child})
	if len(edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(edges))
	}

	// After TTL expires, GC clears waiting children
	time.Sleep(70 * time.Millisecond)
	c.gc(time.Now())

	c.mu.Lock()
	count := len(c.waitingChildren)
	c.mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 waiting children after TTL GC, got %d", count)
	}
}
