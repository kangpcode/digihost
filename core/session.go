// DigiPanel Session, Login Throttle, and Bootstrap Credentials (core/session.go)
// One session implementation for the whole panel: opaque random tokens bound to
// a user, a role, and an expiry. No client supplied role is ever trusted, and no
// credential is hardcoded in the binary. The bootstrap administrator password
// comes from the environment or is generated once and written to a 0600 file.
// Zero emojis, strictly production grade.

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"digipanel/core/audit"
	"digipanel/core/auth"
	"digipanel/core/dbstore"
	"digipanel/core/rbac"
)

const (
	sessionTokenBytes   = 32
	sessionTTL          = 12 * time.Hour
	sessionIdleTTL      = 2 * time.Hour
	maxSessionsPerUser  = 10
	loginFailureLimit   = 5
	loginFailureWindow  = 15 * time.Minute
	loginLockoutPeriod  = 15 * time.Minute
	minPasswordLength   = 10
	maxPasswordLength   = 200
	bootstrapPasswordFN = "initial-admin-password"
)

var (
	ErrSessionNotFound = errors.New("session not found or expired")
	ErrWeakPassword    = errors.New("password must be at least 10 characters and mix letters, digits, or symbols")
	ErrUserExists      = errors.New("user already exists")
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidRole     = errors.New("invalid role")
)

// Session is a live authenticated login.
type Session struct {
	Token     string
	Username  string
	Role      rbac.Role
	IPAddress string
	CreatedAt time.Time
	ExpiresAt time.Time
	LastSeen  time.Time
}

// SessionStore keeps sessions in memory. Tokens are opaque and random; the
// store is the only writer of role information, so a forged token cannot pick
// its own privileges.
type SessionStore struct {
	mu    sync.Mutex
	items map[string]*Session
	now   func() time.Time
}

func NewSessionStore() *SessionStore {
	return &SessionStore{items: make(map[string]*Session), now: time.Now}
}

