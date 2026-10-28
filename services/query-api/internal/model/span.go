// Package model defines domain models for QueryAPI following the Eye of Agamotto architecture.
package model

import (
	"time"
)

// SpanKind represents the OpenTelemetry span kind.
type SpanKind string

const (
	SpanKindUnspecified SpanKind = "UNSPECIFIED"
	SpanKindInternal    SpanKind = "INTERNAL"
	SpanKindServer      SpanKind = "SERVER"
	SpanKindClient      SpanKind = "CLIENT"
	SpanKindProducer    SpanKind = "PRODUCER"
	SpanKindConsumer    SpanKind = "CONSUMER"
)

// StatusCode represents the span status code.
type StatusCode string

const (
	StatusCodeUnset StatusCode = "UNSET"
	StatusCodeOK    StatusCode = "OK"
	StatusCodeError StatusCode = "ERROR"
)

// Span represents an individual unit of work in a distributed trace.
type Span struct {
	TraceID            string            `json:"traceId"`
	SpanID             string            `json:"spanId"`
	ParentSpanID       string            `json:"parentSpanId"`
	ServiceName        string            `json:"serviceName"`
	Name               string            `json:"name"`
	Kind               SpanKind          `json:"kind"`
	StartTime          time.Time         `json:"startTime"`
	DurationNs         uint64            `json:"durationNs"`
	StatusCode         StatusCode        `json:"statusCode"`
	StatusMessage      string            `json:"statusMessage,omitempty"`
	HTTPStatus         int               `json:"httpStatus,omitempty"`
	Attributes         map[string]string `json:"attributes,omitempty"`
	ResourceAttributes map[string]string `json:"resourceAttributes,omitempty"`
	IsError            bool              `json:"isError"`
}

// IsRoot returns true if this span is a root span (has no parent span ID).
func (s *Span) IsRoot() bool {
	return s.ParentSpanID == ""
}

// HasError returns true if this span encountered an error.
func (s *Span) HasError() bool {
	return s.StatusCode == StatusCodeError || s.HTTPStatus >= 400 || s.IsError
}

// SpanNode wraps a Span in a hierarchical tree representation with its child spans.
type SpanNode struct {
	Span     *Span       `json:"span"`
	Children []*SpanNode `json:"children"`
	Depth    int         `json:"depth"`
}

// GetDepth returns the depth level of this node in the span tree (0 for root).
func (n *SpanNode) GetDepth() int {
	if n == nil {
		return 0
	}
	return n.Depth
}
