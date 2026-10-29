// DigiPanel Top Navigation Bar
// Zero emojis, SVG iconography, modern glass styling

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
  User, 
  ChevronDown, 
  Check, 
  SlidersHorizontal,
  FileText,
  Activity
} from 'lucide-react';
import type { UserRole, Language, ThemeMode } from '../types';
import { getTranslation } from '../i18n/translations';

interface HeaderProps {
  currentRole: UserRole;
  onRoleChange: (role: UserRole) => void;
  activeView: string;
  onViewChange: (view: string) => void;
  language: Language;
  onLanguageChange: (lang: Language) => void;
  theme: ThemeMode;
  onThemeToggle: () => void;
  searchQuery: string;
  onSearchChange: (q: string) => void;
  onOpenTerminal: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  currentRole,
  onRoleChange,
  activeView,
  onViewChange,
  language,
  onLanguageChange,
  theme,
  onThemeToggle,
  searchQuery,
  onSearchChange,
  onOpenTerminal
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
      position: 'sticky',
      top: 0,
      zIndex: 50,
      background: 'var(--topbar-bg)',
      backdropFilter: 'blur(20px) saturate(180%)',
      WebkitBackdropFilter: 'blur(20px) saturate(180%)',
      borderBottom: '1px solid var(--glass-border)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '0 24px',
      transition: 'background 0.2s ease, border-color 0.2s ease'
    }}>
      {/* Left: Brand & Role Switcher */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
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
          <div>
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
        </div>

        {/* Hostname Chip (as in AdminBolt image) */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          background: 'rgba(148, 163, 184, 0.08)',
          border: '1px solid var(--glass-border)',
          borderRadius: '20px',
          padding: '4px 12px',
          fontSize: '0.8rem'
        }}>
          <Server size={14} style={{ color: 'var(--text-muted)' }} />
          <span style={{ color: 'var(--text-muted)' }}>{t('hostnameLabel')}:</span>
          <span className="font-mono" style={{ fontWeight: 600, color: 'var(--text-main)' }}>
            panel.digihost.id
          </span>
        </div>

        {/* View Shortcuts */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          {currentRole === 'admin' ? (
            <>
              <button 
                onClick={() => onViewChange('dashboard')}
                className={`btn btn-sm ${activeView === 'dashboard' ? 'btn-primary' : 'btn-secondary'}`}
              >
                <Activity size={14} />
                {t('navDashboard')}
              </button>
              <button 
                onClick={() => onViewChange('cluster')}
                className={`btn btn-sm ${activeView === 'cluster' ? 'btn-primary' : 'btn-secondary'}`}
              >
                <Server size={14} />
                {t('navCluster')}
              </button>
              <button 
                onClick={() => onViewChange('billing')}
                className={`btn btn-sm ${activeView === 'billing' ? 'btn-primary' : 'btn-secondary'}`}
              >
                <FileText size={14} />
                {t('navBilling')}
              </button>
            </>
          ) : (
            <>
              <button 
                onClick={() => onViewChange('home')}
                className={`btn btn-sm ${activeView === 'home' ? 'btn-primary' : 'btn-secondary'}`}
              >
                <Activity size={14} />
                {t('navHome')}
              </button>
              <button 
                onClick={() => onViewChange('tools')}
                className={`btn btn-sm ${activeView === 'tools' ? 'btn-primary' : 'btn-secondary'}`}
              >
                <SlidersHorizontal size={14} />
                {t('searchTools')}
              </button>
            </>
          )}
        </div>
      </div>

      {/* Center: Search Bar */}
      <div style={{ flex: '0 1 380px', position: 'relative', margin: '0 20px' }}>
        <Search 
          size={16} 
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
          style={{ paddingLeft: '38px', paddingRight: '48px', height: '38px' }}
        />
        <span style={{
          position: 'absolute',
          right: '10px',
          top: '50%',
          transform: 'translateY(-50%)',
          fontSize: '0.7rem',
          padding: '2px 6px',
          borderRadius: '4px',
          background: 'rgba(148, 163, 184, 0.15)',
          color: 'var(--text-muted)',
          fontFamily: 'var(--font-mono)'
        }}>
          /
        </span>
      </div>

      {/* Right Controls */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
        {/* Terminal Quick Button */}
        <button 
          onClick={onOpenTerminal}
          className="btn btn-secondary btn-sm"
          title={t('navTerminal')}
          style={{ display: 'flex', alignItems: 'center', gap: '6px' }}
        >
          <span className="font-mono" style={{ fontSize: '0.8rem', fontWeight: 600 }}>&gt;_ SSH</span>
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
            <span style={{
              position: 'absolute',
              top: '6px',
              right: '6px',
              width: '8px',
              height: '8px',
              borderRadius: '50%',
              background: 'var(--brand-primary)'
            }} />
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
                <span className="badge-online">3 Active Services</span>
              </div>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', lineHeight: '1.4' }}>
                <div style={{ padding: '8px 0', borderBottom: '1px solid var(--glass-border)' }}>
                  <div style={{ fontWeight: 600, color: 'var(--text-main)' }}>ACME SSL Auto-Renewed</div>
                  <div>Issued Let's Encrypt wildcard cert for *.digipanel.live</div>
                </div>
                <div style={{ padding: '8px 0' }}>
                  <div style={{ fontWeight: 600, color: 'var(--text-main)' }}>Cluster Node Health</div>
                  <div>All 3 fleet nodes responding with &lt; 5ms latency.</div>
                </div>
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
              gap: '10px',
              padding: '4px 10px 4px 6px',
              borderRadius: '24px',
              background: 'var(--glass-surface)',
              border: '1px solid var(--glass-border)',
              cursor: 'pointer',
              color: 'var(--text-main)'
            }}
          >
            <div style={{
              width: '30px',
              height: '30px',
              borderRadius: '50%',
              background: currentRole === 'admin' ? '#0f172a' : '#ea580c',
              color: '#ffffff',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontWeight: 700,
              fontSize: '0.85rem'
            }}>
              {currentRole === 'admin' ? 'A' : currentRole === 'reseller' ? 'R' : 'T'}
            </div>
            <div style={{ textAlign: 'left', lineHeight: 1.1 }}>
              <div style={{ fontSize: '0.8rem', fontWeight: 700 }}>
                {currentRole === 'admin' ? 'Root Admin' : currentRole === 'reseller' ? 'Reseller Pro' : 'testuser'}
              </div>
              <div style={{ fontSize: '0.68rem', color: 'var(--text-muted)' }}>
                {roleLabels[currentRole]}
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
                Switch Persona
              </div>
              <button 
                onClick={() => { 
                  onRoleChange('admin'); 
                  onViewChange('dashboard'); 
                  setShowRoleMenu(false); 
                }}
                style={{
                  width: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '10px',
                  padding: '8px 12px',
                  borderRadius: '6px',
                  border: 'none',
                  background: currentRole === 'admin' ? 'rgba(234, 88, 12, 0.12)' : 'transparent',
                  color: currentRole === 'admin' ? 'var(--brand-primary)' : 'var(--text-main)',
                  fontWeight: currentRole === 'admin' ? 700 : 500,
                  cursor: 'pointer',
                  textAlign: 'left'
                }}
              >
                <ShieldCheck size={16} />
                <div>
                  <div>{t('adminPanel')}</div>
                  <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>Server Fleet, WHM & Cluster</div>
                </div>
              </button>
              <button 
                onClick={() => { 
                  onRoleChange('reseller'); 
                  onViewChange('dashboard'); 
                  setShowRoleMenu(false); 
                }}
                style={{
                  width: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '10px',
                  padding: '8px 12px',
                  borderRadius: '6px',
                  border: 'none',
                  background: currentRole === 'reseller' ? 'rgba(234, 88, 12, 0.12)' : 'transparent',
                  color: currentRole === 'reseller' ? 'var(--brand-primary)' : 'var(--text-main)',
                  fontWeight: currentRole === 'reseller' ? 700 : 500,
                  cursor: 'pointer',
                  textAlign: 'left'
                }}
              >
                <User size={16} />
                <div>
                  <div>{t('resellerPanel')}</div>
                  <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>Sub-accounts & Quotas</div>
                </div>
              </button>
              <button 
                onClick={() => { 
                  onRoleChange('user'); 
                  onViewChange('home'); 
                  setShowRoleMenu(false); 
                }}
                style={{
                  width: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '10px',
                  padding: '8px 12px',
                  borderRadius: '6px',
                  border: 'none',
                  background: currentRole === 'user' ? 'rgba(234, 88, 12, 0.12)' : 'transparent',
                  color: currentRole === 'user' ? 'var(--brand-primary)' : 'var(--text-main)',
                  fontWeight: currentRole === 'user' ? 700 : 500,
                  cursor: 'pointer',
                  textAlign: 'left'
                }}
              >
                <Server size={16} />
                <div>
                  <div>{t('clientPanel')}</div>
                  <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>cPanel / Self-Service</div>
                </div>
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
};
