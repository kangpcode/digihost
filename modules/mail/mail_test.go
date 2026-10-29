// DigiPanel Email Subsystem Tests (modules/mail/mail_test.go)
// Standards: Zero emojis, strictly production grade.

package mail

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestDomainListingNeverLeaksDKIMPrivateKey guards against serializing the DKIM
// signing key through GET /api/v1/mail/domains or any other JSON response.
func TestDomainListingNeverLeaksDKIMPrivateKey(t *testing.T) {
	mgr := NewManager("103.189.201.42")
	if _, err := mgr.AddDomain("beta-mail.id", "tenant-beta"); err != nil {
		t.Fatalf("AddDomain beta: %v", err)
	}
	if _, err := mgr.AddDomain("alpha-mail.id", "tenant-alpha"); err != nil {
		t.Fatalf("AddDomain alpha: %v", err)
	}

	all := mgr.ListDomains("")
	if len(all) != 2 || all[0].Domain != "alpha-mail.id" {
		t.Fatalf("expected 2 domains sorted by name, got %+v", all)
	}
	if all[0].DKIMPrivate == "" {
		t.Fatal("private key must still exist server-side for signing")
	}

	raw, err := json.Marshal(all)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(raw)
	if strings.Contains(out, "PRIVATE KEY") || strings.Contains(out, "dkimPrivate") {
		t.Fatalf("DKIM private key leaked in JSON: %s", out)
	}

	scoped := mgr.ListDomains("tenant-beta")
	if len(scoped) != 1 || scoped[0].Domain != "beta-mail.id" {
		t.Fatalf("tenant filter failed, got %+v", scoped)
	}
}

func TestDomainProvisioningAndDKIMSPF(t *testing.T) {
	mgr := NewManager("103.189.201.42")

	domain, err := mgr.AddDomain("enterprise-mail.id", "tenant-alpha")
	if err != nil {
		t.Fatalf("Failed to provision mail domain: %v", err)
	}

	// 1. Verify RSA 2048-bit DKIM record
	if domain.DKIMSelector != "default" {
		t.Errorf("Expected selector 'default', got '%s'", domain.DKIMSelector)
	}
	expectedDKIMPrefix := "default._domainkey.enterprise-mail.id. IN TXT \"v=DKIM1; k=rsa; p="
	if !strings.HasPrefix(domain.DKIMRecord, expectedDKIMPrefix) {
		t.Errorf("DKIM record format invalid: %s", domain.DKIMRecord)
	}
	if len(domain.DKIMPublic) < 200 { // RSA 2048 DER base64 is ~392 chars
		t.Errorf("DKIM public key appears too short for RSA 2048: %d chars", len(domain.DKIMPublic))
	}

	// 2. Verify SPF record
	expectedSPF := "enterprise-mail.id. IN TXT \"v=spf1 mx a ip4:103.189.201.42 ~all\""
	if domain.SPFRecord != expectedSPF {
		t.Errorf("Expected SPF record '%s', got '%s'", expectedSPF, domain.SPFRecord)
	}

	// 3. Verify DMARC record
	expectedDMARCPrefix := "_dmarc.enterprise-mail.id. IN TXT \"v=DMARC1; p=quarantine;"
	if !strings.HasPrefix(domain.DMARCRecord, expectedDMARCPrefix) {
		t.Errorf("Expected DMARC record to start with '%s', got '%s'", expectedDMARCPrefix, domain.DMARCRecord)
	}
}

func TestMailboxCreationAndQuotaEnforcement(t *testing.T) {
	mgr := NewManager("103.189.201.42")
	_, _ = mgr.AddDomain("hosting.org", "tenant-1")

	// 1. Create Mailbox with 1MB quota for testing limit
	mb, err := mgr.CreateMailbox("support@hosting.org", "SecureP@ss123", 1)
	if err != nil {
		t.Fatalf("Failed to create mailbox: %v", err)
	}

	if mb.Email != "support@hosting.org" {
		t.Errorf("Expected email support@hosting.org, got %s", mb.Email)
	}
	if mb.PasswordHash == "" || !strings.HasPrefix(mb.PasswordHash, "$argon2id$") {
		t.Errorf("Expected Argon2id password hash, got %s", mb.PasswordHash)
	}

	// 2. Delivery within quota
	_, errDeliver := mgr.DeliverMessage("customer@external.com", "support@hosting.org", "Ticket #101", "Hello Support team.")
	if errDeliver != nil {
		t.Fatalf("Delivery within quota failed: %v", errDeliver)
	}

	// 3. Overflow quota
	hugeBody := strings.Repeat("A", 1024*1024+5000) // Exceeds 1MB
	_, errOverflow := mgr.DeliverMessage("spammer@external.com", "support@hosting.org", "Large Attachment", hugeBody)
	if errOverflow == nil {
		t.Errorf("Expected quota overflow error, but message was accepted")
	}
}

