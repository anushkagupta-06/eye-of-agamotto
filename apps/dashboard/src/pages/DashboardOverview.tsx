import React, { useState } from 'react';
import { MetricsGrid } from '../components/dashboard/MetricsGrid';
import { PipelineStatusCard } from '../components/dashboard/PipelineStatusCard';
import { AnomaliesAlertBanner } from '../components/dashboard/AnomaliesAlertBanner';
import { ServiceHealthTable } from '../components/dashboard/ServiceHealthTable';
import { ThroughputChart } from '../components/dashboard/ThroughputChart';
import { RecentActivityFeed } from '../components/dashboard/RecentActivityFeed';
import { Terminal, Plus, Check, Layers, CheckCircle2, ArrowDown } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useTelemetry } from '../context/TelemetryContext';

export const DashboardOverview: React.FC = () => {
  const { systemProfileName, emitRealServiceTrace } = useTelemetry();
  const [showAddModal, setShowAddModal] = useState(false);
  const [customServiceName, setCustomServiceName] = useState('');
  const [justAdded, setJustAdded] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [emitError, setEmitError] = useState<string | null>(null);
  const [lastAddedServiceName, setLastAddedServiceName] = useState<string | null>(null);

  const handleAddService = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!customServiceName.trim() || isSubmitting) return;
    const name = customServiceName.trim();
    setIsSubmitting(true);
    setEmitError(null);
    try {
      await emitRealServiceTrace(name);
      setJustAdded(true);
      setLastAddedServiceName(name);
      setCustomServiceName('');
      setTimeout(() => {
        setJustAdded(false);
        setShowAddModal(false);
      }, 1500);
    } catch (err: any) {
      console.error('Failed to emit live trace:', err);
      setEmitError(err?.message || 'Failed to emit trace to gateway');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div>
      {/* Top Banner / Title */}
      <div className="page-header">
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <h1 className="page-title">Distributed Telemetry Overview</h1>
            <span className="badge healthy" style={{ fontSize: '0.72rem' }}>
              <Layers size={11} /> {systemProfileName}
            </span>
          </div>
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
              Emit Live OTLP Trace (End-to-End Pipeline)
            </span>
            <button
              onClick={() => setShowAddModal(false)}
              style={{ background: 'transparent', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', fontSize: '0.82rem' }}
            >
              Cancel
            </button>
          </div>
          <p style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', marginBottom: '12px' }}>
            Publishes a real OTLP trace payload directly to the Ingestion Gateway (<code>POST /v1/traces</code>) via NATS JetStream and ClickHouse:
          </p>
          <form onSubmit={handleAddService} style={{ display: 'flex', gap: '10px' }}>
            <input
              type="text"
              placeholder="e.g. payment-service, order-processor, auth-engine..."
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
            <button type="submit" className="btn-primary" disabled={isSubmitting}>
              {justAdded ? <Check size={15} /> : <Plus size={15} />}
              <span>{justAdded ? 'Trace Ingested!' : isSubmitting ? 'Ingesting...' : 'Emit Real Trace'}</span>
            </button>
          </form>
          {emitError && (
            <div style={{ marginTop: '10px', fontSize: '0.78rem', color: '#f87171' }}>
              ⚠️ {emitError}
            </div>
          )}
        </div>
      )}

      {/* Registration Confirmation Banner */}
      {lastAddedServiceName && (
        <div
          style={{
            padding: '12px 18px',
            backgroundColor: 'rgba(16, 185, 129, 0.12)',
            border: '1px solid var(--accent-emerald)',
            borderRadius: '10px',
            marginBottom: '20px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            boxShadow: '0 0 15px rgba(16, 185, 129, 0.15)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <CheckCircle2 size={18} style={{ color: 'var(--accent-emerald)', flexShrink: 0 }} />
            <span style={{ fontSize: '0.85rem', color: '#fff' }}>
              Service <strong style={{ color: 'var(--accent-emerald)' }}>"{lastAddedServiceName}"</strong> is now actively emitting OpenTelemetry data!
            </span>
          </div>
          <div style={{ display: 'flex', gap: '8px' }}>
            <button
              onClick={() => document.getElementById('service-red-table')?.scrollIntoView({ behavior: 'smooth' })}
              className="btn-secondary"
              style={{ padding: '4px 10px', fontSize: '0.74rem' }}
            >
              <ArrowDown size={12} />
              <span>Scroll to Table</span>
            </button>
            <Link
              to="/logs"
              className="btn-secondary"
              style={{ padding: '4px 10px', fontSize: '0.74rem', textDecoration: 'none' }}
            >
              <span>View Logs ↗</span>
            </Link>
          </div>
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
