package model

import "time"

// ServiceEdge represents an observed caller-to-callee dependency between two services.
type ServiceEdge struct {
	Caller      string    `json:"caller"`
	Callee      string    `json:"callee"`
	CallCount   int64     `json:"callCount"`
	ErrorCount  int64     `json:"errorCount"`
	P50Ms       float64   `json:"p50Ms"`
	P95Ms       float64   `json:"p95Ms"`
	P99Ms       float64   `json:"p99Ms"`
	WindowStart time.Time `json:"windowStart,omitempty"`
}

// TopologyGraph represents the service dependency network graph.
type TopologyGraph struct {
	Edges    []*ServiceEdge `json:"edges"`
	Services []string       `json:"services"`
}
