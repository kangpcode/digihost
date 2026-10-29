// DigiPanel post-install step runner (modules/softinstaller/steps.go)
// Executes allowlisted binaries with argument arrays, verifies auxiliary tool
// checksums, runs built-in cleanup actions, and records redacted logs.
// Zero shell strings, zero emojis.

package softinstaller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrStepFailed     = errors.New("application post-install step failed")
	ErrToolUnverified = errors.New("auxiliary tool checksum mismatch")
)

func (m *Manager) runSteps(inst *Instance, steps []Step, adminPassword string, db *DBCredentials) error {
	for i, step := range steps {
		if err := m.runStep(inst, step, adminPassword, db); err != nil {
			if step.Optional {
				m.logf(inst.ID, "optional step %d (%s) failed: %v", i, step.Kind, err)
				continue
			}
			return fmt.Errorf("%w: step %d: %v", ErrStepFailed, i, err)
		}
	}
	return nil
}

func (m *Manager) runStep(inst *Instance, step Step, adminPassword string, db *DBCredentials) error {
	switch step.Kind {
	case StepBuiltin:
		return m.runBuiltin(inst, step.Action)
	case StepExec:
		return m.runExec(inst, step, adminPassword, db)
	default:
		return fmt.Errorf("unsupported step kind %q", step.Kind)
	}
}

func (m *Manager) runExec(inst *Instance, step Step, adminPassword string, db *DBCredentials) error {
	binary, err := m.resolveStepBinary(inst, step.Binary)
	if err != nil {
		return err
	}
	args, redactions, err := m.expandArgs(inst, step.Args, adminPassword, db)
	if err != nil {
		return err
	}
	timeout := step.TimeoutSeconds
	if timeout <= 0 {
		timeout = 300
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	m.logf(inst.ID, "exec %s %s", filepath.Base(binary), redactArgs(args, redactions))
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = inst.Path
	cmd.Env = append(os.Environ(),
		"DIGIPANEL_APP_PATH="+inst.Path,
		"DIGIPANEL_SITE_URL="+inst.SiteURL,
		"DIGIPANEL_DOMAIN="+inst.Domain,
		"DIGIPANEL_ADMIN_USER="+inst.AdminUser,
		"DIGIPANEL_ADMIN_PASSWORD="+adminPassword,
	)
	if db != nil {
		cmd.Env = append(cmd.Env,
			"DIGIPANEL_DB_HOST="+db.Host,
			"DIGIPANEL_DB_PORT="+db.Port,
			"DIGIPANEL_DB_NAME="+db.Name,
			"DIGIPANEL_DB_USER="+db.User,
			"DIGIPANEL_DB_PASSWORD="+db.Password,
		)
	}
	out, runErr := cmd.CombinedOutput()
	tail := redact(string(out), redactions)
	if len(tail) > 4000 {
		tail = tail[len(tail)-4000:]
	}
	if runErr != nil {
		return fmt.Errorf("%s: %v: %s", filepath.Base(binary), runErr, strings.TrimSpace(tail))
	}
	if strings.TrimSpace(tail) != "" {
		m.logf(inst.ID, "output: %s", strings.TrimSpace(tail))
	}
	return nil
}

func (m *Manager) resolveStepBinary(inst *Instance, binary string) (string, error) {
	if strings.HasPrefix(binary, "{tool:") {
		name := strings.TrimSuffix(strings.TrimPrefix(binary, "{tool:"), "}")
		path := filepath.Join(inst.Path, ".digipanel", "tools", name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return "", fmt.Errorf("required tool %q is not present", name)
		}
		return path, nil
	}
	if !allowedStepBinaries[binary] {
		return "", fmt.Errorf("binary %q is not allowlisted", binary)
	}
	found, err := exec.LookPath(binary)
	if err != nil {
		return "", fmt.Errorf("required binary %q not found on PATH", binary)
	}
	return found, nil
}

func (m *Manager) expandArgs(inst *Instance, args []string, adminPassword string, db *DBCredentials) ([]string, []string, error) {
	dbHost, dbPort, dbName, dbUser, dbPassword := "", "", "", "", ""
	if db != nil {
		dbHost, dbPort, dbName, dbUser, dbPassword = db.Host, db.Port, db.Name, db.User, db.Password
	}
	values := map[string]string{
		"siteURL":        inst.SiteURL,
		"path":           inst.Path,
		"dataPath":       inst.DataPath,
		"domain":         inst.Domain,
		"domainPath":     inst.DomainPath,
		"dbHost":         dbHost,
		"dbPort":         dbPort,
		"dbName":         dbName,
		"dbUser":         dbUser,
		"dbPassword":     dbPassword,
		"adminUser":      inst.AdminUser,
		"adminPassword":  adminPassword,
		"adminEmail":     inst.AdminEmail,
		"title":          inst.Title,
		"version":        inst.PackageVersion,
		"port":           fmt.Sprintf("%d", inst.Port),
		"toolRoot":       filepath.Join(inst.Path, ".digipanel", "tools"),
		"opensearchHost": os.Getenv("OPENSEARCH_HOST"),
		"opensearchPort": os.Getenv("OPENSEARCH_PORT"),
	}
	var redactions []string
	for _, secret := range []string{adminPassword, dbPassword} {
		if secret != "" {
			redactions = append(redactions, secret)
		}
	}
	out := make([]string, 0, len(args))
	for _, arg := range args {
		for key, value := range values {
			arg = strings.ReplaceAll(arg, "{"+key+"}", value)
		}
		if strings.Contains(arg, "{") && strings.Contains(arg, "}") && placeholderAny.MatchString(arg) {
			return nil, nil, fmt.Errorf("%w: %s", ErrUnknownPlaceholder, arg)
		}
		out = append(out, arg)
	}
	return out, redactions, nil
}

func redact(value string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "***")
		}
	}
	return value
}

