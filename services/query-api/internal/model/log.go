package model

import "time"

// LogRecord represents an individual log line correlated with a service and trace.
type LogRecord struct {
	Timestamp          time.Time         `json:"timestamp"`
	ServiceName        string            `json:"serviceName"`
	Severity           string            `json:"severity"`
	SeverityNumber     uint8             `json:"severityNumber"`
	Body               string            `json:"body"`
	TraceID            string            `json:"traceId,omitempty"`
	SpanID             string            `json:"spanId,omitempty"`
	Attributes         map[string]string `json:"attributes,omitempty"`
	ResourceAttributes map[string]string `json:"resourceAttributes,omitempty"`
}

// LogQuery defines filtering parameters for searching logs efficiently.
type LogQuery struct {
	Query      string            `json:"query,omitempty"`    // Full-text token search on body (hasToken)
	Service    string            `json:"service,omitempty"`  // Primary-key prefix filter
	Severity   string            `json:"severity,omitempty"` // Minimum severity level (e.g. "ERROR", "WARN")
	TraceID    string            `json:"traceId,omitempty"`  // Exact trace correlation
	SpanID     string            `json:"spanId,omitempty"`   // Exact span correlation
	Attributes map[string]string `json:"attributes,omitempty"`
	From       time.Time         `json:"from"`
	To         time.Time         `json:"to"`
	Limit      int               `json:"limit"`
}

// LogTailFilters defines real-time filter criteria for tailLogs streaming.
type LogTailFilters struct {
	Service     string `json:"service,omitempty"`
	Severity    string `json:"severity,omitempty"`
	Query       string `json:"query,omitempty"`
	BatchBuffer int    `json:"batchBuffer,omitempty"`
}
