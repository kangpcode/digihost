// DigiPanel database provisioner (modules/softinstaller/database.go)
// Real MariaDB/MySQL and PostgreSQL provisioning through their native clients.
// Secrets travel over stdin or environment, never through argv.
// Zero shell strings, zero emojis.

package softinstaller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	ErrDatabaseClientMissing = errors.New("database client binary not found on PATH")
	ErrDatabaseCallFailed    = errors.New("database server call failed")
)

// DBCredentials is the per-instance database identity.
type DBCredentials struct {
	Engine   string `json:"engine"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Name     string `json:"name"`
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
}

// DatabaseEngine is the provisioning contract. Implementations must perform
// real server calls; a failed call must return an error.
type DatabaseEngine interface {
	CreateDatabase(creds DBCredentials) error
	DropDatabase(creds DBCredentials) error
}

var dbIdentRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,31}$`)

func newDatabaseEngine(req InstallRequest) (DatabaseEngine, string, error) {
	engine := strings.ToLower(strings.TrimSpace(req.DBEngine))
	if engine == "" {
		engine = DBMySQL
	}
	host, port := splitHostPort(req.DatabaseURL, defaultPortFor(engine))
	switch engine {
	case DBMySQL:
		bin := req.MySQLBinary
		if bin == "" {
			if found, err := exec.LookPath("mysql"); err == nil {
				bin = found
			} else if found, err := exec.LookPath("mariadb"); err == nil {
				bin = found
			} else {
				return nil, engine, fmt.Errorf("%w: mysql or mariadb", ErrDatabaseClientMissing)
			}
		}
		return &mysqlEngine{binary: bin, host: host, port: port, adminUser: req.DBAdminUser, adminPass: req.DBAdminPass}, engine, nil
	case DBPostgres:
		bin := req.PsqlBinary
		if bin == "" {
			found, err := exec.LookPath("psql")
			if err != nil {
				return nil, engine, fmt.Errorf("%w: psql", ErrDatabaseClientMissing)
			}
			bin = found
		}
		return &postgresEngine{binary: bin, host: host, port: port, adminUser: req.DBAdminUser, adminPass: req.DBAdminPass}, engine, nil
	default:
		return nil, engine, fmt.Errorf("%w: unsupported engine %q", ErrInvalidInput, engine)
	}
}

func defaultPortFor(engine string) string {
	if engine == DBPostgres {
		return "5432"
	}
	return "3306"
}

func splitHostPort(value, fallbackPort string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "127.0.0.1", fallbackPort
	}
	if strings.Contains(value, ":") {
		parts := strings.SplitN(value, ":", 2)
		if parts[0] == "" {
			return "127.0.0.1", parts[1]
		}
		return parts[0], parts[1]
	}
	return value, fallbackPort
}

func generatePassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// --- MySQL / MariaDB ---

type mysqlEngine struct {
	binary    string
	host      string
	port      string
	adminUser string
	adminPass string
}

func (c *mysqlEngine) run(script string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.binary,
		"--host="+c.host,
		"--port="+c.port,
		"--protocol=TCP",
		"--user="+c.adminUser,
		"--batch",
		"--skip-column-names",
		"--default-character-set=utf8mb4",
	)
	cmd.Stdin = strings.NewReader(script)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+c.adminPass)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", ErrDatabaseCallFailed, tailOf(out, 400))
	}
	return nil
}

