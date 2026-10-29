// DigiPanel Sign-In Gate
// The panel renders nothing privileged until the core issues a session token.
// Every failure message comes from the API so throttling and 2FA errors are
// reported exactly as the server decided them.

import React, { useState } from 'react';
import { ShieldCheck, LogIn, AlertCircle, Globe } from 'lucide-react';
import type { Language, ThemeMode } from '../types';
import { getTranslation } from '../i18n/translations';
import { login } from '../api/client';

interface LoginViewProps {
  language: Language;
  onLanguageChange: (language: Language) => void;
  theme: ThemeMode;
  onThemeToggle: () => void;
  onSignedIn: () => void;
}

export const LoginView: React.FC<LoginViewProps> = ({
  language,
  onLanguageChange,
  theme,
  onThemeToggle,
  onSignedIn
}) => {
  const t = (key: any) => getTranslation(language, key);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (submitting) return;
    setSubmitting(true);
    setError('');
    try {
      await login(username.trim(), password, totpCode.trim() || undefined);
      onSignedIn();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '24px',
        background: 'var(--page-bg, var(--sidebar-bg))'
      }}
    >
      <div style={{ width: '100%', maxWidth: '420px', display: 'flex', flexDirection: 'column', gap: '18px' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div
              style={{
                width: '38px',
                height: '38px',
                borderRadius: '11px',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                background: 'linear-gradient(135deg, rgba(234, 88, 12, 0.95), rgba(249, 115, 22, 0.75))',
                color: '#ffffff'
              }}
            >
              <ShieldCheck size={20} />
            </div>
            <div>
              <div style={{ fontSize: '1.05rem', fontWeight: 800, color: 'var(--text-main)' }}>{t('brandName')}</div>
              <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>{t('platformTagline')}</div>
            </div>
          </div>
          <button
            type="button"
            className="btn btn-secondary btn-sm"
            onClick={() => onLanguageChange(language === 'en' ? 'id' : 'en')}
            title={t('language')}
          >
            <Globe size={14} />
            {language.toUpperCase()}
          </button>
        </div>

        <form className="glass-panel" style={{ padding: '26px', display: 'flex', flexDirection: 'column', gap: '14px' }} onSubmit={submit}>
          <div>
            <h1 style={{ fontSize: '1.25rem', fontWeight: 800, color: 'var(--text-main)', marginBottom: '4px' }}>{t('loginTitle')}</h1>
            <p style={{ fontSize: '0.82rem', color: 'var(--text-muted)' }}>{t('loginSubtitle')}</p>
          </div>

          {error ? (
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
                padding: '9px 12px',
                borderRadius: '9px',
                background: 'rgba(239, 68, 68, 0.1)',
                border: '1px solid rgba(239, 68, 68, 0.3)',
                fontSize: '0.8rem',
                color: '#ef4444'
              }}
            >
              <AlertCircle size={15} />
              <span>{error}</span>
            </div>
          ) : null}

          <label style={{ display: 'flex', flexDirection: 'column', gap: '5px', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-muted)' }}>
            {t('loginUsername')}
            <input
              className="input-glass"
              style={{ height: '40px' }}
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              autoFocus
              required
            />
          </label>

          <label style={{ display: 'flex', flexDirection: 'column', gap: '5px', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-muted)' }}>
            {t('loginPassword')}
            <input
              className="input-glass"
              style={{ height: '40px' }}
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              required
            />
          </label>

          <label style={{ display: 'flex', flexDirection: 'column', gap: '5px', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-muted)' }}>
            {t('loginTotp')}
            <input
              className="input-glass"
              style={{ height: '40px' }}
              value={totpCode}
              onChange={(event) => setTotpCode(event.target.value)}
              inputMode="numeric"
              autoComplete="one-time-code"
              placeholder="000000"
            />
            <span style={{ fontSize: '0.7rem', fontWeight: 400 }}>{t('loginTotpHint')}</span>
          </label>

          <button type="submit" className="btn btn-primary" style={{ height: '42px', marginTop: '4px' }} disabled={submitting}>
            <LogIn size={16} />
            {submitting ? t('loginSigningIn') : t('loginSubmit')}
          </button>

          <div style={{ borderTop: '1px solid var(--glass-border)', paddingTop: '10px', display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <span style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>{t('loginSecureNote')}</span>
            <span style={{ fontSize: '0.72rem', color: 'var(--text-dim)' }}>{t('loginFootnote')}</span>
          </div>
        </form>

        <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
          <button type="button" className="btn btn-secondary btn-sm" onClick={onThemeToggle}>
            {theme === 'dark' ? 'Light' : 'Dark'}
          </button>
        </div>
      </div>
    </div>
  );
};
