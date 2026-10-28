package model

import "time"

// Anomaly represents a detected statistical aberration in service metrics.
type Anomaly struct {
	ID                 string    `json:"id"`
	Service            string    `json:"service"`
	Operation          string    `json:"operation,omitempty"`
	Metric             string    `json:"metric"`
	BaselineValue      float64   `json:"baselineValue"`
	ObservedValue      float64   `json:"observedValue"`
	ZScore             float64   `json:"zScore"`
	WindowStart        time.Time `json:"windowStart"`
	WindowEnd          time.Time `json:"windowEnd,omitempty"`
	SuspectedRootCause string    `json:"suspectedRootCause,omitempty"`
	Evidence           string    `json:"evidence,omitempty"`
	RelatedSpans       []string  `json:"relatedSpans,omitempty"`
	Status             string    `json:"status"` // "OPEN" or "RESOLVED"
}

// AnomalyRange specifies time bounds for anomaly queries.
type AnomalyRange struct {
	From  time.Time `json:"from"`
	To    time.Time `json:"to"`
	Limit int       `json:"limit"`
}