func (c *mysqlEngine) CreateDatabase(creds DBCredentials) error {
	if !dbIdentRegex.MatchString(creds.Name) || !dbIdentRegex.MatchString(creds.User) {
		return fmt.Errorf("%w: database identifiers rejected by sanitizer", ErrInvalidInput)
	}
	if !regexp.MustCompile(`^[a-f0-9]{48,128}$`).MatchString(creds.Password) {
		return fmt.Errorf("%w: generated database password rejected by sanitizer", ErrInvalidInput)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;\n", creds.Name)
	for _, host := range mysqlAccountHosts {
		fmt.Fprintf(&b, "CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s';\n", creds.User, host, creds.Password)
		fmt.Fprintf(&b, "ALTER USER '%s'@'%s' IDENTIFIED BY '%s';\n", creds.User, host, creds.Password)
		fmt.Fprintf(&b, "GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%s';\n", creds.Name, creds.User, host)
	}
	b.WriteString("FLUSH PRIVILEGES;\n")
	return c.run(b.String())
}

// mysqlAccountHosts covers loopback connections both with and without reverse
// name resolution enabled on the server.
var mysqlAccountHosts = []string{"localhost", "127.0.0.1"}

func (c *mysqlEngine) DropDatabase(creds DBCredentials) error {
	if !dbIdentRegex.MatchString(creds.Name) || !dbIdentRegex.MatchString(creds.User) {
		return fmt.Errorf("%w: database identifiers rejected by sanitizer", ErrInvalidInput)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "DROP DATABASE IF EXISTS `%s`;\n", creds.Name)
	for _, host := range mysqlAccountHosts {
		fmt.Fprintf(&b, "DROP USER IF EXISTS '%s'@'%s';\n", creds.User, host)
	}
	b.WriteString("FLUSH PRIVILEGES;\n")
	return c.run(b.String())
}

// --- PostgreSQL ---

type postgresEngine struct {
	binary    string
	host      string
	port      string
	adminUser string
	adminPass string
}

func (c *postgresEngine) run(script string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.binary,
		"--host="+c.host,
		"--port="+c.port,
		"--username="+c.adminUser,
		"--no-password",
		"--set=ON_ERROR_STOP=1",
		"--dbname=postgres",
	)
	cmd.Stdin = strings.NewReader(script)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+c.adminPass)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", ErrDatabaseCallFailed, tailOf(out, 400))
	}
	return nil
}

func (c *postgresEngine) CreateDatabase(creds DBCredentials) error {
	if !dbIdentRegex.MatchString(creds.Name) || !dbIdentRegex.MatchString(creds.User) {
		return fmt.Errorf("%w: database identifiers rejected by sanitizer", ErrInvalidInput)
	}
	if !regexp.MustCompile(`^[a-f0-9]{48,128}$`).MatchString(creds.Password) {
		return fmt.Errorf("%w: generated database password rejected by sanitizer", ErrInvalidInput)
	}
	// CREATE DATABASE cannot run inside a transaction; psql autocommits each
	// statement, and the DO block makes role creation idempotent.
	script := fmt.Sprintf(
		"DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%s') THEN CREATE ROLE \"%s\" LOGIN; END IF; END $$;\n"+
			"ALTER ROLE \"%s\" WITH LOGIN PASSWORD '%s';\n"+
			"SELECT 'CREATE DATABASE \"%s\" OWNER \"%s\"' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '%s')\\gexec\n",
		creds.User, creds.User, creds.User, creds.Password, creds.Name, creds.User, creds.Name,
	)
	return c.run(script)
}

func (c *postgresEngine) DropDatabase(creds DBCredentials) error {
	if !dbIdentRegex.MatchString(creds.Name) || !dbIdentRegex.MatchString(creds.User) {
		return fmt.Errorf("%w: database identifiers rejected by sanitizer", ErrInvalidInput)
	}
	script := fmt.Sprintf(
		"SELECT 'DROP DATABASE IF EXISTS \"%s\"' WHERE EXISTS (SELECT FROM pg_database WHERE datname = '%s')\\gexec\n"+
			"DROP ROLE IF EXISTS \"%s\";\n",
		creds.Name, creds.Name, creds.User,
	)
	return c.run(script)
}

func tailOf(data []byte, max int) string {
	s := strings.TrimSpace(string(data))
	if len(s) > max {
		s = s[len(s)-max:]
	}
	return strings.ReplaceAll(s, "\n", " | ")
}

