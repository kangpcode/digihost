// DigiPanel Hosting Plans view.
// Plans are the limit definitions the hosting engine enforces at provisioning
// time. Creating, editing, and deleting a plan here changes the live definition
// used by account creation; a plan assigned to an account cannot be deleted.

import React, { useEffect, useMemo, useState } from 'react';
import { RefreshCw, HardDrive, Network, Database, Mail, Globe, Cpu, ListFilter, Plus, Pencil, Trash2, X, ShieldAlert } from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';
import { OpsEmpty, OpsError, OpsHeader, formatBytes } from './opsShared';

interface AccountPackage {
  id: string;
  name: string;
  diskLimitBytes: number;
  bandwidthLimitBytes: number;
  inodesLimit: number;
  maxDomains: number;
  maxDatabases: number;
  maxEmailAccounts: number;
  defaultPhpVersion: string;
  memoryLimitMb: number;
  pidsLimit: number;
}

interface AccountLite {
  id: string;
  packageId: string;
}

interface PlanFormState {
  name: string;
  diskGb: string;
  bandwidthGb: string;
  inodesLimit: string;
  maxDomains: string;
  maxDatabases: string;
  maxEmailAccounts: string;
  memoryLimitMb: string;
  pidsLimit: string;
  defaultPhpVersion: string;
}

const emptyForm: PlanFormState = {
  name: '',
  diskGb: '5',
  bandwidthGb: '50',
  inodesLimit: '100000',
  maxDomains: '5',
  maxDatabases: '5',
  maxEmailAccounts: '10',
  memoryLimitMb: '512',
  pidsLimit: '75',
  defaultPhpVersion: '8.3'
};

const GIGABYTE = 1024 * 1024 * 1024;

const toForm = (plan: AccountPackage): PlanFormState => ({
  name: plan.name,
  diskGb: String(Math.round((plan.diskLimitBytes / GIGABYTE) * 10) / 10),
  bandwidthGb: String(Math.round((plan.bandwidthLimitBytes / GIGABYTE) * 10) / 10),
  inodesLimit: String(plan.inodesLimit),
  maxDomains: String(plan.maxDomains),
  maxDatabases: String(plan.maxDatabases),
  maxEmailAccounts: String(plan.maxEmailAccounts),
  memoryLimitMb: String(plan.memoryLimitMb),
  pidsLimit: String(plan.pidsLimit),
  defaultPhpVersion: plan.defaultPhpVersion
});

const numberOrZero = (value: string): number => {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? Math.round(parsed) : 0;
};

