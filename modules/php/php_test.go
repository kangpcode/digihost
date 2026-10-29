// DigiPanel PHP Manager Tests (modules/php/php_test.go)
// Exercises real interpreter detection and config file generation. When the
// php binary is absent from the test host the scenarios skip honestly.

package php

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func phpOnHost() bool {
	_, err := exec.LookPath("php")
	return err == nil
}

func TestDetectVersionsMatchesHost(t *testing.T) {
	m := NewManager(t.TempDir())
	versions := m.ListVersions()
	hostHasPHP := phpOnHost()

	if hostHasPHP && len(versions) == 0 {
		t.Fatalf("php binary present but manager detected no versions")
	}
	if !hostHasPHP && len(versions) != 0 {
		t.Fatalf("php binary absent but manager invented a version")
	}
	if hostHasPHP {
		out, err := exec.Command("php", "--version").CombinedOutput()
		if err != nil {
			t.Fatalf("php --version failed while detection claimed presence: %v", err)
		}
		line := strings.SplitN(string(out), "\n", 2)[0]
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			found := false
			for _, v := range versions {
				if v.Full == fields[1] {
					found = true
				}
				if v.BinaryPath == "" {
					t.Fatalf("detected version has empty binary path")
				}
				if len(v.Extensions) == 0 {
					t.Fatalf("real php -m inventory expected to be non-empty")
				}
			}
			if !found {
				t.Fatalf("expected detected version %s in %+v", fields[1], versions)
			}
		}
	}
}

func TestConfigureDomainWritesRealFiles(t *testing.T) {
	if !phpOnHost() {
		t.Skip("php binary not available on this host")
	}
	base := t.TempDir()
	m := NewManager(base)

	versions := m.ListVersions()
	var id string
	for _, v := range versions {
		id = v.ID
		break
	}
	if id == "" {
		t.Skip("no php version detected")
	}
	ver, err := m.GetVersion(id)
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	// choose up to two real extension names from the live inventory
	var requested []string
	for name := range ver.Extensions {
		if len(requested) == 2 {
			break
		}
		requested = append(requested, name)
	}

	req := ConfigureRequest{
		Domain:            "demo.digipanel.test",
		Version:           id,
		MemoryLimit:       "256M",
		MaxExecutionTime:  300,
		UploadMaxFilesize: "128M",
		Extensions:        requested,
		AllowUrlFopen:     "Off",
		UnixUser:          "webuser",
	}
	report, err := m.ConfigureDomain(req)
	if err != nil {
		t.Fatalf("ConfigureDomain: %v", err)
	}

	iniBytes, err := os.ReadFile(report.IniPath)
	if err != nil {
		t.Fatalf("ini file not written: %v", err)
	}
	ini := string(iniBytes)
	if !strings.Contains(ini, "memory_limit = 256M") {
		t.Errorf("ini missing memory_limit, got: %s", ini)
	}
	if !strings.Contains(ini, "max_execution_time = 300") {
		t.Errorf("ini missing max_execution_time")
	}
	if !strings.Contains(ini, "upload_max_filesize = 128M") {
		t.Errorf("ini missing upload_max_filesize")
	}
	if !strings.Contains(ini, "allow_url_fopen = Off") {
		t.Errorf("ini missing allow_url_fopen")
	}
	for _, ext := range requested {
		if !strings.Contains(ini, "extension = "+ext) {
			t.Errorf("ini missing requested extension %s", ext)
		}
	}

	poolBytes, err := os.ReadFile(report.PoolPath)
	if err != nil {
		t.Fatalf("pool file not written: %v", err)
	}
	pool := string(poolBytes)
	if !strings.Contains(pool, "[demo.digipanel.test]") {
		t.Errorf("pool missing section header")
	}
	if !strings.Contains(pool, "user = webuser") || !strings.Contains(pool, "group = webuser") {
		t.Errorf("pool missing pool owner directives")
	}
	if !strings.Contains(pool, "listen = ") {
		t.Errorf("pool missing socket listen directive")
	}

	cfg, err := m.GetDomainConfig("demo.digipanel.test")
	if err != nil {
		t.Fatalf("GetDomainConfig: %v", err)
	}
	if cfg.Version != id || cfg.UnixUser != "webuser" || len(cfg.Extensions) != len(requested) {
		t.Fatalf("stored config mismatch: %+v", cfg)
	}
}