func redactArgs(args []string, secrets []string) string {
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = redact(arg, secrets)
	}
	return strings.Join(out, " ")
}

// ensureTools downloads every declared auxiliary executable into the instance
// and verifies its pinned SHA-256 before use.
func (m *Manager) ensureTools(pkg *Package, inst *Instance) error {
	for _, tool := range pkg.Tools {
		rel := tool.Path
		if rel == "" {
			rel = filepath.Join(".digipanel", "tools", tool.Name)
		}
		dest := filepath.Join(inst.Path, filepath.Clean("/"+rel))
		if !strings.HasPrefix(dest, filepath.Clean(inst.Path)+string(os.PathSeparator)) {
			return fmt.Errorf("%w: tool path escapes target", ErrInvalidInput)
		}
		digest, err := m.downloadTool(tool.URL, dest)
		if err != nil {
			return err
		}
		if !strings.EqualFold(digest, tool.SHA256) {
			_ = os.Remove(dest)
			return fmt.Errorf("%w: %s expected %s got %s", ErrToolUnverified, tool.Name, tool.SHA256, digest)
		}
		if err := os.Chmod(dest, 0755); err != nil {
			return err
		}
		m.logf(inst.ID, "tool %s verified (%s)", tool.Name, digest[:12])
	}
	return nil
}

func (m *Manager) downloadTool(url, dest string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", err
	}
	resp, err := m.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: http status %d", url, resp.StatusCode)
	}
	tmp := dest + ".part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(f, hash), io.LimitReader(resp.Body, 256<<20))
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// runBuiltin performs non-shell actions that are part of the installation
// contract, such as removing an exposed installer directory.
func (m *Manager) runBuiltin(inst *Instance, action string) error {
	switch action {
	case "cleanup.installer":
		for _, rel := range []string{"install", "install-dev"} {
			path := filepath.Join(inst.Path, rel)
			if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(inst.Path)+string(os.PathSeparator)) {
				return fmt.Errorf("%w: cleanup path escaped target", ErrInvalidInput)
			}
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("remove installer directory %s: %w", rel, err)
			}
			m.logf(inst.ID, "removed installer directory %s", rel)
		}
		return nil
	default:
		return fmt.Errorf("unknown builtin action %q", action)
	}
}

// rewriteWordPressURL updates every table so the cloned site answers on its new
// address. It uses the verified WP-CLI tool installed with the package.
func rewriteWordPressURL(inst *Instance, oldURL, newURL string) error {
	tool := filepath.Join(inst.Path, ".digipanel", "tools", "wp-cli")
	if _, err := os.Stat(tool); err != nil {
		if found, lookErr := exec.LookPath("wp"); lookErr == nil {
			tool = found
		} else {
			return fmt.Errorf("clone: wp-cli is not available to rewrite the site URL")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool,
		"search-replace", oldURL, newURL,
		"--all-tables", "--precise", "--skip-columns=guid",
		"--path="+inst.Path,
	)
	cmd.Dir = inst.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("clone: wp search-replace failed: %v: %s", err, tailOf(out, 400))
	}
	return nil
}
