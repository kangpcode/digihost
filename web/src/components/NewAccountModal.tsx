// DigiPanel New Hosting Account modal.
// Submits to the core hosting API. Plan choices are the limit definitions the
// provisioning engine actually enforces, and the result reflects the server
// response instead of a simulated progress sequence.

import React, { useEffect, useState } from 'react';
import { UserPlus, X, AlertCircle } from 'lucide-react';
import type { ApiAccount, Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';

interface PackageOption {
  id: string;
  name: string;
  diskLimitBytes: number;
  defaultPhpVersion: string;
}

interface NewAccountModalProps {
  isOpen: boolean;
  onClose: () => void;
  onAccountCreated: (account?: ApiAccount) => void;
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
  const [plan, setPlan] = useState('');
  const [phpVersion, setPhpVersion] = useState('');
  const [packages, setPackages] = useState<PackageOption[]>([]);
  const [panelPassword, setPanelPassword] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [created, setCreated] = useState<{ account: ApiAccount; panelUsername: string; initialPassword: string } | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    setError('');
    setCreated(null);
    apiFetch<PackageOption[]>('/api/v1/hosting/packages')
      .then((data) => {
        const list = Array.isArray(data) ? data : [];
        setPackages(list);
        if (list.length > 0) {
          setPlan((current) => current || list[0].name);
          setPhpVersion((current) => current || list[0].defaultPhpVersion);
        }
      })
      .catch((cause) => setError(cause instanceof Error ? cause.message : String(cause)));
  }, [isOpen]);

  if (!isOpen) return null;

  const handleDomainChange = (value: string) => {
    setDomain(value);
    const clean = value.split('.')[0].replace(/[^a-z0-9]/gi, '').toLowerCase().slice(0, 12);
    if (!username) setUsername(clean);
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!domain || !username || !plan) return;

    setIsSubmitting(true);
    setError('');
    try {
      const result = await apiFetch<{ account: ApiAccount; panelUsername: string; panelLoginReady: boolean; initialPassword: string }>(
        '/api/v1/hosting/accounts',
        {
          method: 'POST',
          body: JSON.stringify({
            tenantId: 'tenant-default',
            username,
            domain,
            package: plan,
            phpVersion: phpVersion || undefined,
            panelPassword: panelPassword || undefined
          })
        }
      );
      setCreated({ account: result.account, panelUsername: result.panelUsername, initialPassword: result.initialPassword });
      onAccountCreated(result.account);
      setDomain('');
      setUsername('');
      setPanelPassword('');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        className="modal-content"
        onClick={(event) => event.stopPropagation()}
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
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>{t('newAccSubtitle')}</div>
            </div>
          </div>
          <button type="button" onClick={onClose} className="btn-icon">
            <X size={20} />
          </button>
        </div>

        {created ? (
          <div className="glass-card" style={{ padding: '16px', display: 'flex', flexDirection: 'column', gap: '10px', borderColor: 'rgba(16, 185, 129, 0.35)' }}>
            <div style={{ fontSize: '0.88rem', fontWeight: 700 }}>{t('newAccSuccess')}</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
              {created.account.username} &middot; {created.account.primaryDomain} &middot; {created.account.packageId}
            </div>
            <div className="font-mono" style={{ fontSize: '0.74rem', color: 'var(--text-dim)' }}>{created.account.documentRoot}</div>
            <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
              {t('newAccPanelLogin')}: <strong>{created.panelUsername}</strong>
            </div>
            {created.initialPassword ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                <span style={{ fontSize: '0.76rem', color: 'var(--text-muted)' }}>{t('newAccInitialPassword')}</span>
                <code className="font-mono" style={{ fontSize: '0.82rem', padding: '8px 10px', borderRadius: '8px', background: 'rgba(148, 163, 184, 0.14)' }}>
                  {created.initialPassword}
                </code>
                <span style={{ fontSize: '0.72rem', color: 'var(--text-dim)' }}>{t('newAccInitialPasswordHint')}</span>
              </div>
            ) : null}
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
              <button type="button" className="btn btn-secondary btn-sm" onClick={onClose}>{t('close')}</button>
            </div>
          </div>
        ) : (
          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            {error ? (
              <div className="glass-card" style={{ padding: '10px 14px', display: 'flex', gap: '8px', alignItems: 'center', fontSize: '0.82rem', borderColor: 'rgba(239, 68, 68, 0.35)' }}>
                <AlertCircle size={15} style={{ color: '#ef4444', flexShrink: 0 }} />
                <span>{error}</span>
              </div>
            ) : null}

            <label style={{ display: 'flex', flexDirection: 'column', gap: '5px', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
              {t('newAccDomainLabel')}
              <input
                className="input-glass"
                required
                value={domain}
                onChange={(event) => handleDomainChange(event.target.value)}
                placeholder={t('newAccDomainPh')}
              />
            </label>

            <label style={{ display: 'flex', flexDirection: 'column', gap: '5px', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
              {t('newAccUserLabel')}
              <input
                className="input-glass"
                required
                value={username}
                onChange={(event) => setUsername(event.target.value.toLowerCase())}
                placeholder="clientname"
              />
              <span style={{ fontSize: '0.7rem', color: 'var(--text-dim)' }}>
                3-32 karakter, huruf kecil, angka, atau garis bawah
              </span>
            </label>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
              <label style={{ display: 'flex', flexDirection: 'column', gap: '5px', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                {t('thPlan')}
                <select className="input-glass" value={plan} onChange={(event) => setPlan(event.target.value)} required>
                  {packages.length === 0 ? <option value="">{t('plansEmpty')}</option> : null}
                  {packages.map((option) => (
                    <option key={option.id} value={option.name}>{option.name}</option>
                  ))}
                </select>
              </label>

              <label style={{ display: 'flex', flexDirection: 'column', gap: '5px', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                {t('newAccPhpLabel')}
                <input
                  className="input-glass"
                  value={phpVersion}
                  onChange={(event) => setPhpVersion(event.target.value)}
                  placeholder="8.3"
                />
              </label>
            </div>

            <label style={{ display: 'flex', flexDirection: 'column', gap: '5px', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
              {t('newAccClientPassword')}
              <input
                className="input-glass"
                type="password"
                value={panelPassword}
                onChange={(event) => setPanelPassword(event.target.value)}
                autoComplete="new-password"
                minLength={10}
                placeholder={t('newAccClientPasswordPh')}
              />
            </label>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px', marginTop: '6px' }}>
              <button type="button" className="btn btn-secondary btn-sm" onClick={onClose} disabled={isSubmitting}>
                {t('cancel')}
              </button>
              <button type="submit" className="btn btn-primary btn-sm" disabled={isSubmitting || packages.length === 0}>
                <UserPlus size={14} />
                {isSubmitting ? t('newAccProvisioning') : t('create')}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
};
