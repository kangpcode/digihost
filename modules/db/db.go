// DigiPanel Database Manager (modules/db/db.go)
// Real MariaDB/MySQL administration through the native client binaries.
// Every statement is built from validated identifiers; the administrator
// password travels in the environment, never in argv. No statement text is ever
// accepted from a caller, so no SQL can be injected through the API.
// Zero emojis, strictly production grade.

package db

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrNotConfigured   = errors.New("database manager is not configured: set DIGIPANEL_DB_ADMIN_USER and DIGIPANEL_DB_ADMIN_PASSWORD")
	ErrClientMissing   = errors.New("mysql or mariadb client binary not found on PATH")
	ErrInvalidDatabase = errors.New("database name must match ^[a-z][a-z0-9_]{0,63}$")
	ErrInvalidUser     = errors.New("database user name must match ^[a-z][a-z0-9_]{0,31}$")
	ErrInvalidOwner    = errors.New("account name must match ^[a-z][a-z0-9_]{2,31}$")
	ErrInvalidPassword = errors.New("password must be 16-64 characters from A-Z a-z 0-9 _ ! @ # % ^ & * + =")
	ErrUnsupported     = errors.New("only mysql and mariadb engines are supported by this manager")
	ErrTooLong         = errors.New("combined account and suffix name exceeds the engine limit of 32 characters")
)

const (
	EngineMySQL   = "mysql"
	EngineMariaDB = "mariadb"

	defaultCharset = "utf8mb4"
	defaultCollate = "utf8mb4_unicode_ci"
	maxNameLength  = 32
)

var (
	namePattern      = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	ownerPattern     = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)
	passwordPattern  = regexp.MustCompile(`^[A-Za-z0-9_!@#%^&*+=]{16,64}$`)
	passwordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#%^&*+"
)

// Options carries the engine connection settings. Binary is resolved
// automatically when empty.
type Options struct {
	Engine        string
	Binary        string
	Host          string
	Port          string
	AdminUser     string
	AdminPassword string
	Charset       string
	Collation     string
	Timeout       time.Duration
}

// Executor runs the client with explicit arguments. Scripts are fed over stdin.
// Tests substitute a recording executor, production uses the local client.
type Executor interface {
	Run(ctx context.Context, args []string, stdin string) (string, error)
}

