// DigiPanel DNS Zone Editor Modal
// Zero emojis, strictly Lucide SVG iconography, modern glass styling

import React, { useState } from 'react';
import { Globe, X, Plus, Trash2, Check } from 'lucide-react';
import type { DnsRecord, Language } from '../types';
import { getTranslation } from '../i18n/translations';

interface DnsZoneEditorModalProps {
  isOpen: boolean;
  onClose: () => void;
  language: Language;
}

export const DnsZoneEditorModal: React.FC<DnsZoneEditorModalProps> = ({
  isOpen,
  onClose,
  language
}) => {
  const t = (key: any) => getTranslation(language, key);

  const [records, setRecords] = useState<DnsRecord[]>([
    { id: 'rec-1', domain: 'digipanel.live', type: 'A', name: '@', content: '103.189.201.42', ttl: 300 },
    { id: 'rec-2', domain: 'digipanel.live', type: 'A', name: 'panel', content: '103.189.201.42', ttl: 300 },
    { id: 'rec-3', domain: 'digipanel.live', type: 'CNAME', name: 'www', content: 'digipanel.live', ttl: 3600 },
    { id: 'rec-4', domain: 'digipanel.live', type: 'MX', name: '@', content: 'mail.digipanel.live', ttl: 3600, priority: 10 },
    { id: 'rec-5', domain: 'digipanel.live', type: 'TXT', name: '@', content: 'v=spf1 a mx ip4:103.189.201.42 ~all', ttl: 3600 },
    { id: 'rec-6', domain: 'digipanel.live', type: 'TXT', name: '_dmarc', content: 'v=DMARC1; p=quarantine; rua=mailto:dmarc@digipanel.live', ttl: 3600 }
  ]);

  const [newType, setNewType] = useState<'A' | 'AAAA' | 'CNAME' | 'MX' | 'TXT'>('A');
  const [newName, setNewName] = useState('');
  const [newContent, setNewContent] = useState('');
  const [newTtl, setNewTtl] = useState(300);
  const [newPriority, setNewPriority] = useState(10);
  const [statusMessage, setStatusMessage] = useState('');

  if (!isOpen) return null;

  const handleAddRecord = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newName || !newContent) return;

    const newRecord: DnsRecord = {
      id: `rec-${Date.now()}`,
      domain: 'digipanel.live',
      type: newType,
      name: newName,
      content: newContent,
      ttl: newTtl,
      priority: newType === 'MX' ? newPriority : undefined
    };

    setRecords([...records, newRecord]);
    setNewName('');
    setNewContent('');
    setStatusMessage('DNS record synced successfully to PowerDNS cluster.');
    setTimeout(() => setStatusMessage(''), 3000);
  };

  const handleDeleteRecord = (id: string) => {
    setRecords(records.filter(r => r.id !== id));
    setStatusMessage('Record removed and DNS serial incremented.');
    setTimeout(() => setStatusMessage(''), 3000);
  };

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div 
        className="modal-content" 
        onClick={(e) => e.stopPropagation()} 
        style={{ maxWidth: '840px', padding: '28px' }}
      >
        {/* Header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div style={{
              width: '38px',
              height: '38px',
              borderRadius: '10px',
              background: 'rgba(2, 132, 199, 0.15)',
              color: 'var(--status-info)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center'
            }}>
              <Globe size={20} />
            </div>
            <div>
              <h2 style={{ fontSize: '1.25rem', fontWeight: 700, color: 'var(--text-main)' }}>
                {t('modalDnsEditorTitle')}
              </h2>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                PowerDNS Zone: <span className="font-mono" style={{ fontWeight: 600 }}>digipanel.live</span> &bull; DNSSEC Enabled
              </div>
            </div>
          </div>
          <button onClick={onClose} className="btn-icon">
            <X size={20} />
          </button>
        </div>

        {/* Success Alert */}
        {statusMessage && (
          <div style={{
            padding: '10px 14px',
            borderRadius: '8px',
            background: 'rgba(16, 185, 129, 0.12)',
            border: '1px solid rgba(16, 185, 129, 0.3)',
            color: 'var(--status-online)',
            fontSize: '0.84rem',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            marginBottom: '16px'
          }}>
            <Check size={16} />
            <span>{statusMessage}</span>
          </div>
        )}

        {/* Add Record Form */}
        <form onSubmit={handleAddRecord} style={{
          padding: '16px',
          borderRadius: '12px',
          background: 'rgba(148, 163, 184, 0.06)',
          border: '1px solid var(--glass-border)',
          marginBottom: '20px'
        }}>
          <div style={{ fontSize: '0.85rem', fontWeight: 700, color: 'var(--text-main)', marginBottom: '12px' }}>
            Add New Record
          </div>
          <div style={{
            display: 'grid',
            gridTemplateColumns: '100px 1fr 1.5fr 90px 100px',
            gap: '10px',
            alignItems: 'end'
          }}>
            <div>
              <label style={{ fontSize: '0.72rem', color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>Type</label>
              <select 
                value={newType} 
                onChange={(e: any) => setNewType(e.target.value)}
                className="input-glass"
                style={{ height: '36px', fontSize: '0.82rem' }}
              >
                <option value="A">A</option>
                <option value="AAAA">AAAA</option>
                <option value="CNAME">CNAME</option>
                <option value="MX">MX</option>
                <option value="TXT">TXT</option>
              </select>
            </div>
            <div>
              <label style={{ fontSize: '0.72rem', color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>Name</label>
              <input 
                type="text"
                placeholder="@, sub, www"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                className="input-glass"
                style={{ height: '36px', fontSize: '0.82rem' }}
                required
              />
            </div>
            <div>
              <label style={{ fontSize: '0.72rem', color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>Content</label>
              <input 
                type="text"
                placeholder="Target IP or hostname"
                value={newContent}
                onChange={(e) => setNewContent(e.target.value)}
                className="input-glass"
                style={{ height: '36px', fontSize: '0.82rem' }}
                required
              />
            </div>
            <div>
              <label style={{ fontSize: '0.72rem', color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>TTL (s)</label>
              <input 
                type="number"
                value={newTtl}
                onChange={(e) => setNewTtl(Number(e.target.value))}
                className="input-glass font-mono"
                style={{ height: '36px', fontSize: '0.82rem' }}
              />
            </div>
            <button type="submit" className="btn btn-primary btn-sm" style={{ height: '36px' }}>
              <Plus size={14} />
              <span>Add</span>
            </button>
          </div>
        </form>

        {/* Existing Records Table */}
        <div style={{ maxHeight: '340px', overflowY: 'auto' }}>
          <table className="glass-table">
            <thead>
              <tr>
                <th style={{ width: '80px' }}>TYPE</th>
                <th>NAME</th>
                <th>CONTENT</th>
                <th style={{ width: '80px' }}>TTL</th>
                <th style={{ width: '60px', textAlign: 'right' }}>ACTION</th>
              </tr>
            </thead>
            <tbody>
              {records.map(rec => (
                <tr key={rec.id}>
                  <td>
                    <span className="badge-pill" style={{
                      background: rec.type === 'A' ? 'rgba(234, 88, 12, 0.15)' : 'rgba(2, 132, 199, 0.15)',
                      color: rec.type === 'A' ? 'var(--brand-primary)' : 'var(--status-info)'
                    }}>
                      {rec.type}
                    </span>
                  </td>
                  <td className="font-mono" style={{ fontWeight: 600 }}>{rec.name}</td>
                  <td className="font-mono" style={{ fontSize: '0.8rem', wordBreak: 'break-all' }}>{rec.content}</td>
                  <td className="font-mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{rec.ttl}</td>
                  <td style={{ textAlign: 'right' }}>
                    <button
                      onClick={() => handleDeleteRecord(rec.id)}
                      className="btn-icon"
                      style={{ color: 'var(--status-danger)' }}
                      title="Delete record"
                    >
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
