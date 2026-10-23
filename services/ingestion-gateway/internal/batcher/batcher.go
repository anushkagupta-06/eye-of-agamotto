// Package batcher coalesces concurrent ingest requests into fewer, larger queue messages
// ("group commit") without weakening durability.
//
// Submit blocks until the batch containing the caller's payload has been acknowledged by the
// Publisher, so a client is only told "success" once its data is durable in the queue.
// Payloads are OTLP protobuf messages; concatenating serialized protobuf messages of the same
// type is equivalent to merging them (repeated fields append), so batching is a byte append.
//
// Memory is bounded: Submit rejects with ErrOverloaded once MaxInflightBytes of payload is
// waiting to be published.
package batcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrOverloaded means the gateway or the queue cannot take more data right now (maps to 429).
	ErrOverloaded = errors.New("batcher: overloaded")
	// ErrUnavailable means the queue could not be reached (maps to 503).
	ErrUnavailable = errors.New("batcher: queue unavailable")
	// ErrTooLarge means a single payload exceeds the maximum message size (maps to 413).
	ErrTooLarge = errors.New("batcher: payload exceeds maximum message size")
	// ErrClosed means the batcher is shutting down (maps to 503).
	ErrClosed = errors.New("batcher: closed")
)

// Batch is one queue message ready to be published.
type Batch struct {
	Subject  string
	Tenant   string
	Payload  []byte
	Items    int // number of client requests merged into Payload
	Received time.Time
}

// Publisher durably appends a batch to the queue. It should wrap ErrOverloaded or ErrUnavailable
// so callers can distinguish retryable conditions.
type Publisher interface {
	Publish(ctx context.Context, b Batch) error
}

// Result describes a finished publish attempt, for metrics and logging.
type Result struct {
	Batch
	Duration time.Duration
	Err      error
}

// Observer is called once per publish attempt, from the publishing goroutine.
type Observer func(Result)

// Config controls batching behaviour.
type Config struct {
	Subject          string
	TargetBytes      int           // flush as soon as a batch reaches this size
	MaxBytes         int           // never build a batch larger than this
	MaxDelay         time.Duration // flush a non-empty batch after this long
	MaxInflightBytes int64         // admission limit across open and publishing batches
	PublishTimeout   time.Duration
	Observer         Observer // optional
}

func (c Config) validate() error {
	switch {
	case c.Subject == "":
		return errors.New("batcher: subject is required")
	case c.TargetBytes <= 0 || c.TargetBytes > c.MaxBytes:
		return errors.New("batcher: TargetBytes must be in (0, MaxBytes]")
	case c.MaxDelay <= 0:
		return errors.New("batcher: MaxDelay must be positive")
	case c.MaxInflightBytes < int64(c.MaxBytes):
		return errors.New("batcher: MaxInflightBytes must be >= MaxBytes")
	case c.PublishTimeout <= 0:
		return errors.New("batcher: PublishTimeout must be positive")
	}
	return nil
}

type pending struct {
	tenant   string
	buf      []byte
	items    int
	received time.Time
	timer    *time.Timer
	done     chan struct{}
	err      error // written before done is closed
}

// Batcher groups payloads per tenant, because every queue message carries exactly one tenant.
type Batcher struct {
	cfg Config
	pub Publisher

	inflight atomic.Int64

	mu     sync.Mutex
	open   map[string]*pending
	closed bool
	wg     sync.WaitGroup
}

// New validates cfg and returns a Batcher.
func New(cfg Config, pub Publisher) (*Batcher, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if pub == nil {
		return nil, errors.New("batcher: publisher is required")
	}
	return &Batcher{cfg: cfg, pub: pub, open: make(map[string]*pending)}, nil
}

// Inflight returns the bytes currently admitted but not yet acknowledged by the queue.
func (b *Batcher) Inflight() int64 { return b.inflight.Load() }

// Submit enqueues payload for tenant and waits until it is durably published or fails.
// An empty payload is a no-op. The payload is copied, so the caller may reuse it.
func (b *Batcher) Submit(ctx context.Context, tenant string, payload []byte) error {
	n := int64(len(payload))
	if n == 0 {
		return nil
	}
	if len(payload) > b.cfg.MaxBytes {
		return fmt.Errorf("%w: %d > %d bytes", ErrTooLarge, len(payload), b.cfg.MaxBytes)
	}
	if b.inflight.Add(n) > b.cfg.MaxInflightBytes {
		b.inflight.Add(-n)
		return ErrOverloaded
	}
	p, err := b.add(tenant, payload)
	if err != nil {
		b.inflight.Add(-n)
		return err
	}
	select {
	case <-p.done:
		return p.err
	case <-ctx.Done():
		// The batch is still published; only this caller stops waiting.
		return ctx.Err()
	}
}

func (b *Batcher) add(tenant string, payload []byte) (*pending, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, ErrClosed
	}
	p := b.open[tenant]
	if p != nil && len(p.buf)+len(payload) > b.cfg.MaxBytes {
		delete(b.open, tenant)
		b.flushLocked(p)
		p = nil
	}
	if p == nil {
		p = &pending{
			tenant:   tenant,
			buf:      make([]byte, 0, len(payload)),
			received: time.Now(),
			done:     make(chan struct{}),
		}
		b.open[tenant] = p
		// Assigned while holding mu; the timer callback also takes mu, so it cannot observe nil.
		p.timer = time.AfterFunc(b.cfg.MaxDelay, func() { b.onTimer(tenant, p) })
	}
	p.buf = append(p.buf, payload...)
	p.items++
	if len(p.buf) >= b.cfg.TargetBytes {
		delete(b.open, tenant)
		b.flushLocked(p)
	}
	return p, nil
}

func (b *Batcher) onTimer(tenant string, p *pending) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.open[tenant] == p {
		delete(b.open, tenant)
		b.flushLocked(p)
	}
}

// flushLocked must be called with b.mu held and p already removed from b.open.
func (b *Batcher) flushLocked(p *pending) {
	p.timer.Stop()
	b.wg.Add(1)
	go b.publish(p)
}

func (b *Batcher) publish(p *pending) {
	defer b.wg.Done()
	ctx, cancel := context.WithTimeout(context.Background(), b.cfg.PublishTimeout)
	defer cancel()

	batch := Batch{Subject: b.cfg.Subject, Tenant: p.tenant, Payload: p.buf, Items: p.items, Received: p.received}
	start := time.Now()
	err := b.pub.Publish(ctx, batch)

	p.err = err
	b.inflight.Add(-int64(len(p.buf)))
	if b.cfg.Observer != nil {
		b.cfg.Observer(Result{Batch: batch, Duration: time.Since(start), Err: err})
	}
	close(p.done)
}

// Close stops accepting payloads, flushes open batches and waits for in-flight publishes
// until ctx expires.
func (b *Batcher) Close(ctx context.Context) error {
	b.mu.Lock()
	if !b.closed {
		b.closed = true
		for tenant, p := range b.open {
			delete(b.open, tenant)
			b.flushLocked(p)
		}
	}
	b.mu.Unlock()

	drained := make(chan struct{})
	go func() { b.wg.Wait(); close(drained) }()
	select {
	case <-drained:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
