package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/anushkagupta-06/eye-of-agamotto/services/mcp-server/internal/protocol"
)

// ServeStdio starts the MCP JSON-RPC listener over standard input/output.
func ServeStdio(ctx context.Context, handler *MCPHandler) error {
	reader := bufio.NewReader(os.Stdin)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return fmt.Errorf("read from stdin: %w", err)
			}

			if len(line) == 0 || line[0] == '\n' || line[0] == '\r' {
				continue
			}

			var req protocol.JSONRPCRequest
			if err := json.Unmarshal(line, &req); err != nil {
				resp := &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      nil,
					Error: &protocol.JSONRPCError{
						Code:    protocol.ParseError,
						Message: fmt.Sprintf("failed to parse JSON-RPC message: %v", err),
					},
				}
				writeStdioResponse(resp)
				continue
			}

			resp := handler.HandleRequest(ctx, &req)
			if resp != nil {
				writeStdioResponse(resp)
			}
		}
	}
}

func writeStdioResponse(resp *protocol.JSONRPCResponse) {
	bytes, err := json.Marshal(resp)
	if err == nil {
		_, _ = os.Stdout.Write(bytes)
		_, _ = os.Stdout.Write([]byte("\n"))
	}
}
