// DigiPanel Application Catalogue Manifests (modules/softinstaller/manifest.go)
// Declarative package definitions, strict validation, and detached Ed25519
// manifest signatures. Manifests never contain shell strings: every executable
// step is an argument array resolved against an explicit allowlist.
// Zero emojis, zero fabricated versions or checksums.

package softinstaller

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrManifestInvalid    = errors.New("catalogue manifest rejected")
	ErrManifestSignature  = errors.New("catalogue manifest signature invalid")
	ErrManifestUnsigned   = errors.New("catalogue manifest is not signed")
	ErrChecksumMissing    = errors.New("manifest has no checksum source")
	ErrUnknownPlaceholder = errors.New("post-install argument contains an unknown placeholder")
)

const (
	ArchiveTarGz     = "tar.gz"
	ArchiveTarBz2    = "tar.bz2"
	ArchiveZip       = "zip"
	ArchiveComposer  = "composer"
	ArchiveGenerated = "generated"

	ModeStatic    = "static"
	ModePHPCli    = "php-cli"
	ModeNode      = "node"
	ModePython    = "python"
	ModeComposer  = "composer"
	ModeGenerated = "generated"

	DBMySQL    = "mysql"
	DBPostgres = "postgres"
	DBNone     = "none"

	StepExec    = "exec"
	StepBuiltin = "builtin"
)

// Package is one declarative catalogue entry.
type Package struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Category        string       `json:"category"`
	LatestVersion   string       `json:"latestVersion"`
	Description     string       `json:"description"`
	Homepage        string       `json:"homepage,omitempty"`
	DownloadURL     string       `json:"downloadURL,omitempty"`
	ArchiveType     string       `json:"archiveType"`
	ArchiveName     string       `json:"archiveName,omitempty"`
	InnerArchive    string       `json:"innerArchive,omitempty"`
	StripPrefix     string       `json:"stripPrefix,omitempty"`
	PinnedSHA256    string       `json:"pinnedSHA256,omitempty"`
	ChecksumURL     string       `json:"checksumURL,omitempty"`
	ChecksumListURL string       `json:"checksumListURL,omitempty"`
	RequiresDB      bool         `json:"requiresDB"`
	DBEngine        string       `json:"dbEngine,omitempty"`
	ConfigKind      string       `json:"configKind"`
	ConfigFile      string       `json:"configFile,omitempty"`
	AdminPath       string       `json:"adminPath,omitempty"`
	InstallMode     string       `json:"installMode"`
	CompletionMode  string       `json:"completionMode,omitempty"`
	DataDirName     string       `json:"dataDirName,omitempty"`
	ComposerPackage string       `json:"composerPackage,omitempty"`
	ComposerVersion string       `json:"composerVersion,omitempty"`
	Files           []FileSpec   `json:"files,omitempty"`
	Requirements    Requirements `json:"requirements,omitempty"`
	Tools           []Tool       `json:"tools,omitempty"`
	Steps           []Step       `json:"steps,omitempty"`
	UpdateSteps     []Step       `json:"updateSteps,omitempty"`
	UninstallSteps  []Step       `json:"uninstallSteps,omitempty"`
	Notes           string       `json:"notes,omitempty"`
}

// FileSpec is a generated file for InstallMode=generated.
type FileSpec struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    uint32 `json:"mode,omitempty"`
}

// Requirements declares host dependencies checked before extraction.
type Requirements struct {
	Binaries      []string `json:"binaries,omitempty"`
	PHPExtensions []string `json:"phpExtensions,omitempty"`
	PHPVersion    string   `json:"phpVersion,omitempty"`
	Env           []string `json:"env,omitempty"`
	MinDiskMB     int      `json:"minDiskMB,omitempty"`
	MinRAMMB      int      `json:"minRAMMB,omitempty"`
}

// Tool is an auxiliary verified executable downloaded next to the instance.
type Tool struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Path   string `json:"path,omitempty"`
}

// Step is one post-install or update action.
type Step struct {
	Kind           string   `json:"kind"`
	Binary         string   `json:"binary,omitempty"`
	Args           []string `json:"args,omitempty"`
	Action         string   `json:"action,omitempty"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty"`
	Optional       bool     `json:"optional,omitempty"`
}

