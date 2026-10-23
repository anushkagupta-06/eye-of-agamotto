package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadDefaults(t *testing.T) {
	c, err := LoadFrom(env(map[string]string{"NATS_AUTH_TOKEN": "t", "GATEWAY_API_KEYS_FILE": "/k"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.GRPCAddr != ":4317" || c.HTTPAddr != ":4318" || c.NATSStream != "TELEMETRY" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadRequiredAndInvalid(t *testing.T) {
	_, err := LoadFrom(env(map[string]string{"GATEWAY_BATCH_MAX_DELAY": "soon"}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"NATS_AUTH_TOKEN is required", "GATEWAY_API_KEYS_FILE is required", "GATEWAY_BATCH_MAX_DELAY"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestLoadRejectsOversizedBatch(t *testing.T) {
	_, err := LoadFrom(env(map[string]string{
		"NATS_AUTH_TOKEN": "t", "GATEWAY_API_KEYS_FILE": "/k",
		"GATEWAY_BATCH_MAX_BYTES": "4194304",
	}))
	if err == nil || !strings.Contains(err.Error(), "GATEWAY_BATCH_MAX_BYTES") {
		t.Fatalf("expected batch size error, got %v", err)
	}
}
