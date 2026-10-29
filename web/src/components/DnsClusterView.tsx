// DigiPanel DNS Cluster view.
// Reads authoritative zones from the DNS manager, inspects real zone health,
// creates zones, and toggles DNSSEC using the core API.

import React, { useEffect, useMemo, useState } from 'react';
import { RefreshCw, Search, Globe, ShieldCheck, ShieldOff, Plus, Activity } from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';
import { OpsEmpty, OpsError, OpsHeader, StatusPill, formatDate } from './opsShared';

interface DnsRecord {
  id: string;
  zone: string;
  name: string;
  type: string;
  value: string;
  ttl: number;
  priority?: number;
}

interface DnssecKeys {
  enabled: boolean;
  keyTag: number;
  algorithm: number;
  digest: string;
  dsRecord: string;
}

interface Zone {
  name: string;
  tenantId: string;
  serial: number;
  primaryNs: string;
  adminEmail: string;
  records: DnsRecord[] | null;
  dnssec: DnssecKeys;
  createdAt: string;
  updatedAt: string;
}

interface ZoneHealth {
  zoneName: string;
  healthy: boolean;
  hasSoa: boolean;
  hasApexA: boolean;
  nsCount: number;
  hasRedundantNs: boolean;
  hasMx: boolean;
  issues: string[] | null;
}

const emptyCreate = {
  name: '',
  tenantId: 'tenant-default',
  primaryNs: 'ns1.digipanel.id.',
  adminEmail: '',
  serverIp: ''
};

