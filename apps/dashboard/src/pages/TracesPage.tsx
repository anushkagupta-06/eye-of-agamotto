import React, { useState, useMemo } from 'react';
import {
  Layers,
  AlertCircle,
  CheckCircle2,
  FileText,
  ArrowUpRight,
  Activity,
  Info,
  GitCommit,
  Clock,
  Globe
} from 'lucide-react';
import { Link } from 'react-router-dom';
import { useTelemetry } from '../context/TelemetryContext';
import type { Span } from '../types/telemetry';

// Deterministic vibrant color per microservice name (like Jaeger / Datadog)
const SERVICE_COLOR_PALETTE: Record<string, string> = {
  'api-gateway': '#10b981',        // Emerald
  'ingress-gateway': '#10b981',
  'order-router': '#10b981',
  'telehealth-gateway': '#10b981',
  'auth-service': '#06b6d4',       // Cyan
  'user-service': '#8b5cf6',       // Violet
  'checkout-service': '#3b82f6',   // Blue
  'payment-gateway': '#f59e0b',    // Amber
  'matching-engine': '#ec4899',    // Pink
  'risk-engine': '#14b8a6',        // Teal
  'ledger-service': '#a855f7',     // Purple
  'inventory-service': '#10b981',  // Emerald
  'notification-worker': '#f97316',// Orange
  'billing-service': '#6366f1',    // Indigo
  'patient-records-api': '#06b6d4',
  'lab-diagnostic-engine': '#ef4444',
};

function getServiceColor(serviceName: string, isError: boolean): string {
  if (isError) return '#ef4444';
  if (SERVICE_COLOR_PALETTE[serviceName]) return SERVICE_COLOR_PALETTE[serviceName];

  let hash = 0;
  for (let i = 0; i < serviceName.length; i++) {
    hash = serviceName.charCodeAt(i) + ((hash << 5) - hash);
  }
  const hues = [160, 190, 210, 260, 280, 320, 35, 45];
  const hue = hues[Math.abs(hash) % hues.length];
  return `hsl(${hue}, 75%, 55%)`;
}

interface SpanTreeNode {
  span: Span;
  depth: number;
  parent: Span | null;
  children: SpanTreeNode[];
}

