// DigiPanel Reseller identities view.
// Reads the real panel identities from the core directory and reports every
// account whose role grants reseller scope. No passwords or TOTP secrets are
// exposed by the API.

import React, { useEffect, useMemo, useState } from 'react';
import { RefreshCw, Search, ShieldCheck, ShieldOff, Users } from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';
import { OpsEmpty, OpsError, OpsHeader } from './opsShared';

interface PanelUser {
  username: string;
  role: string;
  twoFactorEnabled: boolean;
}

export const ResellersView: React.FC<{ language: Language }> = ({ language }) => {
  const t = (key: any) => getTranslation(language, key);
  const [users, setUsers] = useState<PanelUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');

  const load = async () => {
    try {
      setLoading(true);
      setError('');
      const data = await apiFetch<PanelUser[]>('/api/v1/users?role=reseller');
      setUsers(Array.isArray(data) ? data : []);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    if (!needle) return users;
    return users.filter((user) => user.username.toLowerCase().includes(needle));
  }, [users, search]);

  const protectedCount = users.filter((user) => user.twoFactorEnabled).length;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <OpsHeader
        title={t('resellersTitle')}
        subtitle={t('resellersSubtitle')}
        action={
          <button type="button" className="btn btn-secondary btn-sm" onClick={() => void load()} disabled={loading}>
            <RefreshCw size={14} />
            {t('refresh')}
          </button>
        }
      />

      {error ? <OpsError message={error} onDismiss={() => setError('')} /> : null}

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(190px, 1fr))', gap: '12px' }}>
        <div className="glass-card" style={{ padding: '16px', display: 'flex', alignItems: 'center', gap: '12px' }}>
          <Users size={20} style={{ color: 'var(--brand-primary)' }} />
          <div>
            <div style={{ fontSize: '1.4rem', fontWeight: 800 }}>{users.length}</div>
            <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>{t('resellersTotal')}</div>
          </div>
        </div>
        <div className="glass-card" style={{ padding: '16px', display: 'flex', alignItems: 'center', gap: '12px' }}>
          <ShieldCheck size={20} style={{ color: 'var(--status-online)' }} />
          <div>
            <div style={{ fontSize: '1.4rem', fontWeight: 800 }}>{protectedCount}</div>
            <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>{t('resellersTwoFactor')}</div>
          </div>
        </div>
        <div className="glass-card" style={{ padding: '16px', display: 'flex', alignItems: 'center', gap: '12px' }}>
          <ShieldOff size={20} style={{ color: 'var(--status-warning)' }} />
          <div>
            <div style={{ fontSize: '1.4rem', fontWeight: 800 }}>{users.length - protectedCount}</div>
            <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>{t('resellersWithoutTwoFactor')}</div>
          </div>
        </div>
      </div>

      <div style={{ position: 'relative', maxWidth: '380px' }}>
        <Search size={15} style={{ position: 'absolute', left: '12px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
        <input
          className="input-glass"
          style={{ paddingLeft: '36px', height: '38px' }}
          placeholder={t('resellersSearchPlaceholder')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>

      {loading && users.length === 0 ? (
        <div className="glass-panel" style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>{t('appsLoading')}</div>
      ) : filtered.length === 0 ? (
        <OpsEmpty message={users.length === 0 ? t('resellersEmpty') : t('accountsNoMatches')} />
      ) : (
        <div className="glass-panel" style={{ overflowX: 'auto' }}>
          <table className="glass-table">
            <thead>
              <tr>
                <th>{t('accountsAccount')}</th>
                <th>{t('role')}</th>
                <th>{t('resellersTwoFactor')}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((user) => (
                <tr key={user.username}>
                  <td style={{ fontWeight: 700, fontSize: '0.84rem' }}>{user.username}</td>
                  <td style={{ fontSize: '0.8rem', textTransform: 'capitalize' }}>{user.role}</td>
                  <td>
                    {user.twoFactorEnabled ? (
                      <span className="badge-online" style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                        <ShieldCheck size={11} /> enabled
                      </span>
                    ) : (
                      <span className="badge-warning" style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                        <ShieldOff size={11} /> disabled
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
