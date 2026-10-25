import React, { useState } from 'react';
import { MetricsGrid } from '../components/dashboard/MetricsGrid';
import { PipelineStatusCard } from '../components/dashboard/PipelineStatusCard';
import { AnomaliesAlertBanner } from '../components/dashboard/AnomaliesAlertBanner';
import { ServiceHealthTable } from '../components/dashboard/ServiceHealthTable';
import { ThroughputChart } from '../components/dashboard/ThroughputChart';
import { RecentActivityFeed } from '../components/dashboard/RecentActivityFeed';
import { Terminal, ShieldCheck, Plus, Check, Layers } from 'lucide-react';
import { useTelemetry } from '../context/TelemetryContext';

export const DashboardOverview: React.FC = () => {
  const { tenant, systemProfileName, systemProfileDescription, addCustomServiceTelemetry } = useTelemetry();
  const [showAddModal, setShowAddModal] = useState(false);
  const [customServiceName, setCustomServiceName] = useState('');
  const [justAdded, setJustAdded] = useState(false);

  const handleAddService = (e: React.FormEvent) => {
    e.preventDefault();
    if (!customServiceName.trim()) return;
    addCustomServiceTelemetry(customServiceName);
    setJustAdded(true);
    setCustomServiceName('');
    setTimeout(() => {
      setJustAdded(false);
      setShowAddModal(false);
    }, 1200);
  };

  return (
    <div>
      {/* Top Banner / Title */}
      <div className="page-header">
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
            <h1 className="page-title">Distributed Telemetry Overview</h1>
            <span className="badge healthy" style={{ fontSize: '0.72rem' }}>
              <Layers size={11} /> {systemProfileName}
            </span>
          </div>
          <p className="page-desc">
            {systemProfileDescription} Tenant: <strong style={{ color: 'var(--accent-emerald)' }}>{tenant}</strong>.
          </p>
        </div>

        <div style={{ display: 'flex', gap: '10px' }}>
          <button
            className="btn-secondary"
            onClick={() => setShowAddModal(true)}
            style={{ borderColor: 'rgba(16, 185, 129, 0.4)', color: 'var(--accent-emerald)' }}
          >
            <Plus size={14} />
            <span>Trace Any Microservice</span>
          </button>
          <button
            className="btn-secondary"
            onClick={() => {
              window.open('/settings', '_self');
            }}
          >
            <Terminal size={14} />
            <span>Ingest Smoke Test</span>
          </button>
          <button
            className="btn-primary"
            onClick={() => {
              window.open('/assistant', '_self');
            }}
          >
            <ShieldCheck size={14} />
            <span>AI Telemetry Inspector</span>
          </button>
        </div>
      </div>

      {/* Add Custom Microservice Modal */}
      {showAddModal && (
        <div
          style={{
            marginBottom: '20px',
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--accent-emerald)',
            borderRadius: '12px',
            padding: '18px 22px',
            boxShadow: '0 0 25px rgba(16, 185, 129, 0.15)',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '10px' }}>
            <span style={{ fontWeight: 600, fontSize: '0.9rem', color: '#fff' }}>
              Register & Trace Any Custom Microservice
            </span>
            <button
              onClick={() => setShowAddModal(false)}
              style={{ background: 'transparent', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', fontSize: '0.82rem' }}
            >
              Cancel
            </button>
          </div>
          <p style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', marginBottom: '12px' }}>
            The Eye of Agamotto is a universal OpenTelemetry engine. Type any microservice name below to immediately register and observe it:
          </p>
          <form onSubmit={handleAddService} style={{ display: 'flex', gap: '10px' }}>
            <input
              type="text"
              placeholder="e.g. fraud-detector, patient-service, recommendation-engine..."
              value={customServiceName}
              onChange={(e) => setCustomServiceName(e.target.value)}
              style={{
                flex: 1,
                background: 'var(--bg-card)',
                border: '1px solid var(--border-subtle)',
                borderRadius: '8px',
                padding: '9px 14px',
                color: '#fff',
                fontSize: '0.85rem',
                outline: 'none',
              }}
            />
            <button type="submit" className="btn-primary">
              {justAdded ? <Check size={15} /> : <Plus size={15} />}
              <span>{justAdded ? 'Registered!' : 'Register Service'}</span>
            </button>
          </form>
        </div>
      )}

      {/* Critical Anomaly Alert */}
      <AnomaliesAlertBanner />

      {/* High-level RED Metrics */}
      <MetricsGrid />

      {/* Pipeline Status matching Member 1 & 2's components */}
      <PipelineStatusCard />

      {/* Middle Section: RED Matrix & Volume Trend */}
      <div className="dash-two-col">
        <ServiceHealthTable />
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          <ThroughputChart />
          <RecentActivityFeed />
        </div>
      </div>
    </div>
  );
};
