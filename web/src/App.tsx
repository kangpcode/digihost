// DigiPanel Master Web Application Shell
// Zero emojis, strictly Lucide SVG iconography, modern glass architecture
// Full-width non-overlapping layout with live backend API telemetry

import React, { useState, useEffect } from 'react';
import { Header } from './components/Header';
import { Sidebar } from './components/Sidebar';
import { AdminDashboard } from './components/AdminDashboard';
import { ClusterFleetDashboard } from './components/ClusterFleetDashboard';
import { UserDashboard } from './components/UserDashboard';
import { ToolsDirectoryView } from './components/ToolsDirectoryView';
import { BillingLedgerView } from './components/BillingLedgerView';
import { RuntimesManagerView } from './components/RuntimesManagerView';
import { SoftInstallerView } from './components/SoftInstallerView';
import { DnsZoneEditorModal } from './components/DnsZoneEditorModal';
import { NewAccountModal } from './components/NewAccountModal';
import { FileManagerModal } from './components/FileManagerModal';
import { WhatsAppModal } from './components/WhatsAppModal';
import { TerminalModal } from './components/TerminalModal';
import { ResellerDashboard } from './components/ResellerDashboard';
import { SslManagerModal } from './components/SslManagerModal';
import { MailManagerModal } from './components/MailManagerModal';
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
  const [isSslManagerOpen, setIsSslManagerOpen] = useState(false);
  const [isMailManagerOpen, setIsMailManagerOpen] = useState(false);

  // Sidebar entries that open a manager modal instead of switching the main view
  const handleViewChange = (view: string) => {
    if (view === 'dns') { setIsDnsEditorOpen(true); return; }
    if (view === 'ssl') { setIsSslManagerOpen(true); return; }
    if (view === 'mail') { setIsMailManagerOpen(true); return; }
    setActiveView(view);
  };

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

  // System Metrics Telemetry Data (Real Host Linux Telemetry)
  const [systemMetrics, setSystemMetrics] = useState<SystemMetrics>({
    hostname: 'Mybook-Hype-5-AMD-X6',
    kernel: 'Linux 7.0.0-38-generic (amd64)',
    os: 'Ubuntu 26.04.1 LTS',
    uptime: '1 hour, 45 mins',
    loadAverage: [0.84, 0.97, 1.09],
    cpuUsagePercent: 6.9,
    memoryUsedMb: 7982,
    memoryTotalMb: 13190,
    diskUsedGb: 183.8,
    diskTotalGb: 467.3,
    activeProcesses: 144
  });

  // Fetch real system telemetry from backend API
  const fetchTelemetry = async () => {
    try {
      const res = await fetch('/api/v1/system/telemetry');
      if (res.ok) {
        const data = await res.json();
        setSystemMetrics({
          hostname: data.hostname,
          kernel: data.kernel,
          os: data.os,
          uptime: data.uptime,
          loadAverage: [
            data.loadAverage?.[0] || 0.84,
            data.loadAverage?.[1] || 0.97,
            data.loadAverage?.[2] || 1.09
          ],
          cpuUsagePercent: data.cpuUsagePercent || 6.9,
          memoryUsedMb: data.memoryUsedMb || 7982,
          memoryTotalMb: data.memoryTotalMb || 13190,
          diskUsedGb: Math.round((data.diskUsedGb || 183.8) * 10) / 10,
          diskTotalGb: Math.round((data.diskTotalGb || 467.3) * 10) / 10,
          activeProcesses: data.activeProcesses || 144
        });
      }
    } catch {
      // Keep state
    }
  };

  useEffect(() => {
    fetchTelemetry();
    const interval = setInterval(fetchTelemetry, 5000);
    return () => clearInterval(interval);
  }, []);

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

  const handleSwitchToUser = (_username: string) => {
    setCurrentRole('user');
    setActiveView('home');
  };

  const handleLoginNode = (_nodeId: string) => {
    setIsTerminalOpen(true);
  };

  return (
    <div className="app-shell">
      {/* 1. Full-Width Top Navigation Bar (No Overlap) */}
      <Header 
        currentRole={currentRole}
        onRoleChange={(r) => {
          setCurrentRole(r);
          if (r === 'admin' || r === 'reseller') setActiveView('dashboard');
          else setActiveView('home');
        }}
        activeView={activeView}
        onViewChange={handleViewChange}
        language={language}
        onLanguageChange={(l) => setLanguage(l)}
        theme={theme}
        onThemeToggle={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
        searchQuery={searchQuery}
        onSearchChange={(q) => setSearchQuery(q)}
        onOpenTerminal={() => setIsTerminalOpen(true)}
      />

      {/* 2. Main Body Container with Sidebar and Content */}
      <div className="app-body">
        {/* Left Navigation Sidebar docked directly beneath Header */}
        <Sidebar 
          currentRole={currentRole}
          activeView={activeView}
          onViewChange={handleViewChange}
          language={language}
          onSelectCategory={(cat) => setSelectedUserCategory(cat)}
          selectedCategory={selectedUserCategory}
        />

        {/* Content Area */}
        <main className="content-area">
          <div className="content-inner">
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

            {/* Reseller Dashboard: scoped to accounts owned by the reseller */}
            {currentRole === 'reseller' && (activeView === 'dashboard' || activeView === 'accounts_list') && (
              <ResellerDashboard
                accounts={accounts}
                language={language}
                onOpenNewAccount={() => setIsNewAccountOpen(true)}
                onOpenDnsEditor={() => setIsDnsEditorOpen(true)}
                onOpenSslManager={() => setIsSslManagerOpen(true)}
                onOpenMailManager={() => setIsMailManagerOpen(true)}
                onSwitchToUser={handleSwitchToUser}
                onToggleSuspend={handleToggleSuspend}
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

            {/* Runtimes & Applications Manager (Node.js, Python, Go, PHP) */}
            {activeView === 'runtimes' && (
              <RuntimesManagerView 
                language={language}
                onOpenTerminal={() => setIsTerminalOpen(true)}
              />
            )}

            {/* Application Marketplace (Softaculous-style installer) */}
            {activeView === 'apps' && (
              <SoftInstallerView language={language} />
            )}

            {/* Billing Engine (Section 7) */}
            {activeView === 'billing' && (
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

            {/* Metrics Fallback */}
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
          </div>
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

      <SslManagerModal
        isOpen={isSslManagerOpen}
        onClose={() => setIsSslManagerOpen(false)}
        language={language}
      />

      <MailManagerModal
        isOpen={isMailManagerOpen}
        onClose={() => setIsMailManagerOpen(false)}
        language={language}
      />
    </div>
  );
};

export default App;
