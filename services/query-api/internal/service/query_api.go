package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/clickhouse"
	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/model"
)

// QueryAPI defines the contract for querying traces, logs, anomalies, and service topology.
type QueryAPI interface {
	GetTrace(ctx context.Context, tenantID, traceID string) (*model.Trace, error)
	SearchTraces(ctx context.Context, tenantID string, filters model.TraceFilters) ([]*model.TraceSummary, error)
	SearchLogs(ctx context.Context, tenantID string, query model.LogQuery) ([]*model.LogRecord, error)
	TailLogs(ctx context.Context, tenantID string, filters model.LogTailFilters) (<-chan *model.LogRecord, error)
	GetAnomalies(ctx context.Context, tenantID string, r model.AnomalyRange) ([]*model.Anomaly, error)
	ExplainAnomaly(ctx context.Context, tenantID, anomalyID string) (*model.Anomaly, error)
	GetTopology(ctx context.Context, tenantID string, from, to time.Time) (*model.TopologyGraph, error)
}

// queryAPIService implements QueryAPI.
type queryAPIService struct {
	client *clickhouse.Client
	tailer LogTailer
}

// LogTailer provides streaming for live logs.
type LogTailer interface {
	Tail(ctx context.Context, tenantID string, filters model.LogTailFilters) (<-chan *model.LogRecord, error)
}

// NewQueryAPI initializes a new QueryAPI service.
func NewQueryAPI(client *clickhouse.Client, tailer LogTailer) QueryAPI {
	return &queryAPIService{
		client: client,
		tailer: tailer,
	}
}

// GetTrace retrieves all spans for a specific traceId and reconstructs the call graph & critical path.
// Optimization: queries trace_index first to determine the exact time window, pruning ClickHouse partitions.
func (s *queryAPIService) GetTrace(ctx context.Context, tenantID, traceID string) (*model.Trace, error) {
	if tenantID == "" || traceID == "" {
		return nil, fmt.Errorf("tenantID and traceID are required")
	}

	conn := s.client.Conn()

	// Step 1: Query trace_index for the time window
	var t0, t1 time.Time
	indexQuery := `
		SELECT min(first_seen) AS t0, max(last_seen) AS t1
		FROM observability.trace_index
		WHERE tenant_id = ? AND trace_id = ?
	`
	row := conn.QueryRow(ctx, indexQuery, tenantID, traceID)
	_ = row.Scan(&t0, &t1)

	// Fallback if trace_index hasn't materialized or is missing
	if t0.IsZero() {
		t0 = time.Now().Add(-24 * time.Hour)
		t1 = time.Now()
	} else {
		// Buffer slightly to account for millisecond boundary differences
		t0 = t0.Add(-1 * time.Minute)
		t1 = t1.Add(1 * time.Minute)
	}

	// Step 2: Fetch spans within bounded time window, deduplicated by span_id
	spanQuery := `
		SELECT span_id, parent_span_id, service_name, name, kind, start_time, duration_ns,
		       status_code, status_message, http_status_code, attributes, resource_attributes
		FROM observability.spans
		WHERE tenant_id = ? AND trace_id = ?
		  AND start_time BETWEEN ? AND ?
		ORDER BY ingested_at DESC
		LIMIT 1 BY span_id
	`
	rows, err := conn.Query(ctx, spanQuery, tenantID, traceID, t0, t1)
	if err != nil {
		return nil, fmt.Errorf("failed to query spans: %w", err)
	}
	defer rows.Close()

	var spans []*model.Span
	for rows.Next() {
		var (
			spanID             string
			parentSpanID       string
			serviceName        string
			name               string
			kind               string
			startTime          time.Time
			durationNs         uint64
			statusCode         string
			statusMessage      string
			httpStatusCode     uint16
			attributes         map[string]string
			resourceAttributes map[string]string
		)

		if err := rows.Scan(
			&spanID, &parentSpanID, &serviceName, &name, &kind, &startTime, &durationNs,
			&statusCode, &statusMessage, &httpStatusCode, &attributes, &resourceAttributes,
		); err != nil {
			return nil, fmt.Errorf("scan span row: %w", err)
		}

		span := &model.Span{
			TraceID:            traceID,
			SpanID:             spanID,
			ParentSpanID:       parentSpanID,
			ServiceName:        serviceName,
			Name:               name,
			Kind:               model.SpanKind(kind),
			StartTime:          startTime,
			DurationNs:         durationNs,
			StatusCode:         model.StatusCode(statusCode),
			StatusMessage:      statusMessage,
			HTTPStatus:         int(httpStatusCode),
			Attributes:         attributes,
			ResourceAttributes: resourceAttributes,
			IsError:            statusCode == "ERROR" || httpStatusCode >= 400,
		}
		spans = append(spans, span)
	}

	if len(spans) == 0 {
		return nil, fmt.Errorf("trace %s not found", traceID)
	}

	trace := &model.Trace{
		TraceID: traceID,
		Spans:   spans,
	}

	// Step 3: Build hierarchy, critical path, and wall-clock duration
	trace.BuildTree()

	return trace, nil
}

