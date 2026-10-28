package main

import (
	"sync"
	"time"
)

type bufferedSpan struct {
	span      SpanEvent
	expiresAt time.Time
}

// EdgeCorrelator correlates cross-service spans across different NATS messages.
// It handles:
// 1. Spans arriving in the same message.
// 2. Child arriving before parent (split messages).
// 3. Parent arriving before child (split messages).
// 4. Redelivery / duplicate span events (idempotent edge generation).
// 5. Automatic TTL expiration of unmatched spans to keep memory bounded.
type EdgeCorrelator struct {
	mu              sync.Mutex
	ttl             time.Duration
	spansByID       map[string]bufferedSpan   // key: traceID:spanID -> parent candidate
	waitingChildren map[string][]bufferedSpan // key: traceID:parentSpanID -> waiting server spans
	emittedEdges    map[string]time.Time      // key: tenant:traceID:parentID:childID -> expiresAt (dedup)
}

// NewEdgeCorrelator initializes a thread-safe correlator with a sliding TTL.
func NewEdgeCorrelator(ttl time.Duration) *EdgeCorrelator {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	return &EdgeCorrelator{
		ttl:             ttl,
		spansByID:       make(map[string]bufferedSpan),
		waitingChildren: make(map[string][]bufferedSpan),
		emittedEdges:    make(map[string]time.Time),
	}
}

// Correlate ingests a span batch and returns newly derived cross-service EdgeEvents.
func (c *EdgeCorrelator) Correlate(tenantID string, spans []SpanEvent) []EdgeEvent {
	if tenantID == "" || len(spans) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.gc(now)

	var edges []EdgeEvent

	// Step 1: Index all spans in spansByID and match any previously waiting children
	for _, span := range spans {
		if span.TraceID == "" || span.SpanID == "" || span.Service == "" {
			continue
		}

		key := spanKey(span.TraceID, span.SpanID)
		c.spansByID[key] = bufferedSpan{
			span:      span,
			expiresAt: now.Add(c.ttl),
		}

		// Check if any child spans arrived earlier and are waiting for this parent
		if waitingList, exists := c.waitingChildren[key]; exists {
			for _, w := range waitingList {
				if w.span.Service != span.Service {
					dedupKey := tenantID + ":" + w.span.TraceID + ":" + span.SpanID + ":" + w.span.SpanID
					if _, alreadyEmitted := c.emittedEdges[dedupKey]; !alreadyEmitted {
						c.emittedEdges[dedupKey] = now.Add(c.ttl)
						edges = append(edges, makeEdgeEvent(tenantID, span.Service, w.span))
					}
				}
			}
			delete(c.waitingChildren, key)
		}
	}

	// Step 2: For every SERVER span with a parent, correlate with parent span
	for _, child := range spans {
		if child.SpanKind != "SPAN_KIND_SERVER" || child.TraceID == "" || child.ParentSpanID == "" || child.Service == "" {
			continue
		}

		parentKey := spanKey(child.TraceID, child.ParentSpanID)
		if parent, found := c.spansByID[parentKey]; found {
			if parent.span.Service != "" && parent.span.Service != child.Service {
				dedupKey := tenantID + ":" + child.TraceID + ":" + child.ParentSpanID + ":" + child.SpanID
				if _, alreadyEmitted := c.emittedEdges[dedupKey]; !alreadyEmitted {
					c.emittedEdges[dedupKey] = now.Add(c.ttl)
					edges = append(edges, makeEdgeEvent(tenantID, parent.span.Service, child))
				}
			}
		} else {
			// Parent has not arrived yet. Buffer child to await parent arrival.
			c.waitingChildren[parentKey] = append(c.waitingChildren[parentKey], bufferedSpan{
				span:      child,
				expiresAt: now.Add(c.ttl),
			})
		}
	}

	return edges
}

func makeEdgeEvent(tenantID, callerService string, child SpanEvent) EdgeEvent {
	var isError uint8
	if child.OTelStatusCode == 2 || (child.HTTPStatusCode >= 500 && child.HTTPStatusCode <= 599) {
		isError = 1
	}
	return EdgeEvent{
		TenantID:   tenantID,
		Caller:     callerService,
		Callee:     child.Service,
		Timestamp:  child.StartTime,
		DurationNs: uint64(child.DurationNs),
		IsError:    isError,
	}
}

func (c *EdgeCorrelator) gc(now time.Time) {
	for k, v := range c.spansByID {
		if now.After(v.expiresAt) {
			delete(c.spansByID, k)
		}
	}
	for k, list := range c.waitingChildren {
		var active []bufferedSpan
		for _, item := range list {
			if !now.After(item.expiresAt) {
				active = append(active, item)
			}
		}
		if len(active) == 0 {
			delete(c.waitingChildren, k)
		} else {
			c.waitingChildren[k] = active
		}
	}
	for k, exp := range c.emittedEdges {
		if now.After(exp) {
			delete(c.emittedEdges, k)
		}
	}
}
