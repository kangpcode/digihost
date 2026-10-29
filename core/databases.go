// DigiPanel Database API (core/databases.go)
// MariaDB/MySQL provisioning scoped to hosting accounts. Every object name is
// derived from the owning account, so a client cannot reach another tenant's
// schema. The package plan limits how many databases an account may hold.
// Zero emojis, strictly production grade.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"digipanel/modules/db"
)

type databaseCreationResponse struct {
	Database db.Database `json:"database"`
	User     struct {
		Name     string `json:"name"`
		Host     string `json:"host"`
		Password string `json:"password,omitempty"`
	} `json:"user"`
	Grants []string `json:"grants"`
}

func (state *ServerState) databaseManager() *db.Manager {
	return state.DBManager
}

func (state *ServerState) loadScopedAccount(r *http.Request, username string) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return "", errors.New("username is required")
	}
	if err := state.authorizeAccountScope(r, username); err != nil {
		return "", err
	}
	if _, err := state.HostingManager.GetAccount(username); err != nil {
		return "", err
	}
	return username, nil
}

func (state *ServerState) accountDatabaseLimit(username string) int {
	account, err := state.HostingManager.GetAccount(username)
	if err != nil {
		return 0
	}
	for _, pkg := range state.HostingManager.ListPackages() {
		if pkg.Name == account.PackageID {
			return pkg.MaxDatabases
		}
	}
	return 0
}

func databaseStatusFor(err error) int {
	switch {
	case errors.Is(err, db.ErrNotConfigured), errors.Is(err, db.ErrClientMissing), errors.Is(err, ErrAgentUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, db.ErrInvalidDatabase), errors.Is(err, db.ErrInvalidUser),
		errors.Is(err, db.ErrInvalidOwner), errors.Is(err, db.ErrInvalidPassword),
		errors.Is(err, db.ErrTooLong), errors.Is(err, db.ErrUnsupported):
		return http.StatusBadRequest
	default:
		return http.StatusBadGateway
	}
}

