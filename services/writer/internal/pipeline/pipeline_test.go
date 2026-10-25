package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---- fakes

type fakeMsg struct {
	data      string
	tenant    string
	seq       uint64
	delivered uint64

	mu      sync.Mutex
	acked   bool
	naked   bool
	nakWait time.Duration
	termed  bool
}

func (m *fakeMsg) Data() []byte { return []byte(m.data) }
func (m *fakeMsg) Header(name string) string {
	if name == HeaderTenant {
		return m.tenant
	}
	return ""
}
func (m *fakeMsg) Subject() string   { return "telemetry.test" }
func (m *fakeMsg) Sequence() uint64  { return m.seq }
func (m *fakeMsg) Delivered() uint64 { return m.delivered }
func (m *fakeMsg) Ack() error        { m.mu.Lock(); defer m.mu.Unlock(); m.acked = true; return nil }
func (m *fakeMsg) Term() error       { m.mu.Lock(); defer m.mu.Unlock(); m.termed = true; return nil }
func (m *fakeMsg) Nak(d time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.naked, m.nakWait = true, d
	return nil
}

func msg(seq uint64, data string) *fakeMsg {
	return &fakeMsg{data: data, tenant: "acme", seq: seq, delivered: 1}
}

type fakeSource struct {
	batches [][]Message
	onFetch func()
}

func (s *fakeSource) Fetch(ctx context.Context, max int, wait time.Duration) ([]Message, error) {
	if s.onFetch != nil {
		s.onFetch()
	}
	if len(s.batches) == 0 {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	b := s.batches[0]
	s.batches = s.batches[1:]
	return b, nil
}

type fakeInserter struct {
	mu     sync.Mutex
	calls  [][]string
	tokens []string
	fail   func(rows []string) error
}

func (f *fakeInserter) Insert(ctx context.Context, token string, rows []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string(nil), rows...))
	f.tokens = append(f.tokens, token)
	if f.fail != nil {
		return f.fail(rows)
	}
	return nil
}

type fakeDLQ struct {
	mu   sync.Mutex
	sent []DeadLetter
	err  error
}

func (d *fakeDLQ) Send(ctx context.Context, dl DeadLetter) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.err != nil {
		return d.err
	}
	d.sent = append(d.sent, dl)
	return nil
}

// decode: payload "a,b,c" becomes rows a, b, c; "BAD" is undecodable.
func decode(tenant string, _ time.Time, data []byte) ([]string, error) {
	s := string(data)
	if s == "BAD" {
		return nil, errors.New("cannot decode")
	}
	if s == "" {
		return nil, nil
	}
	return strings.Split(s, ","), nil
}

