// DigiPanel Web SSH Terminal Modal
// Zero emojis, strictly Lucide SVG iconography, modern glass styling

import React, { useState } from 'react';
import { X } from 'lucide-react';
import type { Language } from '../types';
import { getTranslation } from '../i18n/translations';

interface TerminalModalProps {
  isOpen: boolean;
  onClose: () => void;
  language: Language;
}

export const TerminalModal: React.FC<TerminalModalProps> = ({
  isOpen,
  onClose,
  language
}) => {
  const t = (key: any) => getTranslation(language, key);

  const [inputCommand, setInputCommand] = useState('');
  const [history, setHistory] = useState<Array<{ cmd?: string; output: string }>>([
    {
      output: t('terminalBanner')
    }
  ]);

  if (!isOpen) return null;

  const handleRunCommand = async (e: React.FormEvent) => {
    e.preventDefault();
    const cmd = inputCommand.trim();
    if (!cmd) return;

    if (cmd.toLowerCase() === 'clear') {
      setHistory([]);
      setInputCommand('');
      return;
    }

    try {
      const res = await fetch('/api/v1/terminal/exec', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ command: cmd })
      });
      if (res.ok) {
        const data = await res.json();
        setHistory(prev => [...prev, { cmd, output: data.output || t('terminalOk') }]);
      } else {
        setHistory(prev => [...prev, { cmd, output: t('terminalHttpError') }]);
      }
    } catch {
      setHistory(prev => [...prev, { cmd, output: t('terminalConnError') }]);
    }
    setInputCommand('');
  };

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div 
        className="modal-content" 
        onClick={(e) => e.stopPropagation()} 
        style={{ maxWidth: '820px', padding: 0, overflow: 'hidden', background: '#090d16', border: '1px solid rgba(255, 255, 255, 0.12)' }}
      >
        {/* Terminal Header */}
        <div style={{
          padding: '12px 18px',
          background: 'rgba(255, 255, 255, 0.04)',
          borderBottom: '1px solid rgba(255, 255, 255, 0.08)',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span style={{ width: '10px', height: '10px', borderRadius: '50%', background: '#ef4444' }} />
            <span style={{ width: '10px', height: '10px', borderRadius: '50%', background: '#f59e0b' }} />
            <span style={{ width: '10px', height: '10px', borderRadius: '50%', background: '#10b981' }} />
            <span className="font-mono" style={{ fontSize: '0.8rem', color: '#94a3b8', marginLeft: '10px' }}>
              root@panel.digihost.id:~ (SSH Session)
            </span>
          </div>
          <button onClick={onClose} className="btn-icon" style={{ color: '#94a3b8' }}>
            <X size={16} />
          </button>
        </div>

        {/* Output Screen */}
        <div style={{
          padding: '18px 20px',
          minHeight: '340px',
          maxHeight: '440px',
          overflowY: 'auto',
          fontSize: '0.82rem',
          color: '#e2e8f0',
          lineHeight: '1.6'
        }}>
          {history.map((item, idx) => (
            <div key={idx} style={{ marginBottom: '12px' }}>
              {item.cmd && (
                <div style={{ display: 'flex', gap: '8px', color: '#38bdf8' }}>
                  <span style={{ color: '#f59e0b' }}>root@panel:#</span>
                  <span className="font-mono">{item.cmd}</span>
                </div>
              )}
              <pre className="font-mono" style={{ whiteSpace: 'pre-wrap', color: '#cbd5e1', marginTop: '2px' }}>
                {item.output}
              </pre>
            </div>
          ))}

          {/* Interactive Input Form */}
          <form onSubmit={handleRunCommand} style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '10px' }}>
            <span className="font-mono" style={{ color: '#f59e0b' }}>root@panel:#</span>
            <input 
              type="text"
              value={inputCommand}
              onChange={(e) => setInputCommand(e.target.value)}
              placeholder={t('terminalPlaceholder')}
              className="font-mono"
              autoFocus
              style={{
                flex: 1,
                background: 'transparent',
                border: 'none',
                color: '#38bdf8',
                outline: 'none',
                fontSize: '0.82rem'
              }}
            />
          </form>
        </div>
      </div>
    </div>
  );
};
