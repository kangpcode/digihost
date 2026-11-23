# DigiPanel Disaster Recovery & Incident Runbook

Platform: DigiPanel Unified Hosting & Native Billing Platform
Scope: Backup verification, database & state recovery, cluster failover, atomic update rollback, and 24-hour emergency patch pipeline

## 1. Recovery Objectives

- **RPO (Recovery Point Objective)**: Bounded by backup schedule interval (recommended hourly incremental for billing/core DB, daily full encrypted snapshots for hosting accounts).
- **RTO (Recovery Time Objective)**: < 15 minutes for single-node restore or cluster node failover.

## 2. Backup Encryption & Verification

- Backups are encrypted and stored with SHA-256 manifest checksums (`modules/backup` and `/api/v1/backups`).
- Automated restore verification (`POST /api/v1/backups/verify`) validates archive integrity and checksums without overwriting live production directories.
- Store backup passphrases and master encryption keys (`AES-256-GCM` secret store) in an external KMS or restricted `0600` environment configuration outside the backup archive itself.

## 3. Database & Billing Ledger Recovery

1. Stop `digipanel-core` before restoring PostgreSQL snapshots to prevent split writes:
   `systemctl stop digipanel-core`
2. Restore the PostgreSQL database dump or basebackup and verify schema migrations (`core/dbstore/migrations/`).
3. Verify double-entry accounting invariants and audit chain integrity on startup:
   - Confirm `LedgerSummary()` reports `balanced = true` (`totalDebits == totalCredits`).
   - Confirm `AuditLogger.VerifyChain()` succeeds with zero hash-chain breaks.
4. Start `digipanel-core` and verify `GET /api/v1/health` returns `200 OK`.

## 4. Cluster Node Outage & Failover Procedure

1. Run `POST /api/v1/cluster/nodes/poll` (or wait for scheduled fleet polling) to mark unreachable nodes `offline`.
2. Migrate pending or recoverable accounts using `POST /api/v1/cluster/migrate` with `"allowFailover": true` (or `"targetNodeId": "auto"`), which selects an `online` node verified by pinned TLS 1.3 mTLS and SHA-256 archive checksums.
3. Re-synchronize DNS zones across healthy nodes with `POST /api/v1/cluster/dns/sync`.
4. Once the failed node is repaired, re-issue or re-pin its certificate fingerprint and reconnect via `POST /api/v1/cluster/nodes/connect`.

## 5. Atomic Update & Rollback

1. Before upgrading `digipanel-core` or `digipanel-agent`, archive current binaries, configuration, and database snapshot.
2. Verify the new release package signature and checksums (`SHA256SUMS` and `SHA256SUMS.sig` against the trusted public release key).
3. Perform rolling upgrade node-by-node starting with one edge node, checking `/api/v1/health` and `/api/v1/cluster/node/health`.
4. If health checks fail, restore the prior static binary and database snapshot and restart `systemd` units.

## 6. 24-Hour Emergency Security Patch Pipeline

1. Triage incoming vulnerability report per `SECURITY.md` and write a failing regression test in `core/security_audit_test.go` or `agent/agent_test.go`.
2. Apply minimal patch, run `go vet ./...`, `go test -race ./...`, and `./scripts/build_release.sh` to produce signed release artifacts with updated `docs/sbom.json`.
3. Publish security advisory and release notes stating exact CVE/class mitigated and verification checksums within 24 hours of confirmation.