func (state *ServerState) handleDatabaseStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	manager := state.databaseManager()
	status := map[string]any{
		"engine":      "mysql",
		"configured":  manager != nil && manager.Configured(),
		"clientReady": manager != nil && manager.Available(),
		"host":        "",
	}
	if manager != nil {
		options := manager.Options()
		status["host"] = options.Host + ":" + options.Port
		if options.Engine != "" {
			status["engine"] = options.Engine
		}
		if manager.Configured() {
			if version, err := manager.Ping(r.Context()); err == nil {
				status["version"] = version
			} else {
				status["error"] = err.Error()
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (state *ServerState) handleDatabases(w http.ResponseWriter, r *http.Request) {
	manager := state.databaseManager()

	switch r.Method {
	case http.MethodGet:
		if manager == nil || !manager.Configured() {
			writeAPIError(w, db.ErrNotConfigured.Error(), http.StatusServiceUnavailable)
			return
		}
		username, err := state.loadScopedAccount(r, r.URL.Query().Get("username"))
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusForbidden)
			return
		}
		databases, err := manager.ListDatabases(r.Context(), username)
		if err != nil {
			writeAPIError(w, err.Error(), databaseStatusFor(err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(databases)

	case http.MethodPost:
		var body struct {
			Username string `json:"username"`
			Name     string `json:"name"`
		}
		if err := decodeJSONBody(w, r, &body); err != nil {
			writeAPIError(w, "Invalid database payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		if manager == nil || !manager.Configured() {
			writeAPIError(w, db.ErrNotConfigured.Error(), http.StatusServiceUnavailable)
			return
		}
		username, err := state.loadScopedAccount(r, body.Username)
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusForbidden)
			return
		}

		if limit := state.accountDatabaseLimit(username); limit > 0 {
			existing, listErr := manager.ListDatabases(r.Context(), username)
			if listErr != nil {
				writeAPIError(w, listErr.Error(), databaseStatusFor(listErr))
				return
			}
			if len(existing) >= limit {
				writeAPIError(w, fmt.Sprintf("database limit reached for package plan (%d)", limit), http.StatusConflict)
				return
			}
		}

		database, err := manager.CreateDatabase(r.Context(), username, body.Name)
		if err != nil {
			writeAPIError(w, err.Error(), databaseStatusFor(err))
			return
		}

		password, err := db.GeneratePassword()
		if err != nil {
			_ = manager.DropDatabase(r.Context(), username, body.Name)
			writeAPIError(w, "unable to generate a database password", http.StatusInternalServerError)
			return
		}
		suffix := strings.ToLower(strings.TrimSpace(body.Name))
		user, err := manager.CreateUser(r.Context(), username, suffix, password, suffix)
		if err != nil {
			// Never leave a schema behind without its identity.
			_ = manager.DropDatabase(r.Context(), username, body.Name)
			writeAPIError(w, "Database rolled back, user creation failed: "+err.Error(), databaseStatusFor(err))
			return
		}

		response := databaseCreationResponse{Database: *database, Grants: user.Grants}
		response.User.Name = user.Name
		response.User.Host = user.Host
		response.User.Password = password

		state.AuditLogger.Append(state.actorFor(r), "db.create", "database", database.Name,
			fmt.Sprintf(`{"account":%q,"user":%q}`, username, user.Name), clientIP(r, state.TrustedProxies))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(response)

	case http.MethodDelete:
		if manager == nil || !manager.Configured() {
			writeAPIError(w, db.ErrNotConfigured.Error(), http.StatusServiceUnavailable)
			return
		}
		username, err := state.loadScopedAccount(r, r.URL.Query().Get("username"))
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusForbidden)
			return
		}
		name := r.URL.Query().Get("name")
		if err := manager.DropDatabase(r.Context(), username, name); err != nil {
			writeAPIError(w, err.Error(), databaseStatusFor(err))
			return
		}
		state.AuditLogger.Append(state.actorFor(r), "db.drop", "database", name,
			fmt.Sprintf(`{"account":%q}`, username), clientIP(r, state.TrustedProxies))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"dropped": true})

	default:
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (state *ServerState) handleDatabaseUsers(w http.ResponseWriter, r *http.Request) {
	manager := state.databaseManager()

	switch r.Method {
	case http.MethodGet:
		username, err := state.loadScopedAccount(r, r.URL.Query().Get("username"))
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusForbidden)
			return
		}
		if manager == nil || !manager.Configured() {
			writeAPIError(w, db.ErrNotConfigured.Error(), http.StatusServiceUnavailable)
			return
		}
		users, err := manager.ListUsers(r.Context(), username)
		if err != nil {
			writeAPIError(w, err.Error(), databaseStatusFor(err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(users)

	case http.MethodPost, http.MethodPut:
		var body struct {
			Username string `json:"username"`
			Name     string `json:"name"`
			Database string `json:"database"`
			Password string `json:"password"`
		}
		if err := decodeJSONBody(w, r, &body); err != nil {
			writeAPIError(w, "Invalid database user payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		username, err := state.loadScopedAccount(r, body.Username)
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusForbidden)
			return
		}
		if manager == nil || !manager.Configured() {
			writeAPIError(w, db.ErrNotConfigured.Error(), http.StatusServiceUnavailable)
			return
		}

		password := strings.TrimSpace(body.Password)
		if password == "" {
			generated, genErr := db.GeneratePassword()
			if genErr != nil {
				writeAPIError(w, "unable to generate a database password", http.StatusInternalServerError)
				return
			}
			password = generated
		}

		var result any
		if r.Method == http.MethodPut {
			if err := manager.SetPassword(r.Context(), username, body.Name, password); err != nil {
				writeAPIError(w, err.Error(), databaseStatusFor(err))
				return
			}
			result = map[string]any{"name": body.Name, "password": password, "rotated": true}
			state.AuditLogger.Append(state.actorFor(r), "db.user.password", "database_user", body.Name,
				fmt.Sprintf(`{"account":%q}`, username), clientIP(r, state.TrustedProxies))
		} else {
			user, err := manager.CreateUser(r.Context(), username, body.Name, password, body.Database)
			if err != nil {
				writeAPIError(w, err.Error(), databaseStatusFor(err))
				return
			}
			result = map[string]any{"user": user, "password": password}
			state.AuditLogger.Append(state.actorFor(r), "db.user.create", "database_user", user.Name,
				fmt.Sprintf(`{"account":%q,"grants":%v}`, username, user.Grants), clientIP(r, state.TrustedProxies))
		}

		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		_ = json.NewEncoder(w).Encode(result)

	case http.MethodDelete:
		if manager == nil || !manager.Configured() {
			writeAPIError(w, db.ErrNotConfigured.Error(), http.StatusServiceUnavailable)
			return
		}
		username, err := state.loadScopedAccount(r, r.URL.Query().Get("username"))
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusForbidden)
			return
		}
		name := r.URL.Query().Get("name")
		if err := manager.DropUser(r.Context(), username, name); err != nil {
			writeAPIError(w, err.Error(), databaseStatusFor(err))
			return
		}
		state.AuditLogger.Append(state.actorFor(r), "db.user.drop", "database_user", name,
			fmt.Sprintf(`{"account":%q}`, username), clientIP(r, state.TrustedProxies))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"dropped": true})

	default:
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// actorFor returns the authenticated username for audit records.
func (state *ServerState) actorFor(r *http.Request) string {
	session, err := state.Sessions.Resolve(bearerToken(r))
	if err != nil {
		return "anonymous"
	}
	return session.Username
}
