// DigiPanel Global Command Palette (Ctrl+K / Cmd+K / /)
// Liquid Glass + Claymorphism instant navigation across views, tools, and accounts.
// Zero emojis, full i18n, keyboard navigable.

import React, { useMemo, useState } from 'react';
import {
  Search,
  Activity,
  Network,
  Cpu,
  FileText,
  Users,
  Database,
  HardDrive,
  ShieldAlert,
  Boxes,
  Globe,
  ShieldCheck,
  Mail,
  Folder,
  Terminal,
  UserPlus,
  CornerDownLeft,
  X
} from 'lucide-react';
import type { HostingAccount, Language, UserRole } from '../types';
import { getTranslation } from '../i18n/translations';

export interface CommandPaletteProps {
  isOpen: boolean;
  onClose: () => void;
  language: Language;
  role: UserRole;
  accounts: HostingAccount[];
  initialQuery?: string;
  onNavigateView: (view: string) => void;
  onOpenModal: (modal: 'new_account' | 'dns' | 'ssl' | 'mail' | 'files' | 'terminal') => void;
  onSelectAccount: (username: string) => void;
}

interface PaletteItem {
  id: string;
  group: 'views' | 'actions' | 'accounts';
  label: string;
  subtitle?: string;
  icon: React.ComponentType<{ size?: number }>;
  run: () => void;
}

