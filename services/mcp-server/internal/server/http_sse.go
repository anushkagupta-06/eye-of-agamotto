package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/anushkagupta-06/eye-of-agamotto/services/mcp-server/internal/protocol"
)

// HTTPServer exposes the MCP server over HTTP and Server-Sent Events (SSE).
type HTTPServer struct {
	handler *MCPHandler
}

// NewHTTPServer creates an HTTP MCP server.
func NewHTTPServer(handler *MCPHandler) *HTTPServer {
	return &HTTPServer{handler: handler}
}

// RegisterRoutes registers endpoints onto an http.ServeMux.
func (s *HTTPServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/mcp", s.handleMCP)
	mux.HandleFunc("/sse", s.handleSSE)
}

func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"UP","service":"mcp-server"}`))
}

// handleMCP processes standard JSON-RPC HTTP POST requests.
// POST /mcp
func (s *HTTPServer) handleMCP(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req protocol.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONResponse(w, http.StatusBadRequest, &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &protocol.JSONRPCError{
				Code:    protocol.ParseError,
				Message: fmt.Sprintf("invalid JSON-RPC body: %v", err),
			},
		})
		return
	}

	resp := s.handler.HandleRequest(r.Context(), &req)
	if resp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	writeJSONResponse(w, http.StatusOK, resp)
}

// handleSSE handles streaming Server-Sent Events for MCP.
// GET /sse
func (s *HTTPServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send endpoint event pointing client to the POST message endpoint
	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: /mcp\n\n")
	flusher.Flush()

	<-r.Context().Done()
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-Id")
}

func writeJSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
