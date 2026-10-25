import React from 'react';
import { ExternalLink, CheckCircle2, AlertTriangle, XCircle } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useTelemetry } from '../../context/TelemetryContext';
import type { REDMetric } from '../../types/telemetry';

export const ServiceHealthTable: React.FC = () => {
  const { redMetrics } = useTelemetry();

  const getStatusBadge = (status: REDMetric['status']) => {
    switch (status) {
      case 'HEALTHY':
        return (
          <span className="badge healthy">
            <CheckCircle2 size={11} /> HEALTHY
          </span>
        );
      case 'DEGRADED':
        return (
          <span className="badge degraded">
            <AlertTriangle size={11} /> DEGRADED
          </span>
        );
      case 'CRITICAL':
        return (
          <span className="badge critical">
            <XCircle size={11} /> CRITICAL
          </span>
        );
    }
  };

  return (
    <div className="panel-card">
      <div className="panel-title">
        <div>
          <span>Service RED Metrics</span>
          <div style={{ fontSize: '0.75rem', fontWeight: 400, color: 'var(--text-muted)', marginTop: '2px' }}>
            Rollup from ClickHouse table <code>observability.latency_1m</code>
          </div>
        </div>
        <Link to="/traces" style={{ fontSize: '0.8rem', color: 'var(--accent-emerald)', textDecoration: 'none', display: 'flex', alignItems: 'center', gap: '4px' }}>
          Explore all traces <ExternalLink size={12} />
        </Link>
      </div>

      <div className="table-container">
        <table className="telemetry-table">
          <thead>
            <tr>
              <th>Service</th>
              <th>Operation</th>
              <th>Throughput</th>
              <th>Error Rate</th>
              <th>p50</th>
              <th>p95</th>
              <th>p99</th>
              <th>Status</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {redMetrics.map((m) => (
              <tr key={m.serviceName}>
                <td>
                  <strong style={{ color: 'var(--text-primary)' }}>{m.serviceName}</strong>
                </td>
                <td>
                  <span className="code-pill">{m.operation}</span>
                </td>
                <td style={{ fontFamily: 'var(--font-mono)' }}>
                  {m.callsPerMin.toLocaleString()} /min
                </td>
                <td>
                  <span
                    style={{
                      color: m.errorRate > 1 ? 'var(--status-critical)' : 'inherit',
                      fontWeight: m.errorRate > 1 ? 600 : 400,
                      fontFamily: 'var(--font-mono)',
                    }}
                  >
                    {m.errorRate}% ({m.errorCount})
                  </span>
                </td>
                <td style={{ fontFamily: 'var(--font-mono)' }}>{m.p50Ms}ms</td>
                <td style={{ fontFamily: 'var(--font-mono)' }}>{m.p95Ms}ms</td>
                <td
                  style={{
                    fontFamily: 'var(--font-mono)',
                    fontWeight: m.p99Ms > 200 ? 700 : 400,
                    color: m.p99Ms > 200 ? 'var(--accent-amber)' : 'inherit',
                  }}
                >
                  {m.p99Ms}ms
                </td>
                <td>{getStatusBadge(m.status)}</td>
                <td>
                  <Link
                    to={`/traces?service=${m.serviceName}`}
                    className="btn-secondary"
                    style={{ padding: '3px 8px', fontSize: '0.72rem' }}
                  >
                    Traces
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
};