// copyDatabase moves an existing schema between two databases using the native
// dump and restore clients. Credentials travel through the environment. The
// dump is written to a private temporary file and removed afterwards.
func copyDatabase(engine, host, port, adminUser, adminPass, srcName, dstName string) error {
	tmp, err := os.CreateTemp("", "digipanel-dbcopy-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	switch engine {
	case DBMySQL:
		dumpBin, err := lookPathAny("mariadb-dump", "mysqldump")
		if err != nil {
			return err
		}
		restoreBin, err := lookPathAny("mariadb", "mysql")
		if err != nil {
			return err
		}
		dumpArgs := []string{
			"--host=" + host, "--port=" + port, "--protocol=TCP",
			"--user=" + adminUser, "--single-transaction", "--routines",
			"--triggers", "--no-tablespaces", "--default-character-set=utf8mb4",
			"--result-file=" + tmpPath, srcName,
		}
		if strings.Contains(filepath.Base(dumpBin), "mysqldump") {
			// Oracle's mysqldump queries COLUMN_STATISTICS, which MariaDB does
			// not expose. MariaDB's own dump never receives this flag.
			dumpArgs = append([]string{"--column-statistics=0"}, dumpArgs...)
		}
		if err := runWithEnv(dumpBin, dumpArgs, "MYSQL_PWD="+adminPass); err != nil {
			return fmt.Errorf("%w: dump %s: %v", ErrDatabaseCallFailed, srcName, err)
		}
		restoreArgs := []string{
			"--host=" + host, "--port=" + port, "--protocol=TCP",
			"--user=" + adminUser, "--batch", "--default-character-set=utf8mb4", dstName,
		}
		script, err := os.Open(tmpPath)
		if err != nil {
			return err
		}
		defer script.Close()
		if err := runWithStdin(restoreBin, restoreArgs, "MYSQL_PWD="+adminPass, script); err != nil {
			return fmt.Errorf("%w: restore %s: %v", ErrDatabaseCallFailed, dstName, err)
		}
		return nil
	case DBPostgres:
		dumpBin, err := exec.LookPath("pg_dump")
		if err != nil {
			return fmt.Errorf("%w: pg_dump", ErrDatabaseClientMissing)
		}
		restoreBin, err := exec.LookPath("pg_restore")
		if err != nil {
			return fmt.Errorf("%w: pg_restore", ErrDatabaseClientMissing)
		}
		dumpArgs := []string{
			"--host=" + host, "--port=" + port, "--username=" + adminUser,
			"--no-password", "--format=custom", "--file=" + tmpPath, srcName,
		}
		if err := runWithEnv(dumpBin, dumpArgs, "PGPASSWORD="+adminPass); err != nil {
			return fmt.Errorf("%w: dump %s: %v", ErrDatabaseCallFailed, srcName, err)
		}
		restoreArgs := []string{
			"--host=" + host, "--port=" + port, "--username=" + adminUser,
			"--no-password", "--dbname=" + dstName, "--no-owner", "--no-acl", tmpPath,
		}
		if err := runWithEnv(restoreBin, restoreArgs, "PGPASSWORD="+adminPass); err != nil {
			return fmt.Errorf("%w: restore %s: %v", ErrDatabaseCallFailed, dstName, err)
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported engine %q", ErrInvalidInput, engine)
	}
}

func lookPathAny(names ...string) (string, error) {
	for _, name := range names {
		if found, err := exec.LookPath(name); err == nil {
			return found, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrDatabaseClientMissing, strings.Join(names, " or "))
}

func runWithEnv(binary string, args []string, extraEnv string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = append(os.Environ(), extraEnv)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", tailOf(out, 400))
	}
	return nil
}

func runWithStdin(binary string, args []string, extraEnv string, stdin io.Reader) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = append(os.Environ(), extraEnv)
	cmd.Stdin = stdin
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", tailOf(out, 400))
	}
	return nil
}
