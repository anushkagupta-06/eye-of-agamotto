// Client interface connecting the web UI and AI assistant to the Eye of Agamotto MCP Server.

export interface MCPToolCallResult {
  tool: string;
  result: any;
  error?: string;
}

export const MCP_SERVER_URL =
  (import.meta as any).env?.VITE_MCP_SERVER_URL || '/api/mcp';

/**
 * Health check to verify if the Eye of Agamotto MCP server is reachable.
 */
export async function checkMCPHealth(): Promise<boolean> {
  try {
    const res = await fetch(`${MCP_SERVER_URL}/healthz`, { method: 'GET' });
    return res.ok;
  } catch {
    return false;
  }
}

/**
 * Invoke an MCP tool using standard JSON-RPC 2.0 over HTTP.
 */
export async function callMCPTool(toolName: string, args: Record<string, any> = {}): Promise<any> {
  const payload = {
    jsonrpc: '2.0',
    id: Date.now(),
    method: 'tools/call',
    params: {
      name: toolName,
      arguments: args,
    },
  };

  try {
    const response = await fetch(`${MCP_SERVER_URL}/mcp`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      throw new Error(`MCP Server returned HTTP ${response.status}`);
    }

    const data = await response.json();
    if (data.error) {
      throw new Error(data.error.message || 'MCP tool execution failed');
    }

    const textContent = data.result?.content?.[0]?.text;
    if (textContent) {
      try {
        return JSON.parse(textContent);
      } catch {
        return textContent;
      }
    }
    return data.result;
  } catch (err: any) {
    console.warn(`MCP Tool '${toolName}' call failed:`, err);
    throw err;
  }
}

// Convenient typed wrappers for the 6 MCP tools:

export async function mcpSearchTraces(service?: string, minDuration?: number, status?: 'ERROR' | 'OK') {
  return callMCPTool('searchTraces', { service, minDuration, status });
}

export async function mcpGetTrace(traceId: string) {
  return callMCPTool('getTrace', { traceId });
}

export async function mcpQueryLogs(query?: string, range?: string, service?: string, severity?: string) {
  return callMCPTool('queryLogs', { query, range, service, severity });
}

export async function mcpGetServiceTopology(range?: string) {
  return callMCPTool('getServiceTopology', { range });
}

export async function mcpListAnomalies(range?: string) {
  return callMCPTool('listAnomalies', { range });
}

export async function mcpExplainAnomaly(id: string) {
  return callMCPTool('explainAnomaly', { id });
}

export interface ExecutedToolInfo {
  name: string;
  args: Record<string, any>;
  success: boolean;
  durationMs: number;
  summary: string;
}

export interface AssistantChatResult {
  response: string;
  toolsExecuted: ExecutedToolInfo[];
  liveDataGrounded: boolean;
}

/**
 * Route assistant requests through the backend orchestration layer.
 * Keeps provider credentials safe on the server and executes real MCP tools.
 */
export async function callAssistantOrchestrator(message: string): Promise<AssistantChatResult> {
  const res = await fetch(`${MCP_SERVER_URL}/api/assistant/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message }),
  });

  if (!res.ok) {
    throw new Error(`Assistant orchestrator returned HTTP ${res.status}`);
  }

  return res.json();
}
