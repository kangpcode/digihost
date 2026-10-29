// DigiPanel Hosting Accounts view.
// Lists the accounts owned by the hosting manager and performs the real
// suspend, unsuspend, and terminate operations exposed by the core API.

import React, { useEffect, useMemo, useState } from 'react';
import { Plus, RefreshCw, Search, PauseCircle, PlayCircle, Trash2, Database, Mail, Globe } from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';
import { OpsEmpty, OpsError, OpsHeader, StatusPill, UsageBar, formatBytes, formatDate } from './opsShared';

interface Account {
  id: string;
  tenantId: string;
  username: string;
  primaryDomain: string;
  packageId: string;
  homeDir: string;
  documentRoot: string;
  phpVersion: string;
  diskLimitBytes: number;
  diskUsedBytes: number;
  bandwidthLimitBytes: number;
  bandwidthUsedBytes: number;
  inodesLimit: number;
  inodesUsed: number;
  status: string;
  ipAddress: string;
  createdAt: string;
}

export const AccountsListView: React.FC<{
  language: Language;
  onOpenNewAccount: () => void;
}> = ({ language, onOpenNewAccount }) => {
  const t = (key: any) => getTranslation(language, key);

  const [accounts, setAccounts] = useState<Account[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [busy, setBusy] = useState('');
  const [plans, setPlans] = useState<Array<{ name: string }>>([]);

  const load = async () => {
    try {
      setLoading(true);
      setError('');
      const [data, planData] = await Promise.all([
        apiFetch<Account[]>('/api/v1/hosting/accounts'),
        apiFetch<Array<{ name: string }>>('/api/v1/hosting/packages')
      ]);
      setAccounts(Array.isArray(data) ? data : []);
      setPlans(Array.isArray(planData) ? planData : []);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  };

  const changePackage = async (account: Account, nextPackage: string) => {
    if (nextPackage === account.packageId) return;
    try {
      setBusy(account.username);
      setError('');
      await apiFetch('/api/v1/hosting/accounts/package', {
        method: 'POST',
        body: JSON.stringify({ username: account.username, package: nextPackage })
      });
      setNotice(`${account.username}: plan changed to ${nextPackage}`);
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return accounts.filter((account) => {
      if (statusFilter !== 'all' && account.status !== statusFilter) return false;
      if (!needle) return true;
      return (
        account.username.toLowerCase().includes(needle) ||
        account.primaryDomain.toLowerCase().includes(needle) ||
        account.packageId.toLowerCase().includes(needle)
      );
    });
  }, [accounts, search, statusFilter]);

  const act = async (account: Account, action: 'suspend' | 'unsuspend' | 'terminate') => {
    if (action === 'terminate') {
      const confirmed = window.confirm(
        `Terminate ${account.username} (${account.primaryDomain})? Files, DNS, and the hosting record are removed.`
      );
      if (!confirmed) return;
    }
    try {
      setBusy(account.username);
      setError('');
      await apiFetch<Record<string, boolean>>(`/api/v1/hosting/accounts/${action}`, {
        method: 'POST',
        body: JSON.stringify({ username: account.username })
      });
      setNotice(`${account.username}: ${action} completed`);
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <OpsHeader
        title={t('accountsTitle')}
        subtitle={t('accountsSubtitle')}
        action={
          <div style={{ display: 'flex', gap: '8px' }}>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => void load()} disabled={loading}>
              <RefreshCw size={14} />
              {t('refresh')}
            </button>
            <button type="button" className="btn btn-primary btn-sm" onClick={onOpenNewAccount}>
              <Plus size={14} />
              {t('newAccount')}
            </button>
          </div>
        }
      />

      {notice ? (
        <div className="glass-card" style={{ padding: '10px 14px', fontSize: '0.82rem', borderColor: 'rgba(16, 185, 129, 0.35)' }}>
          {notice}
        </div>
      ) : null}

      {error ? <OpsError message={error} onDismiss={() => setError('')} /> : null}

      <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
        <div style={{ position: 'relative', flex: '1 1 280px', maxWidth: '380px' }}>
          <Search size={15} style={{ position: 'absolute', left: '12px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
          <input
            className="input-glass"
            style={{ paddingLeft: '36px', height: '38px' }}
            placeholder={t('accountsSearchPlaceholder')}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>
        <select className="input-glass" style={{ width: '170px', height: '38px' }} value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
          <option value="all">{t('accountsAllStatuses')}</option>
          <option value="active">active</option>
          <option value="suspended">suspended</option>
          <option value="terminated">terminated</option>
        </select>
      </div>

      {loading && accounts.length === 0 ? (
        <div className="glass-panel" style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>{t('appsLoading')}</div>
      ) : filtered.length === 0 ? (
        <OpsEmpty message={accounts.length === 0 ? t('accountsEmpty') : t('accountsNoMatches')}>
          {accounts.length === 0 ? (
            <button type="button" className="btn btn-primary btn-sm" style={{ marginTop: '14px' }} onClick={onOpenNewAccount}>
              <Plus size={14} />
              {t('newAccount')}
            </button>
          ) : null}
        </OpsEmpty>
      ) : (
        <div className="glass-panel" style={{ overflowX: 'auto' }}>
          <table className="glass-table">
            <thead>
              <tr>
                <th>{t('accountsAccount')}</th>
                <th>{t('thPlan')}</th>
                <th>{t('accountsResources')}</th>
                <th>{t('appsStatus')}</th>
                <th>{t('accountsRuntime')}</th>
                <th>{t('accountsCreated')}</th>
                <th style={{ textAlign: 'right' }}>{t('appsActions')}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((account) => (
                <tr key={account.id}>
                  <td>
                    <div style={{ fontWeight: 700, fontSize: '0.84rem' }}>{account.username}</div>
                    <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                      <Globe size={11} />
                      {account.primaryDomain}
                    </div>
                    <div className="font-mono" style={{ fontSize: '0.66rem', color: 'var(--text-dim)' }}>{account.documentRoot}</div>
                  </td>
                  <td style={{ fontSize: '0.78rem' }}>
                    <select
                      className="input-glass"
                      style={{ height: '28px', fontSize: '0.76rem', paddingTop: 0, paddingBottom: 0 }}
                      value={account.packageId}
                      disabled={busy === account.username || account.status === 'terminated'}
                      onChange={(event) => void changePackage(account, event.target.value)}
                      title={t('plansTitle')}
                    >
                      {plans.some((plan) => plan.name === account.packageId) ? null : (
                        <option value={account.packageId}>{account.packageId}</option>
                      )}
                      {plans.map((plan) => (
                        <option key={plan.name} value={plan.name}>{plan.name}</option>
                      ))}
                    </select>
                    <div style={{ fontSize: '0.68rem', color: 'var(--text-muted)', marginTop: '3px' }}>{account.ipAddress}</div>
                  </td>
                  <td>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '7px' }}>
                      <UsageBar used={account.diskUsedBytes} limit={account.diskLimitBytes} label={`${t('diskUsage')} ${formatBytes(account.diskUsedBytes)}`} />
                      <UsageBar used={account.bandwidthUsedBytes} limit={account.bandwidthLimitBytes} label={`${t('bandwidthUsage')} ${formatBytes(account.bandwidthUsedBytes)}`} />
                      <UsageBar used={account.inodesUsed} limit={account.inodesLimit} label={`${t('inodesUsage')} ${account.inodesUsed}`} />
                    </div>
                  </td>
                  <td><StatusPill status={account.status} /></td>
                  <td style={{ fontSize: '0.76rem' }}>
                    <div>PHP {account.phpVersion}</div>
                    <div style={{ fontSize: '0.68rem', color: 'var(--text-muted)', display: 'flex', gap: '8px' }}>
                      <span style={{ display: 'inline-flex', alignItems: 'center', gap: '3px' }}><Database size={10} /> {account.inodesUsed}</span>
                      <span style={{ display: 'inline-flex', alignItems: 'center', gap: '3px' }}><Mail size={10} /> {account.tenantId}</span>
                    </div>
                  </td>
                  <td style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>{formatDate(account.createdAt)}</td>
                  <td>
                    <div style={{ display: 'flex', gap: '4px', justifyContent: 'flex-end' }}>
                      {account.status === 'suspended' ? (
                        <button type="button" className="btn btn-secondary btn-sm" disabled={busy === account.username} onClick={() => void act(account, 'unsuspend')} title={t('accountsUnsuspend')}>
                          <PlayCircle size={13} />
                        </button>
                      ) : (
                        <button type="button" className="btn btn-secondary btn-sm" disabled={busy === account.username || account.status === 'terminated'} onClick={() => void act(account, 'suspend')} title={t('accountsSuspend')}>
                          <PauseCircle size={13} />
                        </button>
                      )}
                      <button type="button" className="btn btn-secondary btn-sm" disabled={busy === account.username || account.status === 'terminated'} onClick={() => void act(account, 'terminate')} title={t('accountsTerminate')}>
                        <Trash2 size={13} style={{ color: '#ef4444' }} />
                      </button>
                    </div>
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
