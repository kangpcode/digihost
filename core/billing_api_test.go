// DigiPanel Native Billing REST API Integration Tests (core/billing_api_test.go)

package main

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNativeBillingRESTLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	state := &ServerState{StorageDir: tmpDir, ModulesDir: tmpDir}
	initStorage(state)
	handler := NewServerHandler(state)
	token := adminToken(t, handler)
	state.billingEngine().SetGatewaySecret("midtrans", "rest-midtrans-secret")

	// 1. Create product
	prodBody, _ := json.Marshal(map[string]any{
		"slug":          "cloud-starter",
		"name":          "Cloud Starter",
		"productType":   "hosting",
		"packageName":   "starter",
		"currency":      "IDR",
		"monthlyCents":  150000,
		"annualCents":   1500000,
		"autoProvision": true,
	})
	req, _ := http.NewRequest("POST", "/api/v1/billing/products", bytes.NewReader(prodBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create product expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var product struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &product)

	// 2. Register client
	cliBody, _ := json.Marshal(map[string]any{
		"fullName": "Siti Aminah",
		"email":    "siti@cloud.example",
		"country":  "ID",
		"currency": "IDR",
	})
	req, _ = http.NewRequest("POST", "/api/v1/billing/clients", bytes.NewReader(cliBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create client expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var client struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &client)

	// 3. Place order
	ordBody, _ := json.Marshal(map[string]any{
		"clientId":     client.ID,
		"productId":    product.ID,
		"billingCycle": "monthly",
		"domain":       "sitihost.example.com",
		"username":     "sitihost1",
	})
	req, _ = http.NewRequest("POST", "/api/v1/billing/orders", bytes.NewReader(ordBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("place order expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var ordResp struct {
		Invoice struct {
			ID          string `json:"id"`
			AmountCents int64  `json:"amountCents"`
		} `json:"invoice"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &ordResp)
	if ordResp.Invoice.AmountCents != 166500 {
		t.Fatalf("expected 150000 + 11%% PPN = 166500, got %d", ordResp.Invoice.AmountCents)
	}

	// 4. Signed Midtrans webhook settles invoice and provisions hosting account
	sigSum := sha512.Sum512([]byte(ordResp.Invoice.ID + "200166500rest-midtrans-secret"))
	hookBody, _ := json.Marshal(map[string]string{
		"order_id":           ordResp.Invoice.ID,
		"status_code":        "200",
		"gross_amount":       "166500",
		"signature_key":      hex.EncodeToString(sigSum[:]),
		"transaction_status": "settlement",
		"transaction_id":     "MID-REST-1",
	})
	req, _ = http.NewRequest("POST", "/api/v1/billing/webhooks/midtrans", bytes.NewReader(hookBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("signed webhook expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := state.HostingManager.GetAccount("sitihost1"); err != nil {
		t.Fatalf("expected sitihost1 hosting account to be provisioned after webhook: %v", err)
	}

	// 5. PDF invoice download
	req, _ = http.NewRequest("GET", "/api/v1/billing/invoices/pdf?id="+ordResp.Invoice.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !bytes.HasPrefix(rr.Body.Bytes(), []byte("%PDF-1.4")) {
		t.Fatalf("expected valid PDF invoice, got status=%d", rr.Code)
	}

	// 6. Reports endpoint
	req, _ = http.NewRequest("GET", "/api/v1/billing/reports", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("reports expected 200, got %d", rr.Code)
	}

	// 7. Xendit invalid callback token rejected with 401, valid token accepted & idempotent
	state.billingEngine().SetGatewaySecret("xendit", "xendit-rest-secret")
	xBody, _ := json.Marshal(map[string]string{
		"id":          "XND-100",
		"external_id": ordResp.Invoice.ID,
		"status":      "PAID",
	})
	req, _ = http.NewRequest("POST", "/api/v1/billing/webhooks/xendit", bytes.NewReader(xBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Callback-Token", "wrong-token")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("invalid xendit callback token expected 401, got %d", rr.Code)
	}

	req, _ = http.NewRequest("POST", "/api/v1/billing/webhooks/xendit", bytes.NewReader(xBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Callback-Token", "xendit-rest-secret")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("valid xendit webhook expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// 8. Refund via REST maintains double-entry ledger balance
	refBody, _ := json.Marshal(map[string]any{
		"invoiceId":   ordResp.Invoice.ID,
		"refundCents": 25000,
		"reason":      "rest partial refund",
	})
	req, _ = http.NewRequest("POST", "/api/v1/billing/invoices/refund", bytes.NewReader(refBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("refund expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	_, debits, credits, balanced := state.billingEngine().LedgerSummary()
	if !balanced || debits != credits {
		t.Fatalf("billing ledger unbalanced after REST refund: debits=%d credits=%d", debits, credits)
	}
}
