package natsio

import (
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/model"
)

func TestDecodeOTLPLogs_Protobuf(t *testing.T) {
	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "payment-gateway"}}},
					},
				},
				ScopeLogs: []*logspb.ScopeLogs{
					{
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano:   uint64(time.Now().UnixNano()),
								SeverityText:   "ERROR",
								SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_ERROR,
								Body:           &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "charge failed: card declined"}},
								TraceId:        []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
								SpanId:         []byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
							},
						},
					},
				},
			},
		},
	}

	protoBytes, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal OTLP protobuf: %v", err)
	}

	records, err := decodeOTLPLogs(protoBytes)
	if err != nil {
		t.Fatalf("decodeOTLPLogs failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	r := records[0]
	if r.ServiceName != "payment-gateway" {
		t.Errorf("expected service payment-gateway, got %s", r.ServiceName)
	}
	if r.Severity != "ERROR" {
		t.Errorf("expected severity ERROR, got %s", r.Severity)
	}
	if r.Body != "charge failed: card declined" {
		t.Errorf("expected body 'charge failed: card declined', got '%s'", r.Body)
	}
	if r.TraceID != "0102030405060708090a0b0c0d0e0f10" {
		t.Errorf("expected hex traceId, got %s", r.TraceID)
	}
	if r.SpanID != "1112131415161718" {
		t.Errorf("expected hex spanId, got %s", r.SpanID)
	}
}

func TestDecodeOTLPLogs_JSONFallback(t *testing.T) {
	record := model.LogRecord{
		ServiceName: "auth-service",
		Severity:    "WARN",
		Body:        "invalid token attempt",
		TraceID:     "abc12345",
	}

	jsonBytes, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}

	records, err := decodeOTLPLogs(jsonBytes)
	if err != nil {
		t.Fatalf("decodeOTLPLogs JSON fallback failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ServiceName != "auth-service" {
		t.Errorf("expected auth-service, got %s", records[0].ServiceName)
	}
	if records[0].Severity != "WARN" {
		t.Errorf("expected WARN, got %s", records[0].Severity)
	}
}
