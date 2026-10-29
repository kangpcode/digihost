// DigiPanel DNS, SSL, and Mail API Integration Tests (core/dns_ssl_mail_test.go)
// Verifies Quality Gate Tahap 4:
// 1. PowerDNS zone management, health checks, and DNSSEC
// 2. ACME SSL automated issuance and renewal
// 3. Postfix/Dovecot/Rspamd mail routing, DKIM, SPF, DMARC, and cleanliness score
// Zero emojis, strictly production grade.

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"digipanel/modules/dns"
	"digipanel/modules/mail"
	"digipanel/modules/ssl"
)

func getAdminToken(t *testing.T, handler http.Handler) string {
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": testAdminPassword,
	})
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Failed to login as admin: code %d", rr.Code)
	}

	var res map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	return res["token"].(string)
}

func TestDnsZonesRestAPI(t *testing.T) {
	_, handler := setupIntegratedTestServer()
	token := getAdminToken(t, handler)

	// 1. Create DNS Zone
	zonePayload, _ := json.Marshal(map[string]string{
		"name":       "enterprise-dns.org",
		"tenantId":   "tenant-corp",
		"primaryNs":  "ns1.digipanel.id",
		"adminEmail": "hostmaster@enterprise-dns.org",
		"serverIp":   "103.189.201.42",
	})
	reqCreate, _ := http.NewRequest("POST", "/api/v1/dns/zones", bytes.NewReader(zonePayload))
	reqCreate.Header.Set("Authorization", "Bearer "+token)
	reqCreate.Header.Set("Content-Type", "application/json")
	rrCreate := httptest.NewRecorder()
	handler.ServeHTTP(rrCreate, reqCreate)

	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("Failed to create DNS zone via REST API: code %d, body: %s", rrCreate.Code, rrCreate.Body.String())
	}

	// 2. Inspect Zone Health
	reqHealth, _ := http.NewRequest("GET", "/api/v1/dns/health?zone=enterprise-dns.org", nil)
	reqHealth.Header.Set("Authorization", "Bearer "+token)
	rrHealth := httptest.NewRecorder()
	handler.ServeHTTP(rrHealth, reqHealth)

	if rrHealth.Code != http.StatusOK {
		t.Fatalf("Failed to get DNS health: code %d", rrHealth.Code)
	}
	var health dns.ZoneHealth
	_ = json.Unmarshal(rrHealth.Body.Bytes(), &health)
	if !health.Healthy {
		t.Errorf("Expected newly created zone to be healthy: %+v", health)
	}

	// 3. Enable DNSSEC
	dnssecPayload, _ := json.Marshal(map[string]interface{}{
		"zone":   "enterprise-dns.org",
		"enable": true,
	})
	reqDnssec, _ := http.NewRequest("POST", "/api/v1/dns/dnssec/toggle", bytes.NewReader(dnssecPayload))
	reqDnssec.Header.Set("Authorization", "Bearer "+token)
	reqDnssec.Header.Set("Content-Type", "application/json")
	rrDnssec := httptest.NewRecorder()
	handler.ServeHTTP(rrDnssec, reqDnssec)

	if rrDnssec.Code != http.StatusOK {
		t.Fatalf("Failed to enable DNSSEC: code %d", rrDnssec.Code)
	}
	var keys dns.DnssecKeys
	_ = json.Unmarshal(rrDnssec.Body.Bytes(), &keys)
	if !keys.Enabled || keys.Algorithm != 13 || keys.DSRecord == "" {
		t.Errorf("Invalid DNSSEC keys returned: %+v", keys)
	}
}

