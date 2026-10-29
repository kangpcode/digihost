// DigiPanel Chained Audit Log Unit Tests (core/audit_test.go)
// Verifies SHA-256 hash chaining and tamper detection

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAuditLogHashChaining(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "digipanel-audit-test-*")
	state := &ServerState{
		StorageDir: tmpDir,
		ModulesDir: tmpDir,
	}
	initStorage(state)

	// Append sequential log entries
	e1 := state.AuditLogger.Append("admin", "domain.create", "domain", "digipanel.id", `{"ip":"103.189.201.42"}`, "127.0.0.1")
	e2 := state.AuditLogger.Append("admin", "dns.record_add", "dns", "rec-99", `{"name":"test"}`, "127.0.0.1")

	entries := state.AuditLogger.GetEntries()
	if e1.PrevHash != entries[0].EntryHash {
		t.Fatalf("e1 PrevHash mismatch with root entry")
	}

	if e2.PrevHash != e1.EntryHash {
		t.Fatalf("e2 PrevHash mismatch with e1 EntryHash")
	}

	valid, _, err := state.AuditLogger.VerifyIntegrity()
	if !valid || err != nil {
		t.Fatalf("Expected audit chain to be valid, got: %v", err)
	}

	handler := NewServerHandler(state)
	req, _ := http.NewRequest("GET", "/api/v1/audit/logs", nil)
	req.Header.Set("Authorization", "Bearer "+TestAuthToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var resp struct {
		IsChainValid bool `json:"isChainValid"`
		Count        int  `json:"count"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&resp)

	if !resp.IsChainValid || resp.Count != 3 {
		t.Errorf("Expected chain valid with 3 entries, got valid=%v count=%d", resp.IsChainValid, resp.Count)
	}
}

func TestAuditLogTamperingDetection(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "digipanel-audit-tamper-test-*")
	state := &ServerState{
		StorageDir: tmpDir,
		ModulesDir: tmpDir,
	}
	initStorage(state)

	state.AuditLogger.Append("admin", "action.one", "res", "1", `{}`, "127.0.0.1")
	state.AuditLogger.Append("admin", "action.two", "res", "2", `{}`, "127.0.0.1")

	valid, _, err := state.AuditLogger.VerifyIntegrity()
	if !valid || err != nil {
		t.Fatalf("Chain should be valid before tampering")
	}

	// Tamper with previous payload in history
	state.AuditLogger.TamperEntryForTesting(1, `{"tampered":true}`)

	validAfterTamper, _, tamperErr := state.AuditLogger.VerifyIntegrity()
	if validAfterTamper || tamperErr == nil {
		t.Errorf("Security failure: tampered audit log was not detected by cryptographic chain verification")
	}
}