var (
	pkgIDRegex       = regexp.MustCompile(`^[a-z][a-z0-9-]{2,40}$`)
	sha256Regex      = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
	safeRelPathRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\./]{1,180}$`)
	placeholderAny   = regexp.MustCompile(`\{[a-zA-Z][a-zA-Z0-9]*\}`)
	domainRegex      = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-\.]{1,253}[a-zA-Z0-9]$`)
	sitePathRegex    = regexp.MustCompile(`^[a-zA-Z0-9_\-./]{0,180}$`)
)

var allowedStepBinaries = map[string]bool{
	"php":      true,
	"composer": true,
	"npm":      true,
	"node":     true,
	"python3":  true,
	"mysql":    true,
	"mariadb":  true,
	"psql":     true,
}

var allowedPlaceholders = map[string]bool{
	"siteURL":        true,
	"path":           true,
	"dataPath":       true,
	"domain":         true,
	"domainPath":     true,
	"dbHost":         true,
	"dbPort":         true,
	"dbName":         true,
	"dbUser":         true,
	"dbPassword":     true,
	"adminUser":      true,
	"adminPassword":  true,
	"adminEmail":     true,
	"title":          true,
	"version":        true,
	"port":           true,
	"toolRoot":       true,
	"opensearchHost": true,
	"opensearchPort": true,
}

var allowedBuiltins = map[string]bool{
	"cleanup.installer": true,
}

var allowedConfigKinds = map[string]bool{
	"none":      true,
	"wordpress": true,
	"laravel":   true,
	"ghost":     true,
	"opencart":  true,
	"phpbb":     true,
	"node":      true,
	"python":    true,
	"drupal":    true,
}

var allowedArchiveTypes = map[string]bool{
	ArchiveTarGz:     true,
	ArchiveTarBz2:    true,
	ArchiveZip:       true,
	ArchiveComposer:  true,
	ArchiveGenerated: true,
}

var allowedInstallModes = map[string]bool{
	ModeStatic:    true,
	ModePHPCli:    true,
	ModeNode:      true,
	ModePython:    true,
	ModeComposer:  true,
	ModeGenerated: true,
}

