import React from 'react';
import { Activity, ArrowRight, Server, ShieldCheck, Database, Network } from 'lucide-react';
import { useTelemetry } from '../context/TelemetryContext';

export const TopologyPage: React.FC = () => {
  const { systemProfile, systemProfileName, redMetrics } = useTelemetry();

  // Derive dynamic edges based on system profile
  const getDynamicEdges = () => {
    switch (systemProfile) {
      case 'GENERAL':
        return [
          { caller: 'api-gateway', callee: 'auth-service', calls: 42100, errors: 84, p95LatencyMs: 18 },
          { caller: 'api-gateway', callee: 'user-service', calls: 28500, errors: 57, p95LatencyMs: 28 },
          { caller: 'api-gateway', callee: 'notification-worker', calls: 14200, errors: 497, p95LatencyMs: 320 },
          { caller: 'api-gateway', callee: 'billing-service', calls: 8900, errors: 115, p95LatencyMs: 118 },
        ];
      case 'FINTECH':
        return [
          { caller: 'order-router', callee: 'risk-engine', calls: 98000, errors: 98, p95LatencyMs: 8 },
          { caller: 'order-router', callee: 'matching-engine', calls: 85000, errors: 2550, p95LatencyMs: 180 },
          { caller: 'matching-engine', callee: 'ledger-service', calls: 45000, errors: 450, p95LatencyMs: 65 },
          { caller: 'market-data-feed', callee: 'order-router', calls: 340000, errors: 34, p95LatencyMs: 2 },
        ];
      case 'HEALTHCARE':
        return [
          { caller: 'telehealth-gateway', callee: 'patient-records-api', calls: 29000, errors: 58, p95LatencyMs: 55 },
          { caller: 'telehealth-gateway', callee: 'lab-diagnostic-engine', calls: 6200, errors: 248, p95LatencyMs: 410 },
          { caller: 'icu-vital-monitor', callee: 'patient-records-api', calls: 140000, errors: 14, p95LatencyMs: 6 },
        ];
      case 'ECOMMERCE':
      default:
        return [
          { caller: 'ingress-gateway', callee: 'checkout-service', calls: 14200, errors: 142, p95LatencyMs: 110 },
          { caller: 'checkout-service', callee: 'payment-gateway', calls: 9850, errors: 394, p95LatencyMs: 310 },
          { caller: 'checkout-service', callee: 'inventory-service', calls: 14200, errors: 12, p95LatencyMs: 22 },
          { caller: 'ingress-gateway', callee: 'auth-service', calls: 21000, errors: 42, p95LatencyMs: 34 },
        ];
    }
  };

  const edges = getDynamicEdges();

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Service Topology Map</h1>
          <p className="page-desc">
            Auto-generated dependency graph depicting service-to-service communication latency and error rates derived from <code>observability.service_edges_1m</code> for <strong>{systemProfileName}</strong>.
          </p>
        </div>
      </div>

      {/* Interactive Topology Visualizer */}
      <div className="panel-card" style={{ marginBottom: '24px' }}>
        <div className="panel-title">
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Network size={18} style={{ color: 'var(--accent-emerald)' }} />
            <span>Distributed Dependency Graph ({systemProfileName})</span>
          </div>
          <span className="badge healthy">Auto-Discovered from Spans</span>
        </div>

        {/* Visual Map Canvas Representation */}
        <div
          style={{
            minHeight: '360px',
            background: 'radial-gradient(circle at 50% 50%, rgba(16, 185, 129, 0.05) 0%, rgba(9, 11, 16, 1) 100%)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '10px',
            position: 'relative',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-around',
            padding: '24px',
            flexWrap: 'wrap',
            gap: '16px',
          }}
        >
          {/* Level 1: Ingress/Gateway */}
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
            <div
              style={{
                width: '150px',
                padding: '14px',
                background: 'var(--bg-card)',
                border: '1px solid var(--accent-emerald)',
                borderRadius: '10px',
                textAlign: 'center',
                boxShadow: '0 0 15px rgba(16, 185, 129, 0.15)',
              }}
            >
              <Server size={22} style={{ color: 'var(--accent-emerald)', marginBottom: '4px' }} />
              <div style={{ fontWeight: 600, fontSize: '0.82rem', color: '#fff' }}>
                {redMetrics[0]?.serviceName || 'gateway-service'}
              </div>
              <span className="badge healthy" style={{ fontSize: '0.65rem', marginTop: '6px' }}>
                {redMetrics[0]?.p50Ms || 4}ms p50
              </span>
            </div>
          </div>

          <ArrowRight size={24} style={{ color: 'var(--text-muted)' }} />

          {/* Level 2: Core Processing Layer */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
            {redMetrics.slice(1, 3).map((m) => (
              <div
                key={m.serviceName}
                style={{
                  width: '160px',
                  padding: '14px',
                  background: 'var(--bg-card)',
                  border: m.status === 'CRITICAL' ? '1px solid var(--status-critical)' : '1px solid var(--border-medium)',
                  borderRadius: '10px',
                  textAlign: 'center',
                }}
              >
                <ShieldCheck size={20} style={{ color: 'var(--accent-cyan)', marginBottom: '4px' }} />
                <div style={{ fontWeight: 600, fontSize: '0.82rem', color: '#fff' }}>{m.serviceName}</div>
                <span className={`badge ${m.status === 'CRITICAL' ? 'critical' : 'healthy'}`} style={{ fontSize: '0.65rem', marginTop: '6px' }}>
                  {m.p95Ms}ms p95
                </span>
              </div>
            ))}
          </div>

          <ArrowRight size={24} style={{ color: 'var(--text-muted)' }} />

          {/* Level 3: Downstream & Integration Layer */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
            {redMetrics.slice(3, 5).map((m) => {
              const isError = m.status === 'CRITICAL' || m.errorRate > 1;
              return (
                <div
                  key={m.serviceName}
                  style={{
                    width: '160px',
                    padding: '14px',
                    background: 'var(--bg-card)',
                    border: isError ? '1px solid var(--status-critical)' : '1px solid var(--border-medium)',
                    borderRadius: '10px',
                    textAlign: 'center',
                    boxShadow: isError ? '0 0 15px rgba(239, 68, 68, 0.2)' : 'none',
                  }}
                >
                  {isError ? (
                    <Activity size={20} style={{ color: 'var(--status-critical)', marginBottom: '4px' }} />
                  ) : (
                    <Database size={20} style={{ color: 'var(--accent-emerald)', marginBottom: '4px' }} />
                  )}
                  <div style={{ fontWeight: 600, fontSize: '0.82rem', color: isError ? '#f87171' : '#fff' }}>
                    {m.serviceName}
                  </div>
                  <span className={`badge ${isError ? 'critical' : 'healthy'}`} style={{ fontSize: '0.65rem', marginTop: '6px' }}>
                    {m.p95Ms}ms ({m.errorRate}% err)
                  </span>
                </div>
              );
            })}
          </div>
        </div>
      </div>

      {/* Service Edges Table */}
      <div className="panel-card">
        <div className="panel-title">
          <span>Service Dependency Edges</span>
          <span style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>Aggregate View ({edges.length} edges)</span>
        </div>

        <div className="table-container">
          <table className="telemetry-table">
            <thead>
              <tr>
                <th>Caller</th>
                <th>Callee</th>
                <th>Calls/min</th>
                <th>Errors</th>
                <th>P95 Latency</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {edges.map((e, idx) => (
                <tr key={idx}>
                  <td style={{ fontWeight: 600, color: '#fff' }}>{e.caller}</td>
                  <td style={{ fontWeight: 600, color: '#fff' }}>{e.callee}</td>
                  <td style={{ fontFamily: 'var(--font-mono)' }}>{e.calls.toLocaleString()}</td>
                  <td style={{ fontFamily: 'var(--font-mono)', color: e.errors > 100 ? '#f87171' : 'inherit' }}>
                    {e.errors}
                  </td>
                  <td style={{ fontFamily: 'var(--font-mono)' }}>{e.p95LatencyMs}ms</td>
                  <td>
                    {e.errors > 100 ? (
                      <span className="badge critical">CRITICAL EDGE</span>
                    ) : (
                      <span className="badge healthy">NORMAL</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
