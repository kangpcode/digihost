// DigiPanel Native Billing REST API (core/billing_api.go)
// Exposes catalog, clients, orders, prorata upgrades, refunds, PDF invoices,
// signed webhooks, registrar domains, support tickets, reports, automation cron,
// GDPR export/anonymization, and WHMCS/Blesta migration importer.
// Zero emojis, strictly production grade.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"digipanel/modules/billing"
	"digipanel/modules/hosting"
)

type coreHostingProvisioner struct {
	mgr *hosting.Manager
}

func (p coreHostingProvisioner) CreateAccount(tenantID, username, domain, pkgName string) error {
	_, err := p.mgr.CreateAccount(hosting.CreateAccountRequest{
		TenantID: tenantID,
		Username: username,
		Domain:   domain,
		Package:  pkgName,
	})
	return err
}

func (p coreHostingProvisioner) SuspendAccount(username string) error {
	return p.mgr.SuspendAccount(username)
}

func (p coreHostingProvisioner) UnsuspendAccount(username string) error {
	return p.mgr.UnsuspendAccount(username)
}

func (p coreHostingProvisioner) TerminateAccount(username string) error {
	return p.mgr.TerminateAccount(username)
}

func (p coreHostingProvisioner) ChangeAccountPackage(username, pkgName string) error {
	_, err := p.mgr.ChangeAccountPackage(username, pkgName)
	return err
}

func (state *ServerState) billingEngine() *billing.Engine {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.Billing == nil {
		var prov billing.Provisioner
		if state.HostingManager != nil {
			prov = coreHostingProvisioner{mgr: state.HostingManager}
		}
		state.Billing = billing.NewEngine(state.StorageDir+"/billing", prov)
	}
	return state.Billing
}

func (state *ServerState) handleBillingProducts(w http.ResponseWriter, r *http.Request) {
	eng := state.billingEngine()
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(eng.ListProducts())
	case http.MethodPost:
		var req billing.Product
		if err := decodeJSONBody(w, r, &req); err != nil {
			writeAPIError(w, "Invalid product payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		created, err := eng.CreateProduct(req)
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.AuditLogger.Append(state.actorUsername(r), "billing.product.create", "product", created.ID,
			fmt.Sprintf(`{"slug":%q,"type":%q}`, created.Slug, created.ProductType), clientIP(r, state.TrustedProxies))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(created)
	default:
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (state *ServerState) handleBillingClients(w http.ResponseWriter, r *http.Request) {
	eng := state.billingEngine()
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(eng.ListClients())
	case http.MethodPost:
		var req billing.Client
		if err := decodeJSONBody(w, r, &req); err != nil {
			writeAPIError(w, "Invalid client payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		created, err := eng.RegisterClient(req)
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.AuditLogger.Append(state.actorUsername(r), "billing.client.create", "client", created.ID,
			fmt.Sprintf(`{"email":%q}`, created.Email), clientIP(r, state.TrustedProxies))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(created)
	default:
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (state *ServerState) handleBillingCoupons(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req billing.Coupon
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid coupon payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	created, err := state.billingEngine().UpsertCoupon(req)
	if err != nil {
		writeAPIError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (state *ServerState) handleBillingOrders(w http.ResponseWriter, r *http.Request) {
	eng := state.billingEngine()
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(eng.ListOrders())
	case http.MethodPost:
		var req billing.PlaceOrderRequest
		if err := decodeJSONBody(w, r, &req); err != nil {
			writeAPIError(w, "Invalid order payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		req.ClientIP = clientIP(r, state.TrustedProxies)
		order, inv, svc, err := eng.PlaceOrder(req)
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.AuditLogger.Append(state.actorUsername(r), "billing.order.create", "order", order.OrderNumber,
			fmt.Sprintf(`{"totalCents":%d,"riskScore":%d}`, order.TotalCents, order.RiskScore), req.ClientIP)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order":   order,
			"invoice": inv,
			"service": svc,
		})
	default:
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (state *ServerState) handleBillingServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state.billingEngine().ListServices())
}

func (state *ServerState) handleBillingServiceUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ServiceID      string `json:"serviceId"`
		NewProductID   string `json:"newProductId"`
		TotalCycleDays int    `json:"totalCycleDays"`
		RemainingDays  int    `json:"remainingDays"`
	}
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid upgrade payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	cents, inv, err := state.billingEngine().UpgradeServicePackage(req.ServiceID, req.NewProductID, req.TotalCycleDays, req.RemainingDays)
	if err != nil {
		writeAPIError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"prorataCents": cents,
		"invoice":      inv,
	})
}

func (state *ServerState) handleBillingRefund(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		InvoiceID   string `json:"invoiceId"`
		RefundCents int64  `json:"refundCents"`
		Reason      string `json:"reason"`
	}
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid refund payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	inv, err := state.billingEngine().IssueRefund(req.InvoiceID, req.RefundCents, req.Reason)
	if err != nil {
		writeAPIError(w, err.Error(), http.StatusBadRequest)
		return
	}
	state.AuditLogger.Append(state.actorUsername(r), "billing.invoice.refund", "invoice", inv.InvoiceNumber,
		fmt.Sprintf(`{"refundCents":%d}`, req.RefundCents), clientIP(r, state.TrustedProxies))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inv)
}

func (state *ServerState) handleBillingInvoicePDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeAPIError(w, "id is required", http.StatusBadRequest)
		return
	}
	pdfBytes, err := state.billingEngine().RenderInvoicePDF(id)
	if err != nil {
		writeAPIError(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", id+".pdf"))
	_, _ = w.Write(pdfBytes)
}

func (state *ServerState) handleBillingWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	gateway := strings.TrimPrefix(r.URL.Path, "/api/v1/billing/webhooks/")
	gateway = strings.Trim(gateway, "/")
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeAPIError(w, "Unable to read webhook payload", http.StatusBadRequest)
		return
	}
	evt, replayed, err := state.billingEngine().ProcessWebhook(gateway, r.Header, body)
	if err != nil {
		writeAPIError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	state.AuditLogger.Append("webhook:"+gateway, "billing.webhook.processed", "invoice", evt.InvoiceID,
		fmt.Sprintf(`{"eventType":%q,"replayed":%t}`, evt.EventType, replayed), clientIP(r, state.TrustedProxies))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"accepted": true,
		"replayed": replayed,
		"event":    evt,
	})
}