// validateManifest applies the full catalogue contract. Every rule that can
// make an install silently unsafe is a hard rejection.
func validateManifest(p *Package) error {
	if !pkgIDRegex.MatchString(p.ID) {
		return fmt.Errorf("%w: id must match %s", ErrManifestInvalid, pkgIDRegex.String())
	}
	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.LatestVersion) == "" {
		return fmt.Errorf("%w: %s: name and latestVersion are required", ErrManifestInvalid, p.ID)
	}
	if strings.TrimSpace(p.Description) == "" {
		return fmt.Errorf("%w: %s: description is required", ErrManifestInvalid, p.ID)
	}
	if !allowedArchiveTypes[p.ArchiveType] {
		return fmt.Errorf("%w: %s: unsupported archiveType %q", ErrManifestInvalid, p.ID, p.ArchiveType)
	}
	if !allowedInstallModes[p.InstallMode] {
		return fmt.Errorf("%w: %s: unsupported installMode %q", ErrManifestInvalid, p.ID, p.InstallMode)
	}
	if !allowedConfigKinds[p.ConfigKind] {
		return fmt.Errorf("%w: %s: unsupported configKind %q", ErrManifestInvalid, p.ID, p.ConfigKind)
	}
	if p.CompletionMode != "" && p.CompletionMode != "automatic" && p.CompletionMode != "wizard" {
		return fmt.Errorf("%w: %s: completionMode %q invalid", ErrManifestInvalid, p.ID, p.CompletionMode)
	}
	if p.InnerArchive != "" {
		if !safeRelPathRegex.MatchString(p.InnerArchive) || strings.Contains(p.InnerArchive, "..") || strings.HasPrefix(p.InnerArchive, "/") {
			return fmt.Errorf("%w: %s: innerArchive path rejected", ErrManifestInvalid, p.ID)
		}
		if !strings.HasSuffix(strings.ToLower(p.InnerArchive), ".zip") {
			return fmt.Errorf("%w: %s: innerArchive must be a zip", ErrManifestInvalid, p.ID)
		}
	}
	if p.DataDirName != "" {
		if !pkgIDRegex.MatchString(p.DataDirName) || strings.Contains(p.DataDirName, "..") {
			return fmt.Errorf("%w: %s: dataDirName rejected", ErrManifestInvalid, p.ID)
		}
	}
	if p.Requirements.PHPVersion != "" {
		if _, err := parseVersionConstraints(p.Requirements.PHPVersion); err != nil {
			return fmt.Errorf("%w: %s: phpVersion: %v", ErrManifestInvalid, p.ID, err)
		}
	}
	if p.Homepage != "" && !strings.HasPrefix(p.Homepage, "https://") {
		return fmt.Errorf("%w: %s: homepage must be https", ErrManifestInvalid, p.ID)
	}
	if p.RequiresDB {
		if p.DBEngine != DBMySQL && p.DBEngine != DBPostgres {
			return fmt.Errorf("%w: %s: requiresDB needs dbEngine mysql or postgres", ErrManifestInvalid, p.ID)
		}
	} else if p.DBEngine != "" && p.DBEngine != DBNone {
		return fmt.Errorf("%w: %s: dbEngine set without requiresDB", ErrManifestInvalid, p.ID)
	}

	switch p.ArchiveType {
	case ArchiveTarGz, ArchiveTarBz2, ArchiveZip:
		if !strings.HasPrefix(p.DownloadURL, "https://") {
			return fmt.Errorf("%w: %s: downloadURL must be https", ErrManifestInvalid, p.ID)
		}
		if p.PinnedSHA256 == "" && p.ChecksumURL == "" && (p.ChecksumListURL == "" || p.ArchiveName == "") {
			return fmt.Errorf("%w: %s: %w", ErrManifestInvalid, p.ID, ErrChecksumMissing)
		}
		if p.PinnedSHA256 != "" && !sha256Regex.MatchString(p.PinnedSHA256) {
			return fmt.Errorf("%w: %s: pinnedSHA256 must be 64 hex characters", ErrManifestInvalid, p.ID)
		}
		if p.ChecksumURL != "" && !strings.HasPrefix(p.ChecksumURL, "https://") {
			return fmt.Errorf("%w: %s: checksumURL must be https", ErrManifestInvalid, p.ID)
		}
		if p.ChecksumListURL != "" {
			if !strings.HasPrefix(p.ChecksumListURL, "https://") || p.ArchiveName == "" {
				return fmt.Errorf("%w: %s: checksumListURL requires https and archiveName", ErrManifestInvalid, p.ID)
			}
		}
	case ArchiveComposer:
		if p.ComposerPackage == "" || strings.ContainsAny(p.ComposerPackage, " \t\n;&|") {
			return fmt.Errorf("%w: %s: composerPackage required and must be a single package name", ErrManifestInvalid, p.ID)
		}
	case ArchiveGenerated:
		if len(p.Files) == 0 {
			return fmt.Errorf("%w: %s: generated package needs files", ErrManifestInvalid, p.ID)
		}
	}

	if p.StripPrefix != "" {
		if strings.Contains(p.StripPrefix, "..") || strings.HasPrefix(p.StripPrefix, "/") {
			return fmt.Errorf("%w: %s: stripPrefix must be relative and must not contain ..", ErrManifestInvalid, p.ID)
		}
	}

	for _, f := range p.Files {
		if !safeRelPathRegex.MatchString(f.Path) || strings.Contains(f.Path, "..") || strings.HasPrefix(f.Path, "/") {
			return fmt.Errorf("%w: %s: generated file path %q rejected", ErrManifestInvalid, p.ID, f.Path)
		}
		if strings.ContainsRune(f.Content, 0) {
			return fmt.Errorf("%w: %s: generated file %q contains NUL", ErrManifestInvalid, p.ID, f.Path)
		}
	}

	for i, tool := range p.Tools {
		if !pkgIDRegex.MatchString(tool.Name) {
			return fmt.Errorf("%w: %s: tools[%d] name invalid", ErrManifestInvalid, p.ID, i)
		}
		if !strings.HasPrefix(tool.URL, "https://") || !sha256Regex.MatchString(tool.SHA256) {
			return fmt.Errorf("%w: %s: tools[%d] needs https URL and sha256", ErrManifestInvalid, p.ID, i)
		}
		if tool.Path != "" && (!safeRelPathRegex.MatchString(tool.Path) || strings.Contains(tool.Path, "..")) {
			return fmt.Errorf("%w: %s: tools[%d] path rejected", ErrManifestInvalid, p.ID, i)
		}
	}

	for _, group := range []struct {
		name  string
		steps []Step
	}{{"steps", p.Steps}, {"updateSteps", p.UpdateSteps}, {"uninstallSteps", p.UninstallSteps}} {
		for i, step := range group.steps {
			if err := validateStep(p.ID, group.name, i, step, p.Tools); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateStep(pkgID, group string, idx int, s Step, tools []Tool) error {
	switch s.Kind {
	case StepExec:
		if !allowedStepBinaries[s.Binary] && !strings.HasPrefix(s.Binary, "{tool:") {
			return fmt.Errorf("%w: %s: %s[%d] binary %q is not allowlisted", ErrManifestInvalid, pkgID, group, idx, s.Binary)
		}
		if strings.HasPrefix(s.Binary, "{tool:") {
			name := strings.TrimSuffix(strings.TrimPrefix(s.Binary, "{tool:"), "}")
			found := false
			for _, t := range tools {
				if t.Name == name {
					found = true
				}
			}
			if !found || !pkgIDRegex.MatchString(name) {
				return fmt.Errorf("%w: %s: %s[%d] references unknown tool %q", ErrManifestInvalid, pkgID, group, idx, name)
			}
		}
		for _, arg := range s.Args {
			if strings.ContainsRune(arg, 0) {
				return fmt.Errorf("%w: %s: %s[%d] argument contains NUL", ErrManifestInvalid, pkgID, group, idx)
			}
			for _, candidate := range placeholderAny.FindAllString(arg, -1) {
				key := strings.Trim(candidate, "{}")
				if !allowedPlaceholders[key] {
					return fmt.Errorf("%w: %s: %s[%d] uses %s", ErrUnknownPlaceholder, pkgID, group, idx, candidate)
				}
			}
		}
	case StepBuiltin:
		if !allowedBuiltins[s.Action] {
			return fmt.Errorf("%w: %s: %s[%d] builtin action %q is not registered", ErrManifestInvalid, pkgID, group, idx, s.Action)
		}
	default:
		return fmt.Errorf("%w: %s: %s[%d] kind %q invalid", ErrManifestInvalid, pkgID, group, idx, s.Kind)
	}
	if s.TimeoutSeconds < 0 || s.TimeoutSeconds > 7200 {
		return fmt.Errorf("%w: %s: %s[%d] timeout out of range", ErrManifestInvalid, pkgID, group, idx)
	}
	return nil
}

// loadManifestFile reads and verifies one manifest. When requireSignature is
// true a detached <name>.sig over the exact file bytes must verify against the
// supplied public key.
func loadManifestFile(path string, requireSignature bool, publicKey ed25519.PublicKey) (*Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", filepath.Base(path), err)
	}
	if requireSignature {
		if publicKey == nil {
			return nil, fmt.Errorf("%w: %s", ErrManifestUnsigned, filepath.Base(path))
		}
		sig, err := readSignature(path + ".sig")
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrManifestSignature, filepath.Base(path), err)
		}
		if !ed25519.Verify(publicKey, data, sig) {
			return nil, fmt.Errorf("%w: %s", ErrManifestSignature, filepath.Base(path))
		}
	}
	var pkg Package
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrManifestInvalid, filepath.Base(path), err)
	}
	if err := validateManifest(&pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

func readSignature(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(raw))
	if decoded, err := hex.DecodeString(text); err == nil && len(decoded) == ed25519.SignatureSize {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(text); err == nil && len(decoded) == ed25519.SignatureSize {
		return decoded, nil
	}
	return nil, errors.New("signature must be hex or base64 encoded Ed25519 signature")
}

// LoadPublicKey accepts a raw 32-byte base64/hex key or a PEM PKIX public key.
func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(raw))
	if block, _ := pem.Decode([]byte(text)); block != nil {
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse public key: %w", err)
		}
		ed, ok := key.(ed25519.PublicKey)
		if !ok {
			return nil, errors.New("public key is not Ed25519")
		}
		return ed, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(text); err == nil && len(decoded) == ed25519.PublicKeySize {
		return ed25519.PublicKey(decoded), nil
	}
	if decoded, err := hex.DecodeString(text); err == nil && len(decoded) == ed25519.PublicKeySize {
		return ed25519.PublicKey(decoded), nil
	}
	return nil, errors.New("public key must be raw base64/hex Ed25519 or PEM PKIX")
}

