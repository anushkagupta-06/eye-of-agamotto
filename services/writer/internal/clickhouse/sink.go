// Package clickhouse bulk-inserts rows into ClickHouse over the native protocol.
package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/anushkagupta-06/eye-of-agamotto/services/writer/internal/model"
	"github.com/anushkagupta-06/eye-of-agamotto/services/writer/internal/pipeline"
)

// Config configures the connection.
type Config struct {
	Addr     string
	Database string
	User     string
	Password string
	MaxConns int
	Timeout  time.Duration // upper bound for one insert, used as the read timeout
}

// Sink owns the connection pool.
type Sink struct {
	conn driver.Conn
}

// Open connects and verifies the credentials with a ping.
func Open(ctx context.Context, cfg Config) (*Sink, error) {
	conn, err := ch.Open(&ch.Options{
		Addr: []string{cfg.Addr},
		Auth: ch.Auth{Database: cfg.Database, Username: cfg.User, Password: cfg.Password},

		DialTimeout:     5 * time.Second,
		ReadTimeout:     cfg.Timeout + 5*time.Second,
		MaxOpenConns:    cfg.MaxConns,
		MaxIdleConns:    cfg.MaxConns,
		ConnMaxLifetime: 30 * time.Minute,
		Compression:     &ch.Compression{Method: ch.CompressionLZ4},
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse: open: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("clickhouse: ping %s as %q: %w", cfg.Addr, cfg.User, err)
	}
	return &Sink{conn: conn}, nil
}

// Ping reports whether ClickHouse is reachable.
func (s *Sink) Ping(ctx context.Context) error { return s.conn.Ping(ctx) }

// Close releases the pool.
func (s *Sink) Close() error { return s.conn.Close() }

// Spans returns the pipeline inserter for observability.spans.
func (s *Sink) Spans() pipeline.Inserter[model.Span] { return spanInserter{s} }

// Logs returns the pipeline inserter for observability.logs.
func (s *Sink) Logs() pipeline.Inserter[model.Log] { return logInserter{s} }

const (
	insertSpans = "INSERT INTO spans (tenant_id, trace_id, span_id, parent_span_id, service_name, name, kind, " +
		"start_time, duration_ns, status_code, status_message, http_status_code, attributes, resource_attributes)"
	insertLogs = "INSERT INTO logs (tenant_id, timestamp, service_name, severity_text, severity_number, body, " +
		"trace_id, span_id, attributes, resource_attributes)"
)

type spanInserter struct{ s *Sink }

func (i spanInserter) Insert(ctx context.Context, token string, rows []model.Span) error {
	return i.s.insert(ctx, token, insertSpans, len(rows), func(b driver.Batch, n int) error {
		r := rows[n]
		return b.Append(r.TenantID, r.TraceID, r.SpanID, r.ParentSpanID, r.ServiceName, r.Name, r.Kind,
			r.StartTime, r.DurationNs, r.StatusCode, r.StatusMessage, r.HTTPStatusCode, r.Attributes, r.ResourceAttributes)
	})
}

type logInserter struct{ s *Sink }

func (i logInserter) Insert(ctx context.Context, token string, rows []model.Log) error {
	return i.s.insert(ctx, token, insertLogs, len(rows), func(b driver.Batch, n int) error {
		r := rows[n]
		return b.Append(r.TenantID, r.Timestamp, r.ServiceName, r.SeverityText, r.SeverityNumber, r.Body,
			r.TraceID, r.SpanID, r.Attributes, r.ResourceAttributes)
	})
}

// insert sends one batch. insert_deduplication_token makes ClickHouse drop an identical retried
// batch; deduplicate_blocks_in_dependent_materialized_views extends that to the rollup tables, so
// a retry after a partial failure cannot double-count or skip them.
func (s *Sink) insert(ctx context.Context, token, query string, n int, appendRow func(driver.Batch, int) error) error {
	ctx = ch.Context(ctx, ch.WithSettings(ch.Settings{
		"insert_deduplication_token":                         token,
		"deduplicate_blocks_in_dependent_materialized_views": 1,
		"async_insert": 0,
	}))
	batch, err := s.conn.PrepareBatch(ctx, query)
	if err != nil {
		return classify(err)
	}
	for k := 0; k < n; k++ {
		if err := appendRow(batch, k); err != nil {
			_ = batch.Abort()
			// A value the client cannot encode is a property of the data, not of the server.
			return fmt.Errorf("%w: encode row %d: %v", pipeline.ErrPermanent, k, err)
		}
	}
	return classify(batch.Send())
}

// Server error codes, from ClickHouse's ErrorCodes list. ASSUMPTION: verified against the
// documented list, but worth a glance if a new failure mode shows up in the logs.
var (
	// Capacity or availability problems: retry later, the data is fine.
	transientCodes = map[int32]bool{
		159: true, // TIMEOUT_EXCEEDED
		164: true, // READONLY
		202: true, // TOO_MANY_SIMULTANEOUS_QUERIES
		203: true, // NO_FREE_CONNECTION
		209: true, // SOCKET_TIMEOUT
		210: true, // NETWORK_ERROR
		241: true, // MEMORY_LIMIT_EXCEEDED
		242: true, // TABLE_IS_READ_ONLY
		252: true, // TOO_MANY_PARTS
		319: true, // UNKNOWN_STATUS_OF_INSERT
	}
	// Misconfiguration: retrying is right, dead-lettering every message would be wrong.
	configCodes = map[int32]bool{
		60:  true, // UNKNOWN_TABLE
		81:  true, // UNKNOWN_DATABASE
		192: true, // UNKNOWN_USER
		193: true, // WRONG_PASSWORD
		194: true, // REQUIRED_PASSWORD
		195: true, // IP_ADDRESS_NOT_ALLOWED
		497: true, // ACCESS_DENIED
		516: true, // AUTHENTICATION_FAILED
	}
)

// classify marks server rejections of the data itself as permanent. Everything else, including
// every network-level error, stays retryable.
func classify(err error) error {
	if err == nil {
		return nil
	}
	var ex *ch.Exception
	if errors.As(err, &ex) && !transientCodes[ex.Code] && !configCodes[ex.Code] {
		return fmt.Errorf("%w: %v", pipeline.ErrPermanent, err)
	}
	return err
}
