// DigiPanel Double-Entry Ledger Unit Tests (core/ledger_test.go)
// Verifies double-entry ledger balance and integer currency precision

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDoubleEntryLedgerBalance(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "digipanel-ledger-test-*")
	state := &ServerState{
		StorageDir: tmpDir,
		ModulesDir: tmpDir,
	}
	initStorage(state)
	handler := NewServerHandler(state)

	req, _ := http.NewRequest("GET", "/api/v1/billing/ledger", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t, handler))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rr.Code)
	}

	var resp struct {
		Entries      []LedgerEntry `json:"entries"`
		TotalDebits  int64         `json:"totalDebits"`
		TotalCredits int64         `json:"totalCredits"`
		IsBalanced   bool          `json:"isBalanced"`
		Currency     string        `json:"currency"`
	}

	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !resp.IsBalanced {
		t.Errorf("Double-entry ledger is not balanced: debits=%d, credits=%d", resp.TotalDebits, resp.TotalCredits)
	}

	if resp.TotalDebits != resp.TotalCredits {
		t.Errorf("Debits %d != Credits %d", resp.TotalDebits, resp.TotalCredits)
	}

	if resp.TotalDebits != 40000000 {
		t.Errorf("Expected total balance of 40,000,000 cents, got %d", resp.TotalDebits)
	}
}

func TestUnbalancedLedgerDetection(t *testing.T) {
	state := &ServerState{
		LedgerItems: []LedgerEntry{
			{ID: 1, TxRef: "TX-999", AccountCode: "1010-CASH", AmountCents: 10000, Direction: "debit", Currency: "IDR"},
			{ID: 2, TxRef: "TX-999", AccountCode: "4010-REVENUE", AmountCents: 9000, Direction: "credit", Currency: "IDR"},
		},
	}
	handler := NewServerHandler(state)

	req, _ := http.NewRequest("GET", "/api/v1/billing/ledger", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t, handler))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var resp struct {
		IsBalanced bool `json:"isBalanced"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&resp)

	if resp.IsBalanced {
		t.Errorf("Ledger should be marked unbalanced when debit (10000) != credit (9000)")
	}
}
