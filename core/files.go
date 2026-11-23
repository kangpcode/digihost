// DigiPanel File Manager API (core/files.go)
// Read-only browsing scoped to a hosting account home directory. Every request
// is resolved against the account record, cleaned, and verified after symlink
// resolution so neither traversal nor symlink tricks can escape the home tree.

package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"digipanel/core/rbac"
	"digipanel/modules/hosting"
)

const (
	maxPreviewBytes = 256 * 1024
	maxListEntries  = 2000
)

// authorizeAccountScope proves that the caller may act on the named hosting
// account. A client role is confined to its own account; elevated roles are
// governed by the route permissions in the RBAC table.
func (state *ServerState) authorizeAccountScope(r *http.Request, username string) error {
	session, err := state.Sessions.Resolve(bearerToken(r))
	if err != nil {
		return os.ErrPermission
	}
	if session.Role == rbac.RoleUser && session.Username != username {
		return os.ErrPermission
	}
	return nil
}

// resolveAccountPath maps a client supplied relative path into an account home
// directory and proves the result stays inside it.
func (state *ServerState) resolveAccountPath(username, relative string) (string, *hosting.HostingAccount, error) {
	account, err := state.HostingManager.GetAccount(username)
	if err != nil {
		return "", nil, err
	}

	clean := strings.TrimSpace(relative)
	if strings.ContainsRune(clean, 0) {
		return "", nil, errors.New("invalid path")
	}
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" {
		clean = "."
	}

	home := filepath.Clean(account.HomeDir)
	candidate := filepath.Join(home, filepath.Clean(clean))
	if candidate != home && !strings.HasPrefix(candidate, home+string(os.PathSeparator)) {
		return "", nil, errors.New("path escapes the account home directory")
	}

	// Resolve symlinks to confirm the real target is still inside the home.
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", nil, err
	}
	if resolved != home && !strings.HasPrefix(resolved, home+string(os.PathSeparator)) {
		return "", nil, errors.New("path escapes the account home directory")
	}
	return resolved, account, nil
}

func (state *ServerState) handleFileList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if username == "" {
		writeAPIError(w, "username is required", http.StatusBadRequest)
		return
	}
	if err := state.authorizeAccountScope(r, username); err != nil {
		writeAPIError(w, "This account is outside your granted scope", http.StatusForbidden)
		return
	}

	resolved, account, err := state.resolveAccountPath(username, r.URL.Query().Get("path"))
	if err != nil {
		writeFileError(w, err)
		return
	}

	info, err := os.Stat(resolved)
	if err != nil {
		writeFileError(w, err)
		return
	}
	if !info.IsDir() {
		writeAPIError(w, "requested path is not a directory", http.StatusBadRequest)
		return
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		writeFileError(w, err)
		return
	}

	files := make([]FileItem, 0, len(entries))
	for _, entry := range entries {
		if len(files) >= maxListEntries {
			break
		}
		entryPath := filepath.Join(resolved, entry.Name())
		entryInfo, err := os.Stat(entryPath)
		if err != nil {
			continue
		}
		relative, err := filepath.Rel(account.HomeDir, entryPath)
		if err != nil {
			continue
		}
		files = append(files, FileItem{
			Name:        entry.Name(),
			Path:        filepath.ToSlash(relative),
			IsDir:       entryInfo.IsDir(),
			Size:        entryInfo.Size(),
			Permissions: entryInfo.Mode().Perm().String(),
			Modified:    entryInfo.ModTime().UTC().Format("2006-01-02 15:04:05"),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(files)
}

func (state *ServerState) handleFileContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := strings.TrimSpace(r.URL.Query().Get("username"))
	fileParam := strings.TrimSpace(r.URL.Query().Get("file"))
	if username == "" || fileParam == "" {
		writeAPIError(w, "username and file are required", http.StatusBadRequest)
		return
	}
	if err := state.authorizeAccountScope(r, username); err != nil {
		writeAPIError(w, "This account is outside your granted scope", http.StatusForbidden)
		return
	}

	resolved, _, err := state.resolveAccountPath(username, fileParam)
	if err != nil {
		writeFileError(w, err)
		return
	}

	info, err := os.Stat(resolved)
	if err != nil {
		writeFileError(w, err)
		return
	}
	if info.IsDir() {
		writeAPIError(w, "requested path is a directory", http.StatusBadRequest)
		return
	}
	if info.Size() > maxPreviewBytes {
		writeAPIError(w, "file is larger than the 256 KB preview limit", http.StatusRequestEntityTooLarge)
		return
	}

	handle, err := os.Open(resolved)
	if err != nil {
		writeFileError(w, err)
		return
	}
	defer handle.Close()

	buffer := make([]byte, maxPreviewBytes)
	read, err := io.ReadFull(handle, buffer)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		writeFileError(w, err)
		return
	}
	buffer = buffer[:read]

	if !utf8.Valid(buffer) || strings.ContainsRune(string(buffer), 0) {
		writeAPIError(w, "binary files cannot be previewed", http.StatusUnsupportedMediaType)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"path":      fileParam,
		"size":      info.Size(),
		"content":   string(buffer),
		"truncated": info.Size() > int64(read),
	})
}

