// DigiPanel Email Infrastructure & Deliverability Modal
// Postfix, Dovecot, Rspamd, DKIM/SPF/DMARC status, and cleanliness scoring
// Zero emojis, strictly clean Lucide SVG iconography, WCAG AA compliant.

import React, { useState, useEffect } from 'react';
import { 
  X, 
  Mail, 
  Plus, 
  ShieldCheck, 
  CheckCircle, 
  AlertTriangle, 
  Send, 
  Inbox, 
  HardDrive, 
  Sparkles,
  Check
} from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';

interface Mailbox {
  email: string;
  domain: string;
  quotaMb: number;
  usedBytes: number;
  active: boolean;
  hourlyLimit: number;
}

interface CleanlinessScore {
  totalScore: number;
  spfPassed: boolean;
  dkimPassed: boolean;
  dmarcPassed: boolean;
  ptrPassed: boolean;
  spamPenaltyScore: number;
  checks: string[];
  recommendation: string;
}

interface MailDomain {
  domain: string;
  tenantId: string;
  dkimSelector: string;
  dkimRecord: string;
  spfRecord: string;
  dmarcRecord: string;
}

// Extracts the quoted TXT value from a zone-file style record line
const txtValue = (record: string) => {
  const m = record.match(/"([^"]*)"/);
  return m ? m[1] : record;
};

interface MailManagerModalProps {
  isOpen: boolean;
  onClose: () => void;
  language: Language;
}

