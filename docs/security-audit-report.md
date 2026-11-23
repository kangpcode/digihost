# DigiPanel Tahap 8, 9, and 10 Verification & Security Audit Report

Date: 2026-11-23
Scope: Verification of Tahap 8 (Native Billing), Tahap 9 (Cluster mTLS & Multi-Tenant SaaS), and execution of Tahap 10 (Security Audit, Competitor CVE Regression Suite, Benchmarks, Documentation, and Signed Release Packaging).

## 1. Audit Findings & Remediations

| Severity | Finding | Remediation | Verification |
| --- | --- | --- | --- |
| High | `POST /api/v1/terminal/exec` and `POST /api/v1/tasks/enqueue` had no entry in `routeRequiredPermissions`, allowing authenticated `RoleStaff` or ungranted tokens past route RBAC; terminal audit logged hardcoded `"admin"`. | Added `"terminal.exec"` (`core/rbac/rbac.go`) and `"system.modules.manage"` mappings in `core/server.go`; updated terminal audit logging to `state.actorUsername(r)` and `clientIP(...)`. | `TestCompetitorCVERegressionSuite` (403 for `RoleStaff`) |
| High | Legacy `POST /api/v1/dns/records` accepted arbitrary record type strings (e.g. `EXEC;id`) without closed-enum validation. | Enforced `dns.ValidateZoneName` and `dns.ValidateRecord` closed enum (`A`, `AAAA`, `CNAME`, `MX`, `TXT`, `NS`, `SOA`, `SRV`, `CAA`, `PTR`). | `TestCompetitorCVERegressionSuite` (400 rejected) |
| Medium | `clientIP()` validated `X-Forwarded-For` and `X-Real-IP` behind trusted proxies, but did not inspect `CF-Connecting-IP`. | Added `CF-Connecting-IP` handling strictly gated by `isTrustedProxy(peer, trustedProxies)`. | `TestCompetitorCVERegressionSuite` |
| Medium | `ClusterManager` lacked active mTLS polling to transition dead nodes `offline` and automatic failover selection during account migration. | Added `PollNodes`, `SelectHealthyNode`, `POST /api/v1/cluster/nodes/poll`, and `allowFailover` fallback in `core/cluster.go`. | `TestClusterNodeFailover` |
| Medium | `TenantManager` lacked API-level provisioning gate for suspended SaaS tenants and `MaxAccounts` quotas. | Added `CanProvisionAccount` & `FilterAccountsByTenant` in `core/tenants.go` and enforced in `POST /api/v1/hosting/accounts`. | `TestCrossTenantIsolationAndQuotaEnforcement` |

## 2. Tahap 8 & 9 Verification Evidence

- **Tahap 8 (Native Billing)**:
  - `TestBillingEndToEndProvisioningWebhooksAndLedger` (`modules/billing`): Verified integer-only cents math, coupons, tax, Midtrans SHA-512 & Stripe HMAC-SHA256 webhook verification, idempotent replay, real `hosting.Manager` auto-provisioning, prorata upgrade, partial refund, balanced double-entry ledger (`debits == credits`), PDF generation (`%PDF-1.4`..`%%EOF`), cron automation (late fee + auto-suspend), fraud scoring, GDPR export/anonymize, and WHMCS migration import.
  - `TestNativeBillingRESTLifecycle` (`core`): Verified REST product/client/order flow, Midtrans & Xendit signed webhooks, 401 rejection on invalid Xendit token, PDF download, reports, and REST refund keeping the double-entry ledger balanced.
- **Tahap 9 (Cluster mTLS & Multi-Tenant SaaS)**:
  - `TestClusterMTLSAndLiveMigration` (`core`): Verified TLS 1.3 mTLS with SHA-256 certificate fingerprint pinning, rejection of unpinned/forged certificates, DNS cluster sync over mTLS, and live tar.gz account migration with SHA-256 integrity check.
  - `TestClusterNodeFailover` (`core`): Verified mTLS node polling marks stopped primary edge node `offline` and migrates accounts cleanly to secondary healthy edge node.
  - `TestMultiTenantSaaSWhiteLabelAndBillingAPI` & `TestCrossTenantIsolationAndQuotaEnforcement` (`core`): Verified dynamic Host-header white-label branding, strict tenant usage isolation, suspended-tenant provisioning refusal (403), and `MaxAccounts` quota enforcement.

## 3. Tahap 10 Security, Benchmark, and Release Evidence

- **All Registered Routes Default-Deny Audit**: `TestAllRegisteredRoutesRequireAuthOrExplicitPublic` enumerated every `mux.HandleFunc` route in `core/server.go` across `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `HEAD` and confirmed `401` on all non-public routes and unsigned billing webhooks.
- **Zero Shell String Scan**: `TestZeroShellStringExecutionInProductionGoCode` and CI regex scan passed with zero violations across `agent/`, `core/`, and `modules/`.
- **Zero-Emoji Policy**: Python Unicode scan passed with 0 violations.
- **API Latency & Throughput (`docs/benchmark-results.txt`)**:
  - 200-concurrent requests to `GET /api/v1/hosting/accounts`: `p50=11.712us`, `p95=141.716us`, `max=229.22us` (well under the 300ms p95 target; with `-race`: `p95=2.18ms`).
  - `BenchmarkAuthenticatedAccountsAPI`: `3317 ns/op`, `2636 B/op`, `29 allocs/op`.
- **Signed Release Packaging (`scripts/build_release.sh`)**:
  - Static `CGO_ENABLED=0` binaries built and verified: `digipanel-core`, `digipanel-agent`, `sbom.json` (99 components), `SHA256SUMS`, and ECDSA P-256 signature verification (`SHA256SUMS.sig`).

## 4. Environment Limitations Observed (Unverified in Sandbox)

- `TestPostgresMigrationsAndRLSIsolation` (`core/dbstore`) was **skipped** because PostgreSQL binaries (`initdb`, `pg_ctl`) are not present in this sandbox environment.
- `modules/runtimes`, `modules/softinstaller`, and frontend `web/` Vite build could not execute `/usr/bin/node` / `npm` inside the sandbox (`Operation not permitted` / `execvp failed`).