// SearchTraces retrieves a paginated list of traces matching filter criteria.
func (s *queryAPIService) SearchTraces(ctx context.Context, tenantID string, f model.TraceFilters) ([]*model.TraceSummary, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}

	if f.From.IsZero() {
		f.From = time.Now().Add(-15 * time.Minute)
	}
	if f.To.IsZero() {
		f.To = time.Now()
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 50
	}

	var sb strings.Builder
	var args []any

	sb.WriteString(`
		SELECT trace_id,
		       any(service_name) AS root_service,
		       any(name) AS root_operation,
		       min(start_time) AS start_time,
		       max(duration_ns) AS duration_ns,
		       count() AS span_count,
		       countIf(status_code = 'ERROR') > 0 AS has_error
		FROM observability.spans
		WHERE tenant_id = ?
		  AND start_time >= ? AND start_time <= ?
	`)
	args = append(args, tenantID, f.From, f.To)

	if f.Service != "" {
		sb.WriteString(" AND service_name = ?")
		args = append(args, f.Service)
	}
	if f.Operation != "" {
		sb.WriteString(" AND name = ?")
		args = append(args, f.Operation)
	}
	if f.MinDurationNs > 0 {
		sb.WriteString(" AND duration_ns >= ?")
		args = append(args, f.MinDurationNs)
	}
	if f.Status != "" {
		sb.WriteString(" AND status_code = ?")
		args = append(args, f.Status)
	}

	sb.WriteString(" GROUP BY trace_id")
	sb.WriteString(" ORDER BY start_time DESC")
	sb.WriteString(fmt.Sprintf(" LIMIT %d OFFSET %d", f.Limit, f.Offset))

	rows, err := s.client.Conn().Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("search traces: %w", err)
	}
	defer rows.Close()

	var summaries []*model.TraceSummary
	for rows.Next() {
		var (
			traceID       string
			rootService   string
			rootOperation string
			startTime     time.Time
			durationNs    uint64
			spanCount     uint64
			hasError      uint8
		)
		if err := rows.Scan(&traceID, &rootService, &rootOperation, &startTime, &durationNs, &spanCount, &hasError); err != nil {
			return nil, fmt.Errorf("scan trace summary: %w", err)
		}
		summaries = append(summaries, &model.TraceSummary{
			TraceID:       traceID,
			RootService:   rootService,
			RootOperation: rootOperation,
			StartTime:     startTime,
			DurationNs:    durationNs,
			SpanCount:     int(spanCount),
			HasError:      hasError > 0,
		})
	}

	return summaries, nil
}

// SearchLogs executes optimized token-indexed full-text and attribute searches over ClickHouse logs.
func (s *queryAPIService) SearchLogs(ctx context.Context, tenantID string, q model.LogQuery) ([]*model.LogRecord, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}

	if q.From.IsZero() {
		q.From = time.Now().Add(-15 * time.Minute)
	}
	if q.To.IsZero() {
		q.To = time.Now()
	}
	if q.Limit <= 0 || q.Limit > 1000 {
		q.Limit = 100
	}

	var sb strings.Builder
	var args []any

	sb.WriteString(`
		SELECT timestamp, service_name, severity_text, severity_number, body,
		       trace_id, span_id, attributes, resource_attributes
		FROM observability.logs
		WHERE tenant_id = ?
		  AND timestamp >= ? AND timestamp <= ?
	`)
	args = append(args, tenantID, q.From, q.To)

	if q.Service != "" {
		sb.WriteString(" AND service_name = ?")
		args = append(args, q.Service)
	}
	if q.Severity != "" {
		sb.WriteString(" AND severity_text = ?")
		args = append(args, q.Severity)
	}
	if q.TraceID != "" {
		sb.WriteString(" AND trace_id = ?")
		args = append(args, q.TraceID)
	}
	if q.SpanID != "" {
		sb.WriteString(" AND span_id = ?")
		args = append(args, q.SpanID)
	}
	if q.Query != "" {
		// Use ClickHouse tokenbf_v1 indexed whole-word search for high throughput
		sb.WriteString(" AND hasToken(body, ?)")
		args = append(args, q.Query)
	}

	for k, v := range q.Attributes {
		sb.WriteString(" AND attributes[?] = ?")
		args = append(args, k, v)
	}

	sb.WriteString(" ORDER BY timestamp DESC")
	sb.WriteString(fmt.Sprintf(" LIMIT %d", q.Limit))

	rows, err := s.client.Conn().Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("search logs: %w", err)
	}
	defer rows.Close()

	var logs []*model.LogRecord
	for rows.Next() {
		var (
			ts                 time.Time
			serviceName        string
			severityText       string
			severityNum        uint8
			body               string
			traceID            string
			spanID             string
			attributes         map[string]string
			resourceAttributes map[string]string
		)

		if err := rows.Scan(
			&ts, &serviceName, &severityText, &severityNum, &body,
			&traceID, &spanID, &attributes, &resourceAttributes,
		); err != nil {
			return nil, fmt.Errorf("scan log row: %w", err)
		}

		logs = append(logs, &model.LogRecord{
			Timestamp:          ts,
			ServiceName:        serviceName,
			Severity:           severityText,
			SeverityNumber:     severityNum,
			Body:               body,
			TraceID:            traceID,
			SpanID:             spanID,
			Attributes:         attributes,
			ResourceAttributes: resourceAttributes,
		})
	}

	return logs, nil
}

