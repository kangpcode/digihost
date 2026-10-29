// DigiPanel Agent privileged operations (agent/ops.go)
// Database, backup, and firewall actions executed by the root daemon only.
// Parameters arrive as a validated map; the manager packages perform all
// identifier and path validation before a binary is executed. No caller can
// submit SQL, nft syntax, or an arbitrary path.
// Zero emojis, strictly production grade.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"digipanel/modules/backup"
	"digipanel/modules/db"
	"digipanel/modules/firewall"
)

const (
	defaultBackupDir = "/var/lib/digipanel/backups"
	defaultHomeBase  = "/home"
	backupTimeout    = 30 * time.Minute
	opsTimeout       = 60 * time.Second
)

func newDBManager() *db.Manager {
	return db.NewManager(db.Options{
		Engine:        os.Getenv("DIGIPANEL_DB_ENGINE"),
		Binary:        os.Getenv("DIGIPANEL_MYSQL_BINARY"),
		Host:          os.Getenv("DIGIPANEL_DB_HOST"),
		Port:          os.Getenv("DIGIPANEL_DB_PORT"),
		AdminUser:     os.Getenv("DIGIPANEL_DB_ADMIN_USER"),
		AdminPassword: os.Getenv("DIGIPANEL_DB_ADMIN_PASSWORD"),
	})
}

func handleDBCommand(action string, params map[string]string) AgentResponse {
	manager := newDBManager()
	if !manager.Configured() {
		return AgentResponse{Success: false, Error: db.ErrNotConfigured.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), opsTimeout)
	defer cancel()

	owner := params["owner"]
	name := params["name"]

	switch action {
	case "db.list":
		databases, err := manager.ListDatabases(ctx, owner)
		if err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		users, err := manager.ListUsers(ctx, owner)
		if err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		payload, err := json.Marshal(map[string]any{"databases": databases, "users": users})
		if err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		return AgentResponse{Success: true, Output: string(payload)}

	case "db.create":
		created, err := manager.CreateDatabase(ctx, owner, name)
		if err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		payload, _ := json.Marshal(created)
		return AgentResponse{Success: true, Output: string(payload)}

	case "db.drop":
		if err := manager.DropDatabase(ctx, owner, name); err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		return AgentResponse{Success: true, Output: "database dropped"}

	case "db.user.create":
		created, err := manager.CreateUser(ctx, owner, name, params["password"], params["database"])
		if err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		payload, _ := json.Marshal(created)
		return AgentResponse{Success: true, Output: string(payload)}

	case "db.user.drop":
		if err := manager.DropUser(ctx, owner, name); err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		return AgentResponse{Success: true, Output: "database user dropped"}

	case "db.user.password":
		if err := manager.SetPassword(ctx, owner, name, params["password"]); err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		return AgentResponse{Success: true, Output: "password rotated"}

	case "db.user.grant":
		if err := manager.Grant(ctx, owner, name, params["database"]); err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		return AgentResponse{Success: true, Output: "privileges granted"}
	}
	return AgentResponse{Success: false, Error: "unsupported database action"}
}

func newBackupEngine() *backup.Engine {
	root := strings.TrimSpace(os.Getenv("DIGIPANEL_BACKUP_DIR"))
	if root == "" {
		root = defaultBackupDir
	}
	homeBase := strings.TrimSpace(os.Getenv("DIGIPANEL_HOME_BASE"))
	if homeBase == "" {
		homeBase = defaultHomeBase
	}
	return backup.NewEngine(backup.Options{
		Root:       root,
		Passphrase: os.Getenv("DIGIPANEL_BACKUP_PASSPHRASE"),
		HomeBase:   homeBase,
	})
}

// confineToHomeBase proves a caller supplied path stays inside the configured
// home base before the archive engine touches it.
func confineToHomeBase(path string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" || clean == "." || !filepath.IsAbs(clean) {
		return "", fmt.Errorf("path must be absolute")
	}
	base := strings.TrimSpace(os.Getenv("DIGIPANEL_HOME_BASE"))
	if base == "" {
		base = defaultHomeBase
	}
	relative, err := filepath.Rel(filepath.Clean(base), clean)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes the configured home base")
	}
	return clean, nil
}

func handleBackupCommand(action string, params map[string]string) AgentResponse {
	engine := newBackupEngine()
	if !engine.Configured() {
		return AgentResponse{Success: false, Error: backup.ErrNoPassphrase.Error()}
	}
	account := params["account"]

	switch action {
	case "backup.create":
		source, err := confineToHomeBase(params["source"])
		if err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), backupTimeout)
		defer cancel()
		archive, err := engine.Create(ctx, account, source, params["label"])
		if err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		payload, _ := json.Marshal(archive)
		return AgentResponse{Success: true, Output: string(payload)}

	case "backup.list":
		archives, err := engine.List(account)
		if err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		payload, _ := json.Marshal(archives)
		return AgentResponse{Success: true, Output: string(payload)}

	case "backup.verify":
		if err := engine.Verify(account, params["id"]); err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		return AgentResponse{Success: true, Output: "archive verified"}

	case "backup.restore":
		target, err := confineToHomeBase(params["target"])
		if err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), backupTimeout)
		defer cancel()
		result, err := engine.Restore(ctx, account, params["id"], target)
		if err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		payload, _ := json.Marshal(result)
		return AgentResponse{Success: true, Output: string(payload)}
	}
	return AgentResponse{Success: false, Error: "unsupported backup action"}
}

func handleFirewallCommand(action string, params map[string]string) AgentResponse {
	ctx, cancel := context.WithTimeout(context.Background(), opsTimeout)
	defer cancel()
	runner := &firewall.LocalRunner{Binary: os.Getenv("DIGIPANEL_NFT_BINARY")}

	switch action {
	case "firewall.apply":
		raw := strings.TrimSpace(params["targets"])
		rules := make([]firewall.Rule, 0, 32)
		if raw != "" {
			for _, item := range strings.Split(raw, ",") {
				target, family, err := firewall.ValidateTarget(item)
				if err != nil {
					return AgentResponse{Success: false, Error: err.Error()}
				}
				rules = append(rules, firewall.Rule{Target: target, Family: family})
			}
		}
		if err := runner.Apply(ctx, rules); err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		return AgentResponse{Success: true, Output: fmt.Sprintf("applied %d blocked targets", len(rules))}

	case "firewall.inspect":
		output, err := runner.Inspect(ctx)
		if err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
		return AgentResponse{Success: true, Output: output}
	}
	return AgentResponse{Success: false, Error: "unsupported firewall action"}
}
