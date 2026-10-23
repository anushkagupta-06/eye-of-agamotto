// Package publisher implements batcher.Publisher on top of NATS JetStream.
package publisher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/batcher"
)

// Header names are part of the queue contract (docs/contracts/ingestion-contract.md).
const (
	HeaderTenant     = "X-Tenant-Id"
	HeaderReceivedAt = "X-Received-At"
)

// Config configures the NATS connection.
type Config struct {
	URL            string
	Token          string
	Stream         string
	ConnectTimeout time.Duration
	Log            *slog.Logger
}

// NATS publishes batches to JetStream and waits for the stream's acknowledgement.
type NATS struct {
	nc     *nats.Conn
	js     jetstream.JetStream
	stream string
}

// Connect fails fast if NATS is unreachable or the stream does not exist; after startup the
// client reconnects forever on its own.
func Connect(ctx context.Context, cfg Config) (*NATS, error) {
	log := cfg.Log
	opts := []nats.Option{
		nats.Name("eoa-ingestion-gateway"),
		nats.Token(cfg.Token),
		nats.Timeout(cfg.ConnectTimeout),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) { log.Warn("nats disconnected", "err", err) }),
		nats.ReconnectHandler(func(c *nats.Conn) { log.Info("nats reconnected", "url", c.ConnectedUrl()) }),
		nats.ClosedHandler(func(_ *nats.Conn) { log.Info("nats connection closed") }),
	}
	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("publisher: connect %s: %w", cfg.URL, err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("publisher: jetstream: %w", err)
	}
	if _, err := js.Stream(ctx, cfg.Stream); err != nil {
		nc.Close()
		return nil, fmt.Errorf("publisher: stream %q not found (run scripts/dev-infra.sh init): %w", cfg.Stream, err)
	}
	return &NATS{nc: nc, js: js, stream: cfg.Stream}, nil
}

// Publish implements batcher.Publisher. Retrying once is safe because Nats-Msg-Id makes the
// stream drop a duplicate delivered within its duplicate window.
func (p *NATS) Publish(ctx context.Context, b batcher.Batch) error {
	sum := sha256.Sum256(b.Payload)
	msg := nats.NewMsg(b.Subject)
	msg.Data = b.Payload
	msg.Header.Set(nats.MsgIdHdr, b.Tenant+":"+hex.EncodeToString(sum[:]))
	msg.Header.Set(HeaderTenant, b.Tenant)
	msg.Header.Set(HeaderReceivedAt, b.Received.UTC().Format(time.RFC3339Nano))
	msg.Header.Set("Content-Type", "application/x-protobuf")

	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = p.js.PublishMsg(ctx, msg); err == nil {
			return nil
		}
		var apiErr *jetstream.APIError
		if errors.As(err, &apiErr) || ctx.Err() != nil {
			break // the server answered, or we are out of time: retrying will not help
		}
		select {
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
		}
	}
	return classify(err)
}

// classify maps JetStream failures onto the batcher's retryable error kinds.
//
// ASSUMPTION (verify with the stream-full test in the setup steps): when the stream is full and
// discard=new, the server answers with a JetStream API error, which we report as overload (429).
// A missing stream or no responders is a deployment problem and reported as unavailable (503).
func classify(err error) error {
	var apiErr *jetstream.APIError
	switch {
	case errors.Is(err, jetstream.ErrStreamNotFound), errors.Is(err, jetstream.ErrNoStreamResponse):
		return fmt.Errorf("%w: %v", batcher.ErrUnavailable, err)
	case errors.As(err, &apiErr):
		return fmt.Errorf("%w: %v", batcher.ErrOverloaded, err)
	default:
		return fmt.Errorf("%w: %v", batcher.ErrUnavailable, err)
	}
}

// Ready reports whether the gateway can currently publish.
func (p *NATS) Ready(ctx context.Context) error {
	if !p.nc.IsConnected() {
		return errors.New("nats: not connected")
	}
	if _, err := p.js.Stream(ctx, p.stream); err != nil {
		return fmt.Errorf("nats: stream %q: %w", p.stream, err)
	}
	return nil
}

// Close drains pending publishes and closes the connection.
func (p *NATS) Close() error { return p.nc.Drain() }
