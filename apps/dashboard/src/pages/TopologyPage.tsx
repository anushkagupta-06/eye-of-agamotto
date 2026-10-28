import React, { useState, useEffect } from 'react';
import { Activity, ArrowRight, Server, ShieldCheck, Database, Network } from 'lucide-react';
import { useTelemetry } from '../context/TelemetryContext';
import { fetchLiveTopology } from '../api/liveTelemetryClient';

export interface TopologyEdge {
  caller: string;
  callee: string;
  calls: number;
  errors: number;
  p95LatencyMs: number;
}

export const TopologyPage: React.FC = () => {
  const { systemProfileName, redMetrics, spans } = useTelemetry();
  const [liveEdges, setLiveEdges] = useState<TopologyEdge[]>([]);

  useEffect(() => {
    let mounted = true;
    fetchLiveTopology().then((edges) => {
      if (mounted) setLiveEdges(edges);
    });
    const interval = setInterval(() => {
      fetchLiveTopology().then((edges) => {
        if (mounted) setLiveEdges(edges);
      });
    }, 5000);
    return () => {
      mounted = false;
      clearInterval(interval);
    };
  }, []);

  // Derive dynamic edges from live backend edges or active spans
  const getDynamicEdges = (): TopologyEdge[] => {
    if (liveEdges.length > 0) {
      return liveEdges;
    }
    const derivedEdges: TopologyEdge[] = [];
    const spanMap = new Map<string, string>(); // spanId -> serviceName

    spans.forEach((s) => {
      spanMap.set(s.spanId, s.serviceName);
    });

    // Check parent-child span pairs
    spans.forEach((s) => {
      if (s.parentSpanId && spanMap.has(s.parentSpanId)) {
        const caller = spanMap.get(s.parentSpanId)!;
        const callee = s.serviceName;
        if (caller !== callee) {
          const calleeMetric = redMetrics.find((m) => m.serviceName === callee);
          const existing = derivedEdges.find((e) => e.caller === caller && e.callee === callee);
          if (!existing) {
            derivedEdges.push({
              caller,
              callee,
              calls: calleeMetric?.callsPerMin || 1,
              errors: calleeMetric?.errorCount || (s.statusCode === 'ERROR' ? 1 : 0),
              p95LatencyMs: calleeMetric?.p95Ms || Math.round(s.durationNs / 1000000),
            });
          }
        }
      }
    });

    return derivedEdges;
  };

  const edges = getDynamicEdges();

  // Dynamically group all active services into 3 architectural tiers
  const isGateway = (name: string) => /gateway|ingress|router|proxy|feed/i.test(name);
  const isDownstream = (name: string) => /db|database|storage|ledger|worker|relay|notification|records/i.test(name);

  const ingressServices = redMetrics.filter((m) => isGateway(m.serviceName));
  const fallbackIngress = ingressServices.length > 0 ? ingressServices : [redMetrics[0]].filter(Boolean);

  const downstreamServices = redMetrics.filter(
    (m) => isDownstream(m.serviceName) && !fallbackIngress.some((g) => g.serviceName === m.serviceName)
  );

  const coreServices = redMetrics.filter(
    (m) =>
      !fallbackIngress.some((g) => g.serviceName === m.serviceName) &&
      !downstreamServices.some((d) => d.serviceName === m.serviceName)
  );

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
          <span className="badge healthy">Auto-Discovered from Spans ({redMetrics.length} Services)</span>
        </div>

        {/* Visual Map Canvas Representation */}
        <div
          style={{
            minHeight: '380px',
            background: 'radial-gradient(circle at 50% 50%, rgba(16, 185, 129, 0.05) 0%, rgba(9, 11, 16, 1) 100%)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '10px',
            position: 'relative',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-around',
            padding: '24px',
            flexWrap: 'wrap',
            gap: '24px',
          }}
        >
          {/* Level 1: Ingress / Gateway Layer */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px', alignItems: 'center' }}>
            <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em', fontWeight: 600 }}>
              Ingress Layer
            </div>
            {fallbackIngress.map((m) => (
              <div
                key={m.serviceName}
                style={{
                  width: '165px',
                  padding: '14px',
                  background: 'var(--bg-card)',
                  border: '1px solid var(--accent-emerald)',
                  borderRadius: '10px',
                  textAlign: 'center',
                  boxShadow: '0 0 15px rgba(16, 185, 129, 0.15)',
                }}
              >
                <Server size={22} style={{ color: 'var(--accent-emerald)', marginBottom: '4px' }} />
                <div style={{ fontWeight: 600, fontSize: '0.82rem', color: '#fff', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {m.serviceName}
                </div>
                <span className="badge healthy" style={{ fontSize: '0.65rem', marginTop: '6px' }}>
                  {m.p50Ms}ms p50
                </span>
              </div>
            ))}
          </div>

          <ArrowRight size={24} style={{ color: 'var(--text-muted)' }} />

          {/* Level 2: Core Processing Layer */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px', alignItems: 'center' }}>
            <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em', fontWeight: 600 }}>
              Processing Layer ({coreServices.length})
            </div>
            {coreServices.map((m) => {
              const isCrit = m.status === 'CRITICAL' || m.errorRate > 1.5;
              return (
                <div
                  key={m.serviceName}
                  style={{
                    width: '170px',
                    padding: '12px 14px',
                    background: 'var(--bg-card)',
                    border: isCrit ? '1px solid var(--status-critical)' : '1px solid var(--border-medium)',
                    borderRadius: '10px',
                    textAlign: 'center',
                    boxShadow: isCrit ? '0 0 15px rgba(239, 68, 68, 0.25)' : 'none',
                  }}
                >
                  <ShieldCheck size={20} style={{ color: isCrit ? 'var(--status-critical)' : 'var(--accent-cyan)', marginBottom: '4px' }} />
                  <div style={{ fontWeight: 600, fontSize: '0.82rem', color: isCrit ? '#f87171' : '#fff', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {m.serviceName}
                  </div>
                  <span className={`badge ${isCrit ? 'critical' : 'healthy'}`} style={{ fontSize: '0.65rem', marginTop: '6px' }}>
                    {m.p95Ms}ms p95 {m.errorRate > 0 && `(${m.errorRate}% err)`}
                  </span>
                </div>
              );
            })}
          </div>

          <ArrowRight size={24} style={{ color: 'var(--text-muted)' }} />

          {/* Level 3: Downstream & Leaf Layer */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px', alignItems: 'center' }}>
            <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em', fontWeight: 600 }}>
              Downstream & Datastores ({downstreamServices.length})
            </div>
            {downstreamServices.map((m) => {
              const isCrit = m.status === 'CRITICAL' || m.errorRate > 1;
              return (
                <div
                  key={m.serviceName}
                  style={{
                    width: '170px',
                    padding: '12px 14px',
                    background: 'var(--bg-card)',
                    border: isCrit ? '1px solid var(--status-critical)' : '1px solid var(--border-medium)',
                    borderRadius: '10px',
                    textAlign: 'center',
                    boxShadow: isCrit ? '0 0 15px rgba(239, 68, 68, 0.2)' : 'none',
                  }}
                >
                  {isCrit ? (
                    <Activity size={20} style={{ color: 'var(--status-critical)', marginBottom: '4px' }} />
                  ) : (
                    <Database size={20} style={{ color: 'var(--accent-emerald)', marginBottom: '4px' }} />
                  )}
                  <div style={{ fontWeight: 600, fontSize: '0.82rem', color: isCrit ? '#f87171' : '#fff', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {m.serviceName}
                  </div>
                  <span className={`badge ${isCrit ? 'critical' : 'healthy'}`} style={{ fontSize: '0.65rem', marginTop: '6px' }}>
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
