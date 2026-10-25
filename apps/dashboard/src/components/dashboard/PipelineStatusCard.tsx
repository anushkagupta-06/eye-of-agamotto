import React from 'react';
import { Server, Database, Cpu, Zap, CheckCircle2 } from 'lucide-react';
import { useTelemetry } from '../../context/TelemetryContext';

export const PipelineStatusCard: React.FC = () => {
  const { pipelineStatus } = useTelemetry();

  const getIcon = (name: string) => {
    if (name.includes('Gateway')) return <Server size={18} style={{ color: 'var(--accent-emerald)' }} />;
    if (name.includes('NATS')) return <Zap size={18} style={{ color: 'var(--accent-cyan)' }} />;
    if (name.includes('ClickHouse')) return <Database size={18} style={{ color: 'var(--accent-purple)' }} />;
    return <Cpu size={18} style={{ color: 'var(--accent-amber)' }} />;
  };

  return (
    <div style={{ marginBottom: '22px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '10px' }}>
        <span style={{ fontSize: '0.8rem', fontWeight: 600, textTransform: 'uppercase', color: 'var(--text-muted)', letterSpacing: '0.05em' }}>
          Data Ingestion & Storage Pipeline (NATS JetStream + ClickHouse)
        </span>
        <span className="badge healthy" style={{ fontSize: '0.7rem' }}>
          <CheckCircle2 size={12} /> Pipeline Synchronized
        </span>
      </div>

      <div className="pipeline-grid">
        {pipelineStatus.map((comp) => (
          <div key={comp.name} className="pipeline-card">
            <div style={{ padding: '6px', borderRadius: '6px', background: 'var(--bg-card)' }}>
              {getIcon(comp.name)}
            </div>
            <div className="pipeline-card-info">
              <div className="pipeline-name">
                <span>{comp.name}</span>
                <span className="badge healthy">{comp.latency}</span>
              </div>
              <div className="pipeline-details">{comp.details}</div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