// Database is one schema owned by a hosting account.
type Database struct {
	Name      string `json:"name"`
	Owner     string `json:"owner"`
	Charset   string `json:"charset"`
	Collation string `json:"collation"`
	Tables    int    `json:"tables"`
	SizeBytes int64  `json:"sizeBytes"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// User is one database identity owned by a hosting account.
type User struct {
	Name   string   `json:"name"`
	Owner  string   `json:"owner"`
	Host   string   `json:"host"`
	Grants []string `json:"grants"`
}

// Manager executes database administration against a live server.
type Manager struct {
	opts Options
	exec Executor
}

// NewManager builds a manager backed by the local client binary.
func NewManager(opts Options) *Manager {
	m := &Manager{opts: opts}
	if m.opts.Timeout == 0 {
		m.opts.Timeout = 30 * time.Second
	}
	if m.opts.Charset == "" {
		m.opts.Charset = defaultCharset
	}
	if m.opts.Collation == "" {
		m.opts.Collation = defaultCollate
	}
	if m.opts.Host == "" {
		m.opts.Host = "127.0.0.1"
	}
	if m.opts.Port == "" {
		m.opts.Port = "3306"
	}
	m.exec = &localExecutor{binary: m.binary(), password: m.opts.AdminPassword}
	return m
}

// NewManagerWithExecutor allows a caller (or a test) to supply the transport.
func NewManagerWithExecutor(opts Options, exec Executor) *Manager {
	m := NewManager(opts)
	if exec != nil {
		m.exec = exec
	}
	return m
}

// Options exposes the resolved configuration for diagnostics.
func (m *Manager) Options() Options {
	return m.opts
}

// Configured reports whether administrator credentials are present. The API
// refuses to advertise database capabilities when the manager is not configured
// instead of failing halfway through provisioning.
func (m *Manager) Configured() bool {
	return m != nil && m.opts.AdminUser != "" && m.opts.AdminPassword != ""
}

// Available reports whether the client binary exists.
func (m *Manager) Available() bool {
	if m == nil {
		return false
	}
	_, err := os.Stat(m.binary())
	return err == nil
}

func (m *Manager) binary() string {
	if m.opts.Binary != "" {
		return m.opts.Binary
	}
	for _, candidate := range []string{"mysql", "mariadb"} {
		if found, err := exec.LookPath(candidate); err == nil {
			return found
		}
	}
	return "mysql"
}

type localExecutor struct {
	binary   string
	password string
}

func (e *localExecutor) Run(ctx context.Context, args []string, stdin string) (string, error) {
	cmd := exec.CommandContext(ctx, e.binary, args...)
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+e.password)
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if len(trimmed) > 400 {
			trimmed = trimmed[:400]
		}
		if trimmed == "" {
			return "", fmt.Errorf("database client failed: %w", err)
		}
		return "", fmt.Errorf("database client failed: %s", trimmed)
	}
	return string(out), nil
}

func (m *Manager) clientArgs() []string {
	return []string{
		"--host=" + m.opts.Host,
		"--port=" + m.opts.Port,
		"--protocol=TCP",
		"--user=" + m.opts.AdminUser,
		"--batch",
		"--skip-column-names",
		"--default-character-set=" + m.opts.Charset,
	}
}

func (m *Manager) run(ctx context.Context, script string) (string, error) {
	if m == nil {
		return "", ErrNotConfigured
	}
	if m.opts.AdminUser == "" || m.opts.AdminPassword == "" {
		return "", ErrNotConfigured
	}
	if m.exec == nil {
		return "", ErrClientMissing
	}
	if _, ok := m.exec.(*localExecutor); ok && !m.Available() {
		return "", ErrClientMissing
	}
	tctx, cancel := context.WithTimeout(ctx, m.opts.Timeout)
	defer cancel()
	return m.exec.Run(tctx, m.clientArgs(), script)
}

// ValidateOwner enforces the hosting account naming rule that database objects
// are derived from.
func ValidateOwner(owner string) error {
	if !ownerPattern.MatchString(owner) {
		return ErrInvalidOwner
	}
	return nil
}

// NormalizeOwner lowercases and trims an account name before validation.
func NormalizeOwner(owner string) string {
	return strings.ToLower(strings.TrimSpace(owner))
}

// ValidateName validates a database or user suffix.
func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return ErrInvalidDatabase
	}
	if len(name) > maxNameLength {
		return ErrTooLong
	}
	return nil
}

// Qualified returns the engine object name for an account and suffix pair.
// Every object is prefixed with the owning account, which is what keeps tenants
// from reaching each other's schemas.
func Qualified(owner, suffix string) (string, error) {
	owner = NormalizeOwner(owner)
	if err := ValidateOwner(owner); err != nil {
		return "", err
	}
	suffix = strings.ToLower(strings.TrimSpace(suffix))
	if err := ValidateName(suffix); err != nil {
		return "", err
	}
	full := owner + "_" + suffix
	if len(full) > maxNameLength {
		return "", ErrTooLong
	}
	return full, nil
}

// ValidatePassword enforces a password alphabet that requires no SQL escaping.
func ValidatePassword(password string) error {
	if !passwordPattern.MatchString(password) {
		return ErrInvalidPassword
	}
	return nil
}

// GeneratePassword returns a random password that satisfies ValidatePassword.
func GeneratePassword() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	out := make([]byte, len(buffer))
	for index, value := range buffer {
		out[index] = passwordAlphabet[int(value)%len(passwordAlphabet)]
	}
	// Guarantee the policy regardless of the draw.
	out[0] = 'A'
	out[1] = '7'
	out[2] = '!'
	return string(out), nil
}

// Ping verifies that the server answers and returns its version string.
func (m *Manager) Ping(ctx context.Context) (string, error) {
	out, err := m.run(ctx, "SELECT VERSION();\n")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ListDatabases returns the schemas owned by one account.
func (m *Manager) ListDatabases(ctx context.Context, owner string) ([]Database, error) {
	owner = NormalizeOwner(owner)
	if err := ValidateOwner(owner); err != nil {
		return nil, err
	}
	prefix := escapeSQLLiteral(owner + `\_%`)
	query := fmt.Sprintf(`SELECT s.SCHEMA_NAME, s.DEFAULT_CHARACTER_SET_NAME, s.DEFAULT_COLLATION_NAME,
  (SELECT COUNT(*) FROM information_schema.TABLES t WHERE t.TABLE_SCHEMA = s.SCHEMA_NAME),
  COALESCE((SELECT SUM(t.DATA_LENGTH + t.INDEX_LENGTH) FROM information_schema.TABLES t WHERE t.TABLE_SCHEMA = s.SCHEMA_NAME), 0)
FROM information_schema.SCHEMATA s
WHERE s.SCHEMA_NAME LIKE '%s' ESCAPE '\\'
ORDER BY s.SCHEMA_NAME;
`, prefix)

	out, err := m.run(ctx, query)
	if err != nil {
		return nil, err
	}

	databases := make([]Database, 0, 4)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		tables := 0
		_, _ = fmt.Sscanf(fields[3], "%d", &tables)
		var size int64
		_, _ = fmt.Sscanf(fields[4], "%d", &size)
		databases = append(databases, Database{
			Name:      fields[0],
			Owner:     owner,
			Charset:   fields[1],
			Collation: fields[2],
			Tables:    tables,
			SizeBytes: size,
		})
	}
	sort.Slice(databases, func(i, j int) bool { return databases[i].Name < databases[j].Name })
	return databases, nil
}

// ListUsers returns the database identities owned by one account with their
// granted schemas.
func (m *Manager) ListUsers(ctx context.Context, owner string) ([]User, error) {
	owner = NormalizeOwner(owner)
	if err := ValidateOwner(owner); err != nil {
		return nil, err
	}
	prefix := escapeSQLLiteral(owner + `\_%`)
	query := fmt.Sprintf(`SELECT u.User, u.Host, COALESCE(GROUP_CONCAT(DISTINCT p.TABLE_SCHEMA ORDER BY p.TABLE_SCHEMA SEPARATOR ','), '')
FROM mysql.user u
LEFT JOIN information_schema.SCHEMA_PRIVILEGES p ON p.GRANTEE = CONCAT('''', u.User, '''@''', u.Host, '''')
WHERE u.User LIKE '%s' ESCAPE '\\'
GROUP BY u.User, u.Host
ORDER BY u.User, u.Host;
`, prefix)

	out, err := m.run(ctx, query)
	if err != nil {
		return nil, err
	}

	users := make([]User, 0, 4)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		grants := make([]string, 0, 2)
		if len(fields) > 2 && fields[2] != "" {
			grants = strings.Split(fields[2], ",")
		}
		users = append(users, User{Name: fields[0], Owner: owner, Host: fields[1], Grants: grants})
	}
	return users, nil
}

// CreateDatabase provisions a schema named <owner>_<suffix>.
func (m *Manager) CreateDatabase(ctx context.Context, owner, suffix string) (*Database, error) {
	name, err := Qualified(owner, suffix)
	if err != nil {
		return nil, err
	}
	script := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET %s COLLATE %s;\n", name, m.opts.Charset, m.opts.Collation)
	if _, err := m.run(ctx, script); err != nil {
		return nil, err
	}
	return &Database{
		Name:      name,
		Owner:     NormalizeOwner(owner),
		Charset:   m.opts.Charset,
		Collation: m.opts.Collation,
	}, nil
}

// DropDatabase removes a schema and everything inside it.
func (m *Manager) DropDatabase(ctx context.Context, owner, suffix string) error {
	name, err := Qualified(owner, suffix)
	if err != nil {
		return err
	}
	_, err = m.run(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;\n", name))
	return err
}

// CreateUser creates a database identity for an account. The user is limited to
// local connections and holds privileges only on its own schema.
func (m *Manager) CreateUser(ctx context.Context, owner, suffix, password, database string) (*User, error) {
	name, err := Qualified(owner, suffix)
	if err != nil {
		return nil, err
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	if database != "" {
		if _, err := Qualified(owner, database); err != nil {
			return nil, err
		}
		database = NormalizeOwner(owner) + "_" + strings.ToLower(strings.TrimSpace(database))
	} else {
		database = name
	}

	var script strings.Builder
	fmt.Fprintf(&script, "CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s';\n", name, password)
	fmt.Fprintf(&script, "ALTER USER '%s'@'localhost' IDENTIFIED BY '%s';\n", name, password)
	fmt.Fprintf(&script, "GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost';\n", database, name)
	script.WriteString("FLUSH PRIVILEGES;\n")

	if _, err := m.run(ctx, script.String()); err != nil {
		return nil, err
	}
	return &User{Name: name, Owner: NormalizeOwner(owner), Host: "localhost", Grants: []string{database}}, nil
}

// DropUser removes a database identity owned by the account.
func (m *Manager) DropUser(ctx context.Context, owner, suffix string) error {
	name, err := Qualified(owner, suffix)
	if err != nil {
		return err
	}
	_, err = m.run(ctx, fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost';\nFLUSH PRIVILEGES;\n", name))
	return err
}

// SetPassword rotates the password of an account owned identity.
func (m *Manager) SetPassword(ctx context.Context, owner, suffix, password string) error {
	name, err := Qualified(owner, suffix)
	if err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	script := fmt.Sprintf("ALTER USER '%s'@'localhost' IDENTIFIED BY '%s';\nFLUSH PRIVILEGES;\n", name, password)
	_, err = m.run(ctx, script)
	return err
}

// Grant gives an account owned identity full privileges on one of its schemas.
func (m *Manager) Grant(ctx context.Context, owner, suffix, database string) error {
	name, err := Qualified(owner, suffix)
	if err != nil {
		return err
	}
	dbName, err := Qualified(owner, database)
	if err != nil {
		return err
	}
	script := fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost';\nFLUSH PRIVILEGES;\n", dbName, name)
	_, err = m.run(ctx, script)
	return err
}

// escapeSQLLiteral escapes a value used inside a quoted LIKE pattern. Only
// owner names reach this function, and they are already restricted to
// [a-z0-9_], so the escape is a second line of defence rather than the primary
// control.
func escapeSQLLiteral(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `'`, `''`, `"`, `\"`, "%", `\%`, "_", `\_`)
	return replacer.Replace(value)
}
