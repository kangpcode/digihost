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
  const [files, setFiles] = useState<FileItem[]>([
    { id: '1', name: 'wp-admin', type: 'folder', size: '4 KB', modified: '2026-10-24 11:20', permissions: '0755' },
    { id: '2', name: 'wp-content', type: 'folder', size: '4 KB', modified: '2026-10-25 14:15', permissions: '0755' },
    { id: '3', name: 'wp-includes', type: 'folder', size: '4 KB', modified: '2026-10-24 11:20', permissions: '0755' },
    { id: '4', name: '.htaccess', type: 'file', size: '284 B', modified: '2026-10-24 11:25', permissions: '0644' },
    { id: '5', name: 'index.php', type: 'file', size: '405 B', modified: '2026-10-24 11:20', permissions: '0644' },
    { id: '6', name: 'wp-config.php', type: 'file', size: '3.4 KB', modified: '2026-10-24 11:22', permissions: '0600' },
  ]);

  const [selectedFile, setSelectedFile] = useState<FileItem | null>(null);
  const [previewContent, setPreviewContent] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleOpenFile = (file: FileItem) => {
    setSelectedFile(file);
    if (file.name === 'index.php') {
      setPreviewContent(`<?php
/**
 * Front to the WordPress application. This file doesn't do anything, but loads
 * wp-blog-header.php which tells WordPress to load the theme and display it.
 *
 * @package WordPress
 */

define( 'WP_USE_THEMES', true );

/** Loads the WordPress Environment and Template */
require __DIR__ . '/wp-blog-header.php';
`);
    } else if (file.name === '.htaccess') {
      setPreviewContent(`# BEGIN WordPress
<IfModule mod_rewrite.c>
RewriteEngine On
RewriteBase /
RewriteRule ^index\\.php$ - [L]
RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteRule . /index.php [L]
</IfModule>
# END WordPress
`);
    } else {
      setPreviewContent(`// Contents of ${file.name}\n// DigiPanel Safe Sandbox`);
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
              <span>New File</span>
            </button>
            <button className="btn btn-secondary btn-sm">
              <Folder size={14} />
              <span>New Folder</span>
            </button>
            <button className="btn btn-primary btn-sm">
              <Upload size={14} />
              <span>Upload</span>
            </button>
          </div>
          <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
            Quota: 0.06 MB / 50 MB
          </div>
        </div>

        {/* Main Grid: File List (Left) & Preview (Right) */}
        <div style={{ display: 'grid', gridTemplateColumns: previewContent ? '1.2fr 1.4fr' : '1fr', gap: '16px' }}>
          {/* File Table */}
          <div style={{ maxHeight: '420px', overflowY: 'auto' }}>
            <table className="glass-table">
              <thead>
                <tr>
                  <th>NAME</th>
                  <th>SIZE</th>
                  <th>PERMS</th>
                  <th style={{ textAlign: 'right' }}>ACTION</th>
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
                  onClick={() => {
                    alert('File saved to server filesystem.');
                  }}
                  className="btn btn-primary btn-sm"
                >
                  <Check size={14} />
                  <span>Save File</span>
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
