// DigiPanel firewall manager tests.
// The runner is recorded instead of executed, so the generated nftables ruleset
// can be asserted exactly without root privileges.

package firewall

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type recordingRunner struct {
	applied   [][]Rule
	inspected string
	applyErr  error
}

func (r *recordingRunner) Apply(ctx context.Context, rules []Rule) error {
	copied := make([]Rule, len(rules))
	copy(copied, rules)
	r.applied = append(r.applied, copied)
	return r.applyErr
}

func (r *recordingRunner) Inspect(ctx context.Context) (string, error) {
	return r.inspected, nil
}

func newTestManager(t *testing.T) (*Manager, *recordingRunner) {
	t.Helper()
	runner := &recordingRunner{inspected: "table inet digipanel { }"}
	manager := NewManager(t.TempDir(), runner)
	manager.now = func() time.Time { return time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC) }
	return manager, runner
}

func TestValidateTarget(t *testing.T) {
	cases := []struct {
		input  string
		want   string
		family string
	}{
		{"203.0.113.7", "203.0.113.7", "ipv4"},
		{"203.0.113.0/24", "203.0.113.0/24", "ipv4"},
		{"2001:db8::1", "2001:db8::1", "ipv6"},
		{"2001:db8::/32", "2001:db8::/32", "ipv6"},
		{"  198.51.100.9 ", "198.51.100.9", "ipv4"},
	}
	for _, testCase := range cases {
		got, family, err := ValidateTarget(testCase.input)
		if err != nil {
			t.Fatalf("%q rejected: %v", testCase.input, err)
		}
		if got != testCase.want || family != testCase.family {
			t.Fatalf("%q -> %q %q, want %q %q", testCase.input, got, family, testCase.want, testCase.family)
		}
	}

	for _, invalid := range []string{"", "not-an-ip", "1.2.3.4; rm -rf /", "1.2.3.4/99", "300.1.1.1"} {
		if _, _, err := ValidateTarget(invalid); err == nil {
			t.Fatalf("%q was accepted", invalid)
		}
	}
}

func TestRenderRulesetIsDeterministicAndScoped(t *testing.T) {
	rules := []Rule{
		{ID: "fw-1", Target: "203.0.113.9", Family: "ipv4"},
		{ID: "fw-2", Target: "203.0.113.0/24", Family: "ipv4"},
		{ID: "fw-3", Target: "2001:db8::5", Family: "ipv6"},
	}
	script := RenderRuleset(rules)
	if !strings.Contains(script, "table inet digipanel {") {
		t.Fatalf("ruleset does not create the managed table: %s", script)
	}
	if !strings.Contains(script, "elements = { 203.0.113.0/24, 203.0.113.9 }") {
		t.Fatalf("ipv4 elements are not sorted in the managed set: %s", script)
	}
	if !strings.Contains(script, "elements = { 2001:db8::5 }") {
		t.Fatalf("ipv6 element missing: %s", script)
	}
	if !strings.Contains(script, "ip saddr @blocked_v4 drop") || !strings.Contains(script, "ip6 saddr @blocked_v6 drop") {
		t.Fatalf("drop rules missing: %s", script)
	}
	if strings.Contains(script, "policy drop") {
		t.Fatal("the panel must never flip the default input policy")
	}
	if strings.Contains(script, "flush ruleset") {
		t.Fatal("the panel must never flush the host ruleset")
	}
}

