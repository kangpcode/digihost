// DigiPanel application shell.
// Every view renders live core API data. No sample accounts, nodes, or metrics
// exist in this layer.

import React, { useCallback, useEffect, useMemo, useState } from 'react';
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
import { AccountsListView } from './components/AccountsListView';
import { PlansView } from './components/PlansView';
import { ResellersView } from './components/ResellersView';
import { DnsClusterView } from './components/DnsClusterView';
import { ServerStatusView } from './components/ServerStatusView';
import { AuditLogView } from './components/AuditLogView';
import { AccountLogsView } from './components/AccountLogsView';
import { apiFetch, logout, readSession, setUnauthorizedHandler, type SessionToken } from './api/client';
import { LoginView } from './components/LoginView';
import { getTranslation } from './i18n/translations';
import type {
  UserRole,
  Language,
  ThemeMode,
  HostingAccount,
  ApiAccount,
  FleetNode,
  FleetResponse,
  ServiceResponse,
  ServiceStatus,
  Telemetry,
  SystemMetrics
} from './types';

const emptyMetrics: SystemMetrics = {
  hostname: '',
  kernel: '',
  os: '',
  uptime: '',
  loadAverage: [0, 0, 0],
  cpuUsagePercent: 0,
  memoryUsedMb: 0,
  memoryTotalMb: 0,
  diskUsedGb: 0,
  diskTotalGb: 0,
  activeProcesses: 0
};

const bytesToMb = (bytes: number): number => Math.round((bytes / (1024 * 1024)) * 100) / 100;

const toViewAccount = (account: ApiAccount): HostingAccount => ({
  id: account.id,
  username: account.username,
  domain: account.primaryDomain,
  plan: account.packageId,
  diskUsageMb: bytesToMb(account.diskUsedBytes),
  diskLimitMb: bytesToMb(account.diskLimitBytes),
  bandwidthUsageMb: bytesToMb(account.bandwidthUsedBytes),
  bandwidthLimitMb: bytesToMb(account.bandwidthLimitBytes),
  inodesCount: account.inodesUsed,
  inodesLimit: account.inodesLimit,
  status: account.status === 'suspended' ? 'suspended' : account.status === 'terminated' ? 'terminated' : 'active',
  phpVersion: account.phpVersion,
  ipAddress: account.ipAddress,
  homeDir: account.homeDir,
  createdAt: account.createdAt ? account.createdAt.slice(0, 10) : ''
});

const roleFromSession = (role: string | undefined): UserRole => {
  if (role === 'admin' || role === 'reseller' || role === 'user') return role;
  // Staff accounts operate the admin surface with the permissions the core
  // grants their role.
  return 'admin';
};

