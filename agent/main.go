// DigiPanel Root Agent Daemon (agent/main.go)
// Listens exclusively on restricted Unix Domain Socket.
// Zero arbitrary shell execution: strict whitelist command dispatcher with array arguments.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	DefaultSocketPath = "/run/digipanel/agent.sock"
	DefaultSocketDir  = "/run/digipanel"
	SocketPerm        = 0660
	GroupName         = "digipanel"
)

var (
	safeIdentifierRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{1,64}$`)
	safeDomainRegex     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-\.]{1,253}[a-zA-Z0-9]$`)
	allowedServices     = map[string]bool{
		"nginx":      true,
		"powerdns":   true,
		"postgresql": true,
		"mariadb":    true,
		"php8.3-fpm": true,
		"postfix":    true,
		"dovecot":    true,
		"rspamd":     true,
		"fail2ban":   true,
		"nftables":   true,
	}
	allowedServiceActions = map[string]bool{
		"status":  true,
		"reload":  true,
		"restart": true,
	}
)

type AgentRequest struct {
	Action string            `json:"action"`
	Params map[string]string `json:"params"`
}

type AgentResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

func getSocketPath() string {
	if custom := os.Getenv("DIGIPANEL_AGENT_SOCKET"); custom != "" {
		return custom
	}
	return DefaultSocketPath
}

func validateInputParam(paramName, val string) error {
	if strings.ContainsAny(val, ";|&`$><\r\n\t") {
		return fmt.Errorf("parameter '%s' contains forbidden shell metacharacters", paramName)
	}
	if strings.Contains(val, "..") || strings.Contains(val, "/") || strings.Contains(val, "\\") {
		return fmt.Errorf("parameter '%s' contains forbidden path traversal characters", paramName)
	}
	if !safeIdentifierRegex.MatchString(val) {
		return fmt.Errorf("parameter '%s' does not match safe identifier regex", paramName)
	}
	return nil
}

func validateDomain(domain string) error {
	if strings.ContainsAny(domain, ";|&`$><\r\n\t /\\") {
		return errors.New("domain contains invalid characters")
	}
	if strings.Contains(domain, "..") {
		return errors.New("domain contains consecutive dots")
	}
	if !safeDomainRegex.MatchString(domain) {
		return errors.New("domain format invalid")
	}
	return nil
}

func executeSafeCommand(ctx context.Context, binary string, args []string) (string, error) {
	// Strictly no shell wrappers (/bin/sh -c is prohibited)
	cmd := exec.CommandContext(ctx, binary, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func handleAgentRequest(req AgentRequest) AgentResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	switch req.Action {
	case "ping":
		return AgentResponse{Success: true, Output: "pong"}

	case "system.status":
		if _, err := os.Stat("/proc/uptime"); err != nil {
			return AgentResponse{Success: false, Error: fmt.Sprintf("kernel procfs unavailable: %v", err)}
		}
		return AgentResponse{Success: true, Output: "systemd, procfs, and agent dispatcher operational"}

	case "account.create_vhost":
		return handleCreateVhost(ctx, req.Params)

	case "account.remove_vhost":
		return handleRemoveVhost(ctx, req.Params)

	case "account.suspend":
		return handleAccountSuspendState(req.Params, true)

	case "account.unsuspend":
		return handleAccountSuspendState(req.Params, false)

	case "ssl.renew":
		return handleSSLRenew(ctx, req.Params)

	case "service.action":
		return handleServiceAction(ctx, req.Params)

	case "user.create_posix":
		return handleCreatePosixUser(ctx, req.Params)

	case "user.delete_posix":
		return handleDeletePosixUser(ctx, req.Params)

	case "db.list", "db.create", "db.drop", "db.user.create", "db.user.drop", "db.user.password", "db.user.grant":
		return handleDBCommand(req.Action, req.Params)

	case "backup.create", "backup.list", "backup.verify", "backup.restore":
		return handleBackupCommand(req.Action, req.Params)

	case "firewall.apply", "firewall.inspect":
		return handleFirewallCommand(req.Action, req.Params)

	case "cgroups.limit":
		return handleCgroupsLimit(req.Params)

	default:
		return AgentResponse{
			Success: false,
			Error:   fmt.Sprintf("Action '%s' is not in the authorized agent whitelist", req.Action),
		}
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	var req AgentRequest
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&req); err != nil {
		resp := AgentResponse{Success: false, Error: "Malformed JSON payload"}
		_ = json.NewEncoder(conn).Encode(resp)
		return
	}

	resp := handleAgentRequest(req)
	_ = json.NewEncoder(conn).Encode(resp)
}

func main() {
	socketPath := getSocketPath()
	socketDir := filepath.Dir(socketPath)

	if os.Getenv("DIGIPANEL_AGENT_SOCKET") == "" && os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "FATAL: digipanel-agent must run with root privileges in production.\n")
		os.Exit(1)
	}

	if err := os.MkdirAll(socketDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create runtime socket directory %s: %v\n", socketDir, err)
		os.Exit(1)
	}

	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to listen on unix socket %s: %v\n", socketPath, err)
		os.Exit(1)
	}
	defer listener.Close()

	_ = os.Chmod(socketPath, SocketPerm)

	if grp, err := user.LookupGroup(GroupName); err == nil {
		if gid, err := strconv.Atoi(grp.Gid); err == nil {
			_ = os.Chown(socketPath, 0, gid)
		}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		_ = os.Remove(socketPath)
		os.Exit(0)
	}()

	fmt.Printf("DigiPanel Agent Daemon started. Listening on %s\n", socketPath)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn)
	}
}
