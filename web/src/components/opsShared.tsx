// Shared presentation helpers for operational admin views.
// Zero emojis, real data only.

import React from 'react';
import { AlertCircle, Inbox } from 'lucide-react';

export const OpsHeader: React.FC<{
  title: string;
  subtitle: string;
  action?: React.ReactNode;
}> = ({ title, subtitle, action }) => (
  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '16px' }}>
    <div>
      <h1 style={{ fontSize: '1.6rem', fontWeight: 800, color: 'var(--text-main)', letterSpacing: '-0.02em', marginBottom: '4px' }}>
        {title}
      </h1>
      <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', maxWidth: '760px' }}>{subtitle}</p>
    </div>
    {action}
  </div>
);

export const OpsError: React.FC<{ message: string; onDismiss?: () => void }> = ({ message, onDismiss }) => (
  <div className="glass-card" style={{ padding: '12px 16px', display: 'flex', alignItems: 'center', gap: '10px', borderColor: 'rgba(239, 68, 68, 0.35)' }}>
    <AlertCircle size={16} style={{ color: '#ef4444', flexShrink: 0 }} />
    <span style={{ fontSize: '0.82rem', flex: 1 }}>{message}</span>
    {onDismiss ? (
      <button type="button" className="btn btn-secondary btn-sm" onClick={onDismiss}>
        Dismiss
      </button>
    ) : null}
  </div>
);

export const OpsEmpty: React.FC<{ message: string; children?: React.ReactNode }> = ({ message, children }) => (
  <div className="glass-panel" style={{ padding: '44px', textAlign: 'center' }}>
    <Inbox size={28} style={{ color: 'var(--text-dim)', marginBottom: '10px' }} />
    <p style={{ fontSize: '0.88rem', color: 'var(--text-muted)' }}>{message}</p>
    {children}
  </div>
);

export const UsageBar: React.FC<{ used: number; limit: number; label: string }> = ({ used, limit, label }) => {
  const ratio = limit > 0 ? Math.min(1, Math.max(0, used / limit)) : 0;
  const colour = ratio >= 0.9 ? '#ef4444' : ratio >= 0.75 ? '#f59e0b' : 'var(--brand-primary)';
  return (
    <div style={{ minWidth: '130px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.7rem', color: 'var(--text-muted)', marginBottom: '3px' }}>
        <span>{label}</span>
        <span>{Math.round(ratio * 100)}%</span>
      </div>
      <div style={{ height: '5px', borderRadius: '999px', background: 'rgba(148, 163, 184, 0.18)', overflow: 'hidden' }}>
        <div style={{ width: `${ratio * 100}%`, height: '100%', background: colour }} />
      </div>
    </div>
  );
};

export const formatBytes = (bytes: number): string => {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const exponent = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
  const value = bytes / Math.pow(1024, exponent);
  return `${value >= 100 || exponent === 0 ? Math.round(value) : value.toFixed(1)} ${units[exponent]}`;
};

export const formatDate = (value: string): string => {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
};

export const StatusPill: React.FC<{ status: string }> = ({ status }) => {
  const normalised = status.toLowerCase();
  const style: React.CSSProperties =
    normalised === 'active' || normalised === 'online' || normalised === 'installed'
      ? { background: 'rgba(16, 185, 129, 0.12)', color: 'var(--status-online)' }
      : normalised === 'suspended' || normalised === 'degraded' || normalised === 'failed'
        ? { background: 'rgba(245, 158, 11, 0.14)', color: 'var(--status-warning)' }
        : normalised === 'terminated' || normalised === 'offline'
          ? { background: 'rgba(239, 68, 68, 0.12)', color: '#ef4444' }
          : { background: 'rgba(148, 163, 184, 0.12)', color: 'var(--text-muted)' };
  return (
    <span className="badge-pill" style={{ ...style, textTransform: 'capitalize' }}>
      {status}
    </span>
  );
};
