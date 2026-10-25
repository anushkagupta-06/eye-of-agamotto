import React, { useState } from 'react';
import { ShieldAlert, Bot } from 'lucide-react';
import { useTelemetry } from '../context/TelemetryContext';
import type { Anomaly } from '../types/telemetry';

export const AnomaliesPage: React.FC = () => {
  const { anomalies } = useTelemetry();
  const [selectedAnomaly, setSelectedAnomaly] = useState<Anomaly>(anomalies[0]);

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Automated Root-Cause Correlation</h1>
          <p className="page-desc">
            Algorithmic anomaly detection and statistical root-cause ranking stored in ClickHouse table <code>observability.anomalies</code>.
          </p>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 1.8fr', gap: '20px' }}>
        {/* Left: Anomalies List */}
        <div className="panel-card" style={{ padding: 0, overflow: 'hidden' }}>
          <div style={{ padding: '14px 16px', borderBottom: '1px solid var(--border-subtle)', fontWeight: 600, fontSize: '0.88rem' }}>
            Detected Anomaly Incidents
          </div>

          <div style={{ display: 'flex', flexDirection: 'column' }}>
            {anomalies.map((anom) => {
              const isSelected = selectedAnomaly?.id === anom.id;
              return (
                <div
                  key={anom.id}
                  onClick={() => setSelectedAnomaly(anom)}
                  style={{
                    padding: '16px',
                    borderBottom: '1px solid var(--border-subtle)',
                    cursor: 'pointer',
                    backgroundColor: isSelected ? 'var(--bg-card-hover)' : 'transparent',
                    borderLeft: isSelected ? '3px solid var(--status-critical)' : '3px solid transparent',
                    transition: 'all 0.15s ease',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
                    <span style={{ fontWeight: 600, color: '#fff', fontSize: '0.86rem' }}>{anom.serviceName}</span>
                    <span className="badge critical" style={{ fontSize: '0.68rem' }}>
                      {anom.zScore}σ SPIKE
                    </span>
                  </div>

                  <div style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', marginBottom: '8px' }}>
                    {anom.metric} ({anom.operation})
                  </div>

                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.74rem', color: 'var(--text-muted)' }}>
                    <span>Root Cause: <strong style={{ color: '#fff' }}>{anom.rootCauseService}</strong></span>
                    <span className="badge neutral">{anom.status}</span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Right: Detailed Root-Cause & Evidence Card */}
        <div className="panel-card">
          <div className="panel-title">
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <ShieldAlert size={20} style={{ color: 'var(--status-critical)' }} />
              <span>Incident Investigation & Evidence</span>
            </div>
            <span className="code-pill">ID: {selectedAnomaly?.id}</span>
          </div>

          {/* Statistical Breakdown */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '14px', marginBottom: '20px' }}>
            <div style={{ backgroundColor: 'var(--bg-card)', padding: '14px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
              <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', textTransform: 'uppercase' }}>Baseline Value</div>
              <div style={{ fontSize: '1.4rem', fontWeight: 700, color: '#fff', marginTop: '4px', fontFamily: 'var(--font-mono)' }}>
                {selectedAnomaly?.baselineValue}ms
              </div>
            </div>

            <div style={{ backgroundColor: 'var(--bg-card)', padding: '14px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
              <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', textTransform: 'uppercase' }}>Observed Value</div>
              <div style={{ fontSize: '1.4rem', fontWeight: 700, color: '#f87171', marginTop: '4px', fontFamily: 'var(--font-mono)' }}>
                {selectedAnomaly?.observedValue}ms
              </div>
            </div>

            <div style={{ backgroundColor: 'var(--bg-card)', padding: '14px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
              <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', textTransform: 'uppercase' }}>Z-Score (Confidence)</div>
              <div style={{ fontSize: '1.4rem', fontWeight: 700, color: 'var(--accent-amber)', marginTop: '4px', fontFamily: 'var(--font-mono)' }}>
                {selectedAnomaly?.zScore} σ
              </div>
            </div>
          </div>

          {/* Root Cause Conclusion */}
          <div style={{ backgroundColor: 'rgba(239, 68, 68, 0.08)', border: '1px solid rgba(239, 68, 68, 0.25)', borderRadius: '10px', padding: '16px', marginBottom: '20px' }}>
            <div style={{ fontWeight: 600, color: '#fca5a5', fontSize: '0.9rem', marginBottom: '6px' }}>
              Identified Root Cause: {selectedAnomaly?.rootCauseService}
            </div>
            <div style={{ fontSize: '0.82rem', color: 'var(--text-primary)', lineHeight: 1.5 }}>
              {selectedAnomaly?.evidence}
            </div>
          </div>

          {/* AI Recommended Remediation */}
          <div style={{ backgroundColor: 'var(--bg-card)', border: '1px solid var(--border-subtle)', borderRadius: '10px', padding: '16px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600, color: 'var(--accent-emerald)', fontSize: '0.86rem', marginBottom: '8px' }}>
              <Bot size={16} />
              <span>Eye of Agamotto AI Recommendation</span>
            </div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: 1.5 }}>
              1. Inspect circuit breaker configuration between <code>{selectedAnomaly?.serviceName}</code> and <code>{selectedAnomaly?.rootCauseService}</code>.
              <br />
              2. Temporary rate limit throttling on payment webhook ingestion to protect upstream worker threads.
              <br />
              3. Check logs filtered by trace ID to view complete stack traces.
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
