// DigiPanel Backup API (core/backups.go)
// Encrypted account archives with manifest verification, restore into a
// staging directory, and persistent schedules executed by the panel clock.
// Zero emojis, strictly production grade.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"digipanel/modules/backup"
)

func backupStatusFor(err error) int {
	switch {
	case errors.Is(err, backup.ErrNoPassphrase):
		return http.StatusServiceUnavailable
	case errors.Is(err, backup.ErrArchiveMissing):
		return http.StatusNotFound
	case errors.Is(err, backup.ErrInvalidAccount), errors.Is(err, backup.ErrInvalidLabel),
		errors.Is(err, backup.ErrInvalidFrequency), errors.Is(err, backup.ErrInvalidHour),
		errors.Is(err, backup.ErrInvalidRetention), errors.Is(err, backup.ErrSourceMissing),
		errors.Is(err, backup.ErrSourceEscaped):
		return http.StatusBadRequest
	case errors.Is(err, backup.ErrChecksum), errors.Is(err, backup.ErrDecryptFailed),
		errors.Is(err, backup.ErrBadMagic), errors.Is(err, backup.ErrTruncatedFrame),
		errors.Is(err, backup.ErrUnsafeEntry):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

func (state *ServerState) backupEngine() *backup.Engine {
	return state.Backups
}

// restoreStagingDir keeps dormant restores outside the live account tree so a
// restore never overwrites production files before an operator moves them.
func (state *ServerState) restoreStagingDir(account, id string) string {
	return filepath.Join(state.StorageDir, "restores", account, id)
}

func (state *ServerState) handleBackups(w http.ResponseWriter, r *http.Request) {
	engine := state.backupEngine()
	if engine == nil {
		writeAPIError(w, backup.ErrNoPassphrase.Error(), http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		username, err := state.loadScopedAccount(r, r.URL.Query().Get("username"))
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusForbidden)
			return
		}
		archives, err := engine.List(username)
		if err != nil {
			writeAPIError(w, err.Error(), backupStatusFor(err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(archives)

	case http.MethodPost:
		var body struct {
			Username string `json:"username"`
			Label    string `json:"label"`
		}
		if err := decodeJSONBody(w, r, &body); err != nil {
			writeAPIError(w, "Invalid backup payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		username, err := state.loadScopedAccount(r, body.Username)
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusForbidden)
			return
		}
		if !engine.Configured() {
			writeAPIError(w, backup.ErrNoPassphrase.Error(), http.StatusServiceUnavailable)
			return
		}
		account, err := state.HostingManager.GetAccount(username)
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusNotFound)
			return
		}
		archive, err := engine.Create(r.Context(), username, account.HomeDir, body.Label)
		if err != nil {
			writeAPIError(w, err.Error(), backupStatusFor(err))
			return
		}
		state.AuditLogger.Append(state.actorFor(r), "backup.create", "backup", archive.ID,
			fmt.Sprintf(`{"account":%q,"bytes":%d,"entries":%d}`, username, archive.SizeBytes, archive.Entries),
			clientIP(r, state.TrustedProxies))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(archive)

	case http.MethodDelete:
		username, err := state.loadScopedAccount(r, r.URL.Query().Get("username"))
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusForbidden)
			return
		}
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if err := engine.Delete(username, id); err != nil {
			writeAPIError(w, err.Error(), backupStatusFor(err))
			return
		}
		state.AuditLogger.Append(state.actorFor(r), "backup.delete", "backup", id,
			fmt.Sprintf(`{"account":%q}`, username), clientIP(r, state.TrustedProxies))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"deleted": true})

	default:
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (state *ServerState) handleBackupRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	engine := state.backupEngine()
	if engine == nil || !engine.Configured() {
		writeAPIError(w, backup.ErrNoPassphrase.Error(), http.StatusServiceUnavailable)
		return
	}

	var body struct {
		Username string `json:"username"`
		ID       string `json:"id"`
		Target   string `json:"target"`
	}
	if err := decodeJSONBody(w, r, &body); err != nil {
		writeAPIError(w, "Invalid restore payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	username, err := state.loadScopedAccount(r, body.Username)
	if err != nil {
		writeAPIError(w, err.Error(), http.StatusForbidden)
		return
	}

	target := strings.TrimSpace(body.Target)
	if target == "" {
		target = state.restoreStagingDir(username, body.ID)
	} else {
		// A caller supplied target must stay inside the panel staging area.
		stagingRoot := filepath.Join(state.StorageDir, "restores")
		clean := filepath.Clean(target)
		relative, relErr := filepath.Rel(stagingRoot, clean)
		if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			writeAPIError(w, "restore target must be inside the panel staging directory", http.StatusBadRequest)
			return
		}
		target = clean
	}

	result, err := engine.Restore(r.Context(), username, body.ID, target)
	if err != nil {
		writeAPIError(w, err.Error(), backupStatusFor(err))
		return
	}
	state.AuditLogger.Append(state.actorFor(r), "backup.restore", "backup", body.ID,
		fmt.Sprintf(`{"account":%q,"target":%q,"files":%d}`, username, result.Target, result.Files),
		clientIP(r, state.TrustedProxies))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (state *ServerState) handleBackupVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	engine := state.backupEngine()
	if engine == nil || !engine.Configured() {
		writeAPIError(w, backup.ErrNoPassphrase.Error(), http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Username string `json:"username"`
		ID       string `json:"id"`
	}
	if err := decodeJSONBody(w, r, &body); err != nil {
		writeAPIError(w, "Invalid verify payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	username, err := state.loadScopedAccount(r, body.Username)
	if err != nil {
		writeAPIError(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := engine.Verify(username, body.ID); err != nil {
		writeAPIError(w, err.Error(), backupStatusFor(err))
		return
	}
	state.AuditLogger.Append(state.actorFor(r), "backup.verify", "backup", body.ID,
		fmt.Sprintf(`{"account":%q,"result":"ok"}`, username), clientIP(r, state.TrustedProxies))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": body.ID, "verified": true})
}

func (state *ServerState) handleBackupStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	engine := state.backupEngine()
	status := map[string]any{"configured": engine != nil && engine.Configured(), "staging": filepath.Join(state.StorageDir, "restores")}
	if engine != nil {
		status["root"] = engine.Root()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (state *ServerState) handleBackupSchedules(w http.ResponseWriter, r *http.Request) {
	engine := state.backupEngine()
	if engine == nil {
		writeAPIError(w, backup.ErrNoPassphrase.Error(), http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		schedules, err := engine.Schedules()
		if err != nil {
			writeAPIError(w, err.Error(), backupStatusFor(err))
			return
		}
		session, _ := state.Sessions.Resolve(bearerToken(r))
		if session != nil && session.Role == "user" {
			scoped := make([]backup.Schedule, 0, 1)
			for _, schedule := range schedules {
				if schedule.Account == session.Username {
					scoped = append(scoped, schedule)
				}
			}
			schedules = scoped
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(schedules)

	case http.MethodPost:
		var body struct {
			Account   string `json:"account"`
			Label     string `json:"label"`
			Frequency string `json:"frequency"`
			Hour      int    `json:"hour"`
			Retention int    `json:"retention"`
			Enabled   *bool  `json:"enabled"`
		}
		if err := decodeJSONBody(w, r, &body); err != nil {
			writeAPIError(w, "Invalid schedule payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		account, err := state.loadScopedAccount(r, body.Account)
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusForbidden)
			return
		}
		enabled := true
		if body.Enabled != nil {
			enabled = *body.Enabled
		}
		schedule, err := engine.SaveSchedule(backup.Schedule{
			Account:   account,
			Label:     body.Label,
			Frequency: body.Frequency,
			Hour:      body.Hour,
			Retention: body.Retention,
			Enabled:   enabled,
		})
		if err != nil {
			writeAPIError(w, err.Error(), backupStatusFor(err))
			return
		}
		state.AuditLogger.Append(state.actorFor(r), "backup.schedule.save", "backup_schedule", schedule.ID,
			fmt.Sprintf(`{"account":%q,"frequency":%q}`, account, schedule.Frequency), clientIP(r, state.TrustedProxies))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(schedule)

	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if err := engine.DeleteSchedule(id); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, backup.ErrScheduleNotFound) {
				status = http.StatusNotFound
			}
			writeAPIError(w, err.Error(), status)
			return
		}
		state.AuditLogger.Append(state.actorFor(r), "backup.schedule.delete", "backup_schedule", id, `{}`,
			clientIP(r, state.TrustedProxies))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"deleted": true})

	default:
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// RunDueBackupSchedules executes every schedule whose window has arrived. It is
// called by the panel clock and directly by tests, so scheduling behaviour is
// observable rather than implied.
func (state *ServerState) RunDueBackupSchedules(ctx context.Context, now time.Time) (int, error) {
	engine := state.backupEngine()
	if engine == nil || !engine.Configured() {
		return 0, nil
	}
	due, err := engine.Due(now)
	if err != nil {
		return 0, err
	}
	executed := 0
	var firstErr error
	for _, schedule := range due {
		account, err := state.HostingManager.GetAccount(schedule.Account)
		if err != nil {
			firstErr = fmt.Errorf("schedule %s: account %s is unavailable", schedule.ID, schedule.Account)
			continue
		}
		archive, err := engine.Create(ctx, schedule.Account, account.HomeDir, schedule.Label)
		if err != nil {
			firstErr = fmt.Errorf("schedule %s: %w", schedule.ID, err)
			continue
		}
		if _, err := engine.Prune(schedule.Account, schedule.Retention); err != nil {
			firstErr = fmt.Errorf("schedule %s prune: %w", schedule.ID, err)
		}
		if _, err := engine.MarkScheduleRun(schedule.ID, now); err != nil {
			firstErr = fmt.Errorf("schedule %s advance: %w", schedule.ID, err)
		}
		state.AuditLogger.Append("scheduler", "backup.schedule.run", "backup", archive.ID,
			fmt.Sprintf(`{"account":%q,"schedule":%q}`, schedule.Account, schedule.ID), "127.0.0.1")
		executed++
	}
	return executed, firstErr
}
