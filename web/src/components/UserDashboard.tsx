// DigiPanel User / Client Dashboard (Matching Image 1)
// Zero emojis, strictly Lucide SVG iconography, modern glass styling

import React, { useState } from 'react';
import { 
  Sparkles, 
  Send, 
  Globe, 
  Layers, 
  Folder, 
  ShieldCheck, 
  ChevronDown, 
  ChevronUp, 
  ExternalLink, 
  MessageSquare, 
  Check, 
  Copy, 
  BarChart3, 
  Terminal, 
  FileText,
  AlertCircle,
  HardDrive
} from 'lucide-react';
import type { AiChatMessage, Language } from '../types';
import { getTranslation } from '../i18n/translations';

interface UserDashboardProps {
  language: Language;
  onOpenDnsEditor: () => void;
  onOpenFileBrowser: () => void;
  onOpenWhatsApp: () => void;
  onOpenTerminal: () => void;
  selectedCategory?: string;
}

export const UserDashboard: React.FC<UserDashboardProps> = ({
  language,
  onOpenDnsEditor,
  onOpenFileBrowser,
  onOpenWhatsApp,
  onOpenTerminal,
  selectedCategory = 'all'
}) => {
  const t = (key: any) => getTranslation(language, key);

  // AI Assistant Chat State
  const [chatMessages, setChatMessages] = useState<AiChatMessage[]>([
    {
      id: 'msg-1',
      sender: 'assistant',
      timestamp: '10:00 AM',
      content: language === 'id' 
        ? 'Halo testuser. Saya siap membantu diagnosis hosting, perpanjangan SSL, pengelolaan DNS PowerDNS, atau optimasi WordPress. Ada yang bisa dibantu?' 
        : 'Hello testuser. I am your automated hosting assistant. I can help configure DNS zones, manage Let\'s Encrypt SSL, check disk allocations, or spin up WordPress instances.'
    }
  ]);
  const [chatInput, setChatInput] = useState('');
  const [isTyping, setIsTyping] = useState(false);
  const [copiedDomain, setCopiedDomain] = useState(false);

  // Collapsible Categories
  const [openSections, setOpenSections] = useState<Record<string, boolean>>({
    domains: true,
    wordpress: true,
    files: true,
    databases: false,
    emails: false
  });

  const toggleSection = (section: string) => {
    setOpenSections(prev => ({ ...prev, [section]: !prev[section] }));
  };

  const handleSendMessage = (textToSend?: string) => {
    const text = textToSend || chatInput;
    if (!text.trim()) return;

    const userMsg: AiChatMessage = {
      id: `msg-${Date.now()}`,
      sender: 'user',
      timestamp: 'Just now',
      content: text
    };

    setChatMessages(prev => [...prev, userMsg]);
    if (!textToSend) setChatInput('');
    setIsTyping(true);

    setTimeout(() => {
      let botResponse = '';
      let action: any = undefined;

      const lower = text.toLowerCase();
      if (lower.includes('ssl') || lower.includes('cert') || lower.includes('https')) {
        botResponse = language === 'id' 
          ? 'Sertifikat SSL Let\'s Encrypt aktif untuk testuser.digipanel.live. Auto-renewal dijadwalkan otomatis melalui ACME daemon setiap 60 hari.'
          : 'Let\'s Encrypt SSL is active for testuser.digipanel.live. Auto-renewal is managed by the ACME daemon every 60 days with HTTP-01 verification.';
        action = { label: 'Open DNS & SSL Manager', actionType: 'open_dns' };
      } else if (lower.includes('disk') || lower.includes('storage') || lower.includes('quota')) {
        botResponse = language === 'id'
          ? 'Kapasitas disk Anda saat ini 0.06 MB dari 50 MB kuota terpakai (0.1%). Tidak ada berkas log usang atau database berlebih yang perlu dibersihkan.'
          : 'Your current disk allocation is 0.06 MB / 50 MB (0.1% utilized). Logs and database indexes are optimal.';
        action = { label: 'Open File Manager', actionType: 'open_files' };
      } else if (lower.includes('dns') || lower.includes('cloudflare')) {
        botResponse = language === 'id'
          ? 'Zona DNS dikelola oleh PowerDNS engine. A record mengarah ke 103.189.201.42. Anda dapat menambah A, CNAME, atau MX record langsung.'
          : 'DNS zone is managed by the PowerDNS cluster. Apex A record resolves to 103.189.201.42. You can manage CNAME, MX, and TXT records anytime.';
        action = { label: 'Launch Zone Editor', actionType: 'open_dns' };
      } else {
        botResponse = language === 'id'
          ? 'Perintah diterima. Sistem DigiPanel memverifikasi lingkungan PHP 8.3 FPM, MariaDB 10.11, dan isolasi cgroups v2 beroperasi normal.'
          : 'Query processed. DigiPanel verifies PHP 8.3 FPM runtime, MariaDB 10.11, and cgroups v2 process isolation are operating nominally.';
      }

      setChatMessages(prev => [
        ...prev,
        {
          id: `msg-${Date.now() + 1}`,
          sender: 'assistant',
          timestamp: 'Just now',
          content: botResponse,
          suggestedAction: action
        }
      ]);
      setIsTyping(false);
    }, 600);
  };

  const copyTemporaryDomain = () => {
    navigator.clipboard.writeText('testuser.digipanel.live');
    setCopiedDomain(true);
    setTimeout(() => setCopiedDomain(false), 2000);
  };

  return (
    <div style={{
      display: 'grid',
      gridTemplateColumns: 'minmax(0, 1fr) 340px',
      gap: '24px',
      alignItems: 'start'
    }}>
      {/* Left Column: AI Assistant + Collapsible Hosting Tools */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
        <h1 style={{ fontSize: '1.5rem', fontWeight: 800, color: 'var(--text-main)', letterSpacing: '-0.02em' }}>
          {t('navHome')}
        </h1>

        {/* AI Assistant Card (Image 1) */}
        <div className="glass-panel" style={{
          padding: '24px',
          background: 'var(--glass-surface)',
          border: '1px solid var(--glass-border-highlight)'
        }}>
          {/* Header */}
          <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            marginBottom: '16px'
          }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              <div style={{
                width: '32px',
                height: '32px',
                borderRadius: '8px',
                background: 'rgba(245, 158, 11, 0.15)',
                color: 'var(--brand-accent)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center'
              }}>
                <Sparkles size={18} />
              </div>
              <h3 style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--text-main)' }}>
                {t('aiAssistantTitle')}
              </h3>
            </div>
            <button 
              onClick={() => setChatMessages([])}
              className="btn btn-secondary btn-sm"
              style={{ fontSize: '0.75rem', padding: '4px 8px' }}
            >
              {t('clearHistory')}
            </button>
          </div>

          {/* Assistant Visual Banner / Prompt suggestions */}
          <div style={{
            padding: '20px',
            borderRadius: '12px',
            background: 'rgba(148, 163, 184, 0.05)',
            border: '1px solid var(--glass-border)',
            textAlign: 'center',
            marginBottom: '16px'
          }}>
            {/* SVG Workstation Illustration */}
            <div style={{ display: 'inline-block', marginBottom: '12px' }}>
              <svg width="120" height="75" viewBox="0 0 120 75" fill="none" xmlns="http://www.w3.org/2000/svg">
                <rect x="25" y="15" width="70" height="42" rx="6" fill="rgba(234, 88, 12, 0.12)" stroke="var(--brand-primary)" strokeWidth="1.5"/>
                <path d="M40 30H80M40 38H65M40 46H72" stroke="var(--brand-primary)" strokeWidth="1.5" strokeLinecap="round"/>
                <circle cx="85" cy="46" r="3" fill="var(--status-online)"/>
                <rect x="50" y="57" width="20" height="8" rx="2" fill="var(--glass-border)"/>
                <path d="M35 65H85" stroke="var(--text-dim)" strokeWidth="2" strokeLinecap="round"/>
              </svg>
            </div>
            <p style={{ fontSize: '0.86rem', color: 'var(--text-muted)', marginBottom: '12px' }}>
              {t('aiAssistantGreeting')}
            </p>

            {/* Quick Prompt Suggestions */}
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', justifyContent: 'center' }}>
              {[
                t('aiSuggestedPrompt1'),
                t('aiSuggestedPrompt2'),
                t('aiSuggestedPrompt3'),
              ].map((suggestion, idx) => (
                <button
                  key={idx}
                  onClick={() => handleSendMessage(suggestion)}
                  style={{
                    fontSize: '0.75rem',
                    padding: '5px 12px',
                    borderRadius: '20px',
                    border: '1px solid var(--glass-border)',
                    background: 'var(--glass-surface-hover)',
                    color: 'var(--text-main)',
                    cursor: 'pointer',
                    transition: 'all 0.15s ease'
                  }}
                >
                  {suggestion}
                </button>
              ))}
            </div>
          </div>

          {/* Chat Stream */}
          {chatMessages.length > 0 && (
            <div style={{
              display: 'flex',
              flexDirection: 'column',
              gap: '10px',
              maxHeight: '220px',
              overflowY: 'auto',
              marginBottom: '16px',
              paddingRight: '6px'
            }}>
              {chatMessages.map(msg => (
                <div 
                  key={msg.id}
                  style={{
                    alignSelf: msg.sender === 'user' ? 'flex-end' : 'flex-start',
                    maxWidth: '85%',
                    padding: '10px 14px',
                    borderRadius: '12px',
                    fontSize: '0.84rem',
                    background: msg.sender === 'user' ? 'linear-gradient(135deg, #ea580c 0%, #f59e0b 100%)' : 'rgba(148, 163, 184, 0.08)',
                    color: msg.sender === 'user' ? '#ffffff' : 'var(--text-main)',
                    border: msg.sender === 'user' ? 'none' : '1px solid var(--glass-border)',
                    boxShadow: 'var(--glass-shadow)'
                  }}
                >
                  <div>{msg.content}</div>
                  {msg.suggestedAction && (
                    <button
                      onClick={() => {
                        if (msg.suggestedAction?.actionType === 'open_dns') onOpenDnsEditor();
                        if (msg.suggestedAction?.actionType === 'open_files') onOpenFileBrowser();
                      }}
                      className="btn btn-secondary btn-sm"
                      style={{ marginTop: '8px', fontSize: '0.74rem' }}
                    >
                      <ExternalLink size={12} />
                      {msg.suggestedAction.label}
                    </button>
                  )}
                </div>
              ))}
              {isTyping && (
                <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)', fontStyle: 'italic' }}>
                  DigiPanel assistant is typing...
                </div>
              )}
            </div>
          )}

          {/* Chat Input Box */}
          <div style={{ display: 'flex', gap: '8px' }}>
            <input 
              type="text"
              placeholder={t('aiAssistantPlaceholder')}
              value={chatInput}
              onChange={(e) => setChatInput(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSendMessage()}
              className="input-glass"
              style={{ flex: 1, height: '42px' }}
            />
            <button 
              onClick={() => handleSendMessage()}
              className="btn btn-primary"
              style={{ padding: '0 18px', height: '42px' }}
            >
              <Send size={16} />
              <span>{t('aiAssistantSend')}</span>
            </button>
          </div>
        </div>

        {/* Section 1: Domains (Image 1) */}
        {(selectedCategory === 'all' || selectedCategory === 'domains') && (
          <div className="glass-panel" style={{ padding: '20px 24px' }}>
            <div 
              onClick={() => toggleSection('domains')}
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                cursor: 'pointer',
                marginBottom: openSections.domains ? '16px' : '0'
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <Globe size={18} style={{ color: 'var(--brand-primary)' }} />
                <h3 style={{ fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-main)' }}>
                  {t('navDomains')}
                </h3>
              </div>
              {openSections.domains ? <ChevronUp size={18} /> : <ChevronDown size={18} />}
            </div>

            {openSections.domains && (
              <div style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
                gap: '14px'
              }}>
                {[
                  { title: t('toolDomains'), icon: Globe, onClick: onOpenDnsEditor },
                  { title: t('toolApplications'), icon: Layers, onClick: () => {} },
                  { title: t('toolZoneEditor'), icon: Globe, onClick: onOpenDnsEditor },
                  { title: t('toolCloudflare'), icon: Globe, onClick: () => {} },
                  { title: t('toolDnsHealth'), icon: ShieldCheck, onClick: onOpenDnsEditor },
                  { title: t('toolRedirects'), icon: ExternalLink, onClick: () => {} },
                  { title: t('toolSslCertificates'), icon: ShieldCheck, onClick: onOpenDnsEditor },
                  { title: t('toolErrorPages'), icon: FileText, onClick: () => {} },
                  { title: t('toolAccessLogs'), icon: FileText, onClick: onOpenTerminal },
                  { title: t('toolErrorLogs'), icon: AlertCircle, onClick: onOpenTerminal }
                ].map((tool, idx) => (
                  <div
                    key={idx}
                    onClick={tool.onClick}
                    className="glass-card"
                    style={{
                      padding: '16px 14px',
                      display: 'flex',
                      alignItems: 'center',
                      gap: '12px',
                      cursor: 'pointer'
                    }}
                  >
                    <div style={{
                      width: '36px',
                      height: '36px',
                      borderRadius: '10px',
                      background: 'rgba(234, 88, 12, 0.12)',
                      color: 'var(--brand-primary)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      flexShrink: 0
                    }}>
                      <tool.icon size={18} />
                    </div>
                    <span style={{ fontSize: '0.84rem', fontWeight: 600, color: 'var(--text-main)' }}>
                      {tool.title}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Section 2: WordPress Management (Image 1) */}
        {(selectedCategory === 'all' || selectedCategory === 'software') && (
          <div className="glass-panel" style={{ padding: '20px 24px' }}>
            <div 
              onClick={() => toggleSection('wordpress')}
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                cursor: 'pointer',
                marginBottom: openSections.wordpress ? '16px' : '0'
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <Layers size={18} style={{ color: 'var(--status-info)' }} />
                <h3 style={{ fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-main)' }}>
                  {t('navWordPress')}
                </h3>
              </div>
              {openSections.wordpress ? <ChevronUp size={18} /> : <ChevronDown size={18} />}
            </div>

            {openSections.wordpress && (
              <div style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
                gap: '14px'
              }}>
                {[
                  { title: 'WordPress Toolkit', icon: Layers },
                  { title: 'Install WordPress 6.6', icon: Sparkles },
                  { title: 'Staging Environment', icon: HardDrive },
                  { title: 'Security Hardening', icon: ShieldCheck },
                  { title: 'Plugin Manager', icon: Layers },
                  { title: 'Clone & Migration', icon: ExternalLink }
                ].map((tool, idx) => (
                  <div
                    key={idx}
                    className="glass-card"
                    style={{
                      padding: '16px 14px',
                      display: 'flex',
                      alignItems: 'center',
                      gap: '12px',
                      cursor: 'pointer'
                    }}
                  >
                    <div style={{
                      width: '36px',
                      height: '36px',
                      borderRadius: '10px',
                      background: 'rgba(2, 132, 199, 0.12)',
                      color: 'var(--status-info)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      flexShrink: 0
                    }}>
                      <tool.icon size={18} />
                    </div>
                    <span style={{ fontSize: '0.84rem', fontWeight: 600, color: 'var(--text-main)' }}>
                      {tool.title}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Section 3: Files */}
        {(selectedCategory === 'all' || selectedCategory === 'files') && (
          <div className="glass-panel" style={{ padding: '20px 24px' }}>
            <div 
              onClick={() => toggleSection('files')}
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                cursor: 'pointer',
                marginBottom: openSections.files ? '16px' : '0'
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <Folder size={18} style={{ color: 'var(--brand-accent)' }} />
                <h3 style={{ fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-main)' }}>
                  {t('navFiles')}
                </h3>
              </div>
              {openSections.files ? <ChevronUp size={18} /> : <ChevronDown size={18} />}
            </div>

            {openSections.files && (
              <div style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
                gap: '14px'
              }}>
                {[
                  { title: t('toolFileManager'), icon: Folder, onClick: onOpenFileBrowser },
                  { title: t('toolImages'), icon: Folder, onClick: onOpenFileBrowser },
                  { title: t('toolDirectoryPrivacy'), icon: ShieldCheck, onClick: () => {} },
                  { title: t('toolDiskUsage'), icon: BarChart3, onClick: () => {} },
                  { title: t('toolWebDisk'), icon: HardDrive, onClick: () => {} },
                  { title: t('toolBackup'), icon: HardDrive, onClick: () => {} },
                  { title: t('toolGitVersionControl'), icon: Terminal, onClick: onOpenTerminal }
                ].map((tool, idx) => (
                  <div
                    key={idx}
                    onClick={tool.onClick}
                    className="glass-card"
                    style={{
                      padding: '16px 14px',
                      display: 'flex',
                      alignItems: 'center',
                      gap: '12px',
                      cursor: 'pointer'
                    }}
                  >
                    <div style={{
                      width: '36px',
                      height: '36px',
                      borderRadius: '10px',
                      background: 'rgba(245, 158, 11, 0.12)',
                      color: 'var(--brand-accent)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      flexShrink: 0
                    }}>
                      <tool.icon size={18} />
                    </div>
                    <span style={{ fontSize: '0.84rem', fontWeight: 600, color: 'var(--text-main)' }}>
                      {tool.title}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Right Column: General Information & Statistics Sidebar (Image 1) */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
        {/* General Information Card */}
        <div className="glass-panel" style={{ padding: '24px' }}>
          <h3 style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--text-main)', marginBottom: '16px' }}>
            {t('generalInformation')}
          </h3>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px', fontSize: '0.84rem' }}>
            {/* Current User */}
            <div>
              <div style={{ color: 'var(--text-muted)', fontSize: '0.78rem', marginBottom: '2px' }}>
                {t('currentUser')}
              </div>
              <div style={{ fontWeight: 700, color: 'var(--text-main)' }}>testuser</div>
            </div>

            {/* Shared IP Address */}
            <div>
              <div style={{ color: 'var(--text-muted)', fontSize: '0.78rem', marginBottom: '2px' }}>
                {t('sharedIpAddress')}
              </div>
              <div className="font-mono" style={{ fontWeight: 600, color: 'var(--text-main)' }}>
                103.189.201.42
              </div>
            </div>

            {/* Free Temporary Domains */}
            <div>
              <div style={{ color: 'var(--text-muted)', fontSize: '0.78rem', marginBottom: '2px' }}>
                {t('freeTemporaryDomains')}
              </div>
              <div style={{ fontSize: '0.74rem', color: 'var(--text-dim)', marginBottom: '6px', lineHeight: 1.3 }}>
                {t('temporaryDomainNotice')}
              </div>
              <div style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                padding: '6px 10px',
                borderRadius: '8px',
                background: 'rgba(148, 163, 184, 0.1)',
                border: '1px solid var(--glass-border)'
              }}>
                <span className="font-mono" style={{ fontSize: '0.8rem', fontWeight: 600 }}>
                  testuser.digipanel.live
                </span>
                <button
                  onClick={copyTemporaryDomain}
                  className="btn-icon"
                  style={{ padding: '4px' }}
                  title="Copy domain"
                >
                  {copiedDomain ? <Check size={14} style={{ color: 'var(--status-online)' }} /> : <Copy size={14} />}
                </button>
              </div>
            </div>

            {/* Home Directory */}
            <div>
              <div style={{ color: 'var(--text-muted)', fontSize: '0.78rem', marginBottom: '2px' }}>
                {t('homeDirectory')}
              </div>
              <div className="font-mono" style={{ fontWeight: 600, fontSize: '0.8rem', color: 'var(--text-main)' }}>
                /home/testuser
              </div>
            </div>

            {/* Last Login IP */}
            <div>
              <div style={{ color: 'var(--text-muted)', fontSize: '0.78rem', marginBottom: '2px' }}>
                {t('lastLoginIp')}
              </div>
              <div className="font-mono" style={{ fontWeight: 600, color: 'var(--text-main)' }}>
                182.253.112.90
              </div>
            </div>
          </div>
        </div>

        {/* WhatsApp Integration Card (Image 1) */}
        <div className="glass-panel" style={{
          padding: '18px 20px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
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
              <div style={{ fontWeight: 700, fontSize: '0.86rem', color: 'var(--text-main)' }}>
                WhatsApp
              </div>
              <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>
                {t('notConnected')}
              </div>
            </div>
          </div>

          <button 
            onClick={onOpenWhatsApp}
            className="btn btn-sm"
            style={{
              background: 'rgba(16, 185, 129, 0.18)',
              color: 'var(--status-online)',
              border: '1px solid rgba(16, 185, 129, 0.3)'
            }}
          >
            {t('connect')}
          </button>
        </div>

        {/* Statistics Card (Image 1) */}
        <div className="glass-panel" style={{ padding: '24px' }}>
          <h3 style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--text-main)', marginBottom: '16px' }}>
            {t('statistics')}
          </h3>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            {/* Disk Usage (Image 1 exact layout) */}
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
                <span style={{ fontSize: '0.84rem', fontWeight: 600, color: 'var(--text-main)' }}>
                  {t('diskUsage')}
                </span>
                <span className="font-mono" style={{ fontSize: '0.84rem', fontWeight: 700, color: 'var(--status-online)' }}>
                  0.1%
                </span>
              </div>
              <div className="font-mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)', marginBottom: '6px' }}>
                0.06MB / 50MB
              </div>
              <div className="progress-bar-container">
                <div className="progress-bar-fill" style={{ width: '0.1%', background: 'var(--status-online)' }} />
              </div>
              <div style={{ display: 'flex', gap: '12px', fontSize: '0.72rem', color: 'var(--text-dim)', marginTop: '4px' }}>
                <span>&bull; {t('filesLabel')} 0.06MB</span>
                <span>&bull; {t('databasesLabel')} 0MB</span>
              </div>
            </div>

            {/* Bandwidth Usage */}
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
                <span style={{ fontSize: '0.84rem', fontWeight: 600, color: 'var(--text-main)' }}>
                  {t('bandwidthUsage')}
                </span>
                <span className="font-mono" style={{ fontSize: '0.84rem', fontWeight: 700, color: 'var(--status-info)' }}>
                  2.8%
                </span>
              </div>
              <div className="font-mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)', marginBottom: '6px' }}>
                1.42 GB / 50 GB
              </div>
              <div className="progress-bar-container">
                <div className="progress-bar-fill" style={{ width: '2.8%', background: 'var(--status-info)' }} />
              </div>
            </div>

            {/* MySQL Databases */}
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
                <span style={{ fontSize: '0.84rem', fontWeight: 600, color: 'var(--text-main)' }}>
                  MySQL Databases
                </span>
                <span className="font-mono" style={{ fontSize: '0.84rem', fontWeight: 700 }}>
                  2 / 10
                </span>
              </div>
              <div className="progress-bar-container">
                <div className="progress-bar-fill" style={{ width: '20%', background: 'var(--brand-primary)' }} />
              </div>
            </div>

            {/* Email Accounts */}
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
                <span style={{ fontSize: '0.84rem', fontWeight: 600, color: 'var(--text-main)' }}>
                  Email Accounts
                </span>
                <span className="font-mono" style={{ fontSize: '0.84rem', fontWeight: 700 }}>
                  4 / 20
                </span>
              </div>
              <div className="progress-bar-container">
                <div className="progress-bar-fill" style={{ width: '20%', background: 'var(--brand-accent)' }} />
              </div>
            </div>

            {/* Inodes */}
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
                <span style={{ fontSize: '0.84rem', fontWeight: 600, color: 'var(--text-main)' }}>
                  {t('inodesUsage')}
                </span>
                <span className="font-mono" style={{ fontSize: '0.84rem', fontWeight: 700 }}>
                  3,727 / 100,000 (3.7%)
                </span>
              </div>
              <div className="progress-bar-container">
                <div className="progress-bar-fill" style={{ width: '3.7%', background: 'var(--status-online)' }} />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
