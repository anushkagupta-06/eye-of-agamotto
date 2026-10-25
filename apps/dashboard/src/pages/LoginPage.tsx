import React, { useState } from 'react';
import { Eye, Key, ShieldCheck, ArrowRight, Sparkles, AlertCircle } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { useTelemetry } from '../context/TelemetryContext';

export const LoginPage: React.FC = () => {
  const { login } = useTelemetry();
  const navigate = useNavigate();

  const [tenantInput, setTenantInput] = useState('demo');
  const [keyInput, setKeyInput] = useState('eoa_cffdedbba2a3d4f0ecb4706923ae02d5d4b2ac1a0c03ff44');
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const success = login(tenantInput, keyInput);
    if (success) {
      navigate('/');
    } else {
      setError('Invalid Tenant ID or API key. Key must start with "eoa_" or match the demo key.');
    }
  };

  const handleQuickDemo = () => {
    login('demo', 'eoa_cffdedbba2a3d4f0ecb4706923ae02d5d4b2ac1a0c03ff44');
    navigate('/');
  };

  return (
    <div
      style={{
        minHeight: '100vh',
        width: '100vw',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'radial-gradient(circle at 50% 30%, #151d2d 0%, #090b10 80%)',
        padding: '20px',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      {/* Background glow effects */}
      <div
        style={{
          position: 'absolute',
          width: '500px',
          height: '500px',
          borderRadius: '50%',
          background: 'rgba(16, 185, 129, 0.08)',
          filter: 'blur(100px)',
          top: '10%',
          left: '30%',
          pointerEvents: 'none',
        }}
      />
      <div
        style={{
          position: 'absolute',
          width: '400px',
          height: '400px',
          borderRadius: '50%',
          background: 'rgba(139, 92, 246, 0.08)',
          filter: 'blur(90px)',
          bottom: '10%',
          right: '25%',
          pointerEvents: 'none',
        }}
      />

      <div
        style={{
          width: '100%',
          maxWidth: '460px',
          backgroundColor: 'rgba(16, 20, 30, 0.9)',
          backdropFilter: 'blur(16px)',
          border: '1px solid var(--border-medium)',
          borderRadius: '16px',
          padding: '36px 32px',
          boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.6), 0 0 30px rgba(16, 185, 129, 0.1)',
          position: 'relative',
          zIndex: 10,
        }}
      >
        {/* Header with Emblem */}
        <div style={{ textAlign: 'center', marginBottom: '28px' }}>
          <div
            style={{
              width: '56px',
              height: '56px',
              borderRadius: '16px',
              background: 'linear-gradient(135deg, #059669 0%, #10b981 50%, #06b6d4 100%)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              margin: '0 auto 16px',
              boxShadow: '0 0 25px rgba(16, 185, 129, 0.35)',
              color: 'white',
            }}
          >
            <Eye size={32} strokeWidth={2.4} />
          </div>

          <h1 style={{ fontSize: '1.45rem', fontWeight: 700, color: '#fff', letterSpacing: '-0.02em', margin: 0 }}>
            Eye of Agamotto
          </h1>
          <p style={{ fontSize: '0.82rem', color: 'var(--text-secondary)', marginTop: '4px' }}>
            Distributed Telemetry & Observability Engine
          </p>
        </div>

        {/* Error message if any */}
        {error && (
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              padding: '10px 14px',
              backgroundColor: 'rgba(239, 68, 68, 0.15)',
              border: '1px solid rgba(239, 68, 68, 0.35)',
              borderRadius: '8px',
              fontSize: '0.78rem',
              color: '#fca5a5',
              marginBottom: '20px',
            }}
          >
            <AlertCircle size={15} style={{ flexShrink: 0 }} />
            <span>{error}</span>
          </div>
        )}

        {/* Form */}
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>
          {/* Tenant ID */}
          <div>
            <label
              style={{
                display: 'block',
                fontSize: '0.78rem',
                fontWeight: 600,
                color: 'var(--text-secondary)',
                marginBottom: '6px',
                textTransform: 'uppercase',
                letterSpacing: '0.04em',
              }}
            >
              Tenant Identifier
            </label>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '10px',
                backgroundColor: 'var(--bg-card)',
                border: '1px solid var(--border-subtle)',
                borderRadius: '8px',
                padding: '10px 14px',
              }}
            >
              <ShieldCheck size={16} style={{ color: 'var(--accent-emerald)' }} />
              <input
                type="text"
                required
                value={tenantInput}
                onChange={(e) => setTenantInput(e.target.value)}
                placeholder="e.g. demo"
                style={{
                  background: 'transparent',
                  border: 'none',
                  outline: 'none',
                  color: '#fff',
                  width: '100%',
                  fontSize: '0.88rem',
                }}
              />
            </div>
          </div>

          {/* API Key */}
          <div>
            <label
              style={{
                display: 'block',
                fontSize: '0.78rem',
                fontWeight: 600,
                color: 'var(--text-secondary)',
                marginBottom: '6px',
                textTransform: 'uppercase',
                letterSpacing: '0.04em',
              }}
            >
              Ingest & Query API Key
            </label>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '10px',
                backgroundColor: 'var(--bg-card)',
                border: '1px solid var(--border-subtle)',
                borderRadius: '8px',
                padding: '10px 14px',
              }}
            >
              <Key size={16} style={{ color: 'var(--accent-cyan)' }} />
              <input
                type="password"
                required
                value={keyInput}
                onChange={(e) => setKeyInput(e.target.value)}
                placeholder="eoa_..."
                style={{
                  background: 'transparent',
                  border: 'none',
                  outline: 'none',
                  color: '#fff',
                  width: '100%',
                  fontSize: '0.88rem',
                  fontFamily: 'var(--font-mono)',
                }}
              />
            </div>
            <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', marginTop: '6px' }}>
              Checked against SHA-256 digest in <code>.data/api-keys.txt</code>
            </div>
          </div>

          {/* Submit Button */}
          <button
            type="submit"
            className="btn-primary"
            style={{
              padding: '12px',
              fontSize: '0.9rem',
              justifyContent: 'center',
              marginTop: '6px',
            }}
          >
            <span>Authenticate & Enter Console</span>
            <ArrowRight size={16} />
          </button>

          {/* Quick Demo Fill button */}
          <button
            type="button"
            onClick={handleQuickDemo}
            className="btn-secondary"
            style={{
              padding: '10px',
              fontSize: '0.82rem',
              justifyContent: 'center',
              borderColor: 'rgba(16, 185, 129, 0.3)',
              color: 'var(--accent-emerald)',
            }}
          >
            <Sparkles size={14} />
            <span>Quick Demo Login (demo tenant)</span>
          </button>
        </form>

        {/* Footer info */}
        <div
          style={{
            marginTop: '24px',
            paddingTop: '16px',
            borderTop: '1px solid var(--border-subtle)',
            textAlign: 'center',
            fontSize: '0.74rem',
            color: 'var(--text-muted)',
          }}
        >
          Eye of Agamotto • NATS JetStream & ClickHouse Telemetry
        </div>
      </div>
    </div>
  );
};
