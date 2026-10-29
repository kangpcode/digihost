# DigiPanel Threat Model (STRIDE) and Security Architecture

Platform: DigiPanel Unified Hosting Platform
Scope: Core daemon, agent, web panels, multi-tenant boundaries, billing subsystem, and cluster fleet
Version: 1.0.0 (Tahap 0 Fondasi)

## 1. System Overview and Trust Boundaries

DigiPanel operates with three primary trust tiers:

1. **Privileged Core Layer (`digipanel-agent`)**:
   - Runs with root privileges to manage system services (systemd, nftables, useradd, quota, powerdns, nginx/openlitespeed, php-fpm).
   - Only listens on a restricted Unix domain socket (`/run/digipanel/agent.sock`) with strict UID/GID permissions (root:digipanel 0660).
   - Zero shell command construction from arbitrary inputs. Every command is strictly parameterized, schema-validated, and checked against a hardcoded whitelist table.
   - No shell interpreters (/bin/sh, /bin/bash) are ever invoked.

2. **Application Core Layer (`digipanel-core`)**:
   - Runs as an unprivileged service user (`digipanel`).
   - Exposes authenticated REST and WebSocket APIs over HTTPS.
   - Enforces default-deny authentication on router level for all HTTP methods (GET, POST, PUT, DELETE, OPTIONS, HEAD).
   - Enforces Role-Based Access Control (RBAC), multi-tenant tenancy checks, rate limiting, and 2FA (TOTP / WebAuthn).
   - Holds encrypted credentials and connects to PostgreSQL using parameterized queries and Row-Level Security (RLS).

3. **Client / Web UI Layer (`web/`)**:
   - Modern glass architecture built on React + TypeScript.
   - Strictly enforces Content Security Policy (CSP), anti-CSRF token synchronization, HttpOnly secure cookies, and input sanitation.
   - Three distinct persona views: Admin (Server Fleet Management), Reseller (Account Groups & Resource Quotas), and User (Self-Service).

---

## 2. STRIDE Threat Analysis

| Category | Threat Description | Attack Vector | Mitigation Strategy |
| :--- | :--- | :--- | :--- |
| **Spoofing** | Attacker attempts to impersonate an Admin or User via stolen session tokens or spoofed cluster node requests. | JWT manipulation, spoofed `X-Forwarded-For` or `CF-Connecting-IP`, forged node certificates. | - Argon2id password hashing with per-user salt.<br>- Mandatory TOTP / WebAuthn support.<br>- Mutual TLS (mTLS) with pinned client certificates for all Cluster Hub-to-Node communications.<br>- Short-lived JWTs with automatic rotation and revocation lists.<br>- Client identity headers only trusted from explicit trusted-proxy whitelist. |
| **Tampering** | Malicious tenant attempts path traversal or symlink attacks to overwrite system files or another user's vhost. | Path traversal (`../../`), symlink exploits, DNS/cron metacharacter injection. | - Strict path canonicalization and ownership verification before file operations.<br>- Linux namespaces, chroot / CageFS equivalent jail per user account.<br>- Read-only system mounts in tenant jail.<br>- Closed enum schema validation for DNS and cron tasks.<br>- Immutable append-only audit log with SHA-256 hash chaining. |
| **Repudiation** | An operator or user modifies DNS records, deletes backups, or alters invoices and denies the action. | Untracked deletion, manipulation of audit entries or timestamps. | - Cryptographically chained audit log recording actor user ID, tenant ID, IP address, timestamp, action, and payload hash.<br>- Double-entry accounting ledger where transactions are balanced and immutable. |
| **Information Disclosure** | Leakage of private database credentials, customer PII, or cross-tenant data. | Cross-tenant database queries, world-readable config files, unauthenticated endpoints. | - Database credentials encrypted using AES-256-GCM with master key stored in KMS / secure environment file.<br>- PostgreSQL Row-Level Security (RLS) for multi-tenant billing and hosting tables.<br>- Router default-deny on all routes; zero secrets in log output. |
| **Denial of Service** | Resource exhaustion via abusive PHP/Node script execution, fork bombs, or API hammering. | HTTP flood attacks, runaway PHP-FPM / Node scripts, unbounded disk consumption. | - cgroups v2 memory, CPU, I/O, and pids limits per tenant account.<br>- nftables rate limiting and Fail2ban integration.<br>- API rate limiter per IP and per API key with token bucket algorithm. |
| **Elevation of Privilege** | Unprivileged web script or compromised PHP-FPM pool attempts to execute root commands. | Shell injection in system calls, sudo bypass, insecure session deserialization. | - Strict separation: web scripts run as dedicated POSIX users.<br>- Systemd socket permissions prevent direct tenant access to `agent.sock`.<br>- Whitelisted agent actions with zero arbitrary shell execution; array arguments only.<br>- Single secure JSON serialization library with schema validation. |

---

## 3. Mandatory Competitor Lesson Controls (11 Controls)

1. **Zero Shell Execution from Strings**: All system commands use string array parameters, no shell interpreters, and strict agent whitelist dispatch.
2. **Default-Deny Router Authentication**: Router-level enforcement across all HTTP verbs (GET, POST, PUT, DELETE, OPTIONS, HEAD).
3. **Single Session and Serialization Standard**: Uniform JSON serialization with strict schema; zero unsafe object deserialization.
4. **Sandboxed Web Terminal**: Non-root account user isolation within dedicated namespaces, timeout limits, and audit capture.
5. **Trusted Proxy Network Identity**: Client IP headers (X-Forwarded-For, CF-Connecting-IP) accepted only from verified proxy CIDRs.
6. **Closed Enum Validation for DNS & Cron**: All DNS record types and cron intervals strictly bounded by schema enums.
7. **Cryptographically Signed Webhooks**: Inbound payment and telemetry callbacks require HMAC signatures and replay prevention.
8. **Personal Mode Port Protection**: On personal installation mode, admin panel binds to non-standard port with IP restriction.
9. **Unprivileged Main Daemon**: Core daemon runs without root privileges under AppArmor/SELinux profiles.
10. **24-Hour Emergency Patch Pipeline**: Verified patch workflow capable of hotfix delivery within 24 hours with automated health-check rollbacks.
11. **Competitor CVE Regression Test Suite**: Automated tests ensuring resilience against known vulnerability patterns from competing platforms.
