// DigiPanel Left Navigation Sidebar
// Zero emojis, strictly clean Lucide SVG iconography, modern glass styling

import React, { useState } from 'react';
import { 
  Home, 
  Globe, 
  Layers, 
  Folder, 
  Mail, 
  ListFilter, 
  BarChart3, 
  Shield, 
  Cpu, 
  Terminal, 
  Search, 
  ChevronRight, 
  ChevronDown, 
  Users, 
  Server, 
  FileText,
  Activity,
  Network,
  Boxes
} from 'lucide-react';
import type { UserRole, Language } from '../types';
import { getTranslation } from '../i18n/translations';

interface SidebarProps {
  currentRole: UserRole;
  activeView: string;
  onViewChange: (view: string) => void;
  language: Language;
  onSelectCategory?: (category: string) => void;
  selectedCategory?: string;
  hostname?: string;
  kernel?: string;
  panelVersion?: string;
}

export const Sidebar: React.FC<SidebarProps> = ({
  currentRole,
  activeView,
  onViewChange,
  language,
  onSelectCategory,
  selectedCategory = 'all',
  hostname = '',
  kernel = '',
  panelVersion = ''
}) => {
  const [sidebarSearch, setSidebarSearch] = useState('');
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({
    home: true,
    domains: true,
    hosting: true,
    cluster: true,
    server: true,
    files: false,
    databases: false,
    emails: false
  });

  const t = (key: any) => getTranslation(language, key);

  const toggleSection = (sec: string) => {
    setExpandedSections(prev => ({ ...prev, [sec]: !prev[sec] }));
  };

  const adminMenuGroups = [
    {
      id: 'home',
      label: t('navHome'),
      icon: Home,
      items: [
        { id: 'dashboard', label: t('navDashboard'), icon: Activity },
        { id: 'metrics', label: t('navMetrics'), icon: BarChart3 },
      ]
    },
    {
      id: 'cluster',
      label: t('navCluster'),
      icon: Network,
      items: [
        { id: 'cluster', label: t('navHubDashboard'), icon: Server },
        { id: 'dns_cluster', label: t('navDnsCluster'), icon: Globe },
      ]
    },
    {
      id: 'reseller',
      label: t('navResellerCenter'),
      icon: Users,
      items: [
        { id: 'resellers_list', label: t('actionResellers'), icon: Users },
      ]
    },
    {
      id: 'hosting',
      label: t('navHostingServices'),
      icon: Layers,
      items: [
        { id: 'accounts_list', label: t('actionAllAccounts'), icon: Users },
        { id: 'plans', label: t('thPlan'), icon: ListFilter },
      ]
    },
    {
      id: 'server',
      label: t('navWebServer'),
      icon: Server,
      items: [
        { id: 'server_status', label: t('navServerStatus'), icon: Activity },
        { id: 'access_logs', label: t('navAccessLogs'), icon: FileText },
        { id: 'audit', label: t('auditTitle'), icon: Shield },
      ]
    },
    {
      id: 'runtimes',
      label: 'Runtimes & Apps',
      icon: Cpu,
      items: [
        { id: 'runtimes', label: 'Node, Python, Go, PHP', icon: Cpu },
        { id: 'apps', label: t('navAppMarketplace'), icon: Layers },
      ]
    },
    {
      id: 'billing',
      label: t('navBilling'),
      icon: FileText,
      items: [
        { id: 'billing', label: t('navBilling'), icon: FileText },
      ]
    }
  ];

  const resellerMenuGroups = [
    {
      id: 'reseller_home',
      label: t('navHome'),
      icon: Home,
      items: [
        { id: 'dashboard', label: t('navDashboard'), icon: Activity },
      ]
    },
    {
      id: 'reseller_accounts',
      label: t('resellerClientAccounts'),
      icon: Users,
      items: [
        { id: 'accounts_list', label: t('actionAllAccounts'), icon: Users },
      ]
    },
    {
      id: 'reseller_services',
      label: t('navHostingServices'),
      icon: Layers,
      items: [
        { id: 'dns', label: t('modalDnsEditorTitle'), icon: Globe },
        { id: 'ssl', label: t('sslManagerTitle'), icon: Shield },
        { id: 'mail', label: t('mailManagerTitle'), icon: Mail },
        { id: 'apps', label: t('navAppMarketplace'), icon: Boxes },
      ]
    },
    {
      id: 'reseller_billing',
      label: t('navBilling'),
      icon: FileText,
      items: [
        { id: 'billing', label: t('navBilling'), icon: FileText },
      ]
    }
  ];

  // Client menu entries all resolve to a capability that exists in this build.
  const userMenuItems = [
    { id: 'home', label: t('navHome'), icon: Home, target: 'home' },
    { id: 'domains', label: t('navDomains'), icon: Globe, target: 'domains' },
    { id: 'files', label: t('navFiles'), icon: Folder, target: 'files' },
    { id: 'emails', label: t('navEmails'), icon: Mail, target: 'emails' },
    { id: 'apps', label: t('navAppMarketplace'), icon: Boxes, target: 'apps' },
    { id: 'runtimes', label: t('navRuntimes'), icon: Cpu, target: 'runtimes' },
    { id: 'metrics', label: t('navMetrics'), icon: BarChart3, target: 'server_status' },
    { id: 'security', label: t('navSecurity'), icon: Shield, target: 'security' },
    { id: 'advanced', label: t('navAdvanced'), icon: Terminal, target: 'advanced' },
  ];

  return (
    <aside style={{
      width: '260px',
      flexShrink: 0,
      background: 'var(--sidebar-bg)',
      borderRight: '1px solid var(--sidebar-border)',
      backdropFilter: 'blur(16px)',
      WebkitBackdropFilter: 'blur(16px)',
      display: 'flex',
      flexDirection: 'column',
      minHeight: 'calc(100vh - 68px)',
      position: 'sticky',
      top: '68px',
      height: 'calc(100vh - 68px)',
      overflowY: 'auto'
    }}>
      {/* Search in menu input as in AdminBolt / cPanel */}
      <div style={{ padding: '16px 16px 12px 16px' }}>
        <div style={{ position: 'relative' }}>
          <Search size={14} style={{
            position: 'absolute',
            left: '12px',
            top: '50%',
            transform: 'translateY(-50%)',
            color: 'var(--text-muted)'
          }} />
          <input 
            type="text"
            placeholder={t('searchPlaceholder').split('(')[0].trim()}
            value={sidebarSearch}
            onChange={(e) => setSidebarSearch(e.target.value)}
            className="input-glass"
            style={{
              paddingLeft: '34px',
              height: '36px',
              fontSize: '0.8rem',
              borderRadius: '8px'
            }}
          />
        </div>
      </div>

      {/* Navigation Content */}
      <div style={{ flex: 1, padding: '0 8px 16px 8px' }}>
        {currentRole === 'admin' || currentRole === 'reseller' ? (
          // Admin & Reseller Multi-Tenant Menu
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            {(currentRole === 'admin' ? adminMenuGroups : resellerMenuGroups).map(group => {
              const matchesSearch = sidebarSearch === '' || 
                group.label.toLowerCase().includes(sidebarSearch.toLowerCase()) ||
                group.items.some(item => item.label.toLowerCase().includes(sidebarSearch.toLowerCase()));

              if (!matchesSearch) return null;

              const isExpanded = expandedSections[group.id] ?? true;

              return (
                <div key={group.id} style={{ marginBottom: '4px' }}>
                  <button
                    onClick={() => toggleSection(group.id)}
                    style={{
                      width: '100%',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '8px 12px',
                      borderRadius: '8px',
                      background: 'transparent',
                      border: 'none',
                      color: 'var(--text-muted)',
                      fontSize: '0.82rem',
                      fontWeight: 600,
                      cursor: 'pointer',
                      textAlign: 'left',
                      transition: 'color 0.15s ease'
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                      <group.icon size={16} />
                      <span>{group.label}</span>
                    </div>
                    {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                  </button>

                  {isExpanded && (
                    <div style={{ paddingLeft: '16px', display: 'flex', flexDirection: 'column', gap: '2px', marginTop: '2px' }}>
                      {group.items.map(item => {
                        const isActive = activeView === item.id;
                        return (
                          <button
                            key={item.id}
                            onClick={() => onViewChange(item.id)}
                            style={{
                              width: '100%',
                              display: 'flex',
                              alignItems: 'center',
                              gap: '10px',
                              padding: '7px 12px',
                              borderRadius: '6px',
                              border: 'none',
                              background: isActive ? 'rgba(234, 88, 12, 0.12)' : 'transparent',
                              color: isActive ? 'var(--brand-primary)' : 'var(--text-main)',
                              fontWeight: isActive ? 700 : 500,
                              fontSize: '0.82rem',
                              cursor: 'pointer',
                              textAlign: 'left',
                              borderLeft: isActive ? '3px solid var(--brand-primary)' : '3px solid transparent',
                              transition: 'all 0.15s ease'
                            }}
                          >
                            <item.icon size={14} />
                            <span>{item.label}</span>
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        ) : (
          // Client / User Panel Menu (Matching Image 1)
          <div style={{ display: 'flex', flexDirection: 'column', gap: '3px' }}>
            {userMenuItems
              .filter(item => sidebarSearch === '' || item.label.toLowerCase().includes(sidebarSearch.toLowerCase()))
              .map(item => {
                const isActive = activeView === item.target || (item.target === 'domains' && activeView === 'home' && selectedCategory === 'domains');

                return (
                  <button
                    key={item.id}
                    onClick={() => {
                      onSelectCategory?.(item.id === 'home' ? 'all' : item.id);
                      onViewChange(item.target);
                    }}
                    style={{
                      width: '100%',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '8px 12px',
                      borderRadius: '8px',
                      border: 'none',
                      background: isActive ? 'rgba(234, 88, 12, 0.12)' : 'transparent',
                      color: isActive ? 'var(--brand-primary)' : 'var(--text-main)',
                      fontWeight: isActive ? 700 : 500,
                      fontSize: '0.84rem',
                      cursor: 'pointer',
                      textAlign: 'left',
                      borderLeft: isActive ? '3px solid var(--brand-primary)' : '3px solid transparent',
                      transition: 'all 0.15s ease'
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                      <item.icon size={16} />
                      <span>{item.label}</span>
                    </div>
                    {item.id !== 'home' && (
                      <ChevronRight size={13} style={{ color: 'var(--text-dim)' }} />
                    )}
                  </button>
                );
              })}
          </div>
        )}
      </div>

      {/* Footer Info Pill */}
      <div style={{
        padding: '14px 16px',
        borderTop: '1px solid var(--glass-border)',
        fontSize: '0.72rem',
        color: 'var(--text-muted)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: '8px'
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', minWidth: 0 }}>
          <span className={`service-dot ${panelVersion ? 'online' : 'offline'}`} />
          <span className="font-mono" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {panelVersion ? `v${panelVersion}` : 'API offline'}
          </span>
        </div>
        <span className="font-mono" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={`${hostname} ${kernel}`}>
          {kernel || hostname || '-'}
        </span>
      </div>
    </aside>
  );
};
