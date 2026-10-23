package ingest

import (
	"bytes"
	"testing"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func span(trace, id, parent []byte, start uint64) *tracepb.Span {
	return &tracepb.Span{TraceId: trace, SpanId: id, ParentSpanId: parent, StartTimeUnixNano: start}
}

func TestFilterTraces(t *testing.T) {
	good := bytes.Repeat([]byte{1}, 16)
	gid := bytes.Repeat([]byte{2}, 8)
	req := &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{
			span(good, gid, nil, 1),             // valid root
			span(good, gid, gid, 1),             // valid child
			span(good[:15], gid, nil, 1),        // short trace id
			span(make([]byte, 16), gid, nil, 1), // zero trace id
			span(good, make([]byte, 8), nil, 1), // zero span id
			span(good, gid, []byte{1, 2, 3}, 1), // bad parent id
			span(good, gid, nil, 0),             // no start time
		}}},
	}}}
	if got := FilterTraces(req); got != 5 {
		t.Fatalf("rejected = %d, want 5", got)
	}
	if got := CountSpans(req); got != 2 {
		t.Fatalf("kept = %d, want 2", got)
	}
}

func TestFilterLogs(t *testing.T) {
	req := &collogspb.ExportLogsServiceRequest{ResourceLogs: []*logspb.ResourceLogs{{
		ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{
			{}, // uncorrelated log is fine
			{TraceId: bytes.Repeat([]byte{1}, 16), SpanId: bytes.Repeat([]byte{2}, 8)},
			{TraceId: []byte{1}}, // malformed trace id
			{SpanId: []byte{1}},  // malformed span id
		}}},
	}}}
	if got := FilterLogs(req); got != 2 {
		t.Fatalf("rejected = %d, want 2", got)
	}
	if got := CountLogs(req); got != 2 {
		t.Fatalf("kept = %d, want 2", got)
	}
}