// TailLogs establishes an active channel for real-time streaming of logs.
func (s *queryAPIService) TailLogs(ctx context.Context, tenantID string, f model.LogTailFilters) (<-chan *model.LogRecord, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}
	if s.tailer != nil {
		return s.tailer.Tail(ctx, tenantID, f)
	}
	// Fallback to empty closed channel if tailer not initialized
	ch := make(chan *model.LogRecord)
	close(ch)
	return ch, nil
}

// GetAnomalies fetches detected statistical anomalies from ClickHouse within a specified range.
func (s *queryAPIService) GetAnomalies(ctx context.Context, tenantID string, r model.AnomalyRange) ([]*model.Anomaly, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}

	if r.From.IsZero() {
		r.From = time.Now().Add(-24 * time.Hour)
	}
	if r.To.IsZero() {
		r.To = time.Now()
	}
	if r.Limit <= 0 || r.Limit > 200 {
		r.Limit = 50
	}

	query := `
		SELECT id, service_name, operation, metric, window_start, window_end,
		       baseline_value, observed_value, z_score, root_cause_service, evidence, status
		FROM observability.anomalies
		WHERE tenant_id = ?
		  AND window_start >= ? AND window_start <= ?
		ORDER BY window_start DESC
		LIMIT ?
	`

	rows, err := s.client.Conn().Query(ctx, query, tenantID, r.From, r.To, r.Limit)
	if err != nil {
		return nil, fmt.Errorf("get anomalies: %w", err)
	}
	defer rows.Close()

	var anomalies []*model.Anomaly
	for rows.Next() {
		var (
			idRaw            uuid.UUID
			serviceName      string
			operation        string
			metric           string
			windowStart      time.Time
			windowEnd        time.Time
			baselineVal      float64
			observedVal      float64
			zScore           float64
			rootCauseService string
			evidence         string
			status           uint8 // Enum8: 0 = OPEN, 1 = RESOLVED
		)

		if err := rows.Scan(
			&idRaw, &serviceName, &operation, &metric, &windowStart, &windowEnd,
			&baselineVal, &observedVal, &zScore, &rootCauseService, &evidence, &status,
		); err != nil {
			return nil, fmt.Errorf("scan anomaly row: %w", err)
		}

		statusStr := "OPEN"
		if status == 1 {
			statusStr = "RESOLVED"
		}

		var relatedSpans []string
		if evidence != "" {
			var evMap map[string]any
			if err := json.Unmarshal([]byte(evidence), &evMap); err == nil {
				if spansList, ok := evMap["related_spans"].([]any); ok {
					for _, item := range spansList {
						if str, isStr := item.(string); isStr {
							relatedSpans = append(relatedSpans, str)
						}
					}
				}
			}
		}

		anomalies = append(anomalies, &model.Anomaly{
			ID:                 idRaw.String(),
			Service:            serviceName,
			Operation:          operation,
			Metric:             metric,
			BaselineValue:      baselineVal,
			ObservedValue:      observedVal,
			ZScore:             zScore,
			WindowStart:        windowStart,
			WindowEnd:          windowEnd,
			SuspectedRootCause: rootCauseService,
			Evidence:           evidence,
			RelatedSpans:       relatedSpans,
			Status:             statusStr,
		})
	}

	return anomalies, nil
}

