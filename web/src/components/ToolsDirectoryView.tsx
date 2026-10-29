// DigiPanel Tools Directory View (Matching Image 4 - cPanel Tools Layout)
// Zero emojis, strictly Lucide SVG iconography, modern glass styling

import React, { useState } from 'react';
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
  ChevronUp
} from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';

interface ToolsDirectoryViewProps {
  language: Language;
  onOpenDns: () => void;
  onOpenFiles: () => void;
  onOpenTerminal: () => void;
  searchFilter?: string;
}

export const ToolsDirectoryView: React.FC<ToolsDirectoryViewProps> = ({
  language,
  onOpenDns,
  onOpenFiles,
  onOpenTerminal,
  searchFilter = ''
}) => {
  const t = (key: any) => getTranslation(language, key);

  const [filterText, setFilterText] = useState(searchFilter);
  const [collapsedCategories, setCollapsedCategories] = useState<Record<string, boolean>>({});

  const toggleCategory = (cat: string) => {
    setCollapsedCategories(prev => ({ ...prev, [cat]: !prev[cat] }));
  };

  const toolSections = [
    {
      id: 'email',
      title: 'Email',
      icon: Mail,
      tools: [
        { name: t('toolEmailAccounts'), action: () => {} },
        { name: t('toolForwarders'), action: () => {} },
        { name: t('toolEmailRouting'), action: () => {} },
        { name: t('toolAutoresponders'), action: () => {} },
        { name: t('toolDefaultAddress'), action: () => {} },
        { name: 'Mailing Lists', action: () => {} },
        { name: t('toolTrackDelivery'), action: () => {} },
        { name: t('toolGlobalEmailFilters'), action: () => {} },
        { name: 'Email Filters', action: () => {} },
        { name: t('toolEmailDeliverability'), action: () => {} },
        { name: t('toolAddressImporter'), action: () => {} },
        { name: t('toolSpamFilters'), action: () => {} },
        { name: t('toolEncryption'), action: () => {} },
        { name: t('toolBoxTrapper'), action: () => {} },
        { name: t('toolCalendarContacts'), action: () => {} },
        { name: t('toolEmailDiskUsage'), action: () => {} }
      ]
    },
    {
      id: 'files',
      title: 'Files',
      icon: Folder,
      tools: [
        { name: t('toolFileManager'), action: onOpenFiles },
        { name: t('toolImages'), action: onOpenFiles },
        { name: t('toolDirectoryPrivacy'), action: () => {} },
        { name: t('toolDiskUsage'), action: () => {} },
        { name: t('toolWebDisk'), action: () => {} },
        { name: t('toolBackup'), action: () => {} },
        { name: t('toolBackupWizard'), action: () => {} },
        { name: t('toolGitVersionControl'), action: onOpenTerminal },
        { name: t('toolFileRestoration'), action: () => {} }
      ]
    },
    {
      id: 'databases',
      title: 'Databases',
      icon: Database,
      tools: [
        { name: t('toolPhpMyAdmin'), action: () => {} },
        { name: 'MySQL Databases', action: () => {} },
        { name: 'MySQL Database Wizard', action: () => {} },
        { name: 'Remote MySQL', action: () => {} },
        { name: t('toolPostgresManager'), action: () => {} },
        { name: 'PostgreSQL Database Wizard', action: () => {} }
      ]
    },
    {
      id: 'domains',
      title: 'Domains',
      icon: Globe,
      tools: [
        { name: t('toolDomains'), action: onOpenDns },
        { name: 'Subdomains', action: onOpenDns },
        { name: 'Aliases', action: onOpenDns },
        { name: t('toolRedirects'), action: onOpenDns },
        { name: t('toolZoneEditor'), action: onOpenDns },
        { name: t('toolCloudflare'), action: onOpenDns },
        { name: t('toolDnsHealth'), action: onOpenDns }
      ]
    },
    {
      id: 'metrics',
      title: 'Metrics',
      icon: BarChart3,
      tools: [
        { name: 'Visitors Log', action: () => {} },
        { name: 'HTTP Errors', action: () => {} },
        { name: 'Bandwidth Breakdown', action: () => {} },
        { name: 'Raw Access Logs', action: onOpenTerminal },
        { name: 'Awstats Web Analytics', action: () => {} },
        { name: 'CPU and Concurrent Connection Usage', action: () => {} }
      ]
    },
    {
      id: 'security',
      title: 'Security',
      icon: Shield,
      tools: [
        { name: t('toolSshAccess'), action: onOpenTerminal },
        { name: t('toolIpBlocker'), action: () => {} },
        { name: t('toolSslCertificates'), action: onOpenDns },
        { name: 'Manage API Tokens', action: () => {} },
        { name: 'Hotlink Protection', action: () => {} },
        { name: t('toolModSecurity'), action: () => {} },
        { name: t('toolTwoFactor'), action: () => {} }
      ]
    },
    {
      id: 'software',
      title: 'Software',
      icon: Cpu,
      tools: [
        { name: 'WordPress Toolkit', action: () => {} },
        { name: t('toolPhpSelector'), action: () => {} },
        { name: t('toolNodeManager'), action: () => {} },
        { name: 'Python App Setup', action: () => {} },
        { name: 'Optimize Website (Gzip / Brotli)', action: () => {} }
      ]
    },
    {
      id: 'advanced',
      title: 'Advanced',
      icon: Terminal,
      tools: [
        { name: t('navTerminal'), action: onOpenTerminal },
        { name: t('toolCronJobs'), action: () => {} },
        { name: 'Track DNS Resolution', action: onOpenDns },
        { name: 'Apache & Nginx Handlers', action: () => {} },
        { name: 'MIME Types Configuration', action: () => {} }
      ]
    }
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      {/* Title & Filter */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px' }}>
        <div>
          <h1 style={{ fontSize: '1.6rem', fontWeight: 800, color: 'var(--text-main)', letterSpacing: '-0.02em', marginBottom: '4px' }}>
            Tools Directory
          </h1>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
            Categorized index of all server capabilities, web software runtimes, and security modules
          </p>
        </div>

        <div style={{ width: '320px', position: 'relative' }}>
          <Search size={16} style={{ position: 'absolute', left: '12px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
          <input 
            type="text"
            placeholder="Filter tools..."
            value={filterText}
            onChange={(e) => setFilterText(e.target.value)}
            className="input-glass"
            style={{ paddingLeft: '38px', height: '38px' }}
          />
        </div>
      </div>

      {/* Sections List */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>
        {toolSections.map(sec => {
          const matchingTools = sec.tools.filter(t => 
            filterText === '' || t.name.toLowerCase().includes(filterText.toLowerCase())
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
                  {matchingTools.map((tool, idx) => (
                    <button
                      key={idx}
                      onClick={tool.action}
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
