package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAnomalyDetector_NormalPeriods(t *testing.T) {
	d := NewAnomalyDetector(3.0)
	now := time.Now().UTC()
	start := now.Truncate(time.Minute)
	end := start.Add(time.Minute)

	// Feed 10 normal observations (around 50ms)
	for i := 0; i < 10; i++ {
		anom := d.EvaluateP99Latency("demo", "checkout", "Pay", 50.0+float64(i%5), start, end, nil)
		if anom != nil {
			t.Fatalf("expected no anomaly during normal period, got: %+v", anom)
		}
	}
}

func TestAnomalyDetector_ThresholdCrossing_Open(t *testing.T) {
	d := NewAnomalyDetector(3.0)
	now := time.Now().UTC()
	start := now.Truncate(time.Minute)
	end := start.Add(time.Minute)

	// Establish baseline: mean ~ 50ms, stddev ~ 15ms
	for i := 0; i < 5; i++ {
		d.EvaluateP99Latency("demo", "auth", "VerifyToken", 50.0, start, end, nil)
	}

	// Trigger spike: 250ms -> (250 - 50) / 15 = 13.3 sigma (>= 3.0)
	failingSpans := []SpanEvent{
		{
			Service:      "auth",
			SpanID:       "span-slow-1",
			ParentSpanID: "root-span",
			DurationNs:   250000000,
		},
	}

	anom := d.EvaluateP99Latency("demo", "auth", "VerifyToken", 250.0, start, end, failingSpans)
	if anom == nil {
		t.Fatalf("expected anomaly to be opened, got nil")
	}

	if anom.Status != "OPEN" {
		t.Errorf("expected status OPEN, got %s", anom.Status)
	}
	if anom.Metric != "p99_latency" {
		t.Errorf("expected metric p99_latency, got %s", anom.Metric)
	}
	if anom.ZScore < 3.0 {
		t.Errorf("expected ZScore >= 3.0, got %f", anom.ZScore)
	}
	if anom.RootCauseService != "auth" {
		t.Errorf("expected root cause auth, got %s", anom.RootCauseService)
	}

	// Verify evidence payload
	var ev AnomalyEvidence
	if err := json.Unmarshal([]byte(anom.Evidence), &ev); err != nil {
		t.Fatalf("invalid evidence JSON: %v", err)
	}
	if len(ev.RelatedSpans) != 1 || ev.RelatedSpans[0] != "span-slow-1" {
		t.Errorf("unexpected related spans in evidence: %+v", ev.RelatedSpans)
	}
	if ev.ZScore < 3.0 {
		t.Errorf("unexpected zscore in evidence: %f", ev.ZScore)
	}
}

func TestAnomalyDetector_RepeatedWindows_Idempotency(t *testing.T) {
	d := NewAnomalyDetector(3.0)
	now := time.Now().UTC()
	start := now.Truncate(time.Minute)
	end := start.Add(time.Minute)

	// Baseline
	d.EvaluateP99Latency("demo", "catalog", "ListItems", 40.0, start, end, nil)

	// Spike window 1
	anom1 := d.EvaluateP99Latency("demo", "catalog", "ListItems", 200.0, start, end, nil)
	if anom1 == nil || anom1.Status != "OPEN" {
		t.Fatalf("expected open anomaly in window 1")
	}

	// Repeated spike window 2
	anom2 := d.EvaluateP99Latency("demo", "catalog", "ListItems", 210.0, start.Add(time.Minute), end.Add(time.Minute), nil)
	if anom2 == nil || anom2.Status != "OPEN" {
		t.Fatalf("expected continued open anomaly in window 2")
	}

	// Deterministic UUID must match for the same tenant:service:metric
	if anom1.ID != anom2.ID {
		t.Errorf("expected identical deterministic UUIDs, got %s vs %s", anom1.ID, anom2.ID)
	}
}

func TestAnomalyDetector_Recovery_Resolved(t *testing.T) {
	d := NewAnomalyDetector(3.0)
	now := time.Now().UTC()
	start := now.Truncate(time.Minute)
	end := start.Add(time.Minute)

	// Baseline
	d.EvaluateP99Latency("demo", "search", "Query", 60.0, start, end, nil)

	// Open anomaly
	anomOpen := d.EvaluateP99Latency("demo", "search", "Query", 300.0, start, end, nil)
	if anomOpen == nil || anomOpen.Status != "OPEN" {
		t.Fatalf("expected anomaly to open")
	}

	// Recovery back to normal (62ms) -> ZScore < 1.5
	anomResolved := d.EvaluateP99Latency("demo", "search", "Query", 62.0, start.Add(time.Minute), end.Add(time.Minute), nil)
	if anomResolved == nil {
		t.Fatalf("expected resolution anomaly event, got nil")
	}
	if anomResolved.Status != "RESOLVED" {
		t.Errorf("expected status RESOLVED, got %s", anomResolved.Status)
	}
	if anomResolved.ID != anomOpen.ID {
		t.Errorf("expected matching ID on resolution, got %s vs %s", anomResolved.ID, anomOpen.ID)
	}

	// Subsequent normal call should produce no event
	anomNext := d.EvaluateP99Latency("demo", "search", "Query", 60.0, start.Add(2*time.Minute), end.Add(2*time.Minute), nil)
	if anomNext != nil {
		t.Errorf("expected no event after resolution, got: %+v", anomNext)
	}
}

func TestAnomalyDetector_ProcessSpans(t *testing.T) {
	d := NewAnomalyDetector(3.0)
	now := time.Now().UTC()

	// Establish baseline
	d.EvaluateP99Latency("demo", "order", "Submit", 50.0, now, now.Add(time.Minute), nil)

	// Process batch with slow spans
	spans := []SpanEvent{
		{
			Service:    "order",
			SpanName:   "Submit",
			SpanID:     "s-slow-1",
			DurationNs: 280000000, // 280ms
		},
		{
			Service:    "order",
			SpanName:   "Submit",
			SpanID:     "s-slow-2",
			DurationNs: 290000000, // 290ms
		},
	}

	anoms := d.ProcessSpans("demo", spans)
	if len(anoms) != 1 {
		t.Fatalf("expected 1 anomaly generated from ProcessSpans, got %d", len(anoms))
	}
	if anoms[0].ServiceName != "order" || anoms[0].Status != "OPEN" {
		t.Errorf("unexpected anomaly: %+v", anoms[0])
	}
}