export const PlansView: React.FC<{ language: Language; canManage?: boolean }> = ({ language, canManage = false }) => {
  const t = (key: any) => getTranslation(language, key);
  const [plans, setPlans] = useState<AccountPackage[]>([]);
  const [accounts, setAccounts] = useState<AccountLite[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [editing, setEditing] = useState<string | null>(null);
  const [form, setForm] = useState<PlanFormState>(emptyForm);
  const [isFormOpen, setIsFormOpen] = useState(false);
  const [saving, setSaving] = useState(false);

  const load = async () => {
    try {
      setLoading(true);
      setError('');
      const [planData, accountData] = await Promise.all([
        apiFetch<AccountPackage[]>('/api/v1/hosting/packages'),
        apiFetch<AccountLite[]>('/api/v1/hosting/accounts')
      ]);
      setPlans(Array.isArray(planData) ? planData : []);
      setAccounts(Array.isArray(accountData) ? accountData : []);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const usage = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const account of accounts) {
      counts[account.packageId] = (counts[account.packageId] || 0) + 1;
    }
    return counts;
  }, [accounts]);

  const openCreate = () => {
    setEditing(null);
    setForm(emptyForm);
    setNotice('');
    setIsFormOpen(true);
  };

  const openEdit = (plan: AccountPackage) => {
    setEditing(plan.name);
    setForm(toForm(plan));
    setNotice('');
    setIsFormOpen(true);
  };

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (saving) return;
    setSaving(true);
    setError('');
    setNotice('');

    const payload = {
      name: form.name.trim().toLowerCase(),
      diskLimitBytes: Math.round(Number(form.diskGb) * GIGABYTE),
      bandwidthLimitBytes: Math.round(Number(form.bandwidthGb) * GIGABYTE),
      inodesLimit: numberOrZero(form.inodesLimit),
      maxDomains: numberOrZero(form.maxDomains),
      maxDatabases: numberOrZero(form.maxDatabases),
      maxEmailAccounts: numberOrZero(form.maxEmailAccounts),
      defaultPhpVersion: form.defaultPhpVersion.trim() || '8.3',
      memoryLimitMb: numberOrZero(form.memoryLimitMb),
      pidsLimit: numberOrZero(form.pidsLimit)
    };

    try {
      if (editing) {
        await apiFetch(`/api/v1/hosting/packages?name=${encodeURIComponent(editing)}`, {
          method: 'PUT',
          body: JSON.stringify(payload)
        });
        setNotice(`${t('plansSaved')}: ${payload.name}`);
      } else {
        await apiFetch('/api/v1/hosting/packages', {
          method: 'POST',
          body: JSON.stringify(payload)
        });
        setNotice(`${t('plansSaved')}: ${payload.name}`);
      }
      setIsFormOpen(false);
      setEditing(null);
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setSaving(false);
    }
  };

  const remove = async (plan: AccountPackage) => {
    if (!window.confirm(t('plansDeleteConfirm'))) return;
    try {
      setError('');
      await apiFetch(`/api/v1/hosting/packages?name=${encodeURIComponent(plan.name)}`, { method: 'DELETE' });
      setNotice(`${t('plansDeleted')}: ${plan.name}`);
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  const field = (label: string, value: string, onChange: (value: string) => void, type: 'text' | 'number' = 'number') => (
    <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.74rem', fontWeight: 600, color: 'var(--text-muted)' }}>
      {label}
      <input
        className="input-glass"
        style={{ height: '34px', fontSize: '0.8rem' }}
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        required
      />
    </label>
  );

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <OpsHeader
        title={t('plansTitle')}
        subtitle={t('plansSubtitle')}
        action={
          <div style={{ display: 'flex', gap: '8px' }}>
            {canManage ? (
              <button type="button" className="btn btn-primary btn-sm" onClick={openCreate}>
                <Plus size={14} />
                {t('plansNew')}
              </button>
            ) : null}
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => void load()} disabled={loading}>
              <RefreshCw size={14} />
              {t('refresh')}
            </button>
          </div>
        }
      />

      {error ? <OpsError message={error} onDismiss={() => setError('')} /> : null}
      {notice ? (
        <div className="glass-card" style={{ padding: '10px 14px', fontSize: '0.8rem', borderColor: 'rgba(16, 185, 129, 0.35)' }}>
          {notice}
        </div>
      ) : null}

      {isFormOpen ? (
        <form className="glass-panel" style={{ padding: '20px', display: 'flex', flexDirection: 'column', gap: '14px' }} onSubmit={submit}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <h2 style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--text-main)' }}>{editing ? t('plansEdit') : t('plansNew')}</h2>
              <p style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{t('plansFormSubtitle')}</p>
            </div>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => setIsFormOpen(false)}>
              <X size={14} />
              {t('plansCancel')}
            </button>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: '12px' }}>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.74rem', fontWeight: 600, color: 'var(--text-muted)' }}>
              {t('plansName')}
              <input
                className="input-glass"
                style={{ height: '34px', fontSize: '0.8rem' }}
                value={form.name}
                onChange={(event) => setForm({ ...form, name: event.target.value })}
                pattern="[a-z][a-z0-9\-]{1,31}"
                title="Lowercase letters, digits, and dashes; must start with a letter"
                disabled={editing !== null}
                required
              />
            </label>
            {field(`${t('plansDisk')} (GB)`, form.diskGb, (value) => setForm({ ...form, diskGb: value }))}
            {field(`${t('plansBandwidth')} (GB)`, form.bandwidthGb, (value) => setForm({ ...form, bandwidthGb: value }))}
            {field(t('plansInodes'), form.inodesLimit, (value) => setForm({ ...form, inodesLimit: value }))}
            {field(t('navDomains'), form.maxDomains, (value) => setForm({ ...form, maxDomains: value }))}
            {field(t('navDatabases'), form.maxDatabases, (value) => setForm({ ...form, maxDatabases: value }))}
            {field(t('navEmails'), form.maxEmailAccounts, (value) => setForm({ ...form, maxEmailAccounts: value }))}
            {field(t('plansMemory'), form.memoryLimitMb, (value) => setForm({ ...form, memoryLimitMb: value }))}
            {field(t('plansPids'), form.pidsLimit, (value) => setForm({ ...form, pidsLimit: value }))}
            {field(t('plansPhp'), form.defaultPhpVersion, (value) => setForm({ ...form, defaultPhpVersion: value }), 'text')}
          </div>

          <div style={{ display: 'flex', gap: '8px' }}>
            <button type="submit" className="btn btn-primary btn-sm" disabled={saving}>
              {editing ? t('plansSave') : t('plansCreate')}
            </button>
          </div>
        </form>
      ) : null}

      {loading && plans.length === 0 ? (
        <div className="glass-panel" style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>{t('appsLoading')}</div>
      ) : plans.length === 0 ? (
        <OpsEmpty message={t('plansEmpty')} />
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: '14px' }}>
          {plans.map((plan) => (
            <div key={plan.id} className="glass-card" style={{ padding: '18px', display: 'flex', flexDirection: 'column', gap: '14px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                  <div style={{ width: '34px', height: '34px', borderRadius: '9px', background: 'rgba(234, 88, 12, 0.12)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <ListFilter size={17} style={{ color: 'var(--brand-primary)' }} />
                  </div>
                  <div>
                    <div style={{ fontSize: '0.94rem', fontWeight: 800, textTransform: 'capitalize' }}>{plan.name}</div>
                    <div className="font-mono" style={{ fontSize: '0.68rem', color: 'var(--text-muted)' }}>{plan.id}</div>
                  </div>
                </div>
                <span className="badge-pill" style={{ background: 'rgba(148, 163, 184, 0.12)', color: 'var(--text-muted)' }}>
                  {usage[plan.name] || 0} {t('plansAccounts')}
                </span>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '9px', fontSize: '0.76rem' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <HardDrive size={13} style={{ color: 'var(--brand-accent)' }} />
                  <span>{formatBytes(plan.diskLimitBytes)} {t('plansDisk')}</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <Network size={13} style={{ color: 'var(--brand-accent)' }} />
                  <span>{formatBytes(plan.bandwidthLimitBytes)} {t('plansBandwidth')}</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <Database size={13} style={{ color: 'var(--brand-accent)' }} />
                  <span>{plan.maxDatabases} {t('navDatabases')}</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <Mail size={13} style={{ color: 'var(--brand-accent)' }} />
                  <span>{plan.maxEmailAccounts} {t('navEmails')}</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <Globe size={13} style={{ color: 'var(--brand-accent)' }} />
                  <span>{plan.maxDomains} {t('navDomains')}</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <Cpu size={13} style={{ color: 'var(--brand-accent)' }} />
                  <span>{plan.memoryLimitMb} MB RAM</span>
                </div>
              </div>

              <div style={{ borderTop: '1px solid var(--glass-border)', paddingTop: '10px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '8px', fontSize: '0.72rem', color: 'var(--text-muted)' }}>
                <span>{t('plansInodes')}: {plan.inodesLimit.toLocaleString()} | {t('plansPids')}: {plan.pidsLimit} | PHP {plan.defaultPhpVersion}</span>
                {canManage ? (
                  <div style={{ display: 'flex', gap: '6px' }}>
                    <button type="button" className="btn btn-secondary btn-sm" onClick={() => openEdit(plan)}>
                      <Pencil size={13} />
                      {t('plansEdit')}
                    </button>
                    <button
                      type="button"
                      className="btn btn-secondary btn-sm"
                      onClick={() => void remove(plan)}
                      disabled={(usage[plan.name] || 0) > 0}
                      title={(usage[plan.name] || 0) > 0 ? t('plansDeleteConfirm') : t('plansDelete')}
                    >
                      {(usage[plan.name] || 0) > 0 ? <ShieldAlert size={13} /> : <Trash2 size={13} />}
                      {t('plansDelete')}
                    </button>
                  </div>
                ) : null}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
