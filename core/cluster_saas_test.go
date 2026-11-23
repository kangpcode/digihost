// DigiPanel Cluster mTLS & Multi-Tenant SaaS Integration Tests (core/cluster_saas_test.go)
// Verifies Tahap 9 Quality Gates:
// 1. Real TLS 1.3 mTLS hub-to-node connection with SHA-256 certificate fingerprint pinning
// 2. Rejection of unpinned / forged node certificates
// 3. DNS cluster synchronization over mTLS
// 4. Live account migration over mTLS with SHA-256 checksum verification & data preservation
// 5. Multi-tenant SaaS subscription lifecycle, white-label branding, and tenant isolation

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"digipanel/modules/hosting"
)

func TestClusterMTLSAndLiveMigration(t *testing.T) {
	// Hub server state
	hubDir := t.TempDir()
	hubState := &ServerState{StorageDir: hubDir, ModulesDir: hubDir}
	initStorage(hubState)
	hubHandler := NewServerHandler(hubState)
	hubToken := adminToken(t, hubHandler)
	cm := hubState.clusterManager()

	// Edge server state
	edgeDir := t.TempDir()
	edgeState := &ServerState{StorageDir: edgeDir, ModulesDir: edgeDir}
	initStorage(edgeState)
	edgeHandler := NewServerHandler(edgeState)
	edgeToken := adminToken(t, edgeHandler)

	// Start real TLS 1.3 mTLS edge server using a certificate issued by the cluster CA
	edgeCert, edgeFP, err := cm.IssueNodeCertificate("edge-sby-01", "127.0.0.1")
	if err != nil {
		t.Fatalf("IssueNodeCertificate: %v", err)
	}
	edgeServer := httptest.NewUnstartedServer(edgeHandler)
	edgeServer.TLS = cm.ServerTLSConfig(edgeCert)
	edgeServer.StartTLS()
	defer edgeServer.Close()

	// 1. Wrong pinned fingerprint must be rejected
	badBody, _ := json.Marshal(map[string]string{
		"name":            "edge-bad",
		"endpointUrl":     edgeServer.URL,
		"ipAddress":       "127.0.0.1",
		"role":            "edge",
		"certFingerprint": "0000000000000000000000000000000000000000000000000000000000000000",
		"nodeToken":       edgeToken,
	})
	req, _ := http.NewRequest("POST", "/api/v1/cluster/nodes/connect", bytes.NewReader(badBody))
	req.Header.Set("Authorization", "Bearer "+hubToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	hubHandler.ServeHTTP(rr, req)
	if rr.Code == http.StatusCreated {
		t.Fatalf("Security violation: unpinned certificate was accepted during node connect")
	}

	// 2. Valid pinned fingerprint succeeds over mTLS
	goodBody, _ := json.Marshal(map[string]string{
		"name":            "edge-sby-01",
		"endpointUrl":     edgeServer.URL,
		"ipAddress":       "127.0.0.1",
		"role":            "edge",
		"certFingerprint": edgeFP,
		"nodeToken":       edgeToken,
	})
	req, _ = http.NewRequest("POST", "/api/v1/cluster/nodes/connect", bytes.NewReader(goodBody))
	req.Header.Set("Authorization", "Bearer "+hubToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	hubHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("connect valid mTLS node expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var connected ManagedClusterNode
	_ = json.Unmarshal(rr.Body.Bytes(), &connected)

	// 3. Create DNS zone on hub and sync across mTLS cluster
	if _, err := hubState.DNSManager.CreateZone("clustersync.example.com", "tenant-default", "ns1.digipanel.id.", "hostmaster@clustersync.example.com", "127.0.0.1"); err != nil {
		t.Fatalf("CreateZone: %v", err)
	}
	syncBody, _ := json.Marshal(map[string]string{"nodeToken": edgeToken})
	req, _ = http.NewRequest("POST", "/api/v1/cluster/dns/sync", bytes.NewReader(syncBody))
	req.Header.Set("Authorization", "Bearer "+hubToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	hubHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("dns sync expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var syncRes struct {
		SyncedNodes int `json:"syncedNodes"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &syncRes)
	if syncRes.SyncedNodes != 1 {
		t.Fatalf("expected 1 synced node, got %d", syncRes.SyncedNodes)
	}
	if len(edgeState.DNSManager.ListZones()) == 0 {
		t.Fatalf("expected edge node to receive synced DNS zone")
	}

	// 4. Provision account on hub with custom payload file and migrate over mTLS to edge
	acc, err := hubState.HostingManager.CreateAccount(hosting.CreateAccountRequest{
		TenantID: "tenant-default",
		Username: "miguser1",
		Domain:   "miguser1.example.com",
		Package:  "starter",
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	markerFile := filepath.Join(acc.DocumentRoot, "data-integrity.txt")
	if err := os.WriteFile(markerFile, []byte("critical-customer-payload-2026"), 0644); err != nil {
		t.Fatalf("write marker file: %v", err)
	}

	migBody, _ := json.Marshal(map[string]string{
		"username":     acc.Username,
		"targetNodeId": connected.ID,
		"nodeToken":    edgeToken,
	})
	req, _ = http.NewRequest("POST", "/api/v1/cluster/migrate", bytes.NewReader(migBody))
	req.Header.Set("Authorization", "Bearer "+hubToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	hubHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("cluster migrate expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	migratedAcc, err := edgeState.HostingManager.GetAccount("miguser1")
	if err != nil {
		t.Fatalf("expected migrated account on edge node: %v", err)
	}
	restoredData, err := os.ReadFile(filepath.Join(migratedAcc.DocumentRoot, "data-integrity.txt"))
	if err != nil || string(restoredData) != "critical-customer-payload-2026" {
		t.Fatalf("migrated data integrity check failed: data=%q err=%v", string(restoredData), err)
	}
}

func TestMultiTenantSaaSWhiteLabelAndBillingAPI(t *testing.T) {
	tmpDir := t.TempDir()
	state := &ServerState{StorageDir: tmpDir, ModulesDir: tmpDir}
	initStorage(state)
	handler := NewServerHandler(state)
	token := adminToken(t, handler)

	// 1. Create SaaS tenant with custom white-label branding
	tBody, _ := json.Marshal(map[string]any{
		"slug":        "nusacloud",
		"companyName": "PT Nusa Cloud",
		"plan":        "growth",
		"trialDays":   14,
		"branding": map[string]string{
			"panelName":    "NusaPanel",
			"hostingName":  "NusaCloud",
			"primaryColor": "#0ea5e9",
			"customDomain": "panel.nusacloud.test",
		},
	})
	req, _ := http.NewRequest("POST", "/api/v1/saas/tenants", bytes.NewReader(tBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create tenant expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var createdTenant Tenant
	_ = json.Unmarshal(rr.Body.Bytes(), &createdTenant)

	// 2. Public branding resolves by Host header
	req, _ = http.NewRequest("GET", "/api/v1/branding", nil)
	req.Host = "panel.nusacloud.test:2083"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("public branding expected 200, got %d", rr.Code)
	}
	var brandResp struct {
		TenantID string             `json:"tenantId"`
		Branding WhiteLabelBranding `json:"branding"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &brandResp)
	if brandResp.TenantID != createdTenant.ID || brandResp.Branding.PanelName != "NusaPanel" {
		t.Fatalf("unexpected white-label branding resolution: %+v", brandResp)
	}

	// 3. Unsigned webhook must be rejected (401)
	req, _ = http.NewRequest("POST", "/api/v1/billing/webhooks/stripe", bytes.NewReader([]byte(`{"id":"evt_1"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("Security failure: unsigned billing webhook must be rejected with 401, got %d", rr.Code)
	}
}

func TestClusterNodeFailover(t *testing.T) {
	hubDir := t.TempDir()
	hubState := &ServerState{StorageDir: hubDir, ModulesDir: hubDir}
	initStorage(hubState)
	hubHandler := NewServerHandler(hubState)
	hubToken := adminToken(t, hubHandler)
	cm := hubState.clusterManager()

	// Primary edge node (will go offline)
	edge1Dir := t.TempDir()
	edge1State := &ServerState{StorageDir: edge1Dir, ModulesDir: edge1Dir}
	initStorage(edge1State)
	edge1Handler := NewServerHandler(edge1State)
	edge1Token := adminToken(t, edge1Handler)
	cert1, fp1, err := cm.IssueNodeCertificate("edge-primary", "127.0.0.1")
	if err != nil {
		t.Fatalf("IssueNodeCertificate edge1: %v", err)
	}
	srv1 := httptest.NewUnstartedServer(edge1Handler)
	srv1.TLS = cm.ServerTLSConfig(cert1)
	srv1.StartTLS()

	// Secondary failover edge node
	edge2Dir := t.TempDir()
	edge2State := &ServerState{StorageDir: edge2Dir, ModulesDir: edge2Dir}
	initStorage(edge2State)
	edge2Handler := NewServerHandler(edge2State)
	edge2Token := adminToken(t, edge2Handler)
	cert2, fp2, err := cm.IssueNodeCertificate("edge-failover", "127.0.0.1")
	if err != nil {
		t.Fatalf("IssueNodeCertificate edge2: %v", err)
	}
	srv2 := httptest.NewUnstartedServer(edge2Handler)
	srv2.TLS = cm.ServerTLSConfig(cert2)
	srv2.StartTLS()
	defer srv2.Close()

	connect := func(name, url, fp, tok string) ManagedClusterNode {
		b, _ := json.Marshal(map[string]string{
			"name": name, "endpointUrl": url, "ipAddress": "127.0.0.1",
			"role": "edge", "certFingerprint": fp, "nodeToken": tok,
		})
		r, _ := http.NewRequest("POST", "/api/v1/cluster/nodes/connect", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+hubToken)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		hubHandler.ServeHTTP(w, r)
		if w.Code != http.StatusCreated {
			t.Fatalf("connect %s expected 201, got %d: %s", name, w.Code, w.Body.String())
		}
		var n ManagedClusterNode
		_ = json.Unmarshal(w.Body.Bytes(), &n)
		return n
	}

	n1 := connect("edge-primary", srv1.URL, fp1, edge1Token)
	n2 := connect("edge-failover", srv2.URL, fp2, edge2Token)

	// Simulate primary edge outage
	srv1.Close()

	pollBody, _ := json.Marshal(map[string]string{"nodeToken": edge2Token})
	req, _ := http.NewRequest("POST", "/api/v1/cluster/nodes/poll", bytes.NewReader(pollBody))
	req.Header.Set("Authorization", "Bearer "+hubToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	hubHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("poll expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var pollResp struct {
		Online  int                  `json:"online"`
		Offline int                  `json:"offline"`
		Nodes   []ManagedClusterNode `json:"nodes"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &pollResp)
	if pollResp.Online != 1 || pollResp.Offline != 1 {
		t.Fatalf("expected 1 online and 1 offline node after outage, got %+v", pollResp)
	}

	// Provision account on hub and migrate targeting the offline node with allowFailover=true
	acc, err := hubState.HostingManager.CreateAccount(hosting.CreateAccountRequest{
		TenantID: "tenant-default",
		Username: "failover1",
		Domain:   "failover1.example.com",
		Package:  "starter",
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if err := os.WriteFile(filepath.Join(acc.DocumentRoot, "failover.txt"), []byte("survives-outage"), 0644); err != nil {
		t.Fatalf("write failover marker: %v", err)
	}

	migBody, _ := json.Marshal(map[string]any{
		"username":      "failover1",
		"targetNodeId":  n1.ID,
		"nodeToken":     edge2Token,
		"allowFailover": true,
	})
	req, _ = http.NewRequest("POST", "/api/v1/cluster/migrate", bytes.NewReader(migBody))
	req.Header.Set("Authorization", "Bearer "+hubToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	hubHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("failover migrate expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var migResp struct {
		TargetNodeID string `json:"targetNodeId"`
		FailoverFrom string `json:"failoverFrom"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &migResp)
	if migResp.TargetNodeID != n2.ID || migResp.FailoverFrom != n1.ID {
		t.Fatalf("expected failover from %s to %s, got %+v", n1.ID, n2.ID, migResp)
	}
	if _, err := edge2State.HostingManager.GetAccount("failover1"); err != nil {
		t.Fatalf("expected migrated account on failover node: %v", err)
	}
}

func TestCrossTenantIsolationAndQuotaEnforcement(t *testing.T) {
	tmpDir := t.TempDir()
	state := &ServerState{StorageDir: tmpDir, ModulesDir: tmpDir}
	initStorage(state)
	handler := NewServerHandler(state)
	token := adminToken(t, handler)
	tm := state.tenantManager()

	tAlpha, err := tm.CreateTenant("alpha-ops", "Alpha Ops", WhiteLabelBranding{PanelName: "AlphaPanel"}, "starter", 0)
	if err != nil {
		t.Fatalf("CreateTenant alpha: %v", err)
	}
	tBeta, err := tm.CreateTenant("beta-ops", "Beta Ops", WhiteLabelBranding{PanelName: "BetaPanel"}, "starter", 0)
	if err != nil {
		t.Fatalf("CreateTenant beta: %v", err)
	}

	// Provision accounts under distinct tenants
	if _, err := state.HostingManager.CreateAccount(hosting.CreateAccountRequest{
		TenantID: tAlpha.ID, Username: "alphaacct1", Domain: "alpha1.example.com", Package: "starter",
	}); err != nil {
		t.Fatalf("create alpha account: %v", err)
	}
	if _, err := state.HostingManager.CreateAccount(hosting.CreateAccountRequest{
		TenantID: tBeta.ID, Username: "betaacct1", Domain: "beta1.example.com", Package: "starter",
	}); err != nil {
		t.Fatalf("create beta account: %v", err)
	}

	// Usage query for alpha must never leak beta's accounts
	req, _ := http.NewRequest("GET", "/api/v1/saas/tenants/usage?tenantId="+tAlpha.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("usage alpha expected 200, got %d", rr.Code)
	}
	var usageResp struct {
		AccountsUsed int                      `json:"accountsUsed"`
		Accounts     []hosting.HostingAccount `json:"accounts"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &usageResp)
	if usageResp.AccountsUsed != 1 || len(usageResp.Accounts) != 1 || usageResp.Accounts[0].Username != "alphaacct1" {
		t.Fatalf("cross-tenant leak or wrong usage count: %+v", usageResp)
	}

	// Suspending tenant beta blocks new hosting account provisioning via REST API
	subBody, _ := json.Marshal(map[string]string{"tenantId": tBeta.ID, "status": "suspended"})
	req, _ = http.NewRequest("POST", "/api/v1/saas/tenants/subscription", bytes.NewReader(subBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("suspend tenant expected 200, got %d", rr.Code)
	}

	acctBody, _ := json.Marshal(map[string]string{
		"tenantId": tBeta.ID, "username": "betaacct2", "domain": "beta2.example.com", "package": "starter",
	})
	req, _ = http.NewRequest("POST", "/api/v1/hosting/accounts", bytes.NewReader(acctBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected suspended tenant provisioning to be blocked with 403, got %d: %s", rr.Code, rr.Body.String())
	}

	// Quota cap check on tenant alpha
	if err := tm.CanProvisionAccount(tAlpha.ID, tAlpha.Subscription.MaxAccounts); err == nil {
		t.Fatalf("expected quota enforcement error at MaxAccounts")
	}
}
