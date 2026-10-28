package model

import "time"

// TraceFilters defines parameters for searching distributed traces.
type TraceFilters struct {
	Service       string        `json:"service,omitempty"`
	Operation     string        `json:"operation,omitempty"`
	MinDurationNs uint64        `json:"minDurationNs,omitempty"`
	Status        string        `json:"status,omitempty"` // "ERROR", "OK"
	From          time.Time     `json:"from"`
	To            time.Time     `json:"to"`
	Limit         int           `json:"limit"`
	Offset        int           `json:"offset,omitempty"`
}
