import React, { useState } from 'react';
import { Filter, Clock, AlertCircle, CheckCircle2, Layers, FileText, ArrowUpRight } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useTelemetry } from '../context/TelemetryContext';
import type { Span } from '../types/telemetry';

export const TracesPage: React.FC = () => {
  const { spans } = useTelemetry();
  const [selectedSpan, setSelectedSpan] = useState<Span>(spans[0]);
  const [filterService, setFilterService] = useState<string>('ALL');
  const [filterStatus, setFilterStatus] = useState<string>('ALL');

  const uniqueServices = Array.from(new Set(spans.map((s) => s.serviceName))).sort();

  const filteredSpans = spans.filter((s) => {
    if (filterService !== 'ALL' && s.serviceName !== filterService) return false;
    if (filterStatus !== 'ALL' && s.statusCode !== filterStatus) return false;
    return true;
  });

  const relatedSpans = spans.filter((s) => s.traceId === selectedSpan?.traceId);
  const maxDuration = Math.max(...relatedSpans.map((s) => s.durationNs), 1);
  const traceStartTimeMs = Math.min(...relatedSpans.map((s) => new Date(s.startTime).getTime()));

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Distributed Traces & Flamegraph</h1>
          <p className="page-desc">
            Inspect high-cardinality spans, waterfall timings, parent-child hierarchies, and error propagation.
          </p>
        </div>
      </div>

      {/* Filter Toolbar */}
      <div
        style={{
          display: 'flex',
          gap: '12px',
          alignItems: 'center',
          backgroundColor: 'var(--bg-surface)',
          padding: '12px 16px',
          borderRadius: '10px',
          border: '1px solid var(--border-subtle)',
          marginBottom: '20px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-muted)' }}>
          <Filter size={16} />
          <span style={{ fontSize: '0.8rem', fontWeight: 600 }}>Filters:</span>
        </div>

        {/* Service Select */}
        <select
          value={filterService}
          onChange={(e) => setFilterService(e.target.value)}
          style={{
            background: 'var(--bg-card)',
            color: 'var(--text-primary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '6px',
            padding: '6px 12px',
            fontSize: '0.8rem',
            outline: 'none',
          }}
        >
          <option value="ALL">All Services ({uniqueServices.length})</option>
          {uniqueServices.map((svc) => (
            <option key={svc} value={svc}>
              {svc}
            </option>
          ))}
        </select>

        {/* Status Select */}
        <select
          value={filterStatus}
          onChange={(e) => setFilterStatus(e.target.value)}
          style={{
            background: 'var(--bg-card)',
            color: 'var(--text-primary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '6px',
            padding: '6px 12px',
            fontSize: '0.8rem',
            outline: 'none',
          }}
        >
          <option value="ALL">All Statuses</option>
          <option value="OK">OK Only</option>
          <option value="ERROR">Errors Only</option>
        </select>

        <div style={{ marginLeft: 'auto', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
          Showing <strong>{filteredSpans.length}</strong> of {spans.length} spans
        </div>
      </div>

      {/* Two-Column Trace Explorer */}
      <div style={{ display: 'grid', gridTemplateColumns: '380px 1fr', gap: '20px', alignItems: 'start' }}>
        {/* Left: Span List */}
        <div className="panel-card" style={{ padding: 0, overflow: 'hidden' }}>
          <div style={{ padding: '16px', borderBottom: '1px solid var(--border-subtle)', fontWeight: 600, fontSize: '0.88rem' }}>
            Ingested Spans ({filteredSpans.length})
          </div>

          <div style={{ maxHeight: '640px', overflowY: 'auto' }}>
            {filteredSpans.map((span) => {
              const isSelected = selectedSpan?.spanId === span.spanId;
              const durationMs = Math.round(span.durationNs / 1000000);

              return (
                <div
                  key={span.spanId}
                  onClick={() => setSelectedSpan(span)}
                  style={{
                    padding: '14px 16px',
                    borderBottom: '1px solid var(--border-subtle)',
                    cursor: 'pointer',
                    backgroundColor: isSelected ? 'var(--bg-card-hover)' : 'transparent',
                    borderLeft: isSelected ? '3px solid var(--accent-emerald)' : '3px solid transparent',
                    transition: 'all 0.15s ease',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
                    <span style={{ fontWeight: 600, color: '#fff', fontSize: '0.85rem' }}>{span.serviceName}</span>
                    <span className={`badge ${span.statusCode === 'ERROR' ? 'critical' : 'healthy'}`}>
                      {span.statusCode === 'ERROR' ? <AlertCircle size={10} /> : <CheckCircle2 size={10} />}
                      {span.httpStatusCode || span.statusCode}
                    </span>
                  </div>

                  <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '8px' }}>
                    {span.name}
                  </div>

                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.74rem', color: 'var(--text-muted)' }}>
                    <span style={{ fontFamily: 'var(--font-mono)' }}>{span.traceId.slice(0, 14)}...</span>
                    <span style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                      <Clock size={12} /> {durationMs}ms
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Right: Trace Flamegraph / Waterfall & Detail Inspector */}
        <div className="panel-card">
          <div className="panel-title">
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Layers size={18} style={{ color: 'var(--accent-emerald)' }} />
              <span>Trace Waterfall Timeline</span>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              <span className="code-pill">trace_id: {selectedSpan?.traceId}</span>
              <Link
                to={`/logs?traceId=${selectedSpan?.traceId}`}
                className="btn-secondary"
                style={{ padding: '4px 10px', fontSize: '0.75rem', gap: '4px' }}
                title="Jump to correlated logs in Log Explorer"
              >
                <FileText size={12} />
                <span>Correlated Logs</span>
                <ArrowUpRight size={12} />
              </Link>
            </div>
          </div>

          {/* Waterfall Timeline */}
          <div style={{ marginBottom: '24px', backgroundColor: 'var(--bg-card)', padding: '16px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
            <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)', marginBottom: '14px', display: 'flex', justifyContent: 'space-between' }}>
              <span>Span Execution Tree ({relatedSpans.length} spans)</span>
              <span>Total Trace Duration: <strong>{Math.round(maxDuration / 1000000)}ms</strong></span>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              {relatedSpans.map((rSpan) => {
                const durationMs = Math.round(rSpan.durationNs / 1000000);
                const widthPercent = Math.max((rSpan.durationNs / maxDuration) * 100, 6);
                const isChild = rSpan.parentSpanId !== '';

                const spanStartTimeMs = new Date(rSpan.startTime).getTime();
                const totalTraceDurationMs = Math.max(maxDuration / 1000000, 1);
                const offsetMs = Math.max(0, spanStartTimeMs - traceStartTimeMs);
                const leftOffsetPercent = Math.min((offsetMs / totalTraceDurationMs) * 100, 85);

                return (
                  <div key={rSpan.spanId} style={{ marginLeft: isChild ? '24px' : '0' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.78rem', marginBottom: '5px' }}>
                      <span style={{ fontWeight: 600, color: rSpan.statusCode === 'ERROR' ? '#f87171' : 'var(--text-primary)' }}>
                        {isChild && '↳ '} {rSpan.serviceName}: {rSpan.name}
                      </span>
                      <span style={{ fontFamily: 'var(--font-mono)', fontSize: '0.74rem', color: 'var(--text-muted)' }}>
                        {durationMs}ms
                      </span>
                    </div>

                    {/* Timeline Track with Offset and Duration */}
                    <div style={{ height: '16px', width: '100%', backgroundColor: 'rgba(255,255,255,0.04)', borderRadius: '4px', overflow: 'hidden', position: 'relative' }}>
                      <div
                        style={{
                          position: 'relative',
                          left: `${leftOffsetPercent}%`,
                          height: '100%',
                          width: `${widthPercent}%`,
                          backgroundColor: rSpan.statusCode === 'ERROR' ? '#ef4444' : '#10b981',
                          borderRadius: '4px',
                          transition: 'all 0.3s ease',
                          boxShadow: rSpan.statusCode === 'ERROR' ? '0 0 10px rgba(239, 68, 68, 0.4)' : 'none',
                        }}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Span Details / Attributes */}
          <div style={{ borderTop: '1px solid var(--border-subtle)', paddingTop: '16px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
              <div style={{ fontWeight: 600, fontSize: '0.88rem', color: '#fff' }}>
                Span Attributes & W3C Trace Context
              </div>
              <span className={`badge ${selectedSpan?.statusCode === 'ERROR' ? 'critical' : 'healthy'}`}>
                {selectedSpan?.statusCode === 'ERROR' ? 'FAILED SPAN' : 'SUCCESS SPAN'}
              </span>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', fontSize: '0.8rem', marginBottom: '14px' }}>
              <div>
                <span style={{ color: 'var(--text-muted)' }}>Span ID:</span>{' '}
                <span className="code-pill">{selectedSpan?.spanId}</span>
              </div>
              <div>
                <span style={{ color: 'var(--text-muted)' }}>Parent Span ID:</span>{' '}
                <span className="code-pill">{selectedSpan?.parentSpanId || 'None (Root Span)'}</span>
              </div>
              <div>
                <span style={{ color: 'var(--text-muted)' }}>Kind:</span>{' '}
                <span className="badge neutral">{selectedSpan?.kind}</span>
              </div>
              <div>
                <span style={{ color: 'var(--text-muted)' }}>HTTP Status:</span>{' '}
                <span className="badge info">{selectedSpan?.httpStatusCode || 'N/A'}</span>
              </div>
              <div>
                <span style={{ color: 'var(--text-muted)' }}>Start Time:</span>{' '}
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: '0.74rem' }}>{selectedSpan?.startTime}</span>
              </div>
              <div>
                <span style={{ color: 'var(--text-muted)' }}>Status Message:</span>{' '}
                <span style={{ color: selectedSpan?.statusCode === 'ERROR' ? '#fca5a5' : 'inherit' }}>
                  {selectedSpan?.statusMessage || 'None'}
                </span>
              </div>
            </div>

            {/* Attributes Table */}
            <div style={{ backgroundColor: 'var(--bg-card)', padding: '12px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
              <div style={{ fontSize: '0.72rem', textTransform: 'uppercase', color: 'var(--text-muted)', marginBottom: '8px', fontWeight: 600 }}>
                Resource & Span Tags
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', fontFamily: 'var(--font-mono)', fontSize: '0.75rem' }}>
                {Object.entries({ ...selectedSpan?.resourceAttributes, ...selectedSpan?.attributes }).map(([k, v]) => (
                  <div key={k} style={{ display: 'flex', gap: '8px' }}>
                    <span style={{ color: 'var(--accent-cyan)' }}>{k}:</span>
                    <span style={{ color: 'var(--text-secondary)' }}>"{v}"</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
