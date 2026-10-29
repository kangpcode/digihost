# DigiPanel Architecture Specification

Platform: DigiPanel Unified Hosting Platform
Version: 1.0.0 (Tahap 0 Fondasi)

## 1. Architectural Principles

DigiPanel is designed with the following core principles:
1. **Lightweight & Privileged-Separated Core**: The unprivileged core server (`digipanel-core`) handles business logic, database queries, and client communication. The root agent daemon (`digipanel-agent`) executes system commands strictly over a local Unix domain socket.
2. **Zero Shell String Execution**: System calls are parameterized as string arrays without invoking `/bin/sh` or `/bin/bash`, preventing shell injection vulnerabilities.
3. **Pluggable Module Subsystem**: Every capability (hosting, dns, mail, db, files, ssl, backup, runtimes, firewall, softinstaller, billing) is self-contained with a `module.json` manifest, migration files, and declarative lifecycle hooks.
4. **Native Double-Entry Billing**: Billing operations are recorded in a dual-entry general ledger where debits equal credits and amounts are stored strictly in the smallest currency unit (integer cents).
5. **Multi-Tenant SaaS Readiness**: Multi-tenant database schema uses PostgreSQL Row-Level Security (RLS) to prevent cross-tenant data leaks.
6. **Zero Emojis & Zero Placeholders**: Production code and documentation strictly prohibit emojis, mock data, and unresolved stubs.

---

## 2. Component Layout

```
digihost/
├── agent/              # Root daemon: system execution over Unix domain socket
│   ├── main.go
│   └── agent_test.go
├── core/               # Application API daemon: authentication, RBAC, billing, ledger
│   ├── server.go
│   ├── schema.sql
│   ├── openapi.yaml
│   ├── auth_test.go
│   ├── ledger_test.go
│   └── audit_test.go
├── modules/            # Pluggable capability modules
│   ├── hosting/        # Account lifecycle, vhost, php-fpm, cgroups v2
│   ├── dns/            # PowerDNS zone management, DNSSEC
│   ├── billing/        # Native double-entry billing and automated provisioning
│   ├── mail/           # Postfix, Dovecot, Rspamd
│   ├── db/             # MariaDB, MySQL, PostgreSQL
│   ├── files/          # Web file manager, SFTP
│   ├── ssl/            # Let's Encrypt / ACME HTTP-01 and DNS-01
│   ├── backup/         # Encrypted incremental backups
│   ├── firewall/       # nftables and fail2ban integration
│   ├── runtimes/       # Node.js, Python, Go, PHP selector and process supervisor
│   └── softinstaller/  # One-click application catalog installer
├── web/                # React 19 + TypeScript + Vite frontend
│   ├── src/
│   │   ├── components/ # Glass UI components
│   │   ├── types/      # Shared interfaces
│   │   └── i18n/       # Indonesian and English localizations
├── installer/          # Single-line installation scripts and pre-flight validation
├── docs/               # System documentation, threat models, and specifications
└── .github/workflows/  # Continuous integration and compliance pipelines
```

---

## 3. Communication Pathways

1. **Client to Core API**:
   - Protocol: HTTPS / TLS 1.3
   - Port: Default 8080 (behind reverse proxy)
   - Authentication: Scoped JWT tokens with Argon2id password verification and TOTP 2FA.
   - Router Policy: Default-deny across all HTTP verbs.

2. **Core API to Root Agent**:
   - Protocol: Unix Domain Socket (`/run/digipanel/agent.sock`)
   - Permissions: `0660`, user `root`, group `digipanel`
   - Data Format: Strict JSON request/response schema.
   - Dispatch: Whitelist actions only.

3. **Core API to Database**:
   - Protocol: PostgreSQL native wire protocol with SSL / Unix socket.
   - Security: Parameterized queries, schema migrations, and Row-Level Security.

4. **Cluster Hub to Edge Nodes**:
   - Protocol: Mutual TLS (mTLS) with pinned client certificates.
   - Transport: gRPC or HTTPS with certificate revocation verification.
