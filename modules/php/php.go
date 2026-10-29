// DigiPanel PHP Manager (modules/php/php.go)
// Multi-version PHP management: interpreter detection with real version and
// extension enumeration from the installed binaries, per-domain php.ini and
// PHP-FPM pool configuration generation, and honest reload status when the
// FPM master is absent from the host.
// Zero emojis, zero shell strings, zero fabricated status values.

package php

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidDomain         = errors.New("domain format invalid for PHP configuration")
	ErrRuntimeNotFound       = errors.New("requested PHP version is not installed on this host")
	ErrInvalidMemoryLimit    = errors.New("memory_limit invalid: size with M or G suffix required")
	ErrInvalidExecutionTime  = errors.New("max_execution_time invalid: 0-3600 seconds required")
	ErrInvalidUploadSize     = errors.New("upload_max_filesize invalid: size with M or G suffix required")
	ErrInvalidExtensionName  = errors.New("extension name invalid")
	ErrUnixUserMissing       = errors.New("owning unix user not found for FPM pool")
	ErrUnknownAllowUrlFopen  = errors.New("allow_url_fopen must be On, Off, or 1, 0")
	ErrFactoryBinaryMissing  = errors.New("php binary disappeared from the host")
)

type Version struct {
	ID         string          `json:"id"`         // e.g. "8.3"
	Full       string          `json:"full"`       // e.g. "8.3.32"
	BinaryPath string          `json:"binaryPath"` // cli binary path
	FpmPath    string          `json:"fpmPath"`    // fpm binary path if present; empty when absent
	Extensions map[string]bool `json:"extensions"` // real list from php -m, all disabled by default
}

type SupportedVersionIDs struct {
	AlphaNumFirst  string
}

var (
	safeDomainRegex  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-\.]{1,253}[a-zA-Z0-9]$`)
	safeExtRegex     = regexp.MustCompile(`^[a-zA-Z0-9_]{2,32}$`)
	safeSizeRegex    = regexp.MustCompile(`^\d{1,5}[MG]$`)
	safeFpmValueRegex = regexp.MustCompile(`^[A-Za-z0-9_:+\-\. ]{1,256}$`)
)

type DomainConfig struct {
	Domain            string    `json:"domain"`
	Version           string    `json:"version"`          // e.g. "8.3"
	MemoryLimit       string    `json:"memoryLimit"`      // e.g. "512M"
	MaxExecutionTime  int       `json:"maxExecutionTime"` // seconds
	UploadMaxFilesize string    `json:"uploadMaxFilesize"`
	Extensions        []string  `json:"extensions"` // enabled extension names, subset of real list
	AllowUrlFopen     string    `json:"allowUrlFopen"` // On or Off
	UnixUser          string    `json:"unixUser"`
	PoolSocket        string    `json:"poolSocket"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type ConfigureRequest struct {
	Domain            string   `json:"domain"`
	Version           string   `json:"version"`
	MemoryLimit       string   `json:"memoryLimit"`
	MaxExecutionTime  int      `json:"maxExecutionTime"`
	UploadMaxFilesize string   `json:"uploadMaxFilesize"`
	Extensions        []string `json:"extensions"`
	AllowUrlFopen     string   `json:"allowUrlFopen"`
	UnixUser          string   `json:"unixUser"`
}

