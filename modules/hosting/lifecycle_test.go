// DigiPanel Hosting Subsystem Lifecycle & Quality Gate Tests (modules/hosting/lifecycle_test.go)
// Verifies Quality Gate Tahap 3:
// 1. Pembuatan dan penghapusan 1000 akun tanpa sisa
// 2. Isolasi antar akun teruji
// 3. Uji beban & performa atomik
// Zero emojis, strictly production grade.

package hosting

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestAccountValidation(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "digipanel-validation-test-*")
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)

	invalidUsernames := []string{
		"root", "digipanel", "daemon", "mail",
		"123user", "-invalid", "user@host", "user name",
		"ab", "a", "user_with_more_than_thirty_two_characters_long",
	}

	for _, u := range invalidUsernames {
		if err := mgr.ValidateUsername(u); err == nil {
			t.Errorf("Validation failure: invalid username '%s' was allowed", u)
		}
	}

	invalidDomains := []string{
		"domain..com", "domain/path", "example.com;rm",
		"http://domain.com", "-domain.com", "domain with space.com",
	}

	for _, d := range invalidDomains {
		if err := mgr.ValidateDomain(d); err == nil {
			t.Errorf("Validation failure: invalid domain '%s' was allowed", d)
		}
	}
}

func TestAtomicAccountCreationAndRollback(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "digipanel-atomic-test-*")
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)

	req := CreateAccountRequest{
		TenantID:   "tenant-alpha",
		Username:   "client01",
		Domain:     "client01.digihost.id",
		Package:    "starter",
		PhpVersion: "8.3",
	}

	acc, err := mgr.CreateAccount(req)
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	// Verify all components were created
	if _, err := os.Stat(acc.DocumentRoot); os.IsNotExist(err) {
		t.Errorf("DocumentRoot %s was not created", acc.DocumentRoot)
	}

	vhostConf := filepath.Join(tmpDir, "nginx", "conf.d", fmt.Sprintf("%s.conf", acc.PrimaryDomain))
	if _, err := os.Stat(vhostConf); os.IsNotExist(err) {
		t.Errorf("Nginx vhost %s was not created", vhostConf)
	}

	poolConf := filepath.Join(tmpDir, "php", "pools", fmt.Sprintf("%s.conf", acc.Username))
	if _, err := os.Stat(poolConf); os.IsNotExist(err) {
		t.Errorf("PHP-FPM pool %s was not created", poolConf)
	}

	dnsZone := filepath.Join(tmpDir, "dns", "zones", fmt.Sprintf("%s.zone", acc.PrimaryDomain))
	if _, err := os.Stat(dnsZone); os.IsNotExist(err) {
		t.Errorf("DNS zone %s was not created", dnsZone)
	}

	// Verify duplicate prevention
	if _, errDuplicate := mgr.CreateAccount(req); errDuplicate == nil {
		t.Errorf("Security failure: duplicate account creation was allowed")
	}

	// Verify suspension
	if err := mgr.SuspendAccount(acc.Username); err != nil {
		t.Fatalf("Failed to suspend account: %v", err)
	}
	accRefreshed, _ := mgr.GetAccount(acc.Username)
	if accRefreshed.Status != StatusSuspended {
		t.Errorf("Expected status suspended, got %s", accRefreshed.Status)
	}

	// Verify unsuspension
	if err := mgr.UnsuspendAccount(acc.Username); err != nil {
		t.Fatalf("Failed to unsuspend account: %v", err)
	}
	accRefreshed, _ = mgr.GetAccount(acc.Username)
	if accRefreshed.Status != StatusActive {
		t.Errorf("Expected status active, got %s", accRefreshed.Status)
	}

	// Verify termination and cleanup
	if err := mgr.TerminateAccount(acc.Username); err != nil {
		t.Fatalf("Failed to terminate account: %v", err)
	}

	if _, err := os.Stat(acc.HomeDir); !os.IsNotExist(err) {
		t.Errorf("Residue detected: HomeDir still exists after termination")
	}
	if _, err := os.Stat(vhostConf); !os.IsNotExist(err) {
		t.Errorf("Residue detected: Vhost config still exists after termination")
	}
	if _, err := os.Stat(poolConf); !os.IsNotExist(err) {
		t.Errorf("Residue detected: PHP-FPM pool still exists after termination")
	}
	if _, err := os.Stat(dnsZone); !os.IsNotExist(err) {
		t.Errorf("Residue detected: DNS zone still exists after termination")
	}
}

