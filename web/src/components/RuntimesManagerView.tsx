// DigiPanel Runtime & Application Manager (Node.js, Python, Go, PHP)
// aaPanel / cPanel Application Toolkit equivalent
// Zero emojis, strictly clean Lucide SVG iconography, modern glass styling

import React, { useState, useEffect } from 'react';
import { 
  Cpu, 
  Play, 
  Square, 
  RotateCw, 
  Plus, 
  Terminal, 
  Check, 
  Settings, 
  Sliders,
  ChevronRight,
  Trash2
} from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';

interface ManagedApp {
  id: string;
  name: string;
  runtime: 'nodejs' | 'python' | 'golang';
  version: string;
  path: string;
  entryFile: string;
  port: number;
  status: 'running' | 'stopped' | 'error';
  pid: number;
  memoryMb: number;
  cpuPercent: number;
  uptime: string;
  environment: string;
}

interface DetectedRuntimes {
  nodejs?: string;
  python?: string;
  golang?: string;
}

interface RuntimesManagerViewProps {
  language: Language;
  onOpenTerminal: () => void;
}

export const RuntimesManagerView: React.FC<RuntimesManagerViewProps> = ({
  language,
  onOpenTerminal
}) => {
  const t = (key: any) => getTranslation(language, key);

  const [activeTab, setActiveTab] = useState<'nodejs' | 'python' | 'golang' | 'php'>('nodejs');
  const [apps, setApps] = useState<ManagedApp[]>([]);
  const [detected, setDetected] = useState<DetectedRuntimes>({});
  const [loading, setLoading] = useState(false);
  const [showAddModal, setShowAddModal] = useState(false);
  const [selectedAppLogs, setSelectedAppLogs] = useState<ManagedApp | null>(null);
  const [appLogLines, setAppLogLines] = useState<string[]>([]);

  // New App Form State
  const [appName, setAppName] = useState('');
  const [appPath, setAppPath] = useState('/home/kangpcode/wwwsite/digihost/storage/apps/');
  const [appEntry, setAppEntry] = useState('index.js');
  const [appPort, setAppPort] = useState(3000);

  // PHP Manager State
  const [activePhpVersion, setActivePhpVersion] = useState('8.3');
  const [memoryLimit, setMemoryLimit] = useState('512M');
  const [maxExecutionTime, setMaxExecutionTime] = useState(300);
  const [uploadMaxFilesize, setUploadMaxFilesize] = useState('128M');
  const [phpExtensions, setPhpExtensions] = useState<Record<string, boolean>>({
    opcache: true,
    redis: true,
    pdo_mysql: true,
    curl: true,
    mbstring: true,
    gd: true,
    openssl: true,
    bcmath: true,
    fileinfo: true,
    intl: true
  });
  const [phpSavedNotice, setPhpSavedNotice] = useState(false);

  // Fetch supervised apps and host-detected runtime versions from the API.
  const fetchApps = async () => {
    try {
      setLoading(true);
      const [appsData, detectedData] = await Promise.all([
        apiFetch<ManagedApp[]>('/api/v1/runtimes/apps'),
        apiFetch<DetectedRuntimes>('/api/v1/runtimes/detected')
      ]);
      setApps(appsData);
      setDetected(detectedData);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchApps();
  }, []);

  const handleAppAction = async (appId: string, action: 'start' | 'stop' | 'restart') => {
    try {
      await apiFetch('/api/v1/runtimes/apps/action', {
        method: 'POST',
        body: JSON.stringify({ appId, action })
      });
      fetchApps();
    } catch (e) {
      console.error(e);
    }
  };

  const handleDeleteApp = async (appId: string) => {
    try {
      await apiFetch(`/api/v1/runtimes/apps?id=${encodeURIComponent(appId)}`, {
        method: 'DELETE'
      });
      fetchApps();
    } catch (e) {
      console.error(e);
    }
  };

  // Live logs: streamed from the runtime module's captured stdout/stderr files.
  const openAppLogs = async (app: ManagedApp) => {
    try {
      setSelectedAppLogs(app);
      setAppLogLines([]);
      const data = await apiFetch<{ lines: string[] }>(
        `/api/v1/runtimes/apps/logs?id=${encodeURIComponent(app.id)}`
      );
      setAppLogLines(data.lines ?? []);
    } catch (e) {
      console.error(e);
      setAppLogLines(['--- log fetch failed ---']);
    }
  };

  const handleCreateApp = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!appName) return;

    // The backend resolves the interpreter version from the detected host
    // binary; the caller never ships a version string.
    try {
      await apiFetch('/api/v1/runtimes/apps', {
        method: 'POST',
        body: JSON.stringify({
          name: appName,
          runtime: activeTab,
          path: appPath + appName,
          entryFile: appEntry,
          port: appPort,
          environment: 'production'
        })
      });
      setShowAddModal(false);
      setAppName('');
      fetchApps();
    } catch (err) {
      console.error(err);
    }
  };

  const toggleExtension = (ext: string) => {
    setPhpExtensions(prev => ({ ...prev, [ext]: !prev[ext] }));
  };

  const savePhpConfig = () => {
    setPhpSavedNotice(true);
    setTimeout(() => setPhpSavedNotice(false), 2500);
  };

  const currentTabApps = apps.filter(a => a.runtime === activeTab);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      {/* Header */}
      <div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '0.82rem', color: 'var(--text-muted)', marginBottom: '4px' }}>
          <span>{t('navSoftware')}</span>
          <ChevronRight size={14} />
          <span style={{ color: 'var(--text-main)', fontWeight: 600 }}>{t('rtBreadcrumb')}</span>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px' }}>
          <div>
            <h1 style={{ fontSize: '1.6rem', fontWeight: 800, color: 'var(--text-main)', letterSpacing: '-0.02em', marginBottom: '4px' }}>
              {t('rtTitle')}
            </h1>
            <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
              {t('rtSubtitle')}
            </p>
          </div>

          <div style={{ display: 'flex', gap: '10px' }}>
            <button 
              onClick={() => {
                if (activeTab === 'nodejs') { setAppEntry('index.js'); setAppPort(3002); }
                else if (activeTab === 'python') { setAppEntry('main.py'); setAppPort(8001); }
                else if (activeTab === 'golang') { setAppEntry('main.go'); setAppPort(5001); }
                setShowAddModal(true);
              }}
              className="btn btn-primary"
            >
              <Plus size={16} />
              <span>{t('rtAddProject').replace('{runtime}', activeTab.toUpperCase())}</span>
            </button>
            <button onClick={onOpenTerminal} className="btn btn-secondary">
              <Terminal size={14} />
              <span>{t('rtDiagnosticShell')}</span>
            </button>
          </div>
        </div>
      </div>

      {/* Runtime Selection Tabs */}
      <div style={{
        display: 'flex',
        gap: '8px',
        borderBottom: '1px solid var(--glass-border)',
        paddingBottom: '8px'
      }}>
        {[
          { id: 'nodejs', label: `Node.js (v${detected.nodejs ?? '-'})`, count: apps.filter(a => a.runtime === 'nodejs').length },
          { id: 'python', label: `Python (v${detected.python ?? '-'})`, count: apps.filter(a => a.runtime === 'python').length },
          { id: 'golang', label: `Go (v${detected.golang ?? '-'})`, count: apps.filter(a => a.runtime === 'golang').length },
          { id: 'php', label: 'PHP & OPcache (v8.3)', count: 1 }
        ].map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id as any)}
            className={`btn btn-sm ${activeTab === tab.id ? 'btn-primary' : 'btn-secondary'}`}
            style={{ display: 'flex', alignItems: 'center', gap: '8px' }}
          >
            <Cpu size={14} />
            <span>{tab.label}</span>
            <span className="badge-pill" style={{
              background: activeTab === tab.id ? 'rgba(255, 255, 255, 0.25)' : 'rgba(148, 163, 184, 0.15)',
              color: activeTab === tab.id ? '#ffffff' : 'var(--text-muted)'
            }}>
              {tab.count}
            </span>
          </button>
        ))}
      </div>

      {/* Main Apps Table (Node.js, Python, Golang) */}
      {activeTab !== 'php' ? (
        <div className="glass-panel" style={{ padding: '24px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <h3 style={{ fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-main)' }}>
              {t('rtActiveProjects').replace('{runtime}', activeTab.toUpperCase())}
            </h3>
            <button onClick={fetchApps} className="btn btn-secondary btn-sm">
              <RotateCw size={13} />
              <span>{t('refresh')}</span>
            </button>
          </div>

          {loading ? (
            <div style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>
              {t('rtLoading')}
            </div>
          ) : currentTabApps.length === 0 ? (
            <div style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>
              {t('rtEmpty').replace('{runtime}', activeTab)}
            </div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table className="glass-table">
                <thead>
                  <tr>
                    <th>{t('rtThName')}</th>
                    <th>{t('rtThPort')}</th>
                    <th>{t('rtThPid')}</th>
                    <th>{t('rtThResources')}</th>
                    <th>{t('mailThStatus')}</th>
                    <th>{t('rtThUptime')}</th>
                    <th style={{ textAlign: 'right' }}>{t('rtThControls')}</th>
                  </tr>
                </thead>
                <tbody>
                  {currentTabApps.map(app => (
                    <tr key={app.id}>
                      <td>
                        <div style={{ fontWeight: 700, color: 'var(--text-main)' }}>{app.name}</div>
                        <div className="font-mono" style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>
                          {app.entryFile} &bull; {app.path}
                        </div>
                      </td>
                      <td>
                        <div className="badge-pill" style={{ background: 'rgba(2, 132, 199, 0.12)', color: 'var(--status-info)' }}>
                          Port {app.port}
                        </div>
                      </td>
                      <td className="font-mono" style={{ fontSize: '0.8rem' }}>
                        {app.pid > 0 ? app.pid : '-'}
                      </td>
                      <td>
                        <div className="font-mono" style={{ fontSize: '0.78rem' }}>
                          <span style={{ color: 'var(--brand-primary)', fontWeight: 600 }}>{app.cpuPercent}% CPU</span>
                          <span style={{ margin: '0 6px', color: 'var(--text-dim)' }}>|</span>
                          <span style={{ color: 'var(--status-info)', fontWeight: 600 }}>{app.memoryMb} MB</span>
                        </div>
                      </td>
                      <td>
                        {app.status === 'running' ? (
                          <span className="badge-online">
                            <span className="service-dot online" />
                            Running
                          </span>
                        ) : (
                          <span className="badge-warning">
                            <span className="service-dot offline" />
                            Stopped
                          </span>
                        )}
                      </td>
                      <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                        {app.uptime}
                      </td>
                      <td style={{ textAlign: 'right' }}>
                        <div style={{ display: 'inline-flex', gap: '6px' }}>
                          {app.status === 'running' ? (
                            <button 
                              onClick={() => handleAppAction(app.id, 'stop')}
                              className="btn btn-secondary btn-sm"
                              title={t('rtStop')}
                              style={{ padding: '4px 8px', color: 'var(--status-danger)' }}
                            >
                              <Square size={12} />
                            </button>
                          ) : (
                            <button 
                              onClick={() => handleAppAction(app.id, 'start')}
                              className="btn btn-secondary btn-sm"
                              title={t('rtStart')}
                              style={{ padding: '4px 8px', color: 'var(--status-online)' }}
                            >
                              <Play size={12} />
                            </button>
                          )}
                          <button 
                            onClick={() => handleAppAction(app.id, 'restart')}
                            className="btn btn-secondary btn-sm"
                            title={t('rtRestart')}
                            style={{ padding: '4px 8px' }}
                          >
                            <RotateCw size={12} />
                          </button>
                          <button 
                            onClick={() => openAppLogs(app)}
                            className="btn btn-secondary btn-sm"
                            title={t('rtInspectLogs')}
                            style={{ padding: '4px 8px', fontSize: '0.74rem' }}
                          >
                            {t('rtLogs')}
                          </button>
                          {app.status !== 'running' && (
                            <button 
                              onClick={() => handleDeleteApp(app.id)}
                              className="btn btn-secondary btn-sm"
                              title={t('rtDelete')}
                              style={{ padding: '4px 8px', color: 'var(--status-danger)' }}
                            >
                              <Trash2 size={12} />
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      ) : (
        /* PHP & OPcache Manager (Matching cPanel MultiPHP / aaPanel) */
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {/* PHP Notice */}
          {phpSavedNotice && (
            <div style={{
              padding: '12px 18px',
              borderRadius: '10px',
              background: 'rgba(16, 185, 129, 0.15)',
              border: '1px solid rgba(16, 185, 129, 0.3)',
              color: 'var(--status-online)',
              display: 'flex',
              alignItems: 'center',
              gap: '10px',
              fontWeight: 600
            }}>
              <Check size={18} />
              <span>{t('rtPhpReloaded')}</span>
            </div>
          )}

          <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 1.6fr', gap: '20px' }}>
            {/* Directives Editor */}
            <div className="glass-panel" style={{ padding: '24px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '18px' }}>
                <Settings size={18} style={{ color: 'var(--brand-primary)' }} />
                <h3 style={{ fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-main)' }}>
                  {t('rtPhpDirectives')}
                </h3>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                <div>
                  <label style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                    {t('rtPhpActiveVersion')}
                  </label>
                  <select 
                    value={activePhpVersion} 
                    onChange={(e) => setActivePhpVersion(e.target.value)}
                    className="input-glass font-mono"
                    style={{ height: '38px' }}
                  >
                    <option value="8.3">PHP 8.3</option>
                    <option value="8.2">PHP 8.2</option>
                    <option value="8.1">PHP 8.1</option>
                  </select>
                </div>

                <div>
                  <label style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                    memory_limit
                  </label>
                  <select 
                    value={memoryLimit} 
                    onChange={(e) => setMemoryLimit(e.target.value)}
                    className="input-glass font-mono"
                    style={{ height: '38px' }}
                  >
                    <option value="128M">128M</option>
                    <option value="256M">256M</option>
                    <option value="512M">512M ({t('recommended')})</option>
                    <option value="1024M">1024M</option>
                  </select>
                </div>

                <div>
                  <label style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                    max_execution_time (seconds)
                  </label>
                  <input 
                    type="number"
                    value={maxExecutionTime}
                    onChange={(e) => setMaxExecutionTime(Number(e.target.value))}
                    className="input-glass font-mono"
                    style={{ height: '38px' }}
                  />
                </div>

                <div>
                  <label style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                    upload_max_filesize
                  </label>
                  <select 
                    value={uploadMaxFilesize} 
                    onChange={(e) => setUploadMaxFilesize(e.target.value)}
                    className="input-glass font-mono"
                    style={{ height: '38px' }}
                  >
                    <option value="32M">32M</option>
                    <option value="64M">64M</option>
                    <option value="128M">128M</option>
                    <option value="256M">256M</option>
                  </select>
                </div>

                <button onClick={savePhpConfig} className="btn btn-primary" style={{ marginTop: '10px' }}>
                  <Check size={16} />
                  <span>{t('rtSaveReload')}</span>
                </button>
              </div>
            </div>

            {/* Extensions Manager */}
            <div className="glass-panel" style={{ padding: '24px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '18px' }}>
                <Sliders size={18} style={{ color: 'var(--status-info)' }} />
                <h3 style={{ fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-main)' }}>
                  {t('rtPhpExtensions')}
                </h3>
              </div>

              <div style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
                gap: '12px'
              }}>
                {Object.entries(phpExtensions).map(([ext, isEnabled]) => (
                  <div 
                    key={ext}
                    onClick={() => toggleExtension(ext)}
                    className="glass-card"
                    style={{
                      padding: '12px 14px',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      cursor: 'pointer',
                      border: isEnabled ? '1px solid var(--status-online)' : '1px solid var(--glass-border)'
                    }}
                  >
                    <span className="font-mono" style={{ fontSize: '0.82rem', fontWeight: 600 }}>
                      {ext}
                    </span>
                    <span className={isEnabled ? 'badge-online' : 'badge-warning'} style={{ fontSize: '0.68rem', padding: '2px 6px' }}>
                      {isEnabled ? t('rtOn') : t('rtOff')}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Modal: Add Project */}
      {showAddModal && (
        <div className="modal-backdrop" onClick={() => setShowAddModal(false)}>
          <div className="modal-content" onClick={(e) => e.stopPropagation()} style={{ maxWidth: '580px', padding: '28px' }}>
            <h3 style={{ fontSize: '1.2rem', fontWeight: 700, marginBottom: '16px' }}>
              {t('rtDeployTitle').replace('{runtime}', activeTab.toUpperCase())}
            </h3>

            <form onSubmit={handleCreateApp} style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              <div>
                <label style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  {t('rtProjectName')}
                </label>
                <input 
                  type="text"
                  placeholder={t('rtProjectNamePh')}
                  value={appName}
                  onChange={(e) => setAppName(e.target.value)}
                  className="input-glass font-mono"
                  style={{ height: '38px' }}
                  required
                />
              </div>

              <div>
                <label style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  {t('rtTargetPath')}
                </label>
                <input 
                  type="text"
                  value={appPath + appName}
                  onChange={(e) => setAppPath(e.target.value)}
                  className="input-glass font-mono"
                  style={{ height: '38px' }}
                  required
                />
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                <div>
                  <label style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                    {t('rtEntryScript')}
                  </label>
                  <input 
                    type="text"
                    value={appEntry}
                    onChange={(e) => setAppEntry(e.target.value)}
                    className="input-glass font-mono"
                    style={{ height: '38px' }}
                    required
                  />
                </div>
                <div>
                  <label style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                    {t('rtListeningPort')}
                  </label>
                  <input 
                    type="number"
                    value={appPort}
                    onChange={(e) => setAppPort(Number(e.target.value))}
                    className="input-glass font-mono"
                    style={{ height: '38px' }}
                    required
                  />
                </div>
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '14px' }}>
                <button type="button" onClick={() => setShowAddModal(false)} className="btn btn-secondary">
                  {t('cancel')}
                </button>
                <button type="submit" className="btn btn-primary">
                  <Check size={16} />
                  <span>{t('rtDeployBtn')}</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: App Logs Viewer */}
      {selectedAppLogs && (
        <div className="modal-backdrop" onClick={() => setSelectedAppLogs(null)}>
          <div className="modal-content" onClick={(e) => e.stopPropagation()} style={{ maxWidth: '720px', padding: '24px', background: '#090d16', border: '1px solid rgba(255,255,255,0.1)' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px' }}>
              <span className="font-mono" style={{ fontSize: '0.88rem', fontWeight: 700, color: '#f8fafc' }}>
                {t('rtServiceLogs')}: {selectedAppLogs.name} ({t('rtThPid')} {selectedAppLogs.pid})
              </span>
              <button onClick={() => setSelectedAppLogs(null)} className="btn-icon" style={{ color: '#94a3b8' }}>
                X
              </button>
            </div>
            <pre className="font-mono" style={{ fontSize: '0.78rem', color: '#38bdf8', lineHeight: '1.6', background: 'rgba(255,255,255,0.02)', padding: '16px', borderRadius: '8px', maxHeight: '360px', overflowY: 'auto' }}>
{appLogLines.join('\n') || '...'}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
};
