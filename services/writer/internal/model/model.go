// Package model defines the rows the writer inserts into ClickHouse. Field order and meaning
// follow infra/clickhouse/init/001_schema.sql.
package model

import "time"

// Span is one row of observability.spans.
type Span struct {
	TenantID           string
	TraceID            string // 32 lowercase hex chars
	SpanID             string // 16 lowercase hex chars
	ParentSpanID       string // "" for root spans
	ServiceName        string
	Name               string
	Kind               string // UNSPECIFIED, INTERNAL, SERVER, CLIENT, PRODUCER, CONSUMER
	StartTime          time.Time
	DurationNs         uint64
	StatusCode         string // UNSET, OK, ERROR
	StatusMessage      string
	HTTPStatusCode     uint16
	Attributes         map[string]string
	ResourceAttributes map[string]string
}

// Log is one row of observability.logs.
type Log struct {
	TenantID           string
	Timestamp          time.Time
	ServiceName        string
	SeverityText       string
	SeverityNumber     uint8
	Body               string
	TraceID            string // "" when not correlated
	SpanID             string
	Attributes         map[string]string
	ResourceAttributes map[string]string
}
