// Package ingest holds the transport-independent ingestion logic shared by gRPC and HTTP:
// admission control, validation, and handing validated data to the batcher.
package ingest

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

// ErrRateLimited means the tenant exceeded its request rate (maps to 429).
var ErrRateLimited = errors.New("ingest: rate limit exceeded")

// Submitter is implemented by *batcher.Batcher.
type Submitter interface {
	Submit(ctx context.Context, tenant string, payload []byte) error
}

// Limiter is implemented by *ratelimit.Limiter.
type Limiter interface{ Allow(key string) bool }

// Recorder receives counters; implemented by the metrics package.
type Recorder interface{ Dropped(signal string, n int64) }

// Service validates and enqueues telemetry.
type Service struct {
	traces, logs Submitter
	limiter      Limiter
	rec          Recorder
}

// New wires the service. All arguments are required.
func New(traces, logs Submitter, limiter Limiter, rec Recorder) *Service {
	return &Service{traces: traces, logs: logs, limiter: limiter, rec: rec}
}

// Admit applies the per-tenant rate limit. Transports call it right after authentication and
// before reading or parsing the body, so rejected clients cost almost nothing.
func (s *Service) Admit(tenant string) error {
	if !s.limiter.Allow(tenant) {
		return ErrRateLimited
	}
	return nil
}

// Traces validates req and publishes it. It returns only after the data is durable in the queue.
func (s *Service) Traces(ctx context.Context, tenant string, req *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	resp := &coltracepb.ExportTraceServiceResponse{}
	if rejected := FilterTraces(req); rejected > 0 {
		s.rec.Dropped("traces", rejected)
		resp.PartialSuccess = &coltracepb.ExportTracePartialSuccess{
			RejectedSpans: rejected,
			ErrorMessage:  "spans with malformed trace_id, span_id, parent_span_id or an unset start time were dropped",
		}
	}
	if CountSpans(req) == 0 {
		return resp, nil
	}
	payload, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ingest: marshal traces: %w", err)
	}
	if err := s.traces.Submit(ctx, tenant, payload); err != nil {
		return nil, err
	}
	return resp, nil
}

// Logs validates req and publishes it. It returns only after the data is durable in the queue.
func (s *Service) Logs(ctx context.Context, tenant string, req *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
	resp := &collogspb.ExportLogsServiceResponse{}
	if rejected := FilterLogs(req); rejected > 0 {
		s.rec.Dropped("logs", rejected)
		resp.PartialSuccess = &collogspb.ExportLogsPartialSuccess{
			RejectedLogRecords: rejected,
			ErrorMessage:       "log records with malformed trace_id or span_id were dropped",
		}
	}
	if CountLogs(req) == 0 {
		return resp, nil
	}
	payload, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ingest: marshal logs: %w", err)
	}
	if err := s.logs.Submit(ctx, tenant, payload); err != nil {
		return nil, err
	}
	return resp, nil
}
