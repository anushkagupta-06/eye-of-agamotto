package main

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const (
	processorDedupBucket = "EOA_PROCESSOR_DEDUP"
	processorDedupTTL    = 48 * time.Hour
)

// openDedupStore opens the persistent JetStream KV bucket, creating it
// when it does not exist.
func openDedupStore(
	ctx context.Context,
	js jetstream.JetStream,
) (jetstream.KeyValue, error) {
	kv, err := js.KeyValue(ctx, processorDedupBucket)
	if err == nil {
		return kv, nil
	}

	kv, createErr := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      processorDedupBucket,
		Description: "Processed trace messages for the stream processor",
		History:     1,
		TTL:         processorDedupTTL,
		Storage:     jetstream.FileStorage,
	})
	if createErr == nil {
		return kv, nil
	}

	// Another processor instance may have created the bucket concurrently.
	kv, err = js.KeyValue(ctx, processorDedupBucket)
	if err == nil {
		return kv, nil
	}

	return nil, fmt.Errorf(
		"open or create deduplication bucket %q: %w",
		processorDedupBucket,
		createErr,
	)
}
