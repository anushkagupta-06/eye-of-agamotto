package clickhouse

import (
	"context"
	"fmt"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/config"
)

// Client wraps the ClickHouse read-only connection.
type Client struct {
	conn driver.Conn
}

// Open creates and verifies a ClickHouse connection using the query_api profile.
func Open(ctx context.Context, cfg *config.Config) (*Client, error) {
	conn, err := ch.Open(&ch.Options{
		Addr: []string{cfg.ClickHouseAddr},
		Auth: ch.Auth{
			Database: cfg.ClickHouseDatabase,
			Username: cfg.ClickHouseUser,
			Password: cfg.ClickHousePassword,
		},
		DialTimeout:     5 * time.Second,
		ReadTimeout:     cfg.QueryTimeout,
		MaxOpenConns:    cfg.ClickHouseMaxConns,
		MaxIdleConns:    cfg.ClickHouseMaxConns,
		ConnMaxLifetime: 30 * time.Minute,
		Compression:     &ch.Compression{Method: ch.CompressionLZ4},
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("clickhouse ping failed: %w", err)
	}

	return &Client{conn: conn}, nil
}

// Ping checks if ClickHouse is reachable and alive.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("clickhouse connection is not initialized")
	}
	return c.conn.Ping(ctx)
}

// Conn returns the raw ClickHouse driver connection for queries.
func (c *Client) Conn() driver.Conn {
	return c.conn
}

// Close closes the underlying connection pool.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
