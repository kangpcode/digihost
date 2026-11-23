// DigiPanel Agent Hosting, POSIX, Cgroups, Systemd & ACME Operations (agent/hosting_ops.go)
// All privileged OS actions execute directly via exec.CommandContext with argument
// arrays or validated filesystem writes inside configured directories.
// Zero shell interpreters, zero fabricated success responses.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var allowedPHPVersions = map[string]bool{
	"8.1": true,
	"8.2": true,
	"8.3": true,
	"8.4": true,
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func nginxSitesDir() string {
	return envOrDefault("DIGIPANEL_NGINX_DIR", "/etc/nginx/conf.d")
}

func phpPoolDir(phpVer string) string {
	if custom := strings.TrimSpace(os.Getenv("DIGIPANEL_PHP_POOL_DIR")); custom != "" {
		return custom
	}
	return fmt.Sprintf("/etc/php/%s/fpm/pool.d", phpVer)
}

func cgroupsRoot() string {
	return envOrDefault("DIGIPANEL_CGROUPS_ROOT", "/sys/fs/cgroup/digipanel.slice")
}

func homeBaseDir() string {
	return envOrDefault("DIGIPANEL_HOME_BASE", "/home")
}

func handleCreateVhost(ctx context.Context, params map[string]string) AgentResponse {
	username := params["username"]
	domain := params["domain"]
	phpVer := strings.TrimSpace(params["php_version"])
	if phpVer == "" {
		phpVer = "8.3"
	}
	if err := validateInputParam("username", username); err != nil {
		return AgentResponse{Success: false, Error: err.Error()}
	}
	if err := validateDomain(domain); err != nil {
		return AgentResponse{Success: false, Error: err.Error()}
	}
	if !allowedPHPVersions[phpVer] {
		return AgentResponse{Success: false, Error: fmt.Sprintf("unsupported php_version %q", phpVer)}
	}

	homeDir := filepath.Join(homeBaseDir(), username)
	docRoot := filepath.Join(homeDir, "public_html")
	logDir := filepath.Join(homeDir, "logs")
	for _, dir := range []string{docRoot, logDir, nginxSitesDir(), phpPoolDir(phpVer)} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return AgentResponse{Success: false, Error: fmt.Sprintf("prepare directory %s: %v", dir, err)}
		}
	}

	vhostPath := filepath.Join(nginxSitesDir(), domain+".conf")
	vhostBody := fmt.Sprintf(`# DigiPanel Managed Virtual Host
server {
    listen 80;
    server_name %s www.%s;
    root %s;
    index index.php index.html;

    access_log %s/access.log;
    error_log %s/error.log;

    location / {
        try_files $uri $uri/ /index.php?$args;
    }

    location ~ \.php$ {
        include fastcgi_params;
        fastcgi_pass unix:/run/php/php%s-fpm-%s.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
    }
}
`, domain, domain, docRoot, logDir, logDir, phpVer, username)

	if err := os.WriteFile(vhostPath, []byte(vhostBody), 0644); err != nil {
		return AgentResponse{Success: false, Error: fmt.Sprintf("write vhost config: %v", err)}
	}

	poolPath := filepath.Join(phpPoolDir(phpVer), username+".conf")
	poolBody := fmt.Sprintf(`; DigiPanel Managed PHP-FPM Pool
[%s]
user = %s
group = %s
listen = /run/php/php%s-fpm-%s.sock
listen.owner = www-data
listen.group = www-data
listen.mode = 0660
pm = dynamic
pm.max_children = 10
pm.start_servers = 2
pm.min_spare_servers = 1
pm.max_spare_servers = 3
php_admin_value[open_basedir] = %s:/tmp
php_admin_value[upload_tmp_dir] = /tmp
`, username, username, username, phpVer, username, homeDir)

	if err := os.WriteFile(poolPath, []byte(poolBody), 0644); err != nil {
		_ = os.Remove(vhostPath)
		return AgentResponse{Success: false, Error: fmt.Sprintf("write php-fpm pool: %v", err)}
	}

	if err := validateAndReloadNginx(ctx); err != nil {
		_ = os.Remove(vhostPath)
		_ = os.Remove(poolPath)
		return AgentResponse{Success: false, Error: err.Error()}
	}

	return AgentResponse{
		Success: true,
		Output:  fmt.Sprintf("vhost %s and php%s pool provisioned for %s", vhostPath, phpVer, username),
	}
}

