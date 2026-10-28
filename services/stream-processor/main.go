package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Println("Eye of Agamotto - Stream Processor")
	log.Println("Starting NATS trace consumer and ClickHouse edge store...")

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	startupCtx, startupCancel := context.WithTimeout(ctx, 15*time.Second)
	defer startupCancel()

	store, err := NewClickHouseEdgeStore(startupCtx)
	if err != nil {
		log.Fatalf("Failed to initialize ClickHouse edge store: %v", err)
	}
	defer store.Close()

	consumer, err := NewTraceConsumer(startupCtx, store)
	if err != nil {
		log.Fatalf("Failed to initialize NATS trace consumer: %v", err)
	}
	defer consumer.Close()

	log.Println("Stream processor started; waiting for trace messages.")

	if err := consumer.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("Stream processor stopped unexpectedly: %v", err)
	}

	log.Println("Stream processor stopped.")
}
