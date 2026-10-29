// DigiPanel application marketplace API tests (core/apps_api_test.go)
// Verifies authentication, RBAC, request validation, and error mapping for the
// installer routes. Successful installations require real archives, databases,
// and runtimes, so those paths are exercised by the softinstaller package tests.

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func setupAppsTestServer(t *testing.T) (*ServerState, http.Handler) {
	t.Helper()
	root := t.TempDir()
	catalogDir := filepath.Join(root, "softinstaller", "catalog")
	if err := os.MkdirAll(catalogDir, 0755); err != nil {
		t.Fatalf("create catalogue dir: %v", err)
	}
	source, err := os.ReadFile(filepath.Join("..", "modules", "softinstaller", "catalog", "wordpress.json"))
	if err != nil {
		t.Fatalf("read catalogue fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(catalogDir, "wordpress.json"), source, 0644); err != nil {
		t.Fatalf("write catalogue fixture: %v", err)
	}
	state := &ServerState{
		StorageDir: filepath.Join(root, "storage"),
		ModulesDir: root,
	}
	initStorage(state)
	if state.SoftInstaller == nil {
		t.Fatal("softinstaller manager was not initialised")
	}
	return state, NewServerHandler(state)
}

func doAppsRequest(handler http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		encoded, _ := json.Marshal(body)
		reader = bytes.NewReader(encoded)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestAppsCatalogueAuthenticationAndContent(t *testing.T) {
	_, handler := setupAppsTestServer(t)

	unauthorised := doAppsRequest(handler, "GET", "/api/v1/apps/catalog", "", nil)
	if unauthorised.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", unauthorised.Code)
	}

	reply := doAppsRequest(handler, "GET", "/api/v1/apps/catalog", TestAuthToken, nil)
	if reply.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", reply.Code, reply.Body.String())
	}
	var catalogue []map[string]any
	if err := json.Unmarshal(reply.Body.Bytes(), &catalogue); err != nil {
		t.Fatalf("decode catalogue: %v", err)
	}
	if len(catalogue) != 1 {
		t.Fatalf("expected one catalogue entry, got %d", len(catalogue))
	}
	entry := catalogue[0]
	if entry["id"] != "wordpress" {
		t.Fatalf("unexpected catalogue entry %v", entry["id"])
	}
	if entry["pinnedSHA256"] == "" || entry["pinnedSHA256"] == nil {
		t.Fatal("catalogue entry lost its pinned checksum")
	}
	if entry["completionMode"] != "automatic" {
		t.Fatalf("unexpected completion mode %v", entry["completionMode"])
	}
}

func TestAppsCatalogueRBAC(t *testing.T) {
	_, handler := setupAppsTestServer(t)

	staffRead := doAppsRequest(handler, "GET", "/api/v1/apps/catalog", "token-role-staff", nil)
	if staffRead.Code != http.StatusOK {
		t.Fatalf("staff should read the catalogue, got %d", staffRead.Code)
	}

	staffInstall := doAppsRequest(handler, "POST", "/api/v1/apps/install", "token-role-staff", map[string]any{"packageId": "wordpress"})
	if staffInstall.Code != http.StatusForbidden {
		t.Fatalf("staff install should be forbidden, got %d", staffInstall.Code)
	}

	userInstall := doAppsRequest(handler, "POST", "/api/v1/apps/install", "token-role-user", map[string]any{"packageId": "wordpress"})
	if userInstall.Code != http.StatusBadRequest {
		t.Fatalf("user install should reach validation (400), got %d: %s", userInstall.Code, userInstall.Body.String())
	}

	invalid := doAppsRequest(handler, "GET", "/api/v1/apps/catalog", "not-a-real-token", nil)
	if invalid.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token should be 401, got %d", invalid.Code)
	}
}

func TestAppsInstallValidationAndErrorMapping(t *testing.T) {
	_, handler := setupAppsTestServer(t)

	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"unknown package", map[string]any{"packageId": "not-in-catalogue"}, http.StatusNotFound},
		{"missing domain", map[string]any{"packageId": "wordpress"}, http.StatusBadRequest},
		{"invalid domain", map[string]any{"packageId": "wordpress", "domain": "bad domain"}, http.StatusBadRequest},
		{"non-https site", map[string]any{"packageId": "wordpress", "domain": "example.test", "siteURL": "http://example.test"}, http.StatusBadRequest},
		{"unknown field", map[string]any{"packageId": "wordpress", "unexpected": true}, http.StatusBadRequest},
		{"database unconfigured", map[string]any{"packageId": "wordpress", "domain": "example.test"}, http.StatusBadRequest},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			response := doAppsRequest(handler, "POST", "/api/v1/apps/install", TestAuthToken, testCase.body)
			if response.Code != testCase.want {
				t.Fatalf("expected %d, got %d: %s", testCase.want, response.Code, response.Body.String())
			}
		})
	}
}

func TestAppsInstanceRoutesValidateIdentifiers(t *testing.T) {
	_, handler := setupAppsTestServer(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{"logs without id", "GET", "/api/v1/apps/logs", nil, http.StatusBadRequest},
		{"logs unknown instance", "GET", "/api/v1/apps/logs?id=app-999", nil, http.StatusNotFound},
		{"delete without id", "DELETE", "/api/v1/apps/instances", nil, http.StatusBadRequest},
		{"delete unknown instance", "DELETE", "/api/v1/apps/instances?id=app-999", nil, http.StatusNotFound},
		{"clone without id", "POST", "/api/v1/apps/clone", map[string]any{}, http.StatusBadRequest},
		{"update without id", "POST", "/api/v1/apps/update", map[string]any{}, http.StatusBadRequest},
		{"uninstall without id", "POST", "/api/v1/apps/uninstall", map[string]any{}, http.StatusBadRequest},
		{"catalogue rejects post", "POST", "/api/v1/apps/catalog", map[string]any{}, http.StatusMethodNotAllowed},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			response := doAppsRequest(handler, testCase.method, testCase.path, TestAuthToken, testCase.body)
			if response.Code != testCase.want {
				t.Fatalf("expected %d, got %d: %s", testCase.want, response.Code, response.Body.String())
			}
		})
	}
}

func TestAppsInstancesStartsEmpty(t *testing.T) {
	_, handler := setupAppsTestServer(t)
	response := doAppsRequest(handler, "GET", "/api/v1/apps/instances", TestAuthToken, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	var instances []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &instances); err != nil {
		t.Fatalf("decode instances: %v", err)
	}
	if len(instances) != 0 {
		t.Fatalf("fresh node should report no instances, got %d", len(instances))
	}
}
