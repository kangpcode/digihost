// DigiPanel Database Manager view.
// Schemas and identities are created through the core API, which prefixes every
// object with the owning account and enforces the package limit. Passwords are
// shown exactly once, when they are generated.

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Database, KeyRound, Plus, RefreshCw, Trash2, UserRound, X, Server } from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch, readSession } from '../api/client';
import { OpsEmpty, OpsError, OpsHeader, formatBytes } from './opsShared';

interface ManagedDatabase {
  name: string;
  owner: string;
  charset: string;
  collation: string;
  tables: number;
  sizeBytes: number;
}

interface DatabaseUser {
  name: string;
  owner: string;
  host: string;
  grants: string[];
}

interface DatabaseStatus {
  engine: string;
  configured: boolean;
  clientReady: boolean;
  host: string;
  version?: string;
  error?: string;
}

interface AccountOption {
  username: string;
  primaryDomain: string;
}

interface CredentialNotice {
  label: string;
  user: string;
  password: string;
  database: string;
}

export const DatabasesView: React.FC<{ language: Language }> = ({ language }) => {
  const t = (key: any) => getTranslation(language, key);
  const session = readSession();
  const lockedAccount = session?.role === 'user' ? session.username : '';

  const [accounts, setAccounts] = useState<AccountOption[]>([]);
  const [account, setAccount] = useState(lockedAccount);
  const [databases, setDatabases] = useState<ManagedDatabase[]>([]);
  const [users, setUsers] = useState<DatabaseUser[]>([]);
  const [status, setStatus] = useState<DatabaseStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [credential, setCredential] = useState<CredentialNotice | null>(null);
  const [busy, setBusy] = useState('');

  const [showDatabaseForm, setShowDatabaseForm] = useState(false);
  const [databaseName, setDatabaseName] = useState('');
  const [showUserForm, setShowUserForm] = useState(false);
  const [userName, setUserName] = useState('');
  const [userDatabase, setUserDatabase] = useState('');

  const loadAccounts = useCallback(async () => {
    if (lockedAccount) {
      setAccounts([{ username: lockedAccount, primaryDomain: '' }]);
      return;
    }
    try {
      const data = await apiFetch<Array<{ username: string; primaryDomain: string }>>('/api/v1/hosting/accounts');
      const list = Array.isArray(data) ? data : [];
      setAccounts(list);
      setAccount((current) => (current && list.some((item) => item.username === current) ? current : list[0]?.username || ''));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  }, [lockedAccount]);

  const loadStatus = useCallback(async () => {
    try {
      const data = await apiFetch<DatabaseStatus>('/api/v1/databases/status');
      setStatus(data);
    } catch (cause) {
      setStatus(null);
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  }, []);

  const loadAccountResources = useCallback(async (username: string) => {
    if (!username) {
      setDatabases([]);
      setUsers([]);
      return;
    }
    setLoading(true);
    setError('');
    const query = `?username=${encodeURIComponent(username)}`;
    const [databaseResult, userResult] = await Promise.allSettled([
      apiFetch<ManagedDatabase[]>(`/api/v1/databases${query}`),
      apiFetch<DatabaseUser[]>(`/api/v1/databases/users${query}`)
    ]);
    if (databaseResult.status === 'fulfilled') {
      setDatabases(Array.isArray(databaseResult.value) ? databaseResult.value : []);
    } else {
      setDatabases([]);
      setError(databaseResult.reason instanceof Error ? databaseResult.reason.message : String(databaseResult.reason));
    }
    if (userResult.status === 'fulfilled') {
      setUsers(Array.isArray(userResult.value) ? userResult.value : []);
    } else {
      setUsers([]);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    void loadAccounts();
    void loadStatus();
  }, [loadAccounts, loadStatus]);

  useEffect(() => {
    void loadAccountResources(account);
  }, [account, loadAccountResources]);

  const refresh = async () => {
    await Promise.all([loadStatus(), loadAccountResources(account)]);
  };

  const requiresConfiguration = status !== null && !status.configured;

  const createDatabase = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!account) return;
    setBusy('database');
    setError('');
    try {
      const response = await apiFetch<{
        database: ManagedDatabase;
        user: { name: string; password: string };
      }>('/api/v1/databases', {
        method: 'POST',
        body: JSON.stringify({ username: account, name: databaseName.trim().toLowerCase() })
      });
      setCredential({
        label: t('dbCreatedNotice'),
        user: response.user.name,
        password: response.user.password,
        database: response.database.name
      });
      setShowDatabaseForm(false);
      setDatabaseName('');
      await refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  const removeDatabase = async (database: ManagedDatabase) => {
    if (!window.confirm(`${t('dbDeleteConfirm')} ${database.name}?`)) return;
    setBusy(database.name);
    setError('');
    try {
      await apiFetch(`/api/v1/databases?username=${encodeURIComponent(account)}&name=${encodeURIComponent(database.name.replace(`${account}_`, ''))}`, {
        method: 'DELETE'
      });
      await refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  const createUser = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!account) return;
    setBusy('user');
    setError('');
    try {
      const response = await apiFetch<{ user: DatabaseUser; password: string }>('/api/v1/databases/users', {
        method: 'POST',
        body: JSON.stringify({
          username: account,
          name: userName.trim().toLowerCase(),
          database: userDatabase || undefined
        })
      });
      setCredential({
        label: t('dbUserCreatedNotice'),
        user: response.user.name,
        password: response.password,
        database: userDatabase || response.user.name
      });
      setShowUserForm(false);
      setUserName('');
      setUserDatabase('');
      await refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  const rotatePassword = async (user: DatabaseUser) => {
    if (!window.confirm(`${t('dbRotateConfirm')} ${user.name}?`)) return;
    setBusy(user.name);
    setError('');
    try {
      const response = await apiFetch<{ name: string; password: string }>('/api/v1/databases/users', {
        method: 'PUT',
        body: JSON.stringify({ username: account, name: user.name })
      });
      setCredential({
        label: t('dbRotatedNotice'),
        user: response.name,
        password: response.password,
        database: user.grants[0] || ''
      });
      await refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  const removeUser = async (user: DatabaseUser) => {
    if (!window.confirm(`${t('dbUserDeleteConfirm')} ${user.name}?`)) return;
    setBusy(user.name);
    setError('');
    try {
      await apiFetch(`/api/v1/databases/users?username=${encodeURIComponent(account)}&name=${encodeURIComponent(user.name.replace(`${account}_`, ''))}`, {
        method: 'DELETE'
      });
      await refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy('');
    }
  };

  const totalSize = useMemo(() => databases.reduce((sum, database) => sum + database.sizeBytes, 0), [databases]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <OpsHeader
        title={t('navDatabases')}
        subtitle={t('dbSubtitle')}
        action={
          <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => void refresh()} disabled={loading}>
              <RefreshCw size={14} />
              {t('refresh')}
            </button>
            <button
              type="button"
              className="btn btn-secondary btn-sm"
              onClick={() => setShowUserForm((value) => !value)}
              disabled={requiresConfiguration || databases.length === 0}
            >
              <UserRound size={14} />
              {t('dbNewUser')}
            </button>
            <button
              type="button"
              className="btn btn-primary btn-sm"
              onClick={() => setShowDatabaseForm((value) => !value)}
              disabled={requiresConfiguration}
            >
              <Plus size={14} />
              {t('dbNew')}
            </button>
          </div>
        }
      />

      {error ? <OpsError message={error} onDismiss={() => setError('')} /> : null}

      {status ? (
        <div className="glass-card" style={{ padding: '12px 16px', display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap', fontSize: '0.8rem' }}>
          <Server size={15} style={{ color: requiresConfiguration ? '#f59e0b' : 'var(--status-online)' }} />
          <span style={{ fontWeight: 700 }}>{status.engine.toUpperCase()}</span>
          <span className="font-mono" style={{ color: 'var(--text-muted)' }}>{status.host || '-'}</span>
          {status.version ? <span className="font-mono" style={{ color: 'var(--text-muted)' }}>{status.version}</span> : null}
          <span className="badge-pill" style={{
            background: requiresConfiguration ? 'rgba(245, 158, 11, 0.14)' : 'rgba(16, 185, 129, 0.12)',
            color: requiresConfiguration ? 'var(--status-warning)' : 'var(--status-online)'
          }}>
            {requiresConfiguration ? t('dbNotConfigured') : t('dbReady')}
          </span>
          {status.error ? <span style={{ color: '#ef4444' }}>{status.error}</span> : null}
        </div>
      ) : null}

      {credential ? (
        <div className="glass-card" style={{ padding: '14px 16px', borderColor: 'rgba(16, 185, 129, 0.35)', display: 'flex', flexDirection: 'column', gap: '6px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ fontSize: '0.84rem', fontWeight: 700 }}>{credential.label}</span>
            <button type="button" className="btn-icon" onClick={() => setCredential(null)}>
              <X size={16} />
            </button>
          </div>
          <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{t('dbShownOnce')}</div>
          <div className="font-mono" style={{ fontSize: '0.8rem', display: 'grid', gap: '3px' }}>
            <span>{t('dbThDatabase')}: {credential.database || '-'}</span>
            <span>{t('dbThUser')}: {credential.user}</span>
            <span>{t('dbThPassword')}: {credential.password}</span>
          </div>
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

      {showDatabaseForm ? (
        <form className="glass-panel" style={{ padding: '18px', display: 'flex', flexDirection: 'column', gap: '12px' }} onSubmit={createDatabase}>
          <div style={{ fontSize: '0.9rem', fontWeight: 700 }}>{t('dbNew')}</div>
          <p style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{t('dbNameHint')}</p>
          <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap', alignItems: 'flex-end' }}>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.74rem', fontWeight: 600, color: 'var(--text-muted)', minWidth: '220px' }}>
              {t('dbNameSuffix')}
              <input
                className="input-glass"
                value={databaseName}
                onChange={(event) => setDatabaseName(event.target.value)}
                pattern="[a-z][a-z0-9_]{0,20}"
                title="Lowercase letters, digits, underscore; must start with a letter"
                required
              />
            </label>
            <span className="font-mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)', paddingBottom: '8px' }}>
              {account ? `${account}_` : ''}{databaseName || '...'}
            </span>
            <button type="submit" className="btn btn-primary btn-sm" disabled={busy === 'database'}>
              {t('create')}
            </button>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => setShowDatabaseForm(false)}>
              {t('cancel')}
            </button>
          </div>
        </form>
      ) : null}

      {showUserForm ? (
        <form className="glass-panel" style={{ padding: '18px', display: 'flex', flexDirection: 'column', gap: '12px' }} onSubmit={createUser}>
          <div style={{ fontSize: '0.9rem', fontWeight: 700 }}>{t('dbNewUser')}</div>
          <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap', alignItems: 'flex-end' }}>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.74rem', fontWeight: 600, color: 'var(--text-muted)', minWidth: '200px' }}>
              {t('dbNameSuffix')}
              <input
                className="input-glass"
                value={userName}
                onChange={(event) => setUserName(event.target.value)}
                pattern="[a-z][a-z0-9_]{0,20}"
                required
              />
            </label>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.74rem', fontWeight: 600, color: 'var(--text-muted)', minWidth: '220px' }}>
              {t('dbThDatabase')}
              <select className="input-glass" value={userDatabase} onChange={(event) => setUserDatabase(event.target.value)}>
                {databases.map((database) => (
                  <option key={database.name} value={database.name.replace(`${account}_`, '')}>
                    {database.name}
                  </option>
                ))}
              </select>
            </label>
            <button type="submit" className="btn btn-primary btn-sm" disabled={busy === 'user'}>
              {t('create')}
            </button>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => setShowUserForm(false)}>
              {t('cancel')}
            </button>
          </div>
        </form>
      ) : null}

      {loading && databases.length === 0 ? (
        <div className="glass-panel" style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>{t('appsLoading')}</div>
      ) : !account ? (
        <OpsEmpty message={t('dbNoAccounts')} />
      ) : databases.length === 0 ? (
        <OpsEmpty message={t('dbEmpty')} />
      ) : (
        <div className="glass-panel" style={{ overflowX: 'auto' }}>
          <div style={{ padding: '12px 16px', fontSize: '0.78rem', color: 'var(--text-muted)' }}>
            {databases.length} {t('dbThDatabase')} &middot; {formatBytes(totalSize)}
          </div>
          <table className="glass-table">
            <thead>
              <tr>
                <th>{t('dbThDatabase')}</th>
                <th>{t('dbThCharset')}</th>
                <th>{t('dbThTables')}</th>
                <th>{t('dbThSize')}</th>
                <th style={{ textAlign: 'right' }}>{t('thActions')}</th>
              </tr>
            </thead>
            <tbody>
              {databases.map((database) => (
                <tr key={database.name}>
                  <td className="font-mono" style={{ fontSize: '0.78rem', fontWeight: 700 }}>
                    <Database size={13} style={{ verticalAlign: '-2px', marginRight: '6px', color: 'var(--brand-primary)' }} />
                    {database.name}
                  </td>
                  <td className="font-mono" style={{ fontSize: '0.74rem' }}>{database.charset}</td>
                  <td className="font-mono" style={{ fontSize: '0.76rem' }}>{database.tables}</td>
                  <td className="font-mono" style={{ fontSize: '0.76rem' }}>{formatBytes(database.sizeBytes)}</td>
                  <td style={{ textAlign: 'right' }}>
                    <button type="button" className="btn btn-secondary btn-sm" onClick={() => void removeDatabase(database)} disabled={busy === database.name}>
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
        <h2 style={{ fontSize: '1rem', fontWeight: 700, marginBottom: '10px' }}>{t('dbUsersTitle')}</h2>
        {users.length === 0 ? (
          <OpsEmpty message={t('dbUsersEmpty')} />
        ) : (
          <div className="glass-panel" style={{ overflowX: 'auto' }}>
            <table className="glass-table">
              <thead>
                <tr>
                  <th>{t('dbThUser')}</th>
                  <th>{t('dbThHost')}</th>
                  <th>{t('dbThGrants')}</th>
                  <th style={{ textAlign: 'right' }}>{t('thActions')}</th>
                </tr>
              </thead>
              <tbody>
                {users.map((user) => (
                  <tr key={`${user.name}@${user.host}`}>
                    <td className="font-mono" style={{ fontSize: '0.78rem', fontWeight: 700 }}>{user.name}</td>
                    <td className="font-mono" style={{ fontSize: '0.74rem' }}>{user.host}</td>
                    <td className="font-mono" style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>
                      {user.grants.length > 0 ? user.grants.join(', ') : '-'}
                    </td>
                    <td style={{ textAlign: 'right', display: 'flex', gap: '6px', justifyContent: 'flex-end' }}>
                      <button type="button" className="btn btn-secondary btn-sm" onClick={() => void rotatePassword(user)} disabled={busy === user.name}>
                        <KeyRound size={12} />
                        {t('dbRotate')}
                      </button>
                      <button type="button" className="btn btn-secondary btn-sm" onClick={() => void removeUser(user)} disabled={busy === user.name}>
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
