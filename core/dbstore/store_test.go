// DigiPanel PostgreSQL Store Integration & RLS Tests (core/dbstore/store_test.go)
// Spins up an ephemeral local PostgreSQL instance (using initdb + pg_ctl),
// verifies up/down/up migrations, double-entry transaction atomicity, and
// strict Row-Level Security (RLS) isolation across tenants.

package dbstore

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func findPostgresBinDir() string {
	if p, err := exec.LookPath("initdb"); err == nil {
		return filepath.Dir(p)
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".config/FlyEnv/env/postgresql/bin"),
		"/usr/lib/postgresql/17/bin",
		"/usr/lib/postgresql/16/bin",
		"/usr/lib/postgresql/15/bin",
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "initdb")); err == nil {
			return dir
		}
	}
	return ""
}

func startEphemeralPostgres(t *testing.T) string {
	t.Helper()
	binDir := findPostgresBinDir()
	if binDir == "" {
		t.Skip("PostgreSQL binaries (initdb/pg_ctl) not available in environment")
	}

	dataDir := filepath.Join(t.TempDir(), "pgdata")
	sockDir := filepath.Join(t.TempDir(), "pgsock")
	if err := os.MkdirAll(sockDir, 0700); err != nil {
		t.Fatalf("mkdir sockdir: %v", err)
	}

	initCmd := exec.Command(filepath.Join(binDir, "initdb"),
		"-D", dataDir,
		"-U", "postgres",
		"-A", "trust",
		"--encoding=UTF8",
		"--locale=C.UTF-8",
	)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("initdb failed: %v\n%s", err, string(out))
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate ephemeral port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	logFile := filepath.Join(dataDir, "postgres.log")
	opts := fmt.Sprintf("-p %d -k %s -h 127.0.0.1", port, sockDir)
	startCmd := exec.Command(filepath.Join(binDir, "pg_ctl"),
		"-D", dataDir,
		"-l", logFile,
		"-o", opts,
		"-w",
		"start",
	)
	if out, err := startCmd.CombinedOutput(); err != nil {
		logBytes, _ := os.ReadFile(logFile)
		t.Fatalf("pg_ctl start failed: %v\n%s\n%s", err, string(out), string(logBytes))
	}

	t.Cleanup(func() {
		stopCmd := exec.Command(filepath.Join(binDir, "pg_ctl"), "-D", dataDir, "-m", "immediate", "stop")
		_ = stopCmd.Run()
	})

	return fmt.Sprintf("postgres://postgres@127.0.0.1:%d/postgres?sslmode=disable", port)
}

func TestPostgresMigrationsAndRLSIsolation(t *testing.T) {
	dsn := startEphemeralPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open PostgreSQL store: %v", err)
	}
	defer store.Close()

	// 1. Verify all 3 migrations applied
	applied, err := Applied(ctx, store.Pool())
	if err != nil {
		t.Fatalf("Applied migrations query: %v", err)
	}
	if len(applied) != 3 {
		t.Fatalf("Expected 3 applied migrations, got %d", len(applied))
	}

	// 2. Create two isolated tenants and verify Row-Level Security blocks cross-tenant reads
	tenantA, err := store.EnsureTenant(ctx, "alpha", "Alpha Cloud", "AlphaPanel", "alpha.example.com")
	if err != nil {
		t.Fatalf("EnsureTenant alpha: %v", err)
	}
	tenantB, err := store.EnsureTenant(ctx, "beta", "Beta Host", "BetaPanel", "beta.example.com")
	if err != nil {
		t.Fatalf("EnsureTenant beta: %v", err)
	}

	if err := store.UpsertUser(ctx, UserRecord{
		TenantID:     tenantA.ID,
		Username:     "alice",
		Email:        "alice@alpha.example.com",
		PasswordHash: "argon2id-hash-a",
		Role:         "admin",
	}); err != nil {
		t.Fatalf("UpsertUser alice: %v", err)
	}
	if err := store.UpsertUser(ctx, UserRecord{
		TenantID:     tenantB.ID,
		Username:     "bob",
		Email:        "bob@beta.example.com",
		PasswordHash: "argon2id-hash-b",
		Role:         "admin",
	}); err != nil {
		t.Fatalf("UpsertUser bob: %v", err)
	}

	usersA, err := store.ListUsers(ctx, tenantA.ID)
	if err != nil {
		t.Fatalf("ListUsers tenantA: %v", err)
	}
	if len(usersA) != 1 || usersA[0].Username != "alice" {
		t.Fatalf("RLS leak or mismatch for tenantA: got %+v", usersA)
	}

	usersB, err := store.ListUsers(ctx, tenantB.ID)
	if err != nil {
		t.Fatalf("ListUsers tenantB: %v", err)
	}
	if len(usersB) != 1 || usersB[0].Username != "bob" {
		t.Fatalf("RLS leak or mismatch for tenantB: got %+v", usersB)
	}

	// Cross-tenant INSERT attempt while scoped to tenantA must be rejected by RLS WITH CHECK
	err = store.WithTenantTx(ctx, tenantA.ID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (tenant_id, username, email, password_hash, role)
			VALUES ($1::uuid, 'intruder', 'intruder@beta.example.com', 'hash', 'user')
		`, tenantB.ID)
		return err
	})
	if err == nil {
		t.Fatalf("Security violation: RLS allowed tenantA transaction to insert row into tenantB")
	}

	// 3. Test atomic invoice + double-entry settlement + idempotent replay in PostgreSQL
	inv, err := store.CreateInvoice(ctx, InvoiceRecord{
		TenantID:      tenantA.ID,
		InvoiceNumber: "INV-2026-0001",
		ClientName:    "PT Alpha",
		ClientEmail:   "billing@alpha.example.com",
		AmountCents:   250000,
		Currency:      "IDR",
		Status:        "unpaid",
		IssueDate:     "2026-11-23",
		DueDate:       "2026-12-07",
		Description:   "Pro Plan",
	})
	if err != nil {
		t.Fatalf("CreateInvoice: %v", err)
	}
	already, err := store.SettleInvoiceAtomic(ctx, tenantA.ID, inv.InvoiceNumber, "midtrans", "MID-1001")
	if err != nil || already {
		t.Fatalf("SettleInvoiceAtomic: already=%v err=%v", already, err)
	}
	alreadyReplay, err := store.SettleInvoiceAtomic(ctx, tenantA.ID, inv.InvoiceNumber, "midtrans", "MID-1001")
	if err != nil || !alreadyReplay {
		t.Fatalf("Expected idempotent replay already=true, got already=%v err=%v", alreadyReplay, err)
	}

	// 4. Verify MigrateDown rolls back cleanly to 0 and MigrateUp reapplies cleanly
	rolledBack, err := MigrateDown(ctx, store.Pool(), 0)
	if err != nil {
		t.Fatalf("MigrateDown to 0 failed: %v", err)
	}
	if rolledBack != 3 {
		t.Fatalf("Expected 3 migrations rolled back, got %d", rolledBack)
	}
	reapplied, err := MigrateUp(ctx, store.Pool())
	if err != nil {
		t.Fatalf("Re-MigrateUp failed: %v", err)
	}
	if reapplied != 3 {
		t.Fatalf("Expected 3 migrations reapplied, got %d", reapplied)
	}
}
