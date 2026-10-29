// DigiPanel Client Dashboard.
// Every value is read from the authenticated account record and every tile
// opens a capability that exists in this build. Capabilities that are not
// implemented are not shown.

import React, { useState } from 'react';
import {
  Globe,
  Folder,
  Mail,
  ShieldCheck,
  Copy,
  Check,
  FileText,
  Terminal,
  Layers,
  Cpu,
  Receipt,
  HardDrive,
  Network
} from 'lucide-react';
import type { HostingAccount, Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { formatBytes, OpsEmpty } from './opsShared';

interface UserDashboardProps {
  language: Language;
  account: HostingAccount | null;
  onOpenDnsEditor: () => void;
  onOpenFileBrowser: () => void;
  onOpenMailManager: () => void;
  onOpenSslManager: () => void;
  onOpenTerminal: () => void;
  onOpenApps: () => void;
  onOpenLogs: () => void;
  onOpenRuntimes: () => void;
  onOpenBilling: () => void;
}

export const UserDashboard: React.FC<UserDashboardProps> = ({
  language,
  account,
  onOpenDnsEditor,
  onOpenFileBrowser,
  onOpenMailManager,
  onOpenSslManager,
  onOpenTerminal,
  onOpenApps,
  onOpenLogs,
  onOpenRuntimes,
  onOpenBilling
}) => {
  const t = (key: any) => getTranslation(language, key);
  const [copied, setCopied] = useState(false);

  if (!account) {
    return (
      <OpsEmpty message={t('clientNoAccount')}>
        <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginTop: '8px' }}>
          {t('clientNoAccountHint')}
        </p>
      </OpsEmpty>
    );
  }

  const diskUsedBytes = account.diskUsageMb * 1024 * 1024;
  const diskLimitBytes = account.diskLimitMb * 1024 * 1024;
  const bandwidthUsedBytes = account.bandwidthUsageMb * 1024 * 1024;
  const bandwidthLimitBytes = account.bandwidthLimitMb * 1024 * 1024;

  const copyDomain = async () => {
    try {
      await navigator.clipboard.writeText(account.domain);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopied(false);
    }
  };

  const tools = [
    { title: t('toolZoneEditor'), icon: Globe, onClick: onOpenDnsEditor },
    { title: t('toolSslCertificates'), icon: ShieldCheck, onClick: onOpenSslManager },
    { title: t('toolEmailAccounts'), icon: Mail, onClick: onOpenMailManager },
    { title: t('toolFileManager'), icon: Folder, onClick: onOpenFileBrowser },
    { title: t('navAppMarketplace'), icon: Layers, onClick: onOpenApps },
    { title: t('navRuntimes'), icon: Cpu, onClick: onOpenRuntimes },
    { title: t('toolAccessLogs'), icon: FileText, onClick: onOpenLogs },
    { title: t('navBilling'), icon: Receipt, onClick: onOpenBilling },
    { title: t('terminalTitle'), icon: Terminal, onClick: onOpenTerminal }
  ];

  const usageRows = [
    { label: t('diskUsage'), used: diskUsedBytes, limit: diskLimitBytes, colour: 'var(--status-online)' },
    { label: t('bandwidthUsage'), used: bandwidthUsedBytes, limit: bandwidthLimitBytes, colour: 'var(--status-info)' },
    { label: t('inodesUsage'), used: account.inodesCount, limit: account.inodesLimit, colour: 'var(--brand-accent)' }
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '22px' }}>
      <div>
        <h1 style={{ fontSize: '1.5rem', fontWeight: 800, color: 'var(--text-main)', letterSpacing: '-0.02em', marginBottom: '4px' }}>
          {t('generalInformation')}
        </h1>
        <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>{account.username} &middot; {account.plan}</p>
      </div>

      <div className="glass-panel" style={{ padding: '22px 24px' }}>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '18px' }}>
          <div>
            <div style={{ color: 'var(--text-muted)', fontSize: '0.76rem', marginBottom: '2px' }}>{t('thUsername')}</div>
            <div style={{ fontWeight: 700, color: 'var(--text-main)' }}>{account.username}</div>
          </div>
          <div>
            <div style={{ color: 'var(--text-muted)', fontSize: '0.76rem', marginBottom: '2px' }}>{t('thDomain')}</div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontWeight: 700, color: 'var(--text-main)' }}>
              {account.domain}
              <button type="button" onClick={() => void copyDomain()} className="btn-icon" title={t('copy')}>
                {copied ? <Check size={13} style={{ color: 'var(--status-online)' }} /> : <Copy size={13} />}
              </button>
            </div>
          </div>
          <div>
            <div style={{ color: 'var(--text-muted)', fontSize: '0.76rem', marginBottom: '2px' }}>{t('sharedIpAddress')}</div>
            <div className="font-mono" style={{ fontWeight: 600, color: 'var(--text-main)' }}>{account.ipAddress || '-'}</div>
          </div>
          <div>
            <div style={{ color: 'var(--text-muted)', fontSize: '0.76rem', marginBottom: '2px' }}>{t('homeDirectory')}</div>
            <div className="font-mono" style={{ fontWeight: 600, fontSize: '0.8rem', color: 'var(--text-main)' }}>{account.homeDir}</div>
          </div>
          <div>
            <div style={{ color: 'var(--text-muted)', fontSize: '0.76rem', marginBottom: '2px' }}>PHP</div>
            <div className="font-mono" style={{ fontWeight: 600, color: 'var(--text-main)' }}>{account.phpVersion}</div>
          </div>
          <div>
            <div style={{ color: 'var(--text-muted)', fontSize: '0.76rem', marginBottom: '2px' }}>{t('accountsCreated')}</div>
            <div className="font-mono" style={{ fontWeight: 600, color: 'var(--text-main)' }}>{account.createdAt}</div>
          </div>
        </div>
      </div>

      <div className="glass-panel" style={{ padding: '22px 24px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '16px' }}>
          <HardDrive size={17} style={{ color: 'var(--brand-primary)' }} />
          <h2 style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--text-main)' }}>{t('statistics')}</h2>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          {usageRows.map((row) => {
            const ratio = row.limit > 0 ? Math.min(1, row.used / row.limit) : 0;
            const isBytes = row.label !== t('inodesUsage');
            const usedLabel = isBytes ? formatBytes(row.used) : row.used.toLocaleString();
            const limitLabel = isBytes ? formatBytes(row.limit) : row.limit.toLocaleString();
            return (
              <div key={row.label}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
                  <span style={{ fontSize: '0.84rem', fontWeight: 600, color: 'var(--text-main)' }}>{row.label}</span>
                  <span className="font-mono" style={{ fontSize: '0.8rem', fontWeight: 700, color: row.colour }}>
                    {(ratio * 100).toFixed(1)}%
                  </span>
                </div>
                <div className="font-mono" style={{ fontSize: '0.76rem', color: 'var(--text-muted)', marginBottom: '6px' }}>
                  {usedLabel} / {limitLabel}
                </div>
                <div className="progress-bar-container">
                  <div className="progress-bar-fill" style={{ width: `${ratio * 100}%`, background: row.colour }} />
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <div className="glass-panel" style={{ padding: '22px 24px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '14px' }}>
          <Network size={17} style={{ color: 'var(--brand-primary)' }} />
          <h2 style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--text-main)' }}>{t('navDomains')}</h2>
        </div>
        <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '12px' }}>{t('temporaryDomainNotice')}</p>
        <div className="glass-card" style={{ padding: '14px 16px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '10px', flexWrap: 'wrap' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Globe size={15} style={{ color: 'var(--status-info)' }} />
            <span style={{ fontWeight: 700, fontSize: '0.86rem' }}>{account.domain}</span>
            <span className="badge-pill" style={{ background: 'rgba(16, 185, 129, 0.12)', color: 'var(--status-online)' }}>
              {account.status}
            </span>
          </div>
          <button type="button" className="btn btn-secondary btn-sm" onClick={onOpenDnsEditor}>
            {t('toolZoneEditor')}
          </button>
        </div>
      </div>

      <div className="glass-panel" style={{ padding: '22px 24px' }}>
        <h2 style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--text-main)', marginBottom: '14px' }}>{t('toolsDirectoryTitle')}</h2>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(190px, 1fr))', gap: '12px' }}>
          {tools.map((tool) => (
            <button
              key={tool.title}
              type="button"
              onClick={tool.onClick}
              className="glass-card"
              style={{
                padding: '15px 14px',
                display: 'flex',
                alignItems: 'center',
                gap: '11px',
                cursor: 'pointer',
                textAlign: 'left',
                border: '1px solid var(--glass-border)'
              }}
            >
              <div style={{
                width: '34px',
                height: '34px',
                borderRadius: '9px',
                background: 'rgba(234, 88, 12, 0.12)',
                color: 'var(--brand-primary)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0
              }}>
                <tool.icon size={17} />
              </div>
              <span style={{ fontSize: '0.83rem', fontWeight: 600, color: 'var(--text-main)' }}>{tool.title}</span>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
};
