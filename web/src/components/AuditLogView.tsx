// DigiPanel Audit Trail view.
// Reads the cryptographically chained audit ledger from the core and reports
// every recorded administrative action with its chain hash.

import React, { useEffect, useMemo, useState } from 'react';
import { RefreshCw, Search, ShieldCheck, ShieldAlert, Download } from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';
import { OpsEmpty, OpsError, OpsHeader, formatDate } from './opsShared';

interface AuditEntry {
  id: number;
  actor: string;
  action: string;
  resourceType: string;
  resourceId: string;
  payloadJson: string;
  ipAddress: string;
  prevHash: string;
  entryHash: string;
  createdAt: string;
}

interface AuditResponse {
  logs: AuditEntry[];
  isChainValid: boolean;
  count: number;
}

export const AuditLogView: React.FC<{ language: Language }> = ({ language }) => {
  const t = (key: any) => getTranslation(language, key);
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [chainValid, setChainValid] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');

  const load = async () => {
    try {
      setLoading(true);
      setError('');
      const data = await apiFetch<AuditResponse>('/api/v1/audit/logs');
      setEntries(Array.isArray(data.logs) ? data.logs : []);
      setChainValid(Boolean(data.isChainValid));
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
    if (!needle) return entries;
    return entries.filter((entry) =>
      [entry.actor, entry.action, entry.resourceType, entry.resourceId, entry.ipAddress]
        .join(' ')
        .toLowerCase()
        .includes(needle)
    );
  }, [entries, search]);

  const download = () => {
    const blob = new Blob([JSON.stringify({ isChainValid: chainValid, logs: entries }, null, 2)], {
      type: 'application/json'
    });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = `digipanel-audit-${new Date().toISOString().replace(/[:.]/g, '-')}.json`;
    anchor.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <OpsHeader
        title={t('auditTitle')}
        subtitle={t('auditSubtitle')}
        action={
          <div style={{ display: 'flex', gap: '8px' }}>
            <button type="button" className="btn btn-secondary btn-sm" onClick={download} disabled={entries.length === 0}>
              <Download size={14} />
              {t('auditExport')}
            </button>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => void load()} disabled={loading}>
              <RefreshCw size={14} />
              {t('refresh')}
            </button>
          </div>
        }
      />

      {error ? <OpsError message={error} onDismiss={() => setError('')} /> : null}

      <div
        className="glass-card"
        style={{
          padding: '12px 16px',
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          borderColor: chainValid ? 'rgba(16, 185, 129, 0.35)' : 'rgba(239, 68, 68, 0.35)'
        }}
      >
        {chainValid ? (
          <ShieldCheck size={17} style={{ color: 'var(--status-online)' }} />
        ) : (
          <ShieldAlert size={17} style={{ color: '#ef4444' }} />
        )}
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: '0.82rem', fontWeight: 700 }}>
            {chainValid ? t('auditChainValid') : t('auditChainBroken')}
          </div>
          <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>
            {entries.length} {t('auditEntries')}
          </div>
        </div>
      </div>

      <div style={{ position: 'relative', maxWidth: '400px' }}>
        <Search size={15} style={{ position: 'absolute', left: '12px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
        <input
          className="input-glass"
          style={{ paddingLeft: '36px', height: '38px' }}
          placeholder={t('auditSearchPlaceholder')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>

      {loading && entries.length === 0 ? (
        <div className="glass-panel" style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>{t('appsLoading')}</div>
      ) : filtered.length === 0 ? (
        <OpsEmpty message={entries.length === 0 ? t('auditEmpty') : t('accountsNoMatches')} />
      ) : (
        <div className="glass-panel" style={{ overflowX: 'auto' }}>
          <table className="glass-table">
            <thead>
              <tr>
                <th>#</th>
                <th>{t('auditTime')}</th>
                <th>{t('auditActor')}</th>
                <th>{t('auditAction')}</th>
                <th>{t('auditResource')}</th>
                <th>IP</th>
                <th>{t('auditHash')}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((entry) => (
                <tr key={`${entry.id}-${entry.entryHash}`}>
                  <td className="font-mono" style={{ fontSize: '0.72rem' }}>{entry.id}</td>
                  <td style={{ fontSize: '0.74rem', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>{formatDate(entry.createdAt)}</td>
                  <td style={{ fontSize: '0.8rem', fontWeight: 600 }}>{entry.actor}</td>
                  <td>
                    <div style={{ fontSize: '0.78rem', fontWeight: 600 }}>{entry.action}</div>
                    {entry.payloadJson ? (
                      <div className="font-mono" style={{ fontSize: '0.66rem', color: 'var(--text-dim)', maxWidth: '240px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={entry.payloadJson}>
                        {entry.payloadJson}
                      </div>
                    ) : null}
                  </td>
                  <td style={{ fontSize: '0.74rem' }}>
                    <div>{entry.resourceType}</div>
                    <div className="font-mono" style={{ fontSize: '0.68rem', color: 'var(--text-muted)' }}>{entry.resourceId}</div>
                  </td>
                  <td className="font-mono" style={{ fontSize: '0.72rem' }}>{entry.ipAddress}</td>
                  <td className="font-mono" style={{ fontSize: '0.66rem', color: 'var(--text-dim)', maxWidth: '160px', overflow: 'hidden', textOverflow: 'ellipsis' }} title={`${entry.prevHash} -> ${entry.entryHash}`}>
                    {entry.entryHash.slice(0, 16)}...
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