export const CommandPalette: React.FC<CommandPaletteProps> = ({
  isOpen,
  onClose,
  language,
  role,
  accounts,
  initialQuery = '',
  onNavigateView,
  onOpenModal,
  onSelectAccount
}) => {
  const t = (key: any) => getTranslation(language, key);
  const [query, setQuery] = useState(initialQuery);
  const [selectedIndex, setSelectedIndex] = useState(0);

  const items = useMemo<PaletteItem[]>(() => {
    const tr = (key: any) => getTranslation(language, key);
    const viewItems: PaletteItem[] = [
      { id: 'v-dash', group: 'views', label: role === 'user' ? tr('navHome') : tr('navDashboard'), icon: Activity, run: () => onNavigateView(role === 'user' ? 'home' : 'dashboard') },
      { id: 'v-apps', group: 'views', label: tr('navAppMarketplace'), icon: Boxes, run: () => onNavigateView('apps') },
      { id: 'v-runtimes', group: 'views', label: tr('navRuntimes'), icon: Cpu, run: () => onNavigateView('runtimes') },
      { id: 'v-db', group: 'views', label: tr('navDatabases'), icon: Database, run: () => onNavigateView('databases') },
      { id: 'v-bak', group: 'views', label: tr('navBackups'), icon: HardDrive, run: () => onNavigateView('backups') },
      { id: 'v-bill', group: 'views', label: tr('navBilling'), icon: FileText, run: () => onNavigateView('billing') },
      { id: 'v-stat', group: 'views', label: tr('navServerStatus'), icon: Activity, run: () => onNavigateView('server_status') },
    ];
    if (role === 'admin') {
      viewItems.push(
        { id: 'v-cluster', group: 'views', label: tr('navHubDashboard'), icon: Network, run: () => onNavigateView('cluster') },
        { id: 'v-accts', group: 'views', label: tr('actionAllAccounts'), icon: Users, run: () => onNavigateView('accounts_list') },
        { id: 'v-fw', group: 'views', label: tr('navFirewall'), icon: ShieldAlert, run: () => onNavigateView('firewall') },
        { id: 'v-audit', group: 'views', label: tr('auditTitle'), icon: ShieldCheck, run: () => onNavigateView('audit') }
      );
    }

    const actionItems: PaletteItem[] = [
      { id: 'a-dns', group: 'actions', label: tr('modalDnsEditorTitle'), icon: Globe, run: () => onOpenModal('dns') },
      { id: 'a-ssl', group: 'actions', label: tr('sslManagerTitle'), icon: ShieldCheck, run: () => onOpenModal('ssl') },
      { id: 'a-mail', group: 'actions', label: tr('mailManagerTitle'), icon: Mail, run: () => onOpenModal('mail') },
      { id: 'a-files', group: 'actions', label: tr('navFiles'), icon: Folder, run: () => onOpenModal('files') },
      { id: 'a-term', group: 'actions', label: tr('terminalTitle'), icon: Terminal, run: () => onOpenModal('terminal') },
    ];
    if (role !== 'user') {
      actionItems.unshift({
        id: 'a-new', group: 'actions', label: tr('actionNewAccount'), icon: UserPlus, run: () => onOpenModal('new_account')
      });
    }

    const accountItems: PaletteItem[] = role !== 'user'
      ? accounts.slice(0, 12).map((acc) => ({
          id: `acc-${acc.id}`,
          group: 'accounts',
          label: acc.username,
          subtitle: `${acc.domain} (${acc.plan})`,
          icon: Users,
          run: () => onSelectAccount(acc.username)
        }))
      : [];

    const all = [...actionItems, ...viewItems, ...accountItems];
    const q = query.trim().toLowerCase();
    if (!q) return all.slice(0, 12);
    return all.filter((item) =>
      item.label.toLowerCase().includes(q) || (item.subtitle && item.subtitle.toLowerCase().includes(q))
    );
  }, [accounts, language, onNavigateView, onOpenModal, onSelectAccount, query, role]);

  const activeIndex = items.length === 0 ? 0 : Math.min(selectedIndex, items.length - 1);

  if (!isOpen) return null;

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
      return;
    }
    if (e.key === 'ArrowDown' && items.length > 0) {
      e.preventDefault();
      setSelectedIndex((activeIndex + 1) % items.length);
      return;
    }
    if (e.key === 'ArrowUp' && items.length > 0) {
      e.preventDefault();
      setSelectedIndex((activeIndex - 1 + items.length) % items.length);
      return;
    }
    if (e.key === 'Enter' && items[activeIndex]) {
      e.preventDefault();
      items[activeIndex].run();
      onClose();
    }
  };

  const groupLabel = (g: PaletteItem['group']) => {
    if (g === 'actions') return t('cmdPaletteGroupActions');
    if (g === 'accounts') return t('cmdPaletteGroupAccounts');
    return t('cmdPaletteGroupViews');
  };

  return (
    <div className="modal-backdrop" onClick={onClose} style={{ alignItems: 'flex-start', paddingTop: '10vh' }}>
      <div
        className="modal-content"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={handleKeyDown}
        style={{ maxWidth: '600px', padding: '18px', display: 'flex', flexDirection: 'column', gap: '12px' }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <div style={{ position: 'relative', flex: 1 }}>
            <Search size={16} style={{ position: 'absolute', left: '14px', top: '50%', transform: 'translateY(-50%)', color: 'var(--brand-primary)' }} />
            <input
              type="text"
              className="input-glass"
              autoFocus
              value={query}
              onChange={(e) => { setQuery(e.target.value); setSelectedIndex(0); }}
              placeholder={t('cmdPalettePlaceholder')}
              style={{ paddingLeft: '40px', height: '44px', fontSize: '0.92rem' }}
            />
          </div>
          <button type="button" onClick={onClose} className="btn-icon" title={t('close')}>
            <X size={16} />
          </button>
        </div>

        <div style={{ maxHeight: '360px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '4px', paddingRight: '2px' }}>
          {items.length === 0 ? (
            <div style={{ padding: '28px', textAlign: 'center', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
              {t('cmdPaletteNoResults')}
            </div>
          ) : (
            items.map((item, idx) => {
              const active = idx === activeIndex;
              const prevGroup = idx > 0 ? items[idx - 1].group : null;
              return (
                <React.Fragment key={item.id}>
                  {item.group !== prevGroup && (
                    <div style={{ fontSize: '0.68rem', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--text-dim)', padding: '6px 10px 2px' }}>
                      {groupLabel(item.group)}
                    </div>
                  )}
                  <button
                    type="button"
                    onMouseEnter={() => setSelectedIndex(idx)}
                    onClick={() => { item.run(); onClose(); }}
                    className={`sidebar-nav-pill ${active ? 'active' : ''}`}
                    style={{ padding: '10px 12px' }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px', minWidth: 0 }}>
                      <div className="clay-icon" style={{ width: '30px', height: '30px', borderRadius: '10px', background: active ? 'var(--brand-primary-light)' : 'var(--glass-surface)', color: active ? 'var(--brand-primary)' : 'var(--text-muted)' }}>
                        <item.icon size={15} />
                      </div>
                      <div style={{ textAlign: 'left', overflow: 'hidden' }}>
                        <div style={{ fontWeight: 700, fontSize: '0.84rem' }}>{item.label}</div>
                        {item.subtitle && (
                          <div className="font-mono" style={{ fontSize: '0.72rem', color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                            {item.subtitle}
                          </div>
                        )}
                      </div>
                    </div>
                    {active && <CornerDownLeft size={14} style={{ color: 'var(--brand-primary)', flexShrink: 0 }} />}
                  </button>
                </React.Fragment>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
};
