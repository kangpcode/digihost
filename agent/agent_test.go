// DigiPanel Agent Security and Whitelist Unit Tests (agent/agent_test.go)
// Verifies:
// 1. Zero shell execution: no meta-character execution
// 2. Strict whitelist dispatch
// 3. Rejection of path traversal attempts
// 4. Input fuzzing against command injection payloads

package main

import (
	"testing"
)

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
	validReqs := []AgentRequest{
		{
			Action: "account.create_vhost",
			Params: map[string]string{
				"username": "client101",
				"domain":   "example.com",
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
			Action: "system.status",
		},
	}

	for _, req := range validReqs {
		resp := handleAgentRequest(req)
		if !resp.Success {
			t.Errorf("Expected valid action '%s' to succeed, got error: %s", req.Action, resp.Error)
		}
	}
}
