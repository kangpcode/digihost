// DigiPanel Server Status view.
// Displays real host telemetry from /proc and statfs, runtime interpreter
// versions probed on the host, loaded module manifests, and cluster members.

import React, { useEffect, useState } from 'react';
import { Activity, RefreshCw, Server, Cpu, Network, Boxes, Clock } from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';
import { OpsEmpty, OpsError, OpsHeader, StatusPill, UsageBar, formatBytes } from './opsShared';

interface Telemetry {
  hostname: string;
  kernel: string;
  os: string;
  uptime: string;
  uptimeSeconds: number;
  loadAverage: number[];
  cpuUsagePercent: number;
  memoryTotalMb: number;
  memoryUsedMb: number;
  memoryFreeMb: number;
  memoryCachedMb: number;
  diskTotalGb: number;
  diskUsedGb: number;
  diskFreeGb: number;
  activeProcesses: number;
  publicIp: string;
  runtimesDetected: {
    nodeVersion: string;
    pythonVersion: string;
    goVersion: string;
    phpVersion: string;
  };
}

interface ClusterNode {
  id: string;
  name: string;
  hostname: string;
  ipAddress: string;
  role: string;
  status: string;
  panelVersion: string;
}

interface ModuleManifest {
  name: string;
  version: string;
  title: string;
  description: string;
  permissions?: string[];
  hooks?: string[];
}

interface Fleet {
  totalNodes: number;
  nodes: ClusterNode[];
}

