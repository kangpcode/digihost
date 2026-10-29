// DigiPanel Top Navigation Bar
// Zero emojis, SVG iconography, modern glass styling
// Clean full-width header with zero overlapping elements

import React, { useState } from 'react';
import { 
  Zap, 
  Search, 
  Bell, 
  Sun, 
  Moon, 
  Globe, 
  Server, 
  ShieldCheck, 
  ChevronDown, 
  Check,
  Activity,
  Network,
  Cpu,
  FileText,
  LogOut
} from 'lucide-react';
import type { UserRole, Language, ThemeMode } from '../types';
import { getTranslation } from '../i18n/translations';

export interface HeaderNotification {
  title: string;
  detail: string;
}

interface HeaderProps {
  currentRole: UserRole;
  onSignOut: () => void;
  activeView: string;
  onViewChange: (view: string) => void;
  language: Language;
  onLanguageChange: (lang: Language) => void;
  theme: ThemeMode;
  onThemeToggle: () => void;
  searchQuery: string;
  onSearchChange: (q: string) => void;
  onOpenTerminal: () => void;
  displayName?: string;
  notifications?: HeaderNotification[];
}

export const Header: React.FC<HeaderProps> = ({
  currentRole,
  onSignOut,
  activeView,
  onViewChange,
  language,
  onLanguageChange,
  theme,
  onThemeToggle,
  searchQuery,
  onSearchChange,
  onOpenTerminal,
  displayName,
  notifications = []
}) => {
  const [showRoleMenu, setShowRoleMenu] = useState(false);
  const [showLangMenu, setShowLangMenu] = useState(false);
  const [showNotifications, setShowNotifications] = useState(false);

  const t = (key: any) => getTranslation(language, key);

  const roleLabels: Record<UserRole, string> = {
    admin: t('adminPanel'),
    reseller: t('resellerPanel'),
    user: t('clientPanel')
  };

  return (
    <header style={{
      height: '68px',
      width: '100%',
      position: 'sticky',
      top: 0,
      zIndex: 100,
      background: 'var(--topbar-bg)',
      backdropFilter: 'blur(20px) saturate(180%)',
      WebkitBackdropFilter: 'blur(20px) saturate(180%)',
      borderBottom: '1px solid var(--glass-border)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '0 24px',
      boxSizing: 'border-box',
      transition: 'background 0.2s ease, border-color 0.2s ease'
    }}>
      {/* Left: Brand Identity & Hostname & Quick Navigation Tabs */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexShrink: 0 }}>
        <div 
          onClick={() => {
            if (currentRole === 'admin') onViewChange('dashboard');
            else onViewChange('home');
          }}
          style={{ 
            display: 'flex', 
            alignItems: 'center', 
            gap: '10px', 
            cursor: 'pointer',
            userSelect: 'none'
          }}
        >
          <div style={{
            width: '36px',
            height: '36px',
            borderRadius: '10px',
            background: 'linear-gradient(135deg, #ea580c 0%, #f59e0b 100%)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#ffffff',
            boxShadow: '0 2px 10px rgba(234, 88, 12, 0.3)'
          }}>
            <Zap size={20} strokeWidth={2.5} />
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span style={{ 
              fontSize: '1.25rem', 
              fontWeight: 800, 
              letterSpacing: '-0.02em', 
              color: 'var(--text-main)' 
            }}>
              {t('brandName')}
            </span>
            <span style={{
              fontSize: '0.68rem',
              fontWeight: 700,
              textTransform: 'uppercase',
              letterSpacing: '0.05em',
              background: currentRole === 'admin' ? 'rgba(234, 88, 12, 0.15)' : 'rgba(2, 132, 199, 0.15)',
              color: currentRole === 'admin' ? 'var(--brand-primary)' : 'var(--status-info)',
              padding: '2px 7px',
              borderRadius: '6px',
              border: `1px solid ${currentRole === 'admin' ? 'var(--brand-primary-border)' : 'rgba(2, 132, 199, 0.3)'}`
            }}>
              {roleLabels[currentRole]}
            </span>
          </div>
        </div>

        {/* Hostname Chip (matching AdminBolt / cPanel header) */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
          background: 'rgba(148, 163, 184, 0.08)',
          border: '1px solid var(--glass-border)',
          borderRadius: '20px',
          padding: '4px 12px',
          fontSize: '0.78rem'
        }}>
          <Server size={13} style={{ color: 'var(--text-muted)' }} />
          <span style={{ color: 'var(--text-muted)' }}>{t('hostnameLabel')}:</span>
          <span className="font-mono" style={{ fontWeight: 600, color: 'var(--text-main)' }}>
            panel.digihost.id
          </span>
        </div>

        {/* Quick View Navigation Tabs (Matching Image) */}
        {currentRole === 'admin' && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginLeft: '6px' }}>
            <button
              onClick={() => onViewChange('dashboard')}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                padding: '6px 14px',
                borderRadius: '8px',
                fontSize: '0.8rem',
                fontWeight: 700,
                cursor: 'pointer',
                border: activeView === 'dashboard' ? '1px solid rgba(234, 88, 12, 0.4)' : '1px solid var(--glass-border)',
                background: activeView === 'dashboard' ? 'linear-gradient(135deg, #ea580c 0%, #f59e0b 100%)' : 'rgba(148, 163, 184, 0.08)',
                color: activeView === 'dashboard' ? '#ffffff' : 'var(--text-main)',
                boxShadow: activeView === 'dashboard' ? '0 2px 8px rgba(234, 88, 12, 0.25)' : 'none',
                transition: 'all 0.15s ease'
              }}
            >
              <Activity size={14} />
              <span>{t('navDashboard')}</span>
            </button>

            <button
              onClick={() => onViewChange('cluster')}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                padding: '6px 14px',
                borderRadius: '8px',
                fontSize: '0.8rem',
                fontWeight: 700,
                cursor: 'pointer',
                border: activeView === 'cluster' ? '1px solid rgba(234, 88, 12, 0.4)' : '1px solid var(--glass-border)',
                background: activeView === 'cluster' ? 'linear-gradient(135deg, #ea580c 0%, #f59e0b 100%)' : 'rgba(148, 163, 184, 0.08)',
                color: activeView === 'cluster' ? '#ffffff' : 'var(--text-main)',
                boxShadow: activeView === 'cluster' ? '0 2px 8px rgba(234, 88, 12, 0.25)' : 'none',
                transition: 'all 0.15s ease'
              }}
            >
              <Network size={14} />
              <span>{t('navCluster')}</span>
            </button>

            <button
              onClick={() => onViewChange('runtimes')}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                padding: '6px 14px',
                borderRadius: '8px',
                fontSize: '0.8rem',
                fontWeight: 700,
                cursor: 'pointer',
                border: activeView === 'runtimes' ? '1px solid rgba(234, 88, 12, 0.4)' : '1px solid var(--glass-border)',
                background: activeView === 'runtimes' ? 'linear-gradient(135deg, #ea580c 0%, #f59e0b 100%)' : 'rgba(148, 163, 184, 0.08)',
                color: activeView === 'runtimes' ? '#ffffff' : 'var(--text-main)',
                boxShadow: activeView === 'runtimes' ? '0 2px 8px rgba(234, 88, 12, 0.25)' : 'none',
                transition: 'all 0.15s ease'
              }}
            >
              <Cpu size={14} />
              <span>{t('navSoftware')}</span>
            </button>

            <button
              onClick={() => onViewChange('billing')}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                padding: '6px 14px',
                borderRadius: '8px',
                fontSize: '0.8rem',
                fontWeight: 700,
                cursor: 'pointer',
                border: activeView === 'billing' ? '1px solid rgba(234, 88, 12, 0.4)' : '1px solid var(--glass-border)',
                background: activeView === 'billing' ? 'linear-gradient(135deg, #ea580c 0%, #f59e0b 100%)' : 'rgba(148, 163, 184, 0.08)',
                color: activeView === 'billing' ? '#ffffff' : 'var(--text-main)',
                boxShadow: activeView === 'billing' ? '0 2px 8px rgba(234, 88, 12, 0.25)' : 'none',
                transition: 'all 0.15s ease'
              }}
            >
              <FileText size={14} />
              <span>{t('navBilling')}</span>
            </button>
          </div>
        )}
      </div>

      {/* Center: Global Search Bar */}
      <div style={{ flex: '1', maxWidth: '300px', minWidth: '120px', position: 'relative', margin: '0 16px' }}>
        <Search 
          size={15} 
          style={{ 
            position: 'absolute', 
            left: '12px', 
            top: '50%', 
            transform: 'translateY(-50%)', 
            color: 'var(--text-muted)' 
          }} 
        />
        <input 
          type="text"
          value={searchQuery}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder={t('searchPlaceholder')}
          className="input-glass"
          style={{ paddingLeft: '36px', paddingRight: '36px', height: '36px', fontSize: '0.82rem' }}
        />
        <span style={{
          position: 'absolute',
          right: '10px',
          top: '50%',
          transform: 'translateY(-50%)',
          fontSize: '0.68rem',
          padding: '1px 5px',
          borderRadius: '4px',
          background: 'rgba(148, 163, 184, 0.15)',
          color: 'var(--text-muted)',
          fontFamily: 'var(--font-mono)'
        }}>
          /
        </span>
      </div>

      {/* Right Controls */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexShrink: 0 }}>
        {/* Terminal Quick Button */}
        <button 
          onClick={onOpenTerminal}
          className="btn btn-secondary btn-sm"
          title={t('navTerminal')}
          style={{ display: 'flex', alignItems: 'center', gap: '6px' }}
        >
          <span className="font-mono" style={{ fontSize: '0.78rem', fontWeight: 600 }}>&gt;_ SSH</span>
        </button>

        {/* Health Indicator Badge */}
        <div className="badge-online" style={{ cursor: 'default' }}>
          <span className="service-dot online" />
          <span>{t('healthy')}</span>
        </div>

        {/* Language Switcher */}
        <div style={{ position: 'relative' }}>
          <button 
            onClick={() => setShowLangMenu(!showLangMenu)}
            className="btn btn-icon"
            title={t('language')}
          >
            <Globe size={18} />
          </button>
          {showLangMenu && (
            <div className="glass-panel" style={{
              position: 'absolute',
              right: 0,
              top: '46px',
              width: '180px',
              padding: '6px',
              zIndex: 100,
              background: 'var(--glass-surface-solid)',
              boxShadow: 'var(--glass-shadow-lg)'
            }}>
              <button 
                onClick={() => { onLanguageChange('en'); setShowLangMenu(false); }}
                style={{
                  width: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '8px 12px',
                  borderRadius: '6px',
                  border: 'none',
                  background: language === 'en' ? 'rgba(234, 88, 12, 0.12)' : 'transparent',
                  color: language === 'en' ? 'var(--brand-primary)' : 'var(--text-main)',
                  fontWeight: language === 'en' ? 600 : 400,
                  cursor: 'pointer'
                }}
              >
                <span>English (US)</span>
                {language === 'en' && <Check size={14} />}
              </button>
              <button 
                onClick={() => { onLanguageChange('id'); setShowLangMenu(false); }}
                style={{
                  width: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '8px 12px',
                  borderRadius: '6px',
                  border: 'none',
                  background: language === 'id' ? 'rgba(234, 88, 12, 0.12)' : 'transparent',
                  color: language === 'id' ? 'var(--brand-primary)' : 'var(--text-main)',
                  fontWeight: language === 'id' ? 600 : 400,
                  cursor: 'pointer'
                }}
              >
                <span>Bahasa Indonesia</span>
                {language === 'id' && <Check size={14} />}
              </button>
            </div>
          )}
        </div>

        {/* Theme Toggle Button */}
        <button 
          onClick={onThemeToggle}
          className="btn btn-icon"
          title={theme === 'dark' ? t('lightMode') : t('darkMode')}
        >
          {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
        </button>

        {/* Notifications Button */}
        <div style={{ position: 'relative' }}>
          <button 
            onClick={() => setShowNotifications(!showNotifications)}
            className="btn btn-icon"
            style={{ position: 'relative' }}
          >
            <Bell size={18} />
            {notifications.length > 0 ? (
              <span style={{
                position: 'absolute',
                top: '6px',
                right: '6px',
                width: '8px',
                height: '8px',
                borderRadius: '50%',
                background: 'var(--brand-primary)'
              }} />
            ) : null}
          </button>
          {showNotifications && (
            <div className="glass-panel" style={{
              position: 'absolute',
              right: 0,
              top: '46px',
              width: '320px',
              padding: '16px',
              zIndex: 100,
              background: 'var(--glass-surface-solid)',
              boxShadow: 'var(--glass-shadow-lg)'
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
                <span style={{ fontWeight: 700, fontSize: '0.9rem' }}>{t('notifications')}</span>
                <span className={notifications.length > 0 ? 'badge-warning' : 'badge-online'}>{notifications.length}</span>
              </div>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', lineHeight: '1.4' }}>
                {notifications.length === 0 ? (
                  <div style={{ padding: '8px 0' }}>{t('headerNoNotifications')}</div>
                ) : (
                  notifications.map((notification, index) => (
                    <div key={`${notification.title}-${index}`} style={{ padding: '8px 0', borderBottom: index < notifications.length - 1 ? '1px solid var(--glass-border)' : 'none' }}>
                      <div style={{ fontWeight: 600, color: 'var(--text-main)' }}>{notification.title}</div>
                      <div>{notification.detail}</div>
                    </div>
                  ))
                )}
              </div>
            </div>
          )}
        </div>

        {/* Persona Role Switcher Dropdown */}
        <div style={{ position: 'relative' }}>
          <button 
            onClick={() => setShowRoleMenu(!showRoleMenu)}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              padding: '4px 10px 4px 6px',
              borderRadius: '24px',
              background: 'var(--glass-surface)',
              border: '1px solid var(--glass-border)',
              cursor: 'pointer',
              color: 'var(--text-main)'
            }}
          >
            <div style={{
              width: '28px',
              height: '28px',
              borderRadius: '50%',
              background: currentRole === 'admin' ? '#0f172a' : '#ea580c',
              color: '#ffffff',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontWeight: 700,
              fontSize: '0.8rem'
            }}>
              {currentRole === 'admin' ? 'A' : currentRole === 'reseller' ? 'R' : 'T'}
            </div>
            <div style={{ textAlign: 'left', lineHeight: 1.1 }}>
              <div style={{ fontSize: '0.78rem', fontWeight: 700 }}>
                {displayName || (currentRole === 'admin' ? t('headerRootAdmin') : currentRole === 'reseller' ? t('headerResellerPro') : t('clientPanel'))}
              </div>
            </div>
            <ChevronDown size={14} style={{ color: 'var(--text-muted)' }} />
          </button>

          {showRoleMenu && (
            <div className="glass-panel" style={{
              position: 'absolute',
              right: 0,
              top: '46px',
              width: '240px',
              padding: '8px',
              zIndex: 100,
              background: 'var(--glass-surface-solid)',
              boxShadow: 'var(--glass-shadow-lg)'
            }}>
              <div style={{ 
                padding: '6px 10px', 
                fontSize: '0.72rem', 
                fontWeight: 700, 
                color: 'var(--text-muted)', 
                textTransform: 'uppercase' 
              }}>
                {t('headerSignedInAs')}
              </div>
              <div style={{ padding: '8px 12px', display: 'flex', alignItems: 'center', gap: '10px' }}>
                <ShieldCheck size={16} style={{ color: 'var(--brand-primary)' }} />
                <div>
                  <div style={{ fontSize: '0.82rem', fontWeight: 700 }}>{displayName}</div>
                  <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>{roleLabels[currentRole]}</div>
                </div>
              </div>
              <button
                type="button"
                onClick={() => {
                  setShowRoleMenu(false);
                  onSignOut();
                }}
                style={{
                  width: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '10px',
                  padding: '8px 12px',
                  borderRadius: '6px',
                  border: 'none',
                  background: 'transparent',
                  color: '#ef4444',
                  fontWeight: 600,
                  cursor: 'pointer',
                  textAlign: 'left'
                }}
              >
                <LogOut size={16} />
                <span>{t('headerSignOut')}</span>
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
};
