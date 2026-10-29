// DigiPanel Chained Audit Log Unit Tests (core/audit/audit_test.go)
// Verifies SHA-256 hash chaining and tamper-evident detection.

package audit

import (
	"testing"
)

func TestAuditLogChainIntegrity(t *testing.T) {
	logger := NewLogger()

	logger.Append("admin", "account.create", "account", "user101", `{"domain":"example.com"}`, "127.0.0.1")
	logger.Append("admin", "dns.zone_create", "zone", "example.com", `{"ttl":3600}`, "127.0.0.1")
	logger.Append("user101", "files.upload", "file", "index.php", `{"bytes":1024}`, "192.168.1.50")

	valid, count, err := logger.VerifyIntegrity()
	if err != nil || !valid {
		t.Fatalf("Audit chain integrity verification failed: %v", err)
	}

	if count != 4 { // 1 genesis + 3 appended
		t.Errorf("Expected 4 entries in audit chain, got %d", count)
	}
}

func TestAuditLogTamperingDetected(t *testing.T) {
	logger := NewLogger()

	logger.Append("admin", "account.create", "account", "client1", `{}`, "127.0.0.1")
	logger.Append("admin", "billing.invoice_pay", "invoice", "INV-100", `{"amount":500000}`, "127.0.0.1")
	logger.Append("admin", "service.activate", "service", "srv-1", `{}`, "127.0.0.1")

	valid, _, err := logger.VerifyIntegrity()
	if !valid || err != nil {
		t.Fatalf("Chain should be valid before tampering")
	}

	// Tamper with payload of entry index 2
	logger.TamperEntryForTesting(2, `{"amount":100000}`)

	validAfterTamper, tamperedIndex, tamperErr := logger.VerifyIntegrity()
	if validAfterTamper || tamperErr == nil {
		t.Errorf("Security failure: tampered audit log was not detected")
	}

	if tamperedIndex != 2 {
		t.Errorf("Expected tamper detection at index 2, got %d", tamperedIndex)
	}
}
