// DigiPanel Tahap 10 & 11 Performance & API Latency Benchmark Suite (core/benchmark_test.go)
// Implements Section 19 repeatable protocol: 5 iterations reporting median & spread.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"digipanel/modules/hosting"
)

func TestAPILatencyP95UnderTarget(t *testing.T) {
	_, handler := setupIntegratedTestServer()
	token := login(t, handler, "admin", testAdminPassword)

	const runs = 5
	const totalRequests = 200
	runP95s := make([]time.Duration, runs)

	for run := 0; run < runs; run++ {
		latencies := make([]time.Duration, totalRequests)
		var wg sync.WaitGroup
		wg.Add(totalRequests)
		for i := 0; i < totalRequests; i++ {
			idx := i
			go func() {
				defer wg.Done()
				start := time.Now()
				req, _ := http.NewRequest("GET", "/api/v1/hosting/accounts", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				latencies[idx] = time.Since(start)
				if rr.Code != http.StatusOK {
					t.Errorf("unexpected status %d", rr.Code)
				}
			}()
		}
		wg.Wait()
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		runP95s[run] = latencies[(totalRequests*95)/100]
	}

	sort.Slice(runP95s, func(i, j int) bool { return runP95s[i] < runP95s[j] })
	medianP95 := runP95s[runs/2]
	t.Logf("API 200-concurrent p95 across 5 runs: runs=%v median=%v min=%v max=%v", runP95s, medianP95, runP95s[0], runP95s[runs-1])
	if medianP95 >= 300*time.Millisecond {
		t.Fatalf("median p95 latency %v exceeded 300ms target", medianP95)
	}
}

func TestBenchmark100SequentialAccounts5Runs(t *testing.T) {
	const runs = 5
	const count = 100
	perAcctMedians := make([]time.Duration, runs)
	perAcctP95s := make([]time.Duration, runs)
	totals := make([]time.Duration, runs)

	for run := 0; run < runs; run++ {
		state, _ := setupIntegratedTestServer()
		durs := make([]time.Duration, count)
		runStart := time.Now()
		for i := 0; i < count; i++ {
			user := fmt.Sprintf("benchu%d%03d", run, i)
			dom := fmt.Sprintf("%s.example.com", user)
			start := time.Now()
			if _, err := state.HostingManager.CreateAccount(hosting.CreateAccountRequest{
				TenantID: "tenant-default",
				Username: user,
				Domain:   dom,
				Package:  "starter",
			}); err != nil {
				t.Fatalf("CreateAccount %s: %v", user, err)
			}
			if _, err := state.DNSManager.CreateZone(dom, "tenant-default", "ns1.digipanel.id.", "hostmaster@"+dom, "127.0.0.1"); err != nil {
				t.Fatalf("CreateZone %s: %v", dom, err)
			}
			if _, err := state.MailManager.AddDomain(dom, "tenant-default"); err != nil {
				t.Fatalf("AddDomain %s: %v", dom, err)
			}
			durs[i] = time.Since(start)
		}
		totals[run] = time.Since(runStart)
		sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
		perAcctMedians[run] = durs[count/2]
		perAcctP95s[run] = durs[(count*95)/100]
	}

	sort.Slice(perAcctMedians, func(i, j int) bool { return perAcctMedians[i] < perAcctMedians[j] })
	sort.Slice(perAcctP95s, func(i, j int) bool { return perAcctP95s[i] < perAcctP95s[j] })
	medianPerAcct := perAcctMedians[runs/2]
	medianP95 := perAcctP95s[runs/2]
	t.Logf("100-account provisioning (user+vhost+dns+mail) x 5 runs: totals=%v median_per_account=%v median_p95=%v", totals, medianPerAcct, medianP95)
	if medianPerAcct >= 3*time.Second {
		t.Fatalf("median account creation %v exceeded 3s target", medianPerAcct)
	}
}

func BenchmarkAuthenticatedAccountsAPI(b *testing.B) {
	_, handler := setupIntegratedTestServer()
	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": testAdminPassword,
	})
	loginReq, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRR := httptest.NewRecorder()
	handler.ServeHTTP(loginRR, loginReq)
	var resp map[string]string
	_ = json.Unmarshal(loginRR.Body.Bytes(), &resp)
	tok := resp["token"]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/hosting/accounts", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("unexpected HTTP %d", rr.Code)
		}
	}
}
