// Shared test support for the core API.
// Tests authenticate through the real login endpoint; there is no static token
// that grants a role, so every privileged request in the suite exercises the
// session store, the RBAC table, and the audit trail exactly as production does.

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"digipanel/core/rbac"
)

// testAdminPassword seeds the administrator through DIGIPANEL_ADMIN_PASSWORD for
// the whole package, so no credential is embedded in the binary or in test data
// that ships with the panel.
const testAdminPassword = "TestAdminPass2026!"

func TestMain(m *testing.M) {
	if err := os.Setenv("DIGIPANEL_ADMIN_PASSWORD", testAdminPassword); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// login performs a real credential exchange and returns the issued token.
func login(t *testing.T, handler http.Handler, username, password string) string {
	t.Helper()

	body, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		t.Fatalf("marshal login payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("login as %s failed: %d %s", username, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Token string `json:"token"`
		Role  string `json:"role"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if len(response.Token) < 32 {
		t.Fatalf("login returned a token that is too short to be a session token: %q", response.Token)
	}
	return response.Token
}

// adminToken returns a live administrator session token.
func adminToken(t *testing.T, handler http.Handler) string {
	t.Helper()
	return login(t, handler, "admin", testAdminPassword)
}

// createUserWithRole provisions a panel account with the requested role and
// returns a session token for it.
func createUserWithRole(t *testing.T, state *ServerState, handler http.Handler, username, password string, role rbac.Role) string {
	t.Helper()
	if err := state.createUser(username, password, role, ""); err != nil {
		t.Fatalf("create %s user: %v", role, err)
	}
	return login(t, handler, username, password)
}
