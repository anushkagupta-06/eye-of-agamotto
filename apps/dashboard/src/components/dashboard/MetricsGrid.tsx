import React from 'react';
import { Activity, Clock, AlertOctagon, Layers, ArrowUpRight, ArrowDownRight } from 'lucide-react';
import { useTelemetry } from '../../context/TelemetryContext';

export const MetricsGrid: React.FC = () => {
  const { stats } = useTelemetry();

  return (
    <div className="metrics-grid">
      {/* Total Spans Throughput */}
      <div className="metric-card">
        <div className="metric-card-header">
          <span className="metric-label">Spans Throughput</span>
          <div className="metric-icon-box" style={{ color: 'var(--accent-emerald)' }}>
            <Activity size={18} />
          </div>
        </div>
        <div className="metric-value">
          {stats.totalSpansPerSec.toLocaleString()} <span style={{ fontSize: '1rem', color: 'var(--text-muted)' }}>/s</span>
        </div>
        <div className="metric-trend">
          <span className="trend-pill positive">
            <ArrowUpRight size={12} /> +12.4%
          </span>
          <span>vs previous window</span>
        </div>
      </div>

      {/* Log Ingestion Rate */}
      <div className="metric-card">
        <div className="metric-card-header">
          <span className="metric-label">Logs Ingested</span>
          <div className="metric-icon-box" style={{ color: 'var(--accent-cyan)' }}>
            <Layers size={18} />
          </div>
        </div>
        <div className="metric-value">
          {stats.totalLogsPerSec.toLocaleString()} <span style={{ fontSize: '1rem', color: 'var(--text-muted)' }}>/s</span>
        </div>
        <div className="metric-trend">
          <span className="trend-pill positive">
            <ArrowUpRight size={12} /> +8.1%
          </span>
          <span>NATS JetStream</span>
        </div>
      </div>

      {/* Average P99 Latency */}
      <div className="metric-card">
        <div className="metric-card-header">
          <span className="metric-label">Avg P99 Latency</span>
          <div className="metric-icon-box" style={{ color: stats.avgP99Ms > 150 ? 'var(--accent-amber)' : 'var(--accent-emerald)' }}>
            <Clock size={18} />
          </div>
        </div>
        <div className="metric-value">
          {stats.avgP99Ms} <span style={{ fontSize: '1rem', color: 'var(--text-muted)' }}>ms</span>
        </div>
        <div className="metric-trend">
          <span className="trend-pill negative">
            <ArrowUpRight size={12} /> +24ms
          </span>
          <span>checkout & payment spike</span>
        </div>
      </div>

      {/* Global Error Rate */}
      <div className="metric-card">
        <div className="metric-card-header">
          <span className="metric-label">Global Error Rate</span>
          <div className="metric-icon-box" style={{ color: stats.errorRate > 1 ? 'var(--status-critical)' : 'var(--status-healthy)' }}>
            <AlertOctagon size={18} />
          </div>
        </div>
        <div className="metric-value" style={{ color: stats.errorRate > 1 ? '#f87171' : 'white' }}>
          {stats.errorRate}%
        </div>
        <div className="metric-trend">
          <span className="trend-pill positive">
            <ArrowDownRight size={12} /> -0.2%
          </span>
          <span>tolerable threshold &lt; 2.0%</span>
        </div>
      </div>
    </div>
  );
};