export const MailManagerModal: React.FC<MailManagerModalProps> = ({
  isOpen,
  onClose,
  language
}) => {
  const [mailboxes, setMailboxes] = useState<Mailbox[]>([]);
  const [mailDomains, setMailDomains] = useState<MailDomain[]>([]);
  const [selectedDomain, setSelectedDomain] = useState('');
  const [loading, setLoading] = useState(false);
  const [showCreateBox, setShowCreateBox] = useState(false);
  const [showTester, setShowTester] = useState(false);
  const [actionMessage, setActionMessage] = useState<string | null>(null);

  // New Mailbox Form
  const [newEmail, setNewEmail] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [newQuotaMb, setNewQuotaMb] = useState(1024);
  const [creating, setCreating] = useState(false);

  // Deliverability Tester: operator pastes real raw message source (headers + body)
  const [testRawEmail, setTestRawEmail] = useState('');
  const [cleanlinessResult, setCleanlinessResult] = useState<CleanlinessScore | null>(null);
  const [analyzing, setAnalyzing] = useState(false);

  const t = (key: any) => getTranslation(language, key);

  const fetchMailData = async () => {
    setLoading(true);
    try {
      const [boxes, domains] = await Promise.all([
        apiFetch<Mailbox[]>('/api/v1/mail/mailboxes'),
        apiFetch<MailDomain[]>('/api/v1/mail/domains')
      ]);
      setMailboxes(boxes || []);
      setMailDomains(domains || []);
      setSelectedDomain(prev => prev || domains?.[0]?.domain || '');
    } catch (err: any) {
      setActionMessage(`${t('mailLoadFailed')}: ${err.message}`);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      setActionMessage(null);
      fetchMailData();
    }
  }, [isOpen]);

  const activeDomain = mailDomains.find(d => d.domain === selectedDomain);
  const isErrorMessage = (msg: string) => msg.toLowerCase().includes('fail');

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) onClose();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  const handleCreateMailbox = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newEmail.trim() || !newPassword.trim()) return;

    setCreating(true);
    setActionMessage(null);
    try {
      const mb = await apiFetch<Mailbox>('/api/v1/mail/mailboxes', {
        method: 'POST',
        body: JSON.stringify({
          email: newEmail.trim(),
          password: newPassword.trim(),
          quotaMb: Number(newQuotaMb)
        })
      });

      setMailboxes([mb, ...mailboxes]);
      setNewEmail('');
      setNewPassword('');
      setShowCreateBox(false);
      setActionMessage(`${t('mailCreated')}: ${mb.email}`);
    } catch (err: any) {
      setActionMessage(`${t('actionFailed')}: ${err.message}`);
    } finally {
      setCreating(false);
    }
  };

  const handleAnalyzeCleanliness = async () => {
    setAnalyzing(true);
    setActionMessage(null);
    try {
      const res = await apiFetch<CleanlinessScore>('/api/v1/mail/cleanliness', {
        method: 'POST',
        body: JSON.stringify({ rawEmail: testRawEmail })
      });
      setCleanlinessResult(res);
    } catch (err: any) {
      setActionMessage(`${t('mailEvalFailed')}: ${err.message}`);
    } finally {
      setAnalyzing(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div 
      className="modal-overlay" 
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="mail-modal-title"
    >
      <div 
        className="modal-container glass-panel" 
        onClick={e => e.stopPropagation()}
        style={{
          width: '780px',
          maxWidth: '92vw',
          maxHeight: '85vh',
          overflowY: 'auto',
          borderRadius: '16px',
          padding: '24px'
        }}
      >
        {/* Header */}
        <div style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: '20px',
          paddingBottom: '16px',
          borderBottom: '1px solid var(--glass-border)'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div style={{
              width: '40px',
              height: '40px',
              borderRadius: '10px',
              background: 'rgba(234, 88, 12, 0.12)',
              color: 'var(--brand-primary)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              border: '1px solid rgba(234, 88, 12, 0.3)'
            }}>
              <Mail size={22} />
            </div>
            <div>
              <h2 id="mail-modal-title" style={{ margin: 0, fontSize: '1.2rem', fontWeight: 700, color: 'var(--text-main)' }}>
                {t('mailManagerTitle')}
              </h2>
              <p style={{ margin: '2px 0 0 0', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                {t('mailManagerSubtitle')}
              </p>
            </div>
          </div>

          <button
            onClick={onClose}
            className="btn-icon"
            style={{
              background: 'transparent',
              border: 'none',
              color: 'var(--text-muted)',
              cursor: 'pointer',
              padding: '6px',
              borderRadius: '8px'
            }}
            aria-label={t('close')}
          >
            <X size={20} />
          </button>
        </div>

        {/* Message Alert */}
        {actionMessage && (
          <div style={{
            padding: '12px 16px',
            borderRadius: '10px',
            marginBottom: '16px',
            fontSize: '0.82rem',
            background: isErrorMessage(actionMessage) ? 'rgba(239, 68, 68, 0.1)' : 'rgba(16, 185, 129, 0.1)',
            color: isErrorMessage(actionMessage) ? 'var(--brand-danger)' : '#10b981',
            border: `1px solid ${isErrorMessage(actionMessage) ? 'rgba(239, 68, 68, 0.2)' : 'rgba(16, 185, 129, 0.2)'}`,
            display: 'flex',
            alignItems: 'center',
            gap: '8px'
          }}>
            {isErrorMessage(actionMessage) ? <AlertTriangle size={16} /> : <CheckCircle size={16} />}
            <span>{actionMessage}</span>
          </div>
        )}

        {/* Authentication Summary: records generated by the mail module for the selected domain */}
        <div className="glass-card" style={{
          padding: '16px',
          borderRadius: '12px',
          marginBottom: '20px'
        }}>
          {mailDomains.length === 0 ? (
            <div style={{ fontSize: '0.82rem', color: 'var(--text-muted)' }}>
              {loading ? t('mailLoadingDomains') : t('mailNoDomains')}
            </div>
          ) : (
            <>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', marginBottom: '12px' }}>
                <label htmlFor="mail-domain-select" style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-muted)' }}>
                  {t('mailDomainLabel')}
                </label>
                <select
                  id="mail-domain-select"
                  value={selectedDomain}
                  onChange={(e) => setSelectedDomain(e.target.value)}
                  className="input-glass"
                  style={{ maxWidth: '280px', height: '32px', fontSize: '0.8rem', borderRadius: '8px', padding: '0 8px' }}
                >
                  {mailDomains.map(d => (
                    <option key={d.domain} value={d.domain}>{d.domain}</option>
                  ))}
                </select>
              </div>

              {activeDomain && (
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '12px' }}>
                  {[
                    { label: t('mailDkimStatus'), value: `selector ${activeDomain.dkimSelector}`, record: activeDomain.dkimRecord, color: '#10b981', bg: 'rgba(16, 185, 129, 0.1)', Icon: ShieldCheck },
                    { label: t('mailSpfStatus'), value: txtValue(activeDomain.spfRecord), record: activeDomain.spfRecord, color: '#3b82f6', bg: 'rgba(59, 130, 246, 0.1)', Icon: CheckCircle },
                    { label: t('mailDmarcStatus'), value: txtValue(activeDomain.dmarcRecord), record: activeDomain.dmarcRecord, color: '#a855f7', bg: 'rgba(168, 85, 247, 0.1)', Icon: ShieldCheck }
                  ].map(item => (
                    <div key={item.label} style={{ display: 'flex', alignItems: 'flex-start', gap: '10px', minWidth: 0 }}>
                      <div style={{ padding: '6px', borderRadius: '6px', background: item.bg, color: item.color, flexShrink: 0 }}>
                        <item.Icon size={16} />
                      </div>
                      <div style={{ minWidth: 0 }}>
                        <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>{item.label}</div>
                        <div
                          className="font-mono"
                          title={item.record}
                          style={{ fontSize: '0.76rem', fontWeight: 600, color: 'var(--text-main)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
                        >
                          {item.value}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </>
          )}
        </div>

        {/* Action Controls */}
        <div style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          gap: '12px',
          marginBottom: '20px'
        }}>
          <button
            onClick={() => setShowCreateBox(!showCreateBox)}
            className="btn-primary"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              padding: '8px 14px',
              borderRadius: '8px',
              fontSize: '0.82rem',
              fontWeight: 600,
              cursor: 'pointer'
            }}
          >
            <Plus size={16} />
            <span>{t('mailCreateBtn')}</span>
          </button>

          <button
            onClick={() => setShowTester(!showTester)}
            className="glass-card"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              padding: '8px 14px',
              borderRadius: '8px',
              fontSize: '0.82rem',
              fontWeight: 600,
              border: '1px solid var(--glass-border)',
              background: 'var(--glass-surface)',
              color: 'var(--text-main)',
              cursor: 'pointer'
            }}
          >
            <Sparkles size={16} style={{ color: 'var(--brand-primary)' }} />
            <span>{t('mailTestDeliverabilityBtn')}</span>
          </button>
        </div>

        {/* Create Mailbox Accordion Form */}
        {showCreateBox && (
          <form 
            onSubmit={handleCreateMailbox}
            className="glass-card" 
            style={{
              padding: '16px',
              borderRadius: '12px',
              marginBottom: '20px',
              display: 'flex',
              flexDirection: 'column',
              gap: '12px',
              background: 'var(--sidebar-bg)'
            }}
          >
            <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 1fr 0.8fr', gap: '12px' }}>
              <div>
                <label style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  {t('mailEmailLabel')}
                </label>
                <input 
                  type="email"
                  placeholder="user@testuser.digipanel.live"
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  className="input-glass"
                  style={{ width: '100%', height: '36px', fontSize: '0.82rem', borderRadius: '8px', padding: '0 10px' }}
                  required
                />
              </div>

              <div>
                <label style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  {t('mailPasswordLabel')}
                </label>
                <input 
                  type="password"
                  placeholder={t('mailPasswordPlaceholder')}
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  className="input-glass"
                  style={{ width: '100%', height: '36px', fontSize: '0.82rem', borderRadius: '8px', padding: '0 10px' }}
                  required
                />
              </div>

              <div>
                <label style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  {t('mailQuotaLabel')}
                </label>
                <input 
                  type="number"
                  value={newQuotaMb}
                  onChange={(e) => setNewQuotaMb(Number(e.target.value))}
                  className="input-glass"
                  style={{ width: '100%', height: '36px', fontSize: '0.82rem', borderRadius: '8px', padding: '0 10px' }}
                  min="100"
                  step="100"
                  required
                />
              </div>
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
              <button
                type="button"
                onClick={() => setShowCreateBox(false)}
                style={{ padding: '6px 12px', borderRadius: '6px', border: '1px solid var(--glass-border)', background: 'transparent', color: 'var(--text-muted)', fontSize: '0.78rem', cursor: 'pointer' }}
              >
                {t('cancel')}
              </button>
              <button
                type="submit"
                disabled={creating}
                className="btn-primary"
                style={{ padding: '6px 14px', borderRadius: '6px', fontSize: '0.78rem', fontWeight: 600, cursor: creating ? 'not-allowed' : 'pointer' }}
              >
                {creating ? t('mailSaving') : t('mailSaveBtn')}
              </button>
            </div>
          </form>
        )}

        {/* Deliverability Cleanliness Tester */}
        {showTester && (
          <div className="glass-card" style={{
            padding: '16px',
            borderRadius: '12px',
            marginBottom: '20px',
            background: 'var(--sidebar-bg)',
            display: 'flex',
            flexDirection: 'column',
            gap: '12px'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <h3 style={{ margin: 0, fontSize: '0.9rem', fontWeight: 600, color: 'var(--text-main)' }}>
                {t('mailCleanlinessScoreTitle')}
              </h3>
              <button
                onClick={handleAnalyzeCleanliness}
                disabled={analyzing || !testRawEmail.trim()}
                className="btn-primary"
                style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '6px 14px', borderRadius: '6px', fontSize: '0.76rem', fontWeight: 600, cursor: analyzing || !testRawEmail.trim() ? 'not-allowed' : 'pointer' }}
              >
                <Send size={13} />
                <span>{analyzing ? t('mailAnalyzing') : t('mailRunAnalysis')}</span>
              </button>
            </div>

            <textarea 
              value={testRawEmail}
              onChange={(e) => setTestRawEmail(e.target.value)}
              placeholder={t('mailTestPlaceholder')}
              aria-label={t('mailRawSourceAria')}
              className="input-glass"
              style={{
                width: '100%',
                height: '110px',
                fontSize: '0.75rem',
                fontFamily: 'monospace',
                borderRadius: '8px',
                padding: '10px',
                resize: 'vertical'
              }}
            />

            {cleanlinessResult && (
              <div style={{
                padding: '12px',
                borderRadius: '8px',
                background: 'var(--glass-surface)',
                border: '1px solid var(--glass-border)'
              }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px' }}>
                  <span style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-main)' }}>
                    {t('mailCleanlinessScoreLabel')}
                  </span>
                  <span style={{
                    fontSize: '1.2rem',
                    fontWeight: 700,
                    color: cleanlinessResult.totalScore >= 9.0 ? '#10b981' : cleanlinessResult.totalScore >= 6.0 ? '#f59e0b' : 'var(--brand-danger)'
                  }}>
                    {cleanlinessResult.totalScore} / 10.0
                  </span>
                </div>
                <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)', marginBottom: '8px' }}>
                  {cleanlinessResult.recommendation}
                </div>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginBottom: '8px' }}>
                  {[
                    { label: 'SPF', ok: cleanlinessResult.spfPassed },
                    { label: 'DKIM', ok: cleanlinessResult.dkimPassed },
                    { label: 'DMARC', ok: cleanlinessResult.dmarcPassed },
                    { label: 'PTR', ok: cleanlinessResult.ptrPassed }
                  ].map(flag => (
                    <span
                      key={flag.label}
                      style={{
                        display: 'inline-flex', alignItems: 'center', gap: '4px',
                        padding: '2px 8px', borderRadius: '6px', fontSize: '0.72rem', fontWeight: 600,
                        background: flag.ok ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)',
                        color: flag.ok ? '#10b981' : 'var(--brand-danger)'
                      }}
                    >
                      {flag.ok ? <Check size={12} /> : <X size={12} />}
                      {flag.label}
                    </span>
                  ))}
                </div>
                <ul style={{ margin: 0, paddingLeft: '20px', fontSize: '0.74rem', color: 'var(--text-main)' }}>
                  {cleanlinessResult.checks.map((chk, i) => (
                    <li key={i}>{chk}</li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}

        {/* Mailboxes Table */}
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '0.82rem' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--glass-border)', color: 'var(--text-muted)', fontSize: '0.74rem' }}>
                <th style={{ padding: '10px 12px' }}>{t('mailThEmail')}</th>
                <th style={{ padding: '10px 12px' }}>{t('mailThQuota')}</th>
                <th style={{ padding: '10px 12px' }}>{t('mailThRateLimit')}</th>
                <th style={{ padding: '10px 12px' }}>{t('mailThStatus')}</th>
              </tr>
            </thead>
            <tbody>
              {mailboxes.length === 0 && (
                <tr>
                  <td colSpan={4} style={{ padding: '20px 12px', textAlign: 'center', color: 'var(--text-muted)' }}>
                    {loading ? t('mailLoadingMailboxes') : t('mailNoMailboxes')}
                  </td>
                </tr>
              )}
              {mailboxes.map(mb => (
                <tr key={mb.email} style={{ borderBottom: '1px solid var(--glass-border)' }}>
                  <td style={{ padding: '12px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Inbox size={14} style={{ color: 'var(--brand-primary)' }} />
                      <span style={{ fontWeight: 600, color: 'var(--text-main)' }}>{mb.email}</span>
                    </div>
                  </td>
                  <td style={{ padding: '12px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '0.78rem', color: 'var(--text-main)', marginBottom: '3px' }}>
                      <HardDrive size={12} style={{ color: 'var(--text-dim)' }} />
                      {Math.round((mb.usedBytes / (1024 * 1024)) * 10) / 10} MB / {mb.quotaMb} MB
                    </div>
                    <div style={{ width: '120px', height: '4px', background: 'var(--sidebar-border)', borderRadius: '2px', overflow: 'hidden' }}>
                      <div style={{
                        width: `${Math.min(100, (mb.usedBytes / (mb.quotaMb * 1024 * 1024)) * 100)}%`,
                        height: '100%',
                        background: 'var(--brand-primary)'
                      }} />
                    </div>
                  </td>
                  <td style={{ padding: '12px', color: 'var(--text-muted)' }}>
                    {mb.hourlyLimit} {t('mailEmailsPerHour')}
                  </td>
                  <td style={{ padding: '12px' }}>
                    <span 
                      className={`badge-status ${mb.active ? 'online' : 'suspended'}`}
                      style={{ padding: '3px 8px', borderRadius: '6px', fontSize: '0.7rem', fontWeight: 600, textTransform: 'uppercase' }}
                    >
                      {mb.active ? t('active') : t('suspended')}
                    </span>
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

export default MailManagerModal;
