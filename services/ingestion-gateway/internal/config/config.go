// Package config loads and validates gateway settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	// streamMaxMsgBytes mirrors max_msg_size in infra/nats/streams/telemetry.json.
	streamMaxMsgBytes = 4 << 20
	// headerAllowance leaves room for NATS headers inside the stream's max message size.
	headerAllowance = 64 << 10
)

// Config holds every tunable of the ingestion gateway.
type Config struct {
	GRPCAddr string
	HTTPAddr string
	OpsAddr  string

	NATSURL    string
	NATSToken  string
	NATSStream string

	APIKeysFile    string
	RateLimitRPS   float64 // per tenant, requests per second; 0 disables
	RateLimitBurst int

	BatchTargetBytes int           // flush a batch once it reaches this size
	BatchMaxBytes    int           // hard cap for one NATS message
	BatchMaxDelay    time.Duration // flush a batch after this long even if small
	MaxInflightBytes int64         // admission limit; beyond it requests get 429
	PublishTimeout   time.Duration
	MaxRequestBytes  int64 // per request, after decompression

	RetryAfter      time.Duration // hint sent with 429/503
	ShutdownTimeout time.Duration
}

// Load reads configuration from the process environment.
func Load() (Config, error) { return LoadFrom(os.Getenv) }

// LoadFrom reads configuration through get, which makes it testable.
func LoadFrom(get func(string) string) (Config, error) {
	var errs []error

	str := func(key, def string) string {
		if v := get(key); v != "" {
			return v
		}
		return def
	}
	required := func(key string) string {
		v := get(key)
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
		return v
	}
	integer := func(key string, def int64) int64 {
		v := get(key)
		if v == "" {
			return def
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			errs = append(errs, fmt.Errorf("%s: invalid non-negative integer %q", key, v))
			return def
		}
		return n
	}
	float := func(key string, def float64) float64 {
		v := get(key)
		if v == "" {
			return def
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < 0 {
			errs = append(errs, fmt.Errorf("%s: invalid non-negative number %q", key, v))
			return def
		}
		return f
	}
	duration := func(key string, def time.Duration) time.Duration {
		v := get(key)
		if v == "" {
			return def
		}
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s: invalid positive duration %q", key, v))
			return def
		}
		return d
	}

	c := Config{
		GRPCAddr: str("GATEWAY_GRPC_ADDR", ":4317"),
		HTTPAddr: str("GATEWAY_HTTP_ADDR", ":4318"),
		OpsAddr:  str("GATEWAY_OPS_ADDR", ":8080"),

		NATSURL:    str("NATS_URL", "nats://127.0.0.1:4222"),
		NATSToken:  required("NATS_AUTH_TOKEN"),
		NATSStream: str("NATS_STREAM", "TELEMETRY"),

		APIKeysFile:    required("GATEWAY_API_KEYS_FILE"),
		RateLimitRPS:   float("GATEWAY_RATE_LIMIT_RPS", 500),
		RateLimitBurst: int(integer("GATEWAY_RATE_LIMIT_BURST", 1000)),

		BatchTargetBytes: int(integer("GATEWAY_BATCH_TARGET_BYTES", 1<<20)),
		BatchMaxBytes:    int(integer("GATEWAY_BATCH_MAX_BYTES", 3_670_016)), // 3.5 MiB
		BatchMaxDelay:    duration("GATEWAY_BATCH_MAX_DELAY", 5*time.Millisecond),
		MaxInflightBytes: integer("GATEWAY_MAX_INFLIGHT_BYTES", 256<<20),
		PublishTimeout:   duration("GATEWAY_PUBLISH_TIMEOUT", 5*time.Second),
		MaxRequestBytes:  integer("GATEWAY_MAX_REQUEST_BYTES", 3<<20),

		RetryAfter:      duration("GATEWAY_RETRY_AFTER", time.Second),
		ShutdownTimeout: duration("GATEWAY_SHUTDOWN_TIMEOUT", 20*time.Second),
	}

	if c.BatchTargetBytes <= 0 || c.BatchTargetBytes > c.BatchMaxBytes {
		errs = append(errs, errors.New("GATEWAY_BATCH_TARGET_BYTES must be in (0, GATEWAY_BATCH_MAX_BYTES]"))
	}
	if c.BatchMaxBytes > streamMaxMsgBytes-headerAllowance {
		errs = append(errs, fmt.Errorf("GATEWAY_BATCH_MAX_BYTES must be <= %d (stream max_msg_size minus header allowance)", streamMaxMsgBytes-headerAllowance))
	}
	if c.MaxRequestBytes <= 0 || c.MaxRequestBytes > int64(c.BatchMaxBytes) {
		errs = append(errs, errors.New("GATEWAY_MAX_REQUEST_BYTES must be in (0, GATEWAY_BATCH_MAX_BYTES]"))
	}
	if c.MaxInflightBytes < int64(c.BatchMaxBytes) {
		errs = append(errs, errors.New("GATEWAY_MAX_INFLIGHT_BYTES must be >= GATEWAY_BATCH_MAX_BYTES"))
	}
	if c.RateLimitRPS > 0 && c.RateLimitBurst < 1 {
		errs = append(errs, errors.New("GATEWAY_RATE_LIMIT_BURST must be >= 1 when rate limiting is enabled"))
	}
	return c, errors.Join(errs...)
}
