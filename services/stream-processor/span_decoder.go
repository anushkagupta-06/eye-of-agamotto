package main

import (
	"encoding/hex"
	"fmt"
	"time"

	tracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

// SpanEvent is our internal representation of an OTLP span.
type SpanEvent struct {
	Service        string
	TraceID        string
	SpanID         string
	ParentSpanID   string
	SpanName       string
	SpanKind       string
	StartTime      time.Time
	EndTime        time.Time
	DurationNs     int64
	HTTPStatusCode int
	OTelStatusCode int32
}

// DecodeOTLPTraces converts an OTLP protobuf message into SpanEvents.
func DecodeOTLPTraces(data []byte) ([]SpanEvent, error) {
	request := &tracev1.ExportTraceServiceRequest{}

	if err := proto.Unmarshal(data, request); err != nil {
		return nil, fmt.Errorf("decode OTLP trace payload: %w", err)
	}

	var events []SpanEvent

	// One request can contain multiple resources and spans.
	for _, resourceSpans := range request.GetResourceSpans() {
		service := getServiceName(resourceSpans.GetResource())

		for _, scopeSpans := range resourceSpans.GetScopeSpans() {
			for _, span := range scopeSpans.GetSpans() {
				startNs := span.GetStartTimeUnixNano()
				endNs := span.GetEndTimeUnixNano()

				durationNs := int64(0)
				if endNs >= startNs {
					durationNs = int64(endNs - startNs)
				}

				events = append(events, SpanEvent{
					Service:        service,
					TraceID:        idToHex(span.GetTraceId()),
					SpanID:         idToHex(span.GetSpanId()),
					ParentSpanID:   idToHex(span.GetParentSpanId()),
					SpanName:       span.GetName(),
					SpanKind:       span.GetKind().String(),
					StartTime:      time.Unix(0, int64(startNs)).UTC(),
					EndTime:        time.Unix(0, int64(endNs)).UTC(),
					DurationNs:     durationNs,
					HTTPStatusCode: getHTTPStatusCode(span.GetAttributes()),
					OTelStatusCode: int32(span.GetStatus().GetCode()),
				})
			}
		}
	}

	return events, nil
}

// getServiceName reads service.name from the resource attributes.
func getServiceName(resource *resourcev1.Resource) string {
	for _, attr := range resource.GetAttributes() {
		if attr.GetKey() == "service.name" {
			if value := attr.GetValue().GetStringValue(); value != "" {
				return value
			}
		}
	}

	return "unknown_service"
}

// idToHex returns a lowercase hexadecimal ID.
// Missing or all-zero IDs become an empty string.
func idToHex(id []byte) string {
	allZero := true

	for _, b := range id {
		if b != 0 {
			allZero = false
			break
		}
	}

	if len(id) == 0 || allZero {
		return ""
	}

	return hex.EncodeToString(id)
}

// getHTTPStatusCode supports both attribute names from the team contract.
func getHTTPStatusCode(attributes []*commonv1.KeyValue) int {
	for _, key := range []string{
		"http.response.status_code",
		"http.status_code",
	} {
		for _, attr := range attributes {
			if attr.GetKey() != key {
				continue
			}

			value := attr.GetValue()
			if value == nil {
				continue
			}

			if code := value.GetIntValue(); code >= 100 && code <= 599 {
				return int(code)
			}

			// Some instrumentation may encode numeric attributes as strings.
			var code int
			if _, err := fmt.Sscanf(value.GetStringValue(), "%d", &code); err == nil &&
				code >= 100 && code <= 599 {
				return code
			}
		}
	}

	return 0
}
