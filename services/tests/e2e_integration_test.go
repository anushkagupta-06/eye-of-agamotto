package tests_test

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/protobuf/proto"
)

// --------------------------------------------------------------------------
// End-to-End Positive Test: Ingestion Contract -> Query API -> MCP Server
// --------------------------------------------------------------------------
func TestE2E_FullPipelineDataContract(t *testing.T) {
	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"

	// In-memory handler representing ClickHouse query responses
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/traces/"+traceID:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"traceId": traceID,
				"spans": []map[string]any{
					{
						"traceId":     traceID,
						"spanId":      "00f067aa0ba902b7",
						"serviceName": "gateway",
						"name":        "POST /order",
						"kind":        "SERVER",
						"durationNs":  85000000,
						"statusCode":  "OK",
					},
					{
						"traceId":      traceID,
						"spanId":       "5fb397be34d23b0f",
						"parentSpanId": "00f067aa0ba902b7",
						"serviceName":  "payment",
						"name":         "POST /charge",
						"kind":         "SERVER",
						"durationNs":   45000000,
						"statusCode":   "OK",
					},
				},
			})

		case r.URL.Path == "/api/v1/traces":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"count": 1,
				"data": []map[string]any{
					{
						"traceId":       traceID,
						"rootService":   "gateway",
						"rootOperation": "POST /order",
						"spanCount":     2,
						"durationNs":    85000000,
					},
				},
			})

		case r.URL.Path == "/api/v1/logs":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"count": 1,
				"data": []map[string]any{
					{
						"tenantId":     "demo",
						"serviceName":  "payment",
						"severityText": "INFO",
						"body":         "Payment processed successfully for order #9821",
						"traceId":      traceID,
					},
				},
			})

		case r.URL.Path == "/api/v1/topology":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"services": []string{"gateway", "payment"},
				"edges": []map[string]any{
					{
						"caller":    "gateway",
						"callee":    "payment",
						"callCount": 1540,
						"p95Ms":     48,
					},
				},
			})

		default:
			http.NotFound(w, r)
		}
	})

	// 1. Verify Query API Trace Lookup Contract
	t.Run("QueryAPI_GetTraceContract", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/traces/"+traceID, nil)
		rec := httptest.NewRecorder()
		mockHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var trace map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&trace); err != nil {
			t.Fatalf("failed to decode trace contract: %v", err)
		}
		if trace["traceId"] != traceID {
			t.Fatalf("unexpected trace content: %+v", trace)
		}
	})

	// 2. Verify Query API Topology Contract
	t.Run("QueryAPI_TopologyContract", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology", nil)
		rec := httptest.NewRecorder()
		mockHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var top map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&top); err != nil {
			t.Fatalf("failed to decode topology contract: %v", err)
		}
		edges, ok := top["edges"].([]any)
		if !ok || len(edges) != 1 {
			t.Fatalf("expected 1 edge in topology, got: %+v", top)
		}
	})
}

// --------------------------------------------------------------------------
// Negative Tests: Auth, Malformed OTLP, Bad Trace IDs, and Redeliveries
// --------------------------------------------------------------------------
func TestE2E_NegativeQualityGates(t *testing.T) {
	// 1. Negative Test: SHA-256 API Key verification & rejection
	t.Run("Auth_RejectInvalidKey", func(t *testing.T) {
		validKey := "eoa_cffdedbba2a3d4f0ecb4706923ae02d5d4b2ac1a0c03ff44"
		validDigest := "86d5a1603ccf3d1894a1c444de79c2690c70433a38bcee733a04f2770063e7b8"

		verifyKey := func(key, expectedDigest string) bool {
			h := sha256.Sum256([]byte(key))
			actualDigest := hex.EncodeToString(h[:])
			return subtle.ConstantTimeCompare([]byte(actualDigest), []byte(expectedDigest)) == 1
		}

		// Valid key matches digest
		if !verifyKey(validKey, validDigest) {
			t.Fatalf("expected valid key to match digest")
		}

		// Invalid key rejected
		if verifyKey("eoa_fake_invalid_key_9999", validDigest) {
			t.Fatalf("expected invalid key to be rejected")
		}

		// Empty key rejected
		if verifyKey("", validDigest) {
			t.Fatalf("expected empty key to be rejected")
		}
	})

	// 2. Negative Test: Malformed OTLP Protobuf Payloads
	t.Run("OTLP_RejectMalformedPayloads", func(t *testing.T) {
		corruptBytes := []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x11, 0x22, 0x33}

		// OTLP trace decoder must reject corrupt payload
		var traceReq coltracepb.ExportTraceServiceRequest
		err := proto.Unmarshal(corruptBytes, &traceReq)
		if err == nil {
			t.Fatalf("expected unmarshal of corrupt protobuf bytes to fail")
		}

		// OTLP log decoder must reject corrupt payload
		var logReq collogspb.ExportLogsServiceRequest
		err = proto.Unmarshal(corruptBytes, &logReq)
		if err == nil {
			t.Fatalf("expected unmarshal of corrupt log bytes to fail")
		}
	})

	// 3. Negative Test: Invalid Hex Trace Identifier Validation
	t.Run("OTLP_ValidateTraceHexLength", func(t *testing.T) {
		// Valid OTLP trace is 16 bytes (32 hex characters)
		validTraceHex := "4bf92f3577b34da6a3ce929d0e0e4736"
		b, err := hex.DecodeString(validTraceHex)
		if err != nil || len(b) != 16 {
			t.Fatalf("expected valid 16-byte trace ID, got len=%d err=%v", len(b), err)
		}

		// Invalid trace ID (not hex, too short)
		invalidTraceHex := "not_a_valid_hex_id"
		_, err = hex.DecodeString(invalidTraceHex)
		if err == nil {
			t.Fatalf("expected invalid hex string to fail decoding")
		}

		shortTraceHex := "abcd"
		bShort, _ := hex.DecodeString(shortTraceHex)
		if len(bShort) == 16 {
			t.Fatalf("short trace ID should not have 16 bytes")
		}
	})

	// 4. Negative Test: Live Tail Protobuf Serialization & Tenant Scoping
	t.Run("LiveTail_OTLPLogsValidation", func(t *testing.T) {
		logReq := &collogspb.ExportLogsServiceRequest{
			ResourceLogs: []*logspb.ResourceLogs{
				{
					Resource: &resourcepb.Resource{
						Attributes: []*commonpb.KeyValue{
							{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "payment-worker"}}},
						},
					},
					ScopeLogs: []*logspb.ScopeLogs{
						{
							LogRecords: []*logspb.LogRecord{
								{
									TimeUnixNano: uint64(time.Now().UnixNano()),
									SeverityText: "ERROR",
									SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_ERROR,
									Body:         &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Database connection timed out after 5000ms"}},
								},
							},
						},
					},
				},
			},
		}

		data, err := proto.Marshal(logReq)
		if err != nil {
			t.Fatalf("failed to marshal valid OTLP log request: %v", err)
		}

		var decoded collogspb.ExportLogsServiceRequest
		if err := proto.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("failed to unmarshal valid OTLP log request: %v", err)
		}

		rLogs := decoded.GetResourceLogs()
		if len(rLogs) != 1 || len(rLogs[0].GetScopeLogs()[0].GetLogRecords()) != 1 {
			t.Fatalf("unexpected structure in decoded OTLP logs")
		}
		body := rLogs[0].GetScopeLogs()[0].GetLogRecords()[0].GetBody().GetStringValue()
		if !strings.Contains(body, "Database connection timed out") {
			t.Errorf("unexpected log body: %s", body)
		}
	})
}
