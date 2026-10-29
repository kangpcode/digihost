// DigiPanel Web SSH Terminal Modal
// Zero emojis, strictly Lucide SVG iconography, modern glass styling

import React, { useState } from 'react';
import { X } from 'lucide-react';
import type { Language } from '../types';

interface TerminalModalProps {
  isOpen: boolean;
  onClose: () => void;
  language: Language;
}

export const TerminalModal: React.FC<TerminalModalProps> = ({
  isOpen,
  onClose
}) => {

  const [inputCommand, setInputCommand] = useState('');
  const [history, setHistory] = useState<Array<{ cmd?: string; output: string }>>([
    {
      output: `DigiPanel Linux Kernel 6.8.0-45-generic (x86_64)
Connected to panel.digihost.id (103.189.201.42)
Type 'help' for available diagnostic commands.`
    }
  ]);

  if (!isOpen) return null;

  const handleRunCommand = (e: React.FormEvent) => {
    e.preventDefault();
    const cmd = inputCommand.trim();
    if (!cmd) return;

    let output = '';
    const lower = cmd.toLowerCase();

    if (lower === 'help') {
      output = `Available Commands:
  digipanel status  - Display platform daemons and socket health
  uptime            - Show server uptime and system load
  df -h             - Show disk storage allocation
  free -m           - Show RAM usage
  nginx -t          - Validate web server configuration
  certbot certs     - List managed Let's Encrypt certificates
  clear             - Clear terminal buffer`;
    } else if (lower === 'digipanel status') {
      output = `● digipanel-agent.service - DigiPanel Root Agent
     Loaded: loaded (/etc/systemd/system/digipanel-agent.service; enabled)
     Active: active (running) since Thu 2026-10-24 04:12:00 UTC
     Socket: /run/digipanel/agent.sock (0660 root:digipanel)
     Memory: 18.4M (limit: 128M)
     CGroup: /system.slice/digipanel-agent.service
             └─3108 /usr/local/bin/digipanel-agent daemon`;
    } else if (lower === 'uptime') {
      output = ` 13:30:12 up 42 days,  6:18,  1 user,  load average: 0.12, 0.08, 0.05`;
    } else if (lower === 'df -h') {
      output = `Filesystem      Size  Used Avail Use% Mounted on
/dev/nvme0n1p2  500G   28G  448G   6% /
tmpfs            16G     0   16G   0% /dev/shm
/dev/nvme0n1p1  512M  6.2M  506M   2% /boot/efi`;
    } else if (lower === 'free -m') {
      output = `               total        used        free      shared  buff/cache   available
Mem:           16384        1842        9812         124        4730       14418
Swap:           4096           0        4096`;
    } else if (lower === 'nginx -t') {
      output = `nginx: the configuration file /etc/nginx/nginx.conf syntax is ok
nginx: configuration file /etc/nginx/nginx.conf test is successful`;
    } else if (lower === 'certbot certs') {
      output = `Found the following certs:
  Certificate Name: digipanel.live
    Serial Number: 4a2b9e81...
    Key Type: ECDSA
    Domains: digipanel.live *.digipanel.live
    Expiry Date: 2026-12-28 10:14:00+00:00 (VALID: 60 days)
    Certificate Path: /etc/letsencrypt/live/digipanel.live/fullchain.pem`;
    } else if (lower === 'clear') {
      setHistory([]);
      setInputCommand('');
      return;
    } else {
      output = `bash: ${cmd}: command not recognized in restricted diagnostic shell. Type 'help'.`;
    }

    setHistory([...history, { cmd, output }]);
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
              placeholder="type 'help' or any command..."
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
