package natsio

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"

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

// IsConnected returns whether NATS connection is active.
func (t *NATSTailer) IsConnected() bool {
	return t != nil && t.nc != nil && t.nc.IsConnected()
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
		// Strict single-tenant check: reject messages with missing or mismatched tenant header
		tenantHeader := msg.Header.Get("X-Tenant-Id")
		if tenantHeader == "" || tenantHeader != tenantID {
			return
		}

		// Decode OTLP protobuf ExportLogsServiceRequest payload or JSON fallback
		records, err := decodeOTLPLogs(msg.Data)
		if err != nil {
			return
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
			case out <- rec:
			case <-ctx.Done():
				return
			default:
				// Slow client / buffer full: discard oldest item to avoid stalling NATS consumer
				select {
				case <-out:
				default:
				}
				select {
				case out <- rec:
				default:
				}
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

// decodeOTLPLogs decodes OTLP protobuf ExportLogsServiceRequest payload or fallback JSON.
func decodeOTLPLogs(data []byte) ([]*model.LogRecord, error) {
	var req collogspb.ExportLogsServiceRequest
	if err := proto.Unmarshal(data, &req); err != nil {
		// Try JSON fallback
		var records []*model.LogRecord
		if errJSON := json.Unmarshal(data, &records); errJSON == nil && len(records) > 0 {
			return records, nil
		}
		var single model.LogRecord
		if errSingle := json.Unmarshal(data, &single); errSingle == nil && single.Body != "" {
			return []*model.LogRecord{&single}, nil
		}
		return nil, fmt.Errorf("decode logs: unmarshal protobuf: %w", err)
	}

	var out []*model.LogRecord
	now := time.Now().UTC()

	for _, rl := range req.GetResourceLogs() {
		resAttrs := flattenAttributes(rl.GetResource().GetAttributes())
		service := resAttrs["service.name"]
		if service == "" {
			service = "unknown_service"
		}

		for _, sl := range rl.GetScopeLogs() {
			for _, l := range sl.GetLogRecords() {
				ts := now
				if l.GetTimeUnixNano() != 0 {
					ts = time.Unix(0, int64(l.GetTimeUnixNano())).UTC()
				} else if l.GetObservedTimeUnixNano() != 0 {
					ts = time.Unix(0, int64(l.GetObservedTimeUnixNano())).UTC()
				}

				traceID := ""
				if len(l.GetTraceId()) > 0 {
					traceID = hex.EncodeToString(l.GetTraceId())
				}
				spanID := ""
				if len(l.GetSpanId()) > 0 {
					spanID = hex.EncodeToString(l.GetSpanId())
				}

				body := anyValueToString(l.GetBody())
				attrs := flattenAttributes(l.GetAttributes())

				out = append(out, &model.LogRecord{
					Timestamp:          ts,
					ServiceName:        service,
					Severity:           l.GetSeverityText(),
					SeverityNumber:     uint8(l.GetSeverityNumber()),
					Body:               body,
					TraceID:            traceID,
					SpanID:             spanID,
					Attributes:         attrs,
					ResourceAttributes: resAttrs,
				})
			}
		}
	}

	return out, nil
}

func flattenAttributes(kvs []*commonpb.KeyValue) map[string]string {
	if len(kvs) == 0 {
		return map[string]string{}
	}
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		m[kv.GetKey()] = anyValueToString(kv.GetValue())
	}
	return m
}

func anyValueToString(v *commonpb.AnyValue) string {
	if v == nil {
		return ""
	}
	switch x := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return x.StringValue
	case *commonpb.AnyValue_BoolValue:
		return strconv.FormatBool(x.BoolValue)
	case *commonpb.AnyValue_IntValue:
		return strconv.FormatInt(x.IntValue, 10)
	case *commonpb.AnyValue_DoubleValue:
		return strconv.FormatFloat(x.DoubleValue, 'g', -1, 64)
	case *commonpb.AnyValue_BytesValue:
		return hex.EncodeToString(x.BytesValue)
	default:
		return fmt.Sprint(v)
	}
}

// Close disconnects the NATS connection.
func (t *NATSTailer) Close() {
	if t != nil && t.nc != nil {
		t.nc.Close()
	}
}
