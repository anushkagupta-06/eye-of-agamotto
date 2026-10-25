import React, { useState } from 'react';
import { Bot, Send, Sparkles, Code2 } from 'lucide-react';
import { useTelemetry } from '../context/TelemetryContext';

export const AssistantPage: React.FC = () => {
  const { tenant } = useTelemetry();
  const [messages, setMessages] = useState([
    {
      role: 'assistant',
      content:
        'Greetings! I am the Eye of Agamotto Telemetry Assistant connected via Model Context Protocol (MCP). I can query ClickHouse spans, explain latency spikes, and diagnose cross-service anomalies in real time.',
      tools: ['mcp://observability/health_check'],
    },
    {
      role: 'user',
      content: 'Why did the checkout-service latency spike over the last 15 minutes?',
    },
    {
      role: 'assistant',
      content:
        'Based on ClickHouse span analysis (`observability.spans`) and the `observability.service_edges_1m` aggregation:\n\n• **Spike Location**: `checkout-service` endpoint `POST /api/v1/checkout` experienced a p99 latency increase from **42ms** to **280ms** (z-score: 4.82σ).\n• **Root Cause**: Downstream calls to `payment-gateway /v2/charge` timed out after 200ms with HTTP 504 status codes.\n• **Correlated Trace**: `5b8efff798038103d269b633813fc60c` indicates retry storms on Stripe payment provider integration.',
      tools: ['mcp://clickhouse/query_spans', 'mcp://stream_processor/explain_anomaly'],
    },
  ]);

  const [inputVal, setInputVal] = useState('');

  const handleSend = () => {
    if (!inputVal.trim()) return;
    const userMsg = inputVal;
    setInputVal('');
    setMessages((prev) => [
      ...prev,
      { role: 'user', content: userMsg },
      {
        role: 'assistant',
        content: `I analyzed telemetry for tenant "${tenant}". The services are currently operating within nominal parameters except for the payment-gateway timeout cluster.`,
        tools: ['mcp://clickhouse/query_latency'],
      },
    ]);
  };

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">AI Assistant & Model Context Protocol (MCP)</h1>
          <p className="page-desc">
            Model Context Protocol server interface exposing telemetry data, flamegraph inspection, and automated diagnosis to LLM assistants.
          </p>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: '20px' }}>
        {/* Chat Window */}
        <div className="panel-card" style={{ display: 'flex', flexDirection: 'column', height: '600px' }}>
          <div className="panel-title" style={{ paddingBottom: '12px', borderBottom: '1px solid var(--border-subtle)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Bot size={18} style={{ color: 'var(--accent-emerald)' }} />
              <span>Agamotto Autonomous Diagnostic Engine</span>
            </div>
            <span className="badge healthy">MCP Online</span>
          </div>

          {/* Messages Stream */}
          <div style={{ flex: 1, overflowY: 'auto', padding: '16px 0', display: 'flex', flexDirection: 'column', gap: '16px' }}>
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
                      width: '32px',
                      height: '32px',
                      borderRadius: '8px',
                      background: 'linear-gradient(135deg, #059669 0%, #10b981 100%)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      color: 'white',
                      flexShrink: 0,
                    }}
                  >
                    <Sparkles size={16} />
                  </div>
                )}

                <div
                  style={{
                    maxWidth: '80%',
                    padding: '12px 16px',
                    borderRadius: '10px',
                    backgroundColor: m.role === 'user' ? 'var(--accent-purple)' : 'var(--bg-card)',
                    color: '#fff',
                    fontSize: '0.84rem',
                    lineHeight: 1.5,
                    border: '1px solid var(--border-subtle)',
                    whiteSpace: 'pre-wrap',
                  }}
                >
                  {m.content}

                  {m.tools && (
                    <div style={{ marginTop: '8px', display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                      {m.tools.map((t, tIdx) => (
                        <span key={tIdx} className="code-pill" style={{ fontSize: '0.7rem', color: 'var(--accent-cyan)' }}>
                          <Code2 size={10} style={{ display: 'inline', marginRight: '4px' }} />
                          {t}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>

          {/* Input Bar */}
          <div style={{ borderTop: '1px solid var(--border-subtle)', paddingTop: '14px', display: 'flex', gap: '10px' }}>
            <input
              type="text"
              placeholder="Ask anything (e.g. 'Show me slowest span in payment-gateway', 'Diagnose error spike')..."
              value={inputVal}
              onChange={(e) => setInputVal(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSend()}
              style={{
                flex: 1,
                backgroundColor: 'var(--bg-card)',
                border: '1px solid var(--border-subtle)',
                borderRadius: '8px',
                padding: '10px 14px',
                color: 'white',
                outline: 'none',
                fontSize: '0.84rem',
              }}
            />
            <button className="btn-primary" onClick={handleSend}>
              <Send size={15} />
              <span>Send</span>
            </button>
          </div>
        </div>

        {/* Right: MCP Tools & Endpoints */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          <div className="panel-card">
            <div className="panel-title">
              <span>Registered MCP Tools</span>
              <span className="badge healthy">v1 Protocol</span>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', fontSize: '0.78rem' }}>
              <div style={{ backgroundColor: 'var(--bg-card)', padding: '10px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                <strong style={{ color: 'var(--accent-emerald)' }}>query_spans</strong>
                <p style={{ color: 'var(--text-secondary)', marginTop: '2px' }}>
                  Executes bounded partition queries against ClickHouse <code>trace_index</code> and <code>spans</code>.
                </p>
              </div>

              <div style={{ backgroundColor: 'var(--bg-card)', padding: '10px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                <strong style={{ color: 'var(--accent-cyan)' }}>get_service_edges</strong>
                <p style={{ color: 'var(--text-secondary)', marginTop: '2px' }}>
                  Retrieves caller-callee communication graphs from <code>service_edges_1m</code>.
                </p>
              </div>

              <div style={{ backgroundColor: 'var(--bg-card)', padding: '10px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                <strong style={{ color: 'var(--accent-purple)' }}>explain_anomaly</strong>
                <p style={{ color: 'var(--text-secondary)', marginTop: '2px' }}>
                  Returns statistical z-score breakdown and root-cause candidate services.
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
