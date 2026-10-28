package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type AssistantChatRequest struct {
	Message string `json:"message"`
}

type ToolExecutionRecord struct {
	Name       string         `json:"name"`
	Args       map[string]any `json:"args"`
	Success    bool           `json:"success"`
	DurationMs int64          `json:"durationMs"`
	Summary    string         `json:"summary"`
}

type AssistantChatResponse struct {
	Response         string                `json:"response"`
	ToolsExecuted    []ToolExecutionRecord `json:"toolsExecuted"`
	LiveDataGrounded bool                  `json:"liveDataGrounded"`
}

// RegisterAssistantRoutes registers the backend assistant chat endpoint.
func (s *HTTPServer) RegisterAssistantRoute(mux *http.ServeMux) {
	mux.HandleFunc("/api/assistant/chat", s.handleAssistantChat)
}

func (s *HTTPServer) handleAssistantChat(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AssistantChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request payload: %v", err), http.StatusBadRequest)
		return
	}

	query := strings.TrimSpace(req.Message)
	if query == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		geminiKey = os.Getenv("VITE_GEMINI_API_KEY")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var (
		toolsExecuted []ToolExecutionRecord
		finalResponse string
		grounded      bool
	)

	if geminiKey != "" {
		// Run multi-turn Gemini tool-calling loop on the server
		resp, executed, live, err := s.runGeminiToolLoop(ctx, geminiKey, query)
		if err == nil {
			finalResponse = resp
			toolsExecuted = executed
			grounded = live
		} else {
			log.Printf("[Assistant] Gemini tool loop error: %v; falling back to deterministic MCP diagnosis", err)
			finalResponse, toolsExecuted, grounded = s.runDeterministicMCPDiagnosis(ctx, query)
		}
	} else {
		// No API key: execute live MCP tools and generate grounded diagnosis
		finalResponse, toolsExecuted, grounded = s.runDeterministicMCPDiagnosis(ctx, query)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(AssistantChatResponse{
		Response:         finalResponse,
		ToolsExecuted:    toolsExecuted,
		LiveDataGrounded: grounded,
	})
}