export const App: React.FC = () => {
  const [theme, setTheme] = useState<ThemeMode>('light');
  const [session, setSession] = useState<SessionToken | null>(() => readSession());
  const [currentRole, setCurrentRole] = useState<UserRole>(() => roleFromSession(readSession()?.role));
  const [activeView, setActiveView] = useState<string>('dashboard');
  const [language, setLanguage] = useState<Language>('en');
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedUserCategory, setSelectedUserCategory] = useState('all');

  const [isDnsEditorOpen, setIsDnsEditorOpen] = useState(false);
  const [isNewAccountOpen, setIsNewAccountOpen] = useState(false);
  const [isFileManagerOpen, setIsFileManagerOpen] = useState(false);
  const [isWhatsAppOpen, setIsWhatsAppOpen] = useState(false);
  const [isTerminalOpen, setIsTerminalOpen] = useState(false);
  const [isSslManagerOpen, setIsSslManagerOpen] = useState(false);
  const [isMailManagerOpen, setIsMailManagerOpen] = useState(false);

  const [telemetry, setTelemetry] = useState<Telemetry | null>(null);
  const [accounts, setAccounts] = useState<ApiAccount[]>([]);
  const [nodes, setNodes] = useState<FleetNode[]>([]);
  const [services, setServices] = useState<ServiceStatus[]>([]);
  const [servicesAvailable, setServicesAvailable] = useState(false);
  const [resellerCount, setResellerCount] = useState(0);
  const [activeAccount, setActiveAccount] = useState<ApiAccount | null>(null);
  const [panelVersion, setPanelVersion] = useState('');
  const [loadError, setLoadError] = useState('');

  const t = (key: any) => getTranslation(language, key);

  const viewAccounts = useMemo(() => accounts.map(toViewAccount), [accounts]);

  const systemMetrics = useMemo<SystemMetrics>(() => {
    if (!telemetry) return emptyMetrics;
    return {
      hostname: telemetry.hostname,
      kernel: telemetry.kernel,
      os: telemetry.os,
      uptime: telemetry.uptime,
      loadAverage: [
        telemetry.loadAverage?.[0] ?? 0,
        telemetry.loadAverage?.[1] ?? 0,
        telemetry.loadAverage?.[2] ?? 0
      ],
      cpuUsagePercent: telemetry.cpuUsagePercent,
      memoryUsedMb: telemetry.memoryUsedMb,
      memoryTotalMb: telemetry.memoryTotalMb,
      diskUsedGb: Math.round(telemetry.diskUsedGb * 10) / 10,
      diskTotalGb: Math.round(telemetry.diskTotalGb * 10) / 10,
      activeProcesses: telemetry.activeProcesses
    };
  }, [telemetry]);

  const loadTelemetry = useCallback(async () => {
    try {
      const data = await apiFetch<Telemetry>('/api/v1/system/telemetry');
      setTelemetry(data);
    } catch (cause) {
      setLoadError(cause instanceof Error ? cause.message : String(cause));
    }
  }, []);

  const loadHostingData = useCallback(async () => {
    const [accountResult, fleetResult, resellerResult, serviceResult] = await Promise.allSettled([
      apiFetch<ApiAccount[]>('/api/v1/hosting/accounts'),
      apiFetch<FleetResponse>('/api/v1/cluster/fleet'),
      apiFetch<Array<{ username: string }>>('/api/v1/users?role=reseller'),
      apiFetch<ServiceResponse>('/api/v1/system/services')
    ]);

    if (accountResult.status === 'fulfilled') {
      const list = Array.isArray(accountResult.value) ? accountResult.value : [];
      setAccounts(list);
      setActiveAccount((current) => {
        if (current) {
          const refreshed = list.find((item) => item.username === current.username);
          return refreshed ?? list[0] ?? null;
        }
        return list[0] ?? null;
      });
    } else {
      setLoadError(accountResult.reason instanceof Error ? accountResult.reason.message : String(accountResult.reason));
    }

    if (fleetResult.status === 'fulfilled') {
      setNodes(Array.isArray(fleetResult.value.nodes) ? fleetResult.value.nodes : []);
    }
    if (resellerResult.status === 'fulfilled') {
      setResellerCount(Array.isArray(resellerResult.value) ? resellerResult.value.length : 0);
    }
    if (serviceResult.status === 'fulfilled') {
      setServices(Array.isArray(serviceResult.value.services) ? serviceResult.value.services : []);
      setServicesAvailable(true);
    } else {
      setServices([]);
      setServicesAvailable(false);
    }
  }, []);

  const reload = useCallback(async () => {
    setLoadError('');
    await Promise.all([loadTelemetry(), loadHostingData()]);
  }, [loadHostingData, loadTelemetry]);

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
  }, [theme]);

  useEffect(() => {
    setUnauthorizedHandler(() => {
      setSession(null);
    });
    return () => setUnauthorizedHandler(null);
  }, []);

  useEffect(() => {
    if (!session) return;
    setCurrentRole(roleFromSession(session.role));
  }, [session]);

  useEffect(() => {
    apiFetch<{ version: string }>('/api/v1/health')
      .then((data) => setPanelVersion(data.version || ''))
      .catch(() => setPanelVersion(''));
  }, []);

  useEffect(() => {
    if (!session) return;
    void reload();
    const interval = window.setInterval(() => {
      void loadTelemetry();
    }, 10000);
    return () => window.clearInterval(interval);
  }, [reload, loadTelemetry, session]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== '/') return;
      const target = event.target as HTMLElement;
      if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA') return;
      event.preventDefault();
      const input = document.querySelector('input[type="text"]') as HTMLInputElement | null;
      input?.focus();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  const handleViewChange = (view: string) => {
    if (view === 'dns' || view === 'domains') { setIsDnsEditorOpen(true); return; }
    if (view === 'ssl' || view === 'security') { setIsSslManagerOpen(true); return; }
    if (view === 'mail' || view === 'emails') { setIsMailManagerOpen(true); return; }
    if (view === 'files') { setIsFileManagerOpen(true); return; }
    if (view === 'advanced') { setActiveView('access_logs'); return; }
    setActiveView(view);
  };

  const handleSignedIn = () => {
    const next = readSession();
    setSession(next);
    const role = roleFromSession(next?.role);
    setCurrentRole(role);
    setActiveView(role === 'user' ? 'home' : 'dashboard');
    void reload();
  };

  const handleSignOut = async () => {
    await logout();
    setSession(null);
    setAccounts([]);
    setNodes([]);
    setServices([]);
    setActiveAccount(null);
  };

  const handleAccountCreated = () => {
    void reload();
  };

  const handleToggleSuspend = async (accountId: string) => {
    const account = accounts.find((item) => item.id === accountId);
    if (!account) return;
    const action = account.status === 'suspended' ? 'unsuspend' : 'suspend';
    try {
      await apiFetch(`/api/v1/hosting/accounts/${action}`, {
        method: 'POST',
        body: JSON.stringify({ username: account.username })
      });
      await reload();
    } catch (cause) {
      setLoadError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  const handleSwitchToUser = (username: string) => {
    const account = accounts.find((item) => item.username === username) ?? null;
    setActiveAccount(account);
    setCurrentRole('user');
    setActiveView('home');
  };

  const activeViewAccount = useMemo(
    () => (activeAccount ? toViewAccount(activeAccount) : null),
    [activeAccount]
  );

  const notifications = useMemo(() => {
    const down = services
      .filter((service) => service.installed && !service.active)
      .map((service) => ({
        title: `${service.name} (${service.unit})`,
        detail: `${t('headerServiceDown')}: ${service.state}`
      }));
    return down;
  }, [services, language]);

  const displayName = session?.username || '';

  if (!session) {
    return (
      <LoginView
        language={language}
        onLanguageChange={(lang) => setLanguage(lang)}
        theme={theme}
        onThemeToggle={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
        onSignedIn={handleSignedIn}
      />
    );
  }

  return (
    <div className="app-shell">
      <Header
        currentRole={currentRole}
        onSignOut={() => void handleSignOut()}
        activeView={activeView}
        onViewChange={handleViewChange}
        language={language}
        onLanguageChange={(lang) => setLanguage(lang)}
        theme={theme}
        onThemeToggle={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
        searchQuery={searchQuery}
        onSearchChange={(query) => setSearchQuery(query)}
        onOpenTerminal={() => setIsTerminalOpen(true)}
        displayName={displayName}
        notifications={notifications}
      />

      <div className="app-body">
        <Sidebar
          currentRole={currentRole}
          activeView={activeView}
          onViewChange={handleViewChange}
          language={language}
          onSelectCategory={(category) => setSelectedUserCategory(category)}
          selectedCategory={selectedUserCategory}
          hostname={systemMetrics.hostname}
          kernel={systemMetrics.kernel}
          panelVersion={panelVersion}
        />

        <main className="content-area">
          <div className="content-inner">
            {loadError ? (
              <div className="glass-card" style={{ padding: '10px 14px', marginBottom: '16px', fontSize: '0.82rem', borderColor: 'rgba(239, 68, 68, 0.35)' }}>
                {loadError}
              </div>
            ) : null}

            {currentRole === 'admin' && activeView === 'dashboard' && (
              <AdminDashboard
                accounts={viewAccounts}
                systemMetrics={systemMetrics}
                resellerCount={resellerCount}
                language={language}
                onOpenNewAccount={() => setIsNewAccountOpen(true)}
                onOpenDnsEditor={() => setIsDnsEditorOpen(true)}
                onOpenSslManager={() => setIsSslManagerOpen(true)}
                onSwitchToUser={handleSwitchToUser}
                onToggleSuspend={(id) => void handleToggleSuspend(id)}
                onViewAccounts={() => setActiveView('accounts_list')}
                onViewResellers={() => setActiveView('resellers_list')}
                onViewMetrics={() => setActiveView('server_status')}
              />
            )}

            {(currentRole === 'reseller') && (activeView === 'dashboard' || activeView === 'accounts_list') && (
              <ResellerDashboard
                accounts={viewAccounts}
                language={language}
                onOpenNewAccount={() => setIsNewAccountOpen(true)}
                onOpenDnsEditor={() => setIsDnsEditorOpen(true)}
                onOpenSslManager={() => setIsSslManagerOpen(true)}
                onOpenMailManager={() => setIsMailManagerOpen(true)}
                onSwitchToUser={handleSwitchToUser}
                onToggleSuspend={(id) => void handleToggleSuspend(id)}
              />
            )}

            {currentRole === 'admin' && activeView === 'accounts_list' && (
              <AccountsListView language={language} onOpenNewAccount={() => setIsNewAccountOpen(true)} />
            )}

            {(currentRole === 'admin' || currentRole === 'reseller') && activeView === 'plans' && (
              <PlansView language={language} canManage={currentRole === 'admin'} />
            )}

            {currentRole === 'admin' && activeView === 'resellers_list' && (
              <ResellersView language={language} />
            )}

            {currentRole === 'admin' && activeView === 'dns_cluster' && (
              <DnsClusterView language={language} />
            )}

            {currentRole === 'admin' && activeView === 'cluster' && (
              <ClusterFleetDashboard
                nodes={nodes}
                telemetry={telemetry}
                services={services}
                servicesAvailable={servicesAvailable}
                inventory={{
                  accounts: accounts.length,
                  domains: accounts.filter((account) => account.primaryDomain).length,
                  databases: 0,
                  resellers: resellerCount
                }}
                language={language}
                onRefreshFleet={() => void reload()}
                onOpenTerminal={() => setIsTerminalOpen(true)}
              />
            )}

            {(activeView === 'server_status' || (currentRole === 'admin' && activeView === 'metrics')) && (
              <ServerStatusView language={language} />
            )}

            {currentRole === 'admin' && activeView === 'access_logs' && (
              <AccountLogsView language={language} />
            )}

            {currentRole === 'admin' && activeView === 'audit' && (
              <AuditLogView language={language} />
            )}

            {activeView === 'runtimes' && (
              <RuntimesManagerView language={language} onOpenTerminal={() => setIsTerminalOpen(true)} />
            )}

            {activeView === 'apps' && (
              <SoftInstallerView language={language} />
            )}

            {(activeView === 'billing') && (
              <BillingLedgerView language={language} />
            )}

            {currentRole === 'user' && activeView === 'home' && (
              <UserDashboard
                language={language}
                account={activeViewAccount}
                onOpenDnsEditor={() => setIsDnsEditorOpen(true)}
                onOpenFileBrowser={() => setIsFileManagerOpen(true)}
                onOpenMailManager={() => setIsMailManagerOpen(true)}
                onOpenSslManager={() => setIsSslManagerOpen(true)}
                onOpenTerminal={() => setIsTerminalOpen(true)}
                onOpenApps={() => setActiveView('apps')}
                onOpenLogs={() => setActiveView('access_logs')}
                onOpenRuntimes={() => setActiveView('runtimes')}
                onOpenBilling={() => setActiveView('billing')}
              />
            )}

            {activeView === 'tools' && (
              <ToolsDirectoryView
                language={language}
                onOpenDns={() => setIsDnsEditorOpen(true)}
                onOpenFiles={() => setIsFileManagerOpen(true)}
                onOpenTerminal={() => setIsTerminalOpen(true)}
                searchFilter={searchQuery}
              />
            )}

            {activeView === 'whatsapp' && (
              <div className="glass-panel" style={{ padding: '24px' }}>
                <button type="button" className="btn btn-primary" onClick={() => setIsWhatsAppOpen(true)}>
                  {t('connect')}
                </button>
              </div>
            )}
          </div>
        </main>
      </div>

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
        account={activeAccount}
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
