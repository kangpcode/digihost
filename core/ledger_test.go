// DigiPanel Double-Entry Ledger Unit Tests (core/ledger_test.go)
// Verifies double-entry ledger balance and integer currency precision

package main

import (
	"bytes"
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
	token := adminToken(t, handler)

	// The ledger starts empty. A real invoice is created and settled so the
	// posting is the result of the payment pipeline rather than seeded data.
	const invoiceAmount int64 = 40000000
	invoicePayload, _ := json.Marshal(map[string]any{
		"clientName":  "PT Nusantara Hosting",
		"clientEmail": "billing@nusantara.example",
		"amountCents": invoiceAmount,
		"currency":    "IDR",
		"description": "Annual reseller plan",
	})
	createReq, _ := http.NewRequest("POST", "/api/v1/billing/invoices", bytes.NewReader(invoicePayload))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	handler.ServeHTTP(createRecorder, createReq)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("Expected 201 creating invoice, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}
	var invoice Invoice
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &invoice); err != nil {
		t.Fatalf("decode invoice: %v", err)
	}

	payPayload, _ := json.Marshal(map[string]string{
		"invoiceId":        invoice.ID,
		"gateway":          "midtrans",
		"gatewayReference": "MID-2026-0001",
	})
	payReq, _ := http.NewRequest("POST", "/api/v1/billing/invoices/pay", bytes.NewReader(payPayload))
	payReq.Header.Set("Authorization", "Bearer "+token)
	payReq.Header.Set("Content-Type", "application/json")
	payRecorder := httptest.NewRecorder()
	handler.ServeHTTP(payRecorder, payReq)
	if payRecorder.Code != http.StatusOK {
		t.Fatalf("Expected 200 settling invoice, got %d: %s", payRecorder.Code, payRecorder.Body.String())
	}

	req, _ := http.NewRequest("GET", "/api/v1/billing/ledger", nil)
	req.Header.Set("Authorization", "Bearer "+token)
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

	if resp.TotalDebits != invoiceAmount {
		t.Errorf("Expected total balance of %d cents, got %d", invoiceAmount, resp.TotalDebits)
	}

	// Replaying the same payment must be idempotent: no second ledger pair.
	replayReq, _ := http.NewRequest("POST", "/api/v1/billing/invoices/pay", bytes.NewReader(payPayload))
	replayReq.Header.Set("Authorization", "Bearer "+token)
	replayReq.Header.Set("Content-Type", "application/json")
	replayRecorder := httptest.NewRecorder()
	handler.ServeHTTP(replayRecorder, replayReq)
	if replayRecorder.Code != http.StatusOK {
		t.Fatalf("Expected 200 replaying settlement, got %d", replayRecorder.Code)
	}

	again, _ := http.NewRequest("GET", "/api/v1/billing/ledger", nil)
	again.Header.Set("Authorization", "Bearer "+token)
	againRecorder := httptest.NewRecorder()
	handler.ServeHTTP(againRecorder, again)
	var replay struct {
		Entries      []LedgerEntry `json:"entries"`
		TotalDebits  int64         `json:"totalDebits"`
		TotalCredits int64         `json:"totalCredits"`
		IsBalanced   bool          `json:"isBalanced"`
	}
	if err := json.NewDecoder(againRecorder.Body).Decode(&replay); err != nil {
		t.Fatalf("decode replayed ledger: %v", err)
	}
	if len(replay.Entries) != len(resp.Entries) || replay.TotalDebits != resp.TotalDebits || !replay.IsBalanced {
		t.Errorf("Replayed payment posted new ledger entries: before=%d after=%d", len(resp.Entries), len(replay.Entries))
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