func TestRealSendReceiveAndDKIMSignature(t *testing.T) {
	mgr := NewManager("103.189.201.42")
	_, _ = mgr.AddDomain("company.test", "tenant-beta")
	_, _ = mgr.CreateMailbox("ceo@company.test", "ExecPass!23", 1024)
	_, _ = mgr.CreateMailbox("cto@company.test", "TechPass!23", 1024)

	// 1. Send outbound message (computes real DKIM signature)
	dkimHeader, err := mgr.SendMessage("ceo@company.test", "cto@company.test", "Quarterly Roadmap", "Let's review the cluster status.")
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	if !strings.HasPrefix(dkimHeader, "DKIM-Signature:") {
		t.Errorf("Expected DKIM-Signature header, got: %s", dkimHeader)
	}
	if !strings.Contains(dkimHeader, "a=rsa-sha256") || !strings.Contains(dkimHeader, "d=company.test") {
		t.Errorf("DKIM header missing required attributes: %s", dkimHeader)
	}

	// 2. Receive message in destination mailbox (IMAP delivery)
	msg, errDeliver := mgr.DeliverMessage("ceo@company.test", "cto@company.test", "Quarterly Roadmap", "Let's review the cluster status.")
	if errDeliver != nil {
		t.Fatalf("Failed to deliver message to destination: %v", errDeliver)
	}

	ctoMailbox, _ := mgr.GetMailbox("cto@company.test")
	if len(ctoMailbox.Messages) != 1 {
		t.Fatalf("Expected 1 message in CTO mailbox, got %d", len(ctoMailbox.Messages))
	}
	if ctoMailbox.Messages[0].ID != msg.ID {
		t.Errorf("Message ID mismatch in mailbox storage")
	}
}

func TestDeliverabilityCleanlinessScoringEngine(t *testing.T) {
	mgr := NewManager("103.189.201.42")

	// 1. Pristine authenticated email with full DKIM, SPF, DMARC
	pristineEmail := "From: ceo@company.test\r\n" +
		"To: partner@vendor.com\r\n" +
		"Subject: Partnership Agreement\r\n" +
		"Date: Thu, 29 Oct 2026 12:00:00 +0000\r\n" +
		"Message-ID: <unique-12345@company.test>\r\n" +
		"DKIM-Signature: v=1; a=rsa-sha256; d=company.test; s=default; b=abc123==;\r\n" +
		"Received-SPF: pass (domain of company.test designates 103.189.201.42 as permitted sender)\r\n" +
		"Authentication-Results: mx.vendor.com; dkim=pass; dmarc=pass (p=quarantine);\r\n\r\n" +
		"Please find attached the signed agreement for your review."

	scoreClean := mgr.CalculateCleanlinessScore(pristineEmail)
	if scoreClean.TotalScore < 9.5 {
		t.Errorf("Expected pristine email score >= 9.5, got %.2f (Checks: %v)", scoreClean.TotalScore, scoreClean.Checks)
	}
	if !scoreClean.DKIMPassed || !scoreClean.SPFPassed || !scoreClean.DMARCPassed {
		t.Errorf("Expected all authentication flags to pass: %+v", scoreClean)
	}

	// 2. Spammy unauthenticated email
	spamEmail := "From: suspicious@unknown.biz\r\n" +
		"To: victim@company.test\r\n" +
		"Subject: CLAIM YOUR PRIZE IMMEDIATELY\r\n\r\n" +
		"Congratulations! You won FREE MONEY in our CASINO BONUS. ACT NOW URGENT!"

	scoreSpam := mgr.CalculateCleanlinessScore(spamEmail)
	if scoreSpam.TotalScore > 4.0 {
		t.Errorf("Expected spam email score <= 4.0, got %.2f (Checks: %v)", scoreSpam.TotalScore, scoreSpam.Checks)
	}
	if scoreSpam.SpamPenaltyScore <= 0 {
		t.Errorf("Expected spam penalty score to be applied for spam keywords")
	}
}

func TestOutboundRateLimiting(t *testing.T) {
	mgr := NewManager("103.189.201.42")
	_, _ = mgr.AddDomain("limittest.com", "tenant-1")
	mb, _ := mgr.CreateMailbox("sender@limittest.com", "Pass123!", 1024)

	// Set low limit for testing
	mb.HourlyLimit = 5

	for i := 0; i < 5; i++ {
		_, err := mgr.SendMessage("sender@limittest.com", fmt.Sprintf("dest%d@other.com", i), "Notice", "Body content")
		if err != nil {
			t.Fatalf("Message %d send failed: %v", i, err)
		}
	}

	// 6th message must be blocked by rate limiter
	_, errBlocked := mgr.SendMessage("sender@limittest.com", "dest6@other.com", "Blocked", "Body content")
	if errBlocked != ErrRateLimitExceeded {
		t.Errorf("Expected ErrRateLimitExceeded, got: %v", errBlocked)
	}
}
