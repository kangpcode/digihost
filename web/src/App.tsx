// DigiPanel Master Web Application Shell
// Zero emojis, strictly Lucide SVG iconography, modern glass architecture

import React, { useState, useEffect } from 'react';
import { Header } from './components/Header';
import { Sidebar } from './components/Sidebar';
import { AdminDashboard } from './components/AdminDashboard';
import { ClusterFleetDashboard } from './components/ClusterFleetDashboard';
import { UserDashboard } from './components/UserDashboard';
import { ToolsDirectoryView } from './components/ToolsDirectoryView';
import { BillingLedgerView } from './components/BillingLedgerView';
import { DnsZoneEditorModal } from './components/DnsZoneEditorModal';
import { NewAccountModal } from './components/NewAccountModal';
import { FileManagerModal } from './components/FileManagerModal';
import { WhatsAppModal } from './components/WhatsAppModal';
import { TerminalModal } from './components/TerminalModal';
import type { UserRole, Language, ThemeMode, HostingAccount, ClusterNode, SystemMetrics } from './types';

export const App: React.FC = () => {
  // Theme and Persona State
  const [theme, setTheme] = useState<ThemeMode>('light');
  const [currentRole, setCurrentRole] = useState<UserRole>('admin');
  const [activeView, setActiveView] = useState<string>('dashboard');
  const [language, setLanguage] = useState<Language>('en');
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedUserCategory, setSelectedUserCategory] = useState('all');

  // Interactive Modals State
  const [isDnsEditorOpen, setIsDnsEditorOpen] = useState(false);
  const [isNewAccountOpen, setIsNewAccountOpen] = useState(false);
  const [isFileManagerOpen, setIsFileManagerOpen] = useState(false);
  const [isWhatsAppOpen, setIsWhatsAppOpen] = useState(false);
  const [isTerminalOpen, setIsTerminalOpen] = useState(false);

  // Apply theme to document element
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
  }, [theme]);

  // Keyboard shortcut listener for search '/'
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === '/' && (e.target as HTMLElement).tagName !== 'INPUT' && (e.target as HTMLElement).tagName !== 'TEXTAREA') {
        e.preventDefault();
        const searchInput = document.querySelector('input[type="text"]') as HTMLInputElement;
        searchInput?.focus();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  // System Metrics Telemetry Data
  const [systemMetrics] = useState<SystemMetrics>({
    hostname: 'panel.digihost.id',
    kernel: 'Linux 6.8.0-45-generic x86_64',
    os: 'Ubuntu 24.04 LTS (Noble Numbat)',
    uptime: '42 days, 6 hours, 18 mins',
    loadAverage: [0.12, 0.08, 0.05],
    cpuUsagePercent: 4.2,
    memoryUsedMb: 1842,
    memoryTotalMb: 16384,
    diskUsedGb: 28.4,
    diskTotalGb: 500,
    activeProcesses: 142
  });

  // Hosting Accounts Data
  const [accounts, setAccounts] = useState<HostingAccount[]>([
    {
      id: 'acc-1',
      username: 'testuser',
      domain: 'testuser.digipanel.live',
      plan: 'Starter Cloud',
      diskUsageMb: 0.06,
      diskLimitMb: 50,
      bandwidthUsageMb: 1420,
      bandwidthLimitMb: 51200,
      databasesCount: 2,
      databasesLimit: 10,
      emailCount: 4,
      emailLimit: 20,
      inodesCount: 3727,
      inodesLimit: 100000,
      status: 'active',
      phpVersion: '8.3',
      ipAddress: '103.189.201.42',
      createdAt: '2026-10-15',
      sslActive: true
    },
    {
      id: 'acc-2',
      username: 'demoadmin',
      domain: 'demoadmin.directadmin.com',
      plan: 'Pro Developer',
      diskUsageMb: 141.2,
      diskLimitMb: 5120,
      bandwidthUsageMb: 8400,
      bandwidthLimitMb: 102400,
      databasesCount: 4,
      databasesLimit: 25,
      emailCount: 8,
      emailLimit: 50,
      inodesCount: 14200,
      inodesLimit: 250000,
      status: 'active',
      phpVersion: '8.2',
      ipAddress: '103.189.201.43',
      createdAt: '2026-09-20',
      sslActive: true
    },
    {
      id: 'acc-3',
      username: 'demoreseller',
      domain: 'demoreseller.directadmin.com',
      plan: 'Reseller Tier 1',
      diskUsageMb: 38.3,
      diskLimitMb: 10240,
      bandwidthUsageMb: 2400,
      bandwidthLimitMb: 204800,
      databasesCount: 1,
      databasesLimit: 100,
      emailCount: 2,
      emailLimit: 100,
      inodesCount: 8900,
      inodesLimit: 500000,
      status: 'active',
      phpVersion: '8.3',
      ipAddress: '103.189.201.44',
      createdAt: '2026-08-11',
      sslActive: true
    },
    {
      id: 'acc-4',
      username: 'infogest',
      domain: 'demo.com',
      plan: 'Starter Cloud',
      diskUsageMb: 0.8,
      diskLimitMb: 1000,
      bandwidthUsageMb: 195,
      bandwidthLimitMb: 10240,
      databasesCount: 0,
      databasesLimit: 5,
      emailCount: 1,
      emailLimit: 10,
      inodesCount: 520,
      inodesLimit: 50000,
      status: 'active',
      phpVersion: '8.1',
      ipAddress: '103.189.201.45',
      createdAt: '2026-10-24',
      sslActive: false
    }
  ]);

  // Cluster Fleet Nodes Data (Matching Image 3)
  const [clusterNodes, setClusterNodes] = useState<ClusterNode[]>([
    {
      id: 'node-hub',
      name: 'Production Fleet',
      hostname: 'hub-master.id',
      ipAddress: '45.76.95.149',
      role: 'hub',
      status: 'online',
      panelVersion: 'v1.2.0',
      cpuPercent: 2,
      ramPercent: 30,
      diskPercent: 3,
      inventory: { accounts: 24, domains: 38, databases: 19, resellers: 4 },
      services: {
        web: true,
        ftp: true,
        dns: true,
        db: true,
        fail2ban: true,
        nftables: true,
        securebox: true,
        postfix: true,
        dovecot: true,
        rspamd: true,
        mlmmj: true
      }
    },
    {
      id: 'node-db',
      name: 'db-01.fra',
      hostname: 'db-01.fra.digihost.id',
      ipAddress: '192.248.183.12',
      role: 'database',
      status: 'online',
      panelVersion: 'v1.2.0',
      cpuPercent: 3,
      ramPercent: 29,
      diskPercent: 3,
      inventory: { accounts: 0, domains: 0, databases: 19, resellers: 0 },
      services: {
        web: true,
        ftp: true,
        dns: true,
        db: true,
        fail2ban: true,
        nftables: true,
        securebox: true,
        postfix: true,
        dovecot: true,
        rspamd: true,
        mlmmj: true
      }
    },
    {
      id: 'node-edge',
      name: 'web-edge-01.sin',
      hostname: 'web-edge-01.sin.digihost.id',
      ipAddress: '139.180.211.88',
      role: 'edge',
      status: 'online',
      panelVersion: 'v1.2.0',
      cpuPercent: 5,
      ramPercent: 32,
      diskPercent: 7,
      inventory: { accounts: 12, domains: 18, databases: 0, resellers: 0 },
      services: {
        web: true,
        ftp: true,
        dns: true,
        db: false,
        fail2ban: true,
        nftables: true,
        securebox: true,
        postfix: true,
        dovecot: true,
        rspamd: true,
        mlmmj: false
      }
    }
  ]);

  // Account Operations
  const handleAccountCreated = (newAcc: HostingAccount) => {
    setAccounts([newAcc, ...accounts]);
  };

  const handleToggleSuspend = (accountId: string) => {
    setAccounts(accounts.map(acc => {
      if (acc.id === accountId) {
        return {
          ...acc,
          status: acc.status === 'active' ? 'suspended' : 'active'
        };
      }
      return acc;
    }));
  };

  const handleSwitchToUser = (username: string) => {
    setCurrentRole('user');
    setActiveView('home');
  };

  const handleLoginNode = (nodeId: string) => {
    setIsTerminalOpen(true);
  };

  return (
    <div className="app-container">
      {/* Navigation Sidebar */}
      <Sidebar 
        currentRole={currentRole}
        activeView={activeView}
        onViewChange={(v) => setActiveView(v)}
        language={language}
        onSelectCategory={(cat) => setSelectedUserCategory(cat)}
        selectedCategory={selectedUserCategory}
      />

      {/* Main Content Area */}
      <div className="app-main">
        <Header 
          currentRole={currentRole}
          onRoleChange={(r) => {
            setCurrentRole(r);
            if (r === 'admin') setActiveView('dashboard');
            else setActiveView('home');
          }}
          activeView={activeView}
          onViewChange={(v) => setActiveView(v)}
          language={language}
          onLanguageChange={(l) => setLanguage(l)}
          theme={theme}
          onThemeToggle={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
          searchQuery={searchQuery}
          onSearchChange={(q) => setSearchQuery(q)}
          onOpenTerminal={() => setIsTerminalOpen(true)}
        />

        <main className="content-body">
          {/* Admin Dashboard (Image 2) */}
          {currentRole === 'admin' && activeView === 'dashboard' && (
            <AdminDashboard 
              accounts={accounts}
              systemMetrics={systemMetrics}
              language={language}
              onOpenNewAccount={() => setIsNewAccountOpen(true)}
              onOpenDnsEditor={() => setIsDnsEditorOpen(true)}
              onSwitchToUser={handleSwitchToUser}
              onToggleSuspend={handleToggleSuspend}
              onViewCluster={() => setActiveView('cluster')}
            />
          )}

          {/* Cluster Fleet Dashboard (Image 3) */}
          {currentRole === 'admin' && activeView === 'cluster' && (
            <ClusterFleetDashboard 
              nodes={clusterNodes}
              language={language}
              onConnectNode={() => setIsTerminalOpen(true)}
              onRefreshFleet={() => {
                setClusterNodes([...clusterNodes]);
              }}
              onOpenTerminal={() => setIsTerminalOpen(true)}
              onLoginNode={handleLoginNode}
            />
          )}

          {/* Billing Engine (Section 7) */}
          {currentRole === 'admin' && activeView === 'billing' && (
            <BillingLedgerView language={language} />
          )}

          {/* User / Client Panel Dashboard (Image 1) */}
          {currentRole === 'user' && activeView === 'home' && (
            <UserDashboard 
              language={language}
              onOpenDnsEditor={() => setIsDnsEditorOpen(true)}
              onOpenFileBrowser={() => setIsFileManagerOpen(true)}
              onOpenWhatsApp={() => setIsWhatsAppOpen(true)}
              onOpenTerminal={() => setIsTerminalOpen(true)}
              selectedCategory={selectedUserCategory}
            />
          )}

          {/* Tools Directory View (Image 4) */}
          {activeView === 'tools' && (
            <ToolsDirectoryView 
              language={language}
              onOpenDns={() => setIsDnsEditorOpen(true)}
              onOpenFiles={() => setIsFileManagerOpen(true)}
              onOpenTerminal={() => setIsTerminalOpen(true)}
              searchFilter={searchQuery}
            />
          )}

          {/* Fallback for other sidebar views */}
          {activeView === 'metrics' && (
            <AdminDashboard 
              accounts={accounts}
              systemMetrics={systemMetrics}
              language={language}
              onOpenNewAccount={() => setIsNewAccountOpen(true)}
              onOpenDnsEditor={() => setIsDnsEditorOpen(true)}
              onSwitchToUser={handleSwitchToUser}
              onToggleSuspend={handleToggleSuspend}
              onViewCluster={() => setActiveView('cluster')}
            />
          )}
        </main>
      </div>

      {/* Modals */}
      <DnsZoneEditorModal 
        isOpen={isDnsEditorOpen}
        onClose={() => setIsDnsEditorOpen(false)}
        language={language}
      />

      <NewAccountModal 
        isOpen={isNewAccountOpen}
        onClose={() => setIsNewAccountOpen(false)}
        onAccountCreated={handleAccountCreated}
        language={language}
      />

      <FileManagerModal 
        isOpen={isFileManagerOpen}
        onClose={() => setIsFileManagerOpen(false)}
        language={language}
      />

      <WhatsAppModal 
        isOpen={isWhatsAppOpen}
        onClose={() => setIsWhatsAppOpen(false)}
        language={language}
      />

      <TerminalModal 
        isOpen={isTerminalOpen}
        onClose={() => setIsTerminalOpen(false)}
        language={language}
      />
    </div>
  );
};

export default App;
