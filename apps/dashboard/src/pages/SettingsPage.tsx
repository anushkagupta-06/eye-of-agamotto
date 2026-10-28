import React, { useState } from 'react';
import { Key, Copy, Check, Terminal, Database, Zap } from 'lucide-react';
import { useTelemetry } from '../context/TelemetryContext';

export const SettingsPage: React.FC = () => {
  const { tenant, apiKey } = useTelemetry();
  const [copiedKey, setCopiedKey] = useState(false);
  const [copiedCurl, setCopiedCurl] = useState(false);

  const keyDigest = '86d5a1603ccf3d1894a1c444de79c2690c70433a38bcee733a04f2770063e7b8';
  const smokeCurl = `curl -X POST http://127.0.0.1:4318/v1/traces \\
  -H "Content-Type: application/json" \\
  -H "X-API-Key: ${apiKey}" \\
  -d '{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"demo-service"}}]},"scopeSpans":[{"spans":[{"traceId":"5b8efff798038103d269b633813fc60c","spanId":"eee19b7ec3c1b174","name":"GET /health","kind":2,"startTimeUnixNano":"'$(date +%s)'000000000","endTimeUnixNano":"'$(date +%s)'000000000"}]}]}]}'`;

  const copyToClipboard = (text: string, type: 'key' | 'curl') => {
    navigator.clipboard.writeText(text);
    if (type === 'key') {
      setCopiedKey(true);
      setTimeout(() => setCopiedKey(false), 2000);
    } else {
      setCopiedCurl(true);
      setTimeout(() => setCopiedCurl(false), 2000);
    }
  };

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Settings & Telemetry Ingestion</h1>
          <p className="page-desc">
            Manage tenant API keys, ingestion gateway authentication, and view pipeline configurations.
          </p>
        </div>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '22px' }}>
        {/* Tenant API Key Card */}
        <div className="panel-card">
          <div className="panel-title">
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Key size={18} style={{ color: 'var(--accent-emerald)' }} />
              <span>Active Tenant Authentication Key</span>
            </div>
            <span className="badge healthy">AUTHENTICATED</span>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 2fr', gap: '16px', alignItems: 'center' }}>
            <div>
              <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>Tenant ID</div>
              <div style={{ fontSize: '1rem', fontWeight: 600, color: '#fff', marginTop: '2px' }}>{tenant}</div>
            </div>

            <div>
              <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>Plaintext API Key (Send as 'X-API-Key' or 'Authorization: Bearer')</div>
              <div style={{ display: 'flex', gap: '8px', marginTop: '4px' }}>
                <input
                  type="text"
                  readOnly
                  value={apiKey}
                  style={{
                    flex: 1,
                    backgroundColor: 'var(--bg-card)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '6px',
                    padding: '8px 12px',
                    fontFamily: 'var(--font-mono)',
                    fontSize: '0.8rem',
                    color: 'var(--accent-emerald)',
                    outline: 'none',
                  }}
                />
                <button className="btn-secondary" onClick={() => copyToClipboard(apiKey, 'key')}>
                  {copiedKey ? <Check size={14} style={{ color: 'var(--accent-emerald)' }} /> : <Copy size={14} />}
                  <span>{copiedKey ? 'Copied' : 'Copy'}</span>
                </button>
              </div>
            </div>
          </div>

          <div style={{ marginTop: '16px', fontSize: '0.74rem', color: 'var(--text-muted)' }}>
            SHA-256 Digest stored in <code>.data/api-keys.txt</code>:{' '}
            <span className="code-pill">{keyDigest}</span>
          </div>
        </div>

        {/* Curl Ingest Smoke Test */}
        <div className="panel-card">
          <div className="panel-title">
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Terminal size={18} style={{ color: 'var(--accent-cyan)' }} />
              <span>OTLP HTTP / JSON Ingestion Smoke Test</span>
            </div>
            <button className="btn-secondary" style={{ padding: '4px 10px', fontSize: '0.76rem' }} onClick={() => copyToClipboard(smokeCurl, 'curl')}>
              {copiedCurl ? <Check size={12} style={{ color: 'var(--accent-emerald)' }} /> : <Copy size={12} />}
              <span>{copiedCurl ? 'Copied command' : 'Copy cURL'}</span>
            </button>
          </div>

          <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '12px' }}>
            Send an OpenTelemetry span batch directly to the running Ingestion Gateway on <code>:4318</code>:
          </p>

          <pre
            style={{
              backgroundColor: 'var(--bg-card)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '8px',
              padding: '14px',
              fontFamily: 'var(--font-mono)',
              fontSize: '0.76rem',
              color: '#38bdf8',
              overflowX: 'auto',
              lineHeight: 1.45,
            }}
          >
            {smokeCurl}
          </pre>
        </div>

        {/* Pipeline Architecture Spec */}
        <div className="panel-card">
          <div className="panel-title">
            <span>Configured Storage & Message Broker Contracts</span>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '16px' }}>
            <div style={{ backgroundColor: 'var(--bg-card)', padding: '16px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600, color: '#fff', marginBottom: '8px' }}>
                <Zap size={16} style={{ color: 'var(--accent-cyan)' }} />
                <span>NATS JetStream Spec</span>
              </div>
              <ul style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', display: 'flex', flexDirection: 'column', gap: '6px', paddingLeft: '16px' }}>
                <li>Stream: <code>TELEMETRY</code> (traces & logs)</li>
                <li>Dead Letter Queue: <code>telemetry.dlq.&gt;</code></li>
                <li>Deduplication window: 120s (<code>Nats-Msg-Id</code>)</li>
                <li>Max message size: 4 MiB</li>
              </ul>
            </div>

            <div style={{ backgroundColor: 'var(--bg-card)', padding: '16px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600, color: '#fff', marginBottom: '8px' }}>
                <Database size={16} style={{ color: 'var(--accent-purple)' }} />
                <span>ClickHouse Schema Spec</span>
              </div>
              <ul style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', display: 'flex', flexDirection: 'column', gap: '6px', paddingLeft: '16px' }}>
                <li>Table <code>observability.spans</code> (ReplacingMergeTree)</li>
                <li>MV <code>observability.trace_index</code> (AggregatingMergeTree)</li>
                <li>Table <code>observability.logs</code> (Tokenbf bloom filters)</li>
                <li>MV <code>observability.latency_1m</code> (RED quantiles)</li>
              </ul>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
