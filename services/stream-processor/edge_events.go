package main

import "time"

// EdgeEvent represents one cross-service call for ClickHouse edge_events.
type EdgeEvent struct {
	TenantID   string
	Caller     string
	Callee     string
	Timestamp  time.Time
	DurationNs uint64
	IsError    uint8
}

// ExtractEdgeEvents follows the storage contract:
// a SERVER span whose parent belongs to a different service creates an edge
// from the parent service to the child service.
func ExtractEdgeEvents(tenantID string, spans []SpanEvent) []EdgeEvent {
	if tenantID == "" {
		return nil
	}

	// Index spans by trace ID and span ID so children can find their parents.
	byID := make(map[string]SpanEvent, len(spans))
	for _, span := range spans {
		if span.TraceID == "" || span.SpanID == "" || span.Service == "" {
			continue
		}
		byID[spanKey(span.TraceID, span.SpanID)] = span
	}

	var events []EdgeEvent

	for _, child := range spans {
		if child.SpanKind != "SPAN_KIND_SERVER" ||
			child.TraceID == "" ||
			child.ParentSpanID == "" ||
			child.Service == "" ||
			child.DurationNs < 0 {
			continue
		}

		parent, found := byID[spanKey(child.TraceID, child.ParentSpanID)]
		if !found || parent.Service == "" || parent.Service == child.Service {
			continue
		}

		var isError uint8
		if child.HTTPStatusCode >= 500 && child.HTTPStatusCode <= 599 {
			isError = 1
		}

		events = append(events, EdgeEvent{
			TenantID:   tenantID,
			Caller:     parent.Service,
			Callee:     child.Service,
			Timestamp:  child.StartTime,
			DurationNs: uint64(child.DurationNs),
			IsError:    isError,
		})
	}

	return events
}