// resolveAccountTargetForWrite resolves a target path inside an account home
// directory for creating, updating, or removing an entry. It evaluates symlinks
// on the deepest existing ancestor so a symlink inside the home tree can never
// redirect a write outside it.
func (state *ServerState) resolveAccountTargetForWrite(username, relative string) (string, *hosting.HostingAccount, error) {
	account, err := state.HostingManager.GetAccount(username)
	if err != nil {
		return "", nil, err
	}

	clean := strings.TrimSpace(relative)
	if strings.ContainsRune(clean, 0) {
		return "", nil, errors.New("invalid path")
	}
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." {
		return "", nil, errors.New("cannot overwrite account root directory")
	}

	home := filepath.Clean(account.HomeDir)
	realHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return "", nil, err
	}
	candidate := filepath.Join(realHome, filepath.Clean(clean))
	if candidate == realHome || !strings.HasPrefix(candidate, realHome+string(os.PathSeparator)) {
		return "", nil, errors.New("path escapes the account home directory")
	}

	// Verify parent directory symlink resolution stays inside realHome.
	parent := filepath.Dir(candidate)
	probe := parent
	for {
		if _, err := os.Stat(probe); err == nil {
			resolvedParent, err := filepath.EvalSymlinks(probe)
			if err != nil {
				return "", nil, err
			}
			if resolvedParent != realHome && !strings.HasPrefix(resolvedParent, realHome+string(os.PathSeparator)) {
				return "", nil, errors.New("path escapes the account home directory")
			}
			break
		}
		next := filepath.Dir(probe)
		if next == probe || len(next) < len(realHome) {
			return "", nil, errors.New("path escapes the account home directory")
		}
		probe = next
	}

	// If target itself exists and is a symlink, verify its target too.
	if info, err := os.Lstat(candidate); err == nil && info.Mode()&os.ModeSymlink != 0 {
		resolvedTarget, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			return "", nil, errors.New("symlink target cannot be resolved safely")
		}
		if resolvedTarget != realHome && !strings.HasPrefix(resolvedTarget, realHome+string(os.PathSeparator)) {
			return "", nil, errors.New("path escapes the account home directory")
		}
	}

	return candidate, account, nil
}

