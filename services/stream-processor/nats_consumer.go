package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	processorDurable = "proc-traces"
	traceSubject     = "telemetry.traces"
	tenantHeader     = "X-Tenant-Id"
)

type TraceConsumer struct {
	nc       *nats.Conn
	consumer jetstream.Consumer
	store    EdgeEventStore
}

func NewTraceConsumer(ctx context.Context, store EdgeEventStore) (*TraceConsumer, error) {
	url := envOrDefault("NATS_URL", "nats://127.0.0.1:4222")
	token := os.Getenv("NATS_AUTH_TOKEN")
	stream := envOrDefault("NATS_STREAM", "TELEMETRY")

	if token == "" {
		return nil, fmt.Errorf("NATS_AUTH_TOKEN is required")
	}
	if store == nil {
		return nil, fmt.Errorf("edge event store is required")
	}

	nc, err := nats.Connect(
		url,
		nats.Name("eoa-stream-processor"),
		nats.Token(token),
		nats.Timeout(5*time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create JetStream client: %w", err)
	}

	if _, err := js.Stream(ctx, stream); err != nil {
		nc.Close()
		return nil, fmt.Errorf("find JetStream stream %q: %w", stream, err)
	}

	consumer, err := js.CreateOrUpdateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Durable:       processorDurable,
		FilterSubject: traceSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       60 * time.Second,
		MaxDeliver:    -1,
		MaxAckPending: 100,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
	})
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create trace consumer: %w", err)
	}

	return &TraceConsumer{
		nc:       nc,
		consumer: consumer,
		store:    store,
	}, nil
}

func (c *TraceConsumer) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		batch, err := c.consumer.Fetch(
			10,
			jetstream.FetchMaxWait(2*time.Second),
		)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("NATS fetch failed: %v", err)
			time.Sleep(time.Second)
			continue
		}

		for msg := range batch.Messages() {
			if err := c.processMessage(ctx, msg); err != nil {
				log.Printf("Trace message processing failed: %v", err)

				if nakErr := msg.NakWithDelay(2 * time.Second); nakErr != nil {
					log.Printf("NATS negative acknowledgement failed: %v", nakErr)
				}
				continue
			}

			if err := msg.Ack(); err != nil {
				log.Printf("NATS acknowledgement failed: %v", err)
			}
		}

		if err := batch.Error(); err != nil && ctx.Err() == nil {
			log.Printf("NATS batch error: %v", err)
		}
	}
}

func (c *TraceConsumer) processMessage(ctx context.Context, msg jetstream.Msg) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	tenantID := msg.Headers().Get(tenantHeader)
	if tenantID == "" {
		return fmt.Errorf("message is missing %s header", tenantHeader)
	}

	spans, err := DecodeOTLPTraces(msg.Data())
	if err != nil {
		return fmt.Errorf("decode OTLP trace payload: %w", err)
	}

	edges := ExtractEdgeEvents(tenantID, spans)

	if err := c.store.InsertEdges(ctx, edges); err != nil {
		return fmt.Errorf("persist %d edge events: %w", len(edges), err)
	}

	log.Printf(
		"Processed trace message: tenant=%s spans=%d edge_events=%d",
		tenantID,
		len(spans),
		len(edges),
	)

	return nil
}

func (c *TraceConsumer) Close() error {
	return c.nc.Drain()
}
