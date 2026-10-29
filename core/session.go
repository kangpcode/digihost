// DigiPanel Session, Login Throttle, and Bootstrap Credentials (core/session.go)
// One session implementation for the whole panel: opaque random tokens bound to
// a user, a role, and an expiry. No client supplied role is ever trusted, and no
// credential is hardcoded in the binary. The bootstrap administrator password
// comes from the environment or is generated once and written to a 0600 file.
// Zero emojis, strictly production grade.

package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"digipanel/core/audit"
	"digipanel/core/auth"
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