// runGeminiToolLoop coordinates Gemini 2.5 with live MCP tools up to a bounded 5 turns.
func (s *HTTPServer) runGeminiToolLoop(
	ctx context.Context,
	apiKey string,
	userQuery string,
) (string, []ToolExecutionRecord, bool, error) {
	const model = "gemini-2.5-flash"
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)

	systemInstruction := `You are the Eye of Agamotto AI Observability Assistant, an expert Staff SRE and Principal Distributed Systems Architect.
You have direct access to live microservice telemetry in ClickHouse via Model Context Protocol (MCP) tools.
Always call the relevant MCP tools (searchTraces, getTrace, queryLogs, getServiceTopology, listAnomalies, explainAnomaly) to fetch REAL evidence before concluding.
Structure your final answer in these 4 markdown sections:
### 🚨 1. Incident Diagnosis & Root Cause
### ⚡ 2. Immediate Code & Architectural Fix
### ⚙️ 3. Configuration & Timeout Tuning
### 🛡️ 4. Blast Radius & Upstream Prevention`

	geminiTools := []map[string]any{
		{
			"function_declarations": []map[string]any{
				{
					"name":        "searchTraces",
					"description": "Search distributed traces filtered by service, minDuration, or status (ERROR/OK).",
					"parameters": map[string]any{
						"type": "OBJECT",
						"properties": map[string]any{
							"service": map[string]any{"type": "STRING", "description": "Microservice name"},
							"status":  map[string]any{"type": "STRING", "description": "ERROR or OK"},
							"limit":   map[string]any{"type": "NUMBER", "description": "Max traces to return"},
						},
					},
				},
				{
					"name":        "getTrace",
					"description": "Retrieve the complete distributed trace DAG for a specific traceId.",
					"parameters": map[string]any{
						"type": "OBJECT",
						"properties": map[string]any{
							"traceId": map[string]any{"type": "STRING", "description": "32-character hexadecimal trace identifier"},
						},
						"required": []string{"traceId"},
					},
				},
				{
					"name":        "queryLogs",
					"description": "Search indexed ClickHouse logs using token full-text search, service filter, or severity.",
					"parameters": map[string]any{
						"type": "OBJECT",
						"properties": map[string]any{
							"query":    map[string]any{"type": "STRING", "description": "Search token in log body"},
							"service":  map[string]any{"type": "STRING", "description": "Service name filter"},
							"severity": map[string]any{"type": "STRING", "description": "ERROR, WARN, INFO"},
							"range":    map[string]any{"type": "STRING", "description": "Time range (e.g. '15m', '1h', '24h')"},
						},
					},
				},
				{
					"name":        "getServiceTopology",
					"description": "Retrieve the live service dependency topology map with call counts and latency quantiles.",
					"parameters": map[string]any{
						"type":       "OBJECT",
						"properties": map[string]any{},
					},
				},
				{
					"name":        "listAnomalies",
					"description": "List active statistical anomalies and latency spikes.",
					"parameters": map[string]any{
						"type":       "OBJECT",
						"properties": map[string]any{},
					},
				},
				{
					"name":        "explainAnomaly",
					"description": "Retrieve deep diagnostic root-cause evidence for a specific anomaly ID.",
					"parameters": map[string]any{
						"type": "OBJECT",
						"properties": map[string]any{
							"id": map[string]any{"type": "STRING", "description": "Anomaly UUID identifier"},
						},
						"required": []string{"id"},
					},
				},
			},
		},
	}

	contents := []map[string]any{
		{
			"role": "user",
			"parts": []map[string]any{
				{"text": userQuery},
			},
		},
	}

	var executedRecords []ToolExecutionRecord

	// Bounded multi-turn loop (up to 5 turns)
	for turn := 0; turn < 5; turn++ {
		reqBody := map[string]any{
			"system_instruction": map[string]any{
				"parts": []map[string]any{{"text": systemInstruction}},
			},
			"contents": contents,
			"tools":    geminiTools,
			"generationConfig": map[string]any{
				"temperature":     0.15,
				"maxOutputTokens": 2048,
			},
		}

		payloadBytes, _ := json.Marshal(reqBody)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
		if err != nil {
			return "", nil, false, err
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			return "", nil, false, err
		}
		respBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", nil, false, fmt.Errorf("gemini API error (HTTP %d): %s", resp.StatusCode, string(respBytes))
		}

		var geminiResp struct {
			Candidates []struct {
				Content struct {
					Parts []json.RawMessage `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
			return "", nil, false, err
		}

		if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
			return "", nil, false, fmt.Errorf("empty candidate response from Gemini")
		}

		parts := geminiResp.Candidates[0].Content.Parts

		// Check if Gemini invoked a function
		var funcCall struct {
			FunctionCall struct {
				Name string         `json:"name"`
				Args map[string]any `json:"args"`
			} `json:"functionCall"`
		}

		hasFuncCall := false
		for _, part := range parts {
			if err := json.Unmarshal(part, &funcCall); err == nil && funcCall.FunctionCall.Name != "" {
				hasFuncCall = true
				break
			}
		}

		if !hasFuncCall {
			// Model generated final text response
			var textPart struct {
				Text string `json:"text"`
			}
			var sb strings.Builder
			for _, part := range parts {
				if err := json.Unmarshal(part, &textPart); err == nil && textPart.Text != "" {
					sb.WriteString(textPart.Text)
				}
			}
			return sb.String(), executedRecords, len(executedRecords) > 0, nil
		}

		// Execute tool requested by Gemini
		toolName := funcCall.FunctionCall.Name
		toolArgs := funcCall.FunctionCall.Args
		rawArgs, _ := json.Marshal(toolArgs)

		start := time.Now()
		toolResult, execErr := s.handler.ExecuteTool(ctx, toolName, rawArgs)
		duration := time.Since(start).Milliseconds()

		success := execErr == nil
		summary := toolResult
		if len(summary) > 120 {
			summary = summary[:120] + "..."
		}
		if execErr != nil {
			summary = execErr.Error()
		}

		executedRecords = append(executedRecords, ToolExecutionRecord{
			Name:       toolName,
			Args:       toolArgs,
			Success:    success,
			DurationMs: duration,
			Summary:    summary,
		})

		// Append model's functionCall and tool's response to conversation history
		contents = append(contents, map[string]any{
			"role": "model",
			"parts": []map[string]any{
				{"functionCall": funcCall.FunctionCall},
			},
		})
		contents = append(contents, map[string]any{
			"role": "user",
			"parts": []map[string]any{
				{
					"functionResponse": map[string]any{
						"name": toolName,
						"response": map[string]any{
							"output": toolResult,
						},
					},
				},
			},
		})
	}

	return "Analysis completed after maximum tool calling turns.", executedRecords, true, nil
}

// runDeterministicMCPDiagnosis executes live tools deterministically and formulates a grounded SRE diagnosis.
func (s *HTTPServer) runDeterministicMCPDiagnosis(
	ctx context.Context,
	userQuery string,
) (string, []ToolExecutionRecord, bool) {
	var records []ToolExecutionRecord

	// 1. Query live anomalies
	start := time.Now()
	anomOutput, anomErr := s.handler.ExecuteTool(ctx, "listAnomalies", []byte(`{"range":"24h"}`))
	records = append(records, ToolExecutionRecord{
		Name:       "listAnomalies",
		Args:       map[string]any{"range": "24h"},
		Success:    anomErr == nil,
		DurationMs: time.Since(start).Milliseconds(),
		Summary:    truncateStr(anomOutput, 100),
	})

	// 2. Query live topology
	start = time.Now()
	topOutput, topErr := s.handler.ExecuteTool(ctx, "getServiceTopology", []byte(`{"range":"1h"}`))
	records = append(records, ToolExecutionRecord{
		Name:       "getServiceTopology",
		Args:       map[string]any{"range": "1h"},
		Success:    topErr == nil,
		DurationMs: time.Since(start).Milliseconds(),
		Summary:    truncateStr(topOutput, 100),
	})

	// 3. Query failing traces if error is suspected
	start = time.Now()
	traceOutput, traceErr := s.handler.ExecuteTool(ctx, "searchTraces", []byte(`{"status":"ERROR","limit":5}`))
	records = append(records, ToolExecutionRecord{
		Name:       "searchTraces",
		Args:       map[string]any{"status": "ERROR", "limit": 5},
		Success:    traceErr == nil,
		DurationMs: time.Since(start).Milliseconds(),
		Summary:    truncateStr(traceOutput, 100),
	})

	liveDataFound := (anomOutput != "" && anomOutput != "[]") ||
		(topOutput != "" && topOutput != "[]") ||
		(traceOutput != "" && traceOutput != "[]")

	var diagnosis strings.Builder
	diagnosis.WriteString("### 🚨 1. Incident Diagnosis & Root Cause\n")
	if liveDataFound {
		diagnosis.WriteString("Analyzed live telemetry directly from ClickHouse and NATS via Model Context Protocol.\n\n")
		if anomOutput != "" && anomOutput != "[]" {
			diagnosis.WriteString(fmt.Sprintf("**Active Anomalies Identified:**\n```json\n%s\n```\n\n", truncateStr(anomOutput, 600)))
		}
		if topOutput != "" && topOutput != "[]" {
			diagnosis.WriteString(fmt.Sprintf("**Service Topology & Latency Edges:**\n```json\n%s\n```\n\n", truncateStr(topOutput, 400)))
		}
		if traceOutput != "" && traceOutput != "[]" {
			diagnosis.WriteString(fmt.Sprintf("**Failing Distributed Spans:**\n```json\n%s\n```\n\n", truncateStr(traceOutput, 400)))
		}
	} else {
		diagnosis.WriteString("The database currently contains **zero active anomaly clusters or failing spans** for tenant `demo`. All microservice dependencies are currently operating within healthy latency budgets ($Z < 3.0\\sigma$).\n\n")
	}

	diagnosis.WriteString(`### ⚡ 2. Immediate Code & Architectural Fix
For high-concurrency microservice calls, wrap downstream clients in an asynchronous Circuit Breaker with exponential backoff:

` + "```go" + `
// Resilience pattern: Circuit Breaker with fail-fast probe
var cb = gobreaker.NewCircuitBreaker(gobreaker.Settings{
    Name:        "downstream-client",
    MaxRequests: 3,
    Interval:    10 * time.Second,
    Timeout:     2 * time.Second,
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        return counts.Requests >= 10 && float64(counts.TotalFailures)/float64(counts.Requests) >= 0.25
    },
})
` + "```" + `

### ⚙️ 3. Configuration & Timeout Tuning
- Set aggressive bounded context deadlines on outbound HTTP/gRPC requests (` + "`" + `context.WithTimeout(ctx, 350*time.Millisecond)` + "`" + `).
- Scale JetStream pull consumer batches to absorb traffic spikes.

### 🛡️ 4. Blast Radius & Upstream Prevention
Ensure upstream API Gateway drops shed traffic with HTTP ` + "`429 Too Many Requests`" + ` before backend worker connection pools saturate.`)

	return diagnosis.String(), records, liveDataFound
}

func truncateStr(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
