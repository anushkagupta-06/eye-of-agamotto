import React from 'react';
import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  GitBranch,
  FileText,
  Network,
  AlertTriangle,
  Bot,
  Settings,
  Activity,
  Eye,
  Key,
  LogOut
} from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { useTelemetry } from '../../context/TelemetryContext';

export const Sidebar: React.FC = () => {
  const { tenant, anomalies, logout } = useTelemetry();
  const navigate = useNavigate();
  const openAnomalies = anomalies.filter((a) => a.status === 'OPEN').length;

  const handleSignOut = () => {
    logout();
    navigate('/login');
  };

  return (
    <aside className="sidebar">
      {/* Brand Header */}
      <div className="sidebar-header">
        <div className="brand-icon">
          <Eye size={22} strokeWidth={2.4} />
        </div>
        <div>
          <div className="brand-title">Eye of Agamotto</div>
          <div className="brand-subtitle">Telemetry Engine</div>
        </div>
      </div>

      {/* Navigation Links */}
      <nav className="sidebar-nav">
        <div className="nav-section-title">Observability</div>

        <NavLink
          to="/"
          end
          className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}
        >
          <LayoutDashboard size={18} />
          <span>Dashboard</span>
        </NavLink>

        <NavLink
          to="/traces"
          className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}
        >
          <GitBranch size={18} />
          <span>Traces & Flamegraph</span>
        </NavLink>

        <NavLink
          to="/logs"
          className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}
        >
          <FileText size={18} />
          <span>Live Logs</span>
        </NavLink>

        <NavLink
          to="/topology"
          className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}
        >
          <Network size={18} />
          <span>Service Topology</span>
        </NavLink>

        <div className="nav-section-title">Intelligence</div>

        <NavLink
          to="/anomalies"
          className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}
        >
          <AlertTriangle size={18} />
          <span>Anomalies & RCA</span>
          {openAnomalies > 0 && <span className="nav-badge">{openAnomalies}</span>}
        </NavLink>

        <NavLink
          to="/assistant"
          className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}
        >
          <Bot size={18} />
          <span>AI Assistant (MCP)</span>
        </NavLink>

        <div className="nav-section-title">Platform</div>

        <NavLink
          to="/settings"
          className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}
        >
          <Settings size={18} />
          <span>Settings & Ingest</span>
        </NavLink>
      </nav>

      {/* Sidebar Footer */}
      <div className="sidebar-footer">
        <div className="tenant-badge">
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <Key size={13} style={{ color: 'var(--text-muted)' }} />
            <span style={{ color: 'var(--text-secondary)' }}>Tenant:</span>
            <strong style={{ color: 'var(--text-primary)' }}>{tenant}</strong>
          </div>
          <span className="badge healthy" style={{ fontSize: '0.65rem', padding: '1px 5px' }}>
            ACTIVE
          </span>
        </div>

        <div className="pipeline-mini-status">
          <span className="status-dot healthy"></span>
          <span>Pipeline: NATS & CH Online</span>
          <Activity size={13} style={{ marginLeft: 'auto', color: 'var(--accent-emerald)' }} />
        </div>

        <button
          onClick={handleSignOut}
          className="btn-secondary"
          style={{
            marginTop: '4px',
            padding: '6px 10px',
            fontSize: '0.74rem',
            justifyContent: 'center',
            color: 'var(--text-muted)',
            width: '100%',
          }}
        >
          <LogOut size={13} />
          <span>Sign Out</span>
        </button>
      </div>
    </aside>
  );
};
