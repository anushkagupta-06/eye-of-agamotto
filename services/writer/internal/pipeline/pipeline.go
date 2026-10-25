// Package pipeline moves messages from the queue into storage with at-least-once delivery.
//
// One Worker repeatedly: fetches a batch of queue messages, decodes them into rows, inserts all
// rows with a single bulk insert, and acknowledges the messages only after the insert succeeded.
//
// Failure handling:
//   - A message that cannot be decoded is poison: it is sent to the dead-letter queue and removed.
//   - A transient insert failure (ClickHouse down, slow, overloaded) negatively acknowledges the
//     whole batch with an exponential delay. Data stays safe in the queue, nothing is dead-lettered.
//   - A permanent insert failure (the data itself is rejected) is bisected: the batch is split in
//     halves until the offending message is isolated and dead-lettered, so one bad message cannot
//     block the rest.
//
// The package depends only on the standard library; the NATS and ClickHouse bindings live in
// their own packages and satisfy the interfaces below.
package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Header names set by the ingestion gateway (docs/contracts/ingestion-contract.md).
const (
	HeaderTenant     = "X-Tenant-Id"
	HeaderReceivedAt = "X-Received-At"
)

// ErrPermanent marks an insert failure caused by the data itself; retrying cannot succeed.
var ErrPermanent = errors.New("pipeline: permanent failure")

// Message is one queue message.
type Message interface {
	Data() []byte
	Header(name string) string
	Subject() string
	Sequence() uint64  // position in the stream, unique and stable across redeliveries
	Delivered() uint64 // how many times the queue has delivered this message
	Ack() error
	Nak(delay time.Duration) error
	Term() error // stop redelivery without treating it as success
}

// Source fetches up to max messages, waiting at most wait. Returning no messages is normal.
type Source interface {
	Fetch(ctx context.Context, max int, wait time.Duration) ([]Message, error)
}

// DeadLetter describes a message the pipeline gives up on.
type DeadLetter struct {
	Signal    string
	Stage     string // "decode" or "insert"
	Reason    string
	Tenant    string
	Subject   string
	Delivered uint64
	Data      []byte
}

// DeadLetterer durably stores a dead letter.
type DeadLetterer interface {
	Send(ctx context.Context, d DeadLetter) error
}

// Decoder turns one queue message payload into rows.
type Decoder[T any] func(tenant string, received time.Time, data []byte) ([]T, error)

// Inserter bulk-inserts rows. token identifies the batch so that a retried identical batch can
// be deduplicated by the store. Wrap data-caused failures with ErrPermanent.
type Inserter[T any] interface {
	Insert(ctx context.Context, token string, rows []T) error
}

// Stats receives counters; implementations must be safe for concurrent use.
type Stats interface {
	Batch(signal string, messages, rows int, d time.Duration, err error)
	Messages(signal, result string, n int) // result: "ack", "nak" or "dead"
}

// NopStats discards everything.
type NopStats struct{}

func (NopStats) Batch(string, int, int, time.Duration, error) {}
func (NopStats) Messages(string, string, int)                 {}

// Config configures a Worker.
type Config struct {
	Signal        string
	MaxMsgs       int
	MaxWait       time.Duration
	InsertTimeout time.Duration
	BackoffBase   time.Duration
	BackoffMax    time.Duration
}

func (c Config) validate() error {
	switch {
	case c.Signal == "":
		return errors.New("pipeline: signal is required")
	case c.MaxMsgs < 1:
		return errors.New("pipeline: MaxMsgs must be >= 1")
	case c.MaxWait <= 0, c.InsertTimeout <= 0, c.BackoffBase <= 0:
		return errors.New("pipeline: MaxWait, InsertTimeout and BackoffBase must be positive")
	case c.BackoffMax < c.BackoffBase:
		return errors.New("pipeline: BackoffMax must be >= BackoffBase")
	}
	return nil
}

// Worker is safe to run in several goroutines only if each uses its own Source.
type Worker[T any] struct {
	cfg    Config
	src    Source
	decode Decoder[T]
	ins    Inserter[T]
	dlq    DeadLetterer
	stats  Stats
	log    *slog.Logger
	now    func() time.Time
}

// NewWorker validates cfg and returns a Worker. stats and log may be nil.
func NewWorker[T any](cfg Config, src Source, decode Decoder[T], ins Inserter[T], dlq DeadLetterer, stats Stats, log *slog.Logger) (*Worker[T], error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if src == nil || decode == nil || ins == nil || dlq == nil {
		return nil, errors.New("pipeline: source, decoder, inserter and dead-letterer are required")
	}
	if stats == nil {
		stats = NopStats{}
	}
	if log == nil {
		log = slog.Default()
	}
	return &Worker[T]{cfg: cfg, src: src, decode: decode, ins: ins, dlq: dlq, stats: stats, log: log, now: time.Now}, nil
}

// Run processes batches until ctx is cancelled. The batch in flight when ctx is cancelled is
// completed first, so shutdown never abandons fetched messages.
func (w *Worker[T]) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if _, err := w.RunOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			w.log.Warn("fetch failed", "signal", w.cfg.Signal, "err", err)
			select {
			case <-time.After(w.cfg.BackoffBase):
			case <-ctx.Done():
				return
			}
		}
	}
}

type item[T any] struct {
	msg  Message
	rows []T
}

