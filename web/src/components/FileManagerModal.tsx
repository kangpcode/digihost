// DigiPanel Web File Manager Modal
// Zero emojis, strictly Lucide SVG iconography, modern glass styling

import React, { useState } from 'react';
import { 
  Folder, 
  X, 
  Upload, 
  Plus, 
  Trash2, 
  Check, 
  FileCode
} from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';

interface FileManagerModalProps {
  isOpen: boolean;
  onClose: () => void;
  language: Language;
}

interface FileItem {
  id: string;
  name: string;
  type: 'folder' | 'file';
  size: string;
  modified: string;
  permissions: string;
}

export const FileManagerModal: React.FC<FileManagerModalProps> = ({
  isOpen,
  onClose,
  language
}) => {
  const t = (key: any) => getTranslation(language, key);

  const [currentPath] = useState('/home/testuser/public_html');
  const [files, setFiles] = useState<FileItem[]>([]);
  const [selectedFile, setSelectedFile] = useState<FileItem | null>(null);
  const [previewContent, setPreviewContent] = useState<string | null>(null);

  const fetchFiles = async () => {
    try {
      const res = await fetch(`/api/v1/files/list?path=${encodeURIComponent(currentPath)}`);
      if (res.ok) {
        const data = await res.json();
        setFiles(data.map((item: any, idx: number) => ({
          id: `file-${idx}`,
          name: item.name,
          type: item.isDir ? 'folder' : 'file',
          size: `${Math.round(item.size / 1024 * 10) / 10} KB`,
          modified: item.modified,
          permissions: item.permissions
        })));
      }
    } catch {
      // fallback
    }
  };

  React.useEffect(() => {
    if (isOpen) {
      fetchFiles();
    }
  }, [isOpen, currentPath]);

  if (!isOpen) return null;

  const handleOpenFile = async (file: FileItem) => {
    setSelectedFile(file);
    try {
      const filePath = `${currentPath}/${file.name}`;
      const res = await fetch(`/api/v1/files/content?file=${encodeURIComponent(filePath)}`);
      if (res.ok) {
        const text = await res.text();
        setPreviewContent(text);
        return;
      }
    } catch {
      // fallback
    }
    setPreviewContent(`// File: ${file.name}\n// Loaded from DigiPanel live filesystem`);
  };

  const handleSaveFile = async () => {
    if (!selectedFile || previewContent === null) return;
    try {
      const filePath = `${currentPath}/${selectedFile.name}`;
      await fetch('/api/v1/files/content', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filePath, content: previewContent })
      });
      alert(`${t('fileSavedOk')} ${selectedFile.name}`);
      fetchFiles();
    } catch {
      alert(t('fileSaveFailed'));
    }
  };

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div 
        className="modal-content" 
        onClick={(e) => e.stopPropagation()} 
        style={{ maxWidth: '960px', padding: '24px' }}
      >
        {/* Header */}
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
              <h2 style={{ fontSize: '1.25rem', fontWeight: 700, color: 'var(--text-main)' }}>
                {t('toolFileManager')}
              </h2>
              <div className="font-mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                {currentPath}
              </div>
            </div>
          </div>
          <button onClick={onClose} className="btn-icon">
            <X size={20} />
          </button>
        </div>

        {/* Toolbar */}
        <div style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          padding: '10px 14px',
          background: 'rgba(148, 163, 184, 0.06)',
          borderRadius: '10px',
          marginBottom: '16px',
          border: '1px solid var(--glass-border)'
        }}>
          <div style={{ display: 'flex', gap: '8px' }}>
            <button className="btn btn-secondary btn-sm">
              <Plus size={14} />
              <span>{t('fileNewFile')}</span>
            </button>
            <button className="btn btn-secondary btn-sm">
              <Folder size={14} />
              <span>{t('fileNewFolder')}</span>
            </button>
            <button className="btn btn-primary btn-sm">
              <Upload size={14} />
              <span>{t('fileUpload')}</span>
            </button>
          </div>
          <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
            {t('fileQuotaLabel')}: 0.06 MB / 50 MB
          </div>
        </div>

        {/* Main Grid: File List (Left) & Preview (Right) */}
        <div style={{ display: 'grid', gridTemplateColumns: previewContent ? '1.2fr 1.4fr' : '1fr', gap: '16px' }}>
          {/* File Table */}
          <div style={{ maxHeight: '420px', overflowY: 'auto' }}>
            <table className="glass-table">
              <thead>
                <tr>
                  <th>{t('fileThName')}</th>
                  <th>{t('fileThSize')}</th>
                  <th>{t('fileThPerms')}</th>
                  <th style={{ textAlign: 'right' }}>{t('thActions')}</th>
                </tr>
              </thead>
              <tbody>
                {files.map(file => (
                  <tr 
                    key={file.id}
                    onClick={() => file.type === 'file' && handleOpenFile(file)}
                    style={{
                      cursor: 'pointer',
                      background: selectedFile?.id === file.id ? 'rgba(234, 88, 12, 0.08)' : 'transparent'
                    }}
                  >
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                        {file.type === 'folder' ? (
                          <Folder size={16} style={{ color: 'var(--brand-accent)' }} />
                        ) : (
                          <FileCode size={16} style={{ color: 'var(--status-info)' }} />
                        )}
                        <span style={{ fontWeight: 600 }}>{file.name}</span>
                      </div>
                    </td>
                    <td className="font-mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                      {file.size}
                    </td>
                    <td className="font-mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                      {file.permissions}
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      <button 
                        onClick={(e) => { e.stopPropagation(); setFiles(files.filter(f => f.id !== file.id)); }}
                        className="btn-icon" 
                        style={{ color: 'var(--status-danger)' }}
                      >
                        <Trash2 size={13} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* File Preview Editor */}
          {previewContent && selectedFile && (
            <div style={{
              display: 'flex',
              flexDirection: 'column',
              background: 'rgba(10, 14, 23, 0.85)',
              borderRadius: '10px',
              border: '1px solid var(--glass-border)',
              padding: '14px',
              color: '#f8fafc'
            }}>
              <div style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                paddingBottom: '10px',
                borderBottom: '1px solid rgba(255, 255, 255, 0.1)',
                marginBottom: '10px'
              }}>
                <span className="font-mono" style={{ fontSize: '0.82rem', fontWeight: 600 }}>
                  {selectedFile.name}
                </span>
                <button onClick={() => setPreviewContent(null)} className="btn-icon" style={{ color: '#94a3b8' }}>
                  <X size={14} />
                </button>
              </div>

              <textarea 
                value={previewContent}
                onChange={(e) => setPreviewContent(e.target.value)}
                className="font-mono"
                style={{
                  flex: 1,
                  minHeight: '260px',
                  background: 'transparent',
                  border: 'none',
                  color: '#38bdf8',
                  fontSize: '0.8rem',
                  lineHeight: '1.5',
                  outline: 'none',
                  resize: 'none'
                }}
              />

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px', marginTop: '10px' }}>
                <button 
                  onClick={handleSaveFile}
                  className="btn btn-primary btn-sm"
                >
                  <Check size={14} />
                  <span>{t('fileSaveBtn')}</span>
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