export const DnsClusterView: React.FC<{ language: Language }> = ({ language }) => {
  const t = (key: any) => getTranslation(language, key);
  const [zones, setZones] = useState<Zone[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState('');
  const [health, setHealth] = useState<ZoneHealth | null>(null);
  const [healthLoading, setHealthLoading] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState(emptyCreate);
  const [creating, setCreating] = useState(false);

  const load = async () => {
    try {
      setLoading(true);
      setError('');
      const data = await apiFetch<Zone[]>('/api/v1/dns/zones');
      setZones(Array.isArray(data) ? data : []);
      if (!selected && data.length > 0) setSelected(data[0].name);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const inspect = async (zoneName: string) => {
    setSelected(zoneName);
    setHealthLoading(true);
    setHealth(null);
    try {
      const data = await apiFetch<ZoneHealth>(`/api/v1/dns/health?zone=${encodeURIComponent(zoneName)}`);
      setHealth(data);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setHealthLoading(false);
    }
  };

  const toggleDnssec = async (zone: Zone) => {
    try {
      setError('');
      await apiFetch(`/api/v1/dns/dnssec/toggle`, {
        method: 'POST',
        body: JSON.stringify({ zone: zone.name, enable: !zone.dnssec?.enabled })
      });
      setNotice(`${zone.name}: DNSSEC ${zone.dnssec?.enabled ? 'disabled' : 'enabled'}`);
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  const createZone = async (event: React.FormEvent) => {
    event.preventDefault();
    try {
      setCreating(true);
      setError('');
      await apiFetch<Zone>('/api/v1/dns/zones', {
        method: 'POST',
        body: JSON.stringify(createForm)
      });
      setNotice(`${createForm.name}: zone created`);
      setShowCreate(false);
      setCreateForm(emptyCreate);
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setCreating(false);
    }
  };

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    if (!needle) return zones;
    return zones.filter((zone) => zone.name.toLowerCase().includes(needle) || zone.tenantId.toLowerCase().includes(needle));
  }, [zones, search]);

  const selectedZone = zones.find((zone) => zone.name === selected) || null;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <OpsHeader
        title={t('dnsClusterTitle')}
        subtitle={t('dnsClusterSubtitle')}
        action={
          <div style={{ display: 'flex', gap: '8px' }}>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => void load()} disabled={loading}>
              <RefreshCw size={14} />
              {t('refresh')}
            </button>
            <button type="button" className="btn btn-primary btn-sm" onClick={() => setShowCreate((value) => !value)}>
              <Plus size={14} />
              {t('dnsClusterAddZone')}
            </button>
          </div>
        }
      />

      {notice ? (
        <div className="glass-card" style={{ padding: '10px 14px', fontSize: '0.82rem', borderColor: 'rgba(16, 185, 129, 0.35)' }}>{notice}</div>
      ) : null}
      {error ? <OpsError message={error} onDismiss={() => setError('')} /> : null}

      {showCreate ? (
        <form onSubmit={createZone} className="glass-panel" style={{ padding: '18px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <div style={{ fontSize: '0.88rem', fontWeight: 700 }}>{t('dnsClusterAddZone')}</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '12px' }}>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.76rem', color: 'var(--text-muted)' }}>
              {t('appsDomain')}
              <input className="input-glass" required value={createForm.name} onChange={(event) => setCreateForm((prev) => ({ ...prev, name: event.target.value }))} placeholder="example.com" />
            </label>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.76rem', color: 'var(--text-muted)' }}>
              Tenant
              <input className="input-glass" value={createForm.tenantId} onChange={(event) => setCreateForm((prev) => ({ ...prev, tenantId: event.target.value }))} />
            </label>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.76rem', color: 'var(--text-muted)' }}>
              {t('dnsClusterPrimaryNs')}
              <input className="input-glass" value={createForm.primaryNs} onChange={(event) => setCreateForm((prev) => ({ ...prev, primaryNs: event.target.value }))} />
            </label>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.76rem', color: 'var(--text-muted)' }}>
              {t('dnsClusterAdminEmail')}
              <input className="input-glass" type="email" value={createForm.adminEmail} onChange={(event) => setCreateForm((prev) => ({ ...prev, adminEmail: event.target.value }))} placeholder="admin@example.com" />
            </label>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.76rem', color: 'var(--text-muted)' }}>
              {t('dnsClusterServerIp')}
              <input className="input-glass" value={createForm.serverIp} onChange={(event) => setCreateForm((prev) => ({ ...prev, serverIp: event.target.value }))} placeholder="203.0.113.10" />
            </label>
          </div>
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => setShowCreate(false)} disabled={creating}>{t('cancel')}</button>
            <button type="submit" className="btn btn-primary btn-sm" disabled={creating}>{t('create')}</button>
          </div>
        </form>
      ) : null}

      <div style={{ position: 'relative', maxWidth: '400px' }}>
        <Search size={15} style={{ position: 'absolute', left: '12px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
        <input
          className="input-glass"
          style={{ paddingLeft: '36px', height: '38px' }}
          placeholder={t('dnsClusterSearchPlaceholder')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>

      {loading && zones.length === 0 ? (
        <div className="glass-panel" style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>{t('appsLoading')}</div>
      ) : filtered.length === 0 ? (
        <OpsEmpty message={zones.length === 0 ? t('dnsClusterEmpty') : t('accountsNoMatches')} />
      ) : (
        <div className="glass-panel" style={{ overflowX: 'auto' }}>
          <table className="glass-table">
            <thead>
              <tr>
                <th>{t('appsDomain')}</th>
                <th>Tenant</th>
                <th>Serial</th>
                <th>{t('dnsClusterRecords')}</th>
                <th>DNSSEC</th>
                <th>{t('dnsClusterUpdated')}</th>
                <th style={{ textAlign: 'right' }}>{t('appsActions')}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((zone) => (
                <tr key={zone.name}>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontWeight: 700, fontSize: '0.84rem' }}>
                      <Globe size={13} style={{ color: 'var(--brand-primary)' }} />
                      {zone.name}
                    </div>
                    <div className="font-mono" style={{ fontSize: '0.66rem', color: 'var(--text-muted)' }}>{zone.primaryNs}</div>
                  </td>
                  <td style={{ fontSize: '0.76rem' }}>{zone.tenantId || '-'}</td>
                  <td className="font-mono" style={{ fontSize: '0.74rem' }}>{zone.serial}</td>
                  <td style={{ fontSize: '0.78rem' }}>{zone.records?.length ?? 0}</td>
                  <td>
                    {zone.dnssec?.enabled ? (
                      <span className="badge-online" style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}><ShieldCheck size={11} /> enabled</span>
                    ) : (
                      <span className="badge-pill" style={{ background: 'rgba(148, 163, 184, 0.12)', color: 'var(--text-muted)' }}>disabled</span>
                    )}
                  </td>
                  <td style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>{formatDate(zone.updatedAt)}</td>
                  <td>
                    <div style={{ display: 'flex', gap: '4px', justifyContent: 'flex-end' }}>
                      <button type="button" className="btn btn-secondary btn-sm" onClick={() => void inspect(zone.name)} title={t('dnsClusterInspect')}>
                        <Activity size={13} />
                      </button>
                      <button type="button" className="btn btn-secondary btn-sm" onClick={() => void toggleDnssec(zone)} title={zone.dnssec?.enabled ? 'Disable DNSSEC' : 'Enable DNSSEC'}>
                        {zone.dnssec?.enabled ? <ShieldOff size={13} /> : <ShieldCheck size={13} />}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {selectedZone ? (
        <div className="glass-panel" style={{ padding: '18px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '10px', marginBottom: '12px' }}>
            <div style={{ fontSize: '0.88rem', fontWeight: 700 }}>
              {selectedZone.name} {t('dnsClusterDetail')}
            </div>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => void inspect(selectedZone.name)} disabled={healthLoading}>
              <Activity size={13} />
              {t('dnsClusterInspect')}
            </button>
          </div>

          {healthLoading ? (
            <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>{t('appsLoading')}</div>
          ) : health ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <StatusPill status={health.healthy ? 'healthy' : 'degraded'} />
                <span style={{ fontSize: '0.76rem', color: 'var(--text-muted)' }}>
                  SOA {health.hasSoa ? 'ok' : 'missing'} &middot; apex A {health.hasApexA ? 'ok' : 'missing'} &middot; NS {health.nsCount} &middot; MX {health.hasMx ? 'ok' : 'missing'}
                </span>
              </div>
              {health.issues && health.issues.length > 0 ? (
                <ul style={{ margin: 0, paddingLeft: '18px', fontSize: '0.78rem', color: 'var(--status-warning)' }}>
                  {health.issues.map((issue) => <li key={issue}>{issue}</li>)}
                </ul>
              ) : (
                <div style={{ fontSize: '0.78rem', color: 'var(--status-online)' }}>{t('dnsClusterNoIssues')}</div>
              )}
              {selectedZone.records && selectedZone.records.length > 0 ? (
                <div style={{ overflowX: 'auto' }}>
                  <table className="glass-table">
                    <thead>
                      <tr>
                        <th>{t('dnsClusterRecord')}</th>
                        <th>{t('dnsClusterType')}</th>
                        <th>{t('dnsClusterValue')}</th>
                        <th>TTL</th>
                      </tr>
                    </thead>
                    <tbody>
                      {selectedZone.records.map((record) => (
                        <tr key={record.id}>
                          <td className="font-mono" style={{ fontSize: '0.74rem' }}>{record.name}</td>
                          <td style={{ fontSize: '0.74rem' }}>{record.type}</td>
                          <td className="font-mono" style={{ fontSize: '0.72rem', wordBreak: 'break-all' }}>{record.value}</td>
                          <td style={{ fontSize: '0.74rem' }}>{record.ttl}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : null}
            </div>
          ) : (
            <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{t('dnsClusterInspectHint')}</div>
          )}
        </div>
      ) : null}
    </div>
  );
};
