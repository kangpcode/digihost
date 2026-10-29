// DigiPanel SSL/TLS ACME Certificate Manager Modal
// Live ACME certificate issuance, AutoSSL evaluation, and renewal
// Zero emojis, strictly clean Lucide SVG iconography, WCAG AA compliant.

import React, { useState, useEffect } from 'react';
import { 
  X, 
  ShieldCheck, 
  RefreshCw, 
  Plus, 
  CheckCircle, 
  AlertTriangle, 
  Globe, 
  Calendar, 
  Lock,
  Sparkles
} from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';

interface Certificate {
  id: string;
  domain: string;
  sans: string[];
  provider: string;
  challenge: string;
  status: string;
  issuer: string;
  daysRemaining: number;
  isWildcard: boolean;
  autoRenew: boolean;
  lastRenewedAt: string;
}

interface SslManagerModalProps {
  isOpen: boolean;
  onClose: () => void;
  language: Language;
}

export const SslManagerModal: React.FC<SslManagerModalProps> = ({
  isOpen,
  onClose,
  language
}) => {
  const [certificates, setCertificates] = useState<Certificate[]>([]);
  const [loading, setLoading] = useState(false);
  const [issuing, setIssuing] = useState(false);
  const [renewingDomain, setRenewingDomain] = useState<string | null>(null);
  const [runningAutoSSL, setRunningAutoSSL] = useState(false);
  const [showIssueForm, setShowIssueForm] = useState(false);
  const [actionMessage, setActionMessage] = useState<string | null>(null);

  // Issue Form State
  const [newDomain, setNewDomain] = useState('');
  const [newProvider, setNewProvider] = useState<'letsencrypt' | 'zerossl'>('letsencrypt');
  const [newChallenge, setNewChallenge] = useState<'http-01' | 'dns-01'>('http-01');
  const [newAutoRenew, setNewAutoRenew] = useState(true);

  const t = (key: any) => getTranslation(language, key);

  const fetchCertificates = async () => {
    setLoading(true);
    try {
      const data = await apiFetch<Certificate[]>('/api/v1/ssl/certificates');
      setCertificates(data || []);
    } catch (err: any) {
      setActionMessage(`${t('sslListFailed')}: ${err.message}`);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      setActionMessage(null);
      fetchCertificates();
    }
  }, [isOpen]);

  // Escape key listener for accessibility
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) onClose();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  const handleIssueCertificate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newDomain.trim()) return;

    setIssuing(true);
    setActionMessage(null);
    try {
      const cert = await apiFetch<Certificate>('/api/v1/ssl/issue', {
        method: 'POST',
        body: JSON.stringify({
          domain: newDomain.trim(),
          provider: newProvider,
          challenge: newDomain.startsWith('*.') ? 'dns-01' : newChallenge,
          autoRenew: newAutoRenew
        })
      });

      setCertificates([cert, ...certificates]);
      setNewDomain('');
      setShowIssueForm(false);
      setActionMessage(`${t('sslIssuedOk')}: ${cert.domain}`);
    } catch (err: any) {
      setActionMessage(`${t('actionFailed')}: ${err.message}`);
    } finally {
      setIssuing(false);
    }
  };

  const handleForceRenew = async (domain: string) => {
    setRenewingDomain(domain);
    setActionMessage(null);
    try {
      const renewed = await apiFetch<Certificate>('/api/v1/ssl/renew', {
        method: 'POST',
        body: JSON.stringify({ domain, force: true })
      });

      setCertificates(certificates.map(c => c.domain === domain ? renewed : c));
      setActionMessage(`${t('sslRenewedOk')}: ${domain}`);
    } catch (err: any) {
      setActionMessage(`${t('actionFailed')}: ${err.message}`);
    } finally {
      setRenewingDomain(null);
    }
  };

  const handleRunAutoSSL = async () => {
    setRunningAutoSSL(true);
    setActionMessage(null);
    try {
      const summary = await apiFetch<any>('/api/v1/ssl/autossl', {
        method: 'POST',
        body: JSON.stringify({})
      });

      await fetchCertificates();
      setActionMessage(`${t('sslAutoSslDone')}: ${summary.renewedCount} ${t('sslRenewedWord')}, ${summary.skippedCount} ${t('sslUpToDateWord')}.`);
    } catch (err: any) {
      setActionMessage(`${t('sslAutoSslFailed')}: ${err.message}`);
    } finally {
      setRunningAutoSSL(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div 
      className="modal-overlay" 
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="ssl-modal-title"
    >
      <div 
        className="modal-container glass-panel" 
        onClick={e => e.stopPropagation()}
        style={{
          width: '740px',
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
              background: 'rgba(16, 185, 129, 0.12)',
              color: '#10b981',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              border: '1px solid rgba(16, 185, 129, 0.3)'
            }}>
              <ShieldCheck size={22} />
            </div>
            <div>
              <h2 id="ssl-modal-title" style={{ margin: 0, fontSize: '1.2rem', fontWeight: 700, color: 'var(--text-main)' }}>
                {t('sslManagerTitle')}
              </h2>
              <p style={{ margin: '2px 0 0 0', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                Automated ACME TLS certificates with Let's Encrypt and AutoSSL
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

        {/* Action Message Alert */}
        {actionMessage && (
          <div style={{
            padding: '12px 16px',
            borderRadius: '10px',
            marginBottom: '16px',
            fontSize: '0.82rem',
            background: actionMessage.includes('failed') ? 'rgba(239, 68, 68, 0.1)' : 'rgba(16, 185, 129, 0.1)',
            color: actionMessage.includes('failed') ? 'var(--brand-danger)' : '#10b981',
            border: `1px solid ${actionMessage.includes('failed') ? 'rgba(239, 68, 68, 0.2)' : 'rgba(16, 185, 129, 0.2)'}`,
            display: 'flex',
            alignItems: 'center',
            gap: '8px'
          }}>
            {actionMessage.includes('failed') ? <AlertTriangle size={16} /> : <CheckCircle size={16} />}
            <span>{actionMessage}</span>
          </div>
        )}

        {/* Top Controls: Issue & AutoSSL Buttons */}
        <div style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          gap: '12px',
          marginBottom: '20px'
        }}>
          <button
            onClick={() => setShowIssueForm(!showIssueForm)}
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
            <span>{t('sslIssueBtn')}</span>
          </button>

          <button
            onClick={handleRunAutoSSL}
            disabled={runningAutoSSL}
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
              cursor: runningAutoSSL ? 'not-allowed' : 'pointer',
              opacity: runningAutoSSL ? 0.6 : 1
            }}
          >
            <Sparkles size={16} style={{ color: '#10b981' }} />
            <span>{runningAutoSSL ? t('sslRunningAutoSsl') : t('sslAutoSslBtn')}</span>
          </button>
        </div>

        {/* Issue Form Accordion */}
        {showIssueForm && (
          <form 
            onSubmit={handleIssueCertificate}
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
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
              <div>
                <label style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  Domain Name (or *.domain.com for wildcard)
                </label>
                <input 
                  type="text"
                  placeholder="e.g. secure.example.com"
                  value={newDomain}
                  onChange={(e) => setNewDomain(e.target.value)}
                  className="input-glass"
                  style={{ width: '100%', height: '36px', fontSize: '0.82rem', borderRadius: '8px', padding: '0 10px' }}
                  required
                />
              </div>

              <div>
                <label style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  Certificate Authority Provider
                </label>
                <select
                  value={newProvider}
                  onChange={(e: any) => setNewProvider(e.target.value)}
                  className="input-glass"
                  style={{ width: '100%', height: '36px', fontSize: '0.82rem', borderRadius: '8px', padding: '0 8px' }}
                >
                  <option value="letsencrypt">Let's Encrypt (Production)</option>
                  <option value="zerossl">ZeroSSL</option>
                </select>
              </div>
            </div>

            <div>
              <label style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '4px' }}>
                <Globe size={13} />
                <span>{t('sslChallengeType')}</span>
              </label>
              <select
                value={newDomain.startsWith('*.') ? 'dns-01' : newChallenge}
                onChange={(e) => setNewChallenge(e.target.value as 'http-01' | 'dns-01')}
                disabled={newDomain.startsWith('*.')}
                className="input-glass"
                style={{ width: '100%', height: '36px', fontSize: '0.82rem', borderRadius: '8px', padding: '0 8px' }}
              >
                <option value="http-01">{t('sslHttp01Option')}</option>
                <option value="dns-01">{t('sslDns01Option')}</option>
              </select>
            </div>

            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.8rem', color: 'var(--text-main)', cursor: 'pointer' }}>
                <input 
                  type="checkbox"
                  checked={newAutoRenew}
                  onChange={(e) => setNewAutoRenew(e.target.checked)}
                />
                <span>{t('sslAutoRenewCheckbox')}</span>
              </label>

              <div style={{ display: 'flex', gap: '8px' }}>
                <button
                  type="button"
                  onClick={() => setShowIssueForm(false)}
                  style={{ padding: '6px 12px', borderRadius: '6px', border: '1px solid var(--glass-border)', background: 'transparent', color: 'var(--text-muted)', fontSize: '0.78rem', cursor: 'pointer' }}
                >
                  {t('cancel')}
                </button>
                <button
                  type="submit"
                  disabled={issuing}
                  className="btn-primary"
                  style={{ padding: '6px 14px', borderRadius: '6px', fontSize: '0.78rem', fontWeight: 600, cursor: issuing ? 'not-allowed' : 'pointer' }}
                >
                  {issuing ? t('sslIssuing') : t('sslSubmitBtn')}
                </button>
              </div>
            </div>
          </form>
        )}

        {/* Certificates List Table */}
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '0.82rem' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--glass-border)', color: 'var(--text-muted)', fontSize: '0.74rem' }}>
                <th style={{ padding: '10px 12px' }}>{t('sslThDomain')}</th>
                <th style={{ padding: '10px 12px' }}>{t('sslThIssuer')}</th>
                <th style={{ padding: '10px 12px' }}>{t('sslThValidity')}</th>
                <th style={{ padding: '10px 12px' }}>{t('mailThStatus')}</th>
                <th style={{ padding: '10px 12px', textAlign: 'right' }}>{t('thActions')}</th>
              </tr>
            </thead>
            <tbody>
              {loading && certificates.length === 0 && (
                <tr>
                  <td colSpan={5} style={{ padding: '20px 12px', textAlign: 'center', color: 'var(--text-muted)' }}>
                    <RefreshCw size={14} className="spin" style={{ verticalAlign: 'middle', marginRight: '6px' }} />
                    {t('sslLoadingCertificates')}
                  </td>
                </tr>
              )}
              {!loading && certificates.length === 0 && (
                <tr>
                  <td colSpan={5} style={{ padding: '20px 12px', textAlign: 'center', color: 'var(--text-muted)' }}>
                    {t('sslNoCertificates')}
                  </td>
                </tr>
              )}
              {certificates.map(cert => (
                <tr key={cert.id} style={{ borderBottom: '1px solid var(--glass-border)' }}>
                  <td style={{ padding: '12px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Lock size={14} style={{ color: '#10b981' }} />
                      <span style={{ fontWeight: 600, color: 'var(--text-main)' }}>{cert.domain}</span>
                    </div>
                  </td>
                  <td style={{ padding: '12px', color: 'var(--text-muted)' }}>
                    {cert.issuer}
                  </td>
                  <td style={{ padding: '12px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <Calendar size={13} style={{ color: 'var(--text-dim)' }} />
                      <span style={{ color: cert.daysRemaining <= 30 ? 'var(--brand-danger)' : 'var(--text-main)' }}>
                        {cert.daysRemaining} {t('sslDaysRemaining')}
                      </span>
                    </div>
                  </td>
                  <td style={{ padding: '12px' }}>
                    <span 
                      className={`badge-status ${cert.status === 'valid' ? 'online' : 'suspended'}`}
                      style={{ padding: '3px 8px', borderRadius: '6px', fontSize: '0.7rem', fontWeight: 600, textTransform: 'uppercase' }}
                    >
                      {cert.status}
                    </span>
                  </td>
                  <td style={{ padding: '12px', textAlign: 'right' }}>
                    <button
                      onClick={() => handleForceRenew(cert.domain)}
                      disabled={renewingDomain === cert.domain}
                      style={{
                        display: 'inline-flex',
                        alignItems: 'center',
                        gap: '4px',
                        padding: '5px 10px',
                        borderRadius: '6px',
                        border: '1px solid var(--glass-border)',
                        background: 'transparent',
                        color: 'var(--brand-primary)',
                        fontSize: '0.74rem',
                        fontWeight: 600,
                        cursor: renewingDomain === cert.domain ? 'not-allowed' : 'pointer'
                      }}
                    >
                      <RefreshCw size={12} className={renewingDomain === cert.domain ? 'spin' : ''} />
                      <span>{t('sslRenewBtn')}</span>
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

export default SslManagerModal;