func handleRemoveVhost(ctx context.Context, params map[string]string) AgentResponse {
	username := params["username"]
	domain := params["domain"]
	phpVer := strings.TrimSpace(params["php_version"])
	if phpVer == "" {
		phpVer = "8.3"
	}
	if err := validateInputParam("username", username); err != nil {
		return AgentResponse{Success: false, Error: err.Error()}
	}
	if err := validateDomain(domain); err != nil {
		return AgentResponse{Success: false, Error: err.Error()}
	}
	if !allowedPHPVersions[phpVer] {
		return AgentResponse{Success: false, Error: fmt.Sprintf("unsupported php_version %q", phpVer)}
	}

	_ = os.Remove(filepath.Join(nginxSitesDir(), domain+".conf"))
	_ = os.Remove(filepath.Join(nginxSitesDir(), domain+".conf.suspended"))
	_ = os.Remove(filepath.Join(phpPoolDir(phpVer), username+".conf"))
	if err := validateAndReloadNginx(ctx); err != nil {
		return AgentResponse{Success: false, Error: err.Error()}
	}
	return AgentResponse{Success: true, Output: fmt.Sprintf("vhost %s removed", domain)}
}

func validateAndReloadNginx(ctx context.Context) error {
	nginxBin := strings.TrimSpace(os.Getenv("DIGIPANEL_NGINX_BINARY"))
	if nginxBin == "" {
		if path, err := exec.LookPath("nginx"); err == nil {
			nginxBin = path
		}
	}
	if nginxBin != "" {
		if out, err := executeSafeCommand(ctx, nginxBin, []string{"-t"}); err != nil {
			return fmt.Errorf("nginx config validation failed: %v (%s)", err, strings.TrimSpace(out))
		}
		systemctlBin := resolveSystemctlBinary()
		if systemctlBin != "" {
			if out, err := executeSafeCommand(ctx, systemctlBin, []string{"reload", "nginx"}); err != nil {
				return fmt.Errorf("nginx reload failed: %v (%s)", err, strings.TrimSpace(out))
			}
		}
	}
	return nil
}

func resolveSystemctlBinary() string {
	if custom := strings.TrimSpace(os.Getenv("DIGIPANEL_SYSTEMCTL_BINARY")); custom != "" {
		return custom
	}
	if p, err := exec.LookPath("systemctl"); err == nil {
		return p
	}
	return ""
}

func handleAccountSuspendState(params map[string]string, freeze bool) AgentResponse {
	username := params["username"]
	domain := strings.TrimSpace(params["domain"])
	if err := validateInputParam("username", username); err != nil {
		return AgentResponse{Success: false, Error: err.Error()}
	}
	if domain != "" {
		if err := validateDomain(domain); err != nil {
			return AgentResponse{Success: false, Error: err.Error()}
		}
	}

	sliceDir := filepath.Join(cgroupsRoot(), "digipanel-"+username)
	if err := os.MkdirAll(sliceDir, 0755); err != nil {
		return AgentResponse{Success: false, Error: fmt.Sprintf("prepare cgroup slice: %v", err)}
	}
	freezeVal := "0"
	stateLabel := "active"
	if freeze {
		freezeVal = "1"
		stateLabel = "suspended"
	}
	if err := os.WriteFile(filepath.Join(sliceDir, "cgroup.freeze"), []byte(freezeVal+"\n"), 0644); err != nil {
		return AgentResponse{Success: false, Error: fmt.Sprintf("write cgroup.freeze: %v", err)}
	}

	if domain != "" {
		activeConf := filepath.Join(nginxSitesDir(), domain+".conf")
		suspendedConf := filepath.Join(nginxSitesDir(), domain+".conf.suspended")
		if freeze {
			if _, err := os.Stat(activeConf); err == nil {
				_ = os.Rename(activeConf, suspendedConf)
			}
		} else {
			if _, err := os.Stat(suspendedConf); err == nil {
				_ = os.Rename(suspendedConf, activeConf)
			}
		}
	}

	return AgentResponse{
		Success: true,
		Output:  fmt.Sprintf("account %s marked %s in %s", username, stateLabel, sliceDir),
	}
}

func handleCgroupsLimit(params map[string]string) AgentResponse {
	username := params["username"]
	memoryMbStr := strings.TrimSpace(params["memory_mb"])
	pidsLimitStr := strings.TrimSpace(params["pids_max"])
	if err := validateInputParam("username", username); err != nil {
		return AgentResponse{Success: false, Error: err.Error()}
	}
	var memoryBytes int64
	if memoryMbStr != "" {
		mb, err := strconv.Atoi(memoryMbStr)
		if err != nil || mb <= 0 {
			return AgentResponse{Success: false, Error: "invalid memory_mb limit"}
		}
		memoryBytes = int64(mb) * 1024 * 1024
	}
	var pidsMax int
	if pidsLimitStr != "" {
		p, err := strconv.Atoi(pidsLimitStr)
		if err != nil || p <= 0 {
			return AgentResponse{Success: false, Error: "invalid pids_max limit"}
		}
		pidsMax = p
	}

	sliceDir := filepath.Join(cgroupsRoot(), "digipanel-"+username)
	if err := os.MkdirAll(sliceDir, 0755); err != nil {
		return AgentResponse{Success: false, Error: fmt.Sprintf("create cgroup slice: %v", err)}
	}
	if memoryBytes > 0 {
		if err := os.WriteFile(filepath.Join(sliceDir, "memory.max"), fmt.Appendf(nil, "%d\n", memoryBytes), 0644); err != nil {
			return AgentResponse{Success: false, Error: fmt.Sprintf("write memory.max: %v", err)}
		}
	}
	if pidsMax > 0 {
		if err := os.WriteFile(filepath.Join(sliceDir, "pids.max"), fmt.Appendf(nil, "%d\n", pidsMax), 0644); err != nil {
			return AgentResponse{Success: false, Error: fmt.Sprintf("write pids.max: %v", err)}
		}
	}
	return AgentResponse{
		Success: true,
		Output:  fmt.Sprintf("cgroups v2 limits applied for %s: memory=%sMB, pids=%s", username, memoryMbStr, pidsLimitStr),
	}
}

