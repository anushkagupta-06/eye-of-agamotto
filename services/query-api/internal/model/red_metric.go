package model

// REDMetric represents Rate, Errors, and Duration telemetry for a service operation.
type REDMetric struct {
	ServiceName string  `json:"serviceName"`
	Operation   string  `json:"operation"`
	CallsPerMin int64   `json:"callsPerMin"`
	ErrorCount  int64   `json:"errorCount"`
	ErrorRate   float64 `json:"errorRate"` // Percentage (0.0 - 100.0)
	P50Ms       float64 `json:"p50Ms"`
	P95Ms       float64 `json:"p95Ms"`
	P99Ms       float64 `json:"p99Ms"`
	Status      string  `json:"status"` // "HEALTHY", "DEGRADED", "CRITICAL"
}
