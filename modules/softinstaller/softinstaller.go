// DigiPanel Application Marketplace Installer (modules/softinstaller)
// Declarative multi-application catalogue with verified downloads, sandboxed
// extraction, real database provisioning, per-application configuration
// renderers, allowlisted post-install steps, cloning, staging, updates, and
// clean removal. Zero shell strings, zero mock statuses, zero emojis.
//
// The manager never runs as root. Target paths are confined to the configured
// installation root, and every reported status reflects a real filesystem or
// database operation.

package softinstaller

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	ErrPackageNotFound    = errors.New("package not present in the installer catalogue")
	ErrInvalidInput       = errors.New("installer request rejected by validation")
	ErrInvalidTarget      = errors.New("target path invalid or outside the managed installation root")
	ErrTargetOccupied     = errors.New("target path already contains files")
	ErrChecksumUnverified = errors.New("downloaded archive checksum could not be verified")
	ErrDownloadFailed     = errors.New("package archive download failed")
	ErrExtractionFailed   = errors.New("archive extraction failed")
	ErrDatabaseRequired   = errors.New("package requires a database; provisioner credentials not configured")
	ErrDatabaseProvision  = errors.New("database provisioning failed")
	ErrInstanceNotFound   = errors.New("installed application instance not found")
	ErrTargetNotWritable  = errors.New("target parent directory is not writable by this process")
	ErrRequirements       = errors.New("host does not satisfy package requirements")
	ErrCloneUnsupported   = errors.New("cloning a database-backed package requires a supported config renderer")
)

// DatabaseDefaults let the core supply the panel-managed database identity
// without exposing it to the browser.
type DatabaseDefaults struct {
	Engine        string
	URL           string
	AdminUser     string
	AdminPassword string
	MySQLBinary   string
	PsqlBinary    string
}

// Config configures a Manager.
type Config struct {
	CatalogDir             string
	BaseInstallDir         string
	StateDir               string
	RequireSignedManifests bool
	ManifestPublicKeyPath  string
	HTTPClient             *http.Client
	DefaultDB              DatabaseDefaults
}

// InstallRequest is one installation order.
type InstallRequest struct {
	PackageID     string `json:"packageId"`
	TargetPath    string `json:"targetPath"`
	Domain        string `json:"domain"`
	DomainPath    string `json:"domainPath"`
	SiteURL       string `json:"siteURL"`
	Title         string `json:"title"`
	AdminUser     string `json:"adminUser"`
	AdminPassword string `json:"adminPassword"`
	AdminEmail    string `json:"adminEmail"`
	DatabaseURL   string `json:"databaseURL"`
	DBEngine      string `json:"dbEngine"`
	DBAdminUser   string `json:"dbAdminUser"`
	DBAdminPass   string `json:"dbAdminPass"`
	MySQLBinary   string `json:"mysqlBinary"`
	PsqlBinary    string `json:"psqlBinary"`
	Port          int    `json:"port"`
	Environment   string `json:"environment"`
	TenantID      string `json:"tenantId"`
	Username      string `json:"username"`
	ShareDatabase bool   `json:"shareDatabase"`
}

// InstallResult returns the instance plus the one-time administrative secret.
type InstallResult struct {
	Instance       *Instance      `json:"instance"`
	AdminPassword  string         `json:"adminPassword,omitempty"`
	AdminPath      string         `json:"adminPath,omitempty"`
	CompletionMode string         `json:"completionMode,omitempty"`
	Database       *DBCredentials `json:"database,omitempty"`
	NextStep       string         `json:"nextStep,omitempty"`
}

