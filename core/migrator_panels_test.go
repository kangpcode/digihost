// DigiPanel Competitor Panel Migration Importer Tests (core/migrator_panels_test.go)

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompetitorPanelMigrationDryRunAndLiveAllPanels(t *testing.T) {
	state, handler := setupIntegratedTestServer()
	token := adminToken(t, handler)

	// 1. Dry-run from cPanel must validate without creating accounts
	dryPayload, _ := json.Marshal(PanelMigrationBundle{
		SourcePanel: "cpanel",
		DryRun:      true,
		Accounts: []ImportedPanelAccount{
			{
				Username: "cpmig1",
				Domain:   "cpmig1.example.com",
				Package:  "starter",
				DNSRecords: []DnsRecord{
					{Name: "www", Type: "A", Content: "203.0.113.10", TTL: 300},
				},
			},
		},
	})
	req, _ := http.NewRequest("POST", "/api/v1/hosting/import", bytes.NewReader(dryPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("dry-run expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := state.HostingManager.GetAccount("cpmig1"); err == nil {
		t.Fatalf("dry-run mutated hosting state")
	}

	// 2. Live migration from all 5 supported competitor control panels
	panels := []struct {
		panel string
		user  string
		dom   string
	}{
		{"cpanel", "migcp1", "migcp1.example.com"},
		{"plesk", "migpl1", "migpl1.example.com"},
		{"directadmin", "migda1", "migda1.example.com"},
		{"hestiacp", "mighe1", "mighe1.example.com"},
		{"aapanel", "migaa1", "migaa1.example.com"},
	}

	for _, tc := range panels {
		body, _ := json.Marshal(PanelMigrationBundle{
			SourcePanel: tc.panel,
			DryRun:      false,
			Accounts: []ImportedPanelAccount{
				{
					Username: tc.user,
					Domain:   tc.dom,
					Package:  "starter",
					DNSRecords: []DnsRecord{
						{Name: "app", Type: "A", Content: "203.0.113.20", TTL: 300},
					},
				},
			},
		})
		req, _ := http.NewRequest("POST", "/api/v1/hosting/import", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("live import %s expected 200, got %d: %s", tc.panel, rr.Code, rr.Body.String())
		}
		var rep PanelMigrationReport
		_ = json.Unmarshal(rr.Body.Bytes(), &rep)
		if rep.AccountsImported != 1 || rep.ZonesImported != 1 || rep.RecordsImported != 1 || rep.MailImported != 1 {
			t.Fatalf("incomplete import report for %s: %+v", tc.panel, rep)
		}
		if _, err := state.HostingManager.GetAccount(tc.user); err != nil {
			t.Fatalf("imported account %s missing: %v", tc.user, err)
		}
	}
}
