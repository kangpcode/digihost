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
