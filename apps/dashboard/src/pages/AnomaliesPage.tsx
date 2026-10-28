import React, { useState, useMemo, useEffect } from 'react';
import {
  ShieldAlert,
  Bot,
  Layers,
  ArrowUpRight,
  TrendingUp,
  Clock,
  Activity,
  AlertTriangle,
  GitBranch,
  FileText,
  CheckCircle2
} from 'lucide-react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useTelemetry } from '../context/TelemetryContext';
import type { Anomaly } from '../types/telemetry';

export const AnomaliesPage: React.FC = () => {
  const { anomalies, spans, logs } = useTelemetry();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const urlId = searchParams.get('id') || '';

  const [selectedAnomalyId, setSelectedAnomalyId] = useState<string>(() => {
    return urlId || anomalies[0]?.id || '';
  });

  useEffect(() => {
    if (urlId) {
      setSelectedAnomalyId(urlId);
    } else if (!selectedAnomalyId && anomalies.length > 0) {
      setSelectedAnomalyId(anomalies[0].id);
    }
  }, [urlId, anomalies]);

  const selectedAnomaly: Anomaly | undefined = useMemo(() => {
    return anomalies.find((a) => a.id === selectedAnomalyId) || anomalies[0];
  }, [anomalies, selectedAnomalyId]);

  // Dynamically find spans whose p99 latency spiked or that errored during the anomaly window
  const correlatedSpans = useMemo(() => {
    if (!selectedAnomaly) return [];

    const baselineMs = selectedAnomaly.baselineValue || 30;

    return spans
      .filter((s) => {
        // Match the affected service or root cause service
        const isTargetService =
          s.serviceName === selectedAnomaly.serviceName ||
          s.serviceName === selectedAnomaly.rootCauseService;
        return isTargetService;
      })
      .map((s) => {
        const durationMs = Math.round(s.durationNs / 1000000);
        const isSpike = durationMs >= baselineMs * 1.8;
        const isError = s.statusCode === 'ERROR' || (s.httpStatusCode && s.httpStatusCode >= 400);

        return {
          ...s,
          durationMs,
          isSpike,
          isError,
          spikeMultiplier: (durationMs / Math.max(baselineMs, 1)).toFixed(1),
        };
      })
      .sort((a, b) => (b.isSpike || b.isError ? 1 : 0) - (a.isSpike || a.isError ? 1 : 0));
  }, [spans, selectedAnomaly]);

  // Dynamically find correlated error logs for the incident
  const correlatedLogs = useMemo(() => {
    if (!selectedAnomaly) return [];
    return logs
      .filter(
        (l) =>
          (l.serviceName === selectedAnomaly.serviceName ||
            l.serviceName === selectedAnomaly.rootCauseService) &&
          (l.severityText === 'ERROR' || l.severityText === 'WARN')
      )
      .slice(0, 4);
  }, [logs, selectedAnomaly]);

  const spikePercentage = selectedAnomaly
    ? Math.round(
        ((selectedAnomaly.observedValue - selectedAnomaly.baselineValue) /
          Math.max(selectedAnomaly.baselineValue, 1)) *
          100
      )
    : 0;

  return (
    <div>
      {/* Page Header */}
      <div className="page-header" style={{ marginBottom: '16px' }}>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
            <h1 className="page-title">Automated Root-Cause Correlation</h1>
            <span className="badge critical" style={{ fontSize: '0.72rem' }}>
              <ShieldAlert size={12} /> Anomaly Engine Active
            </span>
          </div>
          <p className="page-desc">
            Anomaly detection highlighting spans whose p99 latency spiked during error clusters.
          </p>
        </div>

        {/* Global Action */}
        <button
          className="btn-primary"
          onClick={() => {
            navigate('/assistant');
          }}
          style={{ fontSize: '0.8rem', gap: '6px' }}
        >
          <Bot size={15} />
          <span>Ask Gemini AI for Fix</span>
        </button>
      </div>

      {/* Main Two-Column Layout */}
      <div className="anomalies-two-col">
        {/* Left Column: Detected Anomaly Incidents List */}
        <div className="panel-card" style={{ padding: 0, overflow: 'hidden' }}>
          <div
            style={{
              padding: '14px 18px',
              borderBottom: '1px solid var(--border-subtle)',
              fontWeight: 600,
              fontSize: '0.88rem',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
            }}
          >
            <span>Detected Clusters</span>
            <span className="badge critical" style={{ fontSize: '0.68rem' }}>
              {anomalies.length} OPEN
            </span>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column' }}>
            {anomalies.length === 0 ? (
              <div style={{ padding: '36px 16px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '0.82rem' }}>
                <CheckCircle2 size={24} style={{ margin: '0 auto 8px', color: 'var(--accent-emerald)' }} />
                <div style={{ fontWeight: 600, color: 'var(--text-primary)' }}>Zero Anomalies Detected</div>
                <div style={{ fontSize: '0.74rem', marginTop: '4px' }}>All services operating within normal baseline limits.</div>
              </div>
            ) : (
              anomalies.map((anom) => {
                const isSelected = selectedAnomaly?.id === anom.id;
                return (
                  <div
                    key={anom.id}
                    onClick={() => setSelectedAnomalyId(anom.id)}
                    style={{
                      padding: '14px 16px',
                      borderBottom: '1px solid var(--border-subtle)',
                      cursor: 'pointer',
                      backgroundColor: isSelected ? 'rgba(239, 68, 68, 0.08)' : 'transparent',
                      borderLeft: isSelected ? '3px solid var(--status-critical)' : '3px solid transparent',
                      transition: 'all 0.15s ease',
                    }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px', gap: '8px' }}>
                      <span style={{ fontWeight: 600, color: '#fff', fontSize: '0.85rem', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {anom.serviceName}
                      </span>
                      <span className="badge critical" style={{ fontSize: '0.65rem', fontWeight: 700, flexShrink: 0 }}>
                        +{Math.round(((anom.observedValue - anom.baselineValue) / anom.baselineValue) * 100)}% SPIKE
                      </span>
                    </div>

                  <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '6px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {anom.metric}
                  </div>

                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.72rem', color: 'var(--text-muted)' }}>
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      Root: <strong style={{ color: '#fca5a5' }}>{anom.rootCauseService}</strong>
                    </span>
                    <span className="badge neutral" style={{ fontFamily: 'var(--font-mono)', fontSize: '0.65rem', flexShrink: 0 }}>
                      {anom.zScore}σ
                    </span>
                  </div>
                </div>
              );
            }))}
          </div>
        </div>

        {/* Right Column: Detailed Investigation, Spans Highlighting & Root Cause Evidence */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '18px', minWidth: 0, overflow: 'hidden' }}>
          {/* Statistical Breakdown & Deviation Header */}
          <div className="panel-card" style={{ minWidth: 0, overflow: 'hidden' }}>
            <div className="panel-title" style={{ marginBottom: '14px', flexWrap: 'wrap', gap: '8px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: 0 }}>
                <Activity size={18} style={{ color: 'var(--status-critical)', flexShrink: 0 }} />
                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  Statistical Deviation: <strong style={{ color: '#fff' }}>{selectedAnomaly?.serviceName}</strong>
                </span>
              </div>
              <span className="code-pill" style={{ fontSize: '0.7rem' }}>cluster_id: {selectedAnomaly?.id}</span>
            </div>

            {/* Metric Cards */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(130px, 1fr))', gap: '10px', marginBottom: '16px' }}>
              <div style={{ backgroundColor: 'var(--bg-card)', padding: '12px 14px', borderRadius: '8px', border: '1px solid var(--border-subtle)', minWidth: 0 }}>
                <div style={{ fontSize: '0.68rem', color: 'var(--text-muted)', textTransform: 'uppercase', display: 'flex', alignItems: 'center', gap: '4px' }}>
                  <Clock size={11} /> Baseline P99
                </div>
                <div style={{ fontSize: '1.25rem', fontWeight: 700, color: '#fff', marginTop: '4px', fontFamily: 'var(--font-mono)' }}>
                  {selectedAnomaly?.baselineValue}ms
                </div>
              </div>

              <div style={{ backgroundColor: 'var(--bg-card)', padding: '12px 14px', borderRadius: '8px', border: '1px solid rgba(239, 68, 68, 0.4)', minWidth: 0 }}>
                <div style={{ fontSize: '0.68rem', color: '#f87171', textTransform: 'uppercase', display: 'flex', alignItems: 'center', gap: '4px' }}>
                  <TrendingUp size={11} /> Observed P99
                </div>
                <div style={{ fontSize: '1.25rem', fontWeight: 700, color: '#f87171', marginTop: '4px', fontFamily: 'var(--font-mono)' }}>
                  {selectedAnomaly?.observedValue}ms
                </div>
              </div>

              <div style={{ backgroundColor: 'var(--bg-card)', padding: '12px 14px', borderRadius: '8px', border: '1px solid var(--border-subtle)', minWidth: 0 }}>
                <div style={{ fontSize: '0.68rem', color: 'var(--text-muted)', textTransform: 'uppercase' }}>Latency Surge</div>
                <div style={{ fontSize: '1.25rem', fontWeight: 700, color: '#fb923c', marginTop: '4px', fontFamily: 'var(--font-mono)' }}>
                  +{spikePercentage}%
                </div>
              </div>

              <div style={{ backgroundColor: 'var(--bg-card)', padding: '12px 14px', borderRadius: '8px', border: '1px solid var(--border-subtle)', minWidth: 0 }}>
                <div style={{ fontSize: '0.68rem', color: 'var(--text-muted)', textTransform: 'uppercase' }}>Z-Score (Confidence)</div>
                <div style={{ fontSize: '1.25rem', fontWeight: 700, color: 'var(--accent-amber)', marginTop: '4px', fontFamily: 'var(--font-mono)' }}>
                  {selectedAnomaly?.zScore} σ
                </div>
              </div>
            </div>

            {/* Root Cause Correlation Summary Banner */}
            <div
              style={{
                backgroundColor: 'rgba(239, 68, 68, 0.08)',
                border: '1px solid rgba(239, 68, 68, 0.25)',
                borderRadius: '10px',
                padding: '14px 16px',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px', flexWrap: 'wrap' }}>
                <GitBranch size={16} style={{ color: 'var(--status-critical)' }} />
                <span style={{ fontWeight: 600, color: '#fca5a5', fontSize: '0.85rem' }}>
                  Root Cause Dependency Chain:
                </span>
                <span className="badge critical" style={{ fontSize: '0.68rem' }}>
                  CULPRIT: {selectedAnomaly?.rootCauseService}
                </span>
              </div>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-primary)', lineHeight: 1.5 }}>
                {selectedAnomaly?.evidence}
              </div>
            </div>
          </div>

          {/* Core Problem Statement Deliverable: Spans Whose P99 Latency Spiked During Error Clusters */}
          <div className="panel-card" style={{ padding: 0, overflow: 'hidden', minWidth: 0 }}>
            <div
              style={{
                padding: '12px 18px',
                borderBottom: '1px solid var(--border-subtle)',
                backgroundColor: 'var(--bg-card)',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                flexWrap: 'wrap',
                gap: '8px',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <AlertTriangle size={16} style={{ color: 'var(--status-critical)' }} />
                <span style={{ fontWeight: 600, color: '#fff', fontSize: '0.85rem' }}>
                  Correlated Spans with P99 Latency Spikes ({correlatedSpans.length} Spans)
                </span>
              </div>
              <span className="badge critical" style={{ fontSize: '0.68rem' }}>
                Error Cluster Window
              </span>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column' }}>
              {correlatedSpans.length === 0 ? (
                <div style={{ padding: '20px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '0.82rem' }}>
                  No correlated spans matching the current anomaly window.
                </div>
              ) : (
                correlatedSpans.map((span) => {
                  return (
                    <div
                      key={span.spanId}
                      style={{
                        padding: '12px 18px',
                        borderBottom: '1px solid rgba(255, 255, 255, 0.04)',
                        backgroundColor: span.isSpike
                          ? 'rgba(239, 68, 68, 0.06)'
                          : 'transparent',
                        borderLeft: span.isSpike
                          ? '4px solid var(--status-critical)'
                          : '4px solid transparent',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        gap: '12px',
                        flexWrap: 'wrap',
                      }}
                    >
                      {/* Left: Service & Operation */}
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '3px', flex: 1, minWidth: '180px' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexWrap: 'wrap' }}>
                          <span
                            style={{
                              backgroundColor: 'rgba(255, 255, 255, 0.06)',
                              color: '#fff',
                              borderRadius: '4px',
                              padding: '2px 6px',
                              fontSize: '0.7rem',
                              fontWeight: 600,
                            }}
                          >
                            {span.serviceName}
                          </span>
                          <span style={{ fontWeight: 600, fontSize: '0.8rem', color: span.isError ? '#f87171' : '#fff' }}>
                            {span.name}
                          </span>
                        </div>
                        <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                          span_id: {span.spanId} | trace_id: {span.traceId.slice(0, 14)}...
                        </div>
                      </div>

                      {/* Middle: Latency Spike Highlight Indicator */}
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
                        {span.isSpike ? (
                          <div
                            style={{
                              display: 'flex',
                              alignItems: 'center',
                              gap: '6px',
                              backgroundColor: 'rgba(239, 68, 68, 0.15)',
                              border: '1px solid rgba(239, 68, 68, 0.4)',
                              borderRadius: '6px',
                              padding: '3px 8px',
                            }}
                          >
                            <TrendingUp size={12} style={{ color: '#f87171' }} />
                            <span style={{ color: '#fca5a5', fontWeight: 700, fontSize: '0.72rem' }}>
                              P99 SPIKE: {span.durationMs}ms ({span.spikeMultiplier}x)
                            </span>
                          </div>
                        ) : (
                          <span className="badge healthy" style={{ fontSize: '0.7rem' }}>
                            {span.durationMs}ms
                          </span>
                        )}

                        {/* HTTP Status Code */}
                        {span.httpStatusCode && (
                          <span
                            className={`badge ${span.httpStatusCode >= 400 ? 'critical' : 'healthy'}`}
                            style={{ fontSize: '0.68rem', fontFamily: 'var(--font-mono)', fontWeight: 600 }}
                          >
                            HTTP {span.httpStatusCode}
                          </span>
                        )}
                      </div>

                      {/* Right: Deep link to Flamegraph */}
                      <div style={{ flexShrink: 0 }}>
                        <Link
                          to={`/traces`}
                          className="btn-secondary"
                          style={{ padding: '4px 8px', fontSize: '0.72rem', gap: '4px' }}
                          title="Open in Distributed Flamegraph"
                        >
                          <Layers size={11} />
                          <span>View in Flamegraph</span>
                          <ArrowUpRight size={11} />
                        </Link>
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>

          {/* Concurrent Error Logs during Incident */}
          {correlatedLogs.length > 0 && (
            <div className="panel-card" style={{ padding: '14px 18px', minWidth: 0, overflow: 'hidden' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '10px' }}>
                <FileText size={15} style={{ color: 'var(--accent-cyan)' }} />
                <span style={{ fontWeight: 600, color: '#fff', fontSize: '0.84rem' }}>
                  Concurrent Error Logs During Latency Cluster
                </span>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', minWidth: 0 }}>
                {correlatedLogs.map((log, idx) => (
                  <div
                    key={idx}
                    style={{
                      backgroundColor: 'var(--bg-card)',
                      border: '1px solid var(--border-subtle)',
                      borderRadius: '6px',
                      padding: '8px 12px',
                      fontFamily: 'var(--font-mono)',
                      fontSize: '0.74rem',
                      display: 'flex',
                      gap: '8px',
                      alignItems: 'center',
                      minWidth: 0,
                    }}
                  >
                    <span className="badge critical" style={{ fontSize: '0.62rem', flexShrink: 0 }}>
                      {log.severityText}
                    </span>
                    <span style={{ color: 'var(--accent-emerald)', fontWeight: 600, flexShrink: 0 }}>[{log.serviceName}]</span>
                    <span
                      style={{
                        color: 'var(--text-secondary)',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                        minWidth: 0,
                        flex: 1,
                      }}
                    >
                      {log.body}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
