// DigiPanel File Manager.
// Browses the live account tree through the core files API. The backend confines
// every path to the account home after symlink resolution, so this view offers
// navigation and preview only.

import React, { useEffect, useState } from 'react';
import { Folder, FileCode, X, RefreshCw, ArrowUp, AlertCircle } from 'lucide-react';
import type { ApiAccount, Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';
import { formatBytes, OpsEmpty } from './opsShared';

interface FileEntry {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  permissions: string;
  modified: string;
}

interface FileManagerModalProps {
  isOpen: boolean;
  onClose: () => void;
  language: Language;
  account: ApiAccount | null;
}

export const FileManagerModal: React.FC<FileManagerModalProps> = ({
  isOpen,
  onClose,
  language,
  account
}) => {
  const t = (key: any) => getTranslation(language, key);

  const [path, setPath] = useState('');
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [selected, setSelected] = useState<FileEntry | null>(null);
  const [content, setContent] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const load = async (nextPath: string) => {
    if (!account) return;
    setLoading(true);
    setError('');
    try {
      const data = await apiFetch<FileEntry[]>(
        `/api/v1/files/list?username=${encodeURIComponent(account.username)}&path=${encodeURIComponent(nextPath)}`
      );
      setFiles(Array.isArray(data) ? data : []);
      setPath(nextPath);
      setSelected(null);
      setContent(null);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
      setFiles([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen && account) {
      void load('');
    }
  }, [isOpen, account?.username]);

  if (!isOpen) return null;

  const openEntry = async (entry: FileEntry) => {
    if (entry.isDir) {
      await load(entry.path);
      return;
    }
    setSelected(entry);
    setContent(null);
    setError('');
    try {
      const data = await apiFetch<{ content: string; truncated: boolean }>(
        `/api/v1/files/content?username=${encodeURIComponent(account!.username)}&file=${encodeURIComponent(entry.path)}`
      );
      setContent(data.content);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  const goUp = () => {
    if (!path) return;
    const segments = path.split('/').filter(Boolean);
    segments.pop();
    void load(segments.join('/'));
  };

  const root = account ? account.homeDir : '';

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        className="modal-content"
        onClick={(event) => event.stopPropagation()}
        style={{ maxWidth: '980px', padding: '24px' }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div style={{
              width: '38px',
              height: '38px',
              borderRadius: '10px',
              background: 'rgba(245, 158, 11, 0.15)',
              color: 'var(--brand-accent)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center'
            }}>
              <Folder size={20} />
            </div>
            <div>
              <h2 style={{ fontSize: '1.25rem', fontWeight: 700, color: 'var(--text-main)' }}>{t('toolFileManager')}</h2>
              <div className="font-mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                {root}{path ? `/${path}` : ''}
              </div>
            </div>
          </div>
          <button type="button" onClick={onClose} className="btn-icon">
            <X size={20} />
          </button>
        </div>

        {!account ? (
          <OpsEmpty message={t('clientNoAccount')} />
        ) : (
          <>
            <div style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              padding: '10px 14px',
              background: 'rgba(148, 163, 184, 0.06)',
              borderRadius: '10px',
              marginBottom: '16px',
              border: '1px solid var(--glass-border)',
              gap: '10px'
            }}>
              <div style={{ display: 'flex', gap: '8px' }}>
                <button type="button" className="btn btn-secondary btn-sm" onClick={goUp} disabled={!path || loading}>
                  <ArrowUp size={14} />
                  {t('fileUp')}
                </button>
                <button type="button" className="btn btn-secondary btn-sm" onClick={() => void load(path)} disabled={loading}>
                  <RefreshCw size={14} />
                  {t('refresh')}
                </button>
              </div>
              <div style={{ fontSize: '0.76rem', color: 'var(--text-muted)' }}>
                {formatBytes(account.diskUsedBytes)} / {formatBytes(account.diskLimitBytes)}
              </div>
            </div>

            {error ? (
              <div className="glass-card" style={{ padding: '10px 14px', marginBottom: '12px', display: 'flex', gap: '8px', alignItems: 'center', fontSize: '0.8rem', borderColor: 'rgba(239, 68, 68, 0.35)' }}>
                <AlertCircle size={14} style={{ color: '#ef4444' }} />
                <span>{error}</span>
              </div>
            ) : null}

            <div style={{ display: 'grid', gridTemplateColumns: content !== null ? '1.1fr 1.3fr' : '1fr', gap: '16px' }}>
              <div style={{ maxHeight: '430px', overflowY: 'auto' }}>
                {loading ? (
                  <div style={{ padding: '30px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '0.82rem' }}>{t('appsLoading')}</div>
                ) : files.length === 0 ? (
                  <OpsEmpty message={t('accessLogsEmpty')} />
                ) : (
                  <table className="glass-table">
                    <thead>
                      <tr>
                        <th>{t('fileThName')}</th>
                        <th>{t('fileThSize')}</th>
                        <th>{t('fileThPerms')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {files.map((file) => (
                        <tr
                          key={file.path}
                          onClick={() => void openEntry(file)}
                          style={{
                            cursor: 'pointer',
                            background: selected?.path === file.path ? 'rgba(234, 88, 12, 0.08)' : 'transparent'
                          }}
                        >
                          <td>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                              {file.isDir ? (
                                <Folder size={16} style={{ color: 'var(--brand-accent)' }} />
                              ) : (
                                <FileCode size={16} style={{ color: 'var(--status-info)' }} />
                              )}
                              <span style={{ fontWeight: 600 }}>{file.name}</span>
                            </div>
                          </td>
                          <td className="font-mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                            {file.isDir ? '-' : formatBytes(file.size)}
                          </td>
                          <td className="font-mono" style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>{file.permissions}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>

              {content !== null && selected ? (
                <div style={{
                  display: 'flex',
                  flexDirection: 'column',
                  background: 'rgba(10, 14, 23, 0.85)',
                  borderRadius: '10px',
                  border: '1px solid var(--glass-border)',
                  padding: '14px',
                  color: '#f8fafc',
                  maxHeight: '430px'
                }}>
                  <div style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    paddingBottom: '10px',
                    borderBottom: '1px solid rgba(255, 255, 255, 0.1)',
                    marginBottom: '10px'
                  }}>
                    <span className="font-mono" style={{ fontSize: '0.82rem', fontWeight: 600 }}>{selected.name}</span>
                    <button type="button" onClick={() => { setContent(null); setSelected(null); }} className="btn-icon" style={{ color: '#94a3b8' }}>
                      <X size={14} />
                    </button>
                  </div>
                  <pre className="font-mono" style={{ flex: 1, margin: 0, overflow: 'auto', fontSize: '0.78rem', lineHeight: 1.6, color: '#38bdf8', whiteSpace: 'pre-wrap' }}>
                    {content}
                  </pre>
                </div>
              ) : null}
            </div>
          </>
        )}
      </div>
    </div>
  );
};
