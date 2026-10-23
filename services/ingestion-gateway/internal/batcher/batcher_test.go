package batcher

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakePub struct {
	mu      sync.Mutex
	batches []Batch
	err     error
	gate    chan struct{} // if set, Publish blocks until it is closed
}

func (f *fakePub) Publish(ctx context.Context, b Batch) error {
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batches = append(f.batches, b)
	return f.err
}

func (f *fakePub) snapshot() []Batch {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Batch(nil), f.batches...)
}

func cfg() Config {
	return Config{
		Subject: "telemetry.traces", TargetBytes: 100, MaxBytes: 200,
		MaxDelay: 20 * time.Millisecond, MaxInflightBytes: 1000, PublishTimeout: time.Second,
	}
}

func mustNew(t *testing.T, c Config, p Publisher) *Batcher {
	t.Helper()
	b, err := New(c, p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFlushOnDelayCoalescesConcurrentSubmits(t *testing.T) {
	pub := &fakePub{}
	b := mustNew(t, cfg(), pub)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := b.Submit(context.Background(), "acme", []byte("0123456789")); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got := pub.snapshot()
	total := 0
	for _, bt := range got {
		if bt.Tenant != "acme" || bt.Subject != "telemetry.traces" {
			t.Fatalf("bad batch metadata: %+v", bt)
		}
		total += len(bt.Payload)
	}
	if total != 50 {
		t.Fatalf("lost bytes: got %d want 50", total)
	}
	if len(got) >= 5 {
		t.Fatalf("expected coalescing, got %d batches for 5 submits", len(got))
	}
}

func TestFlushOnTargetSize(t *testing.T) {
	pub := &fakePub{}
	c := cfg()
	c.MaxDelay = time.Hour // only size can trigger the flush
	b := mustNew(t, c, pub)
	if err := b.Submit(context.Background(), "acme", bytes.Repeat([]byte("x"), 100)); err != nil {
		t.Fatal(err)
	}
	if got := pub.snapshot(); len(got) != 1 || got[0].Items != 1 {
		t.Fatalf("unexpected batches: %+v", got)
	}
}

func TestBatchesNeverMixTenantsOrExceedMax(t *testing.T) {
	pub := &fakePub{}
	b := mustNew(t, cfg(), pub)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		tenant := "a"
		if i%2 == 1 {
			tenant = "b"
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Submit(context.Background(), tenant, bytes.Repeat([]byte(tenant), 90))
		}()
	}
	wg.Wait()
	for _, bt := range pub.snapshot() {
		if len(bt.Payload) > 200 {
			t.Fatalf("batch of %d bytes exceeds MaxBytes", len(bt.Payload))
		}
		if bytes.Count(bt.Payload, []byte(bt.Tenant)) != len(bt.Payload) {
			t.Fatalf("batch for %q contains other tenant's data", bt.Tenant)
		}
	}
}

func TestTooLarge(t *testing.T) {
	b := mustNew(t, cfg(), &fakePub{})
	err := b.Submit(context.Background(), "acme", make([]byte, 201))
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v", err)
	}
	if b.Inflight() != 0 {
		t.Fatalf("inflight leaked: %d", b.Inflight())
	}
}

func TestOverloadedWhenInflightLimitReached(t *testing.T) {
	pub := &fakePub{gate: make(chan struct{})}
	c := cfg()
	c.MaxInflightBytes = 200
	c.TargetBytes = 100
	b := mustNew(t, c, pub)

	errc := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { errc <- b.Submit(context.Background(), "acme", bytes.Repeat([]byte("x"), 100)) }()
	}
	deadline := time.Now().Add(time.Second)
	for b.Inflight() < 200 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if err := b.Submit(context.Background(), "acme", []byte("y")); !errors.Is(err, ErrOverloaded) {
		t.Fatalf("want ErrOverloaded, got %v", err)
	}
	close(pub.gate)
	for i := 0; i < 2; i++ {
		if err := <-errc; err != nil {
			t.Fatal(err)
		}
	}
	if b.Inflight() != 0 {
		t.Fatalf("inflight not released: %d", b.Inflight())
	}
}

func TestPublishErrorReachesEveryWaiter(t *testing.T) {
	pub := &fakePub{err: ErrUnavailable}
	b := mustNew(t, cfg(), pub)
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := b.Submit(context.Background(), "acme", []byte("x")); !errors.Is(err, ErrUnavailable) {
				t.Errorf("got %v", err)
			}
		}()
	}
	wg.Wait()
	if b.Inflight() != 0 {
		t.Fatalf("inflight not released after failure: %d", b.Inflight())
	}
}

func TestContextCancelStopsWaitingButBatchStillPublishes(t *testing.T) {
	pub := &fakePub{}
	b := mustNew(t, cfg(), pub)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.Submit(ctx, "acme", []byte("x")); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if err := b.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(pub.snapshot()) != 1 {
		t.Fatal("batch should still have been published")
	}
}

func TestCloseFlushesAndRejectsNewWork(t *testing.T) {
	pub := &fakePub{}
	c := cfg()
	c.MaxDelay = time.Hour
	b := mustNew(t, c, pub)
	done := make(chan error, 1)
	go func() { done <- b.Submit(context.Background(), "acme", []byte("x")) }()
	time.Sleep(20 * time.Millisecond)
	if err := b.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := b.Submit(context.Background(), "acme", []byte("y")); !errors.Is(err, ErrClosed) {
		t.Fatalf("got %v", err)
	}
}

func TestEmptyPayloadIsNoop(t *testing.T) {
	pub := &fakePub{}
	b := mustNew(t, cfg(), pub)
	if err := b.Submit(context.Background(), "acme", nil); err != nil {
		t.Fatal(err)
	}
	_ = b.Close(context.Background())
	if len(pub.snapshot()) != 0 {
		t.Fatal("empty payload must not publish")
	}
}

func TestConfigValidation(t *testing.T) {
	bad := cfg()
	bad.TargetBytes = 300
	if _, err := New(bad, &fakePub{}); err == nil {
		t.Fatal("expected validation error")
	}
}