export const TracesPage: React.FC = () => {
  const { spans } = useTelemetry();

  // Group spans into distinct traces
  const tracesMap = useMemo(() => {
    const map = new Map<string, Span[]>();
    spans.forEach((span) => {
      const list = map.get(span.traceId) || [];
      list.push(span);
      map.set(span.traceId, list);
    });
    return map;
  }, [spans]);

  const traceSummaries = useMemo(() => {
    const summaries: Array<{
      traceId: string;
      rootService: string;
      rootOperation: string;
      spanCount: number;
      durationMs: number;
      hasError: boolean;
      httpStatusCode?: number;
      services: string[];
    }> = [];

    tracesMap.forEach((traceSpans, traceId) => {
      const rootSpan = traceSpans.find((s) => !s.parentSpanId) || traceSpans[0];
      const maxDuration = Math.max(...traceSpans.map((s) => s.durationNs), 1);
      const hasError = traceSpans.some((s) => s.statusCode === 'ERROR');
      const services = Array.from(new Set(traceSpans.map((s) => s.serviceName)));

      summaries.push({
        traceId,
        rootService: rootSpan.serviceName,
        rootOperation: rootSpan.name,
        spanCount: traceSpans.length,
        durationMs: Math.round(maxDuration / 1000000),
        hasError,
        httpStatusCode: rootSpan.httpStatusCode,
        services,
      });
    });

    return summaries.sort((a, b) => (b.hasError ? 1 : 0) - (a.hasError ? 1 : 0));
  }, [tracesMap]);

  // Selected Trace State
  const [activeTraceId, setActiveTraceId] = useState<string>(() => {
    return traceSummaries[0]?.traceId || spans[0]?.traceId || '';
  });

  // Filter Trace List
  const [filterMode, setFilterMode] = useState<'ALL' | 'ERRORS'>('ALL');

  const visibleSummaries = useMemo(() => {
    if (filterMode === 'ERRORS') {
      return traceSummaries.filter((t) => t.hasError);
    }
    return traceSummaries;
  }, [traceSummaries, filterMode]);

  // Active trace spans
  const activeTraceSpans = useMemo(() => {
    return tracesMap.get(activeTraceId) || tracesMap.get(traceSummaries[0]?.traceId) || [];
  }, [tracesMap, activeTraceId, traceSummaries]);

  // Calculate total trace timing
  const traceStats = useMemo(() => {
    if (activeTraceSpans.length === 0) return { minTime: 0, maxDurationNs: 1, totalDurationMs: 0 };
    const minTime = Math.min(...activeTraceSpans.map((s) => new Date(s.startTime).getTime()));
    const maxDurationNs = Math.max(...activeTraceSpans.map((s) => s.durationNs), 1);
    const totalDurationMs = Math.round(maxDurationNs / 1000000);
    return { minTime, maxDurationNs, totalDurationMs };
  }, [activeTraceSpans]);

  // Build hierarchical span tree with parent-child links
  const spanTree = useMemo(() => {
    const spanMap = new Map<string, SpanTreeNode>();
    activeTraceSpans.forEach((s) => {
      spanMap.set(s.spanId, { span: s, depth: 0, parent: null, children: [] });
    });

    const roots: SpanTreeNode[] = [];

    activeTraceSpans.forEach((s) => {
      const node = spanMap.get(s.spanId)!;
      if (s.parentSpanId && spanMap.has(s.parentSpanId)) {
        const parentNode = spanMap.get(s.parentSpanId)!;
        node.parent = parentNode.span;
        parentNode.children.push(node);
      } else {
        roots.push(node);
      }
    });

    // Flatten in depth-first order
    const flattened: Array<{ span: Span; depth: number; parent: Span | null }> = [];
    const traverse = (node: SpanTreeNode, depth: number) => {
      flattened.push({ span: node.span, depth, parent: node.parent });
      node.children
        .sort((a, b) => new Date(a.span.startTime).getTime() - new Date(b.span.startTime).getTime())
        .forEach((child) => traverse(child, depth + 1));
    };

    roots
      .sort((a, b) => new Date(a.span.startTime).getTime() - new Date(b.span.startTime).getTime())
      .forEach((root) => traverse(root, 0));

    return flattened;
  }, [activeTraceSpans]);

  // Selected Span for details inspection
  const [selectedSpanId, setSelectedSpanId] = useState<string>(() => {
    const errorSpan = activeTraceSpans.find((s) => s.statusCode === 'ERROR');
    return errorSpan?.spanId || activeTraceSpans[0]?.spanId || '';
  });

  const selectedSpanNode = useMemo(() => {
    return spanTree.find((n) => n.span.spanId === selectedSpanId) || spanTree[0];
  }, [spanTree, selectedSpanId]);

  const selectedSpan = selectedSpanNode?.span;

  // Helper to render HTTP status code badge
  const renderHttpBadge = (httpStatusCode: number | undefined, statusCode: string) => {
    if (httpStatusCode && httpStatusCode > 0) {
      const isErr = httpStatusCode >= 400 || statusCode === 'ERROR';
      const isWarn = httpStatusCode >= 300 && httpStatusCode < 400;
      return (
        <span
          className={`badge ${isErr ? 'critical' : isWarn ? 'warning' : 'healthy'}`}
          style={{ fontSize: '0.68rem', padding: '2px 6px', fontFamily: 'var(--font-mono)', fontWeight: 600 }}
        >
          HTTP {httpStatusCode}
        </span>
      );
    }
    return (
      <span
        className={`badge ${statusCode === 'ERROR' ? 'critical' : 'healthy'}`}
        style={{ fontSize: '0.68rem', padding: '2px 6px' }}
      >
        {statusCode}
      </span>
    );
  };

  return (
    <div>
      {/* Page Title & Trace Switcher Header */}
      <div className="page-header" style={{ marginBottom: '14px' }}>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
            <h1 className="page-title">Distributed Trace Flamegraph</h1>
            <span className="badge healthy" style={{ fontSize: '0.72rem' }}>
              <Layers size={11} /> OpenTelemetry
            </span>
          </div>
          <p className="page-desc">
            Interactive waterfall timeline visualizing spans, parent-child relationships, durations, and HTTP status codes.
          </p>
        </div>

        {/* Action deep-links */}
        <div style={{ display: 'flex', gap: '10px' }}>
          <Link
            to={`/logs?traceId=${activeTraceId}`}
            className="btn-secondary"
            style={{ fontSize: '0.78rem', gap: '6px' }}
            title="Jump to correlated logs in Log Explorer"
          >
            <FileText size={14} />
            <span>Correlated Logs</span>
            <ArrowUpRight size={13} />
          </Link>
        </div>
      </div>

      {/* Trace Selector Pill Carousel */}
      <div
        className="panel-card"
        style={{
          padding: '12px 16px',
          marginBottom: '16px',
          display: 'flex',
          alignItems: 'center',
          gap: '12px',
          flexWrap: 'wrap',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-muted)', fontSize: '0.8rem', fontWeight: 600 }}>
          <Activity size={15} style={{ color: 'var(--accent-emerald)' }} />
          <span>Active Traces:</span>
        </div>

        {/* Filter button */}
        <div style={{ display: 'flex', gap: '6px' }}>
          <button
            type="button"
            className={`btn-secondary ${filterMode === 'ALL' ? 'active' : ''}`}
            onClick={() => setFilterMode('ALL')}
            style={{ padding: '3px 9px', fontSize: '0.72rem' }}
          >
            All ({traceSummaries.length})
          </button>
          <button
            type="button"
            className={`btn-secondary ${filterMode === 'ERRORS' ? 'active' : ''}`}
            onClick={() => setFilterMode('ERRORS')}
            style={{ padding: '3px 9px', fontSize: '0.72rem', color: filterMode === 'ERRORS' ? '#f87171' : 'inherit' }}
          >
            Errors ({traceSummaries.filter((t) => t.hasError).length})
          </button>
        </div>

        {/* Trace Quick Pills */}
        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', flex: 1, minWidth: '300px' }}>
          {visibleSummaries.map((t) => {
            const isActive = t.traceId === activeTraceId;
            return (
              <button
                key={t.traceId}
                type="button"
                onClick={() => {
                  setActiveTraceId(t.traceId);
                  const firstSpan = tracesMap.get(t.traceId)?.[0];
                  if (firstSpan) setSelectedSpanId(firstSpan.spanId);
                }}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                  padding: '6px 12px',
                  borderRadius: '8px',
                  backgroundColor: isActive ? 'rgba(16, 185, 129, 0.15)' : 'var(--bg-card)',
                  border: isActive ? '1px solid var(--accent-emerald)' : '1px solid var(--border-subtle)',
                  color: isActive ? '#fff' : 'var(--text-secondary)',
                  cursor: 'pointer',
                  fontSize: '0.75rem',
                  fontWeight: 500,
                  transition: 'all 0.15s ease',
                }}
              >
                {t.hasError ? (
                  <AlertCircle size={13} style={{ color: '#f87171' }} />
                ) : (
                  <CheckCircle2 size={13} style={{ color: '#10b981' }} />
                )}
                <span style={{ fontWeight: 600 }}>{t.rootService}</span>
                <span style={{ color: 'var(--text-muted)' }}>({t.durationMs}ms)</span>
                {t.httpStatusCode && (
                  <span style={{ fontFamily: 'var(--font-mono)', fontSize: '0.68rem', color: t.httpStatusCode >= 400 ? '#f87171' : 'var(--accent-emerald)' }}>
                    HTTP {t.httpStatusCode}
                  </span>
                )}
                <span className="code-pill" style={{ fontSize: '0.65rem' }}>
                  {t.spanCount} spans
                </span>
              </button>
            );
          })}
        </div>
      </div>

      {/* Main Full-Width Distributed Trace Flamegraph Canvas */}
      <div className="panel-card" style={{ padding: '0', overflow: 'hidden', marginBottom: '20px' }}>
        {/* Flamegraph Top Header */}
        <div
          style={{
            padding: '14px 20px',
            borderBottom: '1px solid var(--border-subtle)',
            backgroundColor: 'var(--bg-card)',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            flexWrap: 'wrap',
            gap: '12px',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <Layers size={18} style={{ color: 'var(--accent-emerald)' }} />
            <div>
              <span style={{ fontWeight: 600, color: '#fff', fontSize: '0.9rem' }}>
                Interactive Waterfall Timeline
              </span>
              <span style={{ color: 'var(--text-muted)', fontSize: '0.75rem', marginLeft: '10px' }}>
                trace_id: <code style={{ color: 'var(--accent-cyan)' }}>{activeTraceId}</code>
              </span>
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '14px', fontSize: '0.78rem' }}>
            <div>
              <span style={{ color: 'var(--text-muted)' }}>Spans:</span>{' '}
              <strong style={{ color: '#fff' }}>{activeTraceSpans.length}</strong>
            </div>
            <div>
              <span style={{ color: 'var(--text-muted)' }}>Total Duration:</span>{' '}
              <strong style={{ color: 'var(--accent-emerald)' }}>{traceStats.totalDurationMs}ms</strong>
            </div>
          </div>
        </div>

        {/* Horizontal Timeline Ruler / Scale */}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '430px 1fr',
            borderBottom: '1px solid var(--border-subtle)',
            backgroundColor: 'rgba(255, 255, 255, 0.02)',
            padding: '8px 20px',
            fontSize: '0.72rem',
            color: 'var(--text-muted)',
            fontFamily: 'var(--font-mono)',
          }}
        >
          <div>Spans, Parent-Child Tree & HTTP Status</div>
          <div style={{ display: 'flex', justifyContent: 'space-between', paddingLeft: '8px', paddingRight: '8px' }}>
            <span>0ms</span>
            <span>{Math.round(traceStats.totalDurationMs * 0.25)}ms</span>
            <span>{Math.round(traceStats.totalDurationMs * 0.5)}ms</span>
            <span>{Math.round(traceStats.totalDurationMs * 0.75)}ms</span>
            <span>{traceStats.totalDurationMs}ms</span>
          </div>
        </div>

        {/* Flamegraph Waterfall Rows */}
        <div style={{ maxHeight: '520px', overflowY: 'auto' }}>
          {spanTree.map(({ span, depth }) => {
            const isSelected = selectedSpan?.spanId === span.spanId;
            const isError = span.statusCode === 'ERROR';
            const durationMs = Math.round(span.durationNs / 1000000);

            // Compute precise start offset & duration width %
            const spanStartTime = new Date(span.startTime).getTime();
            const offsetMs = Math.max(0, spanStartTime - traceStats.minTime);
            const totalMs = Math.max(traceStats.totalDurationMs, 1);
            const leftPercent = Math.min((offsetMs / totalMs) * 100, 92);
            const widthPercent = Math.max((durationMs / totalMs) * 100, 3);

            const serviceColor = getServiceColor(span.serviceName, isError);

            return (
              <div
                key={span.spanId}
                onClick={() => setSelectedSpanId(span.spanId)}
                style={{
                  display: 'grid',
                  gridTemplateColumns: '430px 1fr',
                  padding: '9px 20px',
                  borderBottom: '1px solid rgba(255, 255, 255, 0.04)',
                  backgroundColor: isSelected ? 'rgba(16, 185, 129, 0.08)' : 'transparent',
                  borderLeft: isSelected ? '3px solid var(--accent-emerald)' : '3px solid transparent',
                  cursor: 'pointer',
                  transition: 'all 0.12s ease',
                  alignItems: 'center',
                }}
              >
                {/* Left Column: Indented Tree Node + Parent-Child Indicators + HTTP Badge */}
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '7px',
                    paddingLeft: `${depth * 20}px`,
                    overflow: 'hidden',
                  }}
                >
                  {/* Parent-child depth marker */}
                  {depth === 0 ? (
                    <span className="badge neutral" style={{ fontSize: '0.62rem', padding: '1px 5px', fontWeight: 700 }}>
                      ROOT
                    </span>
                  ) : (
                    <span style={{ color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', fontSize: '0.75rem', fontWeight: 600 }}>
                      ↳
                    </span>
                  )}

                  {/* Microservice Chip */}
                  <span
                    style={{
                      backgroundColor: `${serviceColor}20`,
                      color: serviceColor,
                      border: `1px solid ${serviceColor}50`,
                      borderRadius: '4px',
                      padding: '2px 6px',
                      fontSize: '0.68rem',
                      fontWeight: 600,
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {span.serviceName}
                  </span>

                  {/* Operation Name */}
                  <span
                    style={{
                      fontSize: '0.78rem',
                      color: isError ? '#f87171' : '#fff',
                      fontWeight: isSelected ? 600 : 400,
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                      maxWidth: '150px',
                    }}
                    title={span.name}
                  >
                    {span.name}
                  </span>

                  {/* HTTP Status Code Badge */}
                  {renderHttpBadge(span.httpStatusCode, span.statusCode)}

                  {/* Execution Duration Pill */}
                  <span style={{ fontFamily: 'var(--font-mono)', fontSize: '0.72rem', color: 'var(--text-muted)', marginLeft: 'auto' }}>
                    {durationMs}ms
                  </span>
                </div>

                {/* Right Column: Interactive Flamegraph Bar */}
                <div style={{ position: 'relative', height: '24px', display: 'flex', alignItems: 'center' }}>
                  {/* Grid Vertical Guide Lines */}
                  <div style={{ position: 'absolute', inset: 0, display: 'flex', justifyContent: 'space-between', opacity: 0.15, pointerEvents: 'none' }}>
                    <div style={{ borderLeft: '1px dashed #fff', height: '100%' }} />
                    <div style={{ borderLeft: '1px dashed #fff', height: '100%' }} />
                    <div style={{ borderLeft: '1px dashed #fff', height: '100%' }} />
                    <div style={{ borderLeft: '1px dashed #fff', height: '100%' }} />
                    <div style={{ borderLeft: '1px dashed #fff', height: '100%' }} />
                  </div>

                  {/* The Actual Flamegraph Bar */}
                  <div
                    style={{
                      position: 'absolute',
                      left: `${leftPercent}%`,
                      width: `${widthPercent}%`,
                      height: '18px',
                      backgroundColor: serviceColor,
                      borderRadius: '4px',
                      boxShadow: isSelected
                        ? `0 0 12px ${serviceColor}`
                        : isError
                        ? '0 0 10px rgba(239, 68, 68, 0.4)'
                        : 'none',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '0 6px',
                      color: '#000',
                      fontSize: '0.68rem',
                      fontWeight: 700,
                      transition: 'all 0.2s ease',
                    }}
                    title={`${span.serviceName}: ${span.name} (${durationMs}ms) [HTTP ${span.httpStatusCode || span.statusCode}]`}
                  >
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {durationMs}ms
                    </span>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Selected Span Details Inspector */}
      {selectedSpan && (
        <div className="panel-card" style={{ padding: '18px 22px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px', borderBottom: '1px solid var(--border-subtle)', paddingBottom: '10px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Info size={16} style={{ color: 'var(--accent-emerald)' }} />
              <span style={{ fontWeight: 600, fontSize: '0.9rem', color: '#fff' }}>
                Span Inspector: <span style={{ color: 'var(--accent-cyan)' }}>{selectedSpan.serviceName}</span> — {selectedSpan.name}
              </span>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              {renderHttpBadge(selectedSpan.httpStatusCode, selectedSpan.statusCode)}
              <span className={`badge ${selectedSpan.statusCode === 'ERROR' ? 'critical' : 'healthy'}`}>
                {selectedSpan.statusCode === 'ERROR' ? 'STATUS: ERROR' : 'STATUS: OK'}
              </span>
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '14px', fontSize: '0.8rem', marginBottom: '16px' }}>
            {/* Parent-Child Relationship Details */}
            <div style={{ background: 'var(--bg-card)', padding: '10px 12px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
              <div style={{ color: 'var(--text-muted)', fontSize: '0.72rem', display: 'flex', alignItems: 'center', gap: '4px' }}>
                <GitCommit size={12} /> Parent-Child Hierarchy
              </div>
              <div style={{ color: '#fff', fontSize: '0.78rem', marginTop: '2px' }}>
                {selectedSpanNode?.parent ? (
                  <span>
                    Child of <strong style={{ color: 'var(--accent-cyan)' }}>{selectedSpanNode.parent.serviceName}</strong> (
                    <code style={{ fontSize: '0.72rem' }}>{selectedSpanNode.parent.spanId.slice(0, 8)}</code>)
                  </span>
                ) : (
                  <strong style={{ color: 'var(--accent-emerald)' }}>Root Span (Initiating Call)</strong>
                )}
              </div>
            </div>

            <div style={{ background: 'var(--bg-card)', padding: '10px 12px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
              <div style={{ color: 'var(--text-muted)', fontSize: '0.72rem' }}>Span ID</div>
              <code style={{ color: '#fff', fontSize: '0.78rem' }}>{selectedSpan.spanId}</code>
            </div>

            <div style={{ background: 'var(--bg-card)', padding: '10px 12px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
              <div style={{ color: 'var(--text-muted)', fontSize: '0.72rem', display: 'flex', alignItems: 'center', gap: '4px' }}>
                <Clock size={12} /> Execution Duration
              </div>
              <strong style={{ color: 'var(--accent-emerald)', fontSize: '0.85rem' }}>
                {Math.round(selectedSpan.durationNs / 1000000)}ms ({selectedSpan.durationNs.toLocaleString()} ns)
              </strong>
            </div>

            <div style={{ background: 'var(--bg-card)', padding: '10px 12px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
              <div style={{ color: 'var(--text-muted)', fontSize: '0.72rem', display: 'flex', alignItems: 'center', gap: '4px' }}>
                <Globe size={12} /> HTTP Status Code
              </div>
              <strong style={{ color: selectedSpan.httpStatusCode && selectedSpan.httpStatusCode >= 400 ? '#f87171' : 'var(--accent-emerald)' }}>
                {selectedSpan.httpStatusCode ? `HTTP ${selectedSpan.httpStatusCode}` : 'Non-HTTP'}
              </strong>
            </div>
          </div>

          {/* Span Attributes / Tags */}
          <div style={{ background: 'var(--bg-card)', padding: '12px 14px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
            <div style={{ fontSize: '0.72rem', textTransform: 'uppercase', color: 'var(--text-muted)', marginBottom: '8px', fontWeight: 600 }}>
              OpenTelemetry Attributes & Tags
            </div>
            <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
              {Object.entries({ ...selectedSpan.resourceAttributes, ...selectedSpan.attributes }).map(([k, v]) => (
                <span
                  key={k}
                  style={{
                    backgroundColor: 'rgba(255, 255, 255, 0.04)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '6px',
                    padding: '3px 8px',
                    fontSize: '0.74rem',
                    fontFamily: 'var(--font-mono)',
                  }}
                >
                  <span style={{ color: 'var(--accent-cyan)' }}>{k}</span> = <span style={{ color: '#fff' }}>"{v}"</span>
                </span>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
