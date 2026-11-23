// DigiPanel Agent Security and Whitelist Unit Tests (agent/agent_test.go)
// Verifies:
// 1. Zero shell execution: no meta-character execution
// 2. Strict whitelist dispatch
// 3. Rejection of path traversal attempts
// 4. Input fuzzing against command injection payloads

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAgentProbeScript(t *testing.T, dir, name, logPath string) string {
	t.Helper()
	scriptPath := filepath.Join(dir, name)
	body := "#!/usr/bin/env bash\necho \"$0 $*\" >> " + logPath + "\nexit 0\n"
	if err := os.WriteFile(scriptPath, []byte(body), 0755); err != nil {
		t.Fatalf("write probe script %s: %v", name, err)
	}
	return scriptPath
}

func TestAgentPing(t *testing.T) {
	req := AgentRequest{Action: "ping"}
	resp := handleAgentRequest(req)
	if !resp.Success || resp.Output != "pong" {
		t.Fatalf("Expected pong, got %+v", resp)
	}
}

func TestAgentWhitelistEnforcement(t *testing.T) {
	unauthorizedActions := []string{
		"exec.shell",
		"system.reboot",
		"user.delete_all",
		"rm -rf /",
		"cat /etc/shadow",
		"",
	}

	for _, action := range unauthorizedActions {
		req := AgentRequest{Action: action}
		resp := handleAgentRequest(req)
		if resp.Success {
			t.Errorf("Security violation: unauthorized action '%s' was permitted", action)
		}
	}
}

func TestAgentInputFuzzingShellMetacharacters(t *testing.T) {
	fuzzPayloads := []string{
		"admin; rm -rf /",
		"user && id",
		"user || whoami",
		"test`cat /etc/passwd`",
		"test$(whoami)",
		"test\ncat /etc/shadow",
		"test>out.txt",
		"test<in.txt",
		"test|grep root",
		"user;id;whoami",
	}

	for _, payload := range fuzzPayloads {
		req := AgentRequest{
			Action: "account.create_vhost",
			Params: map[string]string{
				"username": payload,
				"domain":   "example.com",
			},
		}
		resp := handleAgentRequest(req)
		if resp.Success {
			t.Errorf("Security violation: shell metacharacter payload '%s' was allowed", payload)
		}
	}
}

func TestAgentPathTraversal(t *testing.T) {
	traversalPayloads := []string{
		"../../etc/passwd",
		"../root",
		"/etc/nginx/sites-enabled",
		"test/../../shadow",
		"test\\..\\windows",
	}

	for _, payload := range traversalPayloads {
		req := AgentRequest{
			Action: "account.create_vhost",
			Params: map[string]string{
				"username": payload,
				"domain":   "example.com",
			},
		}
		resp := handleAgentRequest(req)
		if resp.Success {
			t.Errorf("Security violation: path traversal payload '%s' was allowed", payload)
		}
	}
}

func TestAgentDomainValidation(t *testing.T) {
	invalidDomains := []string{
		"example.com; rm -rf",
		"-bad-domain.com",
		"domain..com",
		"example.com/path",
		"http://example.com",
		"domain with spaces.com",
	}

	for _, d := range invalidDomains {
		req := AgentRequest{
			Action: "ssl.renew",
			Params: map[string]string{"domain": d},
		}
		resp := handleAgentRequest(req)
		if resp.Success {
			t.Errorf("Security violation: invalid domain format '%s' was allowed", d)
		}
	}
}

func TestAgentValidActions(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "exec.log")
	nginxDir := filepath.Join(tmp, "nginx")
	phpDir := filepath.Join(tmp, "php")
	cgroupsDir := filepath.Join(tmp, "cgroup")
	homeBase := filepath.Join(tmp, "home")

	t.Setenv("DIGIPANEL_NGINX_DIR", nginxDir)
	t.Setenv("DIGIPANEL_PHP_POOL_DIR", phpDir)
	t.Setenv("DIGIPANEL_CGROUPS_ROOT", cgroupsDir)
	t.Setenv("DIGIPANEL_HOME_BASE", homeBase)
	t.Setenv("DIGIPANEL_NGINX_BINARY", writeAgentProbeScript(t, tmp, "nginx", logPath))
	t.Setenv("DIGIPANEL_SYSTEMCTL_BINARY", writeAgentProbeScript(t, tmp, "systemctl", logPath))
	t.Setenv("DIGIPANEL_USERADD_BINARY", writeAgentProbeScript(t, tmp, "useradd", logPath))
	t.Setenv("DIGIPANEL_USERDEL_BINARY", writeAgentProbeScript(t, tmp, "userdel", logPath))
	t.Setenv("DIGIPANEL_CERTBOT_BINARY", writeAgentProbeScript(t, tmp, "certbot", logPath))

	validReqs := []AgentRequest{
		{
			Action: "account.create_vhost",
			Params: map[string]string{
				"username":    "client101",
				"domain":      "example.com",
				"php_version": "8.3",
			},
		},
		{
			Action: "service.action",
			Params: map[string]string{
				"service": "nginx",
				"action":  "status",
			},
		},
		{
			Action: "user.create_posix",
			Params: map[string]string{
				"username": "client101",
				"uid":      "2001",
			},
		},
		{
			Action: "cgroups.limit",
			Params: map[string]string{
				"username":  "client101",
				"memory_mb": "512",
				"pids_max":  "100",
			},
		},
		{
			Action: "account.suspend",
			Params: map[string]string{
				"username": "client101",
				"domain":   "example.com",
			},
		},
		{
			Action: "account.unsuspend",
			Params: map[string]string{
				"username": "client101",
				"domain":   "example.com",
			},
		},
		{
			Action: "ssl.renew",
			Params: map[string]string{
				"domain": "example.com",
			},
		},
		{
			Action: "system.status",
		},
	}

	for _, req := range validReqs {
		resp := handleAgentRequest(req)
		if !resp.Success {
			t.Errorf("Expected valid action '%s' to succeed, got error: %s", req.Action, resp.Error)
		}
	}

	if _, err := os.Stat(filepath.Join(nginxDir, "example.com.conf")); err != nil {
		t.Errorf("expected vhost file to exist after unsuspend: %v", err)
	}
	if _, err := os.Stat(filepath.Join(phpDir, "client101.conf")); err != nil {
		t.Errorf("expected php-fpm pool file to exist: %v", err)
	}
	memRaw, err := os.ReadFile(filepath.Join(cgroupsDir, "digipanel-client101", "memory.max"))
	if err != nil || strings.TrimSpace(string(memRaw)) != "536870912" {
		t.Errorf("expected memory.max=536870912, got %q (err=%v)", string(memRaw), err)
	}
	execLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read exec log: %v", err)
	}
	for _, needle := range []string{"-t", "status nginx", "--uid 2001", "renew --cert-name example.com"} {
		if !strings.Contains(string(execLog), needle) {
			t.Errorf("expected executed binary log to contain %q, log:\n%s", needle, string(execLog))
		}
	}

	// Clean up vhost + user
	for _, req := range []AgentRequest{
		{Action: "account.remove_vhost", Params: map[string]string{"username": "client101", "domain": "example.com", "php_version": "8.3"}},
		{Action: "user.delete_posix", Params: map[string]string{"username": "client101"}},
	} {
		if resp := handleAgentRequest(req); !resp.Success {
			t.Errorf("cleanup action %s failed: %s", req.Action, resp.Error)
		}
	}
}