func TestSslRestAPI(t *testing.T) {
	_, handler := setupIntegratedTestServer()
	token := getAdminToken(t, handler)

	// 1. Issue ACME Certificate
	issuePayload, _ := json.Marshal(ssl.IssueRequest{
		Domain:    "secure-portal.id",
		SANs:      []string{"www.secure-portal.id"},
		Provider:  ssl.ProviderLetsEncrypt,
		Challenge: ssl.ChallengeHTTP01,
		AutoRenew: true,
	})
	reqIssue, _ := http.NewRequest("POST", "/api/v1/ssl/issue", bytes.NewReader(issuePayload))
	reqIssue.Header.Set("Authorization", "Bearer "+token)
	reqIssue.Header.Set("Content-Type", "application/json")
	rrIssue := httptest.NewRecorder()
	handler.ServeHTTP(rrIssue, reqIssue)

	if rrIssue.Code != http.StatusCreated {
		t.Fatalf("Failed to issue certificate: code %d, body: %s", rrIssue.Code, rrIssue.Body.String())
	}

	var cert ssl.Certificate
	_ = json.Unmarshal(rrIssue.Body.Bytes(), &cert)
	if cert.Domain != "secure-portal.id" || cert.Status != ssl.StatusValid {
		t.Errorf("Invalid certificate returned: %+v", cert)
	}

	// A second certificate so the AutoSSL sweep below has more than one entry.
	secondPayload, _ := json.Marshal(ssl.IssueRequest{
		Domain:    "billing-portal.id",
		SANs:      []string{"www.billing-portal.id"},
		Provider:  ssl.ProviderLetsEncrypt,
		Challenge: ssl.ChallengeHTTP01,
		AutoRenew: true,
	})
	reqSecond, _ := http.NewRequest("POST", "/api/v1/ssl/issue", bytes.NewReader(secondPayload))
	reqSecond.Header.Set("Authorization", "Bearer "+token)
	reqSecond.Header.Set("Content-Type", "application/json")
	rrSecond := httptest.NewRecorder()
	handler.ServeHTTP(rrSecond, reqSecond)
	if rrSecond.Code != http.StatusCreated {
		t.Fatalf("Failed to issue second certificate: code %d, body: %s", rrSecond.Code, rrSecond.Body.String())
	}

	// 2. Force Certificate Renewal
	renewPayload, _ := json.Marshal(map[string]interface{}{
		"domain": "secure-portal.id",
		"force":  true,
	})
	reqRenew, _ := http.NewRequest("POST", "/api/v1/ssl/renew", bytes.NewReader(renewPayload))
	reqRenew.Header.Set("Authorization", "Bearer "+token)
	reqRenew.Header.Set("Content-Type", "application/json")
	rrRenew := httptest.NewRecorder()
	handler.ServeHTTP(rrRenew, reqRenew)

	if rrRenew.Code != http.StatusOK {
		t.Fatalf("Failed to force renew certificate: code %d", rrRenew.Code)
	}

	// 3. Run AutoSSL Batch Evaluation
	reqAuto, _ := http.NewRequest("POST", "/api/v1/ssl/autossl", bytes.NewReader([]byte("{}")))
	reqAuto.Header.Set("Authorization", "Bearer "+token)
	reqAuto.Header.Set("Content-Type", "application/json")
	rrAuto := httptest.NewRecorder()
	handler.ServeHTTP(rrAuto, reqAuto)

	if rrAuto.Code != http.StatusOK {
		t.Fatalf("Failed to run AutoSSL evaluation: code %d", rrAuto.Code)
	}
	var summary ssl.AutoSSLSummary
	_ = json.Unmarshal(rrAuto.Body.Bytes(), &summary)
	if summary.CheckedCount < 2 {
		t.Errorf("Expected >= 2 certificates checked in AutoSSL, got %d", summary.CheckedCount)
	}
}

