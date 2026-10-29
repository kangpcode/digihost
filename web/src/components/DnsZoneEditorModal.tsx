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

  const [records, setRecords] = useState<DnsRecord[]>([]);
  const [newType, setNewType] = useState<'A' | 'AAAA' | 'CNAME' | 'MX' | 'TXT'>('A');
  const [newName, setNewName] = useState('');
  const [newContent, setNewContent] = useState('');
  const [newTtl, setNewTtl] = useState(300);
  const [newPriority] = useState(10);
  const [statusMessage, setStatusMessage] = useState('');

  const fetchDns = async () => {
    try {
      const res = await fetch('/api/v1/dns/records');
      if (res.ok) {
        const data = await res.json();
        setRecords(data);
      }
    } catch {
      // fallback
    }
  };

  React.useEffect(() => {
    if (isOpen) {
      fetchDns();
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const handleAddRecord = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newName || !newContent) return;

    try {
      const res = await fetch('/api/v1/dns/records', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain: 'digipanel.live',
          type: newType,
          name: newName,
          content: newContent,
          ttl: newTtl,
          priority: newType === 'MX' ? newPriority : undefined
        })
      });
      if (res.ok) {
        setNewName('');
        setNewContent('');
        setStatusMessage(t('dnsRecordSynced'));
        fetchDns();
        setTimeout(() => setStatusMessage(''), 3000);
      }
    } catch {
      setStatusMessage(t('dnsRecordError'));
    }
  };

  const handleDeleteRecord = async (id: string) => {
    try {
      const res = await fetch(`/api/v1/dns/records?id=${id}`, {
        method: 'DELETE'
      });
      if (res.ok) {
        setStatusMessage(t('dnsRecordDeleted'));
        fetchDns();
        setTimeout(() => setStatusMessage(''), 3000);
      }
    } catch {
      // fallback
    }
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
                PowerDNS {t('dnsZoneLabel')}: <span className="font-mono" style={{ fontWeight: 600 }}>digipanel.live</span> &bull; {t('dnsDnssecEnabled')}
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
            {t('dnsAddRecordTitle')}
          </div>
          <div style={{
            display: 'grid',
            gridTemplateColumns: '100px 1fr 1.5fr 90px 100px',
            gap: '10px',
            alignItems: 'end'
          }}>
            <div>
              <label style={{ fontSize: '0.72rem', color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>{t('dnsLabelType')}</label>
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
              <label style={{ fontSize: '0.72rem', color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>{t('dnsLabelName')}</label>
              <input 
                type="text"
                placeholder={t('dnsPlaceholderName')}
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                className="input-glass"
                style={{ height: '36px', fontSize: '0.82rem' }}
                required
              />
            </div>
            <div>
              <label style={{ fontSize: '0.72rem', color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>{t('dnsLabelContent')}</label>
              <input 
                type="text"
                placeholder={t('dnsPlaceholderContent')}
                value={newContent}
                onChange={(e) => setNewContent(e.target.value)}
                className="input-glass"
                style={{ height: '36px', fontSize: '0.82rem' }}
                required
              />
            </div>
            <div>
              <label style={{ fontSize: '0.72rem', color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>{t('dnsLabelTtl')}</label>
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
              <span>{t('create')}</span>
            </button>
          </div>
        </form>

        {/* Existing Records Table */}
        <div style={{ maxHeight: '340px', overflowY: 'auto' }}>
          <table className="glass-table">
            <thead>
              <tr>
                <th style={{ width: '80px' }}>{t('dnsLabelType')}</th>
                <th>{t('dnsLabelName')}</th>
                <th>{t('dnsLabelContent')}</th>
                <th style={{ width: '80px' }}>{t('dnsThTtl')}</th>
                <th style={{ width: '60px', textAlign: 'right' }}>{t('thActions')}</th>
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
                      title={t('delete')}
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