func randomToken(byteCount int) (string, error) {
	buffer := make([]byte, byteCount)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

// Create issues a fresh token for a user and returns it with its expiry.
func (store *SessionStore) Create(username string, role rbac.Role, ip string) (*Session, error) {
	token, err := randomToken(sessionTokenBytes)
	if err != nil {
		return nil, err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	now := store.now()
	store.pruneLocked(now)

	// Cap concurrent sessions per user, dropping the oldest.
	sameUser := make([]*Session, 0, maxSessionsPerUser+1)
	for _, item := range store.items {
		if item.Username == username {
			sameUser = append(sameUser, item)
		}
	}
	if len(sameUser) >= maxSessionsPerUser {
		oldest := sameUser[0]
		for _, item := range sameUser[1:] {
			if item.CreatedAt.Before(oldest.CreatedAt) {
				oldest = item
			}
		}
		delete(store.items, oldest.Token)
	}

	session := &Session{
		Token:     token,
		Username:  username,
		Role:      role,
		IPAddress: ip,
		CreatedAt: now,
		ExpiresAt: now.Add(sessionTTL),
		LastSeen:  now,
	}
	store.items[token] = session
	copied := *session
	return &copied, nil
}

// Resolve returns the session for a token and refreshes its idle timer.
func (store *SessionStore) Resolve(token string) (*Session, error) {
	if token == "" {
		return nil, ErrSessionNotFound
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	now := store.now()
	session, exists := store.items[token]
	if !exists {
		return nil, ErrSessionNotFound
	}
	if now.After(session.ExpiresAt) || now.Sub(session.LastSeen) > sessionIdleTTL {
		delete(store.items, token)
		return nil, ErrSessionNotFound
	}
	session.LastSeen = now
	copied := *session
	return &copied, nil
}

// Revoke removes one token.
func (store *SessionStore) Revoke(token string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.items, token)
}

// RevokeUser removes every session belonging to a user. Used after a password
// change, a role change, or an account suspension.
func (store *SessionStore) RevokeUser(username string) int {
	store.mu.Lock()
	defer store.mu.Unlock()
	removed := 0
	for token, item := range store.items {
		if item.Username == username {
			delete(store.items, token)
			removed++
		}
	}
	return removed
}

func (store *SessionStore) pruneLocked(now time.Time) {
	for token, item := range store.items {
		if now.After(item.ExpiresAt) || now.Sub(item.LastSeen) > sessionIdleTTL {
			delete(store.items, token)
		}
	}
}

// Count reports live sessions; used by the dashboard session readout.
func (store *SessionStore) Count() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.pruneLocked(store.now())
	return len(store.items)
}

// LoginThrottle blocks a credential pair after repeated failures. The key is a
// username and validated client IP combination, so one attacker cannot lock out
// every user of the panel from a single address.
type LoginThrottle struct {
	mu      sync.Mutex
	entries map[string]*throttleEntry
	now     func() time.Time
}

type throttleEntry struct {
	failures    int
	firstSeen   time.Time
	blockedTill time.Time
}

func NewLoginThrottle() *LoginThrottle {
	return &LoginThrottle{entries: make(map[string]*throttleEntry), now: time.Now}
}

// Check reports whether a login attempt may proceed and how long the caller
// must still wait when it may not.
func (throttle *LoginThrottle) Check(key string) (bool, time.Duration) {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()

	now := throttle.now()
	entry, exists := throttle.entries[key]
	if !exists {
		return true, 0
	}
	if now.Before(entry.blockedTill) {
		return false, entry.blockedTill.Sub(now)
	}
	if now.Sub(entry.firstSeen) > loginFailureWindow {
		delete(throttle.entries, key)
		return true, 0
	}
	return true, 0
}

// Failure records one failed attempt and returns the remaining attempts before
// the key is blocked.
func (throttle *LoginThrottle) Failure(key string) int {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()

	now := throttle.now()
	entry, exists := throttle.entries[key]
	if !exists || now.Sub(entry.firstSeen) > loginFailureWindow {
		entry = &throttleEntry{firstSeen: now}
		throttle.entries[key] = entry
	}
	entry.failures++
	if entry.failures >= loginFailureLimit {
		entry.blockedTill = now.Add(loginLockoutPeriod)
	}
	remaining := loginFailureLimit - entry.failures
	if remaining < 0 {
		remaining = 0
	}
	return remaining
}

// Success clears the failure history for a key.
func (throttle *LoginThrottle) Success(key string) {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()
	delete(throttle.entries, key)
}

// validatePassword enforces the panel password policy.
func validatePassword(password string) error {
	if len(password) < minPasswordLength || len(password) > maxPasswordLength {
		return ErrWeakPassword
	}
	var letters, digits, symbols bool
	for _, runeValue := range password {
		switch {
		case runeValue >= 'a' && runeValue <= 'z', runeValue >= 'A' && runeValue <= 'Z':
			letters = true
		case runeValue >= '0' && runeValue <= '9':
			digits = true
		default:
			symbols = true
		}
	}
	if !letters || (!digits && !symbols) {
		return ErrWeakPassword
	}
	return nil
}

// generatePassword returns a cryptographically random password that satisfies
// the policy without relying on any dictionary word.
func generatePassword(length int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#%*?"
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	out := make([]byte, length)
	for index, value := range buffer {
		out[index] = alphabet[int(value)%len(alphabet)]
	}
	// Guarantee the policy is satisfied regardless of the draw.
	out[0] = 'A'
	out[1] = '7'
	out[2] = '!'
	return string(out), nil
}

// bootstrapAdministrator seeds the single initial administrator. The password is
// taken from DIGIPANEL_ADMIN_PASSWORD when present, otherwise generated once and
// written to <storage>/initial-admin-password with 0600 so the operator can read
// it and change it after the first login.
func bootstrapAdministrator(state *ServerState) error {
	if state.Users == nil {
		state.Users = make(map[string]*UserAccount)
	}
	if _, exists := state.Users["admin"]; exists {
		return nil
	}

	password := os.Getenv("DIGIPANEL_ADMIN_PASSWORD")
	generated := false
	if password == "" {
		random, err := generatePassword(24)
		if err != nil {
			return err
		}
		password = random
		generated = true
	}
	if err := validatePassword(password); err != nil {
		return fmt.Errorf("DIGIPANEL_ADMIN_PASSWORD rejected: %w", err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	state.mu.Lock()
	state.Users["admin"] = &UserAccount{
		Username:         "admin",
		PasswordHash:     hash,
		Role:             rbac.RoleAdmin,
		TwoFactorEnabled: false,
	}
	state.mu.Unlock()

	if !generated {
		return nil
	}

	path := filepath.Join(state.StorageDir, bootstrapPasswordFN)
	content := strings.Join([]string{
		"DigiPanel initial administrator credential",
		"username: admin",
		"password: " + password,
		"Change this password immediately after the first login.",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("write bootstrap credential file: %w", err)
	}
	fmt.Fprintf(os.Stderr, "DigiPanel: generated administrator password written to %s (mode 0600)\n", path)
	return nil
}

// ensureRuntime guarantees that the security subsystems exist before the router
// serves a request. It keeps a constructed ServerState usable even when a caller
// (such as a focused test harness) did not run initStorage.
func (state *ServerState) ensureRuntime() {
	state.mu.Lock()
	if state.Sessions == nil {
		state.Sessions = NewSessionStore()
	}
	if state.APIKeys == nil {
		state.APIKeys = NewAPIKeyStore(state.DB)
	}
	if state.LoginThrottle == nil {
		state.LoginThrottle = NewLoginThrottle()
	}
	if state.AuditLogger == nil {
		state.AuditLogger = audit.NewLogger()
	}
	if len(state.TrustedProxies) == 0 {
		state.TrustedProxies = parseTrustedProxies(os.Getenv("DIGIPANEL_TRUSTED_PROXIES"))
	}
	needsAdministrator := len(state.Users) == 0
	state.mu.Unlock()

	if needsAdministrator {
		if err := bootstrapAdministrator(state); err != nil {
			fmt.Fprintf(os.Stderr, "DigiPanel: administrator bootstrap failed: %v\n", err)
		}
	}
}

// createUser provisions a panel account with a hashed password. Roles are
// validated against the RBAC engine so a caller cannot invent a role string.
func (state *ServerState) createUser(username, password string, role rbac.Role, twoFactorSecret string) error {
	username = strings.TrimSpace(strings.ToLower(username))
	if state.HostingManager != nil {
		if err := state.HostingManager.ValidateUsername(username); err != nil {
			return err
		}
	}
	switch role {
	case rbac.RoleAdmin, rbac.RoleReseller, rbac.RoleUser, rbac.RoleStaff:
	default:
		return ErrInvalidRole
	}
	if err := validatePassword(password); err != nil {
		return err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	state.mu.Lock()
	if state.Users == nil {
		state.Users = make(map[string]*UserAccount)
	}
	if _, exists := state.Users[username]; exists {
		state.mu.Unlock()
		return ErrUserExists
	}
	state.Users[username] = &UserAccount{
		Username:         username,
		PasswordHash:     hash,
		Role:             role,
		TwoFactorEnabled: twoFactorSecret != "",
		TotpSecret:       twoFactorSecret,
	}
	state.mu.Unlock()
	return nil
}

// deleteUser removes a panel account and every live session it holds.
func (state *ServerState) deleteUser(username string) error {
	state.mu.Lock()
	if _, exists := state.Users[username]; !exists {
		state.mu.Unlock()
		return ErrUserNotFound
	}
	if username == "admin" {
		state.mu.Unlock()
		return errors.New("the built-in administrator account cannot be deleted")
	}
	delete(state.Users, username)
	state.mu.Unlock()

	if state.Sessions != nil {
		state.Sessions.RevokeUser(username)
	}
	return nil
}

// ScopedAPIKey is a bearer key with explicit permission scopes and expiration.
type ScopedAPIKey struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	OwnerUsername string    `json:"ownerUsername"`
	Role          rbac.Role `json:"role"`
	Scopes        []string  `json:"scopes"`
	KeyPrefix     string    `json:"keyPrefix"`
	KeyHash       string    `json:"-"`
	ExpiresAt     time.Time `json:"expiresAt"`
	Revoked       bool      `json:"revoked"`
	CreatedAt     time.Time `json:"createdAt"`
}

// APIKeyStore manages scoped API keys in memory and optionally persists them
// in PostgreSQL when dbstore is enabled.
type APIKeyStore struct {
	mu    sync.Mutex
	items map[string]*ScopedAPIKey // hash -> key
	db    *dbstore.Store
}

func NewAPIKeyStore(db *dbstore.Store) *APIKeyStore {
	return &APIKeyStore{items: make(map[string]*ScopedAPIKey), db: db}
}

func (store *APIKeyStore) Create(name, owner string, role rbac.Role, scopes []string, ttl time.Duration) (*ScopedAPIKey, string, error) {
	cleanName := strings.TrimSpace(name)
	if len(cleanName) < 2 || len(cleanName) > 80 {
		return nil, "", errors.New("API key name must be 2-80 characters")
	}
	if len(scopes) == 0 {
		return nil, "", errors.New("at least one permission scope is required")
	}
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	rawSecret, err := randomToken(24)
	if err != nil {
		return nil, "", err
	}
	rawKey := "dpkey_" + rawSecret
	keyHash := dbstore.HashToken(rawKey)
	now := time.Now().UTC()
	rec := &ScopedAPIKey{
		ID:            "key-" + rawSecret[:12],
		Name:          cleanName,
		OwnerUsername: owner,
		Role:          role,
		Scopes:        append([]string(nil), scopes...),
		KeyPrefix:     rawKey[:14],
		KeyHash:       keyHash,
		ExpiresAt:     now.Add(ttl),
		CreatedAt:     now,
	}

	if store.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		created, err := store.db.CreateAPIKey(ctx, dbstore.APIKeyRecord{
			Name:          rec.Name,
			OwnerUsername: rec.OwnerUsername,
			Role:          string(rec.Role),
			Scopes:        rec.Scopes,
			KeyPrefix:     rec.KeyPrefix,
			KeyHash:       rec.KeyHash,
			ExpiresAt:     rec.ExpiresAt,
		})
		if err != nil {
			return nil, "", err
		}
		rec.ID = created.ID
	}

	store.mu.Lock()
	store.items[keyHash] = rec
	store.mu.Unlock()
	copied := *rec
	return &copied, rawKey, nil
}

func (store *APIKeyStore) Resolve(rawKey string) (*ScopedAPIKey, error) {
	if !strings.HasPrefix(rawKey, "dpkey_") {
		return nil, ErrSessionNotFound
	}
	keyHash := dbstore.HashToken(rawKey)
	store.mu.Lock()
	rec, ok := store.items[keyHash]
	store.mu.Unlock()
	if ok {
		if rec.Revoked || time.Now().UTC().After(rec.ExpiresAt) {
			return nil, ErrSessionNotFound
		}
		copied := *rec
		return &copied, nil
	}
	if store.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		dbRec, err := store.db.ResolveAPIKey(ctx, rawKey)
		if err == nil {
			resolved := &ScopedAPIKey{
				ID:            dbRec.ID,
				Name:          dbRec.Name,
				OwnerUsername: dbRec.OwnerUsername,
				Role:          rbac.Role(dbRec.Role),
				Scopes:        dbRec.Scopes,
				KeyPrefix:     dbRec.KeyPrefix,
				KeyHash:       keyHash,
				ExpiresAt:     dbRec.ExpiresAt,
				Revoked:       dbRec.Revoked,
				CreatedAt:     dbRec.CreatedAt,
			}
			store.mu.Lock()
			store.items[keyHash] = resolved
			store.mu.Unlock()
			return resolved, nil
		}
	}
	return nil, ErrSessionNotFound
}

func (store *APIKeyStore) List() []*ScopedAPIKey {
	store.mu.Lock()
	defer store.mu.Unlock()
	out := make([]*ScopedAPIKey, 0, len(store.items))
	for _, k := range store.items {
		copied := *k
		out = append(out, &copied)
	}
	return out
}

func (store *APIKeyStore) Revoke(id string) bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	found := false
	for _, k := range store.items {
		if k.ID == id {
			k.Revoked = true
			found = true
		}
	}
	if store.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := store.db.RevokeAPIKey(ctx, "", id); err == nil {
			found = true
		}
	}
	return found
}

