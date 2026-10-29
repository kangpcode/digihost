// DigiPanel Firewall Manager (modules/firewall/firewall.go)
// nftables blocklist management. Rules are persisted with an explicit expiry,
// rendered into a deterministic ruleset, and applied atomically through the
// privileged agent. The panel never accepts raw nft syntax from a caller.
// Zero emojis, strictly production grade.

package firewall

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	rulesFile   = "rules.json"
	tableName   = "digipanel"
	maxRules    = 4096
	maxReason   = 160
	maxTTLHours = 24 * 365
)

var (
	ErrInvalidTarget = errors.New("target must be a valid IPv4, IPv6, or CIDR block")
	ErrInvalidReason = errors.New("reason must be 3-160 printable characters")
	ErrRuleNotFound  = errors.New("firewall rule not found")
	ErrRuleExists    = errors.New("an active rule already covers this target")
	ErrTooManyRules  = errors.New("firewall rule limit reached")
	ErrRunnerMissing = errors.New("nftables runner is not available")
	ErrInvalidTTL    = errors.New("durationHours must be between 1 and 8760")
	controlPattern   = regexp.MustCompile(`[\x00-\x1f\x7f]`)
)

// Rule is one blocklist entry managed by the panel.
type Rule struct {
	ID        string     `json:"id"`
	Target    string     `json:"target"`
	Family    string     `json:"family"`
	Reason    string     `json:"reason"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// Status reports the applied state of the managed table.
type Status struct {
	AgentAvailable bool      `json:"agentAvailable"`
	Applied        bool      `json:"applied"`
	Table          string    `json:"table"`
	ActiveRules    int       `json:"activeRules"`
	LastAppliedAt  time.Time `json:"lastAppliedAt,omitempty"`
	LastError      string    `json:"lastError,omitempty"`
	Ruleset        string    `json:"ruleset"`
}

// Runner applies the panel blocklist. The agent backs this in production and
// renders the nft script inside the privileged process; tests inject a recorder
// so the generated rules can be asserted without root.
type Runner interface {
	Apply(ctx context.Context, rules []Rule) error
	Inspect(ctx context.Context) (string, error)
}

// LocalRunner executes the nft binary directly. It requires root, so the panel
// itself never uses it; the privileged agent does.
type LocalRunner struct {
	Binary string
}

func (r *LocalRunner) binary() string {
	if r.Binary != "" {
		return r.Binary
	}
	for _, candidate := range []string{"/usr/sbin/nft", "/sbin/nft", "nft"} {
		if found, err := exec.LookPath(candidate); err == nil {
			return found
		}
	}
	return "nft"
}

func (r *LocalRunner) Apply(ctx context.Context, rules []Rule) error {
	cmd := exec.CommandContext(ctx, r.binary(), "-f", "-")
	cmd.Stdin = strings.NewReader(RenderRuleset(rules))
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("nft apply failed: %s", message)
	}
	return nil
}

func (r *LocalRunner) Inspect(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, r.binary(), "list", "table", "inet", tableName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("nft inspect failed: %s", strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

// Manager stores rules and coordinates application.
type Manager struct {
	dir       string
	runner    Runner
	now       func() time.Time
	applied   bool
	lastErr   string
	appliedAt time.Time
}

// NewManager builds a manager rooted at dir. A nil runner leaves the manager in
// a read-only state that reports the agent as unavailable.
func NewManager(dir string, runner Runner) *Manager {
	return &Manager{dir: dir, runner: runner, now: time.Now}
}

// SetRunner swaps the runner, used by the core when the agent becomes reachable.
func (m *Manager) SetRunner(runner Runner) {
	m.runner = runner
}

func (m *Manager) rulesPath() string {
	return filepath.Join(m.dir, rulesFile)
}

// ValidateTarget normalises an IPv4, IPv6, or CIDR value.
func ValidateTarget(value string) (string, string, error) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return "", "", ErrInvalidTarget
	}
	if strings.Contains(clean, "/") {
		ip, block, err := net.ParseCIDR(clean)
		if err != nil || block == nil {
			return "", "", ErrInvalidTarget
		}
		family := "ipv4"
		if ip.To4() == nil {
			family = "ipv6"
		}
		return block.String(), family, nil
	}
	ip := net.ParseIP(clean)
	if ip == nil {
		return "", "", ErrInvalidTarget
	}
	if ip.To4() != nil {
		return ip.To4().String(), "ipv4", nil
	}
	return ip.String(), "ipv6", nil
}

// ValidateReason rejects control characters and empty reasons.
func ValidateReason(reason string) error {
	clean := strings.TrimSpace(reason)
	if len(clean) < 3 || len(clean) > maxReason || controlPattern.MatchString(clean) {
		return ErrInvalidReason
	}
	return nil
}

func covers(a, b string) bool {
	_, aBlock, aErr := net.ParseCIDR(a)
	_, bBlock, bErr := net.ParseCIDR(b)
	if aErr == nil && bErr == nil {
		return aBlock.Contains(bBlock.IP) && bBlock.Contains(aBlock.IP)
	}
	if aErr == nil {
		ip := net.ParseIP(b)
		return ip != nil && aBlock.Contains(ip)
	}
	if bErr == nil {
		ip := net.ParseIP(a)
		return ip != nil && bBlock.Contains(ip)
	}
	return a == b
}

// Rules returns the live rules, dropping entries whose expiry has passed.
func (m *Manager) Rules() ([]Rule, error) {
	items, err := m.load()
	if err != nil {
		return nil, err
	}
	now := m.now()
	active := make([]Rule, 0, len(items))
	dropped := false
	for _, item := range items {
		if item.ExpiresAt != nil && !item.ExpiresAt.After(now) {
			dropped = true
			continue
		}
		active = append(active, item)
	}
	if dropped {
		if err := m.save(active); err != nil {
			return nil, err
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].CreatedAt.After(active[j].CreatedAt) })
	return active, nil
}

func (m *Manager) load() ([]Rule, error) {
	content, err := os.ReadFile(m.rulesPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Rule{}, nil
		}
		return nil, err
	}
	var items []Rule
	if err := json.Unmarshal(content, &items); err != nil {
		return nil, fmt.Errorf("parse firewall rules: %w", err)
	}
	return items, nil
}

func (m *Manager) save(items []Rule) error {
	if err := os.MkdirAll(m.dir, 0700); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	path := m.rulesPath()
	temporary := path + ".partial"
	if err := os.WriteFile(temporary, append(encoded, '\n'), 0600); err != nil {
		return fmt.Errorf("write firewall rules: %w", err)
	}
	return os.Rename(temporary, path)
}

// Block adds a target and applies the ruleset. The write is persisted before
// the ruleset is applied, so a failed apply leaves an auditable pending rule
// rather than a silently dropped one.
func (m *Manager) Block(ctx context.Context, target, reason string, ttl time.Duration) (*Rule, error) {
	normalised, family, err := ValidateTarget(target)
	if err != nil {
		return nil, err
	}
	if err := ValidateReason(reason); err != nil {
		return nil, err
	}
	if ttl < 0 {
		return nil, ErrInvalidTTL
	}

	items, err := m.Rules()
	if err != nil {
		return nil, err
	}
	if len(items) >= maxRules {
		return nil, ErrTooManyRules
	}
	for _, item := range items {
		if covers(item.Target, normalised) {
			return nil, ErrRuleExists
		}
	}

	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return nil, err
	}
	now := m.now().UTC()
	rule := Rule{
		ID:        "fw-" + hex.EncodeToString(suffix),
		Target:    normalised,
		Family:    family,
		Reason:    strings.TrimSpace(reason),
		CreatedAt: now,
	}
	if ttl > 0 {
		expiry := now.Add(ttl)
		rule.ExpiresAt = &expiry
	}

	items = append(items, rule)
	if err := m.save(items); err != nil {
		return nil, err
	}
	if err := m.Apply(ctx); err != nil {
		copied := rule
		return &copied, err
	}
	copied := rule
	return &copied, nil
}

// Unblock removes a rule and re-applies the ruleset.
func (m *Manager) Unblock(ctx context.Context, id string) error {
	items, err := m.Rules()
	if err != nil {
		return err
	}
	remaining := make([]Rule, 0, len(items))
	found := false
	for _, item := range items {
		if item.ID == id {
			found = true
			continue
		}
		remaining = append(remaining, item)
	}
	if !found {
		return ErrRuleNotFound
	}
	if err := m.save(remaining); err != nil {
		return err
	}
	return m.Apply(ctx)
}

// RenderRuleset produces the deterministic nft script for the given rules.
func RenderRuleset(rules []Rule) string {
	var ipv4, ipv6 []string
	for _, rule := range rules {
		switch rule.Family {
		case "ipv6":
			ipv6 = append(ipv6, rule.Target)
		default:
			ipv4 = append(ipv4, rule.Target)
		}
	}
	sort.Strings(ipv4)
	sort.Strings(ipv6)

	var script strings.Builder
	script.WriteString("add table inet " + tableName + "\n")
	script.WriteString("delete table inet " + tableName + "\n")
	script.WriteString("table inet " + tableName + " {\n")
	script.WriteString("  set blocked_v4 {\n    type ipv4_addr\n    flags interval\n")
	if len(ipv4) > 0 {
		script.WriteString("    elements = { " + strings.Join(ipv4, ", ") + " }\n")
	}
	script.WriteString("  }\n")
	script.WriteString("  set blocked_v6 {\n    type ipv6_addr\n    flags interval\n")
	if len(ipv6) > 0 {
		script.WriteString("    elements = { " + strings.Join(ipv6, ", ") + " }\n")
	}
	script.WriteString("  }\n")
	script.WriteString("  chain input {\n")
	script.WriteString("    type filter hook input priority filter - 10; policy accept;\n")
	script.WriteString("    ct state established,related accept comment \"digipanel: established sessions\"\n")
	script.WriteString("    ip saddr @blocked_v4 drop comment \"digipanel: blocked ipv4\"\n")
	script.WriteString("    ip6 saddr @blocked_v6 drop comment \"digipanel: blocked ipv6\"\n")
	script.WriteString("  }\n")
	script.WriteString("}\n")
	return script.String()
}

// Apply renders the live ruleset and hands it to the runner.
func (m *Manager) Apply(ctx context.Context) error {
	if m.runner == nil {
		m.lastErr = ErrRunnerMissing.Error()
		return ErrRunnerMissing
	}
	rules, err := m.Rules()
	if err != nil {
		return err
	}
	if err := m.runner.Apply(ctx, rules); err != nil {
		m.lastErr = err.Error()
		return err
	}
	m.applied = true
	m.appliedAt = m.now().UTC()
	m.lastErr = ""
	return nil
}

// Status inspects the applied table through the runner.
func (m *Manager) Status(ctx context.Context) Status {
	status := Status{Table: "inet " + tableName, LastAppliedAt: m.appliedAt, LastError: m.lastErr}
	rules, err := m.Rules()
	if err == nil {
		status.ActiveRules = len(rules)
		status.Ruleset = RenderRuleset(rules)
	}
	if m.runner == nil {
		status.LastError = ErrRunnerMissing.Error()
		return status
	}
	status.AgentAvailable = true
	if output, err := m.runner.Inspect(ctx); err == nil {
		status.Applied = true
		if status.Ruleset == "" {
			status.Ruleset = output
		}
	} else if status.LastError == "" {
		status.LastError = err.Error()
	}
	return status
}

// AgentRunner sends the validated targets to the privileged agent, which owns
// the nft syntax. A caller therefore cannot inject raw nft statements.
type AgentRunner struct {
	Call func(ctx context.Context, action string, params map[string]string) (string, error)
}

func (r AgentRunner) Apply(ctx context.Context, rules []Rule) error {
	if r.Call == nil {
		return ErrRunnerMissing
	}
	targets := make([]string, 0, len(rules))
	for _, rule := range rules {
		targets = append(targets, rule.Target)
	}
	_, err := r.Call(ctx, "firewall.apply", map[string]string{"targets": strings.Join(targets, ",")})
	return err
}

func (r AgentRunner) Inspect(ctx context.Context) (string, error) {
	if r.Call == nil {
		return "", ErrRunnerMissing
	}
	return r.Call(ctx, "firewall.inspect", nil)
}
