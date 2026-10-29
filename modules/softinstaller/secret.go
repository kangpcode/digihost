// DigiPanel installer secret vault (modules/softinstaller/secret.go)
// Database administrator credentials used for clean uninstall are encrypted at
// rest with AES-256-GCM. The key lives beside the installer state with mode
// 0600 and is generated on first use. Plaintext never reaches logs, argv, or
// the state document.
// Zero emojis, zero shell strings.

package softinstaller

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrSecretUnavailable = errors.New("installer secret vault is unavailable")
	ErrSecretCorrupt     = errors.New("installer secret vault entry is corrupt")
)

// dbAdminRecord is the encrypted payload retained per instance.
type dbAdminRecord struct {
	Engine      string `json:"engine"`
	URL         string `json:"url"`
	AdminUser   string `json:"adminUser"`
	AdminPass   string `json:"adminPass"`
	MySQLBinary string `json:"mysqlBinary,omitempty"`
	PsqlBinary  string `json:"psqlBinary,omitempty"`
}

func (m *Manager) secretKeyPath() string {
	return filepath.Join(m.cfg.StateDir, "secret.key")
}

// loadOrCreateKey returns the 32-byte AES key, creating it with 0600 when the
// state directory has no key yet.
func (m *Manager) loadOrCreateKey() ([]byte, error) {
	path := m.secretKeyPath()
	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) != 32 {
			return nil, fmt.Errorf("%w: key length %d", ErrSecretCorrupt, len(data))
		}
		return data, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return os.ReadFile(path)
		}
		return nil, err
	}
	if _, err := f.Write(key); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return key, nil
}

func (m *Manager) seal(plaintext []byte) (string, error) {
	key, err := m.loadOrCreateKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (m *Manager) open(encoded string) ([]byte, error) {
	key, err := m.loadOrCreateKey()
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSecretCorrupt, err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("%w: truncated ciphertext", ErrSecretCorrupt)
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: authentication failed", ErrSecretCorrupt)
	}
	return plaintext, nil
}

// rememberDBAdmin stores the administrator credentials required to drop the
// instance database during uninstall.
func (m *Manager) rememberDBAdmin(instID string, cfg InstallRequest) error {
	record := dbAdminRecord{
		Engine:      cfg.DBEngine,
		URL:         cfg.DatabaseURL,
		AdminUser:   cfg.DBAdminUser,
		AdminPass:   cfg.DBAdminPass,
		MySQLBinary: cfg.MySQLBinary,
		PsqlBinary:  cfg.PsqlBinary,
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	sealed, err := m.seal(data)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.secrets[instID] = sealed
	m.mu.Unlock()
	return m.persist()
}

// dbAdminConfigFor returns administrator credentials for an instance. Explicit
// credentials in req take precedence; otherwise the encrypted record created at
// install time is used.
func (m *Manager) dbAdminConfigFor(instID string, req InstallRequest) (InstallRequest, error) {
	out := req
	if out.DBAdminUser != "" && out.DBAdminPass != "" {
		return out, nil
	}
	m.mu.Lock()
	sealed, ok := m.secrets[instID]
	m.mu.Unlock()
	if !ok || sealed == "" {
		return out, fmt.Errorf("%w: no administrator credential recorded for %s", ErrSecretUnavailable, instID)
	}
	plaintext, err := m.open(sealed)
	if err != nil {
		return out, err
	}
	var record dbAdminRecord
	if err := json.Unmarshal(plaintext, &record); err != nil {
		return out, fmt.Errorf("%w: %v", ErrSecretCorrupt, err)
	}
	if out.DBEngine == "" {
		out.DBEngine = record.Engine
	}
	if out.DatabaseURL == "" {
		out.DatabaseURL = record.URL
	}
	if out.DBAdminUser == "" {
		out.DBAdminUser = record.AdminUser
	}
	if out.DBAdminPass == "" {
		out.DBAdminPass = record.AdminPass
	}
	if out.MySQLBinary == "" {
		out.MySQLBinary = record.MySQLBinary
	}
	if out.PsqlBinary == "" {
		out.PsqlBinary = record.PsqlBinary
	}
	if out.DBAdminUser == "" || out.DBAdminPass == "" {
		return out, fmt.Errorf("%w: recorded credential is empty", ErrSecretUnavailable)
	}
	return out, nil
}

// forgetDBAdmin removes the encrypted credential for an instance.
func (m *Manager) forgetDBAdmin(instID string) {
	m.mu.Lock()
	delete(m.secrets, instID)
	m.mu.Unlock()
}