func (k *ScopedAPIKey) Allows(permission string) bool {
	if !rbac.HasPermission(k.Role, permission) {
		return false
	}
	for _, scope := range k.Scopes {
		if scope == "*" || scope == permission {
			return true
		}
	}
	return false
}

// isAdminIPAllowed checks whether clientAddr matches the configured admin IP allowlist.
// An empty allowlist allows any authenticated admin origin.
func isAdminIPAllowed(clientAddr string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	host := strings.TrimSpace(clientAddr)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	parsed := net.ParseIP(host)
	for _, entry := range allowlist {
		clean := strings.TrimSpace(entry)
		if clean == "" {
			continue
		}
		if clean == host {
			return true
		}
		if _, cidr, err := net.ParseCIDR(clean); err == nil && parsed != nil && cidr.Contains(parsed) {
			return true
		}
	}
	return false
}

// RequestIdentity represents the authenticated actor resolved from either a
// live login session or a scoped API key.
type RequestIdentity struct {
	Username  string
	Role      rbac.Role
	ExpiresAt time.Time
	IsAPIKey  bool
}

func (state *ServerState) resolveIdentity(r *http.Request) (*RequestIdentity, error) {
	token := bearerToken(r)
	if token == "" {
		return nil, ErrSessionNotFound
	}
	if strings.HasPrefix(token, "dpkey_") {
		if state.APIKeys == nil {
			return nil, ErrSessionNotFound
		}
		k, err := state.APIKeys.Resolve(token)
		if err != nil {
			return nil, err
		}
		return &RequestIdentity{
			Username:  k.OwnerUsername,
			Role:      k.Role,
			ExpiresAt: k.ExpiresAt,
			IsAPIKey:  true,
		}, nil
	}
	if state.Sessions == nil {
		return nil, ErrSessionNotFound
	}
	s, err := state.Sessions.Resolve(token)
	if err != nil {
		return nil, err
	}
	return &RequestIdentity{
		Username:  s.Username,
		Role:      s.Role,
		ExpiresAt: s.ExpiresAt,
		IsAPIKey:  false,
	}, nil
}

func (state *ServerState) actorUsername(r *http.Request) string {
	id, err := state.resolveIdentity(r)
	if err != nil {
		return "anonymous"
	}
	if id.IsAPIKey {
		return "apikey:" + id.Username
	}
	return id.Username
}
