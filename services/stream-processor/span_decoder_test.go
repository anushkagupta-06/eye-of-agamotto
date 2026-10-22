package main

import (
	"testing"
	"time"

	tracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestDecodeOTLPTraces(t *testing.T) {
	start := time.Date(
		2026, time.October, 21,
		10, 0, 0, 0, time.UTC,
	)
	end := start.Add(250 * time.Millisecond)

	request := &tracev1.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcev1.Resource{
					Attributes: []*commonv1.KeyValue{
						{
							Key: "service.name",
							Value: &commonv1.AnyValue{
								Value: &commonv1.AnyValue_StringValue{
									StringValue: "order-service",
								},
							},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           make([]byte, 16),
								SpanId:            []byte{1, 2, 3, 4, 5, 6, 7, 8},
								Name:              "POST /orders",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: uint64(start.UnixNano()),
								EndTimeUnixNano:   uint64(end.UnixNano()),
								Attributes: []*commonv1.KeyValue{
									{
										Key: "http.response.status_code",
										Value: &commonv1.AnyValue{
											Value: &commonv1.AnyValue_IntValue{
												IntValue: 500,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	payload, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal test request: %v", err)
	}

	got, err := DecodeOTLPTraces(payload)
	if err != nil {
		t.Fatalf("DecodeOTLPTraces returned error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 span, got %d", len(got))
	}

	span := got[0]

	if span.Service != "order-service" {
		t.Errorf("expected order-service, got %q", span.Service)
	}

	if span.SpanName != "POST /orders" {
		t.Errorf("unexpected span name: %q", span.SpanName)
	}

	if span.HTTPStatusCode != 500 {
		t.Errorf("expected HTTP status 500, got %d", span.HTTPStatusCode)
	}

	if span.DurationNs != 250_000_000 {
		t.Errorf("expected duration 250000000 ns, got %d", span.DurationNs)
	}

	if span.TraceID != "" {
		t.Errorf("expected all-zero trace ID to be empty, got %q", span.TraceID)
	}

	if span.SpanID != "0102030405060708" {
		t.Errorf("unexpected span ID: %q", span.SpanID)
	}

	if span.SpanKind != tracepb.Span_SPAN_KIND_SERVER.String() {
		t.Errorf("unexpected span kind: %q", span.SpanKind)
	}
}

func TestDecodeOTLPTracesRejectsMalformedPayload(t *testing.T) {
	_, err := DecodeOTLPTraces([]byte{0xff, 0xff, 0xff})

	if err == nil {
		t.Fatal("expected malformed protobuf payload to return an error")
	}
}
