// DigiPanel encrypted backup manager.
// Archives are created by the core engine (tar frames, per-frame AEAD, manifest
// checksum). Restores land in the panel staging area so nothing is written over
// a live account without an operator moving it, and every archive can be
// verified against its manifest.

import React, { useCallback, useEffect, useState } from 'react';
import { Archive, Clock, Download, RefreshCw, ShieldCheck, Trash2, Upload } from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch, readSession } from '../api/client';
import { OpsEmpty, OpsError, OpsHeader, formatBytes, formatDate } from './opsShared';

interface BackupArchive {
  id: string;
  account: string;
  label: string;
  createdAt: string;
  sizeBytes: number;
  plainBytes: number;
  entries: number;
  sha256: string;
  encryption: string;
  format: string;
  restoredAt?: string;
  restoreCount: number;
}

interface BackupSchedule {
  id: string;
  account: string;
  label: string;
  frequency: string;
  hour: number;
  retention: number;
  enabled: boolean;
  createdAt: string;
  lastRun?: string;
  nextRun: string;
}

interface BackupStatus {
  configured: boolean;
  root?: string;
  staging: string;
}

interface RestoreOutcome {
  id: string;
  target: string;
  files: number;
  bytes: number;
  restoredAt: string;
}

interface AccountOption {
  username: string;
  primaryDomain: string;
}

