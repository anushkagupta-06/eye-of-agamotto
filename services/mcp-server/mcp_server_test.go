package main_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anushkagupta-06/eye-of-agamotto/services/mcp-server/internal/client"
	"github.com/anushkagupta-06/eye-of-agamotto/services/mcp-server/internal/protocol"
	"github.com/anushkagupta-06/eye-of-agamotto/services/mcp-server/internal/server"
)

func setupTestServer(t *testing.T) (*server.MCPHandler, *httptest.Server) {
	// Mock QueryAPI server
	mockQueryAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/traces/"):
			_, _ = w.Write([]byte(`{"traceId":"t-123","totalDurationNs":50000000}`))
		case r.URL.Path == "/api/v1/traces":
			_, _ = w.Write([]byte(`{"count":1,"data":[{"traceId":"t-123","rootService":"api-gateway"}]}`))
		case r.URL.Path == "/api/v1/logs":
			_, _ = w.Write([]byte(`{"count":1,"data":[{"serviceName":"auth","body":"token verified"}]}`))
		case r.URL.Path == "/api/v1/topology":
			_, _ = w.Write([]byte(`{"services":["api-gateway","billing"],"edges":[{"caller":"api-gateway","callee":"billing"}]}`))
		case strings.HasPrefix(r.URL.Path, "/api/v1/anomalies/"):
			_, _ = w.Write([]byte(`{"id":"a-123","service":"billing","metric":"p99_latency","zScore":4.5}`))
		case r.URL.Path == "/api/v1/anomalies":
			_, _ = w.Write([]byte(`{"count":1,"data":[{"id":"a-123","service":"billing","zScore":4.5}]}`))
		default:
			http.NotFound(w, r)
		}
	}))

	qc := client.NewQueryAPIClient(mockQueryAPI.URL, "demo")
	handler := server.NewMCPHandler(qc)
	return handler, mockQueryAPI
}

func TestMCPInitialize(t *testing.T) {
	handler, mockAPI := setupTestServer(t)
	defer mockAPI.Close()

	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	resp := handler.HandleRequest(context.Background(), req)
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected successful response, got error: %+v", resp)
	}

	result, ok := resp.Result.(protocol.InitializeResult)
	if !ok {
		t.Fatalf("expected InitializeResult, got %T", resp.Result)
	}

	if result.ServerInfo.Name != "eye-of-agamotto-mcp" {
		t.Errorf("expected name eye-of-agamotto-mcp, got %s", result.ServerInfo.Name)
	}
}

func TestMCPToolsList(t *testing.T) {
	handler, mockAPI := setupTestServer(t)
	defer mockAPI.Close()

	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	resp := handler.HandleRequest(context.Background(), req)
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/list error: %+v", resp)
	}

	resultMap := resp.Result.(map[string]any)
	tools := resultMap["tools"].([]protocol.Tool)

	if len(tools) != 6 {
		t.Fatalf("expected 6 tools, got %d", len(tools))
	}

	expectedTools := []string{
		"searchTraces",
		"getTrace",
		"queryLogs",
		"getServiceTopology",
		"listAnomalies",
		"explainAnomaly",
	}

	for i, expected := range expectedTools {
		if tools[i].Name != expected {
			t.Errorf("tools[%d] expected %s, got %s", i, expected, tools[i].Name)
		}
	}
}

func TestMCPToolsCall(t *testing.T) {
	handler, mockAPI := setupTestServer(t)
	defer mockAPI.Close()

	ctx := context.Background()

	// 1. Test searchTraces
	callParams, _ := json.Marshal(map[string]any{
		"name": "searchTraces",
		"arguments": map[string]any{
			"service": "api-gateway",
			"status":  "OK",
		},
	})
	resp := handler.HandleRequest(ctx, &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "tools/call",
		Params:  callParams,
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("searchTraces failed: %+v", resp)
	}

	// 2. Test getTrace
	callParams, _ = json.Marshal(map[string]any{
		"name": "getTrace",
		"arguments": map[string]any{
			"traceId": "t-123",
		},
	})
	resp = handler.HandleRequest(ctx, &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "tools/call",
		Params:  callParams,
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("getTrace failed: %+v", resp)
	}

	// 3. Test queryLogs
	callParams, _ = json.Marshal(map[string]any{
		"name": "queryLogs",
		"arguments": map[string]any{
			"query": "token",
			"range": "15m",
		},
	})
	resp = handler.HandleRequest(ctx, &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      12,
		Method:  "tools/call",
		Params:  callParams,
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("queryLogs failed: %+v", resp)
	}

	// 4. Test getServiceTopology
	callParams, _ = json.Marshal(map[string]any{
		"name": "getServiceTopology",
		"arguments": map[string]any{
			"range": "15m",
		},
	})
	resp = handler.HandleRequest(ctx, &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "tools/call",
		Params:  callParams,
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("getServiceTopology failed: %+v", resp)
	}

	// 5. Test listAnomalies
	callParams, _ = json.Marshal(map[string]any{
		"name": "listAnomalies",
		"arguments": map[string]any{
			"range": "24h",
		},
	})
	resp = handler.HandleRequest(ctx, &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      14,
		Method:  "tools/call",
		Params:  callParams,
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("listAnomalies failed: %+v", resp)
	}

	// 6. Test explainAnomaly
	callParams, _ = json.Marshal(map[string]any{
		"name": "explainAnomaly",
		"arguments": map[string]any{
			"id": "a-123",
		},
	})
	resp = handler.HandleRequest(ctx, &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      15,
		Method:  "tools/call",
		Params:  callParams,
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("explainAnomaly failed: %+v", resp)
	}
}

func TestMCPHTTPRoutes(t *testing.T) {
	handler, mockAPI := setupTestServer(t)
	defer mockAPI.Close()

	httpServer := server.NewHTTPServer(handler)
	mux := http.NewServeMux()
	httpServer.RegisterRoutes(mux)

	// Health check
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("healthz expected 200, got %d", rr.Code)
	}

	// MCP JSON-RPC over POST
	postBody := `{"jsonrpc":"2.0","id":100,"method":"tools/list"}`
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(postBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("mcp POST expected 200, got %d", rr.Code)
	}

	var jsonResp protocol.JSONRPCResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &jsonResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if jsonResp.Error != nil {
		t.Errorf("unexpected error in response: %+v", jsonResp.Error)
	}

	// Assistant chat endpoint
	chatBody := `{"message":"Analyze active anomalies and failing services"}`
	req = httptest.NewRequest(http.MethodPost, "/api/assistant/chat", strings.NewReader(chatBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("assistant chat expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var chatResp server.AssistantChatResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &chatResp); err != nil {
		t.Fatalf("failed to decode assistant chat response: %v", err)
	}
	if len(chatResp.ToolsExecuted) == 0 {
		t.Fatalf("expected observable toolsExecuted, got none")
	}
	if chatResp.Response == "" {
		t.Fatalf("expected non-empty assistant response")
	}
}
