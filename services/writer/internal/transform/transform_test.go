package transform

import (
	"bytes"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func str(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}
func num(k string, v int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}}
}

func TestSpansMapping(t *testing.T) {
	traceID := bytes.Repeat([]byte{0xab}, 16)
	spanID := bytes.Repeat([]byte{0x01}, 8)
	req := &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{str("service.name", "checkout"), str("k8s.pod.name", "pod-1")}},
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{
			{
				TraceId: traceID, SpanId: spanID, Name: "GET /pay", Kind: tracepb.Span_SPAN_KIND_SERVER,
				StartTimeUnixNano: 1_700_000_000_000_000_000, EndTimeUnixNano: 1_700_000_000_250_000_000,
				Status:     &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: "boom"},
				Attributes: []*commonpb.KeyValue{num("http.response.status_code", 502), str("user_id", "u1")},
			},
			{ // child with end before start, no status, legacy http attribute
				TraceId: traceID, SpanId: bytes.Repeat([]byte{0x02}, 8), ParentSpanId: spanID, Name: "db",
				StartTimeUnixNano: 2000, EndTimeUnixNano: 1000,
				Attributes: []*commonpb.KeyValue{str("http.status_code", "200")},
			},
		}}},
	}}}

	rows := Spans("acme", req)
	if len(rows) != 2 {
		t.Fatalf("got %d rows", len(rows))
	}
	a, b := rows[0], rows[1]
	if a.TenantID != "acme" || a.ServiceName != "checkout" || a.Name != "GET /pay" || a.Kind != "SERVER" {
		t.Errorf("row a: %+v", a)
	}
	if a.TraceID != "abababababababababababababababab" || a.SpanID != "0101010101010101" || a.ParentSpanID != "" {
		t.Errorf("ids: %q %q %q", a.TraceID, a.SpanID, a.ParentSpanID)
	}
	if a.DurationNs != 250_000_000 || a.StatusCode != "ERROR" || a.StatusMessage != "boom" || a.HTTPStatusCode != 502 {
		t.Errorf("row a values: %+v", a)
	}
	if !a.StartTime.Equal(time.Unix(1_700_000_000, 0).UTC()) {
		t.Errorf("start time: %v", a.StartTime)
	}
	if a.Attributes["user_id"] != "u1" || a.ResourceAttributes["k8s.pod.name"] != "pod-1" {
		t.Errorf("attributes: %v / %v", a.Attributes, a.ResourceAttributes)
	}
	if b.ParentSpanID != "0101010101010101" || b.DurationNs != 0 || b.StatusCode != "UNSET" || b.HTTPStatusCode != 200 || b.Kind != "UNSPECIFIED" {
		t.Errorf("row b: %+v", b)
	}
}

func TestUnknownServiceFallback(t *testing.T) {
	req := &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{TraceId: bytes.Repeat([]byte{1}, 16), SpanId: bytes.Repeat([]byte{1}, 8), StartTimeUnixNano: 1}}}},
	}}}
	if got := Spans("t", req)[0].ServiceName; got != "unknown_service" {
		t.Fatalf("service = %q", got)
	}
}

func TestLogsMappingAndTimestampFallbacks(t *testing.T) {
	received := time.Unix(1_800_000_000, 0).UTC()
	req := &collogspb.ExportLogsServiceRequest{ResourceLogs: []*logspb.ResourceLogs{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{str("service.name", "payment")}},
		ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{
			{TimeUnixNano: 5_000_000_000, SeverityText: "ERROR", SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_ERROR,
				Body:    &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "db timeout"}},
				TraceId: bytes.Repeat([]byte{0xcd}, 16), SpanId: bytes.Repeat([]byte{0x03}, 8),
				Attributes: []*commonpb.KeyValue{str("pod", "p1")}},
			{ObservedTimeUnixNano: 7_000_000_000},
			{},
		}}},
	}}}
	rows := Logs("acme", received, req)
	if len(rows) != 3 {
		t.Fatalf("got %d rows", len(rows))
	}
	if rows[0].Body != "db timeout" || rows[0].SeverityNumber != 17 || rows[0].ServiceName != "payment" ||
		rows[0].TraceID != "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd" || rows[0].Attributes["pod"] != "p1" {
		t.Errorf("row 0: %+v", rows[0])
	}
	if !rows[0].Timestamp.Equal(time.Unix(5, 0).UTC()) || !rows[1].Timestamp.Equal(time.Unix(7, 0).UTC()) || !rows[2].Timestamp.Equal(received) {
		t.Errorf("timestamps: %v %v %v", rows[0].Timestamp, rows[1].Timestamp, rows[2].Timestamp)
	}
	if rows[2].TraceID != "" || rows[2].SpanID != "" || rows[2].Body != "" {
		t.Errorf("empty record should have empty ids and body: %+v", rows[2])
	}
}

func TestAnyValueStringification(t *testing.T) {
	cases := map[string]*commonpb.AnyValue{
		"true":          {Value: &commonpb.AnyValue_BoolValue{BoolValue: true}},
		"-42":           {Value: &commonpb.AnyValue_IntValue{IntValue: -42}},
		"1.5":           {Value: &commonpb.AnyValue_DoubleValue{DoubleValue: 1.5}},
		"AQI=":          {Value: &commonpb.AnyValue_BytesValue{BytesValue: []byte{1, 2}}},
		"":              nil,
		"[1,\"a\"]":     {Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: []*commonpb.AnyValue{{Value: &commonpb.AnyValue_IntValue{IntValue: 1}}, {Value: &commonpb.AnyValue_StringValue{StringValue: "a"}}}}}},
		"{\"k\":\"v\"}": {Value: &commonpb.AnyValue_KvlistValue{KvlistValue: &commonpb.KeyValueList{Values: []*commonpb.KeyValue{str("k", "v")}}}},
	}
	for want, in := range cases {
		if got := anyString(in); got != want {
			t.Errorf("anyString = %q, want %q", got, want)
		}
	}
}

func TestDecodeRoundTripAndGarbage(t *testing.T) {
	req := &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{TraceId: bytes.Repeat([]byte{1}, 16), SpanId: bytes.Repeat([]byte{1}, 8), StartTimeUnixNano: 1}}}},
	}}}
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := DecodeSpans("acme", time.Now(), data)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	if _, err := DecodeSpans("acme", time.Now(), []byte{0xff, 0xff, 0xff}); err == nil {
		t.Fatal("garbage must fail to decode")
	}
	if _, err := DecodeLogs("acme", time.Now(), []byte{0xff, 0xff, 0xff}); err == nil {
		t.Fatal("garbage must fail to decode")
	}
}

func TestDurationAndSeverityClamping(t *testing.T) {
	if duration(10, 5) != 0 || duration(10, 0) != 0 || duration(10, 15) != 5 {
		t.Fatal("duration clamping broken")
	}
	if severityNumber(-1) != 0 || severityNumber(25) != 0 || severityNumber(24) != 24 {
		t.Fatal("severity clamping broken")
	}
}