// RunOnce fetches and fully processes one batch, returning the number of messages fetched.
func (w *Worker[T]) RunOnce(ctx context.Context) (int, error) {
	msgs, err := w.src.Fetch(ctx, w.cfg.MaxMsgs, w.cfg.MaxWait)
	if err != nil {
		return 0, err
	}
	if len(msgs) == 0 {
		return 0, nil
	}
	// Past this point the batch is ours: finish it even if the process is shutting down.
	work := context.WithoutCancel(ctx)

	items := make([]item[T], 0, len(msgs))
	for _, m := range msgs {
		tenant := m.Header(HeaderTenant)
		if tenant == "" {
			w.dead(work, m, "decode", errors.New("missing "+HeaderTenant+" header"))
			continue
		}
		received := w.now()
		if ts, err := time.Parse(time.RFC3339Nano, m.Header(HeaderReceivedAt)); err == nil {
			received = ts
		}
		rows, err := w.decode(tenant, received, m.Data())
		if err != nil {
			w.dead(work, m, "decode", err)
			continue
		}
		if len(rows) == 0 {
			w.ack(m)
			continue
		}
		items = append(items, item[T]{msg: m, rows: rows})
	}
	w.insert(work, items)
	return len(msgs), nil
}

func (w *Worker[T]) insert(ctx context.Context, items []item[T]) {
	if len(items) == 0 {
		return
	}
	total := 0
	seqs := make([]uint64, 0, len(items))
	for _, it := range items {
		total += len(it.rows)
		seqs = append(seqs, it.msg.Sequence())
	}
	rows := make([]T, 0, total)
	for _, it := range items {
		rows = append(rows, it.rows...)
	}

	ictx, cancel := context.WithTimeout(ctx, w.cfg.InsertTimeout)
	start := time.Now()
	err := w.ins.Insert(ictx, BatchToken(w.cfg.Signal, seqs), rows)
	cancel()
	w.stats.Batch(w.cfg.Signal, len(items), total, time.Since(start), err)

	switch {
	case err == nil:
		for _, it := range items {
			w.ack(it.msg)
		}
	case errors.Is(err, ErrPermanent) && len(items) > 1:
		w.log.Warn("batch rejected by store, bisecting", "signal", w.cfg.Signal, "messages", len(items), "err", err)
		mid := len(items) / 2
		w.insert(ctx, items[:mid])
		w.insert(ctx, items[mid:])
	case errors.Is(err, ErrPermanent):
		w.dead(ctx, items[0].msg, "insert", err)
	default:
		w.log.Warn("insert failed, will retry", "signal", w.cfg.Signal, "messages", len(items), "rows", total, "err", err)
		for _, it := range items {
			w.nak(it.msg)
		}
	}
}

func (w *Worker[T]) ack(m Message) {
	if err := m.Ack(); err != nil {
		// The message will be redelivered and inserted again; deduplication absorbs that.
		w.log.Warn("ack failed", "signal", w.cfg.Signal, "seq", m.Sequence(), "err", err)
		return
	}
	w.stats.Messages(w.cfg.Signal, "ack", 1)
}

func (w *Worker[T]) nak(m Message) {
	if err := m.Nak(w.backoff(m.Delivered())); err != nil {
		w.log.Warn("nak failed", "signal", w.cfg.Signal, "seq", m.Sequence(), "err", err)
		return
	}
	w.stats.Messages(w.cfg.Signal, "nak", 1)
}

func (w *Worker[T]) dead(ctx context.Context, m Message, stage string, cause error) {
	err := w.dlq.Send(ctx, DeadLetter{
		Signal: w.cfg.Signal, Stage: stage, Reason: cause.Error(),
		Tenant: m.Header(HeaderTenant), Subject: m.Subject(), Delivered: m.Delivered(), Data: m.Data(),
	})
	if err != nil {
		// Never drop a message we could not preserve: retry it later.
		w.log.Error("dead-letter publish failed, will retry", "signal", w.cfg.Signal, "seq", m.Sequence(), "err", err)
		w.nak(m)
		return
	}
	w.log.Error("message dead-lettered", "signal", w.cfg.Signal, "stage", stage, "seq", m.Sequence(), "reason", cause.Error())
	if err := m.Term(); err != nil {
		w.log.Warn("term failed", "signal", w.cfg.Signal, "seq", m.Sequence(), "err", err)
		return
	}
	w.stats.Messages(w.cfg.Signal, "dead", 1)
}

// backoff doubles from BackoffBase for every earlier delivery, capped at BackoffMax.
func (w *Worker[T]) backoff(delivered uint64) time.Duration {
	d := w.cfg.BackoffBase
	for i := uint64(1); i < delivered && d < w.cfg.BackoffMax; i++ {
		d *= 2
	}
	if d > w.cfg.BackoffMax {
		d = w.cfg.BackoffMax
	}
	return d
}

// BatchToken derives a deduplication token from the stream sequences in a batch. Redelivering
// the same messages together produces the same token, so the store can drop the duplicate insert.
func BatchToken(signal string, seqs []uint64) string {
	sorted := append([]uint64(nil), seqs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	parts := make([]string, len(sorted))
	for i, s := range sorted {
		parts[i] = strconv.FormatUint(s, 10)
	}
	sum := sha256.Sum256([]byte(signal + ":" + strings.Join(parts, ",")))
	return fmt.Sprintf("%s-%s", signal, hex.EncodeToString(sum[:16]))
}
