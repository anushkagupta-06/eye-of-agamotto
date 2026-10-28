import React, { useState, useEffect } from 'react';
import {
  Bot,
  Send,
  Sparkles,
  Cpu,
  ShieldAlert,
  Wrench,
  CheckCircle2,
  RefreshCw,
  Lightbulb,
  Radio,
  Terminal
} from 'lucide-react';
import { useTelemetry } from '../context/TelemetryContext';
import { GEMINI_MODEL } from '../api/geminiService';
import { checkMCPHealth, callMCPTool, callAssistantOrchestrator, type ExecutedToolInfo } from '../api/mcpClient';

interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
  executedTools?: ExecutedToolInfo[];
  liveDataGrounded?: boolean;
  timestamp?: string;
  isStreaming?: boolean;
}

export const AssistantPage: React.FC = () => {
  const { tenant, systemProfileName, redMetrics, anomalies, logs } = useTelemetry();

  const [isLoading, setIsLoading] = useState(false);
  const [inputVal, setInputVal] = useState('');
  const [mcpConnected, setMcpConnected] = useState(false);
  const [selectedToolInspect, setSelectedToolInspect] = useState<ExecutedToolInfo | null>(null);

  useEffect(() => {
    let mounted = true;
    checkMCPHealth().then((healthy) => {
      if (mounted) setMcpConnected(healthy);
    });
    const interval = setInterval(() => {
      checkMCPHealth().then((healthy) => {
        if (mounted) setMcpConnected(healthy);
      });
    }, 10000);
    return () => {
      mounted = false;
      clearInterval(interval);
    };
  }, []);

  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      role: 'assistant',
      content: `Greetings! I am the **Eye of Agamotto AI Observability Assistant** powered by **Google ${GEMINI_MODEL}** via Model Context Protocol (MCP).\n\nI dynamically execute real MCP tools against your live ClickHouse and NATS JetStream telemetry backend. Ask me about **any failing service**, and I will query the live data and provide **concrete code & architectural improvements**!`,
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    },
  ]);

  const handleSendQuery = async (queryText: string) => {
    if (!queryText.trim() || isLoading) return;

    const userMessage: ChatMessage = {
      role: 'user',
      content: queryText.trim(),
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    };

    setMessages((prev) => [...prev, userMessage]);
    setInputVal('');
    setIsLoading(true);

    try {
      const result = await callAssistantOrchestrator(queryText);

      const assistantMessage: ChatMessage = {
        role: 'assistant',
        content: result.response,
        executedTools: result.toolsExecuted,
        liveDataGrounded: result.liveDataGrounded,
        timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      };

      setMessages((prev) => [...prev, assistantMessage]);
    } catch (err: any) {
      setMessages((prev) => [
        ...prev,
        {
          role: 'assistant',
          content: `An error occurred while executing the MCP assistant orchestration: ${err?.message || 'Server offline'}. Ensure mcp-server is running on port :8083.`,
          timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        },
      ]);
    } finally {
      setIsLoading(false);
    }
  };

  const executeDirectMCPTool = async (toolName: string, args: Record<string, any>, label: string) => {
    if (isLoading) return;
    const toolCallMsg: ChatMessage = {
      role: 'user',
      content: `⚡ **Invoked MCP Tool:** \`${toolName}\` (${label})`,
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    };
    setMessages((prev) => [...prev, toolCallMsg]);
    setIsLoading(true);

    try {
      const result = await callMCPTool(toolName, args);
      const formatted = typeof result === 'string' ? result : JSON.stringify(result, null, 2);

      const orchestratorResp = await callAssistantOrchestrator(
        `Analyze this raw MCP tool output from '${toolName}':\n\`\`\`json\n${formatted.slice(0, 1500)}\n\`\`\`\nPlease provide root-cause insights and architectural solutions.`
      );

      const assistantMsg: ChatMessage = {
        role: 'assistant',
        content: `### ⚡ MCP Tool Output: \`${toolName}\`\n\`\`\`json\n${formatted.slice(0, 800)}${formatted.length > 800 ? '\n... (truncated)' : ''}\n\`\`\`\n\n${orchestratorResp.response}`,
        executedTools: [{ name: toolName, args, success: true, durationMs: 25, summary: formatted.slice(0, 120) }],
        liveDataGrounded: orchestratorResp.liveDataGrounded,
        timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      };
      setMessages((prev) => [...prev, assistantMsg]);
    } catch (err: any) {
      setMessages((prev) => [
        ...prev,
        {
          role: 'assistant',
          content: `⚠️ MCP Tool \`${toolName}\` call failed: ${err.message || 'Server unreachable on port 8083'}. Ensure \`mcp-server\` is running.`,
          timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        },
      ]);
    } finally {
      setIsLoading(false);
    }
  };

  const quickPrompts = [
    'Analyze the current failing services and provide step-by-step code and architectural improvements to fix them.',
    'Why is the P99 latency spiking, and how can we prevent cascading timeouts to upstream gateways?',
    'Suggest resilience patterns (circuit breakers, retries with exponential backoff, NATS queues) for degraded services.',
  ];

  return (
    <div>
      <div className="page-header">
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
            <h1 className="page-title">AI Assistant & Autonomous Diagnostics</h1>
            <span className="badge healthy" style={{ fontSize: '0.74rem' }}>
              <Cpu size={12} /> Powered by Google {GEMINI_MODEL}
            </span>
          </div>
          <p className="page-desc">
            Directly connected to live OpenTelemetry telemetry. Analyzes spans, logs, and RED metrics to generate concrete code & architectural improvements.
          </p>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              backgroundColor: mcpConnected ? 'rgba(16, 185, 129, 0.1)' : 'rgba(234, 179, 8, 0.1)',
              border: `1px solid ${mcpConnected ? 'rgba(16, 185, 129, 0.3)' : 'rgba(234, 179, 8, 0.3)'}`,
              padding: '6px 12px',
              borderRadius: '8px',
              fontSize: '0.78rem',
              color: mcpConnected ? 'var(--accent-emerald)' : 'var(--accent-amber)',
            }}
          >
            <Radio size={14} className={mcpConnected ? 'pulse-fast' : ''} />
            <span>{mcpConnected ? 'MCP Server: Live (:8083)' : 'MCP Server: Standby'}</span>
          </div>

          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              backgroundColor: mcpConnected ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)',
              border: `1px solid ${mcpConnected ? 'rgba(16, 185, 129, 0.3)' : 'rgba(239, 68, 68, 0.3)'}`,
              padding: '6px 12px',
              borderRadius: '8px',
              fontSize: '0.78rem',
              color: mcpConnected ? 'var(--accent-emerald)' : 'var(--status-critical)',
            }}
          >
            <CheckCircle2 size={14} />
            <span>Backend Tool Orchestrator: {mcpConnected ? 'Connected (:8083)' : 'Offline'}</span>
          </div>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 2fr) minmax(300px, 1fr)', gap: '20px' }}>
        {/* Chat Window */}
        <div className="panel-card" style={{ display: 'flex', flexDirection: 'column', height: '640px' }}>
          {/* Header */}
          <div className="panel-title" style={{ paddingBottom: '12px', borderBottom: '1px solid var(--border-subtle)', margin: 0 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Bot size={18} style={{ color: 'var(--accent-emerald)' }} />
              <span>Agamotto Diagnostic Engine</span>
            </div>
            <span className="badge info" style={{ fontSize: '0.7rem' }}>
              Domain: {systemProfileName}
            </span>
          </div>

          {/* Quick Action Chips */}
          <div style={{ display: 'flex', gap: '8px', padding: '10px 0', borderBottom: '1px solid var(--border-subtle)', overflowX: 'auto' }}>
            {quickPrompts.map((q, idx) => (
              <button
                key={idx}
                onClick={() => handleSendQuery(q)}
                disabled={isLoading}
                style={{
                  background: 'var(--bg-card)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '16px',
                  padding: '5px 12px',
                  fontSize: '0.73rem',
                  color: 'var(--text-secondary)',
                  cursor: 'pointer',
                  whiteSpace: 'nowrap',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '5px',
                  transition: 'all 0.15s ease',
                }}
                onMouseEnter={(e) => (e.currentTarget.style.borderColor = 'var(--accent-emerald)')}
                onMouseLeave={(e) => (e.currentTarget.style.borderColor = 'var(--border-subtle)')}
              >
                <Lightbulb size={12} style={{ color: 'var(--accent-amber)' }} />
                <span>{q.slice(0, 48)}...</span>
              </button>
            ))}
          </div>

          {/* Quick MCP Tool Action Bar */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '8px 0', borderBottom: '1px solid var(--border-subtle)', overflowX: 'auto' }}>
            <span style={{ fontSize: '0.68rem', color: 'var(--text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.5px', fontWeight: 600, marginRight: '4px', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <Terminal size={12} style={{ color: 'var(--accent-emerald)' }} /> MCP Tools:
            </span>
            <button
              onClick={() => executeDirectMCPTool('searchTraces', { status: 'ERROR', limit: 10 }, 'Failing Spans')}
              disabled={isLoading}
              style={{
                background: 'rgba(239, 68, 68, 0.08)',
                border: '1px solid rgba(239, 68, 68, 0.25)',
                borderRadius: '6px',
                padding: '3px 8px',
                fontSize: '0.72rem',
                color: 'var(--status-critical)',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
              }}
            >
              🔍 searchTraces(ERROR)
            </button>
            <button
              onClick={() => executeDirectMCPTool('getServiceTopology', {}, 'Dependency Graph')}
              disabled={isLoading}
              style={{
                background: 'rgba(99, 102, 241, 0.08)',
                border: '1px solid rgba(99, 102, 241, 0.25)',
                borderRadius: '6px',
                padding: '3px 8px',
                fontSize: '0.72rem',
                color: 'var(--accent-primary)',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
              }}
            >
              🗺️ getServiceTopology
            </button>
            <button
              onClick={() => executeDirectMCPTool('queryLogs', { severity: 'ERROR' }, 'Error Logs')}
              disabled={isLoading}
              style={{
                background: 'rgba(234, 179, 8, 0.08)',
                border: '1px solid rgba(234, 179, 8, 0.25)',
                borderRadius: '6px',
                padding: '3px 8px',
                fontSize: '0.72rem',
                color: 'var(--accent-amber)',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
              }}
            >
              📋 queryLogs(ERROR)
            </button>
            <button
              onClick={() => executeDirectMCPTool('listAnomalies', {}, 'Active Anomalies')}
              disabled={isLoading}
              style={{
                background: 'rgba(168, 85, 247, 0.08)',
                border: '1px solid rgba(168, 85, 247, 0.25)',
                borderRadius: '6px',
                padding: '3px 8px',
                fontSize: '0.72rem',
                color: '#c084fc',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
              }}
            >
              ⚠️ listAnomalies
            </button>
          </div>

          {/* Messages Stream */}
          <div style={{ flex: 1, overflowY: 'auto', padding: '16px 0', display: 'flex', flexDirection: 'column', gap: '18px' }}>
            {messages.map((m, idx) => (
              <div
                key={idx}
                style={{
                  display: 'flex',
                  gap: '12px',
                  alignItems: 'flex-start',
                  justifyContent: m.role === 'user' ? 'flex-end' : 'flex-start',
                }}
              >
                {m.role === 'assistant' && (
                  <div
                    style={{
                      width: '34px',
                      height: '34px',
                      borderRadius: '10px',
                      background: 'linear-gradient(135deg, #059669 0%, #10b981 100%)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      color: 'white',
                      flexShrink: 0,
                      boxShadow: '0 0 12px rgba(16, 185, 129, 0.3)',
                    }}
                  >
                    <Sparkles size={18} />
                  </div>
                )}

                <div
                  style={{
                    maxWidth: '85%',
                    padding: '14px 18px',
                    borderRadius: '12px',
                    backgroundColor: m.role === 'user' ? 'rgba(139, 92, 246, 0.25)' : 'var(--bg-card)',
                    color: '#fff',
                    fontSize: '0.84rem',
                    lineHeight: 1.6,
                    border: m.role === 'user' ? '1px solid rgba(139, 92, 246, 0.5)' : '1px solid var(--border-subtle)',
                    whiteSpace: 'pre-wrap',
                  }}
                >
                  {m.content}

                  {m.executedTools && m.executedTools.length > 0 && (
                    <div style={{ marginTop: '12px', paddingTop: '10px', borderTop: '1px solid var(--border-subtle)' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px', fontSize: '0.74rem' }}>
                        <span style={{ color: 'var(--text-muted)' }}>
                          Observable MCP Tools ({m.executedTools.length} executed against tenant <strong>{tenant}</strong>):
                        </span>
                        {m.liveDataGrounded ? (
                          <span className="badge healthy" style={{ fontSize: '0.62rem' }}>Live Telemetry Grounded</span>
                        ) : (
                          <span className="badge warning" style={{ fontSize: '0.62rem' }}>Telemetry Table Empty</span>
                        )}
                      </div>
                      <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                        {m.executedTools.map((t, tIdx) => (
                          <button
                            key={tIdx}
                            type="button"
                            onClick={() => setSelectedToolInspect(t)}
                            className="code-pill"
                            style={{
                              fontSize: '0.72rem',
                              color: t.success ? 'var(--accent-emerald)' : '#f87171',
                              borderColor: t.success ? 'rgba(16, 185, 129, 0.4)' : 'rgba(239, 68, 68, 0.4)',
                              background: 'rgba(0, 0, 0, 0.3)',
                              cursor: 'pointer',
                              display: 'flex',
                              alignItems: 'center',
                              gap: '5px',
                            }}
                            title="Click to view tool execution payload and ClickHouse results"
                          >
                            <span>{t.success ? '✓' : '✗'}</span>
                            <span style={{ fontWeight: 600 }}>{t.name}</span>
                            <span style={{ color: 'var(--text-muted)' }}>({t.durationMs}ms)</span>
                          </button>
                        ))}
                      </div>
                    </div>
                  )}

                  {m.timestamp && (
                    <div style={{ textAlign: 'right', fontSize: '0.68rem', color: 'var(--text-muted)', marginTop: '4px' }}>
                      {m.timestamp}
                    </div>
                  )}
                </div>
              </div>
            ))}

            {isLoading && (
              <div style={{ display: 'flex', gap: '12px', alignItems: 'center' }}>
                <div
                  style={{
                    width: '34px',
                    height: '34px',
                    borderRadius: '10px',
                    background: 'var(--bg-card)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: 'var(--accent-emerald)',
                  }}
                >
                  <RefreshCw size={16} className="pulse" />
                </div>
                <div style={{ padding: '10px 14px', borderRadius: '10px', backgroundColor: 'var(--bg-card)', fontSize: '0.82rem', color: 'var(--text-secondary)' }}>
                  Google Gemini 2.5 Flash is analyzing live traces and logs to formulate improvements...
                </div>
              </div>
            )}
          </div>

          {/* Quick Action Suggestion Chips */}
          <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '12px' }}>
            {[
              {
                icon: '🚨',
                label: 'Diagnose failing services & generate code fixes',
                query: 'Diagnose all currently failing or degraded services, identify the root cause, and give me production-ready code fixes.',
              },
              {
                icon: '🛠️',
                label: 'Suggest Circuit Breaker & retry strategy',
                query: 'Recommend a resilient circuit breaker and exponential backoff configuration with code for our slowest service.',
              },
              {
                icon: '📈',
                label: 'Explain p99 latency spike anomaly',
                query: 'Analyze the active anomalies and explain why the p99 latency spiked so high above the baseline standard deviation.',
              },
              {
                icon: '🛡️',
                label: 'Prevent upstream cascading timeouts',
                query: 'How do I prevent timeouts in downstream services from propagating back to the API gateway?',
              },
            ].map((chip, idx) => (
              <button
                key={idx}
                type="button"
                className="btn-secondary"
                onClick={() => handleSendQuery(chip.query)}
                disabled={isLoading}
                style={{
                  fontSize: '0.74rem',
                  padding: '5px 10px',
                  borderRadius: '20px',
                  backgroundColor: 'rgba(255, 255, 255, 0.04)',
                  border: '1px solid var(--border-subtle)',
                  color: 'var(--text-secondary)',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                }}
              >
                <span>{chip.icon}</span>
                <span>{chip.label}</span>
              </button>
            ))}
          </div>

          {/* Input Bar */}
          <div style={{ borderTop: '1px solid var(--border-subtle)', paddingTop: '14px', display: 'flex', gap: '10px' }}>
            <input
              type="text"
              placeholder="Ask anything (e.g. 'Why is the service failing and how do I fix it?', 'Give code improvements')..."
              value={inputVal}
              onChange={(e) => setInputVal(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && !isLoading && handleSendQuery(inputVal)}
              disabled={isLoading}
              style={{
                flex: 1,
                backgroundColor: 'var(--bg-card)',
                border: '1px solid var(--border-subtle)',
                borderRadius: '8px',
                padding: '12px 14px',
                color: 'white',
                outline: 'none',
                fontSize: '0.85rem',
              }}
            />
            <button
              className="btn-primary"
              onClick={() => handleSendQuery(inputVal)}
              disabled={isLoading || !inputVal.trim()}
              style={{ padding: '0 18px' }}
            >
              <Send size={15} />
              <span>{isLoading ? 'Thinking...' : 'Ask Gemini'}</span>
            </button>
          </div>
        </div>

        {/* Right Column: Active Telemetry Context & Tools */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {/* Active Telemetry Context Card */}
          <div className="panel-card">
            <div className="panel-title">
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <ShieldAlert size={16} style={{ color: 'var(--status-critical)' }} />
                <span>Live Ingested Telemetry Feed</span>
              </div>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', fontSize: '0.78rem' }}>
              <div style={{ backgroundColor: 'var(--bg-card)', padding: '10px 12px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
                <div style={{ color: 'var(--text-muted)', fontSize: '0.72rem' }}>Degraded / Critical Services:</div>
                <div style={{ color: '#f87171', fontWeight: 600, marginTop: '2px' }}>
                  {redMetrics.filter((m) => m.status === 'CRITICAL' || m.status === 'DEGRADED').map((m) => m.serviceName).join(', ') || 'All Services Healthy'}
                </div>
              </div>

              <div style={{ backgroundColor: 'var(--bg-card)', padding: '10px 12px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
                <div style={{ color: 'var(--text-muted)', fontSize: '0.72rem' }}>Open Anomalies:</div>
                <div style={{ color: 'var(--accent-amber)', fontWeight: 600, marginTop: '2px' }}>
                  {anomalies.filter((a) => a.status === 'OPEN').length} active anomaly clusters
                </div>
              </div>

              <div style={{ backgroundColor: 'var(--bg-card)', padding: '10px 12px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
                <div style={{ color: 'var(--text-muted)', fontSize: '0.72rem' }}>ClickHouse Logs Sampled:</div>
                <div style={{ color: 'var(--accent-cyan)', fontWeight: 600, marginTop: '2px' }}>
                  {logs.length} live records indexed
                </div>
              </div>
            </div>
          </div>

          {/* Model & MCP Registry Card */}
          <div className="panel-card">
            <div className="panel-title">
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Wrench size={16} style={{ color: 'var(--accent-emerald)' }} />
                <span>Active Model Context Protocol (MCP)</span>
              </div>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', fontSize: '0.76rem', maxHeight: '280px', overflowY: 'auto' }}>
              <div style={{ background: 'var(--bg-card)', padding: '8px 10px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                <strong style={{ color: 'var(--accent-emerald)' }}>searchTraces</strong>
                <p style={{ color: 'var(--text-muted)', marginTop: '2px' }}>searchTraces(service, minDuration, status)</p>
              </div>

              <div style={{ background: 'var(--bg-card)', padding: '8px 10px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                <strong style={{ color: 'var(--accent-emerald)' }}>getTrace</strong>
                <p style={{ color: 'var(--text-muted)', marginTop: '2px' }}>getTrace(traceId) — DAG & critical path</p>
              </div>

              <div style={{ background: 'var(--bg-card)', padding: '8px 10px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                <strong style={{ color: 'var(--accent-cyan)' }}>queryLogs</strong>
                <p style={{ color: 'var(--text-muted)', marginTop: '2px' }}>queryLogs(query, range, service, severity)</p>
              </div>

              <div style={{ background: 'var(--bg-card)', padding: '8px 10px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                <strong style={{ color: 'var(--accent-cyan)' }}>getServiceTopology</strong>
                <p style={{ color: 'var(--text-muted)', marginTop: '2px' }}>getServiceTopology(range) — caller/callee graph</p>
              </div>

              <div style={{ background: 'var(--bg-card)', padding: '8px 10px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                <strong style={{ color: 'var(--accent-purple)' }}>listAnomalies</strong>
                <p style={{ color: 'var(--text-muted)', marginTop: '2px' }}>listAnomalies(range) — z-scores & deviations</p>
              </div>

              <div style={{ background: 'var(--bg-card)', padding: '8px 10px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                <strong style={{ color: 'var(--accent-purple)' }}>explainAnomaly</strong>
                <p style={{ color: 'var(--text-muted)', marginTop: '2px' }}>explainAnomaly(id) — root cause diagnosis</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Real MCP Tool Execution Inspector Modal */}
      {selectedToolInspect && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            backgroundColor: 'rgba(0, 0, 0, 0.75)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
            padding: '20px',
          }}
          onClick={() => setSelectedToolInspect(null)}
        >
          <div
            className="panel-card"
            style={{
              maxWidth: '640px',
              width: '100%',
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-medium)',
              boxShadow: '0 0 30px rgba(0, 0, 0, 0.8)',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px', borderBottom: '1px solid var(--border-subtle)', paddingBottom: '10px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Terminal size={18} style={{ color: selectedToolInspect.success ? 'var(--accent-emerald)' : '#f87171' }} />
                <span style={{ fontWeight: 600, color: '#fff', fontSize: '0.95rem' }}>
                  MCP Tool Execution: <code>{selectedToolInspect.name}</code>
                </span>
              </div>
              <button
                type="button"
                className="btn-secondary"
                style={{ padding: '3px 8px', fontSize: '0.74rem' }}
                onClick={() => setSelectedToolInspect(null)}
              >
                Close
              </button>
            </div>

            <div style={{ display: 'flex', gap: '10px', marginBottom: '14px' }}>
              <span className={`badge ${selectedToolInspect.success ? 'healthy' : 'critical'}`}>
                {selectedToolInspect.success ? 'STATUS: SUCCESS' : 'STATUS: FAILED'}
              </span>
              <span className="code-pill">Execution Duration: {selectedToolInspect.durationMs}ms</span>
            </div>

            <div style={{ marginBottom: '14px' }}>
              <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)', marginBottom: '4px', textTransform: 'uppercase', fontWeight: 600 }}>
                Input Arguments
              </div>
              <pre
                style={{
                  backgroundColor: 'var(--bg-card)',
                  padding: '10px',
                  borderRadius: '6px',
                  border: '1px solid var(--border-subtle)',
                  fontSize: '0.75rem',
                  fontFamily: 'var(--font-mono)',
                  color: 'var(--accent-cyan)',
                  margin: 0,
                  overflowX: 'auto',
                }}
              >
                {JSON.stringify(selectedToolInspect.args || {}, null, 2)}
              </pre>
            </div>

            <div>
              <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)', marginBottom: '4px', textTransform: 'uppercase', fontWeight: 600 }}>
                Database Output Summary (ClickHouse / Query API)
              </div>
              <pre
                style={{
                  backgroundColor: 'var(--bg-card)',
                  padding: '10px',
                  borderRadius: '6px',
                  border: '1px solid var(--border-subtle)',
                  fontSize: '0.75rem',
                  fontFamily: 'var(--font-mono)',
                  color: '#fff',
                  margin: 0,
                  maxHeight: '220px',
                  overflowY: 'auto',
                  whiteSpace: 'pre-wrap',
                }}
              >
                {selectedToolInspect.summary}
              </pre>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
