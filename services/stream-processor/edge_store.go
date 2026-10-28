package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type EdgeEventStore interface {
	InsertEdges(ctx context.Context, edges []EdgeEvent) error
	Close() error
}

type ClickHouseEdgeStore struct {
	conn clickhouse.Conn
}

func NewClickHouseEdgeStore(ctx context.Context) (*ClickHouseEdgeStore, error) {
	addr := envOrDefault("CH_ADDR", "127.0.0.1:9000")
	database := envOrDefault("CH_DATABASE", "observability")
	username := envOrDefault("CH_WRITER_USER", "writer")
	password := os.Getenv("CH_WRITER_PASSWORD")

	if password == "" {
		return nil, fmt.Errorf("CH_WRITER_PASSWORD is required")
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("open ClickHouse connection: %w", err)
	}

	store := &ClickHouseEdgeStore{conn: conn}

	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("connect to ClickHouse at %s: %w", addr, err)
	}

	return store, nil
}

func (s *ClickHouseEdgeStore) InsertEdges(ctx context.Context, edges []EdgeEvent) error {
	if len(edges) == 0 {
		return nil
	}

	batch, err := s.conn.PrepareBatch(ctx, `
		INSERT INTO edge_events
			(tenant_id, caller, callee, ts, duration_ns, is_error)
	`)
	if err != nil {
		return fmt.Errorf("prepare edge_events insert: %w", err)
	}

	for _, edge := range edges {
		if strings.TrimSpace(edge.TenantID) == "" ||
			strings.TrimSpace(edge.Caller) == "" ||
			strings.TrimSpace(edge.Callee) == "" {
			return fmt.Errorf("edge event has an empty tenant or service name")
		}

		if edge.Timestamp.IsZero() {
			return fmt.Errorf("edge event has no timestamp")
		}

		if err := batch.Append(
			edge.TenantID,
			edge.Caller,
			edge.Callee,
			edge.Timestamp,
			edge.DurationNs,
			edge.IsError,
		); err != nil {
			return fmt.Errorf("append edge event: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("insert edge_events batch: %w", err)
	}

	return nil
}

func (s *ClickHouseEdgeStore) Close() error {
	return s.conn.Close()
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
