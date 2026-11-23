# DigiPanel

DigiPanel is a unified, self-hosted hosting control panel and native billing platform for Linux. It provides a multi-role control panel (Admin, Reseller, User), a self-contained billing engine with double-entry accounting, an application installer similar to Softaculous, a single-command installer with multiple deployment modes, and optional multi-tenant SaaS operation.

No third-party billing product is required. All capabilities are implemented as versioned modules with explicit interfaces, migrations, routes, and UI.

## Features

- Multi-role panels: Admin (server and fleet management), Reseller (accounts, quotas, branding), User (self-service hosting).
- Hosting core: accounts, plans, domains, PHP-FPM per account, databases, files, SSL, DNS, email, backups, firewall, metrics.
- Runtime management: multiple PHP versions per domain, Node.js per application, plus Python, Ruby, and Go application templates.
- Native billing: catalog, clients, orders, invoices, gateway plugins, domain registrar plugins, support tickets, reports, and double-entry ledger.
- Application marketplace: declarative signed manifests, one-click install, clone, stage, update, and WordPress toolkit.
- Cluster and SaaS: hub/node with mTLS, account migration, DNS clustering, multi-tenant isolation, white label.
- Security by default: privileged agent over a Unix socket, default-deny router, RBAC, 2FA, audit log with hash chaining, cgroups v2 isolation.

## Repository Layout

```
agent/       Root-privileged daemon executing whitelisted system commands
core/        Unprivileged API daemon, auth, RBAC, ledger, migrations, OpenAPI
modules/     Pluggable capability modules (hosting, dns, mail, db, files, ssl,
             backup, firewall, runtimes, softinstaller, billing, ...)
web/         React 19 + TypeScript + Vite frontend
installer/   Single-command installer and pre-flight checks
tests/       Unit, integration, installer, and security tests
docs/        Architecture, threat model, and specifications
```

## Quick Start

Install on a supported clean Linux host as root:

```
curl -fsSL https://example.com/install.sh | bash
```

Or run the bundled installer directly:

```
sudo bash installer/install.sh --mode panel-commercial --non-interactive
```

Supported operating systems: AlmaLinux/Rocky 9 and 10, Ubuntu 22.04 and 24.04, Debian 12 and 13. Architectures: x86_64 and aarch64.

Installer modes:

- `panel-personal`: lightweight single-node panel, no billing, no reseller.
- `panel-commercial`: full panel (admin, reseller, user, fleet), no billing.
- `full-billing`: panel plus native billing and client portal.
- `billing-only`: standalone billing engine and client portal.

Common flags: `--domain`, `--panel-name`, `--hosting-name`, `--admin-email`, `--ip`, `--webserver`, `--php-version`, `--dry-run`.

After installation, manage the platform with:

```
digipanel status
digipanel update stable
digipanel uninstall --keep-data
```

## Development

Backend (Go 1.27):

```
go mod tidy
go vet ./...
go test -race ./...
CGO_ENABLED=0 go build -o bin/digipanel-core ./core
CGO_ENABLED=0 go build -o bin/digipanel-agent ./agent
```

Frontend (Node 22):

```
cd web
npm ci
npm run build
npm run lint
```

Installer tests:

```
chmod +x installer/install.sh tests/installer_test.sh
./tests/installer_test.sh
```

## Security

Security policy and vulnerability reporting are documented in `SECURITY.md`. Architecture and threat model are in `docs/architecture.md` and `docs/threat-model.md`.

Key controls enforced across the codebase:

- No shell string execution; system commands use parameterized argument arrays only.
- Default-deny authentication on all HTTP verbs.
- Single JSON serialization standard; no unsafe deserialization.
- Trusted-proxy validation for client identity headers.
- Row-level security for multi-tenant data.
- Immutable, hash-chained audit log.

## Documentation

- `docs/architecture.md` - component layout and communication pathways.
- `docs/threat-model.md` - STRIDE analysis and mandatory security controls.
- `core/openapi.yaml` - API contract.
- `core/schema.sql` - database schema.

## License

See the repository license file for terms. All third-party dependencies must have licenses compatible with commercial distribution; an SBOM is generated in CI.