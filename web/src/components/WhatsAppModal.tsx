// DigiPanel WhatsApp Integration Modal (Matching Image 1)
// Zero emojis, strictly Lucide SVG iconography, modern glass styling

import React, { useState } from 'react';
import { MessageSquare, X, Check, Smartphone } from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';

interface WhatsAppModalProps {
  isOpen: boolean;
  onClose: () => void;
  language: Language;
}

export const WhatsAppModal: React.FC<WhatsAppModalProps> = ({
  isOpen,
  onClose,
  language
}) => {
  const t = (key: any) => getTranslation(language, key);

  const [phoneNumber, setPhoneNumber] = useState('+6281234567890');
  const [notifyOnDown, setNotifyOnDown] = useState(true);
  const [notifyOnSsl, setNotifyOnSsl] = useState(true);
  const [notifyOnSecurity, setNotifyOnSecurity] = useState(true);
  const [isConnected, setIsConnected] = useState(false);

  if (!isOpen) return null;

  const handleConnect = (e: React.FormEvent) => {
    e.preventDefault();
    setIsConnected(true);
    setTimeout(() => {
      onClose();
    }, 1200);
  };

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div 
        className="modal-content" 
        onClick={(e) => e.stopPropagation()} 
        style={{ maxWidth: '540px', padding: '28px' }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div style={{
              width: '38px',
              height: '38px',
              borderRadius: '10px',
              background: 'rgba(16, 185, 129, 0.15)',
              color: 'var(--status-online)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center'
            }}>
              <MessageSquare size={20} />
            </div>
            <div>
              <h2 style={{ fontSize: '1.25rem', fontWeight: 700, color: 'var(--text-main)' }}>
                {t('modalWhatsAppTitle')}
              </h2>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                {t('waSubtitle')}
              </div>
            </div>
          </div>
          <button onClick={onClose} className="btn-icon">
            <X size={20} />
          </button>
        </div>

        {isConnected ? (
          <div style={{ padding: '30px 20px', textAlign: 'center' }}>
            <div style={{
              width: '48px',
              height: '48px',
              borderRadius: '50%',
              background: 'rgba(16, 185, 129, 0.15)',
              color: 'var(--status-online)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              margin: '0 auto 16px auto'
            }}>
              <Check size={24} />
            </div>
            <h3 style={{ fontSize: '1.1rem', fontWeight: 700, color: 'var(--text-main)', marginBottom: '6px' }}>
              {t('waLinked')}
            </h3>
            <p style={{ fontSize: '0.84rem', color: 'var(--text-muted)' }}>
              {t('waTestDispatched')}: {phoneNumber}.
            </p>
          </div>
        ) : (
          <form onSubmit={handleConnect} style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div>
              <label style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                {t('waNumberLabel')}
              </label>
              <div style={{ position: 'relative' }}>
                <Smartphone size={16} style={{ position: 'absolute', left: '12px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
                <input 
                  type="text"
                  value={phoneNumber}
                  onChange={(e) => setPhoneNumber(e.target.value)}
                  className="input-glass font-mono"
                  style={{ paddingLeft: '38px', height: '40px' }}
                  required
                />
              </div>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', marginTop: '6px' }}>
              <span style={{ fontSize: '0.82rem', fontWeight: 700, color: 'var(--text-main)' }}>
                {t('waSubscriptions')}
              </span>
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.84rem', cursor: 'pointer' }}>
                <input 
                  type="checkbox" 
                  checked={notifyOnDown} 
                  onChange={(e) => setNotifyOnDown(e.target.checked)} 
                />
                <span>{t('waNotifyDown')}</span>
              </label>
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.84rem', cursor: 'pointer' }}>
                <input 
                  type="checkbox" 
                  checked={notifyOnSsl} 
                  onChange={(e) => setNotifyOnSsl(e.target.checked)} 
                />
                <span>{t('waNotifySsl')}</span>
              </label>
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.84rem', cursor: 'pointer' }}>
                <input 
                  type="checkbox" 
                  checked={notifyOnSecurity} 
                  onChange={(e) => setNotifyOnSecurity(e.target.checked)} 
                />
                <span>{t('waNotifySecurity')}</span>
              </label>
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '14px' }}>
              <button type="button" onClick={onClose} className="btn btn-secondary">
                {t('cancel')}
              </button>
              <button type="submit" className="btn btn-primary">
                <Check size={16} />
                <span>{t('waPairBtn')}</span>
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
};
