package model_test

import (
	"testing"
	"time"

	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/model"
)

func TestSpanMethods(t *testing.T) {
	rootSpan := &model.Span{
		SpanID:       "s1",
		ParentSpanID: "",
		StatusCode:   model.StatusCodeOK,
		HTTPStatus:   200,
	}
	if !rootSpan.IsRoot() {
		t.Errorf("expected rootSpan.IsRoot() to be true")
	}
	if rootSpan.HasError() {
		t.Errorf("expected rootSpan.HasError() to be false")
	}

	errSpan := &model.Span{
		SpanID:       "s2",
		ParentSpanID: "s1",
		StatusCode:   model.StatusCodeError,
		HTTPStatus:   500,
	}
	if errSpan.IsRoot() {
		t.Errorf("expected errSpan.IsRoot() to be false")
	}
	if !errSpan.HasError() {
		t.Errorf("expected errSpan.HasError() to be true")
	}
}

func TestTraceBuildTreeAndCriticalPath(t *testing.T) {
	now := time.Now()

	// Create a distributed call graph:
	// api-gateway (root, 100ms)
	//   -> auth-service (child 1, 20ms)
	//   -> order-service (child 2, 70ms)
	//        -> payment-service (grandchild 1, 50ms)
	spans := []*model.Span{
		{
			TraceID:     "trace-1",
			SpanID:      "span-root",
			ServiceName: "api-gateway",
			Name:        "GET /checkout",
			StartTime:   now,
			DurationNs:  100 * 1_000_000,
			StatusCode:  model.StatusCodeOK,
		},
		{
			TraceID:      "trace-1",
			SpanID:       "span-auth",
			ParentSpanID: "span-root",
			ServiceName:  "auth-service",
			Name:         "POST /verify",
			StartTime:    now.Add(5 * time.Millisecond),
			DurationNs:   20 * 1_000_000,
			StatusCode:   model.StatusCodeOK,
		},
		{
			TraceID:      "trace-1",
			SpanID:       "span-order",
			ParentSpanID: "span-root",
			ServiceName:  "order-service",
			Name:         "POST /order",
			StartTime:    now.Add(30 * time.Millisecond),
			DurationNs:   70 * 1_000_000,
			StatusCode:   model.StatusCodeOK,
		},
		{
			TraceID:      "trace-1",
			SpanID:       "span-payment",
			ParentSpanID: "span-order",
			ServiceName:  "payment-service",
			Name:         "POST /charge",
			StartTime:    now.Add(40 * time.Millisecond),
			DurationNs:   50 * 1_000_000,
			StatusCode:   model.StatusCodeOK,
		},
	}

	trace := &model.Trace{
		TraceID: "trace-1",
		Spans:   spans,
	}

	root := trace.BuildTree()
	if root == nil {
		t.Fatalf("expected non-nil root")
	}

	if root.Span.SpanID != "span-root" {
		t.Errorf("expected root spanId span-root, got %s", root.Span.SpanID)
	}

	if len(root.Children) != 2 {
		t.Fatalf("expected 2 direct children under root, got %d", len(root.Children))
	}

	if root.Children[0].Span.SpanID != "span-auth" {
		t.Errorf("expected first child span-auth, got %s", root.Children[0].Span.SpanID)
	}

	if root.Children[1].Span.SpanID != "span-order" {
		t.Errorf("expected second child span-order, got %s", root.Children[1].Span.SpanID)
	}

	// Verify depth
	if root.GetDepth() != 0 {
		t.Errorf("expected root depth 0, got %d", root.GetDepth())
	}
	if root.Children[1].GetDepth() != 1 {
		t.Errorf("expected order depth 1, got %d", root.Children[1].GetDepth())
	}
	if root.Children[1].Children[0].GetDepth() != 2 {
		t.Errorf("expected payment depth 2, got %d", root.Children[1].Children[0].GetDepth())
	}

	// Verify Critical Path
	// Critical path should be root -> order -> payment (total duration 100+70+50 = 220ms, vs root -> auth = 120ms)
	criticalPath := trace.GetCriticalPath()
	if len(criticalPath) != 3 {
		t.Fatalf("expected critical path length 3, got %d", len(criticalPath))
	}

	expectedIDs := []string{"span-root", "span-order", "span-payment"}
	for i, expected := range expectedIDs {
		if criticalPath[i].SpanID != expected {
			t.Errorf("criticalPath[%d] expected %s, got %s", i, expected, criticalPath[i].SpanID)
		}
	}

	// Verify total duration
	totalDuration := trace.CalculateTotalDuration()
	if totalDuration == 0 {
		t.Errorf("expected positive total duration")
	}
}