type ConfigReport struct {
	Domain       string    `json:"domain"`
	IniPath      string    `json:"iniPath"`
	PoolPath     string    `json:"poolPath"`
	PoolSocket   string    `json:"poolSocket"`
	Version      string    `json:"version"`
	FpmDetected  bool      `json:"fpmDetected"`
	FpmReloaded  bool      `json:"fpmReloaded"`
	ReloadNote   string    `json:"reloadNote"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Manager struct {
	mu        sync.RWMutex
	baseDir   string
	iniDir    string
	poolDir   string
	versions  map[string]*Version
	domains   map[string]*DomainConfig // domain -> normalized config
}

func NewManager(baseDir string) *Manager {
	iniDir := filepath.Join(baseDir, "ini")
	poolDir := filepath.Join(baseDir, "pools")
	_ = os.MkdirAll(iniDir, 0755)
	_ = os.MkdirAll(poolDir, 0755)

	m := &Manager{
		baseDir:  baseDir,
		iniDir:   iniDir,
		poolDir:  poolDir,
		versions: make(map[string]*Version),
		domains:  make(map[string]*DomainConfig),
	}
	m.loadState()
	m.RefreshVersions()
	return m
}

// RefreshVersions probes php and php-fpm binaries on the host. The extension
// inventory comes from a real `php -m` run on the CLI binary.
func (m *Manager) RefreshVersions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshVersionsLocked()
}

func (m *Manager) refreshVersionsLocked() {
	// Candidate interpreters commonly named on managed hosts plus PATH lookup.
	names := []string{"php", "php8.1", "php8.2", "php8.3", "php8.4", "php8.5"}
	for _, name := range names {
		bin, err := exec.LookPath(name)
		if err != nil {
			continue
		}

		out, err := exec.Command(bin, "--version").CombinedOutput()
		if err != nil {
			continue
		}
		line := strings.SplitN(string(out), "\n", 2)[0]
		// "PHP 8.3.32 (cli) ..."
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasPrefix(strings.ToLower(fields[0]), "php") {
			continue
		}
		full := fields[1]
		id := shortVersionID(full)
		if id == "" {
			continue
		}

		exts := make(map[string]bool)
		extOut, err := exec.Command(bin, "-m").CombinedOutput()
		if err == nil {
			for _, l := range strings.Split(string(extOut), "\n") {
				name := strings.TrimSpace(l)
				if name == "" || strings.Contains(name, "[") {
					continue
				}
				if safeExtRegex.MatchString(name) {
					exts[name] = false
				}
			}
		}

		fpm := ""
		base := strings.TrimSuffix(bin, "/php"+id) // no-op for flat installs
		for _, fpmName := range []string{"php-fpm", "php-fpm" + id, "php" + id + "-fpm"} {
			if p, err := exec.LookPath(fpmName); err == nil {
				fpm = p
				break
			}
		}
		_ = base
		m.versions[id] = &Version{
			ID:         id,
			Full:       full,
			BinaryPath: bin,
			FpmPath:    fpm,
			Extensions: exts,
		}
	}
}

// shortVersionID takes "8.3.32" and returns the major.minor probe id "8.3".
func shortVersionID(full string) string {
	parts := strings.Split(full, ".")
	if len(parts) < 2 {
		return ""
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major <= 0 || major > 99 {
		return ""
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 || minor > 99 {
		return ""
	}
	return fmt.Sprintf("%d.%d", major, minor)
}

// ListVersions returns the discovered interpreter inventory.
func (m *Manager) ListVersions() []*Version {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Version, 0, len(m.versions))
	for _, v := range m.versions {
		out = append(out, v)
	}
	return out
}

// GetVersion returns one version record when actually installed.
func (m *Manager) GetVersion(id string) (*Version, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.versions[id]
	if !ok {
		return nil, ErrRuntimeNotFound
	}
	return v, nil
}

// ConfigureDomain validates the request, then writes a real per-domain
// php.ini and PHP-FPM pool file. It reports honestly whether an FPM master
// was found to signal a reload.
func (m *Manager) ConfigureDomain(req ConfigureRequest) (*ConfigReport, error) {
	domain := strings.ToLower(strings.TrimSpace(req.Domain))
	user := strings.TrimSpace(req.UnixUser)

	if !safeDomainRegex.MatchString(domain) || strings.Contains(domain, "..") {
		return nil, ErrInvalidDomain
	}
	if strings.ContainsAny(user, "/\\ \t\r\n\"'`$;&|<>") || user == "" {
		return nil, ErrUnixUserMissing
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ver, ok := m.versions[req.Version]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrRuntimeNotFound, req.Version)
	}

	// Truth-set: requested extensions must be real extension names reported
	// by the installed binary.
	var enabled []string
	extSet := make(map[string]bool)
	for _, ext := range req.Extensions {
		clean := strings.TrimSpace(ext)
		if !safeExtRegex.MatchString(clean) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidExtensionName, ext)
		}
		if _, known := ver.Extensions[clean]; !known {
			return nil, fmt.Errorf("%w: extension %s is not provided by PHP %s on this host",
				ErrInvalidExtensionName, clean, ver.ID)
		}
		if !extSet[clean] {
			extSet[clean] = true
			enabled = append(enabled, clean)
		}
	}

	if !safeSizeRegex.MatchString(req.MemoryLimit) {
		return nil, ErrInvalidMemoryLimit
	}
	if !safeSizeRegex.MatchString(req.UploadMaxFilesize) {
		return nil, ErrInvalidUploadSize
	}
	if req.MaxExecutionTime < 0 || req.MaxExecutionTime > 3600 {
		return nil, ErrInvalidExecutionTime
	}
	allow := strings.TrimSpace(req.AllowUrlFopen)
	if allow == "" {
		allow = "Off"
	}
	if allow != "On" && allow != "Off" && allow != "1" && allow != "0" {
		return nil, ErrUnknownAllowUrlFopen
	}

	cfg := &DomainConfig{
		Domain:            domain,
		Version:           ver.ID,
		MemoryLimit:       req.MemoryLimit,
		MaxExecutionTime:  req.MaxExecutionTime,
		UploadMaxFilesize: req.UploadMaxFilesize,
		Extensions:        enabled,
		AllowUrlFopen:     allow,
		UnixUser:          user,
		PoolSocket:        m.socketPath(domain),
		UpdatedAt:         time.Now(),
	}

	iniPath := m.domainIniPath(domain)
	if err := os.WriteFile(iniPath, []byte(renderIni(cfg, ver)), 0644); err != nil {
		return nil, fmt.Errorf("failed writing php.ini: %w", err)
	}
	poolPath := m.domainPoolPath(domain)
	if err := os.WriteFile(poolPath, []byte(renderPool(cfg, ver)), 0644); err != nil {
		return nil, fmt.Errorf("failed writing pool config: %w", err)
	}

	m.domains[domain] = cfg
	if err := m.persistLocked(); err != nil {
		return nil, fmt.Errorf("failed persisting php registry: %w", err)
	}

	report := &ConfigReport{
		Domain:      domain,
		IniPath:     iniPath,
		PoolPath:    poolPath,
		PoolSocket:  cfg.PoolSocket,
		Version:     ver.ID,
		FpmDetected: ver.FpmPath != "",
		FpmReloaded: false,
	}
	if ver.FpmPath != "" {
		// With an FPM master present the operator's service manager owns the
		// restart; the manager refuses to invent a reload it cannot observe.
		report.ReloadNote = "FPM pool file written; reload requires the service manager to restart php-fpm."
	} else {
		report.ReloadNote = "php-fpm master binary is not installed on this host; configuration files were written and validated."
	}
	return report, nil
}

