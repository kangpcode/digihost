// DigiPanel managed nftables blocklist.
// Rules are stored by the core and rendered into the "inet digipanel" table by
// the privileged agent. Applying is explicit so the operator sees the generated
// ruleset and any apply error instead of a silent success.

import React, { useCallback, useEffect, useState } from 'react';
import { Ban, Play, RefreshCw, ShieldAlert, ShieldCheck, Trash2 } from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';
import { OpsEmpty, OpsError, OpsHeader, formatDate } from './opsShared';

interface FirewallRule {
  id: string;
  target: string;
  family: string;
  reason: string;
  createdAt: string;
  expiresAt?: string;
}

interface FirewallStatus {
  agentAvailable: boolean;
  applied: boolean;
  table: string;
  activeRules: number;
  lastAppliedAt?: string;
  lastError?: string;
  ruleset: string;
}

const DURATIONS = [
  { hours: 0, key: 'fwPermanent' },
  { hours: 1, key: 'fwHour1' },
  { hours: 24, key: 'fwHours24' },
  { hours: 168, key: 'fwHours168' },
  { hours: 720, key: 'fwHours720' }
];

export const FirewallView: React.FC<{ language: Language }> = ({ language }) => {
  const t = (key: any) => getTranslation(language, key);

  const [rules, setRules] = useState<FirewallRule[]>([]);
  const [status, setStatus] = useState<FirewallStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState('');
  const [target, setTarget] = useState('');
  const [reason, setReason] = useState('');
  const [durationHours, setDurationHours] = useState(24);
  const [showRuleset, setShowRuleset] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    const [ruleResult, statusResult] = await Promise.allSettled([
      apiFetch<FirewallRule[]>('/api/v1/firewall/rules'),
      apiFetch<FirewallStatus>('/api/v1/firewall/status')
    ]);
    if (ruleResult.status === 'fulfilled') {
      setRules(Array.isArray(ruleResult.value) ? ruleResult.value : []);
    } else {
      setRules([]);
      setError(ruleResult.reason instanceof Error ? ruleResult.reason.message : String(ruleResult.reason));
    }
    if (statusResult.status === 'fulfilled') {
      setStatus(statusResult.value);
    } else {
      setStatus(null);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const applyRules = async () => {
    setBusy('apply');
    setError('');
    setNotice('');
    try {
      await apiFetch('/api/v1/firewall/apply', { method: 'POST', body: JSON.stringify({}) });
      setNotice(t('fwApplied'));
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
      await load();
    } finally {
      setBusy('');
    }
  };

  const blockTarget = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy('block');
    setError('');
    setNotice('');
    try {
      const rule = await apiFetch<FirewallRule>('/api/v1/firewall/rules', {
        method: 'POST',
        body: JSON.stringify({ target: target.trim(), reason: reason.trim(), durationHours: Number(durationHours) })
      });
      setNotice(`${t('fwBlocked')}: ${rule.target}${rule.expiresAt ? ` (${t('fwUntil')} ${formatDate(rule.expiresAt)})` : ` (${t('fwPermanent')})`}`);
      setTarget('');
      setReason('');
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  const unblock = async (rule: FirewallRule) => {
    if (!window.confirm(`${t('fwUnblockConfirm')} ${rule.target}?`)) return;
    setBusy(rule.id);
    setError('');
    setNotice('');
    try {
      await apiFetch(`/api/v1/firewall/rules?id=${encodeURIComponent(rule.id)}`, { method: 'DELETE' });
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  const agentAvailable = status?.agentAvailable ?? false;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <OpsHeader
        title={t('navFirewall')}
        subtitle={t('fwSubtitle')}
        action={
          <div style={{ display: 'flex', gap: '8px' }}>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => void load()} disabled={loading}>
              <RefreshCw size={14} />
              {t('refresh')}
            </button>
            <button type="button" className="btn btn-primary btn-sm" onClick={() => void applyRules()} disabled={busy === 'apply' || rules.length === 0}>
              <Play size={14} />
              {busy === 'apply' ? t('fwApplying') : t('fwApply')}
            </button>
          </div>
        }
      />

      {error ? <OpsError message={error} onDismiss={() => setError('')} /> : null}
      {notice ? (
        <div className="glass-card" style={{ padding: '10px 14px', fontSize: '0.8rem', borderColor: 'rgba(16, 185, 129, 0.35)', wordBreak: 'break-all' }}>
          {notice}
        </div>
      ) : null}

      {status ? (
        <div className="glass-card" style={{ padding: '12px 16px', display: 'flex', flexDirection: 'column', gap: '6px', fontSize: '0.78rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
            {status.applied ? (
              <ShieldCheck size={15} style={{ color: 'var(--status-online)' }} />
            ) : (
              <ShieldAlert size={15} style={{ color: 'var(--status-warning)' }} />
            )}
            <span className="font-mono" style={{ fontWeight: 700 }}>{status.table}</span>
            <span className="badge-pill" style={{
              background: status.applied ? 'rgba(16, 185, 129, 0.12)' : 'rgba(245, 158, 11, 0.14)',
              color: status.applied ? 'var(--status-online)' : 'var(--status-warning)'
            }}>
              {status.applied ? t('fwStateApplied') : t('fwStatePending')}
            </span>
            <span style={{ color: 'var(--text-muted)' }}>
              {status.activeRules} {t('fwActiveRules')}
            </span>
            <span style={{ color: 'var(--text-muted)' }}>
              {t('fwAgent')}: {status.agentAvailable ? t('fwAgentOnline') : t('fwAgentOffline')}
            </span>
            {status.lastAppliedAt ? (
              <span style={{ color: 'var(--text-muted)' }}>{t('fwLastApplied')}: {formatDate(status.lastAppliedAt)}</span>
            ) : null}
            {status.ruleset ? (
              <button type="button" className="btn btn-secondary btn-sm" onClick={() => setShowRuleset((value) => !value)}>
                {showRuleset ? t('fwHideRuleset') : t('fwShowRuleset')}
              </button>
            ) : null}
          </div>
          {status.lastError ? <span style={{ color: '#ef4444' }}>{status.lastError}</span> : null}
          {showRuleset && status.ruleset ? (
            <pre className="font-mono" style={{
              margin: 0,
              padding: '10px',
              fontSize: '0.7rem',
              lineHeight: 1.5,
              background: 'rgba(148, 163, 184, 0.08)',
              borderRadius: '8px',
              overflowX: 'auto',
              maxHeight: '320px'
            }}>
              {status.ruleset}
            </pre>
          ) : null}
        </div>
      ) : null}

      <form className="glass-panel" style={{ padding: '16px', display: 'flex', gap: '10px', flexWrap: 'wrap', alignItems: 'flex-end' }} onSubmit={blockTarget}>
        <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.74rem', fontWeight: 600, color: 'var(--text-muted)', minWidth: '200px' }}>
          {t('fwTarget')}
          <input
            className="input-glass"
            value={target}
            onChange={(event) => setTarget(event.target.value)}
            placeholder="203.0.113.10"
            required
          />
        </label>
        <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.74rem', fontWeight: 600, color: 'var(--text-muted)', minWidth: '240px', flex: 1 }}>
          {t('fwReason')}
          <input
            className="input-glass"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder={t('fwReasonPlaceholder')}
            maxLength={120}
          />
        </label>
        <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.74rem', fontWeight: 600, color: 'var(--text-muted)' }}>
          {t('fwDuration')}
          <select className="input-glass" value={durationHours} onChange={(event) => setDurationHours(Number(event.target.value))}>
            {DURATIONS.map((option) => (
              <option key={option.hours} value={option.hours}>{t(option.key)}</option>
            ))}
          </select>
        </label>
        <button type="submit" className="btn btn-primary btn-sm" disabled={busy === 'block' || !agentAvailable}>
          <Ban size={14} />
          {t('fwBlock')}
        </button>
      </form>

      {!agentAvailable ? (
        <div className="glass-card" style={{ padding: '10px 14px', fontSize: '0.78rem', color: 'var(--status-warning)', borderColor: 'rgba(245, 158, 11, 0.35)' }}>
          {t('fwAgentOfflineHint')}
        </div>
      ) : null}

      {rules.length === 0 ? (
        <OpsEmpty message={t('fwEmpty')} />
      ) : (
        <div className="glass-panel" style={{ overflowX: 'auto' }}>
          <table className="glass-table">
            <thead>
              <tr>
                <th>{t('fwTarget')}</th>
                <th>{t('fwFamily')}</th>
                <th>{t('fwReason')}</th>
                <th>{t('fwCreated')}</th>
                <th>{t('fwExpires')}</th>
                <th style={{ textAlign: 'right' }}>{t('thActions')}</th>
              </tr>
            </thead>
            <tbody>
              {rules.map((rule) => (
                <tr key={rule.id}>
                  <td className="font-mono" style={{ fontSize: '0.78rem', fontWeight: 700 }}>{rule.target}</td>
                  <td className="font-mono" style={{ fontSize: '0.74rem' }}>{rule.family}</td>
                  <td style={{ fontSize: '0.76rem' }}>{rule.reason || '-'}</td>
                  <td style={{ fontSize: '0.74rem' }}>{formatDate(rule.createdAt)}</td>
                  <td style={{ fontSize: '0.74rem' }}>{rule.expiresAt ? formatDate(rule.expiresAt) : t('fwPermanent')}</td>
                  <td style={{ textAlign: 'right' }}>
                    <button type="button" className="btn btn-secondary btn-sm" onClick={() => void unblock(rule)} disabled={busy === rule.id}>
                      <Trash2 size={12} />
                      {t('fwUnblock')}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

export default FirewallView;
