// DigiPanel File Manager Security & Write Lifecycle Tests (core/files_test.go)

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"digipanel/modules/hosting"
)

func TestFileManagerWriteLifecycleAndConfinement(t *testing.T) {
	tmpDir := t.TempDir()
	state := &ServerState{
		StorageDir: tmpDir,
		ModulesDir: tmpDir,
	}
	initStorage(state)
	handler := NewServerHandler(state)
	token := adminToken(t, handler)

	acc, err := state.HostingManager.CreateAccount(hosting.CreateAccountRequest{
		Username: "fileuser1",
		Domain:   "fileuser1.example.com",
		Package:  "starter",
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	// 1. Create subdirectory
	mkdirBody, _ := json.Marshal(map[string]string{
		"username": acc.Username,
		"path":     "public_html/assets",
	})
	req, _ := http.NewRequest("POST", "/api/v1/files/mkdir", bytes.NewReader(mkdirBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("mkdir expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// 2. Write file
	writeBody, _ := json.Marshal(map[string]string{
		"username": acc.Username,
		"file":     "public_html/assets/app.txt",
		"content":  "hello digipanel",
	})
	req, _ = http.NewRequest("POST", "/api/v1/files/write", bytes.NewReader(writeBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("write expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. Rename file
	renameBody, _ := json.Marshal(map[string]string{
		"username": acc.Username,
		"from":     "public_html/assets/app.txt",
		"to":       "public_html/assets/app-renamed.txt",
	})
	req, _ = http.NewRequest("POST", "/api/v1/files/rename", bytes.NewReader(renameBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("rename expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// 4. Symlink escape attempt must be rejected
	outside := filepath.Join(tmpDir, "outside-secret.txt")
	_ = os.WriteFile(outside, []byte("secret"), 0600)
	symlinkPath := filepath.Join(acc.HomeDir, "escape-link")
	if err := os.Symlink(tmpDir, symlinkPath); err == nil {
		escapeBody, _ := json.Marshal(map[string]string{
			"username": acc.Username,
			"file":     "escape-link/outside-secret.txt",
			"content":  "pwned",
		})
		req, _ = http.NewRequest("POST", "/api/v1/files/write", bytes.NewReader(escapeBody))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code == http.StatusOK {
			t.Fatalf("Security failure: symlink write escape succeeded")
		}
	}

	// 5. Delete renamed file
	req, _ = http.NewRequest("DELETE", "/api/v1/files?username="+acc.Username+"&path=public_html/assets/app-renamed.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