func newWorker(t *testing.T, src Source, ins Inserter[string], dlq DeadLetterer) *Worker[string] {
	t.Helper()
	w, err := NewWorker[string](Config{
		Signal: "test", MaxMsgs: 10, MaxWait: time.Millisecond, InsertTimeout: time.Second,
		BackoffBase: time.Second, BackoffMax: 8 * time.Second,
	}, src, decode, ins, dlq, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// ---- tests

func TestHappyPathInsertsOnceAndAcksAll(t *testing.T) {
	m1, m2, m3 := msg(1, "a,b"), msg(2, "c"), msg(3, "d,e,f")
	ins := &fakeInserter{}
	w := newWorker(t, &fakeSource{batches: [][]Message{{m1, m2, m3}}}, ins, &fakeDLQ{})

	n, err := w.RunOnce(context.Background())
	if err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(ins.calls) != 1 || len(ins.calls[0]) != 6 {
		t.Fatalf("want one insert of 6 rows, got %v", ins.calls)
	}
	for _, m := range []*fakeMsg{m1, m2, m3} {
		if !m.acked || m.naked || m.termed {
			t.Fatalf("message %d: acked=%v naked=%v termed=%v", m.seq, m.acked, m.naked, m.termed)
		}
	}
}

func TestUndecodableMessageGoesToDLQAndOthersStillInsert(t *testing.T) {
	bad, good := msg(1, "BAD"), msg(2, "x")
	ins, dlq := &fakeInserter{}, &fakeDLQ{}
	w := newWorker(t, &fakeSource{batches: [][]Message{{bad, good}}}, ins, dlq)
	if _, err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !bad.termed || bad.acked {
		t.Fatalf("bad: termed=%v acked=%v", bad.termed, bad.acked)
	}
	if len(dlq.sent) != 1 || dlq.sent[0].Stage != "decode" || dlq.sent[0].Tenant != "acme" || string(dlq.sent[0].Data) != "BAD" {
		t.Fatalf("dlq: %+v", dlq.sent)
	}
	if !good.acked || len(ins.calls) != 1 || len(ins.calls[0]) != 1 {
		t.Fatalf("good message not inserted: acked=%v calls=%v", good.acked, ins.calls)
	}
}

func TestMissingTenantHeaderIsDeadLettered(t *testing.T) {
	m := msg(1, "x")
	m.tenant = ""
	dlq := &fakeDLQ{}
	w := newWorker(t, &fakeSource{batches: [][]Message{{m}}}, &fakeInserter{}, dlq)
	if _, err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !m.termed || len(dlq.sent) != 1 || !strings.Contains(dlq.sent[0].Reason, HeaderTenant) {
		t.Fatalf("termed=%v dlq=%+v", m.termed, dlq.sent)
	}
}

func TestEmptyMessageIsAckedWithoutInsert(t *testing.T) {
	m := msg(1, "")
	ins := &fakeInserter{}
	w := newWorker(t, &fakeSource{batches: [][]Message{{m}}}, ins, &fakeDLQ{})
	if _, err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !m.acked || len(ins.calls) != 0 {
		t.Fatalf("acked=%v calls=%v", m.acked, ins.calls)
	}
}

func TestTransientFailureNaksWithGrowingBackoffAndNeverDeadLetters(t *testing.T) {
	first, third := msg(1, "a"), msg(2, "b")
	third.delivered = 3
	ins := &fakeInserter{fail: func([]string) error { return errors.New("connection refused") }}
	dlq := &fakeDLQ{}
	w := newWorker(t, &fakeSource{batches: [][]Message{{first, third}}}, ins, dlq)
	if _, err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !first.naked || first.nakWait != time.Second {
		t.Fatalf("first delivery should back off 1s, got %v (naked=%v)", first.nakWait, first.naked)
	}
	if !third.naked || third.nakWait != 4*time.Second {
		t.Fatalf("third delivery should back off 4s, got %v", third.nakWait)
	}
	if len(dlq.sent) != 0 || first.acked || first.termed {
		t.Fatal("transient failures must keep data in the queue")
	}
	third.delivered = 50
	if got := w.backoff(third.delivered); got != 8*time.Second {
		t.Fatalf("backoff must cap at BackoffMax, got %v", got)
	}
}

func TestPermanentFailureIsBisectedToThePoisonMessage(t *testing.T) {
	var ms []*fakeMsg
	var batch []Message
	for i := 1; i <= 8; i++ {
		data := fmt.Sprintf("ok%d", i)
		if i == 5 {
			data = "POISON"
		}
		m := msg(uint64(i), data)
		ms, batch = append(ms, m), append(batch, m)
	}
	ins := &fakeInserter{fail: func(rows []string) error {
		for _, r := range rows {
			if r == "POISON" {
				return fmt.Errorf("%w: value out of range", ErrPermanent)
			}
		}
		return nil
	}}
	dlq := &fakeDLQ{}
	w := newWorker(t, &fakeSource{batches: [][]Message{batch}}, ins, dlq)
	if _, err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, m := range ms {
		if m.seq == 5 {
			if !m.termed || m.acked {
				t.Fatalf("poison message: termed=%v acked=%v", m.termed, m.acked)
			}
			continue
		}
		if !m.acked || m.termed || m.naked {
			t.Fatalf("message %d should be acked: acked=%v termed=%v naked=%v", m.seq, m.acked, m.termed, m.naked)
		}
	}
	if len(dlq.sent) != 1 || dlq.sent[0].Stage != "insert" || string(dlq.sent[0].Data) != "POISON" {
		t.Fatalf("dlq: %+v", dlq.sent)
	}
}

func TestDeadLetterFailureNaksInsteadOfDropping(t *testing.T) {
	m := msg(1, "BAD")
	w := newWorker(t, &fakeSource{batches: [][]Message{{m}}}, &fakeInserter{}, &fakeDLQ{err: errors.New("nats down")})
	if _, err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m.termed || !m.naked {
		t.Fatalf("message must stay in the queue: termed=%v naked=%v", m.termed, m.naked)
	}
}

func TestInFlightBatchFinishesAfterShutdownStarts(t *testing.T) {
	m := msg(1, "a")
	ctx, cancel := context.WithCancel(context.Background())
	ins := &fakeInserter{}
	src := &fakeSource{batches: [][]Message{{m}}, onFetch: cancel} // shutdown begins during fetch
	w := newWorker(t, src, ins, &fakeDLQ{})
	if _, err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if !m.acked || len(ins.calls) != 1 {
		t.Fatalf("fetched batch was abandoned: acked=%v calls=%v", m.acked, ins.calls)
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	w := newWorker(t, &fakeSource{}, &fakeInserter{}, &fakeDLQ{})
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after cancel")
	}
}

func TestBatchToken(t *testing.T) {
	a := BatchToken("traces", []uint64{3, 1, 2})
	if a != BatchToken("traces", []uint64{1, 2, 3}) {
		t.Fatal("token must not depend on message order")
	}
	if a == BatchToken("traces", []uint64{1, 2, 4}) || a == BatchToken("logs", []uint64{1, 2, 3}) {
		t.Fatal("token must differ for different batches and signals")
	}
}

func TestNewWorkerValidation(t *testing.T) {
	_, err := NewWorker[string](Config{}, &fakeSource{}, decode, &fakeInserter{}, &fakeDLQ{}, nil, nil)
	if err == nil {
		t.Fatal("expected validation error")
	}
}
