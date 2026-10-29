// DigiPanel Tools Directory.
// Every entry below opens a capability that is implemented in this build and
// backed by the core API. Tools that do not exist yet are not listed, so no
// button on this screen is inert.

import React, { useMemo, useState } from 'react';
import {
  Mail,
  Folder,
  Database,
  Globe,
  BarChart3,
  Shield,
  Cpu,
  Terminal,
  Search,
  ChevronDown,
  ChevronUp,
  Layers,
  FileText,
  ClipboardList
} from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';

export type ToolsTarget =
  | 'dns'
  | 'ssl'
  | 'mail'
  | 'files'
  | 'apps'
  | 'runtimes'
  | 'access_logs'
  | 'billing'
  | 'server_status'
  | 'accounts_list'
  | 'plans'
  | 'audit'
  | 'terminal';

interface ToolsDirectoryViewProps {
  language: Language;
  onNavigate: (target: ToolsTarget) => void;
  searchFilter?: string;
}

interface ToolEntry {
  name: string;
  target: ToolsTarget;
}

export const ToolsDirectoryView: React.FC<ToolsDirectoryViewProps> = ({
  language,
  onNavigate,
  searchFilter = ''
}) => {
  const t = (key: any) => getTranslation(language, key);

  const [filterText, setFilterText] = useState(searchFilter);
  const [collapsedCategories, setCollapsedCategories] = useState<Record<string, boolean>>({});

  const toggleCategory = (cat: string) => {
    setCollapsedCategories((prev) => ({ ...prev, [cat]: !prev[cat] }));
  };

  const toolSections = useMemo<Array<{ id: string; title: string; icon: typeof Mail; tools: ToolEntry[] }>>(
    () => [
      {
        id: 'domains',
        title: t('catDomains'),
        icon: Globe,
        tools: [
          { name: t('toolZoneEditor'), target: 'dns' },
          { name: t('toolSslCertificates'), target: 'ssl' }
        ]
      },
      {
        id: 'files',
        title: t('catFiles'),
        icon: Folder,
        tools: [
          { name: t('toolFileManager'), target: 'files' },
          { name: t('toolFileRestoration'), target: 'files' }
        ]
      },
      {
        id: 'databases',
        title: t('catDatabases'),
        icon: Database,
        tools: [
          { name: t('toolMysqlDatabases'), target: 'runtimes' },
          { name: t('toolPostgresManager'), target: 'runtimes' }
        ]
      },
      {
        id: 'email',
        title: t('catEmail'),
        icon: Mail,
        tools: [
          { name: t('toolEmailAccounts'), target: 'mail' },
          { name: t('toolForwarders'), target: 'mail' },
          { name: t('toolSpamFilters'), target: 'mail' }
        ]
      },
      {
        id: 'metrics',
        title: t('catMetrics'),
        icon: BarChart3,
        tools: [
          { name: t('toolRawAccessLogs'), target: 'access_logs' },
          { name: t('toolHttpErrors'), target: 'access_logs' },
          { name: t('toolCpuUsage'), target: 'server_status' }
        ]
      },
      {
        id: 'security',
        title: t('catSecurity'),
        icon: Shield,
        tools: [
          { name: t('auditTitle'), target: 'audit' },
          { name: t('toolSslCertificates'), target: 'ssl' },
          { name: t('toolSshAccess'), target: 'terminal' }
        ]
      },
      {
        id: 'software',
        title: t('catSoftware'),
        icon: Cpu,
        tools: [
          { name: t('navAppMarketplace'), target: 'apps' },
          { name: t('navRuntimes'), target: 'runtimes' },
          { name: t('toolPhpSelector'), target: 'runtimes' }
        ]
      },
      {
        id: 'advanced',
        title: t('catAdvanced'),
        icon: Terminal,
        tools: [
          { name: t('navTerminal'), target: 'terminal' }
        ]
      },
      {
        id: 'panel',
        title: t('toolsPanelSection'),
        icon: Layers,
        tools: [
          { name: t('actionAllAccounts'), target: 'accounts_list' },
          { name: t('thPlan'), target: 'plans' },
          { name: t('navBilling'), target: 'billing' },
          { name: t('navHubDashboard'), target: 'server_status' }
        ]
      },
      {
        id: 'logs',
        title: t('catLogs'),
        icon: ClipboardList,
        tools: [
          { name: t('navAccessLogs'), target: 'access_logs' },
          { name: t('auditTitle'), target: 'audit' }
        ]
      },
      {
        id: 'billing',
        title: t('navBilling'),
        icon: FileText,
        tools: [
          { name: t('billingTabInvoices'), target: 'billing' },
          { name: t('billingTabLedger'), target: 'billing' }
        ]
      }
    ],
    [language]
  );

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px' }}>
        <div>
          <h1 style={{ fontSize: '1.6rem', fontWeight: 800, color: 'var(--text-main)', letterSpacing: '-0.02em', marginBottom: '4px' }}>
            {t('toolsDirectoryTitle')}
          </h1>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
            {t('toolsDirectorySubtitle')}
          </p>
        </div>

        <div style={{ width: '320px', position: 'relative' }}>
          <Search size={16} style={{ position: 'absolute', left: '12px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
          <input
            type="text"
            placeholder={t('toolsFilterPh')}
            value={filterText}
            onChange={(e) => setFilterText(e.target.value)}
            className="input-glass"
            style={{ paddingLeft: '38px', height: '38px' }}
          />
        </div>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>
        {toolSections.map((sec) => {
          const matchingTools = sec.tools.filter(
            (tool) => filterText === '' || tool.name.toLowerCase().includes(filterText.toLowerCase())
          );

          if (matchingTools.length === 0) return null;

          const isCollapsed = collapsedCategories[sec.id] || false;

          return (
            <div key={sec.id} className="glass-panel" style={{ padding: '20px 24px' }}>
              <div
                onClick={() => toggleCategory(sec.id)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  cursor: 'pointer',
                  marginBottom: isCollapsed ? 0 : '16px'
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                  <sec.icon size={18} style={{ color: 'var(--brand-primary)' }} />
                  <h3 style={{ fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-main)' }}>
                    {sec.title}
                  </h3>
                  <span className="badge-pill" style={{ background: 'rgba(148, 163, 184, 0.12)', color: 'var(--text-muted)' }}>
                    {matchingTools.length}
                  </span>
                </div>
                {isCollapsed ? <ChevronDown size={18} /> : <ChevronUp size={18} />}
              </div>

              {!isCollapsed && (
                <div style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fill, minmax(210px, 1fr))',
                  gap: '12px'
                }}>
                  {matchingTools.map((tool) => (
                    <button
                      key={`${sec.id}-${tool.name}`}
                      type="button"
                      onClick={() => onNavigate(tool.target)}
                      className="glass-card"
                      style={{
                        padding: '12px 14px',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '10px',
                        border: '1px solid var(--glass-border)',
                        background: 'var(--glass-surface)',
                        cursor: 'pointer',
                        textAlign: 'left',
                        color: 'var(--text-main)'
                      }}
                    >
                      <sec.icon size={15} style={{ color: 'var(--brand-accent)', flexShrink: 0 }} />
                      <span style={{ fontSize: '0.82rem', fontWeight: 600 }}>
                        {tool.name}
                      </span>
                    </button>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};
