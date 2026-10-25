// Package config loads and validates writer settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds every tunable of the writer.
type Config struct {
	NATSURL    string
	NATSToken  string
	NATSStream string

	ClickHouseAddr     string // native protocol host:port
	ClickHouseDatabase string
	ClickHouseUser     string
	ClickHousePassword string
	ClickHouseMaxConns int

	OpsAddr string

	Workers       int           // concurrent workers per signal
	MaxMsgs       int           // queue messages per insert, at most
	MaxWait       time.Duration // flush a partial batch after this long
	InsertTimeout time.Duration
	AckWait       time.Duration // JetStream redelivers unacked messages after this
	MaxAckPending int

	BackoffBase     time.Duration // retry delay after the first failed delivery
	BackoffMax      time.Duration
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
	integer := func(key string, def int) int {
		v := get(key)
		if v == "" {
			return def
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			errs = append(errs, fmt.Errorf("%s: invalid non-negative integer %q", key, v))
			return def
		}
		return n
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
		NATSURL:    str("NATS_URL", "nats://127.0.0.1:4222"),
		NATSToken:  required("NATS_AUTH_TOKEN"),
		NATSStream: str("NATS_STREAM", "TELEMETRY"),

		ClickHouseAddr:     str("CH_ADDR", "127.0.0.1:9000"),
		ClickHouseDatabase: str("CH_DATABASE", "observability"),
		ClickHouseUser:     str("CH_WRITER_USER", "writer"),
		ClickHousePassword: required("CH_WRITER_PASSWORD"),
		ClickHouseMaxConns: integer("WRITER_CH_MAX_CONNS", 8),

		OpsAddr: str("WRITER_OPS_ADDR", ":8081"),

		Workers:       integer("WRITER_WORKERS", 2),
		MaxMsgs:       integer("WRITER_MAX_MSGS", 32),
		MaxWait:       duration("WRITER_MAX_WAIT", time.Second),
		InsertTimeout: duration("WRITER_INSERT_TIMEOUT", 20*time.Second),
		AckWait:       duration("WRITER_ACK_WAIT", 60*time.Second),
		MaxAckPending: integer("WRITER_MAX_ACK_PENDING", 1024),

		BackoffBase:     duration("WRITER_BACKOFF_BASE", time.Second),
		BackoffMax:      duration("WRITER_BACKOFF_MAX", 30*time.Second),
		ShutdownTimeout: duration("WRITER_SHUTDOWN_TIMEOUT", 30*time.Second),
	}

	if c.Workers < 1 || c.Workers > 32 {
		errs = append(errs, errors.New("WRITER_WORKERS must be between 1 and 32"))
	}
	if c.MaxMsgs < 1 {
		errs = append(errs, errors.New("WRITER_MAX_MSGS must be >= 1"))
	}
	if c.MaxWait < time.Second {
		errs = append(errs, errors.New("WRITER_MAX_WAIT must be >= 1s (JetStream pull request minimum)"))
	}
	if c.InsertTimeout >= c.AckWait {
		errs = append(errs, errors.New("WRITER_INSERT_TIMEOUT must be shorter than WRITER_ACK_WAIT, or messages are redelivered mid-insert"))
	}
	if c.MaxAckPending < c.Workers*c.MaxMsgs {
		errs = append(errs, errors.New("WRITER_MAX_ACK_PENDING must be >= WRITER_WORKERS * WRITER_MAX_MSGS"))
	}
	if c.BackoffMax < c.BackoffBase {
		errs = append(errs, errors.New("WRITER_BACKOFF_MAX must be >= WRITER_BACKOFF_BASE"))
	}
	if c.ClickHouseMaxConns < 1 {
		errs = append(errs, errors.New("WRITER_CH_MAX_CONNS must be >= 1"))
	}
	return c, errors.Join(errs...)
}