// Instance is the persisted registry record.
type Instance struct {
	ID              string    `json:"id"`
	PackageID       string    `json:"packageId"`
	PackageName     string    `json:"packageName"`
	PackageVersion  string    `json:"packageVersion"`
	TenantID        string    `json:"tenantId,omitempty"`
	Username        string    `json:"username,omitempty"`
	Title           string    `json:"title"`
	Domain          string    `json:"domain"`
	DomainPath      string    `json:"domainPath"`
	SiteURL         string    `json:"siteURL"`
	Path            string    `json:"path"`
	DataPath        string    `json:"dataPath,omitempty"`
	Environment     string    `json:"environment"`
	Status          string    `json:"status"`
	FailureReason   string    `json:"failureReason,omitempty"`
	DBEngine        string    `json:"dbEngine,omitempty"`
	DBName          string    `json:"dbName,omitempty"`
	DBUser          string    `json:"dbUser,omitempty"`
	DBHost          string    `json:"dbHost,omitempty"`
	DBPort          string    `json:"dbPort,omitempty"`
	AdminUser       string    `json:"adminUser,omitempty"`
	AdminPath       string    `json:"adminPath,omitempty"`
	AdminEmail      string    `json:"adminEmail,omitempty"`
	Port            int       `json:"port,omitempty"`
	NextStep        string    `json:"nextStep,omitempty"`
	InstalledSHA256 string    `json:"installedSHA256,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type persistedState struct {
	Instances map[string]*Instance `json:"instances"`
	Secrets   map[string]string    `json:"secrets"`
	NextID    int                  `json:"nextId"`
}

// Manager owns the catalogue and instance registry.
type Manager struct {
	mu         sync.Mutex
	cfg        Config
	pkgs       map[string]*Package
	instances  map[string]*Instance
	secrets    map[string]string
	nextID     atomic.Int64
	httpClient *http.Client
	now        func() time.Time
}

// NewManager loads the catalogue and persisted registry.
func NewManager(cfg Config) (*Manager, error) {
	if cfg.CatalogDir == "" {
		return nil, fmt.Errorf("%w: catalog directory is required", ErrInvalidInput)
	}
	if cfg.BaseInstallDir == "" {
		return nil, fmt.Errorf("%w: base install directory is required", ErrInvalidInput)
	}
	if cfg.StateDir == "" {
		cfg.StateDir = cfg.CatalogDir
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Minute}
	}
	for _, dir := range []string{cfg.CatalogDir, cfg.BaseInstallDir, cfg.StateDir, filepath.Join(cfg.StateDir, "logs"), filepath.Join(cfg.StateDir, "backups")} {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("create %s: %w", dir, err)
		}
	}
	m := &Manager{
		cfg:        cfg,
		pkgs:       make(map[string]*Package),
		instances:  make(map[string]*Instance),
		secrets:    make(map[string]string),
		httpClient: cfg.HTTPClient,
		now:        time.Now,
	}
	if err := m.LoadCatalog(); err != nil {
		return nil, err
	}
	if err := m.loadState(); err != nil {
		return nil, err
	}
	return m, nil
}

// LoadCatalog reloads every manifest from disk. Invalid manifests are returned
// as errors; they are never silently ignored.
func (m *Manager) LoadCatalog() error {
	var publicKey ed25519.PublicKey
	if m.cfg.RequireSignedManifests {
		if m.cfg.ManifestPublicKeyPath == "" {
			return fmt.Errorf("%w: signature verification enabled without a public key", ErrManifestUnsigned)
		}
		key, err := LoadPublicKey(m.cfg.ManifestPublicKeyPath)
		if err != nil {
			return fmt.Errorf("load manifest public key: %w", err)
		}
		publicKey = key
	}
	entries, err := os.ReadDir(m.cfg.CatalogDir)
	if err != nil {
		return fmt.Errorf("read catalogue: %w", err)
	}
	loaded := make(map[string]*Package)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		if name == "state.json" || name == "public.key" {
			continue
		}
		pkg, err := loadManifestFile(filepath.Join(m.cfg.CatalogDir, name), m.cfg.RequireSignedManifests, publicKey)
		if err != nil {
			return err
		}
		if _, exists := loaded[pkg.ID]; exists {
			return fmt.Errorf("%w: duplicate package id %q", ErrManifestInvalid, pkg.ID)
		}
		loaded[pkg.ID] = pkg
	}
	m.mu.Lock()
	m.pkgs = loaded
	m.mu.Unlock()
	return nil
}

// Catalogue returns all manifests sorted by display name.
func (m *Manager) Catalogue() []*Package {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Package, 0, len(m.pkgs))
	for _, pkg := range m.pkgs {
		out = append(out, pkg)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// GetPackage returns one catalogue entry.
func (m *Manager) GetPackage(id string) (*Package, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pkg, ok := m.pkgs[id]
	if !ok {
		return nil, ErrPackageNotFound
	}
	return pkg, nil
}

// Instances returns the registry sorted by creation time, newest first.
func (m *Manager) Instances() []*Instance {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		out = append(out, inst)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// GetInstance returns one record.
func (m *Manager) GetInstance(id string) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inst, ok := m.instances[id]
	if !ok {
		return nil, ErrInstanceNotFound
	}
	return inst, nil
}

// --- installation ---

// Install provisions one application end-to-end. Any failure marks the record
// failed, removes partially written files, and drops a database created by the
// same attempt. No success is reported without a real filesystem result.
func (m *Manager) Install(req InstallRequest) (*InstallResult, error) {
	pkg, err := m.GetPackage(req.PackageID)
	if err != nil {
		return nil, err
	}
	if req.Environment == "" {
		req.Environment = "production"
	}
	if req.Environment != "production" && req.Environment != "staging" {
		return nil, fmt.Errorf("%w: environment must be production or staging", ErrInvalidInput)
	}
	if req.DomainPath != "" {
		req.DomainPath = strings.Trim(req.DomainPath, "/")
	}
	if req.SiteURL == "" && req.Domain != "" {
		req.SiteURL = "https://" + req.Domain
		if req.DomainPath != "" {
			req.SiteURL += "/" + req.DomainPath
		}
	}
	if err := validateInstallRequest(pkg, req); err != nil {
		return nil, err
	}

	id, ordinal := m.nextInstanceID()
	if req.TargetPath == "" {
		slug := strings.Trim(req.DomainPath, "/")
		if slug == "" {
			slug = pkg.ID
		}
		req.TargetPath = filepath.Join(m.cfg.BaseInstallDir, fmt.Sprintf("%s-%s", slug, id))
	}
	target, err := m.confineTarget(req.TargetPath)
	if err != nil {
		return nil, err
	}
	if err := ensureWritableParent(target); err != nil {
		return nil, err
	}
	if err := ensureEmptyTarget(target); err != nil {
		return nil, err
	}
	dataPath := ""
	if pkg.DataDirName != "" {
		dataPath, err = m.confineTarget(target + "-" + pkg.DataDirName)
		if err != nil {
			return nil, err
		}
		if err := ensureEmptyTarget(dataPath); err != nil {
			return nil, err
		}
	}

	if req.AdminUser == "" {
		req.AdminUser = "admin"
	}
	if req.AdminPassword == "" {
		req.AdminPassword, err = generateAdminPassword()
		if err != nil {
			return nil, err
		}
	}
	if req.Title == "" {
		req.Title = pkg.Name + " on " + req.Domain
	}
	if req.AdminEmail == "" {
		req.AdminEmail = "admin@" + req.Domain
	}
	if req.Port <= 0 {
		req.Port = 8080
	}

	inst := &Instance{
		ID:             id,
		PackageID:      pkg.ID,
		PackageName:    pkg.Name,
		PackageVersion: pkg.LatestVersion,
		TenantID:       req.TenantID,
		Username:       req.Username,
		Title:          req.Title,
		Domain:         req.Domain,
		DomainPath:     req.DomainPath,
		SiteURL:        req.SiteURL,
		Path:           target,
		DataPath:       dataPath,
		Environment:    req.Environment,
		Status:         "installing",
		AdminUser:      req.AdminUser,
		AdminPath:      pkg.AdminPath,
		AdminEmail:     req.AdminEmail,
		Port:           req.Port,
		CreatedAt:      m.now().UTC(),
		UpdatedAt:      m.now().UTC(),
	}
	if pkg.RequiresDB {
		inst.DBEngine = pkg.DBEngine
		inst.DBName = fmt.Sprintf("dp_%s_%04d", dbPrefix(pkg.ID), ordinal)
		inst.DBUser = inst.DBName
	}

	m.mu.Lock()
	m.instances[inst.ID] = inst
	m.mu.Unlock()
	_ = m.persist()
	m.logf(inst.ID, "installation started: %s %s", pkg.Name, pkg.LatestVersion)

	var db *DBCredentials
	var engine DatabaseEngine
	stage := target + ".digistage-" + inst.ID
	fail := func(cause error) (*InstallResult, error) {
		_ = os.RemoveAll(stage)
		_ = os.RemoveAll(target)
		if dataPath != "" {
			_ = os.RemoveAll(dataPath)
		}
		if engine != nil && db != nil {
			dropCtx := *db
			_ = engine.DropDatabase(dropCtx)
		}
		inst.Status = "failed"
		inst.FailureReason = cause.Error()
		inst.UpdatedAt = m.now().UTC()
		_ = m.persist()
		m.logf(inst.ID, "installation failed: %v", cause)
		return nil, cause
	}

	if err := m.checkRequirements(pkg, target); err != nil {
		return fail(err)
	}

	staged, digest, err := m.preparePackage(pkg, req, stage)
	if err != nil {
		return fail(err)
	}
	inst.InstalledSHA256 = digest

	if err := activateStage(staged, target); err != nil {
		return fail(fmt.Errorf("activate staged files: %w", err))
	}
	if dataPath != "" {
		if err := os.MkdirAll(dataPath, 0750); err != nil {
			return fail(fmt.Errorf("create application data directory: %w", err))
		}
	}

	if pkg.RequiresDB {
		dbCfg, err := m.resolveDatabaseConfig(pkg, req)
		if err != nil {
			return fail(err)
		}
		engine, _, err = newDatabaseEngine(dbCfg)
		if err != nil {
			return fail(err)
		}
		password, err := generatePassword()
		if err != nil {
			return fail(err)
		}
		db = &DBCredentials{
			Engine:   inst.DBEngine,
			Host:     inst.DBHost,
			Port:     inst.DBPort,
			Name:     inst.DBName,
			User:     inst.DBUser,
			Password: password,
		}
		if db.Host == "" || db.Port == "" {
			host, port := splitHostPort(dbCfg.DatabaseURL, defaultPortFor(inst.DBEngine))
			db.Host, db.Port = host, port
		}
		inst.DBHost, inst.DBPort = db.Host, db.Port
		if err := engine.CreateDatabase(*db); err != nil {
			return fail(fmt.Errorf("%w: %v", ErrDatabaseProvision, err))
		}
		m.logf(inst.ID, "database %s created for user %s", db.Name, db.User)
		if err := m.rememberDBAdmin(inst.ID, dbCfg); err != nil {
			return fail(err)
		}
	}

	renderCtx := &renderContext{Pkg: pkg, Inst: inst, DB: db, AdminPassword: req.AdminPassword, Port: req.Port}
	if err := renderConfig(renderCtx); err != nil {
		return fail(fmt.Errorf("render configuration: %w", err))
	}
	if err := renderInstallMetadata(renderCtx); err != nil {
		return fail(fmt.Errorf("write install metadata: %w", err))
	}
	if err := m.ensureTools(pkg, inst); err != nil {
		return fail(err)
	}
	if err := m.runSteps(inst, pkg.Steps, req.AdminPassword, db); err != nil {
		return fail(err)
	}

	if inst.Environment == "staging" {
		if err := writeStagingGuard(target); err != nil {
			return fail(err)
		}
	}
	inst.Status = "installed"
	inst.UpdatedAt = m.now().UTC()
	if err := m.persist(); err != nil {
		return fail(err)
	}
	m.logf(inst.ID, "installation completed")
	completion := pkg.CompletionMode
	if completion == "" {
		completion = "automatic"
	}
	if completion == "wizard" {
		inst.NextStep = "Files and database are ready. Open " + inst.SiteURL + pkg.AdminPath + " to complete the application setup."
	} else if pkg.InstallMode == ModeNode || pkg.InstallMode == ModePython {
		inst.NextStep = "Start the application from Runtimes & Apps to bind its port."
	}
	_ = m.persist()
	var exposedDB *DBCredentials
	if completion == "wizard" {
		exposedDB = db
	}
	return &InstallResult{Instance: inst, AdminPassword: req.AdminPassword, AdminPath: pkg.AdminPath, CompletionMode: completion, Database: exposedDB, NextStep: inst.NextStep}, nil
}

// nextInstanceID reserves a monotonic identifier without attaching it to a
// database row; a failed install still consumes the ordinal.
func (m *Manager) nextInstanceID() (string, int) {
	n := m.nextID.Add(1)
	if n < 1 {
		n = 1
	}
	return fmt.Sprintf("app-%d", n), int(n)
}

func (m *Manager) preparePackage(pkg *Package, req InstallRequest, stage string) (string, string, error) {
	_ = os.RemoveAll(stage)
	switch pkg.ArchiveType {
	case ArchiveGenerated:
		if err := os.MkdirAll(stage, 0755); err != nil {
			return "", "", err
		}
		for _, file := range pkg.Files {
			mode := os.FileMode(0644)
			if file.Mode != 0 {
				mode = os.FileMode(file.Mode) & 0777
			}
			if err := writeConfigFile(stage, file.Path, file.Content, mode); err != nil {
				return "", "", err
			}
		}
		return stage, "", nil
	case ArchiveComposer:
		composer, err := exec.LookPath("composer")
		if err != nil {
			return "", "", fmt.Errorf("%w: composer binary not found", ErrRequirements)
		}
		name := pkg.ComposerPackage
		if pkg.ComposerVersion != "" {
			name += ":" + pkg.ComposerVersion
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, composer, "create-project", name, stage, "--no-interaction", "--prefer-dist")
		out, err := cmd.CombinedOutput()
		if err != nil {
			_ = os.RemoveAll(stage)
			return "", "", fmt.Errorf("%w: composer create-project: %v: %s", ErrDownloadFailed, err, tailOf(out, 400))
		}
		return stage, "", nil
	default:
		archivePath, digest, err := m.downloadAndVerify(pkg)
		if err != nil {
			return "", "", err
		}
		defer os.Remove(archivePath)
		if err := os.MkdirAll(stage, 0755); err != nil {
			return "", "", err
		}
		if err := extractArchive(archivePath, stage, pkg.ArchiveType, pkg.StripPrefix); err != nil {
			_ = os.RemoveAll(stage)
			return "", "", fmt.Errorf("%w: %v", ErrExtractionFailed, err)
		}
		if pkg.InnerArchive != "" {
			inner := filepath.Join(stage, filepath.FromSlash(pkg.InnerArchive))
			if !strings.HasPrefix(inner, filepath.Clean(stage)+string(os.PathSeparator)) {
				_ = os.RemoveAll(stage)
				return "", "", fmt.Errorf("%w: inner archive path escapes stage", ErrExtractionFailed)
			}
			innerStage := stage + ".inner"
			_ = os.RemoveAll(innerStage)
			if err := os.MkdirAll(innerStage, 0755); err != nil {
				_ = os.RemoveAll(stage)
				return "", "", err
			}
			if err := extractArchive(inner, innerStage, ArchiveZip, ""); err != nil {
				_ = os.RemoveAll(stage)
				_ = os.RemoveAll(innerStage)
				return "", "", fmt.Errorf("%w: nested archive: %v", ErrExtractionFailed, err)
			}
			if err := os.RemoveAll(stage); err != nil {
				_ = os.RemoveAll(innerStage)
				return "", "", err
			}
			if err := os.Rename(innerStage, stage); err != nil {
				_ = os.RemoveAll(innerStage)
				return "", "", err
			}
		}
		return stage, digest, nil
	}
}

func (m *Manager) resolveDatabaseConfig(pkg *Package, req InstallRequest) (InstallRequest, error) {
	cfg := req
	cfg.DBEngine = pkg.DBEngine
	if cfg.DBEngine == "" {
		cfg.DBEngine = m.cfg.DefaultDB.Engine
	}
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = m.cfg.DefaultDB.URL
	}
	if cfg.DBAdminUser == "" {
		cfg.DBAdminUser = m.cfg.DefaultDB.AdminUser
	}
	if cfg.DBAdminPass == "" {
		cfg.DBAdminPass = m.cfg.DefaultDB.AdminPassword
	}
	if cfg.MySQLBinary == "" {
		cfg.MySQLBinary = m.cfg.DefaultDB.MySQLBinary
	}
	if cfg.PsqlBinary == "" {
		cfg.PsqlBinary = m.cfg.DefaultDB.PsqlBinary
	}
	if cfg.DBAdminUser == "" || cfg.DBAdminPass == "" {
		return cfg, ErrDatabaseRequired
	}
	return cfg, nil
}

func (m *Manager) checkRequirements(pkg *Package, target string) error {
	var missing []string
	for _, name := range pkg.Requirements.Binaries {
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, "binary:"+name)
		}
	}
	for _, name := range pkg.Requirements.Env {
		if os.Getenv(name) == "" {
			missing = append(missing, "env:"+name)
		}
	}
	if len(pkg.Requirements.PHPExtensions) > 0 {
		php, err := exec.LookPath("php")
		if err != nil {
			missing = append(missing, "binary:php")
		} else {
			out, err := exec.Command(php, "-m").Output()
			if err != nil {
				missing = append(missing, "php:-m")
			} else {
				installed := make(map[string]bool)
				for _, line := range strings.Split(strings.ToLower(string(out)), "\n") {
					installed[strings.TrimSpace(line)] = true
				}
				for _, ext := range pkg.Requirements.PHPExtensions {
					if !installed[strings.ToLower(ext)] {
						missing = append(missing, "php-ext:"+ext)
					}
				}
			}
		}
	}
	if pkg.Requirements.PHPVersion != "" {
		if php, err := exec.LookPath("php"); err != nil {
			missing = append(missing, "binary:php")
		} else if current, err := phpVersionOf(php); err != nil {
			missing = append(missing, "php:version-unreadable")
		} else if constraints, err := parseVersionConstraints(pkg.Requirements.PHPVersion); err != nil {
			missing = append(missing, "php:invalid-constraint")
		} else if ok, failed := satisfyConstraints(current, constraints); !ok {
			missing = append(missing, "php:"+current+"!="+failed)
		}
	}
	if pkg.Requirements.MinDiskMB > 0 {
		free, err := freeDiskMB(filepath.Dir(target))
		if err == nil && free < int64(pkg.Requirements.MinDiskMB) {
			missing = append(missing, fmt.Sprintf("disk:%dMB<required:%dMB", free, pkg.Requirements.MinDiskMB))
		}
	}
	if pkg.Requirements.MinRAMMB > 0 {
		total := totalRAMMB()
		if total > 0 && total < int64(pkg.Requirements.MinRAMMB) {
			missing = append(missing, fmt.Sprintf("ram:%dMB<required:%dMB", total, pkg.Requirements.MinRAMMB))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing %s", ErrRequirements, strings.Join(missing, ", "))
	}
	return nil
}

// --- download and checksum ---

func (m *Manager) downloadAndVerify(pkg *Package) (string, string, error) {
	tmp, err := os.CreateTemp("", "digipanel-download-*")
	if err != nil {
		return "", "", err
	}
	path := tmp.Name()
	tmp.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pkg.DownloadURL, nil)
	if err != nil {
		_ = os.Remove(path)
		return "", "", err
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		_ = os.Remove(path)
		return "", "", fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = os.Remove(path)
		return "", "", fmt.Errorf("%w: http status %d", ErrDownloadFailed, resp.StatusCode)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		_ = os.Remove(path)
		return "", "", err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(f, hash), io.LimitReader(resp.Body, 2<<30))
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return "", "", fmt.Errorf("%w: write archive failed", ErrDownloadFailed)
	}
	digest := hex.EncodeToString(hash.Sum(nil))

	expected := strings.ToLower(pkg.PinnedSHA256)
	if expected == "" {
		expected, err = m.fetchChecksum(pkg)
		if err != nil {
			_ = os.Remove(path)
			return "", "", err
		}
	}
	if !strings.EqualFold(expected, digest) {
		_ = os.Remove(path)
		return "", "", fmt.Errorf("%w: expected %s got %s", ErrChecksumUnverified, expected, digest)
	}
	return path, digest, nil
}

func (m *Manager) fetchChecksum(pkg *Package) (string, error) {
	url := pkg.ChecksumURL
	if url == "" {
		url = pkg.ChecksumListURL
	}
	resp, err := m.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("%w: fetch checksum: %v", ErrChecksumUnverified, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: checksum endpoint status %d", ErrChecksumUnverified, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	text := string(body)
	if pkg.ChecksumURL != "" {
		for _, field := range strings.Fields(text) {
			if isHex64(field) {
				return strings.ToLower(field), nil
			}
		}
		return "", fmt.Errorf("%w: no sha256 in checksum document", ErrChecksumUnverified)
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		switch {
		case len(fields) == 2 && isHex64(fields[0]) && strings.EqualFold(strings.TrimPrefix(fields[1], "*"), pkg.ArchiveName):
			return strings.ToLower(fields[0]), nil
		case strings.HasPrefix(line, "SHA256 ("+pkg.ArchiveName+") ="):
			value := strings.TrimSpace(strings.TrimPrefix(line, "SHA256 ("+pkg.ArchiveName+") ="))
			if isHex64(value) {
				return strings.ToLower(value), nil
			}
		}
	}
	return "", fmt.Errorf("%w: %s not present in checksum list", ErrChecksumUnverified, pkg.ArchiveName)
}

func isHex64(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// --- clone, update, uninstall ---

// Clone copies an installed instance to a new path. Database-backed packages
// receive a fresh database when the package has a config renderer; otherwise
// the clone is refused instead of silently sharing credentials.
func (m *Manager) Clone(id string, req InstallRequest, staging bool) (*Instance, error) {
	src, err := m.GetInstance(id)
	if err != nil {
		return nil, err
	}
	if src.Status != "installed" && src.Status != "staged" {
		return nil, fmt.Errorf("%w: cannot clone status %s", ErrInvalidInput, src.Status)
	}
	pkg, err := m.GetPackage(src.PackageID)
	if err != nil {
		return nil, err
	}
	if req.TargetPath == "" {
		req.TargetPath = src.Path + "-" + strings.ToLower(stagingLabel(staging))
	}
	dest, err := m.confineTarget(req.TargetPath)
	if err != nil {
		return nil, err
	}
	if err := ensureWritableParent(dest); err != nil {
		return nil, err
	}
	if err := ensureEmptyTarget(dest); err != nil {
		return nil, err
	}
	if err := dirCopySandbox(src.Path, dest); err != nil {
		_ = os.RemoveAll(dest)
		return nil, err
	}

	newID, ordinal := m.nextInstanceID()
	clone := *src
	clone.ID = newID
	clone.Path = dest
	clone.Environment = "production"
	clone.Status = "installed"
	if staging {
		clone.Environment = "staging"
		clone.Status = "staged"
	}
	clone.CreatedAt = m.now().UTC()
	clone.UpdatedAt = clone.CreatedAt
	clone.FailureReason = ""
	clone.NextStep = ""
	if req.Domain != "" {
		clone.Domain = req.Domain
	}
	if req.DomainPath != "" {
		clone.DomainPath = strings.Trim(req.DomainPath, "/")
	}
	if req.SiteURL != "" {
		clone.SiteURL = req.SiteURL
	}
	if pkg.DataDirName != "" && src.DataPath != "" {
		clone.DataPath = dest + "-" + pkg.DataDirName
		if _, err := os.Stat(src.DataPath); err == nil {
			if err := dirCopySandbox(src.DataPath, clone.DataPath); err != nil {
				_ = os.RemoveAll(dest)
				return nil, err
			}
		} else if err := os.MkdirAll(clone.DataPath, 0750); err != nil {
			_ = os.RemoveAll(dest)
			return nil, err
		}
	}

	var db *DBCredentials
	if pkg.RequiresDB && !req.ShareDatabase {
		if pkg.ConfigKind == "none" || pkg.ConfigKind == "phpbb" {
			_ = os.RemoveAll(dest)
			return nil, fmt.Errorf("%w: %s", ErrCloneUnsupported, pkg.ID)
		}
		dbReq, err := m.resolveDatabaseConfig(pkg, req)
		if err != nil {
			_ = os.RemoveAll(dest)
			return nil, err
		}
		engine, _, err := newDatabaseEngine(dbReq)
		if err != nil {
			_ = os.RemoveAll(dest)
			return nil, err
		}
		password, err := generatePassword()
		if err != nil {
			_ = os.RemoveAll(dest)
			return nil, err
		}
		clone.DBEngine = pkg.DBEngine
		clone.DBName = fmt.Sprintf("dp_%s_%04d", dbPrefix(pkg.ID), ordinal)
		clone.DBUser = clone.DBName
		host, port := splitHostPort(dbReq.DatabaseURL, defaultPortFor(clone.DBEngine))
		clone.DBHost, clone.DBPort = host, port
		db = &DBCredentials{Engine: clone.DBEngine, Host: host, Port: port, Name: clone.DBName, User: clone.DBUser, Password: password}
		if err := engine.CreateDatabase(*db); err != nil {
			_ = os.RemoveAll(dest)
			return nil, fmt.Errorf("%w: %v", ErrDatabaseProvision, err)
		}
		if src.DBName != "" {
			srcAdmin, adminErr := m.dbAdminConfigFor(src.ID, dbReq)
			if adminErr != nil {
				_ = engine.DropDatabase(*db)
				_ = os.RemoveAll(dest)
				return nil, adminErr
			}
			if err := copyDatabase(src.DBEngine, db.Host, db.Port, srcAdmin.DBAdminUser, srcAdmin.DBAdminPass, src.DBName, clone.DBName); err != nil {
				_ = engine.DropDatabase(*db)
				_ = os.RemoveAll(dest)
				return nil, err
			}
			m.logf(clone.ID, "database copied from %s to %s", src.DBName, clone.DBName)
		}
		renderCtx := &renderContext{Pkg: pkg, Inst: &clone, DB: db, AdminPassword: req.AdminPassword, Port: clone.Port}
		if err := renderConfig(renderCtx); err != nil {
			_ = engine.DropDatabase(*db)
			_ = os.RemoveAll(dest)
			return nil, err
		}
		if pkg.ConfigKind == "wordpress" && src.SiteURL != "" && clone.SiteURL != "" && src.SiteURL != clone.SiteURL {
			if err := rewriteWordPressURL(&clone, src.SiteURL, clone.SiteURL); err != nil {
				_ = engine.DropDatabase(*db)
				_ = os.RemoveAll(dest)
				return nil, err
			}
		}
		if err := m.rememberDBAdmin(clone.ID, dbReq); err != nil {
			_ = engine.DropDatabase(*db)
			_ = os.RemoveAll(dest)
			return nil, err
		}
	} else if pkg.ConfigKind == "wordpress" || pkg.ConfigKind == "laravel" || pkg.ConfigKind == "ghost" {
		if err := renderSiteURLOnly(pkg.ConfigKind, dest, src.SiteURL, clone.SiteURL); err != nil {
			_ = os.RemoveAll(dest)
			return nil, err
		}
	}
	if staging {
		if err := writeStagingGuard(dest); err != nil {
			_ = os.RemoveAll(dest)
			return nil, err
		}
	}
	m.mu.Lock()
	m.instances[clone.ID] = &clone
	m.mu.Unlock()
	if err := m.persist(); err != nil {
		return nil, err
	}
	m.logf(clone.ID, "cloned from %s into %s", src.ID, dest)
	return &clone, nil
}

func stagingLabel(staging bool) string {
	if staging {
		return "staging"
	}
	return "clone"
}

// Update overlays the current catalogue release over the existing tree so user
// content, uploads, and generated configuration survive. A snapshot is written
// first and restored if any step fails.
func (m *Manager) Update(id string, req InstallRequest) (*Instance, error) {
	inst, err := m.GetInstance(id)
	if err != nil {
		return nil, err
	}
	pkg, err := m.GetPackage(inst.PackageID)
	if err != nil {
		return nil, err
	}
	backup, err := m.snapshot(inst)
	if err != nil {
		return nil, err
	}
	stage := inst.Path + ".updatestage-" + inst.ID
	_ = os.RemoveAll(stage)
	staged, digest, err := m.preparePackage(pkg, req, stage)
	if err != nil {
		_ = os.RemoveAll(stage)
		return nil, err
	}
	rollback := func(cause error) (*Instance, error) {
		_ = os.RemoveAll(stage)
		_ = os.RemoveAll(inst.Path)
		if restoreErr := m.restoreSnapshot(backup, inst.Path); restoreErr != nil {
			inst.Status = "failed"
			inst.FailureReason = fmt.Sprintf("%v; rollback failed: %v", cause, restoreErr)
			inst.UpdatedAt = m.now().UTC()
			_ = m.persist()
			return nil, fmt.Errorf("%v; rollback failed: %v", cause, restoreErr)
		}
		inst.Status = "installed"
		inst.FailureReason = cause.Error()
		inst.UpdatedAt = m.now().UTC()
		_ = m.persist()
		m.logf(inst.ID, "update failed and rolled back: %v", cause)
		return nil, cause
	}
	if err := dirCopySandbox(staged, inst.Path); err != nil {
		return rollback(fmt.Errorf("overlay release files: %w", err))
	}
	_ = os.RemoveAll(stage)
	if err := m.runSteps(inst, pkg.UpdateSteps, req.AdminPassword, nil); err != nil {
		return rollback(err)
	}
	inst.InstalledSHA256 = digest
	inst.PackageVersion = pkg.LatestVersion
	inst.Status = "installed"
	inst.FailureReason = ""
	inst.UpdatedAt = m.now().UTC()
	if err := m.persist(); err != nil {
		return rollback(err)
	}
	m.pruneSnapshots(inst.ID, 3)
	m.logf(inst.ID, "updated to %s", pkg.LatestVersion)
	return inst, nil
}

// Uninstall removes files and, when a database was provisioned, drops it using
// the encrypted administrator credential recorded at install time.
func (m *Manager) Uninstall(id string, req InstallRequest) error {
	inst, err := m.GetInstance(id)
	if err != nil {
		return err
	}
	pkg, err := m.GetPackage(inst.PackageID)
	if err != nil {
		return err
	}
	if pkg.RequiresDB && inst.DBName != "" {
		dbCfg, err := m.dbAdminConfigFor(id, req)
		if err != nil {
			return err
		}
		engine, _, err := newDatabaseEngine(dbCfg)
		if err != nil {
			return err
		}
		db := DBCredentials{Engine: inst.DBEngine, Host: inst.DBHost, Port: inst.DBPort, Name: inst.DBName, User: inst.DBUser}
		if err := engine.DropDatabase(db); err != nil {
			return fmt.Errorf("%w: %v", ErrDatabaseProvision, err)
		}
	}
	if err := os.RemoveAll(inst.Path); err != nil {
		return err
	}
	if inst.DataPath != "" {
		if err := os.RemoveAll(inst.DataPath); err != nil {
			return err
		}
	}
	if err := m.runSteps(inst, pkg.UninstallSteps, "", nil); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.instances, id)
	m.mu.Unlock()
	m.forgetDBAdmin(id)
	_ = os.Remove(filepath.Join(m.cfg.StateDir, "logs", id+".log"))
	m.pruneSnapshots(id, 0)
	if err := m.persist(); err != nil {
		return err
	}
	return nil
}

// Logs returns the last n log lines for an instance.
func (m *Manager) Logs(id string, n int) ([]string, error) {
	if _, err := m.GetInstance(id); err != nil {
		return nil, err
	}
	if n <= 0 || n > 1000 {
		n = 200
	}
	data, err := os.ReadFile(filepath.Join(m.cfg.StateDir, "logs", id+".log"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

// --- persistence ---

func (m *Manager) persist() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := persistedState{Instances: m.instances, Secrets: m.secrets, NextID: int(m.nextID.Load())}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(m.cfg.StateDir, "state.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (m *Manager) loadState() error {
	path := filepath.Join(m.cfg.StateDir, "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("invalid installer state: %w", err)
	}
	if state.Instances != nil {
		m.instances = state.Instances
	}
	if state.Secrets != nil {
		m.secrets = state.Secrets
	}
	maxID := 0
	for id := range m.instances {
		if n, err := strconv.Atoi(strings.TrimPrefix(id, "app-")); err == nil && n > maxID {
			maxID = n
		}
	}
	if state.NextID > maxID {
		maxID = state.NextID
	}
	m.nextID.Store(int64(maxID))
	return nil
}

func (m *Manager) logf(id, format string, args ...any) {
	line := fmt.Sprintf("%s %s\n", m.now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
	path := filepath.Join(m.cfg.StateDir, "logs", id+".log")
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	_, _ = f.WriteString(line)
	_ = f.Close()
}

// --- helpers ---

func (m *Manager) confineTarget(value string) (string, error) {
	clean := filepath.Clean(value)
	if !filepath.IsAbs(clean) || clean == "/" {
		return "", ErrInvalidTarget
	}
	base := filepath.Clean(m.cfg.BaseInstallDir)
	if clean != base && !strings.HasPrefix(clean, base+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: %s", ErrInvalidTarget, clean)
	}
	return clean, nil
}

func ensureEmptyTarget(target string) error {
	entries, err := os.ReadDir(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return ErrTargetOccupied
	}
	return nil
}

func ensureWritableParent(target string) error {
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("%w: %s", ErrTargetNotWritable, parent)
	}
	probe, err := os.CreateTemp(parent, ".digipanel-write-probe-*")
	if err != nil {
		return fmt.Errorf("%w: %s", ErrTargetNotWritable, parent)
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}

func activateStage(stage, target string) error {
	if err := ensureEmptyTarget(target); err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	return os.Rename(stage, target)
}

func generateAdminPassword() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func dbPrefix(id string) string {
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, id)
	if len(clean) > 4 {
		clean = clean[:4]
	}
	if clean == "" {
		clean = "app"
	}
	return clean
}

func writeStagingGuard(target string) error {
	path := filepath.Join(target, ".htaccess")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("Require all denied\n"), 0644)
}

// dirCopySandbox copies regular files and directories, refusing symlinks.
func dirCopySandbox(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink %s", ErrArchiveUnsafe, path)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		info, statErr := d.Info()
		if statErr != nil {
			return statErr
		}
		perm := info.Mode().Perm() & 0755
		if perm == 0 {
			perm = 0644
		}
		outFile, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(outFile, in)
		_ = in.Close()
		_ = outFile.Close()
		return copyErr
	})
}

func renderSiteURLOnly(kind, root, oldURL, newURL string) error {
	if oldURL == "" || oldURL == newURL {
		return nil
	}
	switch kind {
	case "wordpress":
		path := filepath.Join(root, "wp-config.php")
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, []byte(strings.ReplaceAll(string(data), oldURL, newURL)), 0640)
	case "laravel", "node", "python":
		path := filepath.Join(root, ".env")
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := strings.ReplaceAll(string(data), "APP_URL="+dotenvQuote(oldURL), "APP_URL="+dotenvQuote(newURL))
		return os.WriteFile(path, []byte(content), 0640)
	case "ghost":
		path := filepath.Join(root, "config.production.json")
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, []byte(strings.ReplaceAll(string(data), oldURL, newURL)), 0640)
	default:
		return nil
	}
}

func freeDiskMB(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize) / (1024 * 1024), nil
}

func totalRAMMB() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				return kb / 1024
			}
		}
	}
	return 0
}

// phpVersionOf asks the PHP binary for its exact version. The argument is a
// fixed literal, never derived from user input.
func phpVersionOf(php string) (string, error) {
	out, err := exec.Command(php, "-r", "echo PHP_VERSION;").Output()
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(out))
	if !regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,3}$`).MatchString(version) {
		return "", fmt.Errorf("unexpected PHP version %q", version)
	}
	return version, nil
}