func TestConfigureDomainValidation(t *testing.T) {
	m := NewManager(t.TempDir())

	cases := []struct {
		label string
		req   ConfigureRequest
	}{
		{"traversal domain", ConfigureRequest{Domain: "../escape.local", Version: "8.3", MemoryLimit: "256M", MaxExecutionTime: 300, UploadMaxFilesize: "128M", UnixUser: "webuser"}},
		{"bad memory", ConfigureRequest{Domain: "demo.test", Version: "8.3", MemoryLimit: "gigantic", MaxExecutionTime: 300, UploadMaxFilesize: "128M", UnixUser: "webuser"}},
		{"bad execution time", ConfigureRequest{Domain: "demo.test", Version: "8.3", MemoryLimit: "256M", MaxExecutionTime: 99999, UploadMaxFilesize: "128M", UnixUser: "webuser"}},
		{"bad upload", ConfigureRequest{Domain: "demo.test", Version: "8.3", MemoryLimit: "256M", MaxExecutionTime: 300, UploadMaxFilesize: "1000B", UnixUser: "webuser"}},
		{"missing user", ConfigureRequest{Domain: "demo.test", Version: "8.3", MemoryLimit: "256M", MaxExecutionTime: 300, UploadMaxFilesize: "128M", UnixUser: ""}},
		{"hostile user", ConfigureRequest{Domain: "demo.test", Version: "8.3", MemoryLimit: "256M", MaxExecutionTime: 300, UploadMaxFilesize: "128M", UnixUser: "root; rm -rf /"}},
		{"allow_url_fopen garbage", ConfigureRequest{Domain: "demo.test", Version: "8.3", MemoryLimit: "256M", MaxExecutionTime: 300, UploadMaxFilesize: "128M", UnixUser: "webuser", AllowUrlFopen: "maybe"}},
	}
	for _, tc := range cases {
		if _, err := m.ConfigureDomain(tc.req); err == nil {
			t.Errorf("%s: expected validation error, got nil", tc.label)
		}
	}

	// extension forgery: a name not provided by the real binary must fail
	if phpOnHost() {
		if _, err := m.ConfigureDomain(ConfigureRequest{
			Domain: "demo.test", Version: "8.3", MemoryLimit: "256M",
			MaxExecutionTime: 300, UploadMaxFilesize: "128M",
			UnixUser: "webuser", Extensions: []string{"fakespyext"},
		}); err == nil {
			t.Errorf("extension forgery accepted: unknown extension should fail validation")
		}
	} else {
		// unknown version still rejected deterministically on hosts without php
		if _, err := m.ConfigureDomain(ConfigureRequest{
			Domain: "demo.test", Version: "8.3", MemoryLimit: "256M",
			MaxExecutionTime: 300, UploadMaxFilesize: "128M", UnixUser: "webuser",
		}); err == nil {
			t.Errorf("expected ErrRuntimeNotFound on a php-less host")
		}
	}
}

func TestDomainConfigPersistsAcrossReload(t *testing.T) {
	if !phpOnHost() {
		t.Skip("php binary not available on this host")
	}
	base := t.TempDir()
	m1 := NewManager(base)

	var id string
	for _, v := range m1.ListVersions() {
		id = v.ID
		break
	}
	if id == "" {
		t.Skip("no php version detected")
	}
	req := ConfigureRequest{
		Domain: "persist.digipanel.test", Version: id,
		MemoryLimit: "512M", MaxExecutionTime: 180, UploadMaxFilesize: "64M",
		UnixUser: "webuser",
	}
	if _, err := m1.ConfigureDomain(req); err != nil {
		t.Fatalf("ConfigureDomain: %v", err)
	}

	m2 := NewManager(base)
	cfg, err := m2.GetDomainConfig("persist.digipanel.test")
	if err != nil {
		t.Fatalf("reloaded manager lost domain config: %v", err)
	}
	if cfg.MemoryLimit != "512M" || cfg.MaxExecutionTime != 180 {
		t.Fatalf("persisted values mismatch: %+v", cfg)
	}

	// generated configs must exist on disk after reload as well
	if _, err := os.Stat(filepath.Join(base, "ini", "persist.digipanel.test.ini")); err != nil {
		t.Fatalf("persisted ini missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "pools", "persist.digipanel.test.conf")); err != nil {
		t.Fatalf("persisted pool missing: %v", err)
	}
}
