# DigiPanel Threat Model (STRIDE)

Platform: DigiPanel Hosting Platform
Scope: Core daemon, agent, web panels, multi-tenant boundaries, billing subsystem
Version: 1.0.0

## 1. System Overview and Trust Boundaries

DigiPanel operates with three primary trust tiers:

1. **Privileged Core Layer (`digipanel-agent`)**:
   - Runs with root privileges to manage system services (systemd, nftables, useradd, quota, powerdns, nginx/openlitespeed, php-fpm).
   - Only listens on a restricted Unix domain socket (`/run/digipanel/agent.sock`) with strict UID/GID permissions (root:digipanel 0660).
   - Zero shell command construction from arbitrary inputs. Every command is strictly parameterized, schema-validated, and checked against a hardcoded whitelist.

2. **Application Core Layer (`digipanel-core`)**:
   - Runs as an unprivileged service user (`digipanel`).
   - Exposes authenticated REST and WebSocket APIs over HTTPS.
   - Enforces Role-Based Access Control (RBAC), multi-tenant tenancy checks, rate limiting, and 2FA (TOTP / WebAuthn).
   - Holds encrypted credentials and connects to PostgreSQL using parameterized queries.

3. **Client / Web UI Layer (`web/`)**:
   - Modern glass architecture built on React + TypeScript.
   - Strictly enforces Content Security Policy (CSP), anti-CSRF token synchronization, HttpOnly secure cookies, and input sanitation.
   - Three distinct persona views: Admin (Server Fleet Management), Reseller (Account Groups & Resource Quotas), and User (cPanel/Adminbolt style Self-Service).

---

## 2. STRIDE Threat Analysis

| Category | Threat Description | Mitigation Strategy |
| :--- | :--- | :--- |
| **Spoofing** | Attacker attempts to impersonate an Admin or User via stolen session tokens or spoofed cluster node requests. | - Argon2id password hashing with per-user salt.<br>- Mandatory TOTP / WebAuthn support.<br>- Mutual TLS (mTLS) with pinned client certificates for all Cluster Hub-to-Node communications.<br>- Short-lived JWTs with automatic rotation and revocation lists. |
| **Tampering** | Malicious tenant attempts path traversal or symlink attacks to overwrite system files or another user's vhost. | - Strict path canonicalization before file operations.<br>- Linux namespaces, chroot / CageFS equivalent jail per user account.<br>- Read-only system mounts in tenant jail.<br>- Immutable append-only audit log with SHA-256 hash chaining. |
| **Repudiation** | An operator or user modifies DNS records or deletes backups and denies the action. | - Cryptographically chained audit log recording user ID, IP address, timestamp, action, and payload hash.<br>- Remote syslog forwarding support. |
| **Information Disclosure** | Leakage of private database credentials, customer PII, or cross-tenant data. | - Database credentials encrypted using AES-256-GCM with master key stored in KMS / secure environment file.<br>- PostgreSQL Row-Level Security (RLS) for multi-tenant billing tables.<br>- Output masking for sensitive tokens in API responses. |
| **Denial of Service** | Resource exhaustion via abusive PHP/Node script execution, fork bombs, or API hammering. | - cgroups v2 memory, CPU, and pids limits per tenant account.<br>- nftables rate limiting and Fail2ban integration.<br>- API rate limiter per IP and per API key with token bucket algorithm. |
| **Elevation of Privilege** | Unprivileged web script or compromised PHP-FPM pool attempts to execute root commands. | - Strict separation: web scripts run as dedicated POSIX users (e.g., `usr_client1`).<br>- Systemd socket permissions prevent direct tenant access to `agent.sock`.<br>- Whitelisted agent actions with zero arbitrary shell execution. |

---

## 3. Cryptographic and Secrets Policy

- **Symmetric Encryption**: AES-256-GCM for sensitive fields at rest (e.g., API keys, payment tokens).
- **Password Hashing**: Argon2id (m=65536, t=3, p=4).
- **Inter-node Transit**: TLS 1.3 with forward secrecy; mTLS for cluster management.
- **Zero Secrets in Repository**: Environment-based secret provisioning with startup validation.
