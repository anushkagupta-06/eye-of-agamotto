package ingest

import (
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

const (
	traceIDLen = 16
	spanIDLen  = 8
)

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

func validSpan(s *tracepb.Span) bool {
	if len(s.TraceId) != traceIDLen || allZero(s.TraceId) {
		return false
	}
	if len(s.SpanId) != spanIDLen || allZero(s.SpanId) {
		return false
	}
	if len(s.ParentSpanId) != 0 && len(s.ParentSpanId) != spanIDLen {
		return false
	}
	return s.StartTimeUnixNano != 0
}

// FilterTraces removes invalid spans in place and returns how many were removed. A span is
// invalid if its ids are malformed or its start time is unset. Dropping instead of failing the
// whole request follows the OTLP partial-success model: one bad span must not discard a batch.
func FilterTraces(req *coltracepb.ExportTraceServiceRequest) (rejected int64) {
	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			kept := ss.Spans[:0]
			for _, s := range ss.Spans {
				if validSpan(s) {
					kept = append(kept, s)
				} else {
					rejected++
				}
			}
			ss.Spans = kept
		}
	}
	return rejected
}

// CountSpans returns the number of spans in req.
func CountSpans(req *coltracepb.ExportTraceServiceRequest) (n int) {
	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			n += len(ss.Spans)
		}
	}
	return n
}

func validLog(l *logspb.LogRecord) bool {
	if len(l.TraceId) != 0 && len(l.TraceId) != traceIDLen {
		return false
	}
	return len(l.SpanId) == 0 || len(l.SpanId) == spanIDLen
}

// FilterLogs removes log records with malformed correlation ids and returns how many were removed.
func FilterLogs(req *collogspb.ExportLogsServiceRequest) (rejected int64) {
	for _, rl := range req.GetResourceLogs() {
		for _, sl := range rl.GetScopeLogs() {
			kept := sl.LogRecords[:0]
			for _, l := range sl.LogRecords {
				if validLog(l) {
					kept = append(kept, l)
				} else {
					rejected++
				}
			}
			sl.LogRecords = kept
		}
	}
	return rejected
}

// CountLogs returns the number of log records in req.
func CountLogs(req *collogspb.ExportLogsServiceRequest) (n int) {
	for _, rl := range req.GetResourceLogs() {
		for _, sl := range rl.GetScopeLogs() {
			n += len(sl.LogRecords)
		}
	}
	return n
}
