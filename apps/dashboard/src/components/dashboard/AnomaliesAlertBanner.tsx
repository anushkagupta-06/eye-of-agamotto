import React from 'react';
import { ArrowRight, ShieldAlert } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useTelemetry } from '../../context/TelemetryContext';

export const AnomaliesAlertBanner: React.FC = () => {
  const { anomalies } = useTelemetry();
  const criticalAnomaly = anomalies.find((a) => a.severity === 'CRITICAL' && a.status === 'OPEN');

  if (!criticalAnomaly) return null;

  return (
    <div className="alert-banner">
      <div className="alert-content">
        <div style={{ color: 'var(--status-critical)' }}>
          <ShieldAlert size={28} />
        </div>
        <div>
          <div className="alert-title">
            Anomaly Detected: {criticalAnomaly.serviceName} ({criticalAnomaly.operation})
          </div>
          <div className="alert-subtitle">
            Observed P99 latency spiked to {criticalAnomaly.observedValue}ms (Baseline: {criticalAnomaly.baselineValue}ms, Z-Score: {criticalAnomaly.zScore}σ).
            Root Cause: <strong style={{ color: '#fff' }}>{criticalAnomaly.rootCauseService}</strong>.
          </div>
        </div>
      </div>

      <div className="alert-actions">
        <Link to="/anomalies" className="btn-primary" style={{ textDecoration: 'none' }}>
          <span>Investigate RCA</span>
          <ArrowRight size={14} />
        </Link>
      </div>
    </div>
  );
};
