import React from 'react';
import {
  Search,
  AlertCircle,
  RefreshCw
} from 'lucide-react';
import { useTelemetry } from '../../context/TelemetryContext';

export const Header: React.FC = () => {
  const {
    tenant,
    timeRange,
    setTimeRange,
    searchQuery,
    setSearchQuery,
    refreshData,
    isLoading,
    isStale,
    error,
    mockMode,
  } = useTelemetry();

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
      <div className="header-actions" style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
        {/* Time Range Selector */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '4px', background: 'var(--bg-card)', padding: '4px 8px', borderRadius: '8px', border: '1px solid var(--border-subtle)' }}>
          <span style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>Range:</span>
          <select
            value={timeRange}
            onChange={(e) => setTimeRange(e.target.value)}
            style={{
              background: 'transparent',
              border: 'none',
              outline: 'none',
              color: 'var(--text-primary)',
              fontSize: '0.78rem',
              fontWeight: 600,
              cursor: 'pointer',
            }}
          >
            <option value="15m" style={{ background: '#10141e' }}>Last 15 minutes</option>
            <option value="1h" style={{ background: '#10141e' }}>Last 1 hour</option>
            <option value="24h" style={{ background: '#10141e' }}>Last 24 hours</option>
            <option value="7d" style={{ background: '#10141e' }}>Last 7 days</option>
          </select>
        </div>

        {/* Live / Mock Mode Status Pill */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            background: mockMode ? 'rgba(245, 158, 11, 0.1)' : 'rgba(16, 185, 129, 0.1)',
            padding: '5px 10px',
            borderRadius: '8px',
            border: `1px solid ${mockMode ? 'var(--accent-amber)' : 'var(--accent-emerald)'}`,
          }}
        >
          <span
            style={{
              width: '7px',
              height: '7px',
              borderRadius: '50%',
              backgroundColor: mockMode ? 'var(--accent-amber)' : 'var(--accent-emerald)',
              boxShadow: `0 0 6px ${mockMode ? 'var(--accent-amber)' : 'var(--accent-emerald)'}`,
            }}
          />
          <span style={{ fontSize: '0.75rem', fontWeight: 600, color: mockMode ? 'var(--accent-amber)' : 'var(--accent-emerald)' }}>
            {mockMode ? 'Mock Mode' : `Live API (${tenant})`}
          </span>
        </div>

        {/* Error / Stale Notice if any */}
        {error && !mockMode && (
          <div
            title={error}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '4px',
              background: 'rgba(239, 68, 68, 0.1)',
              padding: '4px 8px',
              borderRadius: '6px',
              border: '1px solid var(--accent-red)',
              fontSize: '0.72rem',
              color: 'var(--accent-red)',
            }}
          >
            <AlertCircle size={13} />
            <span>{isStale ? 'Stale Data' : 'API Offline'}</span>
          </div>
        )}

        {/* Manual Refresh Button */}
        <button
          onClick={refreshData}
          disabled={isLoading}
          className="btn-secondary"
          title="Refresh telemetry"
          style={{ padding: '6px 10px', fontSize: '0.74rem', display: 'flex', alignItems: 'center', gap: '5px' }}
        >
          <RefreshCw size={13} className={isLoading ? 'animate-spin' : ''} />
          <span>Refresh</span>
        </button>
      </div>
    </header>
  );
};
