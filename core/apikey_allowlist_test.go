// DigiPanel Scoped API Keys & Admin IP Allowlist Tests (core/apikey_allowlist_test.go)

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScopedAPIKeysAndAdminAllowlist(t *testing.T) {
	tmpDir := t.TempDir()
	state := &ServerState{
		StorageDir: tmpDir,
		ModulesDir: tmpDir,
	}
	initStorage(state)
	handler := NewServerHandler(state)
	token := adminToken(t, handler)

	// 1. Create scoped API key allowed only to read accounts
	createBody, _ := json.Marshal(map[string]any{
		"name":    "ci-readonly",
		"scopes":  []string{"accounts.read"},
		"ttlDays": 7,
	})
	req, _ := http.NewRequest("POST", "/api/v1/auth/apikeys", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create api key expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var createdResp struct {
		APIKey ScopedAPIKey `json:"apiKey"`
		Token  string       `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &createdResp); err != nil || createdResp.Token == "" {
		t.Fatalf("decode api key response: %v", err)
	}

	// 2. Scoped key can GET /api/v1/hosting/accounts
	req, _ = http.NewRequest("GET", "/api/v1/hosting/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+createdResp.Token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("scoped key expected 200 on accounts.read, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. Scoped key must be denied on ungranted scope (billing.invoices.read)
	req, _ = http.NewRequest("GET", "/api/v1/billing/invoices", nil)
	req.Header.Set("Authorization", "Bearer "+createdResp.Token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("scoped key expected 403 on ungranted scope, got %d", rr.Code)
	}

	// 4. Revoke key and verify subsequent use fails with 401
	req, _ = http.NewRequest("DELETE", "/api/v1/auth/apikeys?id="+createdResp.APIKey.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("revoke expected 200, got %d", rr.Code)
	}
	req, _ = http.NewRequest("GET", "/api/v1/hosting/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+createdResp.Token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("revoked key expected 401, got %d", rr.Code)
	}

	// 5. Admin IP allowlist rejects non-listed RemoteAddr
	state.mu.Lock()
	state.AdminAllowlist = []string{"10.99.0.0/24"}
	state.mu.Unlock()
	req, _ = http.NewRequest("GET", "/api/v1/hosting/accounts", nil)
	req.RemoteAddr = "203.0.113.50:44444"
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("admin allowlist expected 403 for unlisted IP, got %d", rr.Code)
	}
	req, _ = http.NewRequest("GET", "/api/v1/hosting/accounts", nil)
	req.RemoteAddr = "10.99.0.12:44444"
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("admin allowlist expected 200 for allowed CIDR, got %d", rr.Code)
	}
}
