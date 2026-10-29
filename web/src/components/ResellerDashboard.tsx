// DigiPanel Reseller Dashboard (web/src/components/ResellerDashboard.tsx)
// Dedicated multi-tenant reseller operations center:
// Sub-account quotas, resource allocation pools, managed clients table,
// quick actions, and package customization.
// Zero emojis, strictly clean Lucide SVG iconography, WCAG AA compliant.

import React, { useState } from 'react';
import { 
  Users, 
  HardDrive, 
  Activity, 
  UserPlus, 
  ShieldCheck, 
  Lock, 
  Unlock, 
  ExternalLink, 
  Search, 
  Globe, 
  Mail, 
  KeyRound
} from 'lucide-react';
import type { HostingAccount, Language } from '../types';
import { getTranslation } from '../i18n/translations';

interface ResellerDashboardProps {
  accounts: HostingAccount[];
  language: Language;
  onOpenNewAccount: () => void;
  onOpenDnsEditor: () => void;
  onOpenSslManager: () => void;
  onOpenMailManager: () => void;
  onSwitchToUser: (username: string) => void;
  onToggleSuspend: (accountId: string) => void;
}

export const ResellerDashboard: React.FC<ResellerDashboardProps> = ({
  accounts,
  language,
  onOpenNewAccount,
  onOpenDnsEditor,
  onOpenSslManager,
  onOpenMailManager,
  onSwitchToUser,
  onToggleSuspend
}) => {
  const [searchFilter, setSearchFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'suspended'>('all');

  const t = (key: any) => getTranslation(language, key);

  // Reseller Pool Quota Aggregations
  const totalAccountsUsed = accounts.length;
  const maxAccountsAllowed = 25;
  const accountsQuotaPercent = Math.min(100, Math.round((totalAccountsUsed / maxAccountsAllowed) * 100));

  const totalDiskUsedMb = accounts.reduce((acc, curr) => acc + curr.diskUsageMb, 0);
  const totalDiskUsedGb = Math.round((totalDiskUsedMb / 1024) * 10) / 10;
  const maxDiskGb = 100.0;
  const diskQuotaPercent = Math.min(100, Math.round((totalDiskUsedGb / maxDiskGb) * 100));

  const totalBwUsedMb = accounts.reduce((acc, curr) => acc + curr.bandwidthUsageMb, 0);
  const totalBwUsedGb = Math.round((totalBwUsedMb / 1024) * 10) / 10;
  const maxBwGb = 1000.0;
  const bwQuotaPercent = Math.min(100, Math.round((totalBwUsedGb / maxBwGb) * 100));

  const totalDatabases = accounts.reduce((acc, curr) => acc + curr.databasesCount, 0);
  const totalDomains = accounts.reduce((acc, curr) => acc + (curr.domain ? 1 : 0), 0);

  const filteredAccounts = accounts.filter(acc => {
    const matchesSearch = searchFilter === '' ||
      acc.username.toLowerCase().includes(searchFilter.toLowerCase()) ||
      acc.domain.toLowerCase().includes(searchFilter.toLowerCase()) ||
      acc.plan.toLowerCase().includes(searchFilter.toLowerCase());
    const matchesStatus = statusFilter === 'all' || acc.status === statusFilter;
    return matchesSearch && matchesStatus;
  });

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      {/* 1. Header Banner */}
      <div className="glass-panel" style={{
        padding: '24px 28px',
        position: 'relative',
        background: 'var(--glass-surface)',
        borderRadius: '16px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        flexWrap: 'wrap',
        gap: '16px'
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <div style={{
            width: '48px',
            height: '48px',
            borderRadius: '12px',
            background: 'linear-gradient(135deg, rgba(234, 88, 12, 0.2), rgba(249, 115, 22, 0.05))',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: 'var(--brand-primary)',
            border: '1px solid rgba(234, 88, 12, 0.3)'
          }}>
            <Users size={24} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: '1.4rem', fontWeight: 700, color: 'var(--text-main)' }}>
              {t('resellerCenterTitle')}
            </h1>
            <p style={{ margin: '4px 0 0 0', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
              {t('resellerCenterSubtitle')}
            </p>
          </div>
        </div>

        <button 
          onClick={onOpenNewAccount}
          className="btn-primary"
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            padding: '10px 18px',
            borderRadius: '10px',
            fontSize: '0.88rem',
            fontWeight: 600,
            cursor: 'pointer'
          }}
        >
          <UserPlus size={16} />
          <span>{t('createClientBtn')}</span>
        </button>
      </div>

      {/* 2. Quota Usage Resource Cards */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
        gap: '20px'
      }}>
        {/* Accounts Quota Card */}
        <div className="glass-card" style={{ padding: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
            <span style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-muted)' }}>
              {t('resellerAccountsQuota')}
            </span>
            <Users size={18} style={{ color: 'var(--brand-primary)' }} />
          </div>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: '8px', marginBottom: '10px' }}>
            <span style={{ fontSize: '1.8rem', fontWeight: 700, color: 'var(--text-main)' }}>
              {totalAccountsUsed}
            </span>
            <span style={{ fontSize: '0.9rem', color: 'var(--text-dim)' }}>
              / {maxAccountsAllowed} accounts ({accountsQuotaPercent}%)
            </span>
          </div>
          <div className="progress-bar-container" style={{ height: '8px', background: 'var(--sidebar-border)', borderRadius: '4px', overflow: 'hidden' }}>
            <div style={{
              width: `${accountsQuotaPercent}%`,
              height: '100%',
              background: accountsQuotaPercent > 80 ? 'var(--brand-danger)' : 'var(--brand-primary)',
              borderRadius: '4px',
              transition: 'width 0.4s ease'
            }} />
          </div>
        </div>

        {/* Disk Quota Card */}
        <div className="glass-card" style={{ padding: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
            <span style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-muted)' }}>
              {t('resellerDiskQuota')}
            </span>
            <HardDrive size={18} style={{ color: '#0284c7' }} />
          </div>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: '8px', marginBottom: '10px' }}>
            <span style={{ fontSize: '1.8rem', fontWeight: 700, color: 'var(--text-main)' }}>
              {totalDiskUsedGb} GB
            </span>
            <span style={{ fontSize: '0.9rem', color: 'var(--text-dim)' }}>
              / {maxDiskGb} GB ({diskQuotaPercent}%)
            </span>
          </div>
          <div className="progress-bar-container" style={{ height: '8px', background: 'var(--sidebar-border)', borderRadius: '4px', overflow: 'hidden' }}>
            <div style={{
              width: `${diskQuotaPercent}%`,
              height: '100%',
              background: '#0284c7',
              borderRadius: '4px',
              transition: 'width 0.4s ease'
            }} />
          </div>
        </div>

        {/* Bandwidth Quota Card */}
        <div className="glass-card" style={{ padding: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
            <span style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-muted)' }}>
              {t('resellerBwQuota')}
            </span>
            <Activity size={18} style={{ color: '#16a34a' }} />
          </div>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: '8px', marginBottom: '10px' }}>
            <span style={{ fontSize: '1.8rem', fontWeight: 700, color: 'var(--text-main)' }}>
              {totalBwUsedGb} GB
            </span>
            <span style={{ fontSize: '0.9rem', color: 'var(--text-dim)' }}>
              / {maxBwGb} GB ({bwQuotaPercent}%)
            </span>
          </div>
          <div className="progress-bar-container" style={{ height: '8px', background: 'var(--sidebar-border)', borderRadius: '4px', overflow: 'hidden' }}>
            <div style={{
              width: `${bwQuotaPercent}%`,
              height: '100%',
              background: '#16a34a',
              borderRadius: '4px',
              transition: 'width 0.4s ease'
            }} />
          </div>
        </div>
      </div>

      {/* 3. Reseller Quick Action Cards */}
      <div className="glass-panel" style={{ padding: '20px', borderRadius: '16px' }}>
        <h2 style={{ margin: '0 0 16px 0', fontSize: '1rem', fontWeight: 600, color: 'var(--text-main)' }}>
          {t('resellerQuickActions')}
        </h2>
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
          gap: '14px'
        }}>
          <button 
            onClick={onOpenDnsEditor}
            className="glass-card"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '12px',
              padding: '14px 16px',
              border: '1px solid var(--glass-border)',
              borderRadius: '12px',
              background: 'var(--glass-surface)',
              cursor: 'pointer',
              textAlign: 'left'
            }}
          >
            <div style={{ padding: '8px', borderRadius: '8px', background: 'rgba(59, 130, 246, 0.1)', color: '#3b82f6' }}>
              <Globe size={18} />
            </div>
            <div>
              <div style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-main)' }}>{t('modalDnsEditorTitle')}</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{totalDomains} zones registered</div>
            </div>
          </button>

          <button 
            onClick={onOpenSslManager}
            className="glass-card"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '12px',
              padding: '14px 16px',
              border: '1px solid var(--glass-border)',
              borderRadius: '12px',
              background: 'var(--glass-surface)',
              cursor: 'pointer',
              textAlign: 'left'
            }}
          >
            <div style={{ padding: '8px', borderRadius: '8px', background: 'rgba(16, 185, 129, 0.1)', color: '#10b981' }}>
              <ShieldCheck size={18} />
            </div>
            <div>
              <div style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-main)' }}>{t('sslManagerTitle')}</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{t('resellerQuickSslDesc')}</div>
            </div>
          </button>

          <button 
            onClick={onOpenMailManager}
            className="glass-card"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '12px',
              padding: '14px 16px',
              border: '1px solid var(--glass-border)',
              borderRadius: '12px',
              background: 'var(--glass-surface)',
              cursor: 'pointer',
              textAlign: 'left'
            }}
          >
            <div style={{ padding: '8px', borderRadius: '8px', background: 'rgba(234, 88, 12, 0.1)', color: 'var(--brand-primary)' }}>
              <Mail size={18} />
            </div>
            <div>
              <div style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-main)' }}>{t('mailManagerTitle')}</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{t('resellerQuickMailDesc')}</div>
            </div>
          </button>

          <button 
            onClick={onOpenNewAccount}
            className="glass-card"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '12px',
              padding: '14px 16px',
              border: '1px solid var(--glass-border)',
              borderRadius: '12px',
              background: 'var(--glass-surface)',
              cursor: 'pointer',
              textAlign: 'left'
            }}
          >
            <div style={{ padding: '8px', borderRadius: '8px', background: 'rgba(168, 85, 247, 0.1)', color: '#a855f7' }}>
              <KeyRound size={18} />
            </div>
            <div>
              <div style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-main)' }}>{t('resellerPackages')}</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{totalDatabases} databases active</div>
            </div>
          </button>
        </div>
      </div>

      {/* 4. Client Accounts Table */}
      <div className="glass-panel" style={{ padding: '24px', borderRadius: '16px' }}>
        <div style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          flexWrap: 'wrap',
          gap: '12px',
          marginBottom: '20px'
        }}>
          <div>
            <h2 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 700, color: 'var(--text-main)' }}>
              {t('resellerClientAccounts')}
            </h2>
            <p style={{ margin: '4px 0 0 0', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
              Showing {filteredAccounts.length} client accounts
            </p>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            {/* Filter pills */}
            <div style={{
              display: 'flex',
              background: 'var(--sidebar-bg)',
              padding: '3px',
              borderRadius: '8px',
              border: '1px solid var(--sidebar-border)'
            }}>
              {(['all', 'active', 'suspended'] as const).map(s => (
                <button
                  key={s}
                  onClick={() => setStatusFilter(s)}
                  style={{
                    padding: '4px 10px',
                    borderRadius: '6px',
                    border: 'none',
                    fontSize: '0.75rem',
                    fontWeight: 600,
                    cursor: 'pointer',
                    background: statusFilter === s ? 'var(--brand-primary)' : 'transparent',
                    color: statusFilter === s ? '#ffffff' : 'var(--text-muted)',
                    transition: 'all 0.15s ease'
                  }}
                >
                  {s.charAt(0).toUpperCase() + s.slice(1)}
                </button>
              ))}
            </div>

            {/* Search Input */}
            <div style={{ position: 'relative' }}>
              <Search size={14} style={{
                position: 'absolute',
                left: '10px',
                top: '50%',
                transform: 'translateY(-50%)',
                color: 'var(--text-muted)'
              }} />
              <input 
                type="text"
                placeholder={t('searchCriteria')}
                value={searchFilter}
                onChange={(e) => setSearchFilter(e.target.value)}
                className="input-glass"
                style={{
                  paddingLeft: '32px',
                  height: '34px',
                  fontSize: '0.8rem',
                  borderRadius: '8px',
                  width: '220px'
                }}
              />
            </div>
          </div>
        </div>

        {/* Responsive Table */}
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '0.84rem' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--glass-border)', color: 'var(--text-muted)', fontSize: '0.75rem' }}>
                <th style={{ padding: '10px 12px' }}>{t('thUsername')}</th>
                <th style={{ padding: '10px 12px' }}>{t('thDomain')}</th>
                <th style={{ padding: '10px 12px' }}>{t('thPlan')}</th>
                <th style={{ padding: '10px 12px' }}>{t('thDisk')}</th>
                <th style={{ padding: '10px 12px' }}>{t('thStatus')}</th>
                <th style={{ padding: '10px 12px', textAlign: 'right' }}>{t('thActions')}</th>
              </tr>
            </thead>
            <tbody>
              {filteredAccounts.map(account => (
                <tr 
                  key={account.id} 
                  style={{
                    borderBottom: '1px solid var(--glass-border)',
                    transition: 'background 0.15s ease'
                  }}
                >
                  <td style={{ padding: '12px', fontWeight: 600, color: 'var(--text-main)' }}>
                    {account.username}
                  </td>
                  <td style={{ padding: '12px', color: 'var(--text-main)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <Globe size={14} style={{ color: 'var(--brand-primary)' }} />
                      <span>{account.domain}</span>
                    </div>
                  </td>
                  <td style={{ padding: '12px', color: 'var(--text-muted)' }}>
                    {account.plan}
                  </td>
                  <td style={{ padding: '12px' }}>
                    <div style={{ fontSize: '0.8rem', color: 'var(--text-main)', marginBottom: '3px' }}>
                      {account.diskUsageMb} MB / {account.diskLimitMb} MB
                    </div>
                    <div style={{ width: '100px', height: '4px', background: 'var(--sidebar-border)', borderRadius: '2px', overflow: 'hidden' }}>
                      <div style={{
                        width: `${Math.min(100, (account.diskUsageMb / account.diskLimitMb) * 100)}%`,
                        height: '100%',
                        background: 'var(--brand-primary)'
                      }} />
                    </div>
                  </td>
                  <td style={{ padding: '12px' }}>
                    <span 
                      className={`badge-status ${account.status === 'active' ? 'online' : 'suspended'}`}
                      style={{
                        padding: '4px 8px',
                        borderRadius: '6px',
                        fontSize: '0.72rem',
                        fontWeight: 600,
                        textTransform: 'uppercase'
                      }}
                    >
                      {account.status}
                    </span>
                  </td>
                  <td style={{ padding: '12px', textAlign: 'right' }}>
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: '8px' }}>
                      <button
                        onClick={() => onToggleSuspend(account.id)}
                        title={account.status === 'active' ? t('actionSuspendAccount') : t('actionUnsuspendAccount')}
                        style={{
                          padding: '6px',
                          borderRadius: '6px',
                          border: '1px solid var(--glass-border)',
                          background: 'transparent',
                          color: account.status === 'active' ? 'var(--brand-danger)' : '#10b981',
                          cursor: 'pointer'
                        }}
                      >
                        {account.status === 'active' ? <Lock size={14} /> : <Unlock size={14} />}
                      </button>

                      <button
                        onClick={() => onSwitchToUser(account.username)}
                        title="Login as User"
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: '4px',
                          padding: '6px 10px',
                          borderRadius: '6px',
                          border: '1px solid var(--brand-primary)',
                          background: 'rgba(234, 88, 12, 0.08)',
                          color: 'var(--brand-primary)',
                          fontSize: '0.75rem',
                          fontWeight: 600,
                          cursor: 'pointer'
                        }}
                      >
                        <span>{t('impersonate')}</span>
                        <ExternalLink size={12} />
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
  );
};

export default ResellerDashboard;
