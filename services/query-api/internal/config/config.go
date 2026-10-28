package config

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime settings for the Query API service.
type Config struct {
	HTTPPort           string
	ClickHouseAddr     string
	ClickHouseDatabase string
	ClickHouseUser     string
	ClickHousePassword string
	ClickHouseMaxConns int
	QueryTimeout       time.Duration

	NATSURL       string
	NATSAuthToken string
}

// Load loads and validates configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPPort:           getEnv("QUERY_API_PORT", "8082"),
		ClickHouseAddr:     getEnv("CLICKHOUSE_ADDR", "127.0.0.1:9000"),
		ClickHouseDatabase: getEnv("CLICKHOUSE_DATABASE", "observability"),
		ClickHouseUser:     getEnv("CLICKHOUSE_USER", "query_api"),
		ClickHousePassword: os.Getenv("CH_READER_PASSWORD"),
		ClickHouseMaxConns: getIntEnv("CLICKHOUSE_MAX_CONNS", 16),
		QueryTimeout:       getDurationEnv("QUERY_TIMEOUT", 10*time.Second),

		NATSURL:       getEnv("NATS_URL", "nats://127.0.0.1:4222"),
		NATSAuthToken: os.Getenv("NATS_AUTH_TOKEN"),
	}

	var errs []error
	if cfg.ClickHousePassword == "" {
		errs = append(errs, errors.New("CH_READER_PASSWORD environment variable is required"))
	}

	// Validate ClickHouse host:port format
	if _, _, err := net.SplitHostPort(cfg.ClickHouseAddr); err != nil {
		errs = append(errs, errors.New("CLICKHOUSE_ADDR must be a valid host:port string"))
	}

	return cfg, errors.Join(errs...)
}

func getEnv(key, defaultVal string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return defaultVal
}

func getIntEnv(key string, defaultVal int) int {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			return i
		}
	}
	return defaultVal
}

func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if d, err := time.ParseDuration(val); err == nil && d > 0 {
			return d
		}
	}
	return defaultVal
}
