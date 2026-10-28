import React from 'react';
import { GitCommit, AlertCircle, ArrowRight } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useTelemetry } from '../../context/TelemetryContext';

export const RecentActivityFeed: React.FC = () => {
  const { spans, logs } = useTelemetry();

  return (
    <div className="panel-card" style={{ width: '100%', minWidth: 0 }}>
      <div className="panel-title">
        <span>Recent Ingested Events</span>
        <span className="badge neutral">Real-time</span>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
        {/* Recent Errors from Spans */}
        {spans
          .filter((s) => s.statusCode === 'ERROR')
          .slice(0, 2)
          .map((span) => (
            <div
              key={span.spanId}
              style={{
                backgroundColor: 'var(--bg-card)',
                border: '1px solid var(--border-subtle)',
                borderRadius: '8px',
                padding: '10px 12px',
                fontSize: '0.8rem',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '4px' }}>
                <span style={{ display: 'flex', alignItems: 'center', gap: '6px', color: '#f87171', fontWeight: 600 }}>
                  <AlertCircle size={14} /> Span Failure ({span.httpStatusCode || 'ERR'})
                </span>
                <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>
                  {Math.round(span.durationNs / 1000000)}ms
                </span>
              </div>
              <div style={{ color: 'var(--text-secondary)', marginBottom: '4px' }}>
                <strong>{span.serviceName}</strong>: {span.name}
              </div>
              <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                trace: {span.traceId.slice(0, 16)}...
              </div>
            </div>
          ))}

        {/* Recent logs */}
        {logs.slice(0, 2).map((log, idx) => (
          <div
            key={idx}
            style={{
              backgroundColor: 'var(--bg-card)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '8px',
              padding: '10px 12px',
              fontSize: '0.8rem',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '4px' }}>
              <span
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  color: log.severityText === 'ERROR' ? '#f87171' : 'var(--accent-emerald)',
                  fontWeight: 600,
                }}
              >
                <GitCommit size={14} /> {log.serviceName}
              </span>
              <span className={`badge ${log.severityText === 'ERROR' ? 'critical' : 'info'}`} style={{ fontSize: '0.65rem' }}>
                {log.severityText}
              </span>
            </div>
            <div
              style={{
                color: 'var(--text-secondary)',
                fontFamily: 'var(--font-mono)',
                fontSize: '0.74rem',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {log.body}
            </div>
          </div>
        ))}
      </div>

      <div style={{ marginTop: '16px', textAlign: 'center' }}>
        <Link
          to="/logs"
          style={{
            fontSize: '0.78rem',
            color: 'var(--accent-cyan)',
            textDecoration: 'none',
            display: 'inline-flex',
            alignItems: 'center',
            gap: '4px',
          }}
        >
          Open Live Log Explorer <ArrowRight size={12} />
        </Link>
      </div>
    </div>
  );
};
