// Package natsio binds the pipeline to NATS JetStream.
package natsio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/anushkagupta-06/eye-of-agamotto/services/writer/internal/pipeline"
)

const dlqSubjectPrefix = "telemetry.dlq."

// Config configures the connection.
type Config struct {
	URL            string
	Token          string
	Stream         string
	ConnectTimeout time.Duration
	Log            *slog.Logger
}

// ConsumerConfig describes a durable pull consumer.
type ConsumerConfig struct {
	Durable       string
	Subject       string
	AckWait       time.Duration
	MaxAckPending int
}

// Conn is a NATS connection with JetStream.
type Conn struct {
	nc     *nats.Conn
	js     jetstream.JetStream
	stream string
}

// Connect fails fast if NATS is unreachable or the streams are missing; afterwards the client
// reconnects forever on its own.
func Connect(ctx context.Context, cfg Config) (*Conn, error) {
	log := cfg.Log
	nc, err := nats.Connect(cfg.URL,
		nats.Name("eoa-writer"),
		nats.Token(cfg.Token),
		nats.Timeout(cfg.ConnectTimeout),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) { log.Warn("nats disconnected", "err", err) }),
		nats.ReconnectHandler(func(c *nats.Conn) { log.Info("nats reconnected", "url", c.ConnectedUrl()) }),
	)
	if err != nil {
		return nil, fmt.Errorf("natsio: connect %s: %w", cfg.URL, err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("natsio: jetstream: %w", err)
	}
	for _, name := range []string{cfg.Stream, cfg.Stream + "_DLQ"} {
		if _, err := js.Stream(ctx, name); err != nil {
			nc.Close()
			return nil, fmt.Errorf("natsio: stream %q not found (run: docker compose -f docker-compose.infra.yml up nats-init): %w", name, err)
		}
	}
	return &Conn{nc: nc, js: js, stream: cfg.Stream}, nil
}

// EnsureConsumer creates or updates the durable consumer. MaxDeliver is unlimited on purpose:
// an unreachable database must never cause messages to be abandoned; poison messages are
// removed explicitly by the pipeline instead.
func (c *Conn) EnsureConsumer(ctx context.Context, cfg ConsumerConfig) error {
	_, err := c.js.CreateOrUpdateConsumer(ctx, c.stream, jetstream.ConsumerConfig{
		Durable:       cfg.Durable,
		FilterSubject: cfg.Subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       cfg.AckWait,
		MaxDeliver:    -1,
		MaxAckPending: cfg.MaxAckPending,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
	})
	if err != nil {
		return fmt.Errorf("natsio: consumer %q: %w", cfg.Durable, err)
	}
	return nil
}

// Source returns a fetcher bound to an existing durable. Workers must not share a Source.
func (c *Conn) Source(ctx context.Context, durable string) (pipeline.Source, error) {
	cons, err := c.js.Consumer(ctx, c.stream, durable)
	if err != nil {
		return nil, fmt.Errorf("natsio: consumer %q: %w", durable, err)
	}
	return &source{cons: cons}, nil
}

// DeadLetters returns the publisher for the dead-letter stream.
func (c *Conn) DeadLetters() pipeline.DeadLetterer { return &dlq{js: c.js} }

// Ready reports whether the writer can reach NATS.
func (c *Conn) Ready(ctx context.Context) error {
	if !c.nc.IsConnected() {
		return errors.New("nats: not connected")
	}
	if _, err := c.js.Stream(ctx, c.stream); err != nil {
		return fmt.Errorf("nats: stream %q: %w", c.stream, err)
	}
	return nil
}

// Close drains the connection.
func (c *Conn) Close() error { return c.nc.Drain() }

type source struct{ cons jetstream.Consumer }

func (s *source) Fetch(ctx context.Context, max int, wait time.Duration) ([]pipeline.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	batch, err := s.cons.Fetch(max, jetstream.FetchMaxWait(wait))
	if err != nil {
		return nil, err
	}
	var out []pipeline.Message
	for m := range batch.Messages() {
		meta, _ := m.Metadata()
		out = append(out, &message{m: m, meta: meta})
	}
	if err := batch.Error(); err != nil && len(out) == 0 {
		return nil, err
	}
	return out, nil
}

type message struct {
	m    jetstream.Msg
	meta *jetstream.MsgMetadata
}

func (m *message) Data() []byte           { return m.m.Data() }
func (m *message) Header(n string) string { return m.m.Headers().Get(n) }
func (m *message) Subject() string        { return m.m.Subject() }
func (m *message) Ack() error             { return m.m.Ack() }
func (m *message) Term() error            { return m.m.Term() }
func (m *message) Sequence() uint64 {
	if m.meta == nil {
		return 0
	}
	return m.meta.Sequence.Stream
}
func (m *message) Delivered() uint64 {
	if m.meta == nil {
		return 1
	}
	return m.meta.NumDelivered
}
func (m *message) Nak(d time.Duration) error {
	if d > 0 {
		return m.m.NakWithDelay(d)
	}
	return m.m.Nak()
}

type dlq struct{ js jetstream.JetStream }

func (d *dlq) Send(ctx context.Context, dl pipeline.DeadLetter) error {
	msg := nats.NewMsg(dlqSubjectPrefix + dl.Signal)
	msg.Data = dl.Data
	// A retried send of the same poison message is dropped by the stream's duplicate window.
	sum := sha256.Sum256(dl.Data)
	msg.Header.Set(nats.MsgIdHdr, dl.Signal+":"+dl.Stage+":"+hex.EncodeToString(sum[:16]))
	msg.Header.Set("X-Error", headerSafe(dl.Reason, 512))
	msg.Header.Set("X-Stage", dl.Stage)
	msg.Header.Set("X-Original-Subject", dl.Subject)
	msg.Header.Set("X-Delivery-Count", strconv.FormatUint(dl.Delivered, 10))
	if dl.Tenant != "" {
		msg.Header.Set(pipeline.HeaderTenant, dl.Tenant)
	}
	if _, err := d.js.PublishMsg(ctx, msg); err != nil {
		return fmt.Errorf("natsio: publish dead letter: %w", err)
	}
	return nil
}

// headerSafe removes characters that are illegal in NATS header values and bounds the length.
func headerSafe(s string, max int) string {
	s = strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
	if len(s) > max {
		s = s[:max]
	}
	return s
}
