
package main

import "testing"
import "time"

func TestProcessEvents(t *testing.T) {
	events := []TelemetryEvent{
		{
			Service:    "payment-service",
			StatusCode: 200,
			DurationMs: 100,
		},
		{
			Service:    "payment-service",
			StatusCode: 500,
			DurationMs: 300,
		},
		{
			Service:    "payment-service",
			StatusCode: 200,
			DurationMs: 200,
		},
	}

	metrics := ProcessEvents(events)
	got := metrics["payment-service"]

	if got == nil {
		t.Fatal("expected payment-service metrics")
	}

	if got.TotalRequests != 3 {
		t.Errorf("expected 3 requests, got %d", got.TotalRequests)
	}

	if got.ServerErrors != 1 {
		t.Errorf("expected 1 server error, got %d", got.ServerErrors)
	}

	if got.TotalDuration != 600 {
		t.Errorf("expected total duration 600 ms, got %d", got.TotalDuration)
	}
}


func TestFilterEventsByWindow(t *testing.T) {
	start := time.Date(
		2026, time.October, 21,
		10, 0, 0, 0, time.UTC,
	)

	events := []TelemetryEvent{
		{
			Service: "payment-service",
			StatusCode: 200,
			DurationMs: 100,
			Timestamp: start.Add(-time.Second),
		},
		{
			Service: "payment-service",
			StatusCode: 200,
			DurationMs: 150,
			Timestamp: start,
		},
		{
			Service: "payment-service",
			StatusCode: 500,
			DurationMs: 850,
			Timestamp: start.Add(5 * time.Second),
		},
		{
			Service: "payment-service",
			StatusCode: 200,
			DurationMs: 200,
			Timestamp: start.Add(10 * time.Second),
		},
	}

	got := FilterEventsByWindow(events, start, 10*time.Second)

	if len(got) != 2 {
		t.Fatalf("expected 2 events inside window, got %d", len(got))
	}

	if got[0].Timestamp != start {
		t.Error("expected event at window start to be included")
	}

	if !got[1].Timestamp.Equal(start.Add(5 * time.Second)) {
		t.Error("expected event at 5 seconds to be included")
	}
}
