package config

import (
	"os"
	"strings"
)

// Config holds runtime configuration for the MCP Server.
type Config struct {
	Mode         string // "stdio" or "http"
	HTTPPort     string
	QueryAPIURL  string
	DefaultTenant string
}

// Load loads configuration from environment variables and CLI parameters.
func Load() *Config {
	return &Config{
		Mode:          getEnv("MCP_MODE", "http"), // default http mode, can switch to stdio
		HTTPPort:      getEnv("MCP_PORT", "8083"),
		QueryAPIURL:   getEnv("QUERY_API_URL", "http://127.0.0.1:8082"),
		DefaultTenant: getEnv("TENANT_ID", "demo"),
	}
}

func getEnv(key, def string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return def
}
