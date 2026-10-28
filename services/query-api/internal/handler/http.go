package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/model"
	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/service"
)

// HTTPHandler exposes REST and Server-Sent Events endpoints for QueryAPI.
type HTTPHandler struct {
	svc service.QueryAPI
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(svc service.QueryAPI) *HTTPHandler {
	return &HTTPHandler{svc: svc}
}

// RegisterRoutes registers endpoints onto an http.ServeMux.
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/api/v1/traces/", h.handleGetTrace)
	mux.HandleFunc("/api/v1/traces", h.handleSearchTraces)
	mux.HandleFunc("/api/v1/logs", h.handleSearchLogs)
	mux.HandleFunc("/api/v1/logs/tail", h.handleTailLogs)
	mux.HandleFunc("/api/v1/anomalies/", h.handleExplainAnomaly)
	mux.HandleFunc("/api/v1/anomalies", h.handleGetAnomalies)
	mux.HandleFunc("/api/v1/topology", h.handleGetTopology)
	mux.HandleFunc("/api/v1/metrics/red", h.handleGetREDMetrics)
}

func (h *HTTPHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"UP"}`))
}

// handleGetTrace returns a single reconstructed trace DAG with critical path.
// GET /api/v1/traces/{traceId}
func (h *HTTPHandler) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 || pathParts[3] == "" {
		http.Error(w, "traceId required in path", http.StatusBadRequest)
		return
	}
	traceID := pathParts[3]
	tenantID := getTenant(r)

	trace, err := h.svc.GetTrace(r.Context(), tenantID, traceID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, trace)
}

// handleSearchTraces queries matching traces.
// GET /api/v1/traces?service=...&minDuration=...&status=...&from=...&to=...&limit=...
func (h *HTTPHandler) handleSearchTraces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	f := model.TraceFilters{
		Service:   q.Get("service"),
		Operation: q.Get("operation"),
		Status:    q.Get("status"),
		Limit:     parseInt(q.Get("limit"), 50),
		Offset:    parseInt(q.Get("offset"), 0),
	}

	if minDur := q.Get("minDuration"); minDur != "" {
		if d, err := strconv.ParseUint(minDur, 10, 64); err == nil {
			f.MinDurationNs = d
		}
	}

	f.From = parseTime(q.Get("from"), time.Now().Add(-15*time.Minute))
	f.To = parseTime(q.Get("to"), time.Now())

	traces, err := h.svc.SearchTraces(r.Context(), getTenant(r), f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  traces,
		"count": len(traces),
	})
}

// handleSearchLogs queries logs by token or attribute.
// GET /api/v1/logs?query=...&service=...&severity=...&from=...&to=...
func (h *HTTPHandler) handleSearchLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	logQuery := model.LogQuery{
		Query:    q.Get("query"),
		Service:  q.Get("service"),
		Severity: q.Get("severity"),
		TraceID:  q.Get("traceId"),
		SpanID:   q.Get("spanId"),
		Limit:    parseInt(q.Get("limit"), 100),
		From:     parseTime(q.Get("from"), time.Now().Add(-15*time.Minute)),
		To:       parseTime(q.Get("to"), time.Now()),
	}

	logs, err := h.svc.SearchLogs(r.Context(), getTenant(r), logQuery)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  logs,
		"count": len(logs),
	})
}

// handleTailLogs streams live logs using Server-Sent Events (SSE).
// GET /api/v1/logs/tail?service=...&severity=...
func (h *HTTPHandler) handleTailLogs(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	f := model.LogTailFilters{
		Service:  q.Get("service"),
		Severity: q.Get("severity"),
		Query:    q.Get("query"),
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch, err := h.svc.TailLogs(r.Context(), getTenant(r), f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case record, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(record)
			if err == nil {
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}

// handleGetAnomalies fetches detected statistical anomalies.
// GET /api/v1/anomalies?range=15m
func (h *HTTPHandler) handleGetAnomalies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	rangeDuration := 24 * time.Hour
	if rStr := q.Get("range"); rStr != "" {
		if d, err := time.ParseDuration(rStr); err == nil {
			rangeDuration = d
		}
	}

	anomRange := model.AnomalyRange{
		From:  time.Now().Add(-rangeDuration),
		To:    time.Now(),
		Limit: parseInt(q.Get("limit"), 50),
	}

	anomalies, err := h.svc.GetAnomalies(r.Context(), getTenant(r), anomRange)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  anomalies,
		"count": len(anomalies),
	})
}

// handleExplainAnomaly fetches the root-cause diagnosis for a specific anomaly ID.
// GET /api/v1/anomalies/{id}
func (h *HTTPHandler) handleExplainAnomaly(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 || pathParts[3] == "" {
		http.Error(w, "anomaly id required in path", http.StatusBadRequest)
		return
	}
	anomalyID := pathParts[3]

	anomaly, err := h.svc.ExplainAnomaly(r.Context(), getTenant(r), anomalyID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, anomaly)
}

// handleGetTopology fetches the service dependency topology graph.
// GET /api/v1/topology?from=...&to=...
func (h *HTTPHandler) handleGetTopology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	from := parseTime(q.Get("from"), time.Now().Add(-15*time.Minute))
	to := parseTime(q.Get("to"), time.Now())

	graph, err := h.svc.GetTopology(r.Context(), getTenant(r), from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, graph)
}

// handleGetREDMetrics returns live Rate, Errors, Duration metrics for microservices.
// GET /api/v1/metrics/red
func (h *HTTPHandler) handleGetREDMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics, err := h.svc.GetREDMetrics(r.Context(), getTenant(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  metrics,
		"count": len(metrics),
	})
}

func getTenant(r *http.Request) string {
	if t := r.Header.Get("X-Tenant-Id"); t != "" {
		return t
	}
	if t := r.URL.Query().Get("tenant"); t != "" {
		return t
	}
	return "demo"
}

func parseInt(s string, def int) int {
	if i, err := strconv.Atoi(s); err == nil && i > 0 {
		return i
	}
	return def
}

func parseTime(s string, def time.Time) time.Time {
	if s == "" {
		return def
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return def
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
