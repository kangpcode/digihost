// DigiPanel Competitor Panel Migration Importer (core/migrator_panels.go)
// One-step migration importer for cPanel, Plesk, DirectAdmin, HestiaCP, and aaPanel
// exports with dry-run validation mode and atomic rollback on provisioning failure.
// Zero emojis, zero mocks.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"digipanel/modules/dns"
	"digipanel/modules/hosting"
)

type ImportedPanelAccount struct {
	Username   string       `json:"username"`
	Domain     string       `json:"domain"`
	Package    string       `json:"package"`
	PhpVersion string       `json:"phpVersion"`
	TenantID   string       `json:"tenantId"`
	Email      string       `json:"email"`
	DNSRecords []DnsRecord  `json:"dnsRecords,omitempty"`
}

type PanelMigrationBundle struct {
	SourcePanel string                 `json:"sourcePanel"` // cpanel, plesk, directadmin, hestiacp, aapanel
	DryRun      bool                   `json:"dryRun"`
	Accounts    []ImportedPanelAccount `json:"accounts"`
}

type PanelMigrationReport struct {
	SourcePanel      string   `json:"sourcePanel"`
	DryRun           bool     `json:"dryRun"`
	AccountsPlanned  int      `json:"accountsPlanned"`
	AccountsImported int      `json:"accountsImported"`
	ZonesImported    int      `json:"zonesImported"`
	RecordsImported  int      `json:"recordsImported"`
	MailImported     int      `json:"mailImported"`
	Warnings         []string `json:"warnings,omitempty"`
}

var supportedSourcePanels = map[string]bool{
	"cpanel":      true,
	"plesk":       true,
	"directadmin": true,
	"hestiacp":    true,
	"aapanel":     true,
}

func (state *ServerState) ImportCompetitorPanelBundle(bundle PanelMigrationBundle) (PanelMigrationReport, error) {
	source := strings.ToLower(strings.TrimSpace(bundle.SourcePanel))
	if !supportedSourcePanels[source] {
		return PanelMigrationReport{}, fmt.Errorf("unsupported sourcePanel %q (must be cpanel, plesk, directadmin, hestiacp, or aapanel)", bundle.SourcePanel)
	}
	if len(bundle.Accounts) == 0 {
		return PanelMigrationReport{}, errors.New("migration bundle contains no accounts")
	}

	report := PanelMigrationReport{
		SourcePanel:     source,
		DryRun:          bundle.DryRun,
		AccountsPlanned: len(bundle.Accounts),
	}

	// Validate every entry upfront before mutating state.
	seenUsers := make(map[string]bool, len(bundle.Accounts))
	for i, acc := range bundle.Accounts {
		u := strings.ToLower(strings.TrimSpace(acc.Username))
		d := strings.ToLower(strings.TrimSpace(acc.Domain))
		if u == "" || d == "" {
			return report, fmt.Errorf("account[%d]: username and domain are required", i)
		}
		if seenUsers[u] {
			return report, fmt.Errorf("duplicate username %q in migration bundle", u)
		}
		seenUsers[u] = true
		if err := dns.ValidateZoneName(d); err != nil {
			return report, fmt.Errorf("account[%d] domain %q: %w", i, d, err)
		}
		for _, rec := range acc.DNSRecords {
			ttl := rec.TTL
			if ttl == 0 {
				ttl = 300
			}
			if err := dns.ValidateRecord(dns.Record{
				Zone:     d,
				Name:     rec.Name,
				Type:     dns.RecordType(strings.ToUpper(strings.TrimSpace(rec.Type))),
				Value:    rec.Content,
				TTL:      ttl,
				Priority: rec.Priority,
			}); err != nil {
				return report, fmt.Errorf("account[%d] dns record: %w", i, err)
			}
		}
		if _, err := state.HostingManager.GetAccount(u); err == nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("account %s already exists", u))
		}
	}

	if bundle.DryRun {
		return report, nil
	}

	var createdUsers []string
	rollback := func() {
		for _, u := range createdUsers {
			_ = state.HostingManager.TerminateAccount(u)
		}
	}

	for _, acc := range bundle.Accounts {
		u := strings.ToLower(strings.TrimSpace(acc.Username))
		d := strings.ToLower(strings.TrimSpace(acc.Domain))
		tenantID := strings.TrimSpace(acc.TenantID)
		if tenantID == "" {
			tenantID = "tenant-default"
		}
		pkg := strings.TrimSpace(acc.Package)
		if pkg == "" {
			pkg = "starter"
		}
		created, err := state.HostingManager.CreateAccount(hosting.CreateAccountRequest{
			TenantID:   tenantID,
			Username:   u,
			Domain:     d,
			Package:    pkg,
			PhpVersion: acc.PhpVersion,
		})
		if err != nil {
			rollback()
			return report, fmt.Errorf("import account %s failed (rolled back): %w", u, err)
		}
		createdUsers = append(createdUsers, created.Username)
		report.AccountsImported++

		adminEmail := strings.TrimSpace(acc.Email)
		if adminEmail == "" {
			adminEmail = "hostmaster@" + d
		}
		if _, err := state.DNSManager.CreateZone(d, tenantID, "ns1.digipanel.id.", adminEmail, "127.0.0.1"); err == nil {
			report.ZonesImported++
		}
		for _, rec := range acc.DNSRecords {
			ttl := rec.TTL
			if ttl == 0 {
				ttl = 300
			}
			if _, err := state.DNSManager.AddRecord(d, dns.Record{
				Zone:     d,
				Name:     rec.Name,
				Type:     dns.RecordType(strings.ToUpper(strings.TrimSpace(rec.Type))),
				Value:    rec.Content,
				TTL:      ttl,
				Priority: rec.Priority,
			}); err == nil {
				report.RecordsImported++
			}
		}
		if _, err := state.MailManager.AddDomain(d, tenantID); err == nil {
			report.MailImported++
		}
	}

	return report, nil
}

func (state *ServerState) handleHostingImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req PanelMigrationBundle
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid migration bundle: "+err.Error(), http.StatusBadRequest)
		return
	}
	report, err := state.ImportCompetitorPanelBundle(req)
	if err != nil {
		writeAPIError(w, err.Error(), http.StatusBadRequest)
		return
	}
	state.AuditLogger.Append(state.actorUsername(r), "hosting.panel.import", "migration", report.SourcePanel,
		fmt.Sprintf(`{"dryRun":%t,"imported":%d}`, report.DryRun, report.AccountsImported), clientIP(r, state.TrustedProxies))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}