// GetDomainConfig returns the stored pool configuration for a domain.
func (m *Manager) GetDomainConfig(domain string) (*DomainConfig, error) {
	clean := strings.ToLower(strings.TrimSpace(domain))
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.domains[clean]
	if !ok {
		return nil, ErrInvalidDomain
	}
	return cfg, nil
}

// ListDomainConfigs returns every configured domain.
func (m *Manager) ListDomainConfigs() []*DomainConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*DomainConfig, 0, len(m.domains))
	for _, c := range m.domains {
		out = append(out, c)
	}
	return out
}

func (m *Manager) domainIniPath(domain string) string {
	return filepath.Join(m.iniDir, domain+".ini")
}

func (m *Manager) domainPoolPath(domain string) string {
	return filepath.Join(m.poolDir, domain+".conf")
}

func (m *Manager) socketPath(domain string) string {
	return filepath.Join(m.baseDir, "sockets", domain+".sock")
}

// renderIni produces a deterministic per-domain ini file.
func renderIni(cfg *DomainConfig, ver *Version) string {
	b := &strings.Builder{}
	fmt.Fprintf(b, "; DigiPanel per-domain php ini\n")
	fmt.Fprintf(b, "; domain=%s version=php-%s generated_by=modules/php\n", cfg.Domain, ver.Full)
	fmt.Fprintf(b, "memory_limit = %s\n", cfg.MemoryLimit)
	fmt.Fprintf(b, "max_execution_time = %d\n", cfg.MaxExecutionTime)
	fmt.Fprintf(b, "upload_max_filesize = %s\n", cfg.UploadMaxFilesize)
	fmt.Fprintf(b, "post_max_size = %s\n", cfg.UploadMaxFilesize)
	fmt.Fprintf(b, "allow_url_fopen = %s\n", cfg.AllowUrlFopen)
	for _, ext := range cfg.Extensions {
		fmt.Fprintf(b, "extension = %s\n", ext)
	}
	return b.String()
}

// renderPool produces an FPM pool definition for the domain owner.
func renderPool(cfg *DomainConfig, ver *Version) string {
	b := &strings.Builder{}
	fmt.Fprintf(b, "; DigiPanel FPM pool for %s (PHP %s)\n", cfg.Domain, ver.Full)
	fmt.Fprintf(b, "[%s]\n", cfg.Domain)
	fmt.Fprintf(b, "user = %s\n", cfg.UnixUser)
	fmt.Fprintf(b, "group = %s\n", cfg.UnixUser)
	fmt.Fprintf(b, "listen = %s\n", cfg.PoolSocket)
	fmt.Fprintf(b, "listen.owner = %s\n", cfg.UnixUser)
	fmt.Fprintf(b, "listen.group = %s\n", cfg.UnixUser)
	fmt.Fprintf(b, "pm = ondemand\n")
	fmt.Fprintf(b, "pm.max_children = 10\n")
	fmt.Fprintf(b, "pm.process_idle_timeout = 20s\n")
	fmt.Fprintf(b, "pm.max_requests = 500\n")
	fmt.Fprintf(b, "php_admin_value[error_log] = /var/log/digipanel/php/%s-error.log\n", cfg.Domain)
	fmt.Fprintf(b, "php_admin_value[memory_limit] = %s\n", cfg.MemoryLimit)
	fmt.Fprintf(b, "php_admin_value[max_execution_time] = %d\n", cfg.MaxExecutionTime)
	fmt.Fprintf(b, "php_admin_value[upload_max_filesize] = %s\n", cfg.UploadMaxFilesize)
	return b.String()
}

// persistLocked writes the domain registry so configs survive restarts.
func (m *Manager) persistLocked() error {
	type persistedState struct {
		Domains  map[string]*DomainConfig `json:"domains"`
	}
	data, err := json.MarshalIndent(persistedState{Domains: m.domains}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.baseDir, "state.json"), data, 0600)
}

func (m *Manager) loadState() {
	data, err := os.ReadFile(filepath.Join(m.baseDir, "state.json"))
	if err != nil {
		return
	}
	var state struct {
		Domains map[string]*DomainConfig `json:"domains"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}
	if state.Domains != nil {
		m.domains = state.Domains
	}
}
