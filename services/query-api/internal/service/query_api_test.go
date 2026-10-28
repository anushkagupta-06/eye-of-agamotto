package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/model"
	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/service"
)

type mockTailer struct {
	connected bool
}

func (m *mockTailer) Tail(ctx context.Context, tenantID string, filters model.LogTailFilters) (<-chan *model.LogRecord, error) {
	ch := make(chan *model.LogRecord)
	close(ch)
	return ch, nil
}

func (m *mockTailer) IsConnected() bool {
	return m.connected
}

func TestCheckHealth(t *testing.T) {
	// When ClickHouse client is nil
	svc := service.NewQueryAPI(nil, &mockTailer{connected: true})
	status, err := svc.CheckHealth(context.Background())
	if err == nil {
		t.Fatalf("expected error when ClickHouse client is uninitialized, got nil")
	}
	if status["clickhouse"] != "DOWN: client not initialized" {
		t.Errorf("expected clickhouse DOWN, got %s", status["clickhouse"])
	}
	if status["nats"] != "UP" {
		t.Errorf("expected nats UP, got %s", status["nats"])
	}
}

func TestTraceContracts_DuplicateAndPartialSpans(t *testing.T) {
	now := time.Now()

	// 1. Trace with duplicate spans (simulating repeated inserts)
	dupTrace := &model.Trace{
		TraceID: "dup-trace-1",
		Spans: []*model.Span{
			{
				TraceID:     "dup-trace-1",
				SpanID:      "root-span",
				ServiceName: "gateway",
				Name:        "GET /order",
				StartTime:   now,
				DurationNs:  100000000,
			},
			{
				TraceID:     "dup-trace-1",
				SpanID:      "root-span", // duplicate spanID
				ServiceName: "gateway",
				Name:        "GET /order",
				StartTime:   now,
				DurationNs:  100000000,
			},
			{
				TraceID:      "dup-trace-1",
				SpanID:       "child-span",
				ParentSpanID: "root-span",
				ServiceName:  "order-service",
				Name:         "POST /order",
				StartTime:    now.Add(10 * time.Millisecond),
				DurationNs:   50000000,
			},
		},
	}

	rootNode := dupTrace.BuildTree()
	if rootNode == nil {
		t.Fatalf("expected root node, got nil")
	}
	if len(dupTrace.Spans) != 2 {
		t.Errorf("expected 2 deduplicated spans, got %d", len(dupTrace.Spans))
	}
	if len(rootNode.Children) != 1 {
		t.Errorf("expected 1 child under root, got %d", len(rootNode.Children))
	}

	// 2. Partial trace where parent span was not captured (orphaned span)
	partialTrace := &model.Trace{
		TraceID: "partial-trace-2",
		Spans: []*model.Span{
			{
				TraceID:      "partial-trace-2",
				SpanID:       "orphan-child-1",
				ParentSpanID: "non-existent-root-id",
				ServiceName:  "payment-service",
				Name:         "POST /charge",
				StartTime:    now,
				DurationNs:   80000000,
			},
			{
				TraceID:      "partial-trace-2",
				SpanID:       "orphan-child-2",
				ParentSpanID: "non-existent-root-id",
				ServiceName:  "fraud-service",
				Name:         "GET /check",
				StartTime:    now.Add(5 * time.Millisecond),
				DurationNs:   20000000,
			},
		},
	}

	partialRoot := partialTrace.BuildTree()
	if partialRoot == nil {
		t.Fatalf("expected synthetic root for disconnected partial trace, got nil")
	}
	if len(partialRoot.Children) != 2 {
		t.Errorf("expected 2 orphaned spans attached under synthetic root, got %d", len(partialRoot.Children))
	}
}