export const ServerStatusView: React.FC<{ language: Language }> = ({ language }) => {
  const t = (key: any) => getTranslation(language, key);
  const [telemetry, setTelemetry] = useState<Telemetry | null>(null);
  const [fleet, setFleet] = useState<Fleet | null>(null);
  const [modules, setModules] = useState<ModuleManifest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = async () => {
    try {
      setLoading(true);
      setError('');
      const [host, cluster, moduleList] = await Promise.allSettled([
        apiFetch<Telemetry>('/api/v1/system/telemetry'),
        apiFetch<Fleet>('/api/v1/cluster/fleet'),
        apiFetch<ModuleManifest[]>('/api/v1/system/modules')
      ]);
      if (host.status === 'fulfilled') setTelemetry(host.value);
      else setError(host.reason instanceof Error ? host.reason.message : String(host.reason));
      if (cluster.status === 'fulfilled') setFleet(cluster.value);
      if (moduleList.status === 'fulfilled') setModules(Array.isArray(moduleList.value) ? moduleList.value : []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
    const interval = window.setInterval(() => {
      void load();
    }, 10000);
    return () => window.clearInterval(interval);
  }, []);

  const memoryUsedMb = telemetry ? telemetry.memoryTotalMb - telemetry.memoryFreeMb : 0;
  const diskUsedBytes = telemetry ? telemetry.diskUsedGb * 1024 * 1024 * 1024 : 0;
  const diskTotalBytes = telemetry ? telemetry.diskTotalGb * 1024 * 1024 * 1024 : 0;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <OpsHeader
        title={t('serverStatusTitle')}
        subtitle={t('serverStatusSubtitle')}
        action={
          <button type="button" className="btn btn-secondary btn-sm" onClick={() => void load()} disabled={loading}>
            <RefreshCw size={14} />
            {t('refresh')}
          </button>
        }
      />

      {error ? <OpsError message={error} onDismiss={() => setError('')} /> : null}

      {loading && !telemetry ? (
        <div className="glass-panel" style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>{t('appsLoading')}</div>
      ) : telemetry ? (
        <>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(210px, 1fr))', gap: '12px' }}>
            {[
              { icon: Server, label: t('hostnameLabel'), value: telemetry.hostname, sub: telemetry.publicIp || '-' },
              { icon: Cpu, label: t('osPlatform'), value: telemetry.os || '-', sub: telemetry.kernel || '-' },
              { icon: Clock, label: t('uptime'), value: telemetry.uptime || '-', sub: `${telemetry.activeProcesses} ${t('serverStatusProcesses')}` },
              { icon: Activity, label: t('loadAverage'), value: (telemetry.loadAverage || []).map((value) => value.toFixed(2)).join(' / ') || '-', sub: `${telemetry.cpuUsagePercent.toFixed(1)}% CPU` }
            ].map((card) => (
              <div key={card.label} className="glass-card" style={{ padding: '16px', display: 'flex', gap: '12px' }}>
                <card.icon size={20} style={{ color: 'var(--brand-primary)', flexShrink: 0, marginTop: '2px' }} />
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-muted)' }}>{card.label}</div>
                  <div style={{ fontSize: '0.9rem', fontWeight: 700, wordBreak: 'break-word' }}>{card.value}</div>
                  <div style={{ fontSize: '0.7rem', color: 'var(--text-dim)', wordBreak: 'break-word' }}>{card.sub}</div>
                </div>
              </div>
            ))}
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '12px' }}>
            <div className="glass-panel" style={{ padding: '18px', display: 'flex', flexDirection: 'column', gap: '14px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.88rem', fontWeight: 700 }}>
                <Cpu size={16} style={{ color: 'var(--brand-primary)' }} />
                {t('serverStatusResources')}
              </div>
              <UsageBar used={telemetry.cpuUsagePercent} limit={100} label="CPU" />
              <UsageBar used={memoryUsedMb} limit={telemetry.memoryTotalMb} label={`${t('memoryUsage')} ${telemetry.memoryUsedMb} / ${telemetry.memoryTotalMb} MB`} />
              <UsageBar used={diskUsedBytes} limit={diskTotalBytes} label={`${t('diskUsage')} ${telemetry.diskUsedGb.toFixed(1)} / ${telemetry.diskTotalGb.toFixed(1)} GB`} />
              <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', display: 'flex', gap: '16px' }}>
                <span>{t('serverStatusCached')}: {formatBytes(telemetry.memoryCachedMb * 1024 * 1024)}</span>
                <span>{t('serverStatusFreeDisk')}: {telemetry.diskFreeGb.toFixed(1)} GB</span>
              </div>
            </div>

            <div className="glass-panel" style={{ padding: '18px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.88rem', fontWeight: 700 }}>
                <Cpu size={16} style={{ color: 'var(--brand-primary)' }} />
                {t('serverStatusRuntimes')}
              </div>
              {[
                ['Node.js', telemetry.runtimesDetected?.nodeVersion],
                ['Python', telemetry.runtimesDetected?.pythonVersion],
                ['Go', telemetry.runtimesDetected?.goVersion],
                ['PHP', telemetry.runtimesDetected?.phpVersion]
              ].map(([name, version]) => (
                <div key={name} style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.78rem', borderBottom: '1px solid var(--glass-border)', paddingBottom: '7px' }}>
                  <span style={{ color: 'var(--text-muted)' }}>{name}</span>
                  <span className="font-mono">{version || t('serverStatusNotDetected')}</span>
                </div>
              ))}
            </div>
          </div>

          <div className="glass-panel" style={{ padding: '18px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.88rem', fontWeight: 700, marginBottom: '12px' }}>
              <Network size={16} style={{ color: 'var(--brand-primary)' }} />
              {t('navCluster')}
              <span className="badge-pill" style={{ background: 'rgba(148, 163, 184, 0.12)', color: 'var(--text-muted)' }}>
                {fleet?.totalNodes ?? 0}
              </span>
            </div>
            {!fleet || fleet.nodes.length === 0 ? (
              <OpsEmpty message={t('serverStatusNoNodes')} />
            ) : (
              <div style={{ overflowX: 'auto' }}>
                <table className="glass-table">
                  <thead>
                    <tr>
                      <th>{t('serverStatusNode')}</th>
                      <th>{t('hostnameLabel')}</th>
                      <th>IP</th>
                      <th>{t('role')}</th>
                      <th>{t('appsStatus')}</th>
                      <th>{t('serverStatusVersion')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {fleet.nodes.map((node) => (
                      <tr key={node.id}>
                        <td style={{ fontWeight: 700, fontSize: '0.82rem' }}>{node.name}</td>
                        <td className="font-mono" style={{ fontSize: '0.75rem' }}>{node.hostname}</td>
                        <td className="font-mono" style={{ fontSize: '0.75rem' }}>{node.ipAddress}</td>
                        <td style={{ fontSize: '0.78rem', textTransform: 'capitalize' }}>{node.role}</td>
                        <td><StatusPill status={node.status} /></td>
                        <td style={{ fontSize: '0.75rem' }}>{node.panelVersion}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          <div className="glass-panel" style={{ padding: '18px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.88rem', fontWeight: 700, marginBottom: '12px' }}>
              <Boxes size={16} style={{ color: 'var(--brand-primary)' }} />
              {t('serverStatusModules')}
              <span className="badge-pill" style={{ background: 'rgba(148, 163, 184, 0.12)', color: 'var(--text-muted)' }}>
                {modules.length}
              </span>
            </div>
            {modules.length === 0 ? (
              <OpsEmpty message={t('serverStatusNoModules')} />
            ) : (
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))', gap: '10px' }}>
                {modules.map((module) => (
                  <div key={module.name} className="glass-card" style={{ padding: '12px 14px' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '8px' }}>
                      <span style={{ fontSize: '0.82rem', fontWeight: 700 }}>{module.title || module.name}</span>
                      <span className="badge-pill" style={{ background: 'rgba(16, 185, 129, 0.12)', color: 'var(--status-online)' }}>loaded</span>
                    </div>
                    <div className="font-mono" style={{ fontSize: '0.68rem', color: 'var(--text-muted)', marginTop: '2px' }}>
                      {module.name} v{module.version}
                    </div>
                    <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', marginTop: '4px', lineHeight: 1.45 }}>{module.description}</div>
                    <div style={{ fontSize: '0.68rem', color: 'var(--text-dim)', marginTop: '6px' }}>
                      {module.permissions?.length ?? 0} {t('serverStatusPermissions')} &middot; {module.hooks?.length ?? 0} {t('serverStatusHooks')}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </>
      ) : (
        <OpsEmpty message={t('serverStatusUnavailable')} />
      )}
    </div>
  );
};
