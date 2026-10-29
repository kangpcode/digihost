// DigiPanel softinstaller test suite (modules/softinstaller/softinstaller_test.go)
// Exercises manifest validation, signature enforcement, extraction sandboxing,
// secret sealing, configuration renderers, and real installation lifecycles.
// Integration tests that need a database or network artefacts are gated by
// environment variables and skip cleanly when absent.

package softinstaller

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	root := t.TempDir()
	m, err := NewManager(Config{
		CatalogDir:     "catalog",
		BaseInstallDir: filepath.Join(root, "sites"),
		StateDir:       filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

func TestCatalogLoadsAndValidates(t *testing.T) {
	m := newTestManager(t)
	catalogue := m.Catalogue()
	if len(catalogue) != 15 {
		t.Fatalf("expected 15 catalogue entries, got %d", len(catalogue))
	}
	required := map[string]bool{
		"wordpress": false, "joomla": false, "drupal": false, "laravel": false,
		"nextcloud": false, "prestashop": false, "opencart": false, "magento": false,
		"ghost": false, "mediawiki": false, "phpbb": false, "matomo": false,
		"moodle": false, "nodejs-app": false, "python-app": false,
	}
	for _, pkg := range catalogue {
		if _, ok := required[pkg.ID]; !ok {
			t.Errorf("unexpected package %q", pkg.ID)
		}
		required[pkg.ID] = true
		if pkg.Description == "" || pkg.LatestVersion == "" {
			t.Errorf("%s: missing description or version", pkg.ID)
		}
	}
	for id, seen := range required {
		if !seen {
			t.Errorf("catalogue is missing %q", id)
		}
	}
}

func TestManifestRejectsUnknownPlaceholder(t *testing.T) {
	pkg := &Package{
		ID: "sample-app", Name: "Sample", Category: "Test", LatestVersion: "1.0.0",
		Description: "sample", ArchiveType: ArchiveGenerated, InstallMode: ModeGenerated,
		ConfigKind: "none",
		Files:      []FileSpec{{Path: "index.php", Content: "<?php echo 'ok';"}},
		Steps:      []Step{{Kind: StepExec, Binary: "php", Args: []string{"-r", "echo '{secret}';"}}},
	}
	err := validateManifest(pkg)
	if !errors.Is(err, ErrUnknownPlaceholder) {
		t.Fatalf("expected ErrUnknownPlaceholder, got %v", err)
	}
}

func TestManifestRejectsNonAllowlistedBinary(t *testing.T) {
	pkg := &Package{
		ID: "sample-app", Name: "Sample", Category: "Test", LatestVersion: "1.0.0",
		Description: "sample", ArchiveType: ArchiveGenerated, InstallMode: ModeGenerated,
		ConfigKind: "none",
		Files:      []FileSpec{{Path: "index.php", Content: "<?php echo 'ok';"}},
		Steps:      []Step{{Kind: StepExec, Binary: "bash", Args: []string{"-c", "id"}}},
	}
	if err := validateManifest(pkg); err == nil {
		t.Fatal("expected rejection for bash")
	}
}

func TestManifestSignatureEnforcement(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	source, err := os.ReadFile(filepath.Join("catalog", "wordpress.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "wordpress.json")
	if err := os.WriteFile(manifestPath, source, 0600); err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(priv, source)
	if err := os.WriteFile(manifestPath+".sig", []byte(base64.StdEncoding.EncodeToString(sig)), 0600); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, "public.key")
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(pub)), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadManifestFile(manifestPath, true, pub)
	if err != nil {
		t.Fatalf("signed manifest rejected: %v", err)
	}
	if loaded.ID != "wordpress" {
		t.Fatalf("unexpected package %q", loaded.ID)
	}
	tampered := bytes.Replace(source, []byte("WordPress"), []byte("Wordpress"), 1)
	if err := os.WriteFile(manifestPath, tampered, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifestFile(manifestPath, true, pub); !errors.Is(err, ErrManifestSignature) {
		t.Fatalf("expected signature failure, got %v", err)
	}
	if _, err := LoadPublicKey(keyPath); err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}
}

func TestArchiveRejectsTraversal(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "evil.tar.gz")
	writeTarGz(t, archive, []tarEntry{{name: "../escape.txt", body: "escape"}})
	dest := t.TempDir()
	err := extractArchive(archive, dest, ArchiveTarGz, "")
	if !errors.Is(err, ErrArchiveUnsafe) {
		t.Fatalf("expected ErrArchiveUnsafe, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(dest), "escape.txt")); statErr == nil {
		t.Fatal("traversal file escaped the destination")
	}
}

func TestArchiveRejectsSymlink(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "link.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0777}); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()
	if err := extractArchive(archive, t.TempDir(), ArchiveTarGz, ""); !errors.Is(err, ErrArchiveUnsafe) {
		t.Fatalf("expected ErrArchiveUnsafe, got %v", err)
	}
}

func TestArchiveStripPrefix(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "app.tar.gz")
	writeTarGz(t, archive, []tarEntry{
		{name: "app/index.php", body: "root"},
		{name: "app/sub/file.txt", body: "nested"},
		{name: "other/ignored.txt", body: "ignored"},
	})
	dest := t.TempDir()
	if err := extractArchive(archive, dest, ArchiveTarGz, "app/"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dest, "index.php")); err != nil || string(data) != "root" {
		t.Fatalf("index.php: %q %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(dest, "sub", "file.txt")); err != nil || string(data) != "nested" {
		t.Fatalf("nested file: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "other")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("entry outside strip prefix was extracted")
	}
}

func TestZipRejectsTraversal(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "evil.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../escape.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("escape"))
	_ = zw.Close()
	_ = f.Close()
	if err := extractArchive(archive, t.TempDir(), ArchiveZip, ""); !errors.Is(err, ErrArchiveUnsafe) {
		t.Fatalf("expected ErrArchiveUnsafe, got %v", err)
	}
}

func TestSecretVaultRoundTrip(t *testing.T) {
	m := newTestManager(t)
	req := InstallRequest{DBEngine: DBMySQL, DatabaseURL: "127.0.0.1:3306", DBAdminUser: "root", DBAdminPass: "super-secret"}
	if err := m.rememberDBAdmin("app-1", req); err != nil {
		t.Fatal(err)
	}
	state, err := os.ReadFile(filepath.Join(m.cfg.StateDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(state), "super-secret") {
		t.Fatal("database administrator password leaked into state.json")
	}
	info, err := os.Stat(filepath.Join(m.cfg.StateDir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("secret.key mode is %o", info.Mode().Perm())
	}
	out, err := m.dbAdminConfigFor("app-1", InstallRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if out.DBAdminUser != "root" || out.DBAdminPass != "super-secret" || out.DatabaseURL != "127.0.0.1:3306" {
		t.Fatalf("unexpected recovered credentials: %+v", out)
	}
}

func TestWordPressConfigRenderer(t *testing.T) {
	root := t.TempDir()
	ctx := &renderContext{
		Pkg: &Package{ID: "wordpress", ConfigKind: "wordpress"},
		Inst: &Instance{
			Path: root, SiteURL: "https://example.test", Domain: "example.test",
			Title: "Example", AdminUser: "admin", AdminEmail: "admin@example.test",
		},
		DB: &DBCredentials{Engine: DBMySQL, Host: "127.0.0.1", Port: "3306", Name: "db", User: "user", Password: "secret"},
	}
	if err := renderConfig(ctx); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "wp-config.php"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"DB_NAME', 'db'", "DB_USER', 'user'", "DB_PASSWORD', 'secret'", "WP_HOME", "DISALLOW_FILE_EDIT"} {
		if !strings.Contains(string(data), fragment) {
			t.Errorf("wp-config.php missing %q", fragment)
		}
	}
	info, err := os.Stat(filepath.Join(root, "wp-config.php"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0640 {
		t.Errorf("wp-config.php mode is %o", info.Mode().Perm())
	}
}

func TestTargetConfinement(t *testing.T) {
	m := newTestManager(t)
	base := filepath.Clean(m.cfg.BaseInstallDir)
	if _, err := m.confineTarget(base); err != nil {
		t.Fatalf("base itself should be allowed: %v", err)
	}
	for _, bad := range []string{"/etc", "/" + filepath.Base(base) + "-other", filepath.Join(base, "..", "escape")} {
		if _, err := m.confineTarget(bad); !errors.Is(err, ErrInvalidTarget) {
			t.Errorf("%s should be rejected, got %v", bad, err)
		}
	}
	if _, err := m.confineTarget(filepath.Join(base, "site")); err != nil {
		t.Errorf("nested target rejected: %v", err)
	}
}

func TestVersionConstraints(t *testing.T) {
	constraints, err := parseVersionConstraints(">=8.1 <8.5")
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := satisfyConstraints("8.3.32", constraints); !ok {
		t.Fatal("8.3.32 should satisfy >=8.1 <8.5")
	}
	if ok, _ := satisfyConstraints("8.5.0", constraints); ok {
		t.Fatal("8.5.0 should not satisfy <8.5")
	}
	if _, err := parseVersionConstraints(">=eight"); err == nil {
		t.Fatal("invalid constraint accepted")
	}
}

func TestGeneratedPythonInstall(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	m := newTestManager(t)
	result, err := m.Install(InstallRequest{
		PackageID: "python-app", Domain: "python.test", SiteURL: "https://python.test",
		Title: "Python Site", AdminUser: "admin", AdminEmail: "admin@python.test",
	})
	if err != nil {
		t.Fatalf("install python template: %v", err)
	}
	if result.Instance.Status != "installed" {
		t.Fatalf("status %q", result.Instance.Status)
	}
	if _, err := os.Stat(filepath.Join(result.Instance.Path, "app.py")); err != nil {
		t.Fatal(err)
	}
	env, err := os.ReadFile(filepath.Join(result.Instance.Path, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "PORT=8080") {
		t.Fatalf(".env missing port: %s", env)
	}
	if err := m.Uninstall(result.Instance.ID, InstallRequest{}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Stat(result.Instance.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("instance directory survived uninstall")
	}
}

func TestGeneratedNodeInstall(t *testing.T) {
	for _, binary := range []string{"node", "npm"} {
		if _, err := exec.LookPath(binary); err != nil {
			t.Skipf("%s not available", binary)
		}
	}
	m := newTestManager(t)
	result, err := m.Install(InstallRequest{
		PackageID: "nodejs-app", Domain: "node.test", SiteURL: "https://node.test",
		Title: "Node Site", AdminUser: "admin", AdminEmail: "admin@node.test", Port: 9080,
	})
	if err != nil {
		t.Fatalf("install node template: %v", err)
	}
	env, err := os.ReadFile(filepath.Join(result.Instance.Path, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "PORT=9080") {
		t.Fatalf(".env missing port: %s", env)
	}
	snapshotPath, err := m.snapshot(result.Instance)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if _, err := os.Stat(snapshotPath); err != nil {
		t.Fatalf("snapshot missing: %v", err)
	}
	upgraded, err := m.Update(result.Instance.ID, InstallRequest{})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if upgraded.Status != "installed" {
		t.Fatalf("status after update %q", upgraded.Status)
	}
	if err := m.Uninstall(result.Instance.ID, InstallRequest{}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
}

func TestMySQLProvisioner(t *testing.T) {
	url := os.Getenv("DIGIPANEL_TEST_MYSQL_URL")
	user := os.Getenv("DIGIPANEL_TEST_MYSQL_USER")
	pass := os.Getenv("DIGIPANEL_TEST_MYSQL_PASS")
	if url == "" || user == "" {
		t.Skip("DIGIPANEL_TEST_MYSQL_URL/User not set")
	}
	engine, _, err := newDatabaseEngine(InstallRequest{DBEngine: DBMySQL, DatabaseURL: url, DBAdminUser: user, DBAdminPass: pass})
	if err != nil {
		t.Fatal(err)
	}
	creds := DBCredentials{Engine: DBMySQL, Name: "dp_probe_0001", User: "dp_probe_0001", Password: strings.Repeat("a", 64)}
	creds.Host, creds.Port = splitHostPort(url, "3306")
	if err := engine.CreateDatabase(creds); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := engine.CreateDatabase(creds); err != nil {
		t.Fatalf("create must be idempotent: %v", err)
	}
	if err := engine.DropDatabase(creds); err != nil {
		t.Fatalf("drop: %v", err)
	}
}

// TestWordPressLifecycle runs a full install, clone, update, and uninstall
// against a real MySQL/MariaDB server and real WordPress and WP-CLI artefacts.
// Provide DIGIPANEL_TEST_MYSQL_URL/USER/PASS and
// DIGIPANEL_TEST_WORDPRESS_ARCHIVE plus DIGIPANEL_TEST_WPCLI_PHAR.
func TestWordPressLifecycle(t *testing.T) {
	dbURL := os.Getenv("DIGIPANEL_TEST_MYSQL_URL")
	dbUser := os.Getenv("DIGIPANEL_TEST_MYSQL_USER")
	dbPass := os.Getenv("DIGIPANEL_TEST_MYSQL_PASS")
	wpArchive := os.Getenv("DIGIPANEL_TEST_WORDPRESS_ARCHIVE")
	wpCLI := os.Getenv("DIGIPANEL_TEST_WPCLI_PHAR")
	if dbURL == "" || dbUser == "" || wpArchive == "" || wpCLI == "" {
		t.Skip("real WordPress integration environment not configured")
	}
	if _, err := exec.LookPath("php"); err != nil {
		t.Skip("php not available")
	}
	archive, err := os.ReadFile(wpArchive)
	if err != nil {
		t.Fatal(err)
	}
	phar, err := os.ReadFile(wpCLI)
	if err != nil {
		t.Fatal(err)
	}
	archiveSum := sha256Hex(archive)
	pharSum := sha256Hex(phar)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wordpress.tar.gz":
			_, _ = w.Write(archive)
		case "/wp-cli.phar":
			_, _ = w.Write(phar)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	m := newTestManager(t)
	pkg, err := m.GetPackage("wordpress")
	if err != nil {
		t.Fatal(err)
	}
	clonePkg := *pkg
	clonePkg.DownloadURL = server.URL + "/wordpress.tar.gz"
	clonePkg.PinnedSHA256 = archiveSum
	clonePkg.Tools = []Tool{{Name: "wp-cli", URL: server.URL + "/wp-cli.phar", SHA256: pharSum, Path: ".digipanel/tools/wp-cli"}}
	m.pkgs["wordpress"] = &clonePkg

	req := InstallRequest{
		PackageID: "wordpress", Domain: "integration.test", SiteURL: "https://integration.test",
		Title: "Integration Site", AdminUser: "admin", AdminPassword: "S3curePass!234",
		AdminEmail: "admin@integration.test", DatabaseURL: dbURL, DBAdminUser: dbUser, DBAdminPass: dbPass,
	}
	result, err := m.Install(req)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	inst := result.Instance
	if inst.Status != "installed" {
		t.Fatalf("status %q reason %q", inst.Status, inst.FailureReason)
	}
	if data, err := os.ReadFile(filepath.Join(inst.Path, "wp-config.php")); err != nil || !strings.Contains(string(data), inst.DBName) {
		t.Fatalf("wp-config.php: %v", err)
	}
	checkMySQLDatabase(t, req, inst.DBName, true)
	wp := filepath.Join(inst.Path, ".digipanel", "tools", "wp-cli")
	out, err := exec.Command(wp, "option", "get", "siteurl", "--path="+inst.Path).CombinedOutput()
	if err != nil {
		t.Fatalf("wp option get: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "https://integration.test" {
		t.Fatalf("siteurl = %q", out)
	}

	clone, err := m.Clone(inst.ID, InstallRequest{Domain: "clone.test", SiteURL: "https://clone.test", DatabaseURL: dbURL, DBAdminUser: dbUser, DBAdminPass: dbPass}, false)
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	checkMySQLDatabase(t, req, clone.DBName, true)
	out, err = exec.Command(wp, "option", "get", "siteurl", "--path="+clone.Path).CombinedOutput()
	if err != nil {
		t.Fatalf("clone wp option get: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "https://clone.test" {
		t.Fatalf("clone siteurl = %q", out)
	}

	if _, err := m.Update(inst.ID, req); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := os.Stat(filepath.Join(inst.Path, "wp-config.php")); err != nil {
		t.Fatal("wp-config.php lost during update")
	}
	if err := m.Uninstall(clone.ID, InstallRequest{}); err != nil {
		t.Fatalf("uninstall clone: %v", err)
	}
	checkMySQLDatabase(t, req, clone.DBName, false)
	if err := m.Uninstall(inst.ID, InstallRequest{}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	checkMySQLDatabase(t, req, inst.DBName, false)
}

// --- helpers ---

type tarEntry struct {
	name string
	body string
}

func writeTarGz(t *testing.T, path string, entries []tarEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: entry.name, Mode: 0644, Size: int64(len(entry.body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tw, entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func checkMySQLDatabase(t *testing.T, req InstallRequest, name string, want bool) {
	t.Helper()
	bin, err := lookPathAny("mariadb", "mysql")
	if err != nil {
		t.Fatal(err)
	}
	host, port := splitHostPort(req.DatabaseURL, "3306")
	cmd := exec.Command(bin, "--host="+host, "--port="+port, "--protocol=TCP", "--user="+req.DBAdminUser, "--batch", "--skip-column-names", "-e", "SHOW DATABASES LIKE '"+name+"'")
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+req.DBAdminPass)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mysql probe: %v: %s", err, out)
	}
	found := false
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == name {
			found = true
		}
	}
	if found != want {
		t.Fatalf("database %s present=%v want=%v (%s)", name, found, want, out)
	}
}
