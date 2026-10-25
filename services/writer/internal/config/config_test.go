package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDefaults(t *testing.T) {
	c, err := LoadFrom(env(map[string]string{"NATS_AUTH_TOKEN": "t", "CH_WRITER_PASSWORD": "p"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.ClickHouseUser != "writer" || c.Workers != 2 || c.NATSStream != "TELEMETRY" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestRequiredAndInvalid(t *testing.T) {
	_, err := LoadFrom(env(map[string]string{"WRITER_WORKERS": "x"}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"NATS_AUTH_TOKEN is required", "CH_WRITER_PASSWORD is required", "WRITER_WORKERS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestInsertTimeoutMustBeBelowAckWait(t *testing.T) {
	_, err := LoadFrom(env(map[string]string{
		"NATS_AUTH_TOKEN": "t", "CH_WRITER_PASSWORD": "p",
		"WRITER_INSERT_TIMEOUT": "90s", "WRITER_ACK_WAIT": "60s",
	}))
	if err == nil || !strings.Contains(err.Error(), "WRITER_INSERT_TIMEOUT") {
		t.Fatalf("got %v", err)
	}
}

func TestMaxWaitMinimum(t *testing.T) {
	_, err := LoadFrom(env(map[string]string{
		"NATS_AUTH_TOKEN": "t", "CH_WRITER_PASSWORD": "p", "WRITER_MAX_WAIT": "100ms",
	}))
	if err == nil || !strings.Contains(err.Error(), "WRITER_MAX_WAIT") {
		t.Fatalf("got %v", err)
	}
}
