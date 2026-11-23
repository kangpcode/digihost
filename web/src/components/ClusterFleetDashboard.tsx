// DigiPanel Cluster and Host Inventory view.
// Renders the node registry that the core actually reports plus live telemetry
// for the local host. Metrics that a remote node does not publish are shown as
// not reported instead of being estimated.

import React, { useEffect, useState } from 'react';
import {
  Server,
  RefreshCw,
  Cpu,
  Users,
  Radio,
  ExternalLink,
  Terminal,
  ChevronRight,
  HardDrive,
  Globe,
  Database,
  Activity,
  Plus
} from 'lucide-react';
import type { Telemetry, ServiceStatus, Language, FleetNode } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';
import { OpsEmpty, StatusPill, UsageBar } from './opsShared';

interface ClusterFleetDashboardProps {
  nodes: FleetNode[];
  telemetry: Telemetry | null;
  services: ServiceStatus[];
  servicesAvailable: boolean;
  inventory: {
    accounts: number;
    domains: number;
    databases: number;
    resellers: number;
  };
  language: Language;
  onRefreshFleet: () => void;
  onOpenTerminal: () => void;
}

export const ClusterFleetDashboard: React.FC<ClusterFleetDashboardProps> = ({
  nodes,
  telemetry,
  services,
  servicesAvailable,
  inventory,
  language,
  onRefreshFleet,
  onOpenTerminal
}) => {
  const t = (key: any) => getTranslation(language, key);
  const [showConnectForm, setShowConnectForm] = useState(false);
  const [nodeForm, setNodeForm] = useState({ name: '', endpointUrl: '', ipAddress: '', role: 'edge', certFingerprint: '', nodeToken: '' });
  const [clusterNotice, setClusterNotice] = useState('');
  const [clusterError, setClusterError] = useState('');
  const [tenants, setTenants] = useState<Array<{ id: string; slug: string; companyName: string; branding: { panelName: string; customDomain: string }; subscription: { planName: string; status: string } }>>([]);
  const [tenantForm, setTenantForm] = useState({ slug: '', companyName: '', plan: 'growth', panelName: '', customDomain: '' });

  const loadTenants = async () => {
    try {
      const list = await apiFetch<typeof tenants>('/api/v1/saas/tenants');
      if (Array.isArray(list)) setTenants(list);
    } catch {
      // Non-admin roles may not view all SaaS tenants
    }
  };

  useEffect(() => {
    let active = true;
    apiFetch<typeof tenants>('/api/v1/saas/tenants')
      .then((list) => { if (active && Array.isArray(list)) setTenants(list); })
      .catch(() => {});
    return () => { active = false; };
  }, []);

  const onlineNodes = nodes.filter((node) => node.status === 'online').length;
  const activeServices = services.filter((service) => service.active).length;
  const memoryPercent = telemetry && telemetry.memoryTotalMb > 0
    ? (telemetry.memoryUsedMb / telemetry.memoryTotalMb) * 100
    : 0;
  const diskPercent = telemetry && telemetry.diskTotalGb > 0
    ? (telemetry.diskUsedGb / telemetry.diskTotalGb) * 100
    : 0;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '0.82rem', color: 'var(--text-muted)', marginBottom: '6px' }}>
          <span>{t('navCluster')}</span>
          <ChevronRight size={14} />
          <span style={{ color: 'var(--text-main)', fontWeight: 600 }}>{t('navHubDashboard')}</span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '16px' }}>
          <div>
            <h1 style={{ fontSize: '1.6rem', fontWeight: 800, color: 'var(--text-main)', letterSpacing: '-0.02em', marginBottom: '4px' }}>
              {t('hubFleet')}
            </h1>
            <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>{t('hubFleetSubtitle')}</p>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
            <button type="button" onClick={onRefreshFleet} className="btn btn-secondary">
              <RefreshCw size={15} />
              <span>{t('refresh')}</span>
            </button>
            <button
              type="button"
              onClick={() => {
                setClusterError('');
                void apiFetch<{ syncedNodes: number; zonesCount: number }>('/api/v1/cluster/dns/sync', { method: 'POST', body: JSON.stringify({}) })
                  .then((res) => setClusterNotice(`DNS synced: ${res.zonesCount} zones -> ${res.syncedNodes} nodes`))
                  .catch((err: unknown) => setClusterError(err instanceof Error ? err.message : String(err)));
              }}
              className="btn btn-secondary"
            >
              <Globe size={15} />
              <span>{t('clusterSyncDns')}</span>
            </button>
            <button type="button" onClick={() => setShowConnectForm((v) => !v)} className="btn btn-secondary">
              <Plus size={15} />
              <span>{t('clusterConnectNode')}</span>
            </button>
            <button type="button" onClick={onOpenTerminal} className="btn btn-primary">
              <Terminal size={15} />
              <span>{t('terminalTitle')}</span>
            </button>
          </div>
        </div>
      </div>

      {clusterNotice ? <div className="glass-card" style={{ padding: '10px 14px', fontSize: '0.82rem', borderColor: 'rgba(16, 185, 129, 0.35)' }}>{clusterNotice}</div> : null}
      {clusterError ? <div className="glass-card" style={{ padding: '10px 14px', fontSize: '0.82rem', borderColor: 'rgba(239, 68, 68, 0.35)' }}>{clusterError}</div> : null}

      {showConnectForm ? (
        <form
          className="glass-panel"
          style={{ padding: '16px', display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(170px, 1fr))', gap: '10px', alignItems: 'end' }}
          onSubmit={(e) => {
            e.preventDefault();
            setClusterError('');
            void apiFetch('/api/v1/cluster/nodes/connect', {
              method: 'POST',
              body: JSON.stringify(nodeForm)
            }).then(() => {
              setShowConnectForm(false);
              setNodeForm({ name: '', endpointUrl: '', ipAddress: '', role: 'edge', certFingerprint: '', nodeToken: '' });
              setClusterNotice('mTLS cluster node enrolled.');
              onRefreshFleet();
            }).catch((err: unknown) => setClusterError(err instanceof Error ? err.message : String(err)));
          }}
        >
          <input className="input-glass" required placeholder="Node name (edge-01)" value={nodeForm.name} onChange={(e) => setNodeForm((n) => ({ ...n, name: e.target.value }))} />
          <input className="input-glass" required placeholder="https://10.0.0.2:8443" value={nodeForm.endpointUrl} onChange={(e) => setNodeForm((n) => ({ ...n, endpointUrl: e.target.value }))} />
          <input className="input-glass" placeholder="IP address" value={nodeForm.ipAddress} onChange={(e) => setNodeForm((n) => ({ ...n, ipAddress: e.target.value }))} />
          <input className="input-glass" required placeholder="SHA-256 cert fingerprint" value={nodeForm.certFingerprint} onChange={(e) => setNodeForm((n) => ({ ...n, certFingerprint: e.target.value }))} />
          <input className="input-glass" type="password" placeholder="Node token" value={nodeForm.nodeToken} onChange={(e) => setNodeForm((n) => ({ ...n, nodeToken: e.target.value }))} />
          <button type="submit" className="btn btn-primary btn-sm">{t('clusterConnectNode')}</button>
        </form>
      ) : null}

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(210px, 1fr))', gap: '14px' }}>
        {[
          { icon: Server, label: t('metricTotalServers'), value: `${onlineNodes} / ${nodes.length}`, sub: t('fleetOnlineSummary') },
          { icon: Users, label: t('metricFleetAccounts'), value: String(inventory.accounts), sub: t('metricDomains') },
          { icon: Globe, label: t('metricDomains'), value: String(inventory.domains), sub: t('navDomains') },
          { icon: Radio, label: t('metricServicesRunning'), value: servicesAvailable ? `${activeServices} / ${services.length}` : '-', sub: servicesAvailable ? t('serverStatusServices') : t('serverStatusUnavailable') },
          { icon: Database, label: t('metricDatabases'), value: String(inventory.databases), sub: t('navDatabases') },
          { icon: Users, label: t('metricResellers'), value: String(inventory.resellers), sub: t('navResellerCenter') }
        ].map((card) => (
          <div key={card.label} className="glass-card" style={{ padding: '16px 18px', display: 'flex', justifyContent: 'space-between', gap: '10px' }}>
            <div>
              <div style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>{card.label}</div>
              <div className="font-mono" style={{ fontSize: '1.35rem', fontWeight: 800, color: 'var(--text-main)' }}>{card.value}</div>
              <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>{card.sub}</div>
            </div>
            <div style={{ width: '38px', height: '38px', borderRadius: '11px', background: 'rgba(234, 88, 12, 0.12)', color: 'var(--brand-primary)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
              <card.icon size={19} />
            </div>
          </div>
        ))}
      </div>

      {telemetry ? (
        <div className="glass-panel" style={{ padding: '20px', display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.92rem', fontWeight: 700 }}>
            <Activity size={16} style={{ color: 'var(--brand-primary)' }} />
            {t('serverStatusResources')}
            <span className="font-mono" style={{ fontSize: '0.72rem', fontWeight: 500, color: 'var(--text-muted)' }}>
              {telemetry.hostname} {telemetry.publicIp ? `(${telemetry.publicIp})` : ''}
            </span>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '16px' }}>
            <UsageBar used={telemetry.cpuUsagePercent} limit={100} label={`CPU ${telemetry.cpuUsagePercent.toFixed(1)}%`} />
            <UsageBar used={telemetry.memoryUsedMb} limit={telemetry.memoryTotalMb} label={`${t('memoryUsage')} ${telemetry.memoryUsedMb} / ${telemetry.memoryTotalMb} MB`} />
            <UsageBar used={telemetry.diskUsedGb} limit={telemetry.diskTotalGb} label={`${t('diskUsage')} ${telemetry.diskUsedGb.toFixed(1)} / ${telemetry.diskTotalGb.toFixed(1)} GB`} />
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '18px', fontSize: '0.75rem', color: 'var(--text-muted)' }}>
            <span>{t('osPlatform')}: {telemetry.os || '-'}</span>
            <span>{t('kernelVersion')}: {telemetry.kernel || '-'}</span>
            <span>{t('uptime')}: {telemetry.uptime || '-'}</span>
            <span>{t('loadAverage')}: {telemetry.loadAverage.map((value) => value.toFixed(2)).join(' / ') || '-'}</span>
            <span>{t('serverStatusProcesses')}: {telemetry.activeProcesses}</span>
            <span>CPU {Math.round(telemetry.cpuUsagePercent)}% &middot; RAM {Math.round(memoryPercent)}% &middot; Disk {Math.round(diskPercent)}%</span>
          </div>
        </div>
      ) : null}

      <div className="glass-panel" style={{ padding: '20px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.95rem', fontWeight: 700, marginBottom: '12px' }}>
          <Server size={17} style={{ color: 'var(--brand-primary)' }} />
          {t('thServer')}
        </div>
        {nodes.length === 0 ? (
          <OpsEmpty message={t('serverStatusNoNodes')} />
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table className="glass-table">
              <thead>
                <tr>
                  <th>{t('thServer')}</th>
                  <th>IP</th>
                  <th>{t('role')}</th>
                  <th>{t('appsStatus')}</th>
                  <th>{t('serverStatusVersion')}</th>
                  <th style={{ textAlign: 'right' }}>{t('thActions')}</th>
                </tr>
              </thead>
              <tbody>
                {nodes.map((node) => (
                  <tr key={node.id}>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontWeight: 700, fontSize: '0.82rem' }}>
                        <Server size={12} style={{ color: 'var(--brand-primary)' }} />
                        {node.name}
                      </div>
                      <div className="font-mono" style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>{node.hostname}</div>
                    </td>
                    <td className="font-mono" style={{ fontSize: '0.75rem' }}>{node.ipAddress || '-'}</td>
                    <td style={{ fontSize: '0.78rem', textTransform: 'capitalize' }}>{node.role}</td>
                    <td><StatusPill status={node.status} /></td>
                    <td className="font-mono" style={{ fontSize: '0.75rem' }}>{node.panelVersion}</td>
                    <td style={{ textAlign: 'right' }}>
                      <button type="button" onClick={onOpenTerminal} className="btn btn-secondary btn-sm">
                        <ExternalLink size={12} />
                        <span>{t('fleetNodeLogin')}</span>
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <div style={{ marginTop: '10px', fontSize: '0.72rem', color: 'var(--text-dim)' }}>
          {t('fleetRemoteMetricsNote')}
        </div>
      </div>

      <div className="glass-panel" style={{ padding: '20px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.95rem', fontWeight: 700, marginBottom: '12px' }}>
          <Cpu size={17} style={{ color: 'var(--brand-primary)' }} />
          {t('serverStatusServices')}
        </div>
        {!servicesAvailable ? (
          <OpsEmpty message={t('serverStatusServicesUnavailable')} />
        ) : services.length === 0 ? (
          <OpsEmpty message={t('serverStatusNoServices')} />
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(210px, 1fr))', gap: '10px' }}>
            {services.map((service) => (
              <div key={service.unit} className="glass-card" style={{ padding: '12px 14px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '8px' }}>
                  <span style={{ fontSize: '0.8rem', fontWeight: 700 }}>{service.name}</span>
                  <StatusPill status={service.installed ? service.state : 'not-installed'} />
                </div>
                <div className="font-mono" style={{ fontSize: '0.68rem', color: 'var(--text-muted)', marginTop: '2px' }}>{service.unit}</div>
                <div style={{ fontSize: '0.68rem', color: 'var(--text-dim)', marginTop: '4px' }}>
                  {t('serverStatusCategory')}: {service.category} &middot; {t('serverStatusEnabled')}: {service.enabled ? t('yes') : t('no')}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="glass-panel" style={{ padding: '20px', display: 'flex', flexDirection: 'column', gap: '14px' }}>
        <div style={{ fontSize: '0.95rem', fontWeight: 700 }}>{t('saasTenantsTitle')}</div>
        <form
          style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: '10px', alignItems: 'end' }}
          onSubmit={(e) => {
            e.preventDefault();
            setClusterError('');
            void apiFetch('/api/v1/saas/tenants', {
              method: 'POST',
              body: JSON.stringify({
                slug: tenantForm.slug,
                companyName: tenantForm.companyName,
                plan: tenantForm.plan,
                branding: { panelName: tenantForm.panelName, customDomain: tenantForm.customDomain }
              })
            }).then(() => {
              setTenantForm({ slug: '', companyName: '', plan: 'growth', panelName: '', customDomain: '' });
              return loadTenants();
            }).catch((err: unknown) => setClusterError(err instanceof Error ? err.message : String(err)));
          }}
        >
          <input className="input-glass" required placeholder="slug" value={tenantForm.slug} onChange={(e) => setTenantForm((x) => ({ ...x, slug: e.target.value }))} />
          <input className="input-glass" required placeholder="Company name" value={tenantForm.companyName} onChange={(e) => setTenantForm((x) => ({ ...x, companyName: e.target.value }))} />
          <select className="input-glass" value={tenantForm.plan} onChange={(e) => setTenantForm((x) => ({ ...x, plan: e.target.value }))}>
            <option value="starter">starter</option>
            <option value="growth">growth</option>
            <option value="enterprise">enterprise</option>
          </select>
          <input className="input-glass" placeholder={t('saasBrandLabel')} value={tenantForm.panelName} onChange={(e) => setTenantForm((x) => ({ ...x, panelName: e.target.value }))} />
          <input className="input-glass" placeholder={t('saasCustomDomain')} value={tenantForm.customDomain} onChange={(e) => setTenantForm((x) => ({ ...x, customDomain: e.target.value }))} />
          <button type="submit" className="btn btn-primary btn-sm">{t('saasAddTenant')}</button>
        </form>
        <div style={{ overflowX: 'auto' }}>
          <table className="glass-table">
            <thead>
              <tr><th>Slug</th><th>{t('saasBrandLabel')}</th><th>{t('saasCustomDomain')}</th><th>{t('saasPlanLabel')}</th><th>Status</th></tr>
            </thead>
            <tbody>
              {tenants.map((tn) => (
                <tr key={tn.id}>
                  <td className="font-mono">{tn.slug}</td>
                  <td>{tn.branding.panelName}</td>
                  <td className="font-mono">{tn.branding.customDomain || '-'}</td>
                  <td>{tn.subscription.planName}</td>
                  <td><StatusPill status={tn.subscription.status} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <div className="glass-panel" style={{ padding: '16px 20px', display: 'flex', alignItems: 'center', gap: '10px', fontSize: '0.78rem', color: 'var(--text-muted)' }}>
        <HardDrive size={15} style={{ color: 'var(--brand-accent)' }} />
        {t('fleetClusterNote')}
      </div>
    </div>
  );
};