func (state *ServerState) handleBillingDomains(w http.ResponseWriter, r *http.Request) {
	eng := state.billingEngine()
	switch r.Method {
	case http.MethodGet:
		check := strings.TrimSpace(r.URL.Query().Get("check"))
		if check != "" {
			avail, err := eng.CheckDomainAvailability(check)
			if err != nil {
				writeAPIError(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"domain": check, "available": avail})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(eng.ListDomains())
	case http.MethodPost:
		var req struct {
			ClientID     string `json:"clientId"`
			Domain       string `json:"domain"`
			Registrar    string `json:"registrar"`
			Years        int    `json:"years"`
			WhoisPrivacy bool   `json:"whoisPrivacy"`
		}
		if err := decodeJSONBody(w, r, &req); err != nil {
			writeAPIError(w, "Invalid domain payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		reg, err := eng.RegisterDomain(req.ClientID, req.Domain, req.Registrar, req.Years, req.WhoisPrivacy)
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(reg)
	default:
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (state *ServerState) handleBillingTickets(w http.ResponseWriter, r *http.Request) {
	eng := state.billingEngine()
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(eng.ListTickets())
	case http.MethodPost:
		var req struct {
			ClientID   string `json:"clientId"`
			Department string `json:"department"`
			Subject    string `json:"subject"`
			Priority   string `json:"priority"`
			Message    string `json:"message"`
		}
		if err := decodeJSONBody(w, r, &req); err != nil {
			writeAPIError(w, "Invalid ticket payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		tkt, err := eng.OpenTicket(req.ClientID, req.Department, req.Subject, req.Priority, req.Message)
		if err != nil {
			writeAPIError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(tkt)
	default:
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (state *ServerState) handleBillingTicketReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TicketID string `json:"ticketId"`
		Message  string `json:"message"`
	}
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid reply payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	tkt, err := state.billingEngine().ReplyTicket(req.TicketID, state.actorUsername(r), true, req.Message)
	if err != nil {
		writeAPIError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tkt)
}

func (state *ServerState) handleBillingReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state.billingEngine().GenerateReport())
}

func (state *ServerState) handleBillingAutomationRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	summary := state.billingEngine().RunAutomationCycle(time.Now().UTC())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func (state *ServerState) handleBillingImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req billing.MigrationBundle
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid import payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	res, err := state.billingEngine().ImportMigrationBundle(req)
	if err != nil {
		writeAPIError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (state *ServerState) handleBillingGDPRExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("clientId"))
	out, err := state.billingEngine().ExportClientGDPR(id)
	if err != nil {
		writeAPIError(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (state *ServerState) handleBillingGDPRAnonymize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ClientID string `json:"clientId"`
	}
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeAPIError(w, "Invalid payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := state.billingEngine().AnonymizeClientGDPR(req.ClientID); err != nil {
		writeAPIError(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"anonymized": true})
}
