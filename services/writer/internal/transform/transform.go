// Package transform converts OTLP requests into ClickHouse rows (the mapping documented in
// docs/contracts/ingestion-contract.md).
package transform

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"google.golang.org/protobuf/proto"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/anushkagupta-06/eye-of-agamotto/services/writer/internal/model"
)

const unknownService = "unknown_service"

var (
	kindNames   = [...]string{"UNSPECIFIED", "INTERNAL", "SERVER", "CLIENT", "PRODUCER", "CONSUMER"}
	statusNames = [...]string{"UNSET", "OK", "ERROR"}
)

// DecodeSpans parses an ExportTraceServiceRequest payload and converts it to rows.
func DecodeSpans(tenant string, _ time.Time, data []byte) ([]model.Span, error) {
	var req coltracepb.ExportTraceServiceRequest
	if err := proto.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("unmarshal traces: %w", err)
	}
	return Spans(tenant, &req), nil
}

// DecodeLogs parses an ExportLogsServiceRequest payload and converts it to rows. received is
// used as the timestamp of records that carry neither a timestamp nor an observed time.
func DecodeLogs(tenant string, received time.Time, data []byte) ([]model.Log, error) {
	var req collogspb.ExportLogsServiceRequest
	if err := proto.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("unmarshal logs: %w", err)
	}
	return Logs(tenant, received, &req), nil
}

// Spans converts a trace export request to rows.
func Spans(tenant string, req *coltracepb.ExportTraceServiceRequest) []model.Span {
	var out []model.Span
	for _, rs := range req.GetResourceSpans() {
		resAttrs := flatten(rs.GetResource().GetAttributes())
		service := serviceName(resAttrs)
		for _, ss := range rs.GetScopeSpans() {
			for _, s := range ss.GetSpans() {
				attrs := flatten(s.GetAttributes())
				out = append(out, model.Span{
					TenantID:           tenant,
					TraceID:            hexID(s.GetTraceId()),
					SpanID:             hexID(s.GetSpanId()),
					ParentSpanID:       hexID(s.GetParentSpanId()),
					ServiceName:        service,
					Name:               s.GetName(),
					Kind:               name(kindNames[:], int32(s.GetKind())),
					StartTime:          nanoTime(s.GetStartTimeUnixNano()),
					DurationNs:         duration(s.GetStartTimeUnixNano(), s.GetEndTimeUnixNano()),
					StatusCode:         name(statusNames[:], int32(s.GetStatus().GetCode())),
					StatusMessage:      s.GetStatus().GetMessage(),
					HTTPStatusCode:     httpStatus(attrs),
					Attributes:         attrs,
					ResourceAttributes: resAttrs,
				})
			}
		}
	}
	return out
}

// Logs converts a logs export request to rows.
func Logs(tenant string, received time.Time, req *collogspb.ExportLogsServiceRequest) []model.Log {
	var out []model.Log
	for _, rl := range req.GetResourceLogs() {
		resAttrs := flatten(rl.GetResource().GetAttributes())
		service := serviceName(resAttrs)
		for _, sl := range rl.GetScopeLogs() {
			for _, l := range sl.GetLogRecords() {
				out = append(out, logRow(tenant, received, service, resAttrs, l))
			}
		}
	}
	return out
}

func logRow(tenant string, received time.Time, service string, resAttrs map[string]string, l *logspb.LogRecord) model.Log {
	ts := received.UTC()
	switch {
	case l.GetTimeUnixNano() != 0:
		ts = nanoTime(l.GetTimeUnixNano())
	case l.GetObservedTimeUnixNano() != 0:
		ts = nanoTime(l.GetObservedTimeUnixNano())
	}
	return model.Log{
		TenantID:           tenant,
		Timestamp:          ts,
		ServiceName:        service,
		SeverityText:       l.GetSeverityText(),
		SeverityNumber:     severityNumber(int32(l.GetSeverityNumber())),
		Body:               anyString(l.GetBody()),
		TraceID:            hexID(l.GetTraceId()),
		SpanID:             hexID(l.GetSpanId()),
		Attributes:         flatten(l.GetAttributes()),
		ResourceAttributes: resAttrs,
	}
}

