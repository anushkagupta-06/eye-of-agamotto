import React, { useState } from 'react';
import {
  Bot,
  Send,
  Sparkles,
  Code2,
  Cpu,
  ShieldAlert,
  Wrench,
  CheckCircle2,
  RefreshCw,
  Lightbulb
} from 'lucide-react';
import { useTelemetry } from '../context/TelemetryContext';
import { askGeminiAssistant, GEMINI_API_KEY_DEFAULT, GEMINI_MODEL } from '../api/geminiService';

interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
  tools?: string[];
  timestamp?: string;
  isStreaming?: boolean;
}

export const AssistantPage: React.FC = () => {
  const { tenant, systemProfileName, redMetrics, anomalies, spans, logs } = useTelemetry();

  const [apiKey] = useState<string>(() => {
    return localStorage.getItem('eoa_gemini_key') || GEMINI_API_KEY_DEFAULT;
  });

  const [isLoading, setIsLoading] = useState(false);
  const [inputVal, setInputVal] = useState('');

  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      role: 'assistant',
      content: `Greetings! I am the **Eye of Agamotto AI Observability Assistant** powered by **Google ${GEMINI_MODEL}** via Model Context Protocol (MCP).\n\nI continuously monitor your OpenTelemetry spans, ClickHouse error logs, and RED metrics. Ask me about **any failing service**, and I will analyze the telemetry and provide **concrete code & architectural improvements** to resolve the root cause!`,
      tools: ['mcp://gemini/analyze_telemetry', 'mcp://clickhouse/query_spans'],
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
      const geminiResponse = await askGeminiAssistant(
        queryText,
        {
          tenant,
          systemProfileName,
          redMetrics,
          anomalies,
          spans,
          logs,
        },
        apiKey
      );

      const assistantMessage: ChatMessage = {
        role: 'assistant',
        content: geminiResponse,
        tools: [
          'mcp://gemini/root_cause_analysis',
          'mcp://clickhouse/query_spans',
          'mcp://stream_processor/explain_anomaly',
        ],
        timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      };

      setMessages((prev) => [...prev, assistantMessage]);
    } catch {
      setMessages((prev) => [
        ...prev,
        {
          role: 'assistant',
          content: 'An error occurred while communicating with Gemini. Please check your network connection or API key.',
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
              backgroundColor: 'rgba(16, 185, 129, 0.1)',
              border: '1px solid rgba(16, 185, 129, 0.3)',
              padding: '6px 12px',
              borderRadius: '8px',
              fontSize: '0.78rem',
              color: 'var(--accent-emerald)',
            }}
          >
            <CheckCircle2 size={14} />
            <span>Gemini Key Active</span>
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

                  {m.tools && (
                    <div style={{ marginTop: '12px', paddingTop: '8px', borderTop: '1px solid var(--border-subtle)', display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                      {m.tools.map((t, tIdx) => (
                        <span key={tIdx} className="code-pill" style={{ fontSize: '0.68rem', color: 'var(--accent-cyan)' }}>
                          <Code2 size={10} style={{ display: 'inline', marginRight: '4px' }} />
                          {t}
                        </span>
                      ))}
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

            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', fontSize: '0.76rem' }}>
              <div style={{ background: 'var(--bg-card)', padding: '8px 10px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                <strong style={{ color: 'var(--accent-emerald)' }}>analyze_telemetry</strong>
                <p style={{ color: 'var(--text-muted)', marginTop: '2px' }}>Extracts high-cardinality error logs and p99 spikes.</p>
              </div>

              <div style={{ background: 'var(--bg-card)', padding: '8px 10px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                <strong style={{ color: 'var(--accent-cyan)' }}>suggest_improvements</strong>
                <p style={{ color: 'var(--text-muted)', marginTop: '2px' }}>Generates circuit breakers, retry budgets, and async NATS patterns.</p>
              </div>

              <div style={{ background: 'var(--bg-card)', padding: '8px 10px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                <strong style={{ color: 'var(--accent-purple)' }}>explain_anomaly</strong>
                <p style={{ color: 'var(--text-muted)', marginTop: '2px' }}>Computes statistical z-score and root cause ranking.</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
