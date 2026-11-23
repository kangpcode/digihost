# DigiPanel Operator & Module Developer Guide

Platform: DigiPanel Unified Hosting & Native Billing Platform
Scope: Installation modes, service operations, RBAC, native billing, mTLS cluster & SaaS white-label, and module development

## 1. Deployment Modes

Run `installer/install.sh` on AlmaLinux/Rocky 9-10, Ubuntu 22.04/24.04, or Debian 12/13 as `root`:

| Mode | Capabilities Included | Typical Use |
| --- | --- | --- |
| `panel-personal` | Single-node panel, no billing, no reseller, restricted admin access | Personal servers |
| `panel-commercial` | Full multi-role panel (Admin, Reseller, User, Fleet), no billing | Hosting providers using external billing |
| `full-billing` | Full panel plus native billing engine, client portal, tickets, auto-provisioning | Integrated hosting operators |
| `billing-only` | Standalone billing engine and client portal linked to panel nodes | Dedicated billing hub |

Key flags:
- `--domain <fqdn>`: verify DNS points to host IP and provision ACME TLS certificate.
- `--panel-name <name>` and `--hosting-name <name>`: customize white-label branding at install time.
- `--webserver nginx|openlitespeed`: choose primary web server.
- `--ip <ipv4>`: override auto-detected public IP.
- `--dry-run`: validate prerequisites and plan steps without modifying the system.

## 2. Service Architecture & Security Defaults

- `digipanel-agent`: runs as `root`, listens strictly on `/run/digipanel/agent.sock` (`0660`, `root:digipanel`), executes only whitelisted parameterized commands with zero shell strings.
- `digipanel-core`: runs unprivileged (`digipanel`), serves REST/WebSocket APIs with default-deny router authentication across all HTTP verbs, Argon2id password hashing, TOTP 2FA, and SHA-256 hash-chained audit logs.
- Bootstrap administrator credentials: set `DIGIPANEL_ADMIN_PASSWORD` or read the generated `0600` credential file `<storage>/initial-admin-password` and rotate immediately after first login.
- Proxy & IP allowlist controls:
  - `DIGIPANEL_TRUSTED_PROXIES`: comma-separated proxy IPs or CIDRs allowed to supply `CF-Connecting-IP`, `X-Forwarded-For`, and `X-Real-IP`.
  - `DIGIPANEL_ADMIN_ALLOWLIST`: comma-separated IPs or CIDRs restricting `admin` and `staff` access.

## 3. Native Billing Operations (Tahap 8)

- All monetary amounts use integer minor units (`AmountCents`), never floating point.
- Every invoice, payment settlement, and refund writes balanced double-entry debit/credit records verified via `LedgerSummary()`.
- Supported signed webhook gateways (`POST /api/v1/billing/webhooks/{gateway}`):
  - `midtrans` (`signature_key` SHA-512)
  - `stripe` (`Stripe-Signature` HMAC-SHA256)
  - `xendit` (`X-Callback-Token` constant-time verification)
  - `paypal` (`X-Paypal-Transmission-Sig` HMAC-SHA256)
- Cron automation (`POST /api/v1/billing/automation/run`): generates renewal invoices, sends due reminders, applies late fees, auto-suspends overdue services, and auto-terminates expired services idempotently.
- Migration importer (`POST /api/v1/billing/import`): supports WHMCS/Blesta JSON bundles with `dryRun` mode.

## 4. Cluster Hub/Node mTLS & Multi-Tenant SaaS (Tahap 9)

- Connect edge/database nodes via `POST /api/v1/cluster/nodes/connect` using TLS 1.3 mTLS and mandatory SHA-256 `certFingerprint` pinning.
- Poll node fleet health via `POST /api/v1/cluster/nodes/poll`; unreachable nodes transition to `offline`.
- Live account migration (`POST /api/v1/cluster/migrate`): creates a `.tar.gz` archive, computes SHA-256 checksum, transfers over pinned mTLS, verifies checksum on target, and supports automatic failover (`allowFailover: true` or `targetNodeId: "auto"`) with full rollback on extraction failure.
- Multi-tenant white-label (`POST /api/v1/saas/tenants`, `PUT /api/v1/saas/tenants/branding`, `GET /api/v1/branding`): resolves panel name, hosting name, logo, colors, language, timezone, currency, and SMTP settings dynamically per `Host` header or `tenant` query parameter without code changes.
- Tenant suspension and `maxAccounts` SaaS quotas are enforced before account provisioning.

## 5. Module Development Contract

Each capability lives under `modules/<name>/` with:
- `module.json`: metadata, semantic version, dependencies, declared RBAC permissions, lifecycle hooks, and configuration schema.
- Explicit Go package interfaces consumed by `core/` without bypassing the agent security boundary or default-deny router.
- Automated unit and integration tests covering happy paths, invalid inputs, and security boundaries.
