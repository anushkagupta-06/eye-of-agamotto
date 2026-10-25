import React, { useState, useEffect } from 'react';
import {
  Search,
  ExternalLink,
  AlertCircle,
  Info,
  AlertTriangle,
  Radio,
  Copy,
  Check,
  ChevronDown,
  ChevronRight,
  Filter,
  RefreshCw,
  Sparkles,
  SlidersHorizontal,
  Code2
} from 'lucide-react';
import { Link } from 'react-router-dom';
import { useTelemetry } from '../context/TelemetryContext';
import type { LogRecord } from '../types/telemetry';

export const LogsPage: React.FC = () => {
  const { logs: initialLogs } = useTelemetry();
  const [logs, setLogs] = useState<LogRecord[]>(initialLogs);

  // Filter States
  const [searchTerm, setSearchTerm] = useState('');
  const [isRegexMode, setIsRegexMode] = useState(false);
  const [regexError, setRegexError] = useState<string | null>(null);
  const [severityFilter, setSeverityFilter] = useState<string>('ALL');
  const [selectedService, setSelectedService] = useState<string>('ALL');
  const [hasTraceOnly, setHasTraceOnly] = useState<boolean>(false);

  // Live Tail State
  const [isLiveTail, setIsLiveTail] = useState(true);

  // Dynamically derive unique services from the current logs stream
  const uniqueServices = Array.from(new Set(logs.map((l) => l.serviceName))).sort();

  // Expanded Row State
  const [expandedLogIdx, setExpandedLogIdx] = useState<number | null>(null);
  const [copiedIdx, setCopiedIdx] = useState<number | null>(null);

  // Live tail simulation: appends fresh log periodically if live tail is active
  useEffect(() => {
    if (!isLiveTail) return;
    const interval = setInterval(() => {
      const services = ['checkout-service', 'payment-gateway', 'inventory-service', 'auth-service', 'ingestion-gateway'];
      const randomService = services[Math.floor(Math.random() * services.length)];
      const severities: LogRecord['severityText'][] = ['INFO', 'INFO', 'DEBUG', 'WARN', 'ERROR'];
      const randomSeverity = severities[Math.floor(Math.random() * severities.length)];

      const sampleBodies: Record<LogRecord['severityText'], string[]> = {
        INFO: [
          `Batch ingestion committed: published 16 records to stream TELEMETRY`,
          `Cache hit on redis cluster replica (latency: 120us)`,
          `HTTP request completed with status 200 OK`,
          `Refreshed tenant auth cache for demo tenant`,
        ],
        DEBUG: [
          `Evaluating bloom filter on table observability.logs`,
          `NATS JetStream ping acknowledgment received (roundtrip: 0.3ms)`,
          `Connection pool reused for worker thread #14`,
        ],
        WARN: [
          `Upstream latency approaching circuit breaker threshold: 190ms`,
          `Worker queue depth elevated: 86 pending tasks`,
          `Rate limit token bucket 80% consumed for tenant demo`,
        ],
        ERROR: [
          `Downstream call failed: connection timeout after 200ms`,
          `HTTP 504 Gateway Timeout from external payment processor`,
          `PostgreSQL query execution exceeded 500ms deadline`,
        ],
        FATAL: [
          `Fatal panic in worker loop: unhandled nil pointer exception`,
        ],
      };

      const messages = sampleBodies[randomSeverity];
      const randomBody = messages[Math.floor(Math.random() * messages.length)];
      const hasTrace = Math.random() > 0.4;

      const newLog: LogRecord = {
        tenantId: 'demo',
        timestamp: new Date().toISOString(),
        serviceName: randomService,
        severityText: randomSeverity,
        severityNumber: randomSeverity === 'ERROR' ? 17 : randomSeverity === 'WARN' ? 13 : 9,
        body: randomBody,
        traceId: hasTrace ? '5b8efff798038103d269b633813fc60c' : '',
        spanId: hasTrace ? 'eee19b7ec3c1b174' : '',
        attributes: {
          runtime: 'production',
          worker_id: `wrk_${Math.floor(Math.random() * 100)}`,
        },
        resourceAttributes: {
          'service.name': randomService,
          'host.name': 'prod-node-01',
        },
      };

      setLogs((prev) => [newLog, ...prev.slice(0, 79)]);
    }, 4000);

    return () => clearInterval(interval);
  }, [isLiveTail]);

  // Filter logs logic
  const filteredLogs = logs.filter((log) => {
    if (severityFilter !== 'ALL' && log.severityText !== severityFilter) return false;
    if (selectedService !== 'ALL' && log.serviceName !== selectedService) return false;
    if (hasTraceOnly && !log.traceId) return false;

    if (searchTerm.trim()) {
      if (isRegexMode) {
        try {
          const regex = new RegExp(searchTerm, 'i');
          const matchesBody = regex.test(log.body);
          const matchesTrace = log.traceId ? regex.test(log.traceId) : false;
          const matchesService = regex.test(log.serviceName);
          if (!matchesBody && !matchesTrace && !matchesService) return false;
        } catch {
          // If regex is syntactically invalid, keep all and show warning
          return true;
        }
      } else {
        const query = searchTerm.toLowerCase();
        const matchesBody = log.body.toLowerCase().includes(query);
        const matchesTrace = log.traceId ? log.traceId.toLowerCase().includes(query) : false;
        const matchesService = log.serviceName.toLowerCase().includes(query);
        if (!matchesBody && !matchesTrace && !matchesService) return false;
      }
    }
    return true;
  });

  // Validate regex syntax on input change
  const handleSearchChange = (val: string) => {
    setSearchTerm(val);
    if (isRegexMode && val.trim()) {
      try {
        new RegExp(val);
        setRegexError(null);
      } catch (err: unknown) {
        if (err instanceof Error) {
          setRegexError(err.message);
        } else {
          setRegexError('Invalid regular expression');
        }
      }
    } else {
      setRegexError(null);
    }
  };

  const copyLogJSON = (log: LogRecord, idx: number) => {
    navigator.clipboard.writeText(JSON.stringify(log, null, 2));
    setCopiedIdx(idx);
    setTimeout(() => setCopiedIdx(null), 1800);
  };

  const getSeverityBadge = (severity: LogRecord['severityText']) => {
    switch (severity) {
      case 'ERROR':
      case 'FATAL':
        return (
          <span className="badge critical">
            <AlertCircle size={10} /> {severity}
          </span>
        );
      case 'WARN':
        return (
          <span className="badge degraded">
            <AlertTriangle size={10} /> {severity}
          </span>
        );
      case 'DEBUG':
        return (
          <span className="badge neutral">
            <Code2 size={10} /> {severity}
          </span>
        );
      case 'INFO':
      default:
        return (
          <span className="badge info">
            <Info size={10} /> {severity}
          </span>
        );
    }
  };

  // Compute severity counts
  const countError = logs.filter((l) => l.severityText === 'ERROR' || l.severityText === 'FATAL').length;
  const countWarn = logs.filter((l) => l.severityText === 'WARN').length;
  const countInfo = logs.filter((l) => l.severityText === 'INFO').length;
  const countDebug = logs.filter((l) => l.severityText === 'DEBUG').length;

  return (
    <div>
      {/* Header */}
      <div className="page-header">
        <div>
          <h1 className="page-title">Live Log Explorer</h1>
          <p className="page-desc">
            High-cardinality structured log stream indexed in ClickHouse <code>observability.logs</code> with <code>tokenbf_v1</code> bloom filters and ZSTD(3) compression.
          </p>
        </div>

        <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
          {/* Live Tail Toggle Button */}
          <button
            className={`btn-secondary ${isLiveTail ? 'active' : ''}`}
            onClick={() => setIsLiveTail(!isLiveTail)}
            style={{
              borderColor: isLiveTail ? 'var(--accent-emerald)' : 'var(--border-subtle)',
              color: isLiveTail ? 'var(--accent-emerald)' : 'var(--text-secondary)',
              gap: '8px',
            }}
          >
            <Radio size={14} className={isLiveTail ? 'pulse' : ''} />
            <span>{isLiveTail ? 'Live Tail (Active)' : 'Live Tail (Paused)'}</span>
          </button>

          <button
            className="btn-secondary"
            onClick={() => setLogs(initialLogs)}
            title="Reset to baseline dataset"
          >
            <RefreshCw size={14} />
            <span>Reset</span>
          </button>
        </div>
      </div>

      {/* Log Volume Frequency Breakdown */}
      <div className="panel-card" style={{ marginBottom: '20px', padding: '16px 20px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <SlidersHorizontal size={15} style={{ color: 'var(--accent-emerald)' }} />
            <span style={{ fontSize: '0.84rem', fontWeight: 600, color: '#fff' }}>Log Stream Volume & Severity Distribution</span>
          </div>
          <div style={{ display: 'flex', gap: '14px', fontSize: '0.74rem' }}>
            <span style={{ display: 'flex', alignItems: 'center', gap: '4px', color: '#f87171' }}>
              <span style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: '#ef4444' }} /> {countError} Errors
            </span>
            <span style={{ display: 'flex', alignItems: 'center', gap: '4px', color: '#fbbf24' }}>
              <span style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: '#f59e0b' }} /> {countWarn} Warnings
            </span>
            <span style={{ display: 'flex', alignItems: 'center', gap: '4px', color: '#60a5fa' }}>
              <span style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: '#3b82f6' }} /> {countInfo} Info
            </span>
            <span style={{ display: 'flex', alignItems: 'center', gap: '4px', color: 'var(--text-muted)' }}>
              <span style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: 'var(--border-medium)' }} /> {countDebug} Debug
            </span>
          </div>
        </div>

        {/* Visual Histogram Bar */}
        <div style={{ height: '10px', width: '100%', backgroundColor: 'var(--bg-card)', borderRadius: '6px', overflow: 'hidden', display: 'flex' }}>
          <div style={{ width: `${(countError / logs.length) * 100}%`, backgroundColor: '#ef4444', transition: 'width 0.3s ease' }} title={`${countError} Errors`} />
          <div style={{ width: `${(countWarn / logs.length) * 100}%`, backgroundColor: '#f59e0b', transition: 'width 0.3s ease' }} title={`${countWarn} Warnings`} />
          <div style={{ width: `${(countInfo / logs.length) * 100}%`, backgroundColor: '#3b82f6', transition: 'width 0.3s ease' }} title={`${countInfo} Info`} />
          <div style={{ width: `${(countDebug / logs.length) * 100}%`, backgroundColor: '#475569', transition: 'width 0.3s ease' }} title={`${countDebug} Debug`} />
        </div>
      </div>

      {/* Main Filter & Search Control Panel */}
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          gap: '12px',
          backgroundColor: 'var(--bg-surface)',
          padding: '16px',
          borderRadius: '12px',
          border: '1px solid var(--border-subtle)',
          marginBottom: '20px',
        }}
      >
        {/* Search Bar + Regex Toggle */}
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
          <div
            style={{
              flex: 1,
              display: 'flex',
              alignItems: 'center',
              gap: '10px',
              background: 'var(--bg-card)',
              padding: '10px 14px',
              borderRadius: '8px',
              border: regexError ? '1px solid var(--status-critical)' : '1px solid var(--border-subtle)',
            }}
          >
            <Search size={16} style={{ color: 'var(--text-muted)' }} />
            <input
              type="text"
              placeholder={isRegexMode ? 'Enter Regular Expression (e.g. timeout.*504|redis.*replica)...' : 'Search log body, message text, trace ID...'}
              value={searchTerm}
              onChange={(e) => handleSearchChange(e.target.value)}
              style={{
                background: 'transparent',
                border: 'none',
                outline: 'none',
                color: 'var(--text-primary)',
                width: '100%',
                fontSize: '0.85rem',
                fontFamily: isRegexMode ? 'var(--font-mono)' : 'inherit',
              }}
            />
            {searchTerm && (
              <button
                onClick={() => handleSearchChange('')}
                style={{ background: 'transparent', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', fontSize: '0.8rem' }}
              >
                Clear
              </button>
            )}
          </div>

          {/* Regex Mode Switcher */}
          <button
            onClick={() => {
              setIsRegexMode(!isRegexMode);
              if (!isRegexMode && searchTerm) handleSearchChange(searchTerm);
            }}
            className={`btn-secondary ${isRegexMode ? 'active' : ''}`}
            style={{
              padding: '10px 14px',
              fontFamily: 'var(--font-mono)',
              fontSize: '0.8rem',
              borderColor: isRegexMode ? 'var(--accent-purple)' : 'var(--border-subtle)',
              color: isRegexMode ? 'var(--accent-purple)' : 'var(--text-secondary)',
            }}
            title="Toggle regular expression mode"
          >
            <span>.* RegExp</span>
          </button>
        </div>

        {/* Syntax Error Warning if invalid regex */}
        {regexError && (
          <div style={{ fontSize: '0.75rem', color: '#f87171', display: 'flex', alignItems: 'center', gap: '6px', paddingLeft: '4px' }}>
            <AlertCircle size={13} />
            <span>Regex Error: {regexError}</span>
          </div>
        )}

        {/* Quick Filter Pills Bar */}
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center', flexWrap: 'wrap', paddingTop: '4px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', color: 'var(--text-muted)', fontSize: '0.78rem' }}>
            <Filter size={14} />
            <span>Severity:</span>
          </div>

          {(['ALL', 'ERROR', 'WARN', 'INFO', 'DEBUG'] as const).map((sev) => {
            const isSelected = severityFilter === sev;
            return (
              <button
                key={sev}
                onClick={() => setSeverityFilter(sev)}
                style={{
                  background: isSelected ? 'var(--bg-card-hover)' : 'var(--bg-card)',
                  color: isSelected ? '#fff' : 'var(--text-secondary)',
                  border: isSelected ? '1px solid var(--accent-emerald)' : '1px solid var(--border-subtle)',
                  borderRadius: '6px',
                  padding: '4px 10px',
                  fontSize: '0.76rem',
                  cursor: 'pointer',
                  fontWeight: isSelected ? 600 : 400,
                  transition: 'all 0.15s ease',
                }}
              >
                {sev}
              </button>
            );
          })}

          <div style={{ width: '1px', height: '18px', backgroundColor: 'var(--border-subtle)', margin: '0 4px' }} />

          {/* Service Dropdown */}
          <select
            value={selectedService}
            onChange={(e) => setSelectedService(e.target.value)}
            style={{
              background: 'var(--bg-card)',
              color: 'var(--text-primary)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '6px',
              padding: '5px 10px',
              fontSize: '0.76rem',
              outline: 'none',
              cursor: 'pointer',
            }}
          >
            <option value="ALL">All Services ({uniqueServices.length})</option>
            {uniqueServices.map((svc) => (
              <option key={svc} value={svc}>
                {svc}
              </option>
            ))}
          </select>

          {/* Trace Only Filter Pill */}
          <button
            onClick={() => setHasTraceOnly(!hasTraceOnly)}
            style={{
              background: hasTraceOnly ? 'rgba(6, 182, 212, 0.15)' : 'var(--bg-card)',
              color: hasTraceOnly ? 'var(--accent-cyan)' : 'var(--text-secondary)',
              border: hasTraceOnly ? '1px solid var(--accent-cyan)' : '1px solid var(--border-subtle)',
              borderRadius: '6px',
              padding: '4px 10px',
              fontSize: '0.76rem',
              cursor: 'pointer',
              fontWeight: hasTraceOnly ? 600 : 400,
              display: 'flex',
              alignItems: 'center',
              gap: '4px',
            }}
          >
            <Sparkles size={12} />
            <span>Trace Correlated Only</span>
          </button>

          <span style={{ marginLeft: 'auto', fontSize: '0.75rem', color: 'var(--text-muted)' }}>
            Showing <strong>{filteredLogs.length}</strong> of {logs.length} events
          </span>
        </div>
      </div>

      {/* Logs Table & Details View */}
      <div className="panel-card" style={{ padding: 0, overflow: 'hidden' }}>
        {/* Table Header */}
        <div
          style={{
            padding: '12px 18px',
            borderBottom: '1px solid var(--border-subtle)',
            backgroundColor: 'var(--bg-card)',
            display: 'grid',
            gridTemplateColumns: '40px 150px 140px 90px 1fr 140px',
            gap: '12px',
            alignItems: 'center',
            fontSize: '0.72rem',
            fontWeight: 600,
            color: 'var(--text-muted)',
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
          }}
        >
          <span></span>
          <span>Timestamp (UTC)</span>
          <span>Service</span>
          <span>Severity</span>
          <span>Message</span>
          <span style={{ textAlign: 'right' }}>Trace / Actions</span>
        </div>

        {/* Empty state */}
        {filteredLogs.length === 0 && (
          <div style={{ padding: '40px 20px', textAlign: 'center', color: 'var(--text-muted)' }}>
            <AlertCircle size={28} style={{ margin: '0 auto 8px', opacity: 0.5 }} />
            <div style={{ fontSize: '0.9rem', color: 'var(--text-secondary)' }}>No log records matched your query</div>
            <div style={{ fontSize: '0.78rem', marginTop: '4px' }}>Try loosening your regex filter or switching severity to ALL</div>
          </div>
        )}

        {/* Logs List with Expandable Inspection */}
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          {filteredLogs.map((log, index) => {
            const isExpanded = expandedLogIdx === index;
            const isCopied = copiedIdx === index;

            return (
              <React.Fragment key={index}>
                <div
                  onClick={() => setExpandedLogIdx(isExpanded ? null : index)}
                  style={{
                    padding: '11px 18px',
                    borderBottom: '1px solid var(--border-subtle)',
                    display: 'grid',
                    gridTemplateColumns: '40px 150px 140px 90px 1fr 140px',
                    gap: '12px',
                    alignItems: 'center',
                    fontSize: '0.82rem',
                    cursor: 'pointer',
                    backgroundColor: isExpanded
                      ? 'var(--bg-card-hover)'
                      : log.severityText === 'ERROR'
                      ? 'rgba(239, 68, 68, 0.04)'
                      : log.severityText === 'WARN'
                      ? 'rgba(245, 158, 11, 0.03)'
                      : 'transparent',
                    transition: 'background-color 0.1s ease',
                  }}
                >
                  {/* Expand Chevron */}
                  <span style={{ color: 'var(--text-muted)' }}>
                    {isExpanded ? <ChevronDown size={15} /> : <ChevronRight size={15} />}
                  </span>

                  {/* Timestamp */}
                  <span style={{ color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', fontSize: '0.74rem' }}>
                    {new Date(log.timestamp).toLocaleTimeString()}.{new Date(log.timestamp).getMilliseconds().toString().padStart(3, '0')}
                  </span>

                  {/* Service */}
                  <span style={{ fontWeight: 600, color: '#fff', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {log.serviceName}
                  </span>

                  {/* Severity */}
                  <span>{getSeverityBadge(log.severityText)}</span>

                  {/* Body Message */}
                  <div
                    style={{
                      fontFamily: 'var(--font-mono)',
                      color: log.severityText === 'ERROR' ? '#fca5a5' : 'var(--text-primary)',
                      fontSize: '0.78rem',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                    title={log.body}
                  >
                    {log.body}
                  </div>

                  {/* Trace Link & Quick Action */}
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: '8px' }}>
                    {log.traceId ? (
                      <Link
                        to="/traces"
                        onClick={(e) => e.stopPropagation()}
                        style={{
                          fontSize: '0.72rem',
                          fontFamily: 'var(--font-mono)',
                          color: 'var(--accent-cyan)',
                          textDecoration: 'none',
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '3px',
                          background: 'rgba(6, 182, 212, 0.12)',
                          padding: '2px 6px',
                          borderRadius: '4px',
                        }}
                        title={`Correlated Trace: ${log.traceId}`}
                      >
                        <span>{log.traceId.slice(0, 7)}...</span>
                        <ExternalLink size={10} />
                      </Link>
                    ) : (
                      <span style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>unlinked</span>
                    )}
                  </div>
                </div>

                {/* Expanded In-depth Log Record Drawer */}
                {isExpanded && (
                  <div
                    style={{
                      padding: '18px 24px',
                      backgroundColor: 'rgba(9, 11, 16, 0.95)',
                      borderBottom: '1px solid var(--border-subtle)',
                      display: 'flex',
                      flexDirection: 'column',
                      gap: '14px',
                    }}
                  >
                    {/* Drawer Header with Copy actions */}
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--accent-emerald)' }}>
                        Structured Telemetry Record Details
                      </span>

                      <div style={{ display: 'flex', gap: '8px' }}>
                        <button
                          className="btn-secondary"
                          onClick={() => copyLogJSON(log, index)}
                          style={{ padding: '4px 10px', fontSize: '0.74rem' }}
                        >
                          {isCopied ? <Check size={12} style={{ color: 'var(--accent-emerald)' }} /> : <Copy size={12} />}
                          <span>{isCopied ? 'Copied' : 'Copy JSON'}</span>
                        </button>

                        {log.traceId && (
                          <Link
                            to="/traces"
                            className="btn-primary"
                            style={{ padding: '4px 10px', fontSize: '0.74rem', textDecoration: 'none' }}
                          >
                            <span>Inspect in Flamegraph</span>
                            <ExternalLink size={12} />
                          </Link>
                        )}
                      </div>
                    </div>

                    {/* Metadata Grid */}
                    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '10px', fontSize: '0.76rem' }}>
                      <div style={{ background: 'var(--bg-card)', padding: '8px 12px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                        <span style={{ color: 'var(--text-muted)' }}>Timestamp (ISO):</span>
                        <div style={{ color: '#fff', fontFamily: 'var(--font-mono)', marginTop: '2px' }}>{log.timestamp}</div>
                      </div>

                      <div style={{ background: 'var(--bg-card)', padding: '8px 12px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                        <span style={{ color: 'var(--text-muted)' }}>Trace ID:</span>
                        <div style={{ color: log.traceId ? 'var(--accent-cyan)' : 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginTop: '2px' }}>
                          {log.traceId || 'None'}
                        </div>
                      </div>

                      <div style={{ background: 'var(--bg-card)', padding: '8px 12px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                        <span style={{ color: 'var(--text-muted)' }}>Span ID:</span>
                        <div style={{ color: log.spanId ? '#fff' : 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginTop: '2px' }}>
                          {log.spanId || 'None'}
                        </div>
                      </div>

                      <div style={{ background: 'var(--bg-card)', padding: '8px 12px', borderRadius: '6px', border: '1px solid var(--border-subtle)' }}>
                        <span style={{ color: 'var(--text-muted)' }}>Severity Level:</span>
                        <div style={{ color: '#fff', fontFamily: 'var(--font-mono)', marginTop: '2px' }}>
                          {log.severityText} (OTLP Code: {log.severityNumber})
                        </div>
                      </div>
                    </div>

                    {/* Raw Message Box */}
                    <div style={{ background: 'var(--bg-card)', padding: '12px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
                      <div style={{ fontSize: '0.72rem', textTransform: 'uppercase', color: 'var(--text-muted)', marginBottom: '4px', fontWeight: 600 }}>
                        Full Body Text
                      </div>
                      <div style={{ fontFamily: 'var(--font-mono)', fontSize: '0.8rem', color: '#fff', lineHeight: 1.5 }}>
                        {log.body}
                      </div>
                    </div>

                    {/* Attributes Map */}
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '14px' }}>
                      <div style={{ background: 'var(--bg-card)', padding: '12px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
                        <div style={{ fontSize: '0.72rem', textTransform: 'uppercase', color: 'var(--text-muted)', marginBottom: '8px', fontWeight: 600 }}>
                          Log Attributes Map (Map(String, String))
                        </div>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontFamily: 'var(--font-mono)', fontSize: '0.74rem' }}>
                          {Object.entries(log.attributes).length > 0 ? (
                            Object.entries(log.attributes).map(([k, v]) => (
                              <div key={k} style={{ display: 'flex', gap: '8px' }}>
                                <span style={{ color: 'var(--accent-purple)' }}>{k}:</span>
                                <span style={{ color: 'var(--text-secondary)' }}>"{v}"</span>
                              </div>
                            ))
                          ) : (
                            <span style={{ color: 'var(--text-muted)' }}>No custom attributes</span>
                          )}
                        </div>
                      </div>

                      <div style={{ background: 'var(--bg-card)', padding: '12px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
                        <div style={{ fontSize: '0.72rem', textTransform: 'uppercase', color: 'var(--text-muted)', marginBottom: '8px', fontWeight: 600 }}>
                          Resource Attributes
                        </div>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontFamily: 'var(--font-mono)', fontSize: '0.74rem' }}>
                          {Object.entries(log.resourceAttributes).map(([k, v]) => (
                            <div key={k} style={{ display: 'flex', gap: '8px' }}>
                              <span style={{ color: 'var(--accent-cyan)' }}>{k}:</span>
                              <span style={{ color: 'var(--text-secondary)' }}>"{v}"</span>
                            </div>
                          ))}
                        </div>
                      </div>
                    </div>
                  </div>
                )}
              </React.Fragment>
            );
          })}
        </div>
      </div>
    </div>
  );
};
