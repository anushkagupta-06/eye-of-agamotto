package natsio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/model"
)

const (
	logsSubject = "telemetry.logs"
)

// NATSTailer streams live log events directly from the NATS JetStream telemetry bus.
type NATSTailer struct {
	nc *nats.Conn
	js nats.JetStreamContext
}

// NewNATSTailer creates a new tailer connected to NATS.
func NewNATSTailer(url, token string) (*NATSTailer, error) {
	if url == "" {
		return nil, nil
	}

	opts := []nats.Option{
		nats.Name("query-api-tailer"),
		nats.Timeout(3 * time.Second),
	}
	if token != "" {
		opts = append(opts, nats.Token(token))
	}

	nc, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats connect for live tail: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("nats jetstream context: %w", err)
	}

	return &NATSTailer{nc: nc, js: js}, nil
}

// Tail establishes an ephemeral real-time stream subscription for a tenant.
func (t *NATSTailer) Tail(ctx context.Context, tenantID string, f model.LogTailFilters) (<-chan *model.LogRecord, error) {
	bufSize := f.BatchBuffer
	if bufSize <= 0 {
		bufSize = 100
	}
	out := make(chan *model.LogRecord, bufSize)

	if t == nil || t.nc == nil {
		close(out)
		return out, nil
	}

	// Ephemeral push subscription on telemetry.logs delivering only new events
	sub, err := t.nc.Subscribe(logsSubject, func(msg *nats.Msg) {
		// Filter by X-Tenant-Id header
		tenantHeader := msg.Header.Get("X-Tenant-Id")
		if tenantHeader != "" && tenantHeader != tenantID {
			return
		}

		// Parse the log record payload (handles JSON batch or single record)
		var records []model.LogRecord
		if err := json.Unmarshal(msg.Data, &records); err != nil {
			var single model.LogRecord
			if errSingle := json.Unmarshal(msg.Data, &single); errSingle == nil {
				records = append(records, single)
			}
		}

		for _, rec := range records {
			if f.Service != "" && rec.ServiceName != f.Service {
				continue
			}
			if f.Severity != "" && !strings.EqualFold(rec.Severity, f.Severity) {
				continue
			}
			if f.Query != "" && !strings.Contains(strings.ToLower(rec.Body), strings.ToLower(f.Query)) {
				continue
			}

			select {
			case out <- &rec:
			case <-ctx.Done():
				return
			default:
				// Avoid blocking consumer if reader drops behind
			}
		}
	})

	if err != nil {
		close(out)
		return nil, fmt.Errorf("subscribe to logs subject: %w", err)
	}

	go func() {
		<-ctx.Done()
		_ = sub.Unsubscribe()
		close(out)
	}()

	return out, nil
}

// Close disconnects the NATS connection.
func (t *NATSTailer) Close() {
	if t != nil && t.nc != nil {
		t.nc.Close()
	}
}
