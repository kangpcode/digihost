// DigiPanel database manager tests.
// These assert the validation contract and the exact statements sent to the
// engine client. A recording executor stands in for a live server; the live
// client path is exercised by the deployment smoke test.

package db

import (
	"context"
	"strings"
	"testing"
)

type recordingExecutor struct {
	args     [][]string
	scripts  []string
	response string
	err      error
}

func (r *recordingExecutor) Run(ctx context.Context, args []string, stdin string) (string, error) {
	r.args = append(r.args, args)
	r.scripts = append(r.scripts, stdin)
	return r.response, r.err
}

func testManager(response string) (*Manager, *recordingExecutor) {
	executor := &recordingExecutor{response: response}
	manager := NewManagerWithExecutor(Options{
		Engine:        EngineMySQL,
		AdminUser:     "root",
		AdminPassword: "admin-secret",
	}, executor)
	return manager, executor
}

func TestQualifiedRequiresAccountPrefix(t *testing.T) {
	name, err := Qualified("Acme_Client", "Shop")
	if err != nil {
		t.Fatalf("qualified name: %v", err)
	}
	if name != "acme_client_shop" {
		t.Fatalf("unexpected qualified name %q", name)
	}

	// The prefix keeps tenants apart, so an owner without a valid prefix fails.
	for _, owner := range []string{"", "1bad", "a", "../etc", "owner-name"} {
		if _, err := Qualified(owner, "shop"); err == nil {
			t.Fatalf("owner %q was accepted", owner)
		}
	}
	if _, err := Qualified("acme_client", "shop; DROP DATABASE mysql"); err == nil {
		t.Fatal("identifier with SQL metacharacters was accepted")
	}
	if _, err := Qualified("acme_client", "user_name_that_is_longer_than_32"); err == nil {
		t.Fatal("overlong identifier was accepted")
	}
}

func TestPasswordPolicy(t *testing.T) {
	if err := ValidatePassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	if err := ValidatePassword("valid'but-quoted-password"); err == nil {
		t.Fatal("password with a quote was accepted")
	}
	generated, err := GeneratePassword()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := ValidatePassword(generated); err != nil {
		t.Fatalf("generated password rejected: %v", err)
	}
}

func TestCreateDatabaseSendsPrefixedStatement(t *testing.T) {
	manager, executor := testManager("")
	if _, err := manager.CreateDatabase(context.Background(), "acme_client", "shop"); err != nil {
		t.Fatalf("create database: %v", err)
	}
	if len(executor.scripts) != 1 {
		t.Fatalf("expected one statement, got %d", len(executor.scripts))
	}
	script := executor.scripts[0]
	if !strings.Contains(script, "CREATE DATABASE IF NOT EXISTS `acme_client_shop`") {
		t.Fatalf("statement does not create the prefixed schema: %s", script)
	}
	if !strings.Contains(script, "utf8mb4") {
		t.Fatalf("charset is missing: %s", script)
	}
	for _, arg := range executor.args[0] {
		if strings.Contains(arg, "admin-secret") {
			t.Fatal("administrator password leaked into argv")
		}
	}
}

func TestCreateUserGrantsOnlyOwnSchema(t *testing.T) {
	manager, executor := testManager("")
	user, err := manager.CreateUser(context.Background(), "acme_client", "shop", "StrongPassword123!", "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if user.Name != "acme_client_shop" || user.Host != "localhost" {
		t.Fatalf("unexpected user %+v", user)
	}
	script := executor.scripts[0]
	if !strings.Contains(script, "GRANT ALL PRIVILEGES ON `acme_client_shop`.* TO 'acme_client_shop'@'localhost'") {
		t.Fatalf("grant is not scoped to the account schema: %s", script)
	}
	if strings.Contains(script, "WITH GRANT OPTION") {
		t.Fatal("grant option must never be issued")
	}
}

func TestCreateUserRejectsUnsafeSchemaSuffix(t *testing.T) {
	manager, executor := testManager("")
	// The schema argument is a suffix of the same account, so a caller cannot
	// name another tenant's schema and no statement is sent when it is invalid.
	if _, err := manager.CreateUser(context.Background(), "acme_client", "shop", "StrongPassword123!", "other/client"); err == nil {
		t.Fatal("schema suffix with a path separator was accepted")
	}
	if len(executor.scripts) != 0 {
		t.Fatalf("invalid input still reached the database: %v", executor.scripts)
	}
}

func TestNotConfiguredFailsClosed(t *testing.T) {
	manager := NewManagerWithExecutor(Options{}, &recordingExecutor{})
	if manager.Configured() {
		t.Fatal("manager reports configured without credentials")
	}
	if _, err := manager.Ping(context.Background()); err == nil {
		t.Fatal("Ping must fail when credentials are absent")
	}
	if _, err := manager.CreateDatabase(context.Background(), "acme_client", "shop"); err == nil {
		t.Fatal("CreateDatabase must fail when credentials are absent")
	}
}

func TestListDatabasesParsesRows(t *testing.T) {
	manager, _ := testManager("acme_client_shop\tutf8mb4\tutf8mb4_unicode_ci\t12\t4096\nacme_client_blog\tutf8mb4\tutf8mb4_unicode_ci\t3\t1024\n")
	databases, err := manager.ListDatabases(context.Background(), "acme_client")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(databases) != 2 {
		t.Fatalf("expected 2 databases, got %d", len(databases))
	}
	if databases[0].Name != "acme_client_blog" || databases[1].Tables != 12 || databases[1].SizeBytes != 4096 {
		t.Fatalf("unexpected rows: %+v", databases)
	}
}

func TestListDatabasesRejectsInvalidOwner(t *testing.T) {
	manager, executor := testManager("")
	if _, err := manager.ListDatabases(context.Background(), "1bad"); err == nil {
		t.Fatal("invalid owner was accepted")
	}
	if len(executor.scripts) != 0 {
		t.Fatalf("invalid owner still reached the database: %v", executor.scripts)
	}
}
