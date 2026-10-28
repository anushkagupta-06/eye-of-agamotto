package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anushkagupta-06/eye-of-agamotto/services/mcp-server/internal/client"
	"github.com/anushkagupta-06/eye-of-agamotto/services/mcp-server/internal/protocol"
)

// MCPHandler handles incoming JSON-RPC requests for the Model Context Protocol.
type MCPHandler struct {
	queryClient *client.QueryAPIClient
}

// NewMCPHandler creates a new MCP protocol handler.
func NewMCPHandler(qc *client.QueryAPIClient) *MCPHandler {
	return &MCPHandler{queryClient: qc}
}

// GetAvailableTools returns the 6 MCP tools defined in the Eye of Agamotto architecture.
func (h *MCPHandler) GetAvailableTools() []protocol.Tool {
	return []protocol.Tool{
		{
			Name:        "searchTraces",
			Description: "Search distributed traces filtered by microservice name, minimum duration threshold, and error/success status.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDef{
					"service": {
						Type:        "string",
						Description: "Microservice name (e.g. 'api-gateway', 'billing-service', 'notification-worker')",
					},
					"minDuration": {
						Type:        "number",
						Description: "Minimum execution duration filter in milliseconds (e.g. 100 for traces > 100ms)",
					},
					"status": {
						Type:        "string",
						Description: "Filter by status: 'ERROR' for failing traces or 'OK' for healthy traces",
						Enum:        []string{"ERROR", "OK"},
					},
					"limit": {
						Type:        "number",
						Description: "Maximum number of traces to return (default 20)",
					},
				},
			},
		},
		{
			Name:        "getTrace",
			Description: "Retrieve the complete distributed trace DAG for a specific traceId, including span hierarchy, execution durations, HTTP status codes, and critical path analysis.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDef{
					"traceId": {
						Type:        "string",
						Description: "The 32-character hexadecimal trace identifier",
					},
				},
				Required: []string{"traceId"},
			},
		},
		{
			Name:        "queryLogs",
			Description: "Search indexed ClickHouse logs using token full-text search, service filter, severity level, or trace correlation within a time window.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDef{
					"query": {
						Type:        "string",
						Description: "Search text token in log body (e.g. 'connection refused', 'timeout', 'deadlock')",
					},
					"range": {
						Type:        "string",
						Description: "Time range window (e.g. '15m', '1h', '24h')",
					},
					"service": {
						Type:        "string",
						Description: "Service name to filter logs (e.g. 'auth-service')",
					},
					"severity": {
						Type:        "string",
						Description: "Filter by minimum severity level (e.g. 'ERROR', 'WARN', 'INFO')",
						Enum:        []string{"ERROR", "WARN", "INFO", "DEBUG"},
					},
					"traceId": {
						Type:        "string",
						Description: "Filter logs correlated to a specific traceId",
					},
				},
			},
		},
		{
			Name:        "getServiceTopology",
			Description: "Retrieve the current directed service dependency topology map, including caller-to-callee edges, call counts, error counts, and P50/P95/P99 latency quantiles.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDef{
					"range": {
						Type:        "string",
						Description: "Lookback window for service edge aggregation (e.g. '15m', '1h')",
					},
				},
			},
		},
		{
			Name:        "listAnomalies",
			Description: "List detected statistical anomalies (p99 latency spikes, error rate aberrations, Z-score deviations) across all services.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDef{
					"range": {
						Type:        "string",
						Description: "Time range for anomalies (e.g. '1h', '24h', '7d')",
					},
					"limit": {
						Type:        "number",
						Description: "Maximum number of anomalies to return (default 50)",
					},
				},
			},
		},
		{
			Name:        "explainAnomaly",
			Description: "Retrieve deep diagnostic root-cause evidence, baseline vs observed deviations, and correlated failing spans for a specific anomaly ID.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDef{
					"id": {
						Type:        "string",
						Description: "The UUID identifier of the anomaly to diagnose",
					},
				},
				Required: []string{"id"},
			},
		},
	}
}

// HandleRequest processes an incoming JSON-RPC request and returns the corresponding response.
func (h *MCPHandler) HandleRequest(ctx context.Context, req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: protocol.InitializeResult{
				ProtocolVersion: "2024-11-05",
				Capabilities: protocol.ServerCapabilities{
					Tools: &protocol.ToolsCapability{ListChanged: false},
				},
				ServerInfo: protocol.ServerInfo{
					Name:    "eye-of-agamotto-mcp",
					Version: "1.0.0",
				},
			},
		}

	case "notifications/initialized":
		return nil // Notification, no response required

	case "ping":
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"status": "pong"},
		}

	case "tools/list":
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": h.GetAvailableTools(),
			},
		}

	case "tools/call":
		var params protocol.ToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &protocol.JSONRPCError{
					Code:    protocol.InvalidParams,
					Message: fmt.Sprintf("invalid tool call params: %v", err),
				},
			}
		}

		resultText, err := h.ExecuteTool(ctx, params.Name, params.Arguments)
		if err != nil {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.ToolResult{
					Content: []protocol.ToolContent{
						{Type: "text", Text: fmt.Sprintf("Error executing tool '%s': %v", params.Name, err)},
					},
					IsError: true,
				},
			}
		}

		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: protocol.ToolResult{
				Content: []protocol.ToolContent{
					{Type: "text", Text: resultText},
				},
			},
		}

	default:
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.JSONRPCError{
				Code:    protocol.MethodNotFound,
				Message: fmt.Sprintf("method '%s' not supported by MCP server", req.Method),
			},
		}
	}
}

// ExecuteTool executes one of the 6 tools by delegating to QueryAPI.
func (h *MCPHandler) ExecuteTool(ctx context.Context, toolName string, rawArgs json.RawMessage) (string, error) {
	var args map[string]any
	if len(rawArgs) > 0 {
		_ = json.Unmarshal(rawArgs, &args)
	}
	if args == nil {
		args = make(map[string]any)
	}

	getString := func(key string) string {
		if v, ok := args[key].(string); ok {
			return v
		}
		return ""
	}

	getFloat := func(key string) float64 {
		if v, ok := args[key].(float64); ok {
			return v
		}
		return 0
	}

	switch toolName {
	case "searchTraces":
		service := getString("service")
		status := getString("status")
		minDurMs := getFloat("minDuration")
		minDurNs := uint64(minDurMs * 1_000_000)
		limit := int(getFloat("limit"))
		return h.queryClient.SearchTraces(ctx, service, minDurNs, status, limit)

	case "getTrace":
		traceID := getString("traceId")
		if traceID == "" {
			return "", fmt.Errorf("traceId parameter is required")
		}
		return h.queryClient.GetTrace(ctx, traceID)

	case "queryLogs":
		query := getString("query")
		timeRange := getString("range")
		service := getString("service")
		severity := getString("severity")
		limit := int(getFloat("limit"))
		return h.queryClient.QueryLogs(ctx, query, timeRange, service, severity, limit)

	case "getServiceTopology":
		timeRange := getString("range")
		return h.queryClient.GetServiceTopology(ctx, timeRange)

	case "listAnomalies":
		timeRange := getString("range")
		limit := int(getFloat("limit"))
		return h.queryClient.ListAnomalies(ctx, timeRange, limit)

	case "explainAnomaly":
		id := getString("id")
		if id == "" {
			return "", fmt.Errorf("anomaly id parameter is required")
		}
		return h.queryClient.ExplainAnomaly(ctx, id)

	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}