func (state *ServerState) handleFileWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		File     string `json:"file"`
		Content  string `json:"content"`
	}
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || strings.TrimSpace(req.File) == "" {
		writeAPIError(w, "username and file are required", http.StatusBadRequest)
		return
	}
	if err := state.authorizeAccountScope(r, username); err != nil {
		writeAPIError(w, "This account is outside your granted scope", http.StatusForbidden)
		return
	}
	if len(req.Content) > 2*1024*1024 {
		writeAPIError(w, "content exceeds 2 MB write limit", http.StatusRequestEntityTooLarge)
		return
	}

	target, _, err := state.resolveAccountTargetForWrite(username, req.File)
	if err != nil {
		writeFileError(w, err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		writeFileError(w, err)
		return
	}
	if err := os.WriteFile(target, []byte(req.Content), 0644); err != nil {
		writeFileError(w, err)
		return
	}
	state.AuditLogger.Append(state.actorUsername(r), "files.write", "file", req.File,
		`{"username":"`+username+`"}`, clientIP(r, state.TrustedProxies))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"written": true, "path": req.File, "bytes": len(req.Content)})
}

func (state *ServerState) handleFileMkdir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Path     string `json:"path"`
	}
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || strings.TrimSpace(req.Path) == "" {
		writeAPIError(w, "username and path are required", http.StatusBadRequest)
		return
	}
	if err := state.authorizeAccountScope(r, username); err != nil {
		writeAPIError(w, "This account is outside your granted scope", http.StatusForbidden)
		return
	}
	target, _, err := state.resolveAccountTargetForWrite(username, req.Path)
	if err != nil {
		writeFileError(w, err)
		return
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		writeFileError(w, err)
		return
	}
	state.AuditLogger.Append(state.actorUsername(r), "files.mkdir", "file", req.Path,
		`{"username":"`+username+`"}`, clientIP(r, state.TrustedProxies))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"created": true, "path": req.Path})
}

func (state *ServerState) handleFileRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		From     string `json:"from"`
		To       string `json:"to"`
	}
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || strings.TrimSpace(req.From) == "" || strings.TrimSpace(req.To) == "" {
		writeAPIError(w, "username, from, and to are required", http.StatusBadRequest)
		return
	}
	if err := state.authorizeAccountScope(r, username); err != nil {
		writeAPIError(w, "This account is outside your granted scope", http.StatusForbidden)
		return
	}
	src, _, err := state.resolveAccountPath(username, req.From)
	if err != nil {
		writeFileError(w, err)
		return
	}
	dst, _, err := state.resolveAccountTargetForWrite(username, req.To)
	if err != nil {
		writeFileError(w, err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		writeFileError(w, err)
		return
	}
	if err := os.Rename(src, dst); err != nil {
		writeFileError(w, err)
		return
	}
	state.AuditLogger.Append(state.actorUsername(r), "files.rename", "file", req.From,
		`{"username":"`+username+`","to":"`+req.To+`"}`, clientIP(r, state.TrustedProxies))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"renamed": true, "from": req.From, "to": req.To})
}

func (state *ServerState) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	pathParam := strings.TrimSpace(r.URL.Query().Get("path"))
	if username == "" || pathParam == "" {
		writeAPIError(w, "username and path are required", http.StatusBadRequest)
		return
	}
	if err := state.authorizeAccountScope(r, username); err != nil {
		writeAPIError(w, "This account is outside your granted scope", http.StatusForbidden)
		return
	}
	target, _, err := state.resolveAccountTargetForWrite(username, pathParam)
	if err != nil {
		writeFileError(w, err)
		return
	}
	if err := os.RemoveAll(target); err != nil {
		writeFileError(w, err)
		return
	}
	state.AuditLogger.Append(state.actorUsername(r), "files.delete", "file", pathParam,
		`{"username":"`+username+`"}`, clientIP(r, state.TrustedProxies))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"deleted": true, "path": pathParam})
}

func writeFileError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case os.IsNotExist(err):
		status = http.StatusNotFound
	case os.IsPermission(err):
		status = http.StatusForbidden
	case strings.Contains(err.Error(), "escapes"), strings.Contains(err.Error(), "invalid path"):
		status = http.StatusBadRequest
	case strings.Contains(err.Error(), "not found"):
		status = http.StatusNotFound
	}
	writeAPIError(w, err.Error(), status)
}