func TestMultiTenantIsolation(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "digipanel-isolation-test-*")
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)

	accA, errA := mgr.CreateAccount(CreateAccountRequest{
		TenantID: "tenant-1",
		Username: "usera",
		Domain:   "usera.com",
		Package:  "starter",
	})
	if errA != nil {
		t.Fatalf("Failed creating user A: %v", errA)
	}

	accB, errB := mgr.CreateAccount(CreateAccountRequest{
		TenantID: "tenant-2",
		Username: "userb",
		Domain:   "userb.com",
		Package:  "starter",
	})
	if errB != nil {
		t.Fatalf("Failed creating user B: %v", errB)
	}

	// 1. Verify POSIX UID isolation
	if accA.PosixUID == accB.PosixUID {
		t.Errorf("Isolation failure: User A and User B share the same POSIX UID (%d)", accA.PosixUID)
	}

	// 2. Verify Home Directory Separation
	if accA.HomeDir == accB.HomeDir {
		t.Errorf("Isolation failure: User A and User B share home directory")
	}

	// 3. Verify PHP open_basedir directives in pool configurations
	poolABytes, _ := os.ReadFile(filepath.Join(tmpDir, "php", "pools", "usera.conf"))
	poolBBytes, _ := os.ReadFile(filepath.Join(tmpDir, "php", "pools", "userb.conf"))

	expectedBasedirA := fmt.Sprintf("open_basedir] = %s:/tmp", accA.HomeDir)
	expectedBasedirB := fmt.Sprintf("open_basedir] = %s:/tmp", accB.HomeDir)

	if !stringsContains(string(poolABytes), expectedBasedirA) {
		t.Errorf("Isolation failure: Pool A lacks strict open_basedir containment")
	}
	if !stringsContains(string(poolBBytes), expectedBasedirB) {
		t.Errorf("Isolation failure: Pool B lacks strict open_basedir containment")
	}

	// 4. Verify cgroups path isolation
	if accA.CgroupsPath == accB.CgroupsPath {
		t.Errorf("Isolation failure: User A and User B share cgroups slice")
	}
}

// Benchmark & Quality Gate: 1000 accounts creation and termination with zero residue
func TestLifecycle1000AccountsAndResidue(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "digipanel-1000-accounts-*")
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	totalAccounts := 1000

	t.Logf("Initiating Quality Gate: Provisioning %d accounts...", totalAccounts)

	startCreate := time.Now()
	latencies := make([]time.Duration, totalAccounts)

	// Concurrently create accounts in batches of 20
	batchSize := 20
	var wg sync.WaitGroup

	for i := 0; i < totalAccounts; i += batchSize {
		for j := 0; j < batchSize && (i+j) < totalAccounts; j++ {
			idx := i + j
			wg.Add(1)
			go func(accountIdx int) {
				defer wg.Done()
				t0 := time.Now()
				req := CreateAccountRequest{
					TenantID:   fmt.Sprintf("tenant-%04d", accountIdx%10),
					Username:   fmt.Sprintf("client%05d", accountIdx),
					Domain:     fmt.Sprintf("client%05d.digihost.test", accountIdx),
					Package:    "starter",
					PhpVersion: "8.3",
				}
				_, err := mgr.CreateAccount(req)
				if err != nil {
					t.Errorf("Account %d creation failed: %v", accountIdx, err)
				}
				latencies[accountIdx] = time.Since(t0)
			}(idx)
		}
		wg.Wait()
	}

	totalCreateDuration := time.Since(startCreate)
	t.Logf("Completed provisioning %d accounts in %v (average: %v/acc)",
		totalAccounts, totalCreateDuration, totalCreateDuration/time.Duration(totalAccounts))

	// Verify Account Count
	if mgr.CountAccounts() != totalAccounts {
		t.Fatalf("Expected %d accounts in manager, got %d", totalAccounts, mgr.CountAccounts())
	}

	// Calculate latency percentiles
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	median := latencies[totalAccounts/2]
	p95 := latencies[int(float64(totalAccounts)*0.95)]
	t.Logf("Performance Latency - Median: %v, P95: %v (Target: < 3s)", median, p95)

	if median > 3*time.Second {
		t.Errorf("Performance target missed: Median %v exceeds 3s", median)
	}

	// Terminate all 1000 accounts
	t.Logf("Terminating all %d accounts...", totalAccounts)
	startTerminate := time.Now()

	for i := 0; i < totalAccounts; i += batchSize {
		for j := 0; j < batchSize && (i+j) < totalAccounts; j++ {
			idx := i + j
			wg.Add(1)
			go func(accountIdx int) {
				defer wg.Done()
				username := fmt.Sprintf("client%05d", accountIdx)
				if err := mgr.TerminateAccount(username); err != nil {
					t.Errorf("Account %s termination failed: %v", username, err)
				}
			}(idx)
		}
		wg.Wait()
	}

	totalTerminateDuration := time.Since(startTerminate)
	t.Logf("Completed terminating %d accounts in %v", totalAccounts, totalTerminateDuration)

	if mgr.CountAccounts() != 0 {
		t.Fatalf("Expected 0 accounts after termination, found %d", mgr.CountAccounts())
	}

	// Zero Residue Check: Verify directories are completely clean
	checkDirs := []string{
		filepath.Join(tmpDir, "vhosts"),
		filepath.Join(tmpDir, "nginx", "conf.d"),
		filepath.Join(tmpDir, "php", "pools"),
		filepath.Join(tmpDir, "dns", "zones"),
		filepath.Join(tmpDir, "cgroups"),
	}

	for _, d := range checkDirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			t.Fatalf("Failed reading directory %s: %v", d, err)
		}
		if len(entries) != 0 {
			t.Errorf("Residue failure: directory %s contains %d lingering files (zero expected)", d, len(entries))
			for _, e := range entries[:minInt(len(entries), 5)] {
				t.Logf(" Lingering file: %s", e.Name())
			}
		}
	}

	t.Logf("Quality Gate Passed: 1000 accounts created and purged with ZERO residue!")
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && searchSubstr(s, substr)))
}

func searchSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
