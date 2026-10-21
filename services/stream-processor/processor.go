
package main

import "time"

// TelemetryEvent represents one recorded request.
type TelemetryEvent struct {
	Service    string `json:"service"`
	StatusCode int    `json:"status_code"`
	DurationMs int    `json:"duration_ms"`
	Timestamp  time.Time `json:"timestamp"`
}

// ServiceMetrics stores aggregated metrics for one service.
type ServiceMetrics struct {
	TotalRequests int
	ServerErrors  int
	TotalDuration int
}

// ProcessEvents calculates metrics separately for each service.
func ProcessEvents(events []TelemetryEvent) map[string]*ServiceMetrics {
	metrics := make(map[string]*ServiceMetrics)

	for _, event := range events {
		// Ignore invalid telemetry.
		if event.Service == "" ||
			event.StatusCode < 100 ||
			event.StatusCode > 599 ||
			event.DurationMs < 0 {
			continue
		}

		// Create a metrics entry when a service appears for the first time.
		if metrics[event.Service] == nil {
			metrics[event.Service] = &ServiceMetrics{}
		}

		m := metrics[event.Service]

		m.TotalRequests++
		m.TotalDuration += event.DurationMs

		if event.StatusCode >= 500 {
			m.ServerErrors++
		}
	}

	return metrics
}


func FilterEventsByWindow(
	events []TelemetryEvent,
	start time.Time,
	window time.Duration,
) []TelemetryEvent {
	var filtered []TelemetryEvent
	end := start.Add(window)

	for _, event := range events {
		// Include start; exclude end.
		if !event.Timestamp.Before(start) &&
			event.Timestamp.Before(end) {
			filtered = append(filtered, event)
		}
	}

	return filtered
}
