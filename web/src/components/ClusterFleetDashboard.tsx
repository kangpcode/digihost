// DigiPanel Cluster Fleet Dashboard (Matching Image 3)
// Zero emojis, strictly Lucide SVG iconography, modern glass styling

import React from 'react';
import { 
  Server, 
  RefreshCw, 
  Plus, 
  CheckCircle2, 
  AlertTriangle, 
  Cpu, 
  Users, 
  Radio, 
  HeartPulse, 
  ShieldCheck, 
  ExternalLink,
  Terminal,
  ChevronRight
} from 'lucide-react';
import type { ClusterNode, Language } from '../types';
import { getTranslation } from '../i18n/translations';

interface ClusterFleetDashboardProps {
  nodes: ClusterNode[];
  language: Language;
  onConnectNode: () => void;
  onRefreshFleet: () => void;
  onOpenTerminal: () => void;
  onLoginNode: (nodeId: string) => void;
}

export const ClusterFleetDashboard: React.FC<ClusterFleetDashboardProps> = ({
  nodes,
  language,
  onConnectNode,
  onRefreshFleet,
  onOpenTerminal,
  onLoginNode
}) => {
  const t = (key: any) => getTranslation(language, key);

  const totalServers = nodes.length;
  const onlineServers = nodes.filter(n => n.status === 'online').length;
  const avgCpu = (nodes.reduce((acc, curr) => acc + curr.cpuPercent, 0) / (nodes.length || 1)).toFixed(1);
  const avgRam = (nodes.reduce((acc, curr) => acc + curr.ramPercent, 0) / (nodes.length || 1)).toFixed(1);
  const totalFleetAccounts = nodes.reduce((acc, curr) => acc + curr.inventory.accounts, 0);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      {/* Breadcrumb & Action Header (Image 3) */}
      <div>
        <div style={{ 
          display: 'flex', 
          alignItems: 'center', 
          gap: '6px', 
          fontSize: '0.82rem', 
          color: 'var(--text-muted)',
          marginBottom: '6px'
        }}>
          <span>{t('navCluster')}</span>
          <ChevronRight size={14} />
          <span style={{ color: 'var(--text-main)', fontWeight: 600 }}>{t('navHubDashboard')}</span>
        </div>

        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          flexWrap: 'wrap',
          gap: '16px'
        }}>
          <div>
            <h1 style={{
              fontSize: '1.6rem',
              fontWeight: 800,
              color: 'var(--text-main)',
              letterSpacing: '-0.02em',
              marginBottom: '4px'
            }}>
              {t('hubFleet')}
            </h1>
            <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
              {t('hubFleetSubtitle')}
            </p>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <button 
              onClick={onConnectNode}
              className="btn btn-primary"
            >
              <Plus size={16} />
              <span>{t('connectNode')}</span>
            </button>
            <button 
              onClick={onRefreshFleet}
              className="btn btn-secondary"
            >
              <RefreshCw size={15} />
              <span>{t('refresh')}</span>
            </button>
          </div>
        </div>
      </div>

      {/* Overall Health Card (Image 3) */}
      <div className="glass-panel" style={{
        padding: '18px 24px',
        display: 'flex',
        alignItems: 'center',
        gap: '16px'
      }}>
        <div style={{
          width: '44px',
          height: '44px',
          borderRadius: '12px',
          background: 'rgba(16, 185, 129, 0.15)',
          color: 'var(--status-online)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          flexShrink: 0
        }}>
          <CheckCircle2 size={24} />
        </div>
        <div>
          <h3 style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--text-main)', marginBottom: '2px' }}>
            Overall Health
          </h3>
          <div style={{ fontSize: '0.84rem', color: 'var(--text-muted)' }}>
            System status: <span style={{ fontWeight: 600, color: 'var(--status-online)' }}>Healthy</span>
            <span style={{ margin: '0 8px', color: 'var(--glass-border)' }}>|</span>
            {onlineServers} online, 0 degraded, 0 offline
          </div>
        </div>
      </div>

      {/* 8 Fleet Metrics Grid (Image 3) */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
        gap: '16px'
      }}>
        {/* 1. Total Servers */}
        <div className="glass-card" style={{ padding: '18px 20px', display: 'flex', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
              {t('metricTotalServers')}
            </div>
            <div className="font-mono" style={{ fontSize: '1.5rem', fontWeight: 800, color: 'var(--text-main)', marginBottom: '4px' }}>
              {totalServers}
            </div>
            <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>
              {onlineServers} online, 0 degraded, 0 offline
            </div>
          </div>
          <div style={{
            width: '40px',
            height: '40px',
            borderRadius: '12px',
            background: 'rgba(245, 158, 11, 0.12)',
            color: 'var(--brand-accent)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <Server size={20} />
          </div>
        </div>

        {/* 2. Services Running */}
        <div className="glass-card" style={{ padding: '18px 20px', display: 'flex', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
              {t('metricServicesRunning')}
            </div>
            <div className="font-mono" style={{ fontSize: '1.5rem', fontWeight: 800, color: 'var(--text-main)', marginBottom: '4px' }}>
              30/30
            </div>
            <div style={{ fontSize: '0.74rem', color: 'var(--status-online)', fontWeight: 600 }}>
              &uarr; 100% uptime
            </div>
          </div>
          <div style={{
            width: '40px',
            height: '40px',
            borderRadius: '12px',
            background: 'rgba(16, 185, 129, 0.12)',
            color: 'var(--status-online)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <Radio size={20} />
          </div>
        </div>

        {/* 3. Active Alerts */}
        <div className="glass-card" style={{ padding: '18px 20px', display: 'flex', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
              {t('metricActiveAlerts')}
            </div>
            <div className="font-mono" style={{ fontSize: '1.5rem', fontWeight: 800, color: 'var(--text-main)', marginBottom: '4px' }}>
              0
            </div>
            <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>
              0 warnings, 0 critical
            </div>
          </div>
          <div style={{
            width: '40px',
            height: '40px',
            borderRadius: '12px',
            background: 'rgba(245, 158, 11, 0.12)',
            color: 'var(--brand-accent)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <AlertTriangle size={20} />
          </div>
        </div>

        {/* 4. Avg Resources */}
        <div className="glass-card" style={{ padding: '18px 20px', display: 'flex', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
              {t('metricAvgResources')}
            </div>
            <div className="font-mono" style={{ fontSize: '1.4rem', fontWeight: 800, color: 'var(--text-main)', marginBottom: '4px' }}>
              {avgCpu}% / {avgRam}%
            </div>
            <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>
              CPU / Memory utilization
            </div>
          </div>
          <div style={{
            width: '40px',
            height: '40px',
            borderRadius: '12px',
            background: 'rgba(148, 163, 184, 0.15)',
            color: 'var(--text-muted)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <Cpu size={20} />
          </div>
        </div>

        {/* 5. Fleet Accounts */}
        <div className="glass-card" style={{ padding: '18px 20px', display: 'flex', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
              {t('metricFleetAccounts')}
            </div>
            <div className="font-mono" style={{ fontSize: '1.5rem', fontWeight: 800, color: 'var(--text-main)', marginBottom: '4px' }}>
              {totalFleetAccounts}
            </div>
            <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>
              domains across the fleet
            </div>
          </div>
          <div style={{
            width: '40px',
            height: '40px',
            borderRadius: '12px',
            background: 'rgba(2, 132, 199, 0.12)',
            color: 'var(--status-info)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <Users size={20} />
          </div>
        </div>

        {/* 6. Panel Versions */}
        <div className="glass-card" style={{ padding: '18px 20px', display: 'flex', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
              {t('metricPanelVersions')}
            </div>
            <div className="font-mono" style={{ fontSize: '1.5rem', fontWeight: 800, color: 'var(--text-main)', marginBottom: '4px' }}>
              1
            </div>
            <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>
              fleet in sync &bull; Hub: v1.2.0
            </div>
          </div>
          <div style={{
            width: '40px',
            height: '40px',
            borderRadius: '12px',
            background: 'rgba(99, 102, 241, 0.12)',
            color: '#6366f1',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <RefreshCw size={20} />
          </div>
        </div>

        {/* 7. Last Poll */}
        <div className="glass-card" style={{ padding: '18px 20px', display: 'flex', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
              {t('metricLastPoll')}
            </div>
            <div style={{ fontSize: '1.25rem', fontWeight: 800, color: 'var(--text-main)', marginBottom: '4px' }}>
              1 min ago
            </div>
            <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>
              polled every 60 seconds
            </div>
          </div>
          <div style={{
            width: '40px',
            height: '40px',
            borderRadius: '12px',
            background: 'rgba(244, 63, 94, 0.12)',
            color: '#f43f5e',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <HeartPulse size={20} />
          </div>
        </div>

        {/* 8. DNS Conflicts */}
        <div className="glass-card" style={{ padding: '18px 20px', display: 'flex', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
              {t('metricDnsConflicts')}
            </div>
            <div className="font-mono" style={{ fontSize: '1.5rem', fontWeight: 800, color: 'var(--text-main)', marginBottom: '4px' }}>
              0
            </div>
            <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>
              no duplicate domains
            </div>
          </div>
          <div style={{
            width: '40px',
            height: '40px',
            borderRadius: '12px',
            background: 'rgba(245, 158, 11, 0.12)',
            color: 'var(--brand-accent)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <ShieldCheck size={20} />
          </div>
        </div>
      </div>

      {/* Comprehensive Server Fleet Table (Image 3) */}
      <div className="glass-panel" style={{ padding: '24px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <div>
            <h3 style={{ fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-main)', marginBottom: '2px' }}>
              Fleet Infrastructure Matrix
            </h3>
            <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
              Per-node live resource telemetry, service health checks, and inventory
            </p>
          </div>
          <button onClick={onOpenTerminal} className="btn btn-secondary btn-sm">
            <Terminal size={14} />
            <span>Fleet Terminal</span>
          </button>
        </div>

        <div style={{ overflowX: 'auto' }}>
          <table className="glass-table">
            <thead>
              <tr>
                <th>{t('thServer')}</th>
                <th>{t('thResources')} (CPU / RAM / Disk)</th>
                <th>{t('thInventory')}</th>
                <th>{t('thServices')}</th>
                <th style={{ textAlign: 'right' }}>{t('thActions')}</th>
              </tr>
            </thead>
            <tbody>
              {nodes.map(node => (
                <tr key={node.id}>
                  {/* Server Info */}
                  <td style={{ minWidth: '220px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '2px' }}>
                      <span style={{ fontWeight: 700, color: 'var(--text-main)' }}>{node.name}</span>
                      {node.role === 'hub' && (
                        <span className="badge-pill" style={{ background: 'rgba(234, 88, 12, 0.15)', color: 'var(--brand-primary)' }}>
                          HUB
                        </span>
                      )}
                      <span className="badge-online">
                        {node.status}
                      </span>
                    </div>
                    <div className="font-mono" style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>
                      {node.ipAddress} &bull; {node.panelVersion}
                    </div>
                  </td>

                  {/* Resources */}
                  <td style={{ minWidth: '180px' }}>
                    <div style={{ display: 'flex', gap: '8px', fontSize: '0.75rem', fontWeight: 600 }}>
                      <span className="font-mono" style={{ color: 'var(--brand-primary)' }}>
                        CPU {node.cpuPercent}%
                      </span>
                      <span className="font-mono" style={{ color: 'var(--status-info)' }}>
                        RAM {node.ramPercent}%
                      </span>
                      <span className="font-mono" style={{ color: 'var(--status-online)' }}>
                        Disk {node.diskPercent}%
                      </span>
                    </div>
                    <div className="progress-bar-container" style={{ marginTop: '6px' }}>
                      <div 
                        className="progress-bar-fill" 
                        style={{ width: `${node.ramPercent}%`, background: 'var(--status-info)' }} 
                      />
                    </div>
                  </td>

                  {/* Inventory */}
                  <td style={{ minWidth: '160px' }}>
                    <div className="font-mono" style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                      Acc: <strong style={{ color: 'var(--text-main)' }}>{node.inventory.accounts}</strong> &bull; Dom: <strong style={{ color: 'var(--text-main)' }}>{node.inventory.domains}</strong> &bull; DB: <strong style={{ color: 'var(--text-main)' }}>{node.inventory.databases}</strong>
                    </div>
                  </td>

                  {/* Services Matrix (Image 3) */}
                  <td style={{ minWidth: '260px' }}>
                    <div style={{ display: 'flex', gap: '12px', alignItems: 'center' }}>
                      <div title="Web (nginx)" style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '0.72rem' }}>
                        <span className={`service-dot ${node.services.web ? 'online' : 'offline'}`} />
                        <span style={{ color: 'var(--text-muted)' }}>nginx</span>
                      </div>
                      <div title="FTP (pure-ftpd)" style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '0.72rem' }}>
                        <span className={`service-dot ${node.services.ftp ? 'online' : 'offline'}`} />
                        <span style={{ color: 'var(--text-muted)' }}>ftpd</span>
                      </div>
                      <div title="DNS (pdns)" style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '0.72rem' }}>
                        <span className={`service-dot ${node.services.dns ? 'online' : 'offline'}`} />
                        <span style={{ color: 'var(--text-muted)' }}>pdns</span>
                      </div>
                      <div title="Database (mariadb)" style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '0.72rem' }}>
                        <span className={`service-dot ${node.services.db ? 'online' : 'offline'}`} />
                        <span style={{ color: 'var(--text-muted)' }}>mariadb</span>
                      </div>
                      <div title="Security (fail2ban)" style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '0.72rem' }}>
                        <span className={`service-dot ${node.services.fail2ban ? 'online' : 'offline'}`} />
                        <span style={{ color: 'var(--text-muted)' }}>f2b</span>
                      </div>
                      <div title="Mail (postfix/dovecot)" style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '0.72rem' }}>
                        <span className={`service-dot ${node.services.postfix ? 'online' : 'offline'}`} />
                        <span style={{ color: 'var(--text-muted)' }}>mail</span>
                      </div>
                    </div>
                  </td>

                  {/* Action */}
                  <td style={{ textAlign: 'right' }}>
                    <button 
                      onClick={() => onLoginNode(node.id)}
                      className="btn btn-secondary btn-sm"
                      style={{ padding: '5px 10px', fontSize: '0.75rem' }}
                    >
                      <ExternalLink size={12} />
                      <span>Node Login</span>
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
