// DigiPanel New Hosting Account Creation Modal
// Zero emojis, strictly Lucide SVG iconography, modern glass styling

import React, { useState } from 'react';
import { UserPlus, X, Globe, Lock, Check } from 'lucide-react';
import type { HostingAccount, Language } from '../types';
import { getTranslation } from '../i18n/translations';

interface NewAccountModalProps {
  isOpen: boolean;
  onClose: () => void;
  onAccountCreated: (account: HostingAccount) => void;
  language: Language;
}

export const NewAccountModal: React.FC<NewAccountModalProps> = ({
  isOpen,
  onClose,
  onAccountCreated,
  language
}) => {
  const t = (key: any) => getTranslation(language, key);

  const [domain, setDomain] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [plan, setPlan] = useState('Standard Cloud');
  const [phpVersion, setPhpVersion] = useState('8.3');
  const [diskLimitMb] = useState(10240);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [stepStatus, setStepStatus] = useState<string[]>([]);

  if (!isOpen) return null;

  const handleDomainChange = (val: string) => {
    setDomain(val);
    const cleanUser = val.split('.')[0].replace(/[^a-z0-9]/gi, '').toLowerCase().slice(0, 8);
    if (!username || username === '') {
      setUsername(cleanUser || 'client');
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!domain || !username || !password) return;

    setIsSubmitting(true);
    setStepStatus(['Allocating POSIX user & home directory...']);

    setTimeout(() => {
      setStepStatus(prev => [...prev, 'Configuring Nginx vhost & PHP-FPM 8.3 pool...']);
    }, 400);

    setTimeout(() => {
      setStepStatus(prev => [...prev, 'Initializing MariaDB database & user grant...']);
    }, 800);

    setTimeout(() => {
      setStepStatus(prev => [...prev, 'Issuing Let\'s Encrypt SSL certificate via ACME...']);
    }, 1200);

    setTimeout(() => {
      const newAcc: HostingAccount = {
        id: `acc-${Date.now()}`,
        username,
        domain,
        plan,
        diskUsageMb: 12,
        diskLimitMb,
        bandwidthUsageMb: 0,
        bandwidthLimitMb: 51200,
        databasesCount: 1,
        databasesLimit: 5,
        emailCount: 1,
        emailLimit: 10,
        inodesCount: 420,
        inodesLimit: 50000,
        status: 'active',
        phpVersion,
        ipAddress: '103.189.201.42',
        createdAt: new Date().toISOString().split('T')[0],
        sslActive: true
      };

      onAccountCreated(newAcc);
      setIsSubmitting(false);
      onClose();
    }, 1600);
  };

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div 
        className="modal-content" 
        onClick={(e) => e.stopPropagation()} 
        style={{ maxWidth: '640px', padding: '28px' }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div style={{
              width: '38px',
              height: '38px',
              borderRadius: '10px',
              background: 'rgba(234, 88, 12, 0.15)',
              color: 'var(--brand-primary)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center'
            }}>
              <UserPlus size={20} />
            </div>
            <div>
              <h2 style={{ fontSize: '1.25rem', fontWeight: 700, color: 'var(--text-main)' }}>
                {t('modalNewAccountTitle')}
              </h2>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                Atomic account creation with automatic rollback and cgroups v2 quotas
              </div>
            </div>
          </div>
          <button onClick={onClose} className="btn-icon">
            <X size={20} />
          </button>
        </div>

        {isSubmitting ? (
          <div style={{ padding: '30px 20px', textAlign: 'center' }}>
            <div style={{
              width: '48px',
              height: '48px',
              borderRadius: '50%',
              border: '3px solid rgba(234, 88, 12, 0.2)',
              borderTopColor: 'var(--brand-primary)',
              animation: 'spin 0.8s linear infinite',
              margin: '0 auto 20px auto'
            }} />
            <h3 style={{ fontSize: '1.1rem', fontWeight: 700, marginBottom: '12px' }}>
              Provisioning Account...
            </h3>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxWidth: '420px', margin: '0 auto', textAlign: 'left', fontSize: '0.82rem' }}>
              {stepStatus.map((step, idx) => (
                <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--status-online)' }}>
                  <Check size={14} />
                  <span>{step}</span>
                </div>
              ))}
            </div>
          </div>
        ) : (
          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div>
              <label style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                Primary Domain Name
              </label>
              <div style={{ position: 'relative' }}>
                <Globe size={16} style={{ position: 'absolute', left: '12px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
                <input 
                  type="text"
                  placeholder="example.com"
                  value={domain}
                  onChange={(e) => handleDomainChange(e.target.value)}
                  className="input-glass font-mono"
                  style={{ paddingLeft: '38px', height: '40px' }}
                  required
                />
              </div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '14px' }}>
              <div>
                <label style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                  Linux System Username
                </label>
                <input 
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="input-glass font-mono"
                  style={{ height: '40px' }}
                  required
                />
              </div>

              <div>
                <label style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                  Account Password
                </label>
                <div style={{ position: 'relative' }}>
                  <Lock size={15} style={{ position: 'absolute', left: '12px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
                  <input 
                    type="password"
                    placeholder="Argon2id secure password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="input-glass"
                    style={{ paddingLeft: '36px', height: '40px' }}
                    required
                  />
                </div>
              </div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '14px' }}>
              <div>
                <label style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                  Package Plan
                </label>
                <select 
                  value={plan} 
                  onChange={(e) => setPlan(e.target.value)}
                  className="input-glass"
                  style={{ height: '40px' }}
                >
                  <option value="Starter Cloud">Starter Cloud (5 GB Disk)</option>
                  <option value="Standard Cloud">Standard Cloud (10 GB Disk)</option>
                  <option value="Pro Developer">Pro Developer (25 GB Disk)</option>
                  <option value="Enterprise Fleet">Enterprise Fleet (100 GB Disk)</option>
                </select>
              </div>

              <div>
                <label style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                  PHP Version
                </label>
                <select 
                  value={phpVersion} 
                  onChange={(e) => setPhpVersion(e.target.value)}
                  className="input-glass font-mono"
                  style={{ height: '40px' }}
                >
                  <option value="8.3">PHP 8.3 FPM (Recommended)</option>
                  <option value="8.2">PHP 8.2 FPM</option>
                  <option value="8.1">PHP 8.1 FPM</option>
                </select>
              </div>
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '12px' }}>
              <button type="button" onClick={onClose} className="btn btn-secondary">
                {t('cancel')}
              </button>
              <button type="submit" className="btn btn-primary">
                <Check size={16} />
                <span>{t('create')}</span>
              </button>
            </div>
          </form>
        )}
      </div>
      <style>{`
        @keyframes spin {
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
};
