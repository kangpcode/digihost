// DigiPanel Host Inventory (core/inventory.go)
// Real host inspection for telemetry and service state. Every value comes from
// /proc, net interfaces, statfs, or a whitelisted systemctl probe. Nothing is
// fabricated and no shell string is ever executed.

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// --- CPU sampling ---------------------------------------------------------

type cpuSample struct {
	total uint64
	idle  uint64
}

var (
	cpuMu     sync.Mutex
	cpuPrev   cpuSample
	cpuPrimed bool
)

// readCPUSample returns the aggregate CPU ticks from /proc/stat.
func readCPUSample() (cpuSample, bool) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return cpuSample{}, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return cpuSample{}, false
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuSample{}, false
	}
	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return cpuSample{}, false
		}
		values = append(values, value)
	}
	if len(values) < 4 {
		return cpuSample{}, false
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	// guest and guest_nice are already included in user and nice.
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return cpuSample{total: total, idle: idle}, true
}

// cpuUsagePercent computes utilisation between the previous and current sample
// and keeps the current sample as the new baseline. The first observation
// reports utilisation since boot, which is exact rather than estimated.
func cpuUsagePercent() float64 {
	current, ok := readCPUSample()
	if !ok {
		return 0
	}

	cpuMu.Lock()
	defer cpuMu.Unlock()

	previous := cpuPrev
	primed := cpuPrimed
	cpuPrev = current
	cpuPrimed = true

	if !primed || current.total <= previous.total || current.total < previous.total {
		if current.total == 0 {
			return 0
		}
		used := float64(current.total-current.idle) / float64(current.total) * 100
		return roundPercent(used)
	}

	deltaTotal := current.total - previous.total
	deltaIdle := current.idle - previous.idle
	if deltaTotal == 0 {
		return 0
	}
	used := float64(deltaTotal-deltaIdle) / float64(deltaTotal) * 100
	if used < 0 {
		used = 0
	}
	if used > 100 {
		used = 100
	}
	return roundPercent(used)
}

func roundPercent(value float64) float64 {
	return float64(int(value*10+0.5)) / 10.0
}

// --- Process count --------------------------------------------------------

// processCount counts numeric entries in /proc, which each represent a live
// process. The value is exact for the host, not a goroutine estimate.
func processCount() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "" {
			continue
		}
		digits := true
		for _, r := range name {
			if r < '0' || r > '9' {
				digits = false
				break
			}
		}
		if digits {
			count++
		}
	}
	return count
}

// --- Runtime detection ----------------------------------------------------

// detectedRuntimes reports the interpreter versions actually installed on the
// host. Empty strings mean the runtime is absent.
func detectedRuntimes() map[string]string {
	return map[string]string{
		"nodeVersion":   probeVersion("node", "--version"),
		"pythonVersion": probeVersion("python3", "--version"),
		"goVersion":     probeVersion("go", "version"),
		"phpVersion":    probeVersion("php", "-v"),
	}
}

func probeVersion(binary string, args ...string) string {
	if _, err := exec.LookPath(binary); err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, binary, args...).Output()
	if err != nil && len(output) == 0 {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return ""
	}
	first := strings.TrimSpace(lines[0])
	if len(first) > 200 {
		first = first[:200]
	}
	return first
}

// --- Service inventory ----------------------------------------------------

// ServiceState is the real systemd state of a known hosting service.
type ServiceState struct {
	Unit      string `json:"unit"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Active    bool   `json:"active"`
	Enabled   bool   `json:"enabled"`
	State     string `json:"state"`
	Installed bool   `json:"installed"`
}

type serviceSpec struct {
	unit     string
	name     string
	category string
}

var managedServices = []serviceSpec{
	{unit: "nginx", name: "Nginx", category: "web"},
	{unit: "openlitespeed", name: "OpenLiteSpeed", category: "web"},
	{unit: "apache2", name: "Apache HTTPD", category: "web"},
	{unit: "php8.3-fpm", name: "PHP-FPM 8.3", category: "runtime"},
	{unit: "mariadb", name: "MariaDB", category: "database"},
	{unit: "mysql", name: "MySQL", category: "database"},
	{unit: "postgresql", name: "PostgreSQL", category: "database"},
	{unit: "postfix", name: "Postfix", category: "mail"},
	{unit: "dovecot", name: "Dovecot", category: "mail"},
	{unit: "rspamd", name: "Rspamd", category: "mail"},
	{unit: "pdns", name: "PowerDNS", category: "dns"},
	{unit: "fail2ban", name: "Fail2ban", category: "security"},
	{unit: "nftables", name: "nftables", category: "security"},
	{unit: "pure-ftpd", name: "Pure-FTPd", category: "ftp"},
	{unit: "cron", name: "Cron", category: "system"},
	{unit: "crond", name: "Cron", category: "system"},
	{unit: "sshd", name: "OpenSSH", category: "system"},
}

// probeServices reads the live systemd state for every managed unit.
func probeServices() ([]ServiceState, error) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil, fmt.Errorf("systemctl is not available on this host")
	}

	results := make([]ServiceState, 0, len(managedServices))
	for _, spec := range managedServices {
		state := ServiceState{
			Unit:     spec.unit,
			Name:     spec.name,
			Category: spec.category,
			State:    "unknown",
		}
		active, activeErr := systemctlQuery("is-active", spec.unit)
		enabled, enabledErr := systemctlQuery("is-enabled", spec.unit)

		state.State = active
		state.Active = active == "active"
		state.Enabled = enabled == "enabled"
		// A unit that systemd does not know about is not installed. Both probes
		// return "not-found" in that case.
		state.Installed = !(active == "not-found" || enabled == "not-found") &&
			activeErr == nil && enabledErr == nil
		if active == "not-found" || enabled == "not-found" {
			state.Installed = false
		}
		if !state.Installed {
			state.State = "not-installed"
		}
		results = append(results, state)
	}
	return results, nil
}

func systemctlQuery(verb, unit string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, "systemctl", verb, unit).Output()
	value := strings.TrimSpace(string(output))
	if value == "" && err != nil {
		return "unknown", err
	}
	return value, err
}
