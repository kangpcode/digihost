// DigiPanel Application Marketplace (Softaculous-style installer)
// Catalogue cards, verified installation, instance lifecycle, staging clones,
// updates, logs, and wizard hand-off for applications that finish in-browser.
// Zero emojis, strictly Lucide SVG iconography, real API data only.

import React, { useEffect, useMemo, useState } from 'react';
import {
  Boxes,
  Search,
  RefreshCw,
  Download,
  Database,
  ShieldCheck,
  AlertCircle,
  CheckCircle2,
  X,
  ScrollText,
  Copy,
  CopyCheck,
  ArrowUpCircle,
  GitBranch,
  Trash2,
  ServerCog,
  Info,
  ExternalLink
} from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';

interface PackageRequirements {
  binaries?: string[];
  phpExtensions?: string[];
  phpVersion?: string;
  env?: string[];
  minDiskMB?: number;
  minRAMMB?: number;
}

interface CatalogPackage {
  id: string;
  name: string;
  category: string;
  latestVersion: string;
  description: string;
  homepage?: string;
  archiveType: string;
  requiresDB: boolean;
  dbEngine?: string;
  adminPath?: string;
  installMode: string;
  completionMode?: string;
  notes?: string;
  requirements?: PackageRequirements;
}

interface AppInstance {
  id: string;
  packageId: string;
  packageName: string;
  packageVersion: string;
  title: string;
  domain: string;
  domainPath: string;
  siteURL: string;
  path: string;
  dataPath?: string;
  environment: string;
  status: string;
  failureReason?: string;
  dbEngine?: string;
  dbName?: string;
  dbUser?: string;
  dbHost?: string;
  dbPort?: string;
  adminUser?: string;
  adminPath?: string;
  adminEmail?: string;
  port?: number;
  nextStep?: string;
  createdAt: string;
  updatedAt: string;
}

interface DatabaseCredentials {
  engine: string;
  name: string;
  user: string;
  password: string;
  host: string;
  port: string;
}

interface InstallResult {
  instance: AppInstance;
  adminPassword?: string;
  adminPath?: string;
  completionMode?: string;
  database?: DatabaseCredentials;
  nextStep?: string;
}

interface InstallFormState {
  domain: string;
  domainPath: string;
  siteURL: string;
  title: string;
  adminUser: string;
  adminEmail: string;
  adminPassword: string;
  port: string;
  databaseURL: string;
  dbEngine: string;
  dbAdminUser: string;
  dbAdminPass: string;
}

const emptyForm: InstallFormState = {
  domain: '',
  domainPath: '',
  siteURL: '',
  title: '',
  adminUser: 'admin',
  adminEmail: '',
  adminPassword: '',
  port: '',
  databaseURL: '',
  dbEngine: '',
  dbAdminUser: '',
  dbAdminPass: ''
};

const Modal: React.FC<{
  title: string;
  onClose: () => void;
  children: React.ReactNode;
  width?: string;
}> = ({ title, onClose, children, width = '640px' }) => (
  <div
    style={{
      position: 'fixed',
      inset: 0,
      background: 'rgba(15, 23, 42, 0.55)',
      backdropFilter: 'blur(6px)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: '24px',
      zIndex: 200
    }}
    onClick={onClose}
  >
    <div
      className="glass-panel"
      style={{ width: '100%', maxWidth: width, maxHeight: '90vh', overflowY: 'auto', padding: '24px' }}
      onClick={(event) => event.stopPropagation()}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '18px' }}>
        <h2 style={{ fontSize: '1.15rem', fontWeight: 800, color: 'var(--text-main)' }}>{title}</h2>
        <button type="button" className="btn-icon" onClick={onClose} aria-label="Close">
          <X size={18} />
        </button>
      </div>
      {children}
    </div>
  </div>
);

