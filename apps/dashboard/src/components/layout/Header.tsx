import React from 'react';
import {
  Search,
  Bell,
  RefreshCw,
  Clock,
  Radio,
  Boxes
} from 'lucide-react';
import { useTelemetry } from '../../context/TelemetryContext';
import type { SystemProfileType } from '../../api/telemetryClient';

export const Header: React.FC = () => {
  const {
    systemProfile,
    setSystemProfile,
    timeRange,
    setTimeRange,
    autoRefresh,
    setAutoRefresh,
    searchQuery,
    setSearchQuery,
    anomalies,
    refreshData,
  } = useTelemetry();

  const openAnomalies = anomalies.filter((a) => a.status === 'OPEN').length;

  return (
    <header className="top-header">
      {/* Search Input */}
      <div className="header-search">
        <Search size={16} />
        <input
          type="text"
          placeholder="Search trace ID, service name, or regex... (Press /)"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
        />
        <span className="code-pill" style={{ fontSize: '0.65rem' }}>⌘K</span>
      </div>

      {/* Header Actions */}
      <div className="header-actions">
        {/* System Architecture Domain Switcher */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', background: 'var(--bg-card)', padding: '3px 8px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
          <Boxes size={14} style={{ color: 'var(--accent-purple)' }} />
          <span style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>System:</span>
          <select
            value={systemProfile}
            onChange={(e) => setSystemProfile(e.target.value as SystemProfileType)}
            style={{
              background: 'transparent',
              border: 'none',
              outline: 'none',
              color: 'var(--text-primary)',
              fontSize: '0.78rem',
              fontWeight: 600,
              cursor: 'pointer',
            }}
            title="Switch microservices architecture profile"
          >
            <option value="GENERAL" style={{ background: '#10141e' }}>General Cloud Microservices</option>
            <option value="ECOMMERCE" style={{ background: '#10141e' }}>E-Commerce & Digital Retail</option>
            <option value="FINTECH" style={{ background: '#10141e' }}>FinTech & Trading Engine</option>
            <option value="HEALTHCARE" style={{ background: '#10141e' }}>Healthcare & Diagnostics</option>
          </select>
        </div>

        {/* Time Range Selector */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Clock size={15} style={{ color: 'var(--text-muted)' }} />
          <div className="time-range-selector">
            {(['5m', '15m', '1h', '24h'] as const).map((range) => (
              <button
                key={range}
                className={`time-range-btn ${timeRange === range ? 'active' : ''}`}
                onClick={() => setTimeRange(range)}
              >
                {range}
              </button>
            ))}
          </div>
        </div>

        {/* Live Auto-refresh Indicator */}
        <button
          className={`btn-secondary ${autoRefresh ? 'active' : ''}`}
          style={{
            padding: '5px 10px',
            fontSize: '0.78rem',
            color: autoRefresh ? 'var(--accent-emerald)' : 'var(--text-muted)',
            borderColor: autoRefresh ? 'rgba(16, 185, 129, 0.4)' : 'var(--border-subtle)',
          }}
          onClick={() => setAutoRefresh(!autoRefresh)}
          title="Toggle live telemetry polling"
        >
          <Radio size={14} className={autoRefresh ? 'pulse' : ''} />
          <span>{autoRefresh ? 'Live (5s)' : 'Paused'}</span>
        </button>

        {/* Manual Refresh Button */}
        <button className="icon-btn" onClick={refreshData} title="Refresh Telemetry">
          <RefreshCw size={16} />
        </button>

        {/* Notifications */}
        <button className="icon-btn" title="Open Anomalies">
          <Bell size={16} />
          {openAnomalies > 0 && <span className="icon-badge">{openAnomalies}</span>}
        </button>
      </div>
    </header>
  );
};