export const BackupsView: React.FC<{ language: Language }> = ({ language }) => {
  const t = (key: any) => getTranslation(language, key);
  const session = readSession();
  const lockedAccount = session?.role === 'user' ? session.username : '';

  const [accounts, setAccounts] = useState<AccountOption[]>([]);
  const [account, setAccount] = useState(lockedAccount);
  const [archives, setArchives] = useState<BackupArchive[]>([]);
  const [schedules, setSchedules] = useState<BackupSchedule[]>([]);
  const [status, setStatus] = useState<BackupStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState('');

  const [label, setLabel] = useState('');
  const [scheduleFrequency, setScheduleFrequency] = useState('daily');
  const [scheduleHour, setScheduleHour] = useState(2);
  const [scheduleRetention, setScheduleRetention] = useState(7);
  const [scheduleLabel, setScheduleLabel] = useState('scheduled');

  const loadAccounts = useCallback(async () => {
    if (lockedAccount) {
      setAccounts([{ username: lockedAccount, primaryDomain: '' }]);
      return;
    }
    try {
      const data = await apiFetch<AccountOption[]>('/api/v1/hosting/accounts');
      const list = Array.isArray(data) ? data : [];
      setAccounts(list);
      setAccount((current) => (current && list.some((item) => item.username === current) ? current : list[0]?.username || ''));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  }, [lockedAccount]);

  const loadStatus = useCallback(async () => {
    try {
      setStatus(await apiFetch<BackupStatus>('/api/v1/backups/status'));
    } catch (cause) {
      setStatus(null);
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  }, []);

  const loadSchedules = useCallback(async () => {
    try {
      const data = await apiFetch<BackupSchedule[]>('/api/v1/backups/schedules');
      setSchedules(Array.isArray(data) ? data : []);
    } catch {
      setSchedules([]);
    }
  }, []);

  const loadArchives = useCallback(async (username: string) => {
    if (!username) {
      setArchives([]);
      return;
    }
    setLoading(true);
    try {
      const data = await apiFetch<BackupArchive[]>(`/api/v1/backups?username=${encodeURIComponent(username)}`);
      setArchives(Array.isArray(data) ? data : []);
    } catch (cause) {
      setArchives([]);
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadAccounts();
    void loadStatus();
    void loadSchedules();
  }, [loadAccounts, loadStatus, loadSchedules]);

  useEffect(() => {
    void loadArchives(account);
  }, [account, loadArchives]);

  const refresh = async () => {
    await Promise.all([loadStatus(), loadArchives(account), loadSchedules()]);
  };

  const configured = status === null || status.configured;

  const createArchive = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!account) return;
    setBusy('create');
    setError('');
    setNotice('');
    try {
      const archive = await apiFetch<BackupArchive>('/api/v1/backups', {
        method: 'POST',
        body: JSON.stringify({ username: account, label: label.trim() })
      });
      setNotice(`${t('backupCreated')}: ${archive.id} - ${archive.entries} ${t('backupEntries')}, ${formatBytes(archive.sizeBytes)}`);
      setLabel('');
      await refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  const verifyArchive = async (archive: BackupArchive) => {
    setBusy(archive.id);
    setError('');
    setNotice('');
    try {
      await apiFetch('/api/v1/backups/verify', {
        method: 'POST',
        body: JSON.stringify({ username: account, id: archive.id })
      });
      setNotice(`${t('backupVerified')}: ${archive.id}`);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  const restoreArchive = async (archive: BackupArchive) => {
    if (!window.confirm(`${t('backupRestoreConfirm')} ${archive.id}?`)) return;
    setBusy(archive.id);
    setError('');
    setNotice('');
    try {
      const result = await apiFetch<RestoreOutcome>('/api/v1/backups/restore', {
        method: 'POST',
        body: JSON.stringify({ username: account, id: archive.id })
      });
      setNotice(`${t('backupRestored')}: ${result.files} ${t('backupFiles')}, ${formatBytes(result.bytes)} -> ${result.target}`);
      await refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  const deleteArchive = async (archive: BackupArchive) => {
    if (!window.confirm(`${t('backupDeleteConfirm')} ${archive.id}?`)) return;
    setBusy(archive.id);
    setError('');
    setNotice('');
    try {
      await apiFetch(`/api/v1/backups?username=${encodeURIComponent(account)}&id=${encodeURIComponent(archive.id)}`, {
        method: 'DELETE'
      });
      await refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  const saveSchedule = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!account) return;
    setBusy('schedule');
    setError('');
    setNotice('');
    try {
      const schedule = await apiFetch<BackupSchedule>('/api/v1/backups/schedules', {
        method: 'POST',
        body: JSON.stringify({
          account,
          label: scheduleLabel.trim(),
          frequency: scheduleFrequency,
          hour: Number(scheduleHour),
          retention: Number(scheduleRetention)
        })
      });
      setNotice(`${t('backupScheduleSaved')}: ${schedule.frequency} ${t('backupAtHour')} ${schedule.hour}:00, ${t('backupNextRun')} ${formatDate(schedule.nextRun)}`);
      await loadSchedules();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  const deleteSchedule = async (schedule: BackupSchedule) => {
    if (!window.confirm(`${t('backupScheduleDeleteConfirm')} ${schedule.account}?`)) return;
    setBusy(schedule.id);
    setError('');
    try {
      await apiFetch(`/api/v1/backups/schedules?id=${encodeURIComponent(schedule.id)}`, { method: 'DELETE' });
      await loadSchedules();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <OpsHeader
        title={t('navBackups')}
        subtitle={t('backupSubtitle')}
        action={
          <div style={{ display: 'flex', gap: '8px' }}>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => void refresh()} disabled={loading}>
              <RefreshCw size={14} />
              {t('refresh')}
            </button>
          </div>
        }
      />

      {error ? <OpsError message={error} onDismiss={() => setError('')} /> : null}
      {notice ? (
        <div className="glass-card" style={{ padding: '10px 14px', fontSize: '0.8rem', borderColor: 'rgba(16, 185, 129, 0.35)', wordBreak: 'break-all' }}>
          {notice}
        </div>
      ) : null}

      {status ? (
        <div className="glass-card" style={{ padding: '12px 16px', display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.78rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <ShieldCheck size={15} style={{ color: configured ? 'var(--status-online)' : 'var(--status-warning)' }} />
            <span style={{ fontWeight: 700 }}>
              {configured ? t('backupEncryptionActive') : t('backupNotConfigured')}
            </span>
          </div>
          {status.root ? <span className="font-mono" style={{ color: 'var(--text-muted)' }}>{t('backupRoot')}: {status.root}</span> : null}
          <span className="font-mono" style={{ color: 'var(--text-muted)' }}>{t('backupStaging')}: {status.staging}</span>
        </div>
      ) : null}

      {!lockedAccount ? (
        <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.76rem', fontWeight: 600, color: 'var(--text-muted)', maxWidth: '320px' }}>
          {t('dbAccount')}
          <select className="input-glass" value={account} onChange={(event) => setAccount(event.target.value)}>
            {accounts.length === 0 ? <option value="">{t('dbNoAccounts')}</option> : null}
            {accounts.map((option) => (
              <option key={option.username} value={option.username}>
                {option.username}{option.primaryDomain ? ` - ${option.primaryDomain}` : ''}
              </option>
            ))}
          </select>
        </label>
      ) : null}

      <form className="glass-panel" style={{ padding: '16px', display: 'flex', gap: '10px', flexWrap: 'wrap', alignItems: 'flex-end' }} onSubmit={createArchive}>
        <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.74rem', fontWeight: 600, color: 'var(--text-muted)', minWidth: '240px', flex: 1 }}>
          {t('backupLabel')}
          <input
            className="input-glass"
            value={label}
            onChange={(event) => setLabel(event.target.value)}
            placeholder={t('backupLabelPlaceholder')}
            maxLength={64}
          />
        </label>
        <button type="submit" className="btn btn-primary btn-sm" disabled={!account || !configured || busy === 'create'}>
          <Archive size={14} />
          {busy === 'create' ? t('backupRunning') : t('backupCreate')}
        </button>
      </form>

      {loading && archives.length === 0 ? (
        <div className="glass-panel" style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>{t('appsLoading')}</div>
      ) : !account ? (
        <OpsEmpty message={t('dbNoAccounts')} />
      ) : archives.length === 0 ? (
        <OpsEmpty message={t('backupEmpty')} />
      ) : (
        <div className="glass-panel" style={{ overflowX: 'auto' }}>
          <table className="glass-table">
            <thead>
              <tr>
                <th>{t('backupThArchive')}</th>
                <th>{t('backupThCreated')}</th>
                <th>{t('backupThSize')}</th>
                <th>{t('backupThEntries')}</th>
                <th>{t('backupThRestores')}</th>
                <th style={{ textAlign: 'right' }}>{t('thActions')}</th>
              </tr>
            </thead>
            <tbody>
              {archives.map((archive) => (
                <tr key={archive.id}>
                  <td>
                    <div className="font-mono" style={{ fontSize: '0.76rem', fontWeight: 700 }}>{archive.id}</div>
                    <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>{archive.label || '-'} &middot; {archive.encryption}</div>
                  </td>
                  <td style={{ fontSize: '0.74rem' }}>{formatDate(archive.createdAt)}</td>
                  <td className="font-mono" style={{ fontSize: '0.74rem' }}>{formatBytes(archive.sizeBytes)}</td>
                  <td className="font-mono" style={{ fontSize: '0.74rem' }}>{archive.entries}</td>
                  <td className="font-mono" style={{ fontSize: '0.74rem' }}>{archive.restoreCount}</td>
                  <td style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>
                    <button type="button" className="btn btn-secondary btn-sm" onClick={() => void verifyArchive(archive)} disabled={busy === archive.id}>
                      <ShieldCheck size={12} />
                      {t('backupVerify')}
                    </button>{' '}
                    <button type="button" className="btn btn-secondary btn-sm" onClick={() => void restoreArchive(archive)} disabled={busy === archive.id}>
                      <Download size={12} />
                      {t('backupRestore')}
                    </button>{' '}
                    <button type="button" className="btn btn-secondary btn-sm" onClick={() => void deleteArchive(archive)} disabled={busy === archive.id}>
                      <Trash2 size={12} />
                      {t('delete')}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div>
        <h2 style={{ fontSize: '1rem', fontWeight: 700, marginBottom: '10px' }}>{t('backupSchedules')}</h2>
        <form className="glass-panel" style={{ padding: '16px', display: 'flex', gap: '10px', flexWrap: 'wrap', alignItems: 'flex-end', marginBottom: '12px' }} onSubmit={saveSchedule}>
          <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.74rem', fontWeight: 600, color: 'var(--text-muted)' }}>
            {t('backupLabel')}
            <input className="input-glass" value={scheduleLabel} onChange={(event) => setScheduleLabel(event.target.value)} maxLength={64} required style={{ minWidth: '160px' }} />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.74rem', fontWeight: 600, color: 'var(--text-muted)' }}>
            {t('backupFrequency')}
            <select className="input-glass" value={scheduleFrequency} onChange={(event) => setScheduleFrequency(event.target.value)}>
              <option value="daily">{t('backupDaily')}</option>
              <option value="weekly">{t('backupWeekly')}</option>
              <option value="monthly">{t('backupMonthly')}</option>
            </select>
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.74rem', fontWeight: 600, color: 'var(--text-muted)' }}>
            {t('backupHour')}
            <input
              type="number"
              min={0}
              max={23}
              className="input-glass"
              value={scheduleHour}
              onChange={(event) => setScheduleHour(Number(event.target.value))}
              style={{ width: '90px' }}
            />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.74rem', fontWeight: 600, color: 'var(--text-muted)' }}>
            {t('backupRetention')}
            <input
              type="number"
              min={1}
              max={365}
              className="input-glass"
              value={scheduleRetention}
              onChange={(event) => setScheduleRetention(Number(event.target.value))}
              style={{ width: '90px' }}
            />
          </label>
          <button type="submit" className="btn btn-primary btn-sm" disabled={!account || !configured || busy === 'schedule'}>
            <Upload size={14} />
            {t('backupSaveSchedule')}
          </button>
        </form>

        {schedules.length === 0 ? (
          <OpsEmpty message={t('backupNoSchedules')} />
        ) : (
          <div className="glass-panel" style={{ overflowX: 'auto' }}>
            <table className="glass-table">
              <thead>
                <tr>
                  <th>{t('dbAccount')}</th>
                  <th>{t('backupFrequency')}</th>
                  <th>{t('backupHour')}</th>
                  <th>{t('backupRetention')}</th>
                  <th>{t('backupLastRun')}</th>
                  <th>{t('backupNextRun')}</th>
                  <th style={{ textAlign: 'right' }}>{t('thActions')}</th>
                </tr>
              </thead>
              <tbody>
                {schedules.map((schedule) => (
                  <tr key={schedule.id}>
                    <td className="font-mono" style={{ fontSize: '0.76rem', fontWeight: 700 }}>{schedule.account}</td>
                    <td style={{ fontSize: '0.76rem', textTransform: 'capitalize' }}>{schedule.frequency}</td>
                    <td className="font-mono" style={{ fontSize: '0.76rem' }}>{String(schedule.hour).padStart(2, '0')}:00</td>
                    <td className="font-mono" style={{ fontSize: '0.76rem' }}>{schedule.retention}</td>
                    <td style={{ fontSize: '0.74rem' }}>{schedule.lastRun ? formatDate(schedule.lastRun) : '-'}</td>
                    <td style={{ fontSize: '0.74rem' }}>
                      <Clock size={12} style={{ verticalAlign: '-2px', marginRight: '4px', color: 'var(--text-muted)' }} />
                      {formatDate(schedule.nextRun)}
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      <button type="button" className="btn btn-secondary btn-sm" onClick={() => void deleteSchedule(schedule)} disabled={busy === schedule.id}>
                        <Trash2 size={12} />
                        {t('delete')}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
};

export default BackupsView;