const Field: React.FC<{
  label: string;
  hint?: string;
  required?: boolean;
  children: React.ReactNode;
}> = ({ label, hint, required, children }) => (
  <label style={{ display: 'flex', flexDirection: 'column', gap: '5px', fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-muted)' }}>
    <span>
      {label}
      {required ? <span style={{ color: 'var(--brand-primary)' }}> *</span> : null}
    </span>
    {children}
    {hint ? <span style={{ fontSize: '0.72rem', fontWeight: 500, color: 'var(--text-dim)' }}>{hint}</span> : null}
  </label>
);

const completionLabel = (mode?: string): string => {
  if (mode === 'wizard') return 'Browser finish';
  return 'Automatic';
};

export const SoftInstallerView: React.FC<{ language: Language }> = ({ language }) => {
  const t = (key: any) => getTranslation(language, key);

  const [tab, setTab] = useState<'catalog' | 'instances'>('catalog');
  const [packages, setPackages] = useState<CatalogPackage[]>([]);
  const [instances, setInstances] = useState<AppInstance[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [search, setSearch] = useState('');
  const [category, setCategory] = useState('all');

  const [selected, setSelected] = useState<CatalogPackage | null>(null);
  const [form, setForm] = useState<InstallFormState>(emptyForm);
  const [formError, setFormError] = useState('');
  const [installing, setInstalling] = useState(false);
  const [result, setResult] = useState<InstallResult | null>(null);
  const [copied, setCopied] = useState('');

  const [logsFor, setLogsFor] = useState<AppInstance | null>(null);
  const [logLines, setLogLines] = useState<string[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);

  const [cloneSource, setCloneSource] = useState<AppInstance | null>(null);
  const [cloneForm, setCloneForm] = useState({ domain: '', siteURL: '', domainPath: '', targetPath: '', staging: false });
  const [cloneBusy, setCloneBusy] = useState(false);
  const [cloneError, setCloneError] = useState('');

  const [updateTarget, setUpdateTarget] = useState<AppInstance | null>(null);
  const [actionBusy, setActionBusy] = useState('');
  const [actionError, setActionError] = useState('');

  const [notice, setNotice] = useState('');

  const loadAll = async () => {
    try {
      setLoading(true);
      setLoadError('');
      const [catalog, list] = await Promise.all([
        apiFetch<CatalogPackage[]>('/api/v1/apps/catalog'),
        apiFetch<AppInstance[]>('/api/v1/apps/instances')
      ]);
      setPackages(Array.isArray(catalog) ? catalog : []);
      setInstances(Array.isArray(list) ? list : []);
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : String(error));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadAll();
  }, []);

  const categories = useMemo(() => {
    const unique = new Set(packages.map((pkg) => pkg.category).filter(Boolean));
    return ['all', ...Array.from(unique).sort()];
  }, [packages]);

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return packages.filter((pkg) => {
      if (category !== 'all' && pkg.category !== category) return false;
      if (!needle) return true;
      return (
        pkg.name.toLowerCase().includes(needle) ||
        pkg.description.toLowerCase().includes(needle) ||
        pkg.category.toLowerCase().includes(needle)
      );
    });
  }, [packages, category, search]);

  const copyValue = async (label: string, value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(label);
      window.setTimeout(() => setCopied(''), 2000);
    } catch {
      setCopied('');
    }
  };

  const openInstall = (pkg: CatalogPackage) => {
    setSelected(pkg);
    setForm({ ...emptyForm, dbEngine: pkg.dbEngine || '' });
    setFormError('');
    setResult(null);
  };

  const updateForm = (key: keyof InstallFormState, value: string) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const submitInstall = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selected) return;
    setFormError('');
    if (!form.domain.trim()) {
      setFormError('A domain is required for this installation.');
      return;
    }
    const payload: Record<string, unknown> = {
      packageId: selected.id,
      domain: form.domain.trim(),
      domainPath: form.domainPath.trim(),
      siteURL: form.siteURL.trim(),
      title: form.title.trim(),
      adminUser: form.adminUser.trim(),
      adminEmail: form.adminEmail.trim(),
      adminPassword: form.adminPassword,
      databaseURL: form.databaseURL.trim(),
      dbEngine: form.dbEngine,
      dbAdminUser: form.dbAdminUser.trim(),
      dbAdminPass: form.dbAdminPass
    };
    const port = Number.parseInt(form.port, 10);
    if (!Number.isNaN(port) && port > 0) payload.port = port;
    try {
      setInstalling(true);
      const response = await apiFetch<InstallResult>('/api/v1/apps/install', {
        method: 'POST',
        body: JSON.stringify(payload)
      });
      setResult(response);
      setInstances((prev) => [response.instance, ...prev]);
      setNotice(`${response.instance.packageName} installed at ${response.instance.siteURL}`);
    } catch (error) {
      setFormError(error instanceof Error ? error.message : String(error));
    } finally {
      setInstalling(false);
    }
  };

  const openLogs = async (inst: AppInstance) => {
    setLogsFor(inst);
    setLogLines([]);
    setLogsLoading(true);
    try {
      const data = await apiFetch<{ id: string; lines: string[] }>(
        `/api/v1/apps/logs?id=${encodeURIComponent(inst.id)}`
      );
      setLogLines(Array.isArray(data.lines) ? data.lines : []);
    } catch (error) {
      setLogLines([`Could not read logs: ${error instanceof Error ? error.message : String(error)}`]);
    } finally {
      setLogsLoading(false);
    }
  };

  const openClone = (inst: AppInstance) => {
    setCloneSource(inst);
    setCloneError('');
    setCloneForm({ domain: '', siteURL: '', domainPath: '', targetPath: '', staging: false });
  };

  const submitClone = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!cloneSource) return;
    if (!cloneForm.domain.trim()) {
      setCloneError('A domain is required for the clone.');
      return;
    }
    try {
      setCloneBusy(true);
      setCloneError('');
      const clone = await apiFetch<AppInstance>('/api/v1/apps/clone', {
        method: 'POST',
        body: JSON.stringify({
          id: cloneSource.id,
          domain: cloneForm.domain.trim(),
          siteURL: cloneForm.siteURL.trim(),
          domainPath: cloneForm.domainPath.trim(),
          targetPath: cloneForm.targetPath.trim(),
          staging: cloneForm.staging
        })
      });
      setInstances((prev) => [clone, ...prev]);
      setCloneSource(null);
      setNotice(
        cloneForm.staging
          ? `Staging copy created at ${clone.siteURL}`
          : `Clone created at ${clone.siteURL}`
      );
    } catch (error) {
      setCloneError(error instanceof Error ? error.message : String(error));
    } finally {
      setCloneBusy(false);
    }
  };

  const submitUpdate = async () => {
    if (!updateTarget) return;
    try {
      setActionBusy(updateTarget.id);
      setActionError('');
      const updated = await apiFetch<AppInstance>('/api/v1/apps/update', {
        method: 'POST',
        body: JSON.stringify({ id: updateTarget.id })
      });
      setInstances((prev) => prev.map((item) => (item.id === updated.id ? updated : item)));
      setUpdateTarget(null);
      setNotice(`${updated.packageName} updated to ${updated.packageVersion}`);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : String(error));
    } finally {
      setActionBusy('');
    }
  };

  const submitUninstall = async (inst: AppInstance) => {
    try {
      setActionBusy(inst.id);
      setActionError('');
      await apiFetch<{ deleted: boolean }>(`/api/v1/apps/instances?id=${encodeURIComponent(inst.id)}`, {
        method: 'DELETE'
      });
      setInstances((prev) => prev.filter((item) => item.id !== inst.id));
      setNotice(`${inst.packageName} removed from ${inst.siteURL}`);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : String(error));
    } finally {
      setActionBusy('');
    }
  };

  const confirmUninstall = (inst: AppInstance) => {
    const message = `Remove ${inst.packageName} at ${inst.siteURL}? Files and the provisioned database are deleted. This cannot be undone.`;
    if (window.confirm(message)) {
      void submitUninstall(inst);
    }
  };

  const tabButton = (id: 'catalog' | 'instances', label: string, count: number) => {
    const active = tab === id;
    return (
      <button
        type="button"
        onClick={() => setTab(id)}
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: '8px',
          padding: '8px 16px',
          borderRadius: '8px',
          border: '1px solid ' + (active ? 'rgba(234, 88, 12, 0.35)' : 'var(--glass-border)'),
          background: active ? 'rgba(234, 88, 12, 0.12)' : 'var(--glass-surface)',
          color: active ? 'var(--brand-primary)' : 'var(--text-main)',
          fontWeight: 700,
          fontSize: '0.82rem',
          cursor: 'pointer'
        }}
      >
        {label}
        <span className="badge-pill" style={{ background: 'rgba(148, 163, 184, 0.16)', color: 'var(--text-muted)' }}>
          {count}
        </span>
      </button>
    );
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '22px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '16px' }}>
        <div>
          <h1 style={{ fontSize: '1.6rem', fontWeight: 800, color: 'var(--text-main)', letterSpacing: '-0.02em', marginBottom: '4px' }}>
            {t('appsPageTitle')}
          </h1>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', maxWidth: '760px' }}>
            {t('appsSubtitle')}
          </p>
        </div>
        <div style={{ display: 'flex', gap: '8px' }}>
          {tabButton('catalog', t('appsCatalogue'), packages.length)}
          {tabButton('instances', t('appsInstances'), instances.length)}
          <button
            type="button"
            className="btn btn-secondary btn-sm"
            onClick={() => void loadAll()}
            disabled={loading}
          >
            <RefreshCw size={14} className={loading ? 'spin' : undefined} />
            {t('refresh')}
          </button>
        </div>
      </div>

      {notice ? (
        <div className="glass-card" style={{ padding: '12px 16px', display: 'flex', alignItems: 'center', gap: '10px', borderColor: 'rgba(16, 185, 129, 0.35)' }}>
          <CheckCircle2 size={16} style={{ color: 'var(--status-online)' }} />
          <span style={{ fontSize: '0.82rem', flex: 1 }}>{notice}</span>
          <button type="button" className="btn-icon" onClick={() => setNotice('')} aria-label="Dismiss">
            <X size={14} />
          </button>
        </div>
      ) : null}

      {actionError ? (
        <div className="glass-card" style={{ padding: '12px 16px', display: 'flex', alignItems: 'center', gap: '10px', borderColor: 'rgba(239, 68, 68, 0.35)' }}>
          <AlertCircle size={16} style={{ color: '#ef4444' }} />
          <span style={{ fontSize: '0.82rem', flex: 1 }}>{actionError}</span>
          <button type="button" className="btn-icon" onClick={() => setActionError('')} aria-label="Dismiss">
            <X size={14} />
          </button>
        </div>
      ) : null}

      {loadError ? (
        <div className="glass-panel" style={{ padding: '20px', borderColor: 'rgba(239, 68, 68, 0.35)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '6px' }}>
            <AlertCircle size={18} style={{ color: '#ef4444' }} />
            <strong style={{ fontSize: '0.92rem' }}>{t('appsUnavailable')}</strong>
          </div>
          <p style={{ fontSize: '0.82rem', color: 'var(--text-muted)' }}>{loadError}</p>
          <p style={{ fontSize: '0.78rem', color: 'var(--text-dim)', marginTop: '8px' }}>
            {t('appsUnavailableHint')}
          </p>
        </div>
      ) : null}

      {!loadError && tab === 'catalog' && (
        <>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', alignItems: 'center' }}>
            <div style={{ position: 'relative', flex: '1 1 280px', maxWidth: '380px' }}>
              <Search size={15} style={{ position: 'absolute', left: '12px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
              <input
                type="text"
                className="input-glass"
                style={{ paddingLeft: '36px', height: '38px' }}
                placeholder={t('appsSearchPlaceholder')}
                value={search}
                onChange={(event) => setSearch(event.target.value)}
              />
            </div>
            <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
              {categories.map((item) => {
                const active = category === item;
                return (
                  <button
                    key={item}
                    type="button"
                    onClick={() => setCategory(item)}
                    className="btn btn-sm"
                    style={{
                      background: active ? 'rgba(234, 88, 12, 0.12)' : 'var(--glass-surface)',
                      border: '1px solid ' + (active ? 'rgba(234, 88, 12, 0.35)' : 'var(--glass-border)'),
                      color: active ? 'var(--brand-primary)' : 'var(--text-main)'
                    }}
                  >
                    {item === 'all' ? t('appsAllCategories') : item}
                  </button>
                );
              })}
            </div>
          </div>

          {loading && packages.length === 0 ? (
            <div className="glass-panel" style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>
              {t('appsLoading')}
            </div>
          ) : filtered.length === 0 ? (
            <div className="glass-panel" style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>
              {t('appsNoMatches')}
            </div>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(290px, 1fr))', gap: '14px' }}>
              {filtered.map((pkg) => (
                <div key={pkg.id} className="glass-card" style={{ padding: '18px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: '8px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px', minWidth: 0 }}>
                      <div style={{ width: '36px', height: '36px', borderRadius: '9px', background: 'rgba(234, 88, 12, 0.12)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                        <Boxes size={18} style={{ color: 'var(--brand-primary)' }} />
                      </div>
                      <div style={{ minWidth: 0 }}>
                        <div style={{ fontSize: '0.92rem', fontWeight: 800, color: 'var(--text-main)' }}>{pkg.name}</div>
                        <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>
                          {pkg.category} &middot; v{pkg.latestVersion}
                        </div>
                      </div>
                    </div>
                    {pkg.completionMode === 'wizard' ? (
                      <span className="badge-warning">{completionLabel(pkg.completionMode)}</span>
                    ) : (
                      <span className="badge-online">{completionLabel(pkg.completionMode)}</span>
                    )}
                  </div>

                  <p style={{ fontSize: '0.79rem', color: 'var(--text-muted)', flex: 1, lineHeight: 1.5 }}>{pkg.description}</p>

                  <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap', fontSize: '0.7rem', color: 'var(--text-muted)' }}>
                    <span className="badge-pill" style={{ background: 'rgba(148, 163, 184, 0.12)', display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                      <Database size={11} />
                      {pkg.requiresDB ? (pkg.dbEngine || 'database') : t('appsNoDatabase')}
                    </span>
                    <span className="badge-pill" style={{ background: 'rgba(148, 163, 184, 0.12)', display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                      <ShieldCheck size={11} />
                      {pkg.archiveType}
                    </span>
                    {pkg.requirements?.phpVersion ? (
                      <span className="badge-pill" style={{ background: 'rgba(148, 163, 184, 0.12)' }}>
                        PHP {pkg.requirements.phpVersion}
                      </span>
                    ) : null}
                  </div>

                  {pkg.notes ? (
                    <div style={{ display: 'flex', gap: '6px', fontSize: '0.72rem', color: 'var(--text-dim)', lineHeight: 1.45 }}>
                      <Info size={12} style={{ flexShrink: 0, marginTop: '2px' }} />
                      <span>{pkg.notes}</span>
                    </div>
                  ) : null}

                  <div style={{ display: 'flex', gap: '8px' }}>
                    <button type="button" className="btn btn-primary btn-sm" style={{ flex: 1 }} onClick={() => openInstall(pkg)}>
                      <Download size={14} />
                      {t('appsInstall')}
                    </button>
                    {pkg.homepage ? (
                      <a
                        href={pkg.homepage}
                        target="_blank"
                        rel="noreferrer"
                        className="btn btn-secondary btn-sm"
                        aria-label={`${pkg.name} homepage`}
                      >
                        <ExternalLink size={14} />
                      </a>
                    ) : null}
                  </div>
                </div>
              ))}
            </div>
          )}
        </>
      )}

      {!loadError && tab === 'instances' && (
        <div className="glass-panel" style={{ padding: '0', overflowX: 'auto' }}>
          {instances.length === 0 ? (
            <div style={{ padding: '48px', textAlign: 'center' }}>
              <ServerCog size={30} style={{ color: 'var(--text-dim)', marginBottom: '10px' }} />
              <p style={{ fontSize: '0.9rem', color: 'var(--text-muted)' }}>{t('appsNoInstances')}</p>
              <button type="button" className="btn btn-primary btn-sm" style={{ marginTop: '14px' }} onClick={() => setTab('catalog')}>
                {t('appsBrowseCatalogue')}
              </button>
            </div>
          ) : (
            <table className="glass-table">
              <thead>
                <tr>
                  <th>{t('appsApplication')}</th>
                  <th>{t('appsSite')}</th>
                  <th>{t('appsStatus')}</th>
                  <th>{t('appsDatabase')}</th>
                  <th>{t('appsEnvironment')}</th>
                  <th style={{ textAlign: 'right' }}>{t('appsActions')}</th>
                </tr>
              </thead>
              <tbody>
                {instances.map((inst) => {
                  const busy = actionBusy === inst.id;
                  return (
                    <tr key={inst.id}>
                      <td>
                        <div style={{ fontWeight: 700, fontSize: '0.84rem' }}>{inst.packageName}</div>
                        <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>
                          v{inst.packageVersion} &middot; {inst.title}
                        </div>
                        {inst.failureReason ? (
                          <div style={{ fontSize: '0.72rem', color: '#ef4444', marginTop: '3px', maxWidth: '320px' }}>
                            {inst.failureReason}
                          </div>
                        ) : null}
                      </td>
                      <td>
                        <a
                          href={inst.siteURL}
                          target="_blank"
                          rel="noreferrer"
                          style={{ fontSize: '0.8rem', color: 'var(--brand-primary)', textDecoration: 'none', fontWeight: 600 }}
                        >
                          {inst.domain || inst.siteURL}
                        </a>
                        <div className="font-mono" style={{ fontSize: '0.68rem', color: 'var(--text-dim)', maxWidth: '320px', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                          {inst.path}
                        </div>
                      </td>
                      <td>
                        <span className={inst.status === 'installed' ? 'badge-online' : inst.status === 'failed' ? 'badge-warning' : 'badge-pill'} style={inst.status === 'failed' ? { background: 'rgba(239, 68, 68, 0.12)', color: '#ef4444' } : undefined}>
                          {inst.status}
                        </span>
                      </td>
                      <td style={{ fontSize: '0.76rem', color: 'var(--text-muted)' }}>
                        {inst.dbName ? (
                          <>
                            <div className="font-mono">{inst.dbName}</div>
                            <div style={{ fontSize: '0.68rem' }}>{inst.dbEngine} {inst.dbHost ? `@ ${inst.dbHost}:${inst.dbPort}` : ''}</div>
                          </>
                        ) : (
                          'not provisioned'
                        )}
                      </td>
                      <td>
                        <span className="badge-pill" style={{ background: inst.environment === 'staging' ? 'rgba(245, 158, 11, 0.14)' : 'rgba(148, 163, 184, 0.12)', color: inst.environment === 'staging' ? 'var(--status-warning)' : 'var(--text-muted)' }}>
                          {inst.environment}
                        </span>
                      </td>
                      <td>
                        <div style={{ display: 'flex', gap: '4px', justifyContent: 'flex-end', flexWrap: 'wrap' }}>
                          <button type="button" className="btn btn-secondary btn-sm" onClick={() => void openLogs(inst)} title={t('appsLogs')}>
                            <ScrollText size={13} />
                          </button>
                          <button type="button" className="btn btn-secondary btn-sm" onClick={() => setUpdateTarget(inst)} disabled={busy} title={t('appsUpdate')}>
                            <ArrowUpCircle size={13} />
                          </button>
                          <button type="button" className="btn btn-secondary btn-sm" onClick={() => openClone(inst)} disabled={busy} title={t('appsClone')}>
                            <GitBranch size={13} />
                          </button>
                          <button type="button" className="btn btn-secondary btn-sm" onClick={() => confirmUninstall(inst)} disabled={busy} title={t('appsUninstall')}>
                            <Trash2 size={13} style={{ color: '#ef4444' }} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>
      )}

      {selected && (
        <Modal
          title={result ? `${result.instance.packageName} installed` : `${t('appsInstall')} ${selected.name} ${selected.latestVersion}`}
          onClose={() => {
            setSelected(null);
            setResult(null);
          }}
          width="720px"
        >
          {result ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', color: 'var(--status-online)' }}>
                <CheckCircle2 size={18} />
                <strong style={{ fontSize: '0.92rem' }}>
                  {result.instance.packageName} is available at {result.instance.siteURL}
                </strong>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '10px' }}>
                <div className="glass-card" style={{ padding: '12px' }}>
                  <div style={{ fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-muted)', marginBottom: '4px' }}>
                    {t('appsFiles')}
                  </div>
                  <div className="font-mono" style={{ fontSize: '0.74rem', wordBreak: 'break-all' }}>{result.instance.path}</div>
                </div>
                <div className="glass-card" style={{ padding: '12px' }}>
                  <div style={{ fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-muted)', marginBottom: '4px' }}>
                    {t('appsAdminAccount')}
                  </div>
                  <div style={{ fontSize: '0.78rem' }}>{result.instance.adminUser || 'admin'}</div>
                  {result.adminPassword ? (
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginTop: '2px' }}>
                      <code className="font-mono" style={{ fontSize: '0.74rem', wordBreak: 'break-all' }}>{result.adminPassword}</code>
                      <button type="button" className="btn-icon" onClick={() => void copyValue('admin', result.adminPassword || '')} aria-label="Copy password">
                        {copied === 'admin' ? <CopyCheck size={13} /> : <Copy size={13} />}
                      </button>
                    </div>
                  ) : null}
                </div>
                {result.database ? (
                  <div className="glass-card" style={{ padding: '12px' }}>
                    <div style={{ fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-muted)', marginBottom: '4px' }}>
                      {t('appsWizardDatabase')}
                    </div>
                    <div className="font-mono" style={{ fontSize: '0.72rem', lineHeight: 1.6 }}>
                      <div>name: {result.database.name}</div>
                      <div>user: {result.database.user}</div>
                      <div>host: {result.database.host}:{result.database.port}</div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                        <span>password: {result.database.password}</span>
                        <button type="button" className="btn-icon" onClick={() => void copyValue('db', result.database?.password || '')} aria-label="Copy database password">
                          {copied === 'db' ? <CopyCheck size={13} /> : <Copy size={13} />}
                        </button>
                      </div>
                    </div>
                  </div>
                ) : null}
              </div>

              {result.nextStep ? (
                <div style={{ display: 'flex', gap: '8px', padding: '12px', borderRadius: '8px', background: 'rgba(245, 158, 11, 0.1)', border: '1px solid rgba(245, 158, 11, 0.25)' }}>
                  <Info size={15} style={{ color: 'var(--status-warning)', flexShrink: 0, marginTop: '1px' }} />
                  <div style={{ fontSize: '0.8rem', lineHeight: 1.5 }}>
                    <strong>{t('appsNextStep')}: </strong>
                    {result.nextStep}
                    {result.adminPath ? (
                      <a
                        href={result.instance.siteURL + result.adminPath}
                        target="_blank"
                        rel="noreferrer"
                        style={{ marginLeft: '6px', color: 'var(--brand-primary)', fontWeight: 700 }}
                      >
                        {t('appsOpenInstaller')}
                      </a>
                    ) : null}
                  </div>
                </div>
              ) : null}

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
                <button type="button" className="btn btn-secondary" onClick={() => setTab('instances')}>
                  {t('appsViewInstances')}
                </button>
                <button type="button" className="btn btn-primary" onClick={() => setSelected(null)}>
                  {t('close')}
                </button>
              </div>
            </div>
          ) : (
            <form onSubmit={submitInstall} style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '12px' }}>
                <Field label={t('appsDomain')} required hint={t('appsDomainHint')}>
                  <input className="input-glass" value={form.domain} onChange={(event) => updateForm('domain', event.target.value)} placeholder="example.com" autoFocus />
                </Field>
                <Field label={t('appsDomainPath')} hint={t('appsDomainPathHint')}>
                  <input className="input-glass" value={form.domainPath} onChange={(event) => updateForm('domainPath', event.target.value)} placeholder="blog" />
                </Field>
                <Field label={t('appsTitle')}>
                  <input className="input-glass" value={form.title} onChange={(event) => updateForm('title', event.target.value)} placeholder={selected.name} />
                </Field>
                <Field label={t('appsAdminUser')}>
                  <input className="input-glass" value={form.adminUser} onChange={(event) => updateForm('adminUser', event.target.value)} placeholder="admin" />
                </Field>
                <Field label={t('appsAdminEmail')} hint={t('appsAdminEmailHint')}>
                  <input className="input-glass" value={form.adminEmail} onChange={(event) => updateForm('adminEmail', event.target.value)} placeholder="admin@example.com" />
                </Field>
                <Field label={t('appsAdminPassword')} hint={t('appsAdminPasswordHint')}>
                  <input className="input-glass" type="password" autoComplete="new-password" value={form.adminPassword} onChange={(event) => updateForm('adminPassword', event.target.value)} placeholder={t('appsGeneratedWhenBlank')} />
                </Field>
                {selected.installMode === 'node' || selected.installMode === 'python' ? (
                  <Field label={t('appsPort')} hint={t('appsPortHint')}>
                    <input className="input-glass" inputMode="numeric" value={form.port} onChange={(event) => updateForm('port', event.target.value)} placeholder="8080" />
                  </Field>
                ) : null}
              </div>

              {selected.requiresDB ? (
                <div style={{ borderTop: '1px solid var(--glass-border)', paddingTop: '12px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.82rem', fontWeight: 700 }}>
                    <Database size={15} style={{ color: 'var(--brand-primary)' }} />
                    {t('appsDatabaseConnection')}
                  </div>
                  <p style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>
                    {t('appsDatabaseDefaultsHint')}
                  </p>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '12px' }}>
                    <Field label={t('appsEngine')}>
                      <input className="input-glass" value={selected.dbEngine || t('appsServerDefault')} readOnly />
                    </Field>
                    <Field label={t('appsDatabaseURL')} hint={t('appsDatabaseURLHint')}>
                      <input className="input-glass" value={form.databaseURL} onChange={(event) => updateForm('databaseURL', event.target.value)} placeholder="127.0.0.1:3306" />
                    </Field>
                    <Field label={t('appsAdminUser')}>
                      <input className="input-glass" value={form.dbAdminUser} onChange={(event) => updateForm('dbAdminUser', event.target.value)} placeholder={t('appsServerDefault')} />
                    </Field>
                    <Field label={t('appsAdminPassword')}>
                      <input className="input-glass" type="password" autoComplete="new-password" value={form.dbAdminPass} onChange={(event) => updateForm('dbAdminPass', event.target.value)} placeholder={t('appsServerDefault')} />
                    </Field>
                  </div>
                </div>
              ) : null}

              {selected.completionMode === 'wizard' ? (
                <div style={{ display: 'flex', gap: '8px', padding: '10px 12px', borderRadius: '8px', background: 'rgba(245, 158, 11, 0.1)', border: '1px solid rgba(245, 158, 11, 0.25)' }}>
                  <Info size={15} style={{ color: 'var(--status-warning)', flexShrink: 0, marginTop: '1px' }} />
                  <span style={{ fontSize: '0.78rem', lineHeight: 1.5 }}>
                    {t('appsWizardNotice')}
                  </span>
                </div>
              ) : null}

              {formError ? (
                <div style={{ display: 'flex', gap: '8px', padding: '10px 12px', borderRadius: '8px', background: 'rgba(239, 68, 68, 0.1)', border: '1px solid rgba(239, 68, 68, 0.3)' }}>
                  <AlertCircle size={15} style={{ color: '#ef4444', flexShrink: 0, marginTop: '1px' }} />
                  <span style={{ fontSize: '0.8rem', lineHeight: 1.5 }}>{formError}</span>
                </div>
              ) : null}

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
                <button type="button" className="btn btn-secondary" onClick={() => setSelected(null)} disabled={installing}>
                  {t('cancel')}
                </button>
                <button type="submit" className="btn btn-primary" disabled={installing}>
                  <Download size={15} />
                  {installing ? t('appsInstalling') : t('appsInstall')}
                </button>
              </div>
            </form>
          )}
        </Modal>
      )}

      {logsFor && (
        <Modal title={`${logsFor.packageName} ${t('appsLogs')}`} onClose={() => setLogsFor(null)} width="760px">
          <div className="font-mono" style={{ background: 'rgba(15, 23, 42, 0.65)', color: '#e2e8f0', borderRadius: '8px', padding: '14px', fontSize: '0.74rem', lineHeight: 1.65, maxHeight: '440px', overflow: 'auto', whiteSpace: 'pre-wrap' }}>
            {logsLoading ? t('appsLoading') : logLines.length === 0 ? t('appsNoLogs') : logLines.join('\n')}
          </div>
        </Modal>
      )}

      {cloneSource && (
        <Modal title={`${t('appsClone')} ${cloneSource.packageName}`} onClose={() => setCloneSource(null)}>
          <form onSubmit={submitClone} style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
              {t('appsCloneHint')}
            </p>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '12px' }}>
              <Field label={t('appsDomain')} required>
                <input className="input-glass" value={cloneForm.domain} onChange={(event) => setCloneForm((prev) => ({ ...prev, domain: event.target.value }))} placeholder="staging.example.com" autoFocus />
              </Field>
              <Field label={t('appsDomainPath')}>
                <input className="input-glass" value={cloneForm.domainPath} onChange={(event) => setCloneForm((prev) => ({ ...prev, domainPath: event.target.value }))} placeholder="blog" />
              </Field>
              <Field label={t('appsSiteURL')} hint={t('appsSiteURLHint')}>
                <input className="input-glass" value={cloneForm.siteURL} onChange={(event) => setCloneForm((prev) => ({ ...prev, siteURL: event.target.value }))} placeholder="https://staging.example.com" />
              </Field>
              <Field label={t('appsTargetPath')} hint={t('appsTargetPathHint')}>
                <input className="input-glass" value={cloneForm.targetPath} onChange={(event) => setCloneForm((prev) => ({ ...prev, targetPath: event.target.value }))} placeholder="/storage/sites/clone" />
              </Field>
            </div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.82rem', color: 'var(--text-main)' }}>
              <input type="checkbox" checked={cloneForm.staging} onChange={(event) => setCloneForm((prev) => ({ ...prev, staging: event.target.checked }))} />
              {t('appsCreateAsStaging')}
            </label>
            {cloneError ? (
              <div style={{ display: 'flex', gap: '8px', padding: '10px 12px', borderRadius: '8px', background: 'rgba(239, 68, 68, 0.1)', border: '1px solid rgba(239, 68, 68, 0.3)' }}>
                <AlertCircle size={15} style={{ color: '#ef4444', flexShrink: 0, marginTop: '1px' }} />
                <span style={{ fontSize: '0.8rem', lineHeight: 1.5 }}>{cloneError}</span>
              </div>
            ) : null}
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
              <button type="button" className="btn btn-secondary" onClick={() => setCloneSource(null)} disabled={cloneBusy}>
                {t('cancel')}
              </button>
              <button type="submit" className="btn btn-primary" disabled={cloneBusy}>
                <GitBranch size={15} />
                {cloneBusy ? t('appsCloning') : t('appsClone')}
              </button>
            </div>
          </form>
        </Modal>
      )}

      {updateTarget && (
        <Modal title={`${t('appsUpdate')} ${updateTarget.packageName}`} onClose={() => setUpdateTarget(null)}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            <p style={{ fontSize: '0.82rem', color: 'var(--text-muted)', lineHeight: 1.6 }}>
              {t('appsUpdateHint')}
            </p>
            <div className="glass-card" style={{ padding: '12px' }}>
              <div style={{ fontSize: '0.8rem', fontWeight: 700 }}>{updateTarget.packageName}</div>
              <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>
                {updateTarget.siteURL} &middot; {t('appsInstalledVersion')} {updateTarget.packageVersion}
              </div>
            </div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
              <button type="button" className="btn btn-secondary" onClick={() => setUpdateTarget(null)} disabled={actionBusy === updateTarget.id}>
                {t('cancel')}
              </button>
              <button type="button" className="btn btn-primary" onClick={() => void submitUpdate()} disabled={actionBusy === updateTarget.id}>
                <ArrowUpCircle size={15} />
                {actionBusy === updateTarget.id ? t('appsUpdating') : t('appsUpdate')}
              </button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
};
