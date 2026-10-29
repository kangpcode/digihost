// DigiPanel Core Authentication & Default-Deny Router Unit Tests (core/auth_test.go)
// Verifies Rule 18.2: Default-Deny across all HTTP methods

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func setupTestServer() http.Handler {
	tmpDir, _ := os.MkdirTemp("", "digipanel-core-test-*")
	state := &ServerState{
		StorageDir: tmpDir,
		ModulesDir: tmpDir,
	}
	initStorage(state)
	return NewServerHandler(state)
}

func TestPublicRoutesPermitted(t *testing.T) {
	handler := setupTestServer()

	// 1. GET /api/v1/health should succeed without auth
	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /api/v1/health, got %d", rr.Code)
	}

	// 2. POST /api/v1/auth/login with valid credentials
	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /api/v1/auth/login, got %d", rr.Code)
	}
}

func TestDefaultDenyOnAllHttpVerbs(t *testing.T) {
	handler := setupTestServer()

	protectedEndpoints := []string{
		"/api/v1/auth/me",
		"/api/v1/system/telemetry",
		"/api/v1/runtimes/apps",
		"/api/v1/billing/ledger",
		"/api/v1/audit/logs",
		"/api/v1/dns/records",
		"/api/v1/files/list",
		"/api/v1/system/modules",
	}

	verbs := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD"}

	for _, endpoint := range protectedEndpoints {
		for _, verb := range verbs {
			req, _ := http.NewRequest(verb, endpoint, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("Security failure: endpoint %s with verb %s was not denied (got HTTP %d, expected 401)",
					endpoint, verb, rr.Code)
			}
		}
	}
}

func TestAuthenticatedAccessAllowed(t *testing.T) {
	handler := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+TestAuthToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK with valid bearer token, got %d", rr.Code)
	}
}
