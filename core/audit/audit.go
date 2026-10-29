// DigiPanel Cryptographic Chained Audit Log Engine (core/audit/audit.go)
// Features: Append-only ledger with SHA-256 hash chaining and tamper-evident verification.
// Zero emojis, strictly production grade.

package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

const RootGenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

var (
	ErrTamperedEntryHash = errors.New("audit log entry hash mismatch: record was tampered with")
	ErrBrokenHashChain   = errors.New("audit log prev_hash mismatch: chain integrity broken")
)

type LogEntry struct {
	ID           int64     `json:"id"`
	Actor        string    `json:"actor"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resourceType"`
	ResourceID   string    `json:"resourceId"`
	PayloadJSON  string    `json:"payloadJson"`
	IPAddress    string    `json:"ipAddress"`
	PrevHash     string    `json:"prevHash"`
	EntryHash    string    `json:"entryHash"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Logger struct {
	mu      sync.RWMutex
	entries []LogEntry
}

func NewLogger() *Logger {
	l := &Logger{
		entries: make([]LogEntry, 0),
	}
	// Bootstrap with genesis entry
	genesis := LogEntry{
		ID:           1,
		Actor:        "system",
		Action:       "audit.genesis",
		ResourceType: "ledger",
		ResourceID:   "root-0",
		PayloadJSON:  `{"status":"genesis"}`,
		IPAddress:    "127.0.0.1",
		PrevHash:     RootGenesisHash,
		CreatedAt:    time.Now().UTC(),
	}
	genesis.EntryHash = ComputeHash(genesis.PrevHash, genesis.Actor, genesis.Action, genesis.ResourceType, genesis.ResourceID, genesis.PayloadJSON, genesis.CreatedAt)
	l.entries = append(l.entries, genesis)
	return l
}

// ComputeHash calculates the deterministic SHA-256 hash of an audit record
func ComputeHash(prevHash, actor, action, resType, resID, payload string, t time.Time) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d", prevHash, actor, action, resType, resID, payload, t.UnixNano())
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// Append records a new immutable entry chained to the previous hash
func (l *Logger) Append(actor, action, resType, resID, payload, ip string) LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	lastHash := RootGenesisHash
	if len(l.entries) > 0 {
		lastHash = l.entries[len(l.entries)-1].EntryHash
	}

	now := time.Now().UTC()
	entry := LogEntry{
		ID:           int64(len(l.entries) + 1),
		Actor:        actor,
		Action:       action,
		ResourceType: resType,
		ResourceID:   resID,
		PayloadJSON:  payload,
		IPAddress:    ip,
		PrevHash:     lastHash,
		CreatedAt:    now,
	}
	entry.EntryHash = ComputeHash(lastHash, actor, action, resType, resID, payload, now)
	l.entries = append(l.entries, entry)
	return entry
}

// VerifyIntegrity walks through all entries and verifies both hash authenticity and chain links
func (l *Logger) VerifyIntegrity() (bool, int, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for i := 0; i < len(l.entries); i++ {
		entry := l.entries[i]

		// Check link to previous entry
		if i > 0 {
			if entry.PrevHash != l.entries[i-1].EntryHash {
				return false, i, fmt.Errorf("%w at index %d (expected prev_hash %s, got %s)",
					ErrBrokenHashChain, i, l.entries[i-1].EntryHash, entry.PrevHash)
			}
		} else {
			if entry.PrevHash != RootGenesisHash {
				return false, 0, fmt.Errorf("%w at genesis entry", ErrBrokenHashChain)
			}
		}

		// Recompute hash
		expectedHash := ComputeHash(entry.PrevHash, entry.Actor, entry.Action, entry.ResourceType, entry.ResourceID, entry.PayloadJSON, entry.CreatedAt)
		if entry.EntryHash != expectedHash {
			return false, i, fmt.Errorf("%w at index %d: entry modified", ErrTamperedEntryHash, i)
		}
	}

	return true, len(l.entries), nil
}

// GetEntries returns a slice copy of audit entries
func (l *Logger) GetEntries() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	copied := make([]LogEntry, len(l.entries))
	copy(copied, l.entries)
	return copied
}

// TamperEntryForTesting alters an entry to test tamper detection mechanisms
func (l *Logger) TamperEntryForTesting(index int, newPayload string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if index >= 0 && index < len(l.entries) {
		l.entries[index].PayloadJSON = newPayload
	}
}
