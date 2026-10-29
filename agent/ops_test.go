// DigiPanel agent privileged-operation tests.
// These prove that the database, backup, and firewall actions fail closed when
// the node is not configured, and that hostile parameters never reach a binary.

package main

import (
	"strings"
	"testing"
)

func TestDatabaseCommandsFailClosedWithoutCredentials(t *testing.T) {
	t.Setenv("DIGIPANEL_DB_ADMIN_USER", "")
	t.Setenv("DIGIPANEL_DB_ADMIN_PASSWORD", "")

	for _, action := range []string{"db.list", "db.create", "db.drop", "db.user.create", "db.user.drop", "db.user.password", "db.user.grant"} {
		response := handleAgentRequest(AgentRequest{Action: action, Params: map[string]string{
			"owner": "acme_client",
			"name":  "shop",
		}})
		if response.Success {
			t.Fatalf("action %s reported success without administrator credentials", action)
		}
		if !strings.Contains(response.Error, "not configured") {
			t.Fatalf("action %s returned an unexpected error: %s", action, response.Error)
		}
	}
}

func TestDatabaseCommandRejectsInjectionParameters(t *testing.T) {
	t.Setenv("DIGIPANEL_DB_ADMIN_USER", "root")
	t.Setenv("DIGIPANEL_DB_ADMIN_PASSWORD", "admin-password")
	t.Setenv("DIGIPANEL_MYSQL_BINARY", "/bin/false")

	for _, payload := range []string{
		"acme_client; DROP DATABASE mysql",
		"../etc/passwd",
		"`id`",
		"acme client",
	} {
		response := handleAgentRequest(AgentRequest{Action: "db.create", Params: map[string]string{
			"owner": "acme_client",
			"name":  payload,
		}})
		if response.Success {
			t.Fatalf("injection payload %q was accepted", payload)
		}
	}
}

func TestBackupCommandsRequirePassphrase(t *testing.T) {
	t.Setenv("DIGIPANEL_BACKUP_PASSPHRASE", "")
	t.Setenv("DIGIPANEL_BACKUP_DIR", t.TempDir())
	t.Setenv("DIGIPANEL_HOME_BASE", t.TempDir())

	for _, action := range []string{"backup.create", "backup.list", "backup.verify", "backup.restore"} {
		response := handleAgentRequest(AgentRequest{Action: action, Params: map[string]string{
			"account": "acme_client",
			"source":  "/home/acme_client",
			"id":      "bk-20260101T000000-abcdef",
			"target":  "/home/acme_client",
		}})
		if response.Success {
			t.Fatalf("action %s succeeded without an encryption passphrase", action)
		}
	}
}

func TestBackupSourceConfinement(t *testing.T) {
	homeBase := t.TempDir()
	t.Setenv("DIGIPANEL_BACKUP_PASSPHRASE", "strong-passphrase")
	t.Setenv("DIGIPANEL_BACKUP_DIR", t.TempDir())
	t.Setenv("DIGIPANEL_HOME_BASE", homeBase)

	for _, source := range []string{"/etc", "../../etc", homeBase + "/../escape", "relative/path"} {
		response := handleAgentRequest(AgentRequest{Action: "backup.create", Params: map[string]string{
			"account": "acme_client",
			"source":  source,
		}})
		if response.Success {
			t.Fatalf("source %q escaped the home base", source)
		}
	}
}

func TestFirewallRejectsRawSyntaxAndBadTargets(t *testing.T) {
	t.Setenv("DIGIPANEL_NFT_BINARY", "/bin/false")

	payloads := []string{
		"1.2.3.4; flush ruleset",
		"1.2.3.4\nadd rule inet filter input accept",
		"not-an-address",
		"1.2.3.4/99",
		"$(id)",
	}
	for _, payload := range payloads {
		response := handleAgentRequest(AgentRequest{Action: "firewall.apply", Params: map[string]string{
			"targets": payload,
		}})
		if response.Success {
			t.Fatalf("firewall payload %q was accepted", payload)
		}
	}
}

func TestFirewallRejectsUnknownAction(t *testing.T) {
	response := handleAgentRequest(AgentRequest{Action: "firewall.flush", Params: map[string]string{}})
	if response.Success {
		t.Fatal("unknown firewall action was permitted")
	}
}
