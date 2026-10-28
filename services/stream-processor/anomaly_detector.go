package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AnomalyEvent models a stored incident in observability.anomalies.
type AnomalyEvent struct {
	TenantID         string
	ID               uuid.UUID
	ServiceName      string
	Operation        string
	Metric           string
	WindowStart      time.Time
	WindowEnd        time.Time
	BaselineValue    float64
	ObservedValue    float64
	ZScore           float64
	RootCauseService string
	Evidence         string
	Status           string // "OPEN" or "RESOLVED"
	UpdatedAt        time.Time
}

type AnomalyEvidence struct {
	Summary       string   `json:"summary"`
	Baseline      float64  `json:"baseline"`
	Observed      float64  `json:"observed"`
	ZScore        float64  `json:"z_score"`
	Threshold     float64  `json:"threshold"`
	SampleCount   int      `json:"sample_count"`
	RelatedSpans  []string `json:"related_spans"`
}

type serviceBaseline struct {
	samples []float64
	mean    float64
	stddev  float64
}

// AnomalyDetector evaluates RED metrics and traces to detect statistically significant latency or error spikes.
type AnomalyDetector struct {
	mu           sync.Mutex
	thresholdZ   float64 // default: 3.0
	baselines    map[string]*serviceBaseline // key: tenant:service:metric
	activeOpen   map[string]uuid.UUID        // key: tenant:service:metric -> deterministic anomaly ID
}

func NewAnomalyDetector(thresholdZ float64) *AnomalyDetector {
	if thresholdZ <= 0 {
		thresholdZ = 3.0
	}
	return &AnomalyDetector{
		thresholdZ: thresholdZ,
		baselines:  make(map[string]*serviceBaseline),
		activeOpen: make(map[string]uuid.UUID),
	}
}

// DeterministicAnomalyID generates a stable UUID for a specific service anomaly.
// This ensures repeated evaluations update the exact same row in ClickHouse ReplacingMergeTree.
func DeterministicAnomalyID(tenantID, serviceName, metric string) uuid.UUID {
	h := sha1.New()
	_, _ = h.Write([]byte(fmt.Sprintf("%s:%s:%s", tenantID, serviceName, metric)))
	u, _ := uuid.FromBytes(h.Sum(nil)[:16])
	return u
}

