// DigiPanel Terminal Zero-Shell Execution Unit Tests (core/terminal_test.go)
// Verifies Rule 18.1: Zero shell execution from string, whitelist enforcement

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestTerminalProhibitsShellInterpreters(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "digipanel-terminal-test-*")
	state := &ServerState{
		StorageDir: tmpDir,
		ModulesDir: tmpDir,
	}
	initStorage(state)
	handler := NewServerHandler(state)

	forbiddenCommands := []string{
		"bash -c id",
		"sh -c whoami",
		"zsh -c uptime",
		"rm -rf /",
		"eval whoami",
		"python3 -c import os",
	}

	for _, cmd := range forbiddenCommands {
		body, _ := json.Marshal(map[string]string{"command": cmd})
		req, _ := http.NewRequest("POST", "/api/v1/terminal/exec", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+TestAuthToken)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		var resp struct {
			Success bool   `json:"success"`
			Output  string `json:"output"`
		}
		_ = json.NewDecoder(rr.Body).Decode(&resp)

		// Either HTTP 403 Forbidden or success=false
		if rr.Code == http.StatusOK && resp.Success {
			t.Errorf("Security failure: forbidden command '%s' was executed", cmd)
		}
	}
}

func TestTerminalExecutesSafeWhitelistedBinary(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "digipanel-terminal-test-*")
	state := &ServerState{
		StorageDir: tmpDir,
		ModulesDir: tmpDir,
	}
	initStorage(state)
	handler := NewServerHandler(state)

	body, _ := json.Marshal(map[string]string{"command": "whoami"})
	req, _ := http.NewRequest("POST", "/api/v1/terminal/exec", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+TestAuthToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for safe binary whoami, got %d", rr.Code)
	}

	var resp struct {
		Success bool   `json:"success"`
		Output  string `json:"output"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&resp)

	if !resp.Success || resp.Output == "" {
		t.Errorf("Expected whoami to succeed, got success=%v, output=%s", resp.Success, resp.Output)
	}
}
