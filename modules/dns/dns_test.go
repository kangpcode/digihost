// DigiPanel PowerDNS Subsystem Tests (modules/dns/dns_test.go)
// Standards: Zero emojis, strictly production grade.

package dns

import (
	"strings"
	"testing"
)

func TestZoneValidation(t *testing.T) {
	validZones := []string{"example.com", "my-site.id", "sub.domain.co.id"}
	for _, z := range validZones {
		if err := ValidateZoneName(z); err != nil {
			t.Errorf("Validation failed for valid zone '%s': %v", z, err)
		}
	}

	invalidZones := []string{
		"domain..com", "domain;rm", "domain/path", "domain with space.com",
		"a", "ab", "-startwithhyphen.com",
	}
	for _, z := range invalidZones {
		if err := ValidateZoneName(z); err == nil {
			t.Errorf("Validation should fail for invalid zone '%s'", z)
		}
	}
}

func TestRecordValidation(t *testing.T) {
	validRecords := []Record{
		{Name: "@", Type: TypeA, Value: "1.2.3.4", TTL: 3600},
		{Name: "ipv6", Type: TypeAAAA, Value: "2001:db8::1", TTL: 3600},
		{Name: "www", Type: TypeCNAME, Value: "example.com.", TTL: 3600},
		{Name: "@", Type: TypeMX, Value: "mail.example.com.", TTL: 3600, Priority: 10},
		{Name: "_dmarc", Type: TypeTXT, Value: "\"v=DMARC1; p=none\"", TTL: 3600},
	}
	for _, r := range validRecords {
		if err := ValidateRecord(r); err != nil {
			t.Errorf("Record validation failed for valid record %+v: %v", r, err)
		}
	}

	invalidRecords := []Record{
		{Name: "@", Type: TypeA, Value: "999.999.999.999", TTL: 3600},           // Invalid IPv4
		{Name: "@", Type: TypeAAAA, Value: "1.2.3.4", TTL: 3600},                // IPv4 in AAAA
		{Name: "bad", Type: "UNKNOWN", Value: "1.2.3.4", TTL: 3600},             // Unknown type
		{Name: "bad", Type: TypeA, Value: "1.2.3.4", TTL: 10},                   // TTL too low (< 60)
		{Name: "bad", Type: TypeMX, Value: "mail.com", TTL: 3600, Priority: -1}, // Negative priority
	}
	for _, r := range invalidRecords {
		if err := ValidateRecord(r); err == nil {
			t.Errorf("Record validation should fail for invalid record %+v", r)
		}
	}
}

func TestZoneLifecycleAndRecords(t *testing.T) {
	mgr := NewManager()

	// 1. Create Zone
	zone, err := mgr.CreateZone("digipanel.test", "tenant-1", "ns1.digipanel.id", "admin@digipanel.test", "103.189.201.42")
	if err != nil {
		t.Fatalf("Failed to create zone: %v", err)
	}

	if zone.Name != "digipanel.test" {
		t.Errorf("Expected zone name digipanel.test, got %s", zone.Name)
	}

	// Duplicate rejection
	if _, errDup := mgr.CreateZone("digipanel.test", "tenant-1", "", "", ""); errDup == nil {
		t.Errorf("Security failure: duplicate zone creation was permitted")
	}

	// 2. Add Record
	newRec, err := mgr.AddRecord("digipanel.test", Record{
		Name:  "blog",
		Type:  TypeA,
		Value: "103.189.201.43",
		TTL:   3600,
	})
	if err != nil {
		t.Fatalf("Failed to add record: %v", err)
	}

	if newRec.ID == "" {
		t.Errorf("Record ID was not assigned")
	}

	// 3. Delete Record
	if err := mgr.DeleteRecord("digipanel.test", newRec.ID); err != nil {
		t.Fatalf("Failed to delete record: %v", err)
	}

	// Verify SOA cannot be deleted
	if err := mgr.DeleteRecord("digipanel.test", "rec-soa"); err == nil {
		t.Errorf("Security failure: deletion of mandatory SOA record was permitted")
	}
}

func TestDNSSECEnablementAndDSRecord(t *testing.T) {
	mgr := NewManager()
	zone, err := mgr.CreateZone("secure-crypto.id", "tenant-alpha", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create zone: %v", err)
	}

	dnssec, err := mgr.EnableDNSSEC(zone.Name)
	if err != nil {
		t.Fatalf("Failed to enable DNSSEC: %v", err)
	}

	if !dnssec.Enabled {
		t.Errorf("DNSSEC reported disabled after enablement")
	}
	if dnssec.Algorithm != 13 { // ECDSAP256SHA256
		t.Errorf("Expected algorithm 13 (ECDSAP256SHA256), got %d", dnssec.Algorithm)
	}
	if dnssec.DigestType != 2 { // SHA-256
		t.Errorf("Expected digest type 2 (SHA-256), got %d", dnssec.DigestType)
	}
	if len(dnssec.Digest) != 64 { // SHA-256 hex string is 64 chars
		t.Errorf("Expected 64-character SHA-256 hex digest, got length %d", len(dnssec.Digest))
	}

	// Verify standard DS record syntax
	expectedPrefix := "secure-crypto.id. IN DS "
	if !strings.HasPrefix(dnssec.DSRecord, expectedPrefix) {
		t.Errorf("DS record '%s' missing expected prefix '%s'", dnssec.DSRecord, expectedPrefix)
	}

	// Export BIND zone and verify DS record inclusion
	bindExport, err := mgr.ExportBindZone(zone.Name)
	if err != nil {
		t.Fatalf("Failed to export BIND zone: %v", err)
	}

	if !strings.Contains(bindExport, dnssec.DSRecord) {
		t.Errorf("Exported BIND zone missing DNSSEC DS record")
	}

	// Disable DNSSEC
	if err := mgr.DisableDNSSEC(zone.Name); err != nil {
		t.Fatalf("Failed to disable DNSSEC: %v", err)
	}
	zRefreshed, _ := mgr.GetZone(zone.Name)
	if zRefreshed.Dnssec.Enabled {
		t.Errorf("DNSSEC still enabled after disable call")
	}
}

func TestZoneHealthChecks(t *testing.T) {
	mgr := NewManager()

	// 1. Healthy Zone
	z, _ := mgr.CreateZone("healthy.org", "tenant-1", "", "", "1.2.3.4")
	health, err := mgr.InspectZoneHealth(z.Name)
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	if !health.Healthy {
		t.Errorf("Expected zone to be healthy, got issues: %v", health.Issues)
	}
	if !health.HasSOA || !health.HasApexA || !health.HasRedundantNS {
		t.Errorf("Health flags incorrect: %+v", health)
	}

	// 2. Unhealthy Zone missing NS redundancy
	zUnhealthy, _ := mgr.CreateZone("unhealthy.org", "tenant-1", "", "", "1.2.3.4")
	_ = mgr.DeleteRecord(zUnhealthy.Name, "rec-ns2")
	_ = mgr.DeleteRecord(zUnhealthy.Name, "rec-ns1")

	healthBad, err := mgr.InspectZoneHealth(zUnhealthy.Name)
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	if healthBad.Healthy {
		t.Errorf("Expected zone to be unhealthy due to missing NS records")
	}
}
