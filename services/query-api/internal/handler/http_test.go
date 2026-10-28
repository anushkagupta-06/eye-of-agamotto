package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/handler"
	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/model"
)

type mockQueryAPI struct{}

func (m *mockQueryAPI) GetTrace(ctx context.Context, tenantID, traceID string) (*model.Trace, error) {
	return &model.Trace{
		TraceID:       traceID,
		TotalDuration: 42000000,
		Spans: []*model.Span{
			{
				TraceID:     traceID,
				SpanID:      "s1",
				ServiceName: "gateway",
				Name:        "GET /test",
			},
		},
	}, nil
}

func (m *mockQueryAPI) SearchTraces(ctx context.Context, tenantID string, f model.TraceFilters) ([]*model.TraceSummary, error) {
	return []*model.TraceSummary{
		{
			TraceID:     "trace-abc",
			RootService: "gateway",
			SpanCount:   3,
		},
	}, nil
}

func (m *mockQueryAPI) SearchLogs(ctx context.Context, tenantID string, q model.LogQuery) ([]*model.LogRecord, error) {
	return []*model.LogRecord{
		{
			ServiceName: "auth",
			Severity:    "ERROR",
			Body:        "token expired",
		},
	}, nil
}

func (m *mockQueryAPI) TailLogs(ctx context.Context, tenantID string, f model.LogTailFilters) (<-chan *model.LogRecord, error) {
	ch := make(chan *model.LogRecord, 1)
	ch <- &model.LogRecord{
		ServiceName: "worker",
		Severity:    "INFO",
		Body:        "processing job",
	}
	close(ch)
	return ch, nil
}

func (m *mockQueryAPI) GetAnomalies(ctx context.Context, tenantID string, r model.AnomalyRange) ([]*model.Anomaly, error) {
	return []*model.Anomaly{
		{
			ID:      "anom-1",
			Service: "checkout",
			Metric:  "p99_latency",
			ZScore:  4.2,
		},
	}, nil
}

func (m *mockQueryAPI) GetTopology(ctx context.Context, tenantID string, from, to time.Time) (*model.TopologyGraph, error) {
	return &model.TopologyGraph{
		Services: []string{"gateway", "checkout"},
		Edges: []*model.ServiceEdge{
			{
				Caller:    "gateway",
				Callee:    "checkout",
				CallCount: 1500,
			},
		},
	}, nil
}

func TestHTTPHandlerRoutes(t *testing.T) {
	mockSvc := &mockQueryAPI{}
	h := handler.NewHTTPHandler(mockSvc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// 1. Health check
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// 2. GetTrace
	req = httptest.NewRequest(http.MethodGet, "/api/v1/traces/trace-123", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var trace model.Trace
	if err := json.Unmarshal(rr.Body.Bytes(), &trace); err != nil {
		t.Fatalf("unmarshal trace: %v", err)
	}
	if trace.TraceID != "trace-123" {
		t.Errorf("expected traceId trace-123, got %s", trace.TraceID)
	}

	// 3. SearchTraces
	req = httptest.NewRequest(http.MethodGet, "/api/v1/traces?service=gateway&status=OK", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// 4. SearchLogs
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs?service=auth&query=token", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// 5. GetAnomalies
	req = httptest.NewRequest(http.MethodGet, "/api/v1/anomalies?range=1h", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// 6. GetTopology
	req = httptest.NewRequest(http.MethodGet, "/api/v1/topology", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var topo model.TopologyGraph
	if err := json.Unmarshal(rr.Body.Bytes(), &topo); err != nil {
		t.Fatalf("unmarshal topology: %v", err)
	}
	if len(topo.Edges) != 1 || topo.Edges[0].Caller != "gateway" {
		t.Errorf("unexpected topology edge: %+v", topo.Edges)
	}
}
