// DigiPanel database, backup, and firewall API tests.
// The managers are wired to recording transports so the HTTP contract, the
// account scoping, the package limits, and the scheduling behaviour are all
// asserted without a live database server or root nftables access.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"digipanel/core/rbac"
	"digipanel/modules/backup"
	"digipanel/modules/db"
	"digipanel/modules/firewall"
	"digipanel/modules/hosting"
)

// fakeDBExecutor keeps an in-memory catalog so the list and limit paths behave
// like a live server, while every statement is still recorded for inspection.
type fakeDBExecutor struct {
	mu        sync.Mutex
	scripts   []string
	databases []string
}

var (
	createDatabasePattern = regexp.MustCompile("CREATE DATABASE IF NOT EXISTS `([a-z0-9_]+)`")
	dropDatabasePattern   = regexp.MustCompile("DROP DATABASE IF EXISTS `([a-z0-9_]+)`")
)

func (f *fakeDBExecutor) Run(ctx context.Context, args []string, stdin string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scripts = append(f.scripts, stdin)

	if match := createDatabasePattern.FindStringSubmatch(stdin); match != nil {
		for _, existing := range f.databases {
			if existing == match[1] {
				return "", nil
			}
		}
		f.databases = append(f.databases, match[1])
		return "", nil
	}
	if match := dropDatabasePattern.FindStringSubmatch(stdin); match != nil {
		remaining := make([]string, 0, len(f.databases))
		for _, existing := range f.databases {
			if existing != match[1] {
				remaining = append(remaining, existing)
			}
		}
		f.databases = remaining
		return "", nil
	}
	if strings.Contains(stdin, "information_schema.SCHEMATA") {
		var builder strings.Builder
		for _, name := range f.databases {
			builder.WriteString(name + "\tutf8mb4\tutf8mb4_unicode_ci\t1\t1024\n")
		}
		return builder.String(), nil
	}
	return "", nil
}

type fakeFirewallRunner struct {
	applied [][]firewall.Rule
}

func (f *fakeFirewallRunner) Apply(ctx context.Context, rules []firewall.Rule) error {
	copied := make([]firewall.Rule, len(rules))
	copy(copied, rules)
	f.applied = append(f.applied, copied)
	return nil
}

func (f *fakeFirewallRunner) Inspect(ctx context.Context) (string, error) {
	return "table inet digipanel { }", nil
}

