// DigiPanel Billing Invoices (core/billing.go)
// Native invoice lifecycle with integer minor units, sequential numbering, and
// atomic double-entry posting on payment. No card data is ever accepted: a
// payment carries only a gateway identifier and its external reference.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Invoice is a billable document. Amounts are integer minor units in the
// invoice currency, never floating point.
type Invoice struct {
	ID               string     `json:"id"`
	InvoiceNumber    string     `json:"invoiceNumber"`
	ClientName       string     `json:"clientName"`
	ClientEmail      string     `json:"clientEmail"`
	AmountCents      int64      `json:"amountCents"`
	Currency         string     `json:"currency"`
	Status           string     `json:"status"`
	IssueDate        string     `json:"issueDate"`
	DueDate          string     `json:"dueDate"`
	Description      string     `json:"description"`
	Gateway          string     `json:"gateway,omitempty"`
	GatewayReference string     `json:"gatewayReference,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	PaidAt           *time.Time `json:"paidAt,omitempty"`
}

// InvoiceStore owns invoice state and the monotonic numbering sequence.
type InvoiceStore struct {
	mu    sync.Mutex
	items []*Invoice
	next  int
}

var (
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	invoiceDate     = "2006-01-02"
)

// Allowed payment gateways. The panel stores tokens and references only.
var allowedGateways = map[string]bool{
	"manual":        true,
	"bank_transfer": true,
	"midtrans":      true,
	"xendit":        true,
	"stripe":        true,
	"paypal":        true,
}

type createInvoiceRequest struct {
	ClientName  string `json:"clientName"`
	ClientEmail string `json:"clientEmail"`
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency"`
	IssueDate   string `json:"issueDate"`
	DueDate     string `json:"dueDate"`
	Description string `json:"description"`
}

func (state *ServerState) invoiceStore() *InvoiceStore {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.Invoices == nil {
		state.Invoices = &InvoiceStore{next: 1}
	}
	return state.Invoices
}

