import React from 'react';
import { BarChart3 } from 'lucide-react';

export const ThroughputChart: React.FC = () => {
  const bars = [
    { label: '10:00', height: 45, calls: '38k' },
    { label: '10:02', height: 58, calls: '49k' },
    { label: '10:04', height: 78, calls: '66k' },
    { label: '10:06', height: 95, calls: '81k' },
    { label: '10:08', height: 88, calls: '75k' },
    { label: '10:10', height: 68, calls: '58k' },
    { label: '10:12', height: 74, calls: '63k' },
  ];

  return (
    <div className="panel-card" style={{ width: '100%', minWidth: 0 }}>
      <div className="panel-title">
        <div>
          <span>Telemetry Stream Volume</span>
          <div style={{ fontSize: '0.74rem', fontWeight: 400, color: 'var(--text-muted)', marginTop: '2px' }}>
            NATS JetStream (telemetry.traces & telemetry.logs)
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <BarChart3 size={15} style={{ color: 'var(--accent-emerald)' }} />
          <span style={{ fontSize: '0.74rem', color: 'var(--text-secondary)' }}>Live buckets</span>
        </div>
      </div>

      <div className="chart-bar-container">
        {bars.map((bar, idx) => (
          <div key={idx} className="chart-bar-col">
            <div
              className="chart-bar"
              style={{ height: `${bar.height}%` }}
              title={`${bar.label}: ${bar.calls} items ingested`}
            />
            <span className="chart-label">{bar.label}</span>
          </div>
        ))}
      </div>

      <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '12px', fontSize: '0.74rem', color: 'var(--text-muted)' }}>
        <span>Peak: 81k req/min</span>
        <span>Storage tier: ClickHouse ZSTD(1)</span>
      </div>
    </div>
  );
};