// prepareServices wires real managers with recording transports and returns the
// seeded account used by the scoping tests.
func prepareServices(t *testing.T) (*ServerState, http.Handler, string) {
	t.Helper()
	state, handler := setupIntegratedTestServer()

	executor := &fakeDBExecutor{}
	state.DBManager = db.NewManagerWithExecutor(db.Options{
		AdminUser:     "root",
		AdminPassword: "test-admin-password",
	}, executor)

	state.Backups = backup.NewEngine(backup.Options{
		Root:       filepath.Join(state.StorageDir, "backups"),
		Passphrase: "test-backup-passphrase",
		HomeBase:   filepath.Join(state.StorageDir, "vhosts"),
	})
	state.Firewall = firewall.NewManager(filepath.Join(state.StorageDir, "firewall"), &fakeFirewallRunner{})

	if _, err := state.HostingManager.CreatePackage(hosting.AccountPackage{
		Name:                "serviceplan",
		DiskLimitBytes:      1 << 30,
		BandwidthLimitBytes: 1 << 40,
		InodesLimit:         100000,
		MaxDomains:          5,
		MaxDatabases:        1,
		MaxEmailAccounts:    5,
		DefaultPhpVersion:   "8.3",
		MemoryLimitMb:       512,
		PidsLimit:           64,
	}); err != nil {
		t.Fatalf("create package: %v", err)
	}
	account, err := state.HostingManager.CreateAccount(hosting.CreateAccountRequest{
		TenantID: "tenant-default",
		Username: "svc_client",
		Domain:   "svc-client.test",
		Package:  "serviceplan",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return state, handler, account.Username
}

func doJSON(t *testing.T, handler http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestDatabaseLifecycleThroughAPI(t *testing.T) {
	state, handler, account := prepareServices(t)
	admin := adminToken(t, handler)

	created := doJSON(t, handler, http.MethodPost, "/api/v1/databases", admin, map[string]string{
		"username": account,
		"name":     "shop",
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("create database: %d %s", created.Code, created.Body.String())
	}
	var payload struct {
		Database struct {
			Name string `json:"name"`
		} `json:"database"`
		User struct {
			Name     string `json:"name"`
			Password string `json:"password"`
		} `json:"user"`
		Grants []string `json:"grants"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode creation: %v", err)
	}
	if payload.Database.Name != "svc_client_shop" || payload.User.Name != "svc_client_shop" {
		t.Fatalf("objects are not prefixed by the account: %+v", payload)
	}
	if len(payload.User.Password) < 16 {
		t.Fatalf("expected a generated password, got %q", payload.User.Password)
	}
	if err := db.ValidatePassword(payload.User.Password); err != nil {
		t.Fatalf("generated password does not satisfy the policy: %v", err)
	}

	// The package allows exactly one database, so the second request is refused
	// before anything reaches the engine.
	if rows := doJSON(t, handler, http.MethodPost, "/api/v1/databases", admin, map[string]string{
		"username": account,
		"name":     "blog",
	}); rows.Code != http.StatusConflict {
		t.Fatalf("expected package limit to reject the second database, got %d", rows.Code)
	}

	listed := doJSON(t, handler, http.MethodGet, "/api/v1/databases?username="+account, admin, nil)
	if listed.Code != http.StatusOK {
		t.Fatalf("list databases: %d", listed.Code)
	}

	// A client identity cannot address another account or a different name.
	client := createUserWithRole(t, state, handler, account, "ClientPass2026!", rbac.RoleUser)
	forbidden := doJSON(t, handler, http.MethodGet, "/api/v1/databases?username=someone_else", client, nil)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("cross-account database listing must be forbidden, got %d", forbidden.Code)
	}
	own := doJSON(t, handler, http.MethodGet, "/api/v1/databases?username="+account, client, nil)
	if own.Code != http.StatusOK {
		t.Fatalf("client must read its own databases, got %d", own.Code)
	}

	deleted := doJSON(t, handler, http.MethodDelete, "/api/v1/databases?username="+account+"&name=shop", admin, nil)
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete database: %d %s", deleted.Code, deleted.Body.String())
	}
}

func TestDatabaseStatusReportsConfiguration(t *testing.T) {
	_, handler := setupIntegratedTestServer()
	admin := adminToken(t, handler)

	// Without credentials the capability reports unconfigured instead of failing
	// on first use.
	status := doJSON(t, handler, http.MethodGet, "/api/v1/databases/status", admin, nil)
	if status.Code != http.StatusOK {
		t.Fatalf("status: %d", status.Code)
	}
	var payload map[string]any
	_ = json.Unmarshal(status.Body.Bytes(), &payload)
	if payload["configured"] != false {
		t.Fatalf("status claims configuration without credentials: %+v", payload)
	}

	if code := doJSON(t, handler, http.MethodGet, "/api/v1/databases?username=admin", admin, nil).Code; code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without a database manager, got %d", code)
	}
}

func TestBackupLifecycleAndRestore(t *testing.T) {
	state, handler, account := prepareServices(t)
	admin := adminToken(t, handler)

	if err := os.MkdirAll(filepath.Join(state.StorageDir, "vhosts", account, "public_html"), 0750); err != nil {
		t.Fatalf("seed home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(state.StorageDir, "vhosts", account, "public_html", "index.html"), []byte("<h1>ok</h1>"), 0640); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	created := doJSON(t, handler, http.MethodPost, "/api/v1/backups", admin, map[string]string{
		"username": account,
		"label":    "manual",
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("create backup: %d %s", created.Code, created.Body.String())
	}
	var archive backup.Archive
	if err := json.Unmarshal(created.Body.Bytes(), &archive); err != nil {
		t.Fatalf("decode archive: %v", err)
	}
	if archive.ID == "" || archive.SizeBytes == 0 || archive.Entries == 0 {
		t.Fatalf("incomplete archive record: %+v", archive)
	}

	listed := doJSON(t, handler, http.MethodGet, "/api/v1/backups?username="+account, admin, nil)
	if listed.Code != http.StatusOK {
		t.Fatalf("list backups: %d", listed.Code)
	}
	var archives []backup.Archive
	_ = json.Unmarshal(listed.Body.Bytes(), &archives)
	if len(archives) != 1 || archives[0].ID != archive.ID {
		t.Fatalf("unexpected archive list: %+v", archives)
	}

	verified := doJSON(t, handler, http.MethodPost, "/api/v1/backups/verify", admin, map[string]string{
		"username": account,
		"id":       archive.ID,
	})
	if verified.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", verified.Code, verified.Body.String())
	}

	restored := doJSON(t, handler, http.MethodPost, "/api/v1/backups/restore", admin, map[string]string{
		"username": account,
		"id":       archive.ID,
	})
	if restored.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", restored.Code, restored.Body.String())
	}
	var result backup.RestoreResult
	if err := json.Unmarshal(restored.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode restore: %v", err)
	}
	if result.Files < 1 {
		t.Fatalf("expected at least one restored file, got %d", result.Files)
	}
	content, err := os.ReadFile(filepath.Join(result.Target, "public_html", "index.html"))
	if err != nil || string(content) != "<h1>ok</h1>" {
		t.Fatalf("restored content mismatch: %q %v", content, err)
	}

	// A restore target outside the staging area is refused.
	escaped := doJSON(t, handler, http.MethodPost, "/api/v1/backups/restore", admin, map[string]string{
		"username": account,
		"id":       archive.ID,
		"target":   "/etc",
	})
	if escaped.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an out-of-tree restore target, got %d", escaped.Code)
	}

	deleted := doJSON(t, handler, http.MethodDelete, "/api/v1/backups?username="+account+"&id="+archive.ID, admin, nil)
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete backup: %d %s", deleted.Code, deleted.Body.String())
	}
}

func TestBackupScheduleRunsAndPrunes(t *testing.T) {
	state, handler, account := prepareServices(t)
	admin := adminToken(t, handler)

	if err := os.MkdirAll(filepath.Join(state.StorageDir, "vhosts", account, "public_html"), 0750); err != nil {
		t.Fatalf("seed home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(state.StorageDir, "vhosts", account, "public_html", "index.html"), []byte("scheduled"), 0640); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	saved := doJSON(t, handler, http.MethodPost, "/api/v1/backups/schedules", admin, map[string]any{
		"account":   account,
		"label":     "nightly",
		"frequency": "daily",
		"hour":      2,
		"retention": 2,
		"enabled":   true,
	})
	if saved.Code != http.StatusCreated {
		t.Fatalf("save schedule: %d %s", saved.Code, saved.Body.String())
	}
	var schedule backup.Schedule
	if err := json.Unmarshal(saved.Body.Bytes(), &schedule); err != nil {
		t.Fatalf("decode schedule: %v", err)
	}
	if schedule.NextRun.IsZero() || schedule.Frequency != "daily" {
		t.Fatalf("unexpected schedule: %+v", schedule)
	}

	// Nothing is due yet.
	if count, err := state.RunDueBackupSchedules(context.Background(), time.Now()); err != nil || count != 0 {
		t.Fatalf("schedule fired early: %d %v", count, err)
	}

	later := schedule.NextRun.Add(time.Minute)
	if count, err := state.RunDueBackupSchedules(context.Background(), later); err != nil || count != 1 {
		t.Fatalf("expected one scheduled backup, got %d (%v)", count, err)
	}

	archives, err := state.Backups.List(account)
	if err != nil || len(archives) != 1 {
		t.Fatalf("scheduled archive missing: %+v %v", archives, err)
	}
	if archives[0].Label != "nightly" {
		t.Fatalf("scheduled archive lost its label: %+v", archives[0])
	}

	refreshed, err := state.Backups.Schedules()
	if err != nil || len(refreshed) != 1 || refreshed[0].LastRun == nil {
		t.Fatalf("schedule was not advanced: %+v %v", refreshed, err)
	}
	if !refreshed[0].NextRun.After(later) {
		t.Fatalf("next window did not advance: %s", refreshed[0].NextRun)
	}

	// Retention is applied on each run: three archives, keep two.
	for index := 0; index < 2; index++ {
		if _, err := state.Backups.Create(context.Background(), account, filepath.Join(state.StorageDir, "vhosts", account), "manual"); err != nil {
			t.Fatalf("create archive: %v", err)
		}
	}
	if removed, err := state.Backups.Prune(account, 2); err != nil || removed != 1 {
		t.Fatalf("expected one pruned archive, got %d (%v)", removed, err)
	}
}

func TestFirewallBlocklistThroughAPI(t *testing.T) {
	state, handler, _ := prepareServices(t)
	admin := adminToken(t, handler)

	created := doJSON(t, handler, http.MethodPost, "/api/v1/firewall/rules", admin, map[string]any{
		"target":        "203.0.113.5",
		"reason":        "credential stuffing",
		"durationHours": 24,
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("block: %d %s", created.Code, created.Body.String())
	}
	var rule firewall.Rule
	if err := json.Unmarshal(created.Body.Bytes(), &rule); err != nil {
		t.Fatalf("decode rule: %v", err)
	}
	if rule.Target != "203.0.113.5" || rule.ExpiresAt == nil {
		t.Fatalf("unexpected rule: %+v", rule)
	}

	if duplicate := doJSON(t, handler, http.MethodPost, "/api/v1/firewall/rules", admin, map[string]any{
		"target":        "203.0.113.5",
		"reason":        "repeat",
		"durationHours": 1,
	}); duplicate.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a duplicate target, got %d", duplicate.Code)
	}

	if invalid := doJSON(t, handler, http.MethodPost, "/api/v1/firewall/rules", admin, map[string]any{
		"target":        "not-an-ip",
		"reason":        "invalid input",
		"durationHours": 1,
	}); invalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an invalid target, got %d", invalid.Code)
	}

	listed := doJSON(t, handler, http.MethodGet, "/api/v1/firewall/rules", admin, nil)
	if listed.Code != http.StatusOK {
		t.Fatalf("list rules: %d", listed.Code)
	}
	var rules []firewall.Rule
	_ = json.Unmarshal(listed.Body.Bytes(), &rules)
	if len(rules) != 1 {
		t.Fatalf("unexpected rule list: %+v", rules)
	}

	status := doJSON(t, handler, http.MethodGet, "/api/v1/firewall/status", admin, nil)
	if status.Code != http.StatusOK {
		t.Fatalf("status: %d", status.Code)
	}
	if !strings.Contains(status.Body.String(), "inet digipanel") {
		t.Fatalf("status does not name the managed table: %s", status.Body.String())
	}

	// A client account holds no firewall capability at all.
	account := state.HostingManager.ListAccounts()[0].Username
	client := createUserWithRole(t, state, handler, account, "ClientPass2026!", rbac.RoleUser)
	if code := doJSON(t, handler, http.MethodGet, "/api/v1/firewall/rules", client, nil).Code; code != http.StatusForbidden {
		t.Fatalf("client must not read the firewall, got %d", code)
	}

	if removed := doJSON(t, handler, http.MethodDelete, "/api/v1/firewall/rules?id="+rule.ID, admin, nil); removed.Code != http.StatusOK {
		t.Fatalf("unblock: %d %s", removed.Code, removed.Body.String())
	}
}
