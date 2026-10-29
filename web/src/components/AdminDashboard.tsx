// DigiPanel Admin Dashboard (Matching Image 2)
// Zero emojis, strictly Lucide SVG iconography, modern glass styling

import React, { useState } from 'react';
import { 
  Zap, 
  X, 
  Check, 
  UserPlus, 
  Users, 
  Globe, 
  UserCheck, 
  BarChart3, 
  ShieldCheck, 
  HardDrive, 
  Database, 
  ExternalLink,
  Server
} from 'lucide-react';
import type { HostingAccount, SystemMetrics, Language } from '../types';
import { getTranslation } from '../i18n/translations';

interface AdminDashboardProps {
  accounts: HostingAccount[];
  systemMetrics: SystemMetrics;
  language: Language;
  onOpenNewAccount: () => void;
  onOpenDnsEditor: (domain?: string) => void;
  onSwitchToUser: (username: string) => void;
  onToggleSuspend: (accountId: string) => void;
  onViewCluster: () => void;
}

export const AdminDashboard: React.FC<AdminDashboardProps> = ({
  accounts,
  systemMetrics,
  language,
  onOpenNewAccount,
  onOpenDnsEditor,
  onSwitchToUser,
  onToggleSuspend,
  onViewCluster
}) => {
  const [showWelcome, setShowWelcome] = useState(true);

  const t = (key: any) => getTranslation(language, key);

  const totalHostingAccounts = accounts.length;
  const totalDomains = accounts.reduce((acc, curr) => acc + (curr.domain ? 1 : 0), 0);
  const totalDatabases = accounts.reduce((acc, curr) => acc + curr.databasesCount, 0);
  const totalResellers = 4;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      {/* Welcome Banner (Image 2) */}
      {showWelcome && (
        <div className="glass-panel" style={{
          padding: '24px 28px',
          position: 'relative',
          background: 'var(--glass-surface)',
          border: '1px solid var(--glass-border-highlight)',
          boxShadow: 'var(--glass-shadow)'
        }}>
          <button 
            onClick={() => setShowWelcome(false)}
            className="btn-icon"
            style={{ position: 'absolute', top: '16px', right: '16px' }}
            title={t('close')}
          >
            <X size={18} />
          </button>

          <div style={{ display: 'flex', gap: '20px', alignItems: 'flex-start' }}>
            {/* Zap Icon in squircle */}
            <div style={{
              width: '56px',
              height: '56px',
              borderRadius: '16px',
              background: 'linear-gradient(135deg, rgba(234, 88, 12, 0.15) 0%, rgba(245, 158, 11, 0.2) 100%)',
              border: '1px solid rgba(234, 88, 12, 0.3)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: 'var(--brand-primary)',
              flexShrink: 0
            }}>
              <Zap size={28} strokeWidth={2.5} />
            </div>

            <div style={{ flex: 1 }}>
              <h2 style={{
                fontSize: '1.5rem',
                fontWeight: 800,
                color: 'var(--text-main)',
                letterSpacing: '-0.02em',
                marginBottom: '4px'
              }}>
                {t('welcomeTitle')}
              </h2>
              <h3 style={{
                fontSize: '1rem',
                fontWeight: 600,
                color: 'var(--brand-primary)',
                marginBottom: '10px'
              }}>
                {t('welcomeSubtitle')}
              </h3>
              <p style={{
                fontSize: '0.875rem',
                color: 'var(--text-muted)',
                lineHeight: 1.6,
                maxWidth: '920px',
                marginBottom: '16px'
              }}>
                {t('welcomeDescription')}
              </p>

              {/* 4 Feature Badges */}
              <div style={{
                display: 'flex',
                flexWrap: 'wrap',
                gap: '16px 24px',
                fontSize: '0.84rem',
                fontWeight: 600,
                color: 'var(--text-main)'
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <div style={{
                    width: '18px',
                    height: '18px',
                    borderRadius: '50%',
                    background: 'rgba(16, 185, 129, 0.15)',
                    color: 'var(--status-online)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center'
                  }}>
                    <Check size={12} strokeWidth={3} />
                  </div>
                  <span>{t('featControl')}</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <div style={{
                    width: '18px',
                    height: '18px',
                    borderRadius: '50%',
                    background: 'rgba(16, 185, 129, 0.15)',
                    color: 'var(--status-online)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center'
                  }}>
                    <Check size={12} strokeWidth={3} />
                  </div>
                  <span>{t('featSecurity')}</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <div style={{
                    width: '18px',
                    height: '18px',
                    borderRadius: '50%',
                    background: 'rgba(16, 185, 129, 0.15)',
                    color: 'var(--status-online)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center'
                  }}>
                    <Check size={12} strokeWidth={3} />
                  </div>
                  <span>{t('featManagement')}</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <div style={{
                    width: '18px',
                    height: '18px',
                    borderRadius: '50%',
                    background: 'rgba(16, 185, 129, 0.15)',
                    color: 'var(--status-online)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center'
                  }}>
                    <Check size={12} strokeWidth={3} />
                  </div>
                  <span>{t('featMultiServer')}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Quick Actions Header */}
      <div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '14px' }}>
          <Zap size={18} style={{ color: 'var(--brand-primary)' }} />
          <h3 style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--text-main)' }}>
            {t('quickActions')}
          </h3>
        </div>

        {/* 6 Quick Action Cards */}
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(170px, 1fr))',
          gap: '16px'
        }}>
          {/* 1. New Account */}
          <div 
            onClick={onOpenNewAccount}
            className="glass-card" 
            style={{
              padding: '20px 16px',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              cursor: 'pointer',
              textAlign: 'center'
            }}
          >
            <div style={{
              width: '44px',
              height: '44px',
              borderRadius: '12px',
              background: 'rgba(234, 88, 12, 0.12)',
              color: 'var(--brand-primary)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginBottom: '10px'
            }}>
              <UserPlus size={22} />
            </div>
            <span style={{ fontSize: '0.86rem', fontWeight: 700, color: 'var(--text-main)' }}>
              {t('actionNewAccount')}
            </span>
          </div>

          {/* 2. All Accounts */}
          <div 
            onClick={onOpenNewAccount}
            className="glass-card" 
            style={{
              padding: '20px 16px',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              cursor: 'pointer',
              textAlign: 'center'
            }}
          >
            <div style={{
              width: '44px',
              height: '44px',
              borderRadius: '12px',
              background: 'rgba(16, 185, 129, 0.12)',
              color: 'var(--status-online)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginBottom: '10px'
            }}>
              <Users size={22} />
            </div>
            <span style={{ fontSize: '0.86rem', fontWeight: 700, color: 'var(--text-main)' }}>
              {t('actionAllAccounts')}
            </span>
          </div>

          {/* 3. DNS Records */}
          <div 
            onClick={() => onOpenDnsEditor()}
            className="glass-card" 
            style={{
              padding: '20px 16px',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              cursor: 'pointer',
              textAlign: 'center'
            }}
          >
            <div style={{
              width: '44px',
              height: '44px',
              borderRadius: '12px',
              background: 'rgba(2, 132, 199, 0.12)',
              color: 'var(--status-info)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginBottom: '10px'
            }}>
              <Globe size={22} />
            </div>
            <span style={{ fontSize: '0.86rem', fontWeight: 700, color: 'var(--text-main)' }}>
              {t('actionDnsRecords')}
            </span>
          </div>

          {/* 4. Resellers */}
          <div 
            onClick={onViewCluster}
            className="glass-card" 
            style={{
              padding: '20px 16px',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              cursor: 'pointer',
              textAlign: 'center'
            }}
          >
            <div style={{
              width: '44px',
              height: '44px',
              borderRadius: '12px',
              background: 'rgba(245, 158, 11, 0.12)',
              color: 'var(--brand-accent)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginBottom: '10px'
            }}>
              <UserCheck size={22} />
            </div>
            <span style={{ fontSize: '0.86rem', fontWeight: 700, color: 'var(--text-main)' }}>
              {t('actionResellers')}
            </span>
          </div>

          {/* 5. Metrics */}
          <div 
            onClick={onViewCluster}
            className="glass-card" 
            style={{
              padding: '20px 16px',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              cursor: 'pointer',
              textAlign: 'center'
            }}
          >
            <div style={{
              width: '44px',
              height: '44px',
              borderRadius: '12px',
              background: 'rgba(239, 68, 68, 0.12)',
              color: 'var(--status-danger)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginBottom: '10px'
            }}>
              <BarChart3 size={22} />
            </div>
            <span style={{ fontSize: '0.86rem', fontWeight: 700, color: 'var(--text-main)' }}>
              {t('actionMetrics')}
            </span>
          </div>

          {/* 6. SSL Certificates */}
          <div 
            onClick={() => onOpenDnsEditor()}
            className="glass-card" 
            style={{
              padding: '20px 16px',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              cursor: 'pointer',
              textAlign: 'center'
            }}
          >
            <div style={{
              width: '44px',
              height: '44px',
              borderRadius: '12px',
              background: 'rgba(147, 51, 234, 0.12)',
              color: '#9333ea',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginBottom: '10px'
            }}>
              <ShieldCheck size={22} />
            </div>
            <span style={{ fontSize: '0.86rem', fontWeight: 700, color: 'var(--text-main)' }}>
              {t('actionSslCertificates')}
            </span>
          </div>
        </div>
      </div>

      {/* 4 Counter Metric Cards (Image 2) */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
        gap: '16px'
      }}>
        {/* 1. Hosting Accounts */}
        <div className="glass-card" style={{
          padding: '20px 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between'
        }}>
          <div>
            <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
              {t('metricHostingAccounts')}
            </div>
            <div className="font-mono" style={{ fontSize: '1.75rem', fontWeight: 800, color: 'var(--text-main)' }}>
              {totalHostingAccounts}
            </div>
          </div>
          <div style={{
            width: '48px',
            height: '48px',
            borderRadius: '14px',
            background: 'rgba(234, 88, 12, 0.15)',
            color: 'var(--brand-primary)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <HardDrive size={24} />
          </div>
        </div>

        {/* 2. Domains */}
        <div className="glass-card" style={{
          padding: '20px 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between'
        }}>
          <div>
            <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
              {t('metricDomains')}
            </div>
            <div className="font-mono" style={{ fontSize: '1.75rem', fontWeight: 800, color: 'var(--text-main)' }}>
              {totalDomains}
            </div>
          </div>
          <div style={{
            width: '48px',
            height: '48px',
            borderRadius: '14px',
            background: 'rgba(16, 185, 129, 0.15)',
            color: 'var(--status-online)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <Globe size={24} />
          </div>
        </div>

        {/* 3. Databases */}
        <div className="glass-card" style={{
          padding: '20px 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between'
        }}>
          <div>
            <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
              {t('metricDatabases')}
            </div>
            <div className="font-mono" style={{ fontSize: '1.75rem', fontWeight: 800, color: 'var(--text-main)' }}>
              {totalDatabases}
            </div>
          </div>
          <div style={{
            width: '48px',
            height: '48px',
            borderRadius: '14px',
            background: 'rgba(2, 132, 199, 0.15)',
            color: 'var(--status-info)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <Database size={24} />
          </div>
        </div>

        {/* 4. Resellers */}
        <div className="glass-card" style={{
          padding: '20px 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between'
        }}>
          <div>
            <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
              {t('metricResellers')}
            </div>
            <div className="font-mono" style={{ fontSize: '1.75rem', fontWeight: 800, color: 'var(--text-main)' }}>
              {totalResellers}
            </div>
          </div>
          <div style={{
            width: '48px',
            height: '48px',
            borderRadius: '14px',
            background: 'rgba(245, 158, 11, 0.15)',
            color: 'var(--brand-accent)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <Users size={24} />
          </div>
        </div>
      </div>

      {/* Grid: System Information (Left) & Recent Accounts (Right) */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'minmax(320px, 1fr) minmax(400px, 1.8fr)',
        gap: '20px',
        alignItems: 'start'
      }}>
        {/* System Information Card */}
        <div className="glass-panel" style={{ padding: '24px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '16px' }}>
            <Server size={18} style={{ color: 'var(--brand-primary)' }} />
            <h3 style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--text-main)' }}>
              {t('systemInformation')}
            </h3>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px', fontSize: '0.84rem' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', paddingBottom: '10px', borderBottom: '1px solid var(--glass-border)' }}>
              <span style={{ color: 'var(--text-muted)' }}>{t('hostnameLabel')}</span>
              <span className="font-mono" style={{ fontWeight: 600 }}>{systemMetrics.hostname}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', paddingBottom: '10px', borderBottom: '1px solid var(--glass-border)' }}>
              <span style={{ color: 'var(--text-muted)' }}>{t('osPlatform')}</span>
              <span style={{ fontWeight: 600 }}>{systemMetrics.os}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', paddingBottom: '10px', borderBottom: '1px solid var(--glass-border)' }}>
              <span style={{ color: 'var(--text-muted)' }}>{t('kernelVersion')}</span>
              <span className="font-mono" style={{ fontSize: '0.78rem' }}>{systemMetrics.kernel}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', paddingBottom: '10px', borderBottom: '1px solid var(--glass-border)' }}>
              <span style={{ color: 'var(--text-muted)' }}>{t('uptime')}</span>
              <span style={{ fontWeight: 600, color: 'var(--status-online)' }}>{systemMetrics.uptime}</span>
            </div>

            {/* CPU & Memory Gauges */}
            <div style={{ marginTop: '4px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '6px' }}>
                <span style={{ color: 'var(--text-muted)' }}>{t('cpuUsage')}</span>
                <span className="font-mono" style={{ fontWeight: 600 }}>{systemMetrics.cpuUsagePercent}%</span>
              </div>
              <div className="progress-bar-container">
                <div 
                  className="progress-bar-fill" 
                  style={{ width: `${systemMetrics.cpuUsagePercent}%`, background: 'var(--brand-primary)' }} 
                />
              </div>
            </div>

            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '6px' }}>
                <span style={{ color: 'var(--text-muted)' }}>{t('memoryUsage')}</span>
                <span className="font-mono" style={{ fontWeight: 600 }}>
                  {Math.round((systemMetrics.memoryUsedMb / systemMetrics.memoryTotalMb) * 100)}% ({systemMetrics.memoryUsedMb} MB / {systemMetrics.memoryTotalMb} MB)
                </span>
              </div>
              <div className="progress-bar-container">
                <div 
                  className="progress-bar-fill" 
                  style={{ 
                    width: `${(systemMetrics.memoryUsedMb / systemMetrics.memoryTotalMb) * 100}%`, 
                    background: 'var(--status-info)' 
                  }} 
                />
              </div>
            </div>

            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '6px' }}>
                <span style={{ color: 'var(--text-muted)' }}>{t('diskUsage')}</span>
                <span className="font-mono" style={{ fontWeight: 600 }}>
                  {Math.round((systemMetrics.diskUsedGb / systemMetrics.diskTotalGb) * 100)}% ({systemMetrics.diskUsedGb} GB / {systemMetrics.diskTotalGb} GB)
                </span>
              </div>
              <div className="progress-bar-container">
                <div 
                  className="progress-bar-fill" 
                  style={{ 
                    width: `${(systemMetrics.diskUsedGb / systemMetrics.diskTotalGb) * 100}%`, 
                    background: 'var(--status-online)' 
                  }} 
                />
              </div>
            </div>
          </div>
        </div>

        {/* Recent Accounts Table */}
        <div className="glass-panel" style={{ padding: '24px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Users size={18} style={{ color: 'var(--brand-primary)' }} />
              <h3 style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--text-main)' }}>
                {t('actionAllAccounts')}
              </h3>
            </div>
            <button 
              onClick={onOpenNewAccount}
              className="btn btn-secondary btn-sm"
            >
              {t('viewAll')}
            </button>
          </div>

          <div style={{ overflowX: 'auto' }}>
            <table className="glass-table">
              <thead>
                <tr>
                  <th>{t('thUsername')}</th>
                  <th>{t('thDomain')}</th>
                  <th>{t('thPlan')}</th>
                  <th>{t('thDisk')}</th>
                  <th>{t('thStatus')}</th>
                  <th style={{ textAlign: 'right' }}>{t('thActions')}</th>
                </tr>
              </thead>
              <tbody>
                {accounts.map(acc => (
                  <tr key={acc.id}>
                    <td>
                      <div style={{ fontWeight: 700, color: 'var(--text-main)' }}>
                        {acc.username}
                      </div>
                      <div className="font-mono" style={{ fontSize: '0.72rem', color: 'var(--text-dim)' }}>
                        {acc.ipAddress}
                      </div>
                    </td>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                        <Globe size={13} style={{ color: 'var(--status-info)' }} />
                        <span style={{ fontWeight: 600 }}>{acc.domain}</span>
                      </div>
                    </td>
                    <td>
                      <span className="badge-pill" style={{ background: 'rgba(148, 163, 184, 0.15)', color: 'var(--text-muted)' }}>
                        {acc.plan}
                      </span>
                    </td>
                    <td className="font-mono" style={{ fontSize: '0.8rem' }}>
                      {acc.diskUsageMb} MB / {acc.diskLimitMb} MB
                    </td>
                    <td>
                      {acc.status === 'active' ? (
                        <span className="badge-online">
                          <span className="service-dot online" />
                          {t('active')}
                        </span>
                      ) : (
                        <span className="badge-warning">
                          <span className="service-dot offline" />
                          {t('suspended')}
                        </span>
                      )}
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      <div style={{ display: 'inline-flex', gap: '6px' }}>
                        <button
                          onClick={() => onSwitchToUser(acc.username)}
                          className="btn btn-secondary btn-sm"
                          title={t('impersonate')}
                          style={{ padding: '4px 8px', fontSize: '0.75rem' }}
                        >
                          <ExternalLink size={12} />
                          <span>{t('impersonate')}</span>
                        </button>
                        <button
                          onClick={() => onToggleSuspend(acc.id)}
                          className="btn btn-secondary btn-sm"
                          style={{
                            padding: '4px 8px',
                            fontSize: '0.75rem',
                            color: acc.status === 'active' ? 'var(--status-danger)' : 'var(--status-online)'
                          }}
                        >
                          {acc.status === 'active' ? 'Suspend' : 'Unsuspend'}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
};