func TestBlockPersistsAndApplies(t *testing.T) {
	manager, runner := newTestManager(t)
	ctx := context.Background()

	rule, err := manager.Block(ctx, "203.0.113.7", "brute force", 24*time.Hour)
	if err != nil {
		t.Fatalf("block: %v", err)
	}
	if rule.ID == "" || rule.ExpiresAt == nil || rule.Family != "ipv4" {
		t.Fatalf("unexpected rule %+v", rule)
	}
	if len(runner.applied) != 1 || len(runner.applied[0]) != 1 {
		t.Fatalf("ruleset was not applied: %+v", runner.applied)
	}

	rules, err := manager.Rules()
	if err != nil || len(rules) != 1 || rules[0].Target != "203.0.113.7" {
		t.Fatalf("rule was not persisted: %+v %v", rules, err)
	}

	if _, err := manager.Block(ctx, "203.0.113.7", "duplicate", time.Hour); !errors.Is(err, ErrRuleExists) {
		t.Fatalf("expected ErrRuleExists, got %v", err)
	}
	// A wider block that already covers the target is also a duplicate.
	if _, err := manager.Block(ctx, "203.0.113.0/24", "wide block", time.Hour); !errors.Is(err, ErrRuleExists) {
		t.Fatalf("expected ErrRuleExists for overlapping block, got %v", err)
	}

	if err := manager.Unblock(ctx, rule.ID); err != nil {
		t.Fatalf("unblock: %v", err)
	}
	rules, _ = manager.Rules()
	if len(rules) != 0 {
		t.Fatalf("rule survived unblock: %+v", rules)
	}
	// Two applies: the successful block and the unblock. The rejected
	// duplicates never reach the runner.
	if len(runner.applied) != 2 {
		t.Fatalf("expected two applies, got %d", len(runner.applied))
	}
}

func TestExpiredRulesAreDropped(t *testing.T) {
	manager, _ := newTestManager(t)
	base := manager.now()
	ctx := context.Background()

	if _, err := manager.Block(ctx, "198.51.100.4", "temporary ban", time.Hour); err != nil {
		t.Fatalf("block: %v", err)
	}
	manager.now = func() time.Time { return base.Add(2 * time.Hour) }
	rules, err := manager.Rules()
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expired rule is still active: %+v", rules)
	}
	script := RenderRuleset(rules)
	if strings.Contains(script, "198.51.100.4") {
		t.Fatal("expired target still rendered into the ruleset")
	}
}

func TestBlockValidatesInputWithoutApplying(t *testing.T) {
	manager, runner := newTestManager(t)
	ctx := context.Background()

	if _, err := manager.Block(ctx, "1.2.3.4", "no", 0); !errors.Is(err, ErrInvalidReason) {
		t.Fatalf("expected ErrInvalidReason, got %v", err)
	}
	if _, err := manager.Block(ctx, "1.2.3.4", "valid reason", -time.Hour); !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("expected ErrInvalidTTL, got %v", err)
	}
	if len(runner.applied) != 0 {
		t.Fatal("invalid input reached the runner")
	}
}

func TestMissingRunnerFailsClosed(t *testing.T) {
	manager := NewManager(t.TempDir(), nil)
	if err := manager.Apply(context.Background()); !errors.Is(err, ErrRunnerMissing) {
		t.Fatalf("expected ErrRunnerMissing, got %v", err)
	}
	status := manager.Status(context.Background())
	if status.AgentAvailable || status.Applied {
		t.Fatalf("status claims the agent is available: %+v", status)
	}
	if status.Ruleset == "" {
		t.Fatal("status must still describe the pending ruleset")
	}
}

func TestStatusReportsAppliedTable(t *testing.T) {
	manager, _ := newTestManager(t)
	ctx := context.Background()
	if _, err := manager.Block(ctx, "203.0.113.11", "scanner", 0); err != nil {
		t.Fatalf("block: %v", err)
	}
	status := manager.Status(ctx)
	if !status.AgentAvailable || !status.Applied || status.ActiveRules != 1 {
		t.Fatalf("unexpected status %+v", status)
	}
	if status.Table != "inet digipanel" {
		t.Fatalf("unexpected table name %q", status.Table)
	}
}

func TestApplyFailureIsReported(t *testing.T) {
	manager, runner := newTestManager(t)
	runner.applyErr = errors.New("nft: permission denied")
	if _, err := manager.Block(context.Background(), "203.0.113.12", "scanner", 0); err == nil {
		t.Fatal("apply failure was swallowed")
	}
	status := manager.Status(context.Background())
	if !strings.Contains(status.LastError, "permission denied") {
		t.Fatalf("status does not surface the failure: %+v", status)
	}
}