func validateInstallRequest(pkg *Package, req InstallRequest) error {
	if !domainRegex.MatchString(req.Domain) {
		return fmt.Errorf("%w: domain format invalid", ErrInvalidInput)
	}
	if !sitePathRegex.MatchString(req.DomainPath) || strings.Contains(req.DomainPath, "..") {
		return fmt.Errorf("%w: domain path rejected", ErrInvalidInput)
	}
	if req.SiteURL == "" || !strings.HasPrefix(req.SiteURL, "https://") {
		return fmt.Errorf("%w: siteURL must be https", ErrInvalidInput)
	}
	if strings.ContainsAny(req.SiteURL, "\"'`;\n\r") || len(req.SiteURL) > 300 {
		return fmt.Errorf("%w: siteURL contains rejected characters", ErrInvalidInput)
	}
	if req.AdminUser != "" && !regexp.MustCompile(`^[a-zA-Z0-9._\-]{1,64}$`).MatchString(req.AdminUser) {
		return fmt.Errorf("%w: admin user format invalid", ErrInvalidInput)
	}
	if req.AdminEmail != "" && !regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`).MatchString(req.AdminEmail) {
		return fmt.Errorf("%w: admin email format invalid", ErrInvalidInput)
	}
	if req.Title != "" && !regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} ._&\-]{0,119}$`).MatchString(req.Title) {
		return fmt.Errorf("%w: site title contains rejected characters", ErrInvalidInput)
	}
	if req.PackageID == "" {
		return fmt.Errorf("%w: package id is required", ErrInvalidInput)
	}
	if pkg.RequiresDB && req.DatabaseURL == "" {
		return ErrDatabaseRequired
	}
	return nil
}