func TestMailRestAPI(t *testing.T) {
	_, handler := setupIntegratedTestServer()
	token := getAdminToken(t, handler)

	// 1. Configure Mail Domain
	domPayload, _ := json.Marshal(map[string]string{
		"domain":   "cloudmail.id",
		"tenantId": "tenant-corp",
	})
	reqDom, _ := http.NewRequest("POST", "/api/v1/mail/domains", bytes.NewReader(domPayload))
	reqDom.Header.Set("Authorization", "Bearer "+token)
	reqDom.Header.Set("Content-Type", "application/json")
	rrDom := httptest.NewRecorder()
	handler.ServeHTTP(rrDom, reqDom)

	if rrDom.Code != http.StatusCreated {
		t.Fatalf("Failed to configure mail domain: code %d", rrDom.Code)
	}

	var md mail.MailDomain
	_ = json.Unmarshal(rrDom.Body.Bytes(), &md)
	if md.DKIMRecord == "" || md.SPFRecord == "" || md.DMARCRecord == "" {
		t.Errorf("Missing automated email authentication records: %+v", md)
	}

	// 2. Create Mailbox
	mbPayload, _ := json.Marshal(map[string]interface{}{
		"email":    "ops@cloudmail.id",
		"password": "SecurePassword123!",
		"quotaMb":  2048,
	})
	reqMb, _ := http.NewRequest("POST", "/api/v1/mail/mailboxes", bytes.NewReader(mbPayload))
	reqMb.Header.Set("Authorization", "Bearer "+token)
	reqMb.Header.Set("Content-Type", "application/json")
	rrMb := httptest.NewRecorder()
	handler.ServeHTTP(rrMb, reqMb)

	if rrMb.Code != http.StatusCreated {
		t.Fatalf("Failed to create mailbox: code %d", rrMb.Code)
	}

	// 3. Send Outbound Email (DKIM Signed)
	sendPayload, _ := json.Marshal(map[string]string{
		"from":    "ops@cloudmail.id",
		"to":      "alerts@datacenter.net",
		"subject": "Cluster Capacity Alert",
		"body":    "All nodes operating within normal thermal and memory parameters.",
	})
	reqSend, _ := http.NewRequest("POST", "/api/v1/mail/send", bytes.NewReader(sendPayload))
	reqSend.Header.Set("Authorization", "Bearer "+token)
	reqSend.Header.Set("Content-Type", "application/json")
	rrSend := httptest.NewRecorder()
	handler.ServeHTTP(rrSend, reqSend)

	if rrSend.Code != http.StatusOK {
		t.Fatalf("Failed to send outbound email: code %d, body: %s", rrSend.Code, rrSend.Body.String())
	}

	// 4. Inbound Delivery (IMAP Storage)
	delivPayload, _ := json.Marshal(map[string]string{
		"from":    "noc@datacenter.net",
		"to":      "ops@cloudmail.id",
		"subject": "Scheduled Maintenance Completed",
		"body":    "Core switch upgrade completed with zero packet drop.",
	})
	reqDeliv, _ := http.NewRequest("POST", "/api/v1/mail/deliver", bytes.NewReader(delivPayload))
	reqDeliv.Header.Set("Authorization", "Bearer "+token)
	reqDeliv.Header.Set("Content-Type", "application/json")
	rrDeliv := httptest.NewRecorder()
	handler.ServeHTTP(rrDeliv, reqDeliv)

	if rrDeliv.Code != http.StatusOK {
		t.Fatalf("Failed to deliver inbound message: code %d", rrDeliv.Code)
	}

	// 5. Calculate Deliverability Cleanliness Score
	pristineSample := "From: ops@cloudmail.id\r\n" +
		"To: client@customer.com\r\n" +
		"Subject: Service Deployment Notice\r\n" +
		"Date: Thu, 29 Oct 2026 12:00:00 +0000\r\n" +
		"DKIM-Signature: v=1; a=rsa-sha256; d=cloudmail.id; s=default; b=sig==;\r\n" +
		"Received-SPF: pass (designated sender)\r\n" +
		"Authentication-Results: mx.customer.com; dmarc=pass;\r\n\r\n" +
		"Your requested VPS instance has been deployed successfully."

	cleanPayload, _ := json.Marshal(map[string]string{
		"rawEmail": pristineSample,
	})
	reqClean, _ := http.NewRequest("POST", "/api/v1/mail/cleanliness", bytes.NewReader(cleanPayload))
	reqClean.Header.Set("Authorization", "Bearer "+token)
	reqClean.Header.Set("Content-Type", "application/json")
	rrClean := httptest.NewRecorder()
	handler.ServeHTTP(rrClean, reqClean)

	if rrClean.Code != http.StatusOK {
		t.Fatalf("Failed to calculate cleanliness score: code %d", rrClean.Code)
	}

	var score mail.CleanlinessScore
	_ = json.Unmarshal(rrClean.Body.Bytes(), &score)
	if score.TotalScore < 9.5 {
		t.Errorf("Expected cleanliness score >= 9.5, got %.2f", score.TotalScore)
	}
}
