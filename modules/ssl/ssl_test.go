// DigiPanel SSL/TLS Engine Tests (modules/ssl/ssl_test.go)
// Standards: Zero emojis, strictly production grade.

package ssl

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"
	"time"
)

func TestStandardACMEIssuanceHTTP01(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "digipanel-ssl-test-*")
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)

	req := IssueRequest{
		Domain:    "digihost.id",
		SANs:      []string{"www.digihost.id"},
		Provider:  ProviderLetsEncrypt,
		Challenge: ChallengeHTTP01,
		AutoRenew: true,
	}

	cert, err := mgr.IssueCertificate(req)
	if err != nil {
		t.Fatalf("Failed to issue ACME certificate: %v", err)
	}

	if cert.Domain != "digihost.id" {
		t.Errorf("Expected domain digihost.id, got %s", cert.Domain)
	}
	if cert.Status != StatusValid {
		t.Errorf("Expected status valid, got %s", cert.Status)
	}
	if cert.DaysRemaining < 80 {
		t.Errorf("Expected >= 80 days remaining for fresh 90-day cert, got %d", cert.DaysRemaining)
	}

	// Validate PEM Decode
	block, _ := pem.Decode([]byte(cert.CertPEM))
	if block == nil {
		t.Fatalf("Failed to decode certificate PEM block")
	}

	parsed, errParse := x509.ParseCertificate(block.Bytes)
	if errParse != nil {
		t.Fatalf("Failed to parse X.509 certificate: %v", errParse)
	}
	if parsed.Subject.CommonName != "digihost.id" {
		t.Errorf("Subject CommonName mismatch: %s", parsed.Subject.CommonName)
	}
}

func TestWildcardCertificateDNS01Enforcement(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "digipanel-ssl-wildcard-*")
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)

	// 1. HTTP-01 on wildcard MUST FAIL
	_, errFail := mgr.IssueCertificate(IssueRequest{
		Domain:    "*.enterprise.id",
		Challenge: ChallengeHTTP01,
	})
	if errFail == nil {
		t.Errorf("Security failure: wildcard certificate was permitted with HTTP-01 challenge")
	}

	// 2. DNS-01 on wildcard MUST SUCCEED
	cert, errPass := mgr.IssueCertificate(IssueRequest{
		Domain:    "*.enterprise.id",
		Challenge: ChallengeDNS01,
		AutoRenew: true,
	})
	if errPass != nil {
		t.Fatalf("Failed to issue wildcard certificate via DNS-01: %v", errPass)
	}

	if !cert.IsWildcard {
		t.Errorf("Certificate IsWildcard flag should be true")
	}
	if cert.Challenge != ChallengeDNS01 {
		t.Errorf("Expected challenge dns-01, got %s", cert.Challenge)
	}
}

func TestAutoSSLAndAutomatedRenewal(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "digipanel-ssl-autossl-*")
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)

	// Issue 1: Valid fresh certificate (90 days)
	_, _ = mgr.IssueCertificate(IssueRequest{
		Domain:    "fresh.com",
		AutoRenew: true,
	})

	// Issue 2: Expiring certificate (simulate certificate created with 10 days remaining)
	certPEM, keyPEM, _, _ := GenerateX509Certificate("expiring.com", nil, "Let's Encrypt", 10*24*time.Hour, true)
	_, _ = mgr.ImportCustomCertificate("expiring.com", certPEM, keyPEM)
	// Enable AutoRenew on expiring cert
	expiringCert, _ := mgr.GetCertificate("expiring.com")
	expiringCert.AutoRenew = true

	// Run AutoSSL Evaluation
	summary := mgr.RunAutoSSLEvaluation()

	if summary.CheckedCount != 2 {
		t.Errorf("Expected 2 checked certs, got %d", summary.CheckedCount)
	}
	if summary.RenewedCount != 1 {
		t.Errorf("Expected 1 renewed cert, got %d", summary.RenewedCount)
	}
	if len(summary.RenewedDomains) == 0 || summary.RenewedDomains[0] != "expiring.com" {
		t.Errorf("Expected 'expiring.com' to be auto-renewed, got: %v", summary.RenewedDomains)
	}

	// Verify renewed certificate now has fresh ~90 days validity
	refreshed, _ := mgr.GetCertificate("expiring.com")
	if refreshed.DaysRemaining < 80 {
		t.Errorf("Expected renewed certificate to have >= 80 days remaining, got %d", refreshed.DaysRemaining)
	}
}

func TestEarlyRenewalRejectionWithoutForce(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "digipanel-ssl-early-*")
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	_, _ = mgr.IssueCertificate(IssueRequest{
		Domain:    "nonexpiring.com",
		AutoRenew: true,
	})

	// Non-force renewal of fresh cert must be rejected to prevent ACME rate-limiting
	_, err := mgr.RenewCertificate("nonexpiring.com", false)
	if err == nil {
		t.Errorf("Expected early non-forced renewal to be rejected")
	}

	// Forced renewal should succeed
	renewed, errForce := mgr.RenewCertificate("nonexpiring.com", true)
	if errForce != nil {
		t.Fatalf("Forced renewal failed: %v", errForce)
	}
	if renewed.Domain != "nonexpiring.com" {
		t.Errorf("Domain mismatch: %s", renewed.Domain)
	}
}