// ExplainAnomaly retrieves the root-cause diagnosis and evidence for a specific anomaly ID.
func (s *queryAPIService) ExplainAnomaly(ctx context.Context, tenantID, anomalyID string) (*model.Anomaly, error) {
	if tenantID == "" || anomalyID == "" {
		return nil, fmt.Errorf("tenantID and anomalyID are required")
	}

	query := `
		SELECT id, service_name, operation, metric, window_start, window_end,
		       baseline_value, observed_value, z_score, root_cause_service, evidence, status
		FROM observability.anomalies
		WHERE tenant_id = ? AND id = ?
		LIMIT 1
	`

	var (
		idRaw            uuid.UUID
		serviceName      string
		operation        string
		metric           string
		windowStart      time.Time
		windowEnd        time.Time
		baselineVal      float64
		observedVal      float64
		zScore           float64
		rootCauseService string
		evidence         string
		status           uint8
	)

	row := s.client.Conn().QueryRow(ctx, query, tenantID, anomalyID)
	if err := row.Scan(
		&idRaw, &serviceName, &operation, &metric, &windowStart, &windowEnd,
		&baselineVal, &observedVal, &zScore, &rootCauseService, &evidence, &status,
	); err != nil {
		return nil, fmt.Errorf("explain anomaly %s: %w", anomalyID, err)
	}

	statusStr := "OPEN"
	if status == 1 {
		statusStr = "RESOLVED"
	}

	var relatedSpans []string
	if evidence != "" {
		var evMap map[string]any
		if err := json.Unmarshal([]byte(evidence), &evMap); err == nil {
			if spansList, ok := evMap["related_spans"].([]any); ok {
				for _, item := range spansList {
					if str, isStr := item.(string); isStr {
						relatedSpans = append(relatedSpans, str)
					}
				}
			}
		}
	}

	return &model.Anomaly{
		ID:                 idRaw.String(),
		Service:            serviceName,
		Operation:          operation,
		Metric:             metric,
		BaselineValue:      baselineVal,
		ObservedValue:      observedVal,
		ZScore:             zScore,
		WindowStart:        windowStart,
		WindowEnd:          windowEnd,
		SuspectedRootCause: rootCauseService,
		Evidence:           evidence,
		RelatedSpans:       relatedSpans,
		Status:             statusStr,
	}, nil
}

// GetTopology constructs the active service dependency graph using rolled-up edge events.
func (s *queryAPIService) GetTopology(ctx context.Context, tenantID string, from, to time.Time) (*model.TopologyGraph, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}

	if from.IsZero() {
		from = time.Now().Add(-15 * time.Minute)
	}
	if to.IsZero() {
		to = time.Now()
	}

	query := `
		SELECT caller, callee,
		       sum(calls) AS call_count,
		       sum(errors) AS error_count,
		       quantilesTDigestMerge(0.5, 0.95, 0.99)(duration_q) AS quantiles
		FROM observability.service_edges_1m
		WHERE tenant_id = ?
		  AND minute >= ? AND minute <= ?
		GROUP BY caller, callee
	`

	rows, err := s.client.Conn().Query(ctx, query, tenantID, from, to)
	if err != nil {
		// Fallback to empty topology if no edges yet
		return &model.TopologyGraph{Edges: []*model.ServiceEdge{}, Services: []string{}}, nil
	}
	defer rows.Close()

	serviceSet := make(map[string]struct{})
	var edges []*model.ServiceEdge

	for rows.Next() {
		var (
			caller     string
			callee     string
			calls      uint64
			errors     uint64
			quantiles  []float64
		)

		if err := rows.Scan(&caller, &callee, &calls, &errors, &quantiles); err != nil {
			return nil, fmt.Errorf("scan topology edge: %w", err)
		}

		serviceSet[caller] = struct{}{}
		serviceSet[callee] = struct{}{}

		var p50, p95, p99 float64
		if len(quantiles) >= 3 {
			p50 = quantiles[0] / 1e6 // Convert nanoseconds to milliseconds
			p95 = quantiles[1] / 1e6
			p99 = quantiles[2] / 1e6
		}

		edges = append(edges, &model.ServiceEdge{
			Caller:     caller,
			Callee:     callee,
			CallCount:  int64(calls),
			ErrorCount: int64(errors),
			P50Ms:      p50,
			P95Ms:      p95,
			P99Ms:      p99,
		})
	}

	var services []string
	for svc := range serviceSet {
		services = append(services, svc)
	}

	return &model.TopologyGraph{
		Edges:    edges,
		Services: services,
	}, nil
}