func serviceName(resAttrs map[string]string) string {
	if s := resAttrs["service.name"]; s != "" {
		return s
	}
	return unknownService
}

// hexID renders an id as lowercase hex, or "" when it is absent or all zero bytes.
func hexID(b []byte) string {
	for _, c := range b {
		if c != 0 {
			return hex.EncodeToString(b)
		}
	}
	return ""
}

func name(names []string, v int32) string {
	if v < 0 || int(v) >= len(names) {
		return names[0]
	}
	return names[v]
}

func nanoTime(ns uint64) time.Time {
	if ns > math.MaxInt64 {
		ns = math.MaxInt64
	}
	return time.Unix(0, int64(ns)).UTC()
}

// duration returns end-start, clamped at 0 when the end is unset or precedes the start.
func duration(start, end uint64) uint64 {
	if end <= start {
		return 0
	}
	return end - start
}

func severityNumber(v int32) uint8 {
	if v < 0 || v > 24 {
		return 0
	}
	return uint8(v)
}

func httpStatus(attrs map[string]string) uint16 {
	for _, key := range [...]string{"http.response.status_code", "http.status_code"} {
		if v, ok := attrs[key]; ok {
			if n, err := strconv.ParseUint(v, 10, 16); err == nil {
				return uint16(n)
			}
		}
	}
	return 0
}

// flatten stringifies attributes: scalars as text, arrays and maps as JSON, bytes as base64.
func flatten(kvs []*commonpb.KeyValue) map[string]string {
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		m[kv.GetKey()] = anyString(kv.GetValue())
	}
	return m
}

func anyString(v *commonpb.AnyValue) string {
	switch x := v.GetValue().(type) {
	case nil:
		return ""
	case *commonpb.AnyValue_StringValue:
		return x.StringValue
	case *commonpb.AnyValue_BoolValue:
		return strconv.FormatBool(x.BoolValue)
	case *commonpb.AnyValue_IntValue:
		return strconv.FormatInt(x.IntValue, 10)
	case *commonpb.AnyValue_DoubleValue:
		return strconv.FormatFloat(x.DoubleValue, 'g', -1, 64)
	case *commonpb.AnyValue_BytesValue:
		return base64.StdEncoding.EncodeToString(x.BytesValue)
	default: // arrays and key/value lists
		b, err := json.Marshal(toAny(v))
		if err != nil {
			return fmt.Sprint(toAny(v))
		}
		return string(b)
	}
}

func toAny(v *commonpb.AnyValue) any {
	switch x := v.GetValue().(type) {
	case nil:
		return nil
	case *commonpb.AnyValue_StringValue:
		return x.StringValue
	case *commonpb.AnyValue_BoolValue:
		return x.BoolValue
	case *commonpb.AnyValue_IntValue:
		return x.IntValue
	case *commonpb.AnyValue_DoubleValue:
		if math.IsNaN(x.DoubleValue) || math.IsInf(x.DoubleValue, 0) {
			return strconv.FormatFloat(x.DoubleValue, 'g', -1, 64)
		}
		return x.DoubleValue
	case *commonpb.AnyValue_BytesValue:
		return base64.StdEncoding.EncodeToString(x.BytesValue)
	case *commonpb.AnyValue_ArrayValue:
		arr := make([]any, 0, len(x.ArrayValue.GetValues()))
		for _, e := range x.ArrayValue.GetValues() {
			arr = append(arr, toAny(e))
		}
		return arr
	case *commonpb.AnyValue_KvlistValue:
		m := make(map[string]any, len(x.KvlistValue.GetValues()))
		for _, kv := range x.KvlistValue.GetValues() {
			m[kv.GetKey()] = toAny(kv.GetValue())
		}
		return m
	}
	return nil
}

var _ = tracepb.Span_SPAN_KIND_SERVER // keeps the enum documented next to kindNames
