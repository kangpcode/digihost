// DigiPanel Sign-In Gate - Liquid Glass & Claymorphism Edition
// Zero emojis, real API authentication, progressive 2FA disclosure.

import React, { useState } from 'react';
import { ShieldCheck, LogIn, AlertCircle, Globe, Eye, EyeOff, Sun, Moon, KeyRound } from 'lucide-react';
import type { Language, ThemeMode } from '../types';
import { getTranslation } from '../i18n/translations';
import { login } from '../api/client';

interface LoginViewProps {
  language: Language;
  onLanguageChange: (language: Language) => void;
  theme: ThemeMode;
  onThemeToggle: () => void;
  onSignedIn: () => void;
  brandName?: string;
}

export const LoginView: React.FC<LoginViewProps> = ({
  language,
  onLanguageChange,
  theme,
  onThemeToggle,
  onSignedIn,
  brandName
}) => {
  const t = (key: any) => getTranslation(language, key);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [showTotp, setShowTotp] = useState(false);
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
      const msg = cause instanceof Error ? cause.message : String(cause);
      if (msg.toLowerCase().includes('totp') || msg.toLowerCase().includes('2fa')) {
        setShowTotp(true);
      }
      setError(msg);
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
        position: 'relative',
        zIndex: 1
      }}
    >
      <div style={{ width: '100%', maxWidth: '440px', display: 'flex', flexDirection: 'column', gap: '18px' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div
              className="clay-icon"
              style={{
                width: '44px',
                height: '44px',
                borderRadius: '15px',
                background: 'linear-gradient(135deg, var(--brand-primary) 0%, var(--brand-accent) 100%)',
                color: '#ffffff',
                boxShadow: 'var(--clay-btn-shadow)'
              }}
            >
              <ShieldCheck size={22} />
            </div>
            <div>
              <div style={{ fontSize: '1.15rem', fontWeight: 800, color: 'var(--text-main)', letterSpacing: '-0.02em' }}>
                {brandName || t('brandName')}
              </div>
              <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>{t('platformTagline')}</div>
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <button
              type="button"
              className="btn btn-secondary btn-sm"
              onClick={() => onLanguageChange(language === 'en' ? 'id' : 'en')}
              title={t('language')}
            >
              <Globe size={14} />
              {language.toUpperCase()}
            </button>
            <button
              type="button"
              className="btn-icon"
              onClick={onThemeToggle}
              title={theme === 'dark' ? t('lightMode') : t('darkMode')}
            >
              {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
            </button>
          </div>
        </div>

        <form className="glass-panel" style={{ padding: '28px', display: 'flex', flexDirection: 'column', gap: '16px' }} onSubmit={submit}>
          <div>
            <h1 style={{ fontSize: '1.35rem', fontWeight: 800, color: 'var(--text-main)', marginBottom: '4px', letterSpacing: '-0.02em' }}>
              {t('loginTitle')}
            </h1>
            <p style={{ fontSize: '0.84rem', color: 'var(--text-muted)' }}>{t('loginSubtitle')}</p>
          </div>

          {error ? (
            <div
              className="glass-card"
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '10px',
                padding: '11px 14px',
                borderRadius: '14px',
                background: 'var(--status-danger-bg)',
                border: '1px solid rgba(239, 68, 68, 0.35)',
                fontSize: '0.82rem',
                color: 'var(--status-danger)'
              }}
            >
              <AlertCircle size={16} style={{ flexShrink: 0 }} />
              <span>{error}</span>
            </div>
          ) : null}

          <label style={{ display: 'flex', flexDirection: 'column', gap: '6px', fontSize: '0.8rem', fontWeight: 700, color: 'var(--text-muted)' }}>
            {t('loginUsername')}
            <input
              className="input-glass"
              style={{ height: '42px' }}
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              autoFocus
              required
            />
          </label>

          <label style={{ display: 'flex', flexDirection: 'column', gap: '6px', fontSize: '0.8rem', fontWeight: 700, color: 'var(--text-muted)' }}>
            {t('loginPassword')}
            <div style={{ position: 'relative' }}>
              <input
                className="input-glass"
                style={{ height: '42px', paddingRight: '42px' }}
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                autoComplete="current-password"
                required
              />
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                aria-label={showPassword ? t('loginHidePassword') : t('loginShowPassword')}
                style={{
                  position: 'absolute',
                  right: '10px',
                  top: '50%',
                  transform: 'translateY(-50%)',
                  background: 'transparent',
                  border: 'none',
                  color: 'var(--text-muted)',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center'
                }}
              >
                {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
          </label>

          {!showTotp ? (
            <button
              type="button"
              onClick={() => setShowTotp(true)}
              style={{
                alignSelf: 'flex-start',
                display: 'inline-flex',
                alignItems: 'center',
                gap: '6px',
                background: 'transparent',
                border: 'none',
                color: 'var(--brand-primary)',
                fontSize: '0.78rem',
                fontWeight: 700,
                cursor: 'pointer',
                padding: '2px 0'
              }}
            >
              <KeyRound size={13} />
              <span>{t('loginHave2fa')}</span>
            </button>
          ) : (
            <label style={{ display: 'flex', flexDirection: 'column', gap: '6px', fontSize: '0.8rem', fontWeight: 700, color: 'var(--text-muted)' }}>
              {t('loginTotp')}
              <input
                className="input-glass font-mono"
                style={{ height: '42px', letterSpacing: '0.18em' }}
                value={totpCode}
                onChange={(event) => setTotpCode(event.target.value)}
                inputMode="numeric"
                autoComplete="one-time-code"
                placeholder="000000"
              />
              <span style={{ fontSize: '0.72rem', fontWeight: 500, color: 'var(--text-dim)' }}>{t('loginTotpHint')}</span>
            </label>
          )}

          <button type="submit" className="btn btn-primary" style={{ height: '44px', marginTop: '4px', fontSize: '0.92rem' }} disabled={submitting}>
            <LogIn size={17} />
            {submitting ? t('loginSigningIn') : t('loginSubmit')}
          </button>

          <div style={{ borderTop: '1px solid var(--glass-border)', paddingTop: '12px', display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <span style={{ fontSize: '0.74rem', color: 'var(--text-muted)', fontWeight: 600 }}>{t('loginSecureNote')}</span>
            <span style={{ fontSize: '0.72rem', color: 'var(--text-dim)' }}>{t('loginFootnote')}</span>
          </div>
        </form>
      </div>
    </div>
  );
};