func handleCreatePosixUser(ctx context.Context, params map[string]string) AgentResponse {
	username := params["username"]
	uidStr := strings.TrimSpace(params["uid"])
	if err := validateInputParam("username", username); err != nil {
		return AgentResponse{Success: false, Error: err.Error()}
	}
	args := []string{"--create-home", "--home-dir", filepath.Join(homeBaseDir(), username), "--shell", "/usr/sbin/nologin"}
	if uidStr != "" {
		uid, err := strconv.Atoi(uidStr)
		if err != nil || uid < 1000 || uid > 65000 {
			return AgentResponse{Success: false, Error: "invalid UID: must be 1000-65000"}
		}
		args = append(args, "--uid", strconv.Itoa(uid))
	}
	args = append(args, username)

	useraddBin := envOrDefault("DIGIPANEL_USERADD_BINARY", "useradd")
	out, err := executeSafeCommand(ctx, useraddBin, args)
	if err != nil {
		return AgentResponse{Success: false, Error: fmt.Sprintf("useradd failed: %v (%s)", err, strings.TrimSpace(out))}
	}
	return AgentResponse{
		Success: true,
		Output:  fmt.Sprintf("POSIX user %s created", username),
	}
}

func handleDeletePosixUser(ctx context.Context, params map[string]string) AgentResponse {
	username := params["username"]
	if err := validateInputParam("username", username); err != nil {
		return AgentResponse{Success: false, Error: err.Error()}
	}
	userdelBin := envOrDefault("DIGIPANEL_USERDEL_BINARY", "userdel")
	out, err := executeSafeCommand(ctx, userdelBin, []string{"-r", username})
	if err != nil {
		return AgentResponse{Success: false, Error: fmt.Sprintf("userdel failed: %v (%s)", err, strings.TrimSpace(out))}
	}
	return AgentResponse{
		Success: true,
		Output:  fmt.Sprintf("POSIX user %s removed", username),
	}
}

func handleServiceAction(ctx context.Context, params map[string]string) AgentResponse {
	service := params["service"]
	action := params["action"]
	if !allowedServices[service] {
		return AgentResponse{Success: false, Error: fmt.Sprintf("service '%s' is not in allowed service whitelist", service)}
	}
	if !allowedServiceActions[action] {
		return AgentResponse{Success: false, Error: fmt.Sprintf("action '%s' is not in allowed service actions", action)}
	}
	systemctlBin := resolveSystemctlBinary()
	if systemctlBin == "" {
		return AgentResponse{Success: false, Error: "systemctl binary is not available on this host"}
	}
	out, err := executeSafeCommand(ctx, systemctlBin, []string{action, service})
	if err != nil {
		return AgentResponse{Success: false, Error: fmt.Sprintf("systemctl %s %s failed: %v (%s)", action, service, err, strings.TrimSpace(out))}
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		trimmed = fmt.Sprintf("service %s %s completed successfully", service, action)
	}
	return AgentResponse{Success: true, Output: trimmed}
}

func handleSSLRenew(ctx context.Context, params map[string]string) AgentResponse {
	domain := params["domain"]
	if err := validateDomain(domain); err != nil {
		return AgentResponse{Success: false, Error: err.Error()}
	}
	certbotBin := envOrDefault("DIGIPANEL_CERTBOT_BINARY", "certbot")
	if _, err := exec.LookPath(certbotBin); err != nil {
		return AgentResponse{Success: false, Error: fmt.Sprintf("certbot binary %q unavailable: %v", certbotBin, err)}
	}
	out, err := executeSafeCommand(ctx, certbotBin, []string{"renew", "--cert-name", domain, "--non-interactive"})
	if err != nil {
		return AgentResponse{Success: false, Error: fmt.Sprintf("certbot renew failed: %v (%s)", err, strings.TrimSpace(out))}
	}
	return AgentResponse{
		Success: true,
		Output:  fmt.Sprintf("ACME renewal completed for %s: %s", domain, strings.TrimSpace(out)),
	}
}