func (store *InvoiceStore) list() []*Invoice {
	store.mu.Lock()
	defer store.mu.Unlock()

	out := make([]*Invoice, 0, len(store.items))
	for _, item := range store.items {
		copied := *item
		out = append(out, &copied)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (store *InvoiceStore) get(id string) (*Invoice, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, item := range store.items {
		if item.ID == id || item.InvoiceNumber == id {
			copied := *item
			return &copied, true
		}
	}
	return nil, false
}

func (store *InvoiceStore) create(req createInvoiceRequest) (*Invoice, error) {
	clientName := strings.TrimSpace(req.ClientName)
	if len(clientName) < 2 || len(clientName) > 120 {
		return nil, errors.New("clientName must be 2-120 characters")
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(req.ClientEmail)); err != nil {
		return nil, errors.New("clientEmail must be a valid address")
	}
	if req.AmountCents <= 0 {
		return nil, errors.New("amountCents must be a positive integer in minor units")
	}
	if req.AmountCents > 1_000_000_000_000 {
		return nil, errors.New("amountCents exceeds the supported maximum")
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "IDR"
	}
	if !currencyPattern.MatchString(currency) {
		return nil, errors.New("currency must be a three letter ISO 4217 code")
	}

	issue := time.Now()
	if strings.TrimSpace(req.IssueDate) != "" {
		parsed, err := time.Parse(invoiceDate, req.IssueDate)
		if err != nil {
			return nil, errors.New("issueDate must use YYYY-MM-DD")
		}
		issue = parsed
	}
	due := issue.AddDate(0, 0, 14)
	if strings.TrimSpace(req.DueDate) != "" {
		parsed, err := time.Parse(invoiceDate, req.DueDate)
		if err != nil {
			return nil, errors.New("dueDate must use YYYY-MM-DD")
		}
		due = parsed
	}
	if due.Before(issue) {
		return nil, errors.New("dueDate cannot be earlier than issueDate")
	}

	description := strings.TrimSpace(req.Description)
	if description == "" {
		return nil, errors.New("description is required")
	}
	if len(description) > 300 {
		return nil, errors.New("description must be at most 300 characters")
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	sequence := store.next
	store.next++
	invoice := &Invoice{
		ID:            fmt.Sprintf("inv-%d", sequence),
		InvoiceNumber: fmt.Sprintf("INV-%d-%04d", issue.Year(), sequence),
		ClientName:    clientName,
		ClientEmail:   strings.TrimSpace(req.ClientEmail),
		AmountCents:   req.AmountCents,
		Currency:      currency,
		Status:        "unpaid",
		IssueDate:     issue.Format(invoiceDate),
		DueDate:       due.Format(invoiceDate),
		Description:   description,
		CreatedAt:     time.Now().UTC(),
	}
	store.items = append(store.items, invoice)
	copied := *invoice
	return &copied, nil
}

// markPaidLocked settles an invoice. The caller must hold both the server and
// the store mutex so that the status change and the ledger posting commit as one
// unit.
func (store *InvoiceStore) markPaidLocked(id, gateway, reference string) (*Invoice, bool, error) {
	if !allowedGateways[gateway] {
		return nil, false, fmt.Errorf("unsupported payment gateway %q", gateway)
	}
	if len(reference) > 160 {
		return nil, false, errors.New("gatewayReference must be at most 160 characters")
	}

	for _, item := range store.items {
		if item.ID != id && item.InvoiceNumber != id {
			continue
		}
		if item.Status == "paid" {
			// Idempotent replay: return the settled invoice without posting a
			// second set of ledger entries.
			copied := *item
			return &copied, true, nil
		}
		if item.Status == "cancelled" {
			return nil, false, errors.New("cancelled invoices cannot be paid")
		}
		now := time.Now().UTC()
		item.Status = "paid"
		item.Gateway = gateway
		item.GatewayReference = strings.TrimSpace(reference)
		item.PaidAt = &now
		copied := *item
		return &copied, false, nil
	}
	return nil, false, errors.New("invoice not found")
}

// postInvoiceLedger records the balanced debit and credit pair for a payment.
// Callers must hold the state mutex.
func (state *ServerState) postInvoiceLedger(invoice *Invoice) {
	cashAccount := "1010-CASH-GATEWAY-" + strings.ToUpper(invoice.Gateway)
	revenueAccount := "4010-REVENUE-HOSTING-PLANS"
	txRef := invoice.InvoiceNumber

	nextID := int64(1)
	for _, entry := range state.LedgerItems {
		if entry.ID >= nextID {
			nextID = entry.ID + 1
		}
	}
	state.LedgerItems = append(state.LedgerItems,
		LedgerEntry{ID: nextID, TxRef: txRef, AccountCode: cashAccount, AmountCents: invoice.AmountCents, Direction: "debit", Currency: invoice.Currency},
		LedgerEntry{ID: nextID + 1, TxRef: txRef, AccountCode: revenueAccount, AmountCents: invoice.AmountCents, Direction: "credit", Currency: invoice.Currency},
	)
}

func (state *ServerState) handleInvoices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		invoices := state.invoiceStore().list()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(invoices)
	case http.MethodPost:
		var req createInvoiceRequest
		if err := decodeJSONBody(w, r, &req); err != nil {
			writeAPIError(w, "Invalid invoice payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		invoice, err := state.invoiceStore().create(req)
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.AuditLogger.Append("admin", "billing.invoice.create", "invoice", invoice.InvoiceNumber,
			fmt.Sprintf(`{"client":%q,"amountCents":%d,"currency":%q}`, invoice.ClientEmail, invoice.AmountCents, invoice.Currency), r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(invoice)
	default:
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (state *ServerState) handleInvoicePayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		InvoiceID        string `json:"invoiceId"`
		Gateway          string `json:"gateway"`
		GatewayReference string `json:"gatewayReference"`
	}
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid payment payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.InvoiceID) == "" {
		writeAPIError(w, "invoiceId is required", http.StatusBadRequest)
		return
	}
	gateway := strings.ToLower(strings.TrimSpace(req.Gateway))
	if gateway == "" {
		gateway = "manual"
	}

	// Lock order: server then store. Settlement and ledger posting commit under
	// the same critical section, so a paid invoice always has its balanced pair.
	state.mu.Lock()
	if state.Invoices == nil {
		state.Invoices = &InvoiceStore{next: 1}
	}
	store := state.Invoices
	store.mu.Lock()

	invoice, alreadyPaid, err := store.markPaidLocked(req.InvoiceID, gateway, req.GatewayReference)
	if err != nil {
		store.mu.Unlock()
		state.mu.Unlock()
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		writeAPIError(w, err.Error(), status)
		return
	}
	if !alreadyPaid {
		state.postInvoiceLedger(invoice)
	}
	store.mu.Unlock()
	state.mu.Unlock()

	if !alreadyPaid {
		state.AuditLogger.Append("admin", "billing.invoice.paid", "invoice", invoice.InvoiceNumber,
			fmt.Sprintf(`{"gateway":%q,"amountCents":%d}`, gateway, invoice.AmountCents), r.RemoteAddr)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"invoice":          invoice,
		"alreadyProcessed": alreadyPaid,
	})
}