// EvaluateP99Latency checks a 1-minute window p99 latency against the baseline.
// Returns an AnomalyEvent if an anomaly was opened or resolved, or nil if behavior is normal.
func (d *AnomalyDetector) EvaluateP99Latency(
	tenantID string,
	serviceName string,
	operation string,
	observedP99Ms float64,
	windowStart time.Time,
	windowEnd time.Time,
	failingSpans []SpanEvent,
) *AnomalyEvent {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := tenantID + ":" + serviceName + ":p99_latency"
	b, exists := d.baselines[key]
	if !exists {
		// Seed baseline with typical SLA (e.g. 50ms, stddev 15ms) until historical samples accumulate
		b = &serviceBaseline{
			samples: []float64{observedP99Ms},
			mean:    observedP99Ms,
			stddev:  15.0,
		}
		d.baselines[key] = b
	}

	// Calculate Z-Score: (observed - mean) / stddev
	zScore := 0.0
	if b.stddev > 0 {
		zScore = (observedP99Ms - b.mean) / b.stddev
	}

	anomalyID := DeterministicAnomalyID(tenantID, serviceName, "p99_latency")
	_, isOpen := d.activeOpen[key]

	now := time.Now().UTC()

	// 1. Threshold Crossed -> OPEN ANOMALY
	if zScore >= d.thresholdZ {
		d.activeOpen[key] = anomalyID

		var relatedSpans []string
		rootCause := serviceName
		maxDuration := int64(0)

		for _, s := range failingSpans {
			if s.Service == serviceName || s.ParentSpanID != "" {
				relatedSpans = append(relatedSpans, s.SpanID)
				if s.DurationNs > maxDuration {
					maxDuration = s.DurationNs
					if s.Service != "" {
						rootCause = s.Service
					}
				}
			}
		}

		evidenceObj := AnomalyEvidence{
			Summary:      fmt.Sprintf("P99 latency spiked to %.1fms (baseline: %.1fms, Z-Score: %.2fσ)", observedP99Ms, b.mean, zScore),
			Baseline:     math.Round(b.mean*10) / 10,
			Observed:     math.Round(observedP99Ms*10) / 10,
			ZScore:       math.Round(zScore*100) / 100,
			Threshold:    d.thresholdZ,
			SampleCount:  len(b.samples),
			RelatedSpans: relatedSpans,
		}
		evBytes, _ := json.Marshal(evidenceObj)

		return &AnomalyEvent{
			TenantID:         tenantID,
			ID:               anomalyID,
			ServiceName:      serviceName,
			Operation:        operation,
			Metric:           "p99_latency",
			WindowStart:      windowStart,
			WindowEnd:        windowEnd,
			BaselineValue:    b.mean,
			ObservedValue:    observedP99Ms,
			ZScore:           zScore,
			RootCauseService: rootCause,
			Evidence:         string(evBytes),
			Status:           "OPEN",
			UpdatedAt:        now,
		}
	}

	// 2. Recovery -> RESOLVE ANOMALY
	if isOpen && zScore < 1.5 {
		delete(d.activeOpen, key)

		evidenceObj := AnomalyEvidence{
			Summary:      fmt.Sprintf("P99 latency recovered to %.1fms (baseline: %.1fms, Z-Score: %.2fσ)", observedP99Ms, b.mean, zScore),
			Baseline:     math.Round(b.mean*10) / 10,
			Observed:     math.Round(observedP99Ms*10) / 10,
			ZScore:       math.Round(zScore*100) / 100,
			Threshold:    d.thresholdZ,
			SampleCount:  len(b.samples),
			RelatedSpans: []string{},
		}
		evBytes, _ := json.Marshal(evidenceObj)

		return &AnomalyEvent{
			TenantID:         tenantID,
			ID:               anomalyID,
			ServiceName:      serviceName,
			Operation:        operation,
			Metric:           "p99_latency",
			WindowStart:      windowStart,
			WindowEnd:        windowEnd,
			BaselineValue:    b.mean,
			ObservedValue:    observedP99Ms,
			ZScore:           zScore,
			RootCauseService: serviceName,
			Evidence:         string(evBytes),
			Status:           "RESOLVED",
			UpdatedAt:        now,
		}
	}

	// 3. Normal conditions: incrementally update rolling baseline samples
	if zScore < d.thresholdZ {
		b.samples = append(b.samples, observedP99Ms)
		if len(b.samples) > 60 {
			b.samples = b.samples[1:]
		}
		// Recompute mean and stddev
		sum := 0.0
		for _, v := range b.samples {
			sum += v
		}
		b.mean = sum / float64(len(b.samples))

		sqDiff := 0.0
		for _, v := range b.samples {
			sqDiff += (v - b.mean) * (v - b.mean)
		}
		b.stddev = math.Sqrt(sqDiff / float64(len(b.samples)))
		if b.stddev < 5.0 {
			b.stddev = 5.0 // prevent division by zero or overly sensitive threshold
		}
	}

	return nil
}

// ProcessSpans evaluates latencies across spans in a message batch, detects anomalies, and returns any events that should be persisted.
func (d *AnomalyDetector) ProcessSpans(tenantID string, spans []SpanEvent) []*AnomalyEvent {
	if tenantID == "" || len(spans) == 0 {
		return nil
	}

	type svcOpKey struct {
		service   string
		operation string
	}

	durationsBySvc := make(map[svcOpKey][]float64)
	spansBySvc := make(map[svcOpKey][]SpanEvent)

	for _, span := range spans {
		if span.Service == "" {
			continue
		}
		op := span.SpanName
		if op == "" {
			op = "default"
		}
		key := svcOpKey{service: span.Service, operation: op}
		durMs := float64(span.DurationNs) / 1e6
		durationsBySvc[key] = append(durationsBySvc[key], durMs)
		spansBySvc[key] = append(spansBySvc[key], span)
	}

	now := time.Now().UTC()
	windowStart := now.Truncate(time.Minute)
	windowEnd := windowStart.Add(time.Minute)

	var results []*AnomalyEvent
	for key, durs := range durationsBySvc {
		if len(durs) == 0 {
			continue
		}
		// Sort durations to compute p99
		sort.Float64s(durs)
		idx := int(float64(len(durs)-1) * 0.99)
		p99 := durs[idx]

		anom := d.EvaluateP99Latency(
			tenantID,
			key.service,
			key.operation,
			p99,
			windowStart,
			windowEnd,
			spansBySvc[key],
		)
		if anom != nil {
			results = append(results, anom)
		}
	}

	return results
}