// versionConstraint is one parsed comparison such as ">=8.1".
type versionConstraint struct {
	op      string
	version string
}

// parseVersionConstraints accepts space-separated comparisons, for example
// ">=8.1 <8.5" or a bare "8.1" meaning ">=8.1".
func parseVersionConstraints(spec string) ([]versionConstraint, error) {
	fields := strings.Fields(spec)
	if len(fields) == 0 {
		return nil, errors.New("empty version constraint")
	}
	out := make([]versionConstraint, 0, len(fields))
	for _, field := range fields {
		op := ""
		version := field
		for _, candidate := range []string{">=", "<=", ">", "<", "=", "=="} {
			if strings.HasPrefix(field, candidate) {
				op = candidate
				version = strings.TrimPrefix(field, candidate)
				break
			}
		}
		if op == "" {
			op = ">="
		}
		if op == "==" {
			op = "="
		}
		if !regexp.MustCompile(`^[0-9]+(\.[0-9]+){0,3}$`).MatchString(version) {
			return nil, fmt.Errorf("invalid version %q", version)
		}
		out = append(out, versionConstraint{op: op, version: version})
	}
	return out, nil
}

func compareVersions(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func satisfyConstraints(current string, constraints []versionConstraint) (bool, string) {
	for _, c := range constraints {
		cmp := compareVersions(current, c.version)
		ok := false
		switch c.op {
		case ">=":
			ok = cmp >= 0
		case ">":
			ok = cmp > 0
		case "<=":
			ok = cmp <= 0
		case "<":
			ok = cmp < 0
		case "=":
			ok = cmp == 0
		}
		if !ok {
			return false, c.op + c.version
		}
	}
	return true, ""
}
