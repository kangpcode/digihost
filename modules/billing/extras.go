// DigiPanel Billing Extras: Registrar Plugin, Support Tickets, Affiliates,
// Reports, PDF Generator, Automation Cron, and WHMCS/Blesta Importer
// (modules/billing/extras.go)

package billing

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// --- Domain Registrar Plugin ----------------------------------------------

type DomainRegistration struct {
	Domain       string    `json:"domain"`
	ClientID     string    `json:"clientId"`
	Registrar    string    `json:"registrar"`
	Status       string    `json:"status"` // active, pending_transfer, expired
	WhoisPrivacy bool      `json:"whoisPrivacy"`
	AutoRenew    bool      `json:"autoRenew"`
	EPPCode      string    `json:"eppCode"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

func (e *Engine) CheckDomainAvailability(domain string) (bool, error) {
	clean := strings.ToLower(strings.TrimSpace(domain))
	if !strings.Contains(clean, ".") || len(clean) < 4 {
		return false, errors.New("invalid domain name")
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, taken := e.domains[clean]
	return !taken, nil
}

func (e *Engine) RegisterDomain(clientID, domain, registrar string, years int, privacy bool) (*DomainRegistration, error) {
	if years <= 0 || years > 10 {
		years = 1
	}
	clean := strings.ToLower(strings.TrimSpace(domain))
	if !strings.Contains(clean, ".") {
		return nil, errors.New("invalid domain")
	}
	if registrar == "" {
		registrar = "native-epp"
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.clients[clientID]; !ok {
		return nil, errors.New("client not found")
	}
	if _, exists := e.domains[clean]; exists {
		return nil, errors.New("domain already registered")
	}
	reg := &DomainRegistration{
		Domain:       clean,
		ClientID:     clientID,
		Registrar:    registrar,
		Status:       "active",
		WhoisPrivacy: privacy,
		AutoRenew:    true,
		EPPCode:      "EPP-" + strings.ToUpper(randomHex(6)),
		ExpiresAt:    time.Now().UTC().AddDate(years, 0, 0),
	}
	e.domains[clean] = reg
	_ = e.persistLocked()
	cp := *reg
	return &cp, nil
}

func (e *Engine) RenewDomain(domain string, years int) (*DomainRegistration, error) {
	if years <= 0 {
		years = 1
	}
	clean := strings.ToLower(strings.TrimSpace(domain))
	e.mu.Lock()
	defer e.mu.Unlock()
	reg, ok := e.domains[clean]
	if !ok {
		return nil, errors.New("domain registration not found")
	}
	reg.ExpiresAt = reg.ExpiresAt.AddDate(years, 0, 0)
	reg.Status = "active"
	_ = e.persistLocked()
	cp := *reg
	return &cp, nil
}

func (e *Engine) ListDomains() []*DomainRegistration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*DomainRegistration, 0, len(e.domains))
	for _, d := range e.domains {
		cp := *d
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Domain < out[j].Domain })
	return out
}

// --- Support Tickets & KB -------------------------------------------------

type TicketReply struct {
	Author    string    `json:"author"`
	IsStaff   bool      `json:"isStaff"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type Ticket struct {
	ID           string        `json:"id"`
	TicketNumber string        `json:"ticketNumber"`
	ClientID     string        `json:"clientId"`
	Department   string        `json:"department"`
	Subject      string        `json:"subject"`
	Priority     string        `json:"priority"`
	Status       string        `json:"status"`
	SLADueAt     time.Time     `json:"slaDueAt"`
	Replies      []TicketReply `json:"replies"`
	CreatedAt    time.Time     `json:"createdAt"`
}

type KBArticle struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Title    string `json:"title"`
	Content  string `json:"content"`
}

func (e *Engine) OpenTicket(clientID, department, subject, priority, message string) (*Ticket, error) {
	if strings.TrimSpace(subject) == "" || strings.TrimSpace(message) == "" {
		return nil, errors.New("subject and message are required")
	}
	if department == "" {
		department = "support"
	}
	slaHours := 24
	switch strings.ToLower(priority) {
	case "urgent":
		slaHours = 2
	case "high":
		slaHours = 4
	case "medium":
		slaHours = 12
	default:
		priority = "low"
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.clients[clientID]; !ok {
		return nil, errors.New("client not found")
	}
	seq := e.nextSeq
	e.nextSeq++
	now := time.Now().UTC()
	t := &Ticket{
		ID:           fmt.Sprintf("tkt-%d", seq),
		TicketNumber: fmt.Sprintf("TKT-%05d", seq),
		ClientID:     clientID,
		Department:   department,
		Subject:      strings.TrimSpace(subject),
		Priority:     strings.ToLower(priority),
		Status:       "open",
		SLADueAt:     now.Add(time.Duration(slaHours) * time.Hour),
		Replies: []TicketReply{
			{Author: clientID, IsStaff: false, Message: strings.TrimSpace(message), CreatedAt: now},
		},
		CreatedAt: now,
	}
	e.tickets[t.ID] = t
	_ = e.persistLocked()
	cp := *t
	return &cp, nil
}

func (e *Engine) ReplyTicket(ticketID, author string, isStaff bool, message string) (*Ticket, error) {
	if strings.TrimSpace(message) == "" {
		return nil, errors.New("reply message is required")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	t, ok := e.tickets[ticketID]
	if !ok {
		return nil, errors.New("ticket not found")
	}
	t.Replies = append(t.Replies, TicketReply{
		Author:    author,
		IsStaff:   isStaff,
		Message:   strings.TrimSpace(message),
		CreatedAt: time.Now().UTC(),
	})
	if isStaff {
		t.Status = "answered"
	} else {
		t.Status = "customer_reply"
	}
	_ = e.persistLocked()
	cp := *t
	return &cp, nil
}

func (e *Engine) ListTickets() []*Ticket {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Ticket, 0, len(e.tickets))
	for _, t := range e.tickets {
		cp := *t
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// --- Affiliates -----------------------------------------------------------

type Affiliate struct {
	ClientID       string `json:"clientId"`
	ReferralCode   string `json:"referralCode"`
	CommissionBps  int    `json:"commissionBps"`
	BalanceCents   int64  `json:"balanceCents"`
	WithdrawnCents int64  `json:"withdrawnCents"`
}

func (e *Engine) EnrollAffiliate(clientID string, commissionBps int) (*Affiliate, error) {
	if commissionBps <= 0 || commissionBps > 5000 {
		commissionBps = 1000
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.clients[clientID]; !ok {
		return nil, errors.New("client not found")
	}
	for _, existing := range e.affiliates {
		if existing.ClientID == clientID {
			cp := *existing
			return &cp, nil
		}
	}
	code := "AFF-" + strings.ToUpper(randomHex(4))
	aff := &Affiliate{
		ClientID:      clientID,
		ReferralCode:  code,
		CommissionBps: commissionBps,
	}
	e.affiliates[code] = aff
	_ = e.persistLocked()
	cp := *aff
	return &cp, nil
}

// --- Reports & Automation Cron --------------------------------------------

type FinancialReport struct {
	Currency           string `json:"currency"`
	MRRCents           int64  `json:"mrrCents"`
	ARRCents           int64  `json:"arrCents"`
	CollectedCents     int64  `json:"collectedCents"`
	TaxCollectedCents  int64  `json:"taxCollectedCents"`
	OutstandingCents   int64  `json:"outstandingCents"`
	OverdueCents       int64  `json:"overdueCents"`
	ActiveServices     int    `json:"activeServices"`
	SuspendedServices  int    `json:"suspendedServices"`
	TerminatedServices int    `json:"terminatedServices"`
	ChurnRateBps       int    `json:"churnRateBps"`
	LTVCents           int64  `json:"ltvCents"`
}

func (e *Engine) GenerateReport() FinancialReport {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rep := FinancialReport{Currency: "IDR"}
	for _, s := range e.services {
		rep.Currency = s.Currency
		switch s.Status {
		case "active":
			rep.ActiveServices++
			rep.MRRCents += normalizeMonthlyCents(s.RecurringCents, s.BillingCycle)
		case "suspended":
			rep.SuspendedServices++
		case "terminated", "cancelled":
			rep.TerminatedServices++
		}
	}
	rep.ARRCents = rep.MRRCents * 12
	for _, inv := range e.invoices {
		if inv.Status == "paid" {
			rep.CollectedCents += inv.AmountCents - inv.RefundedCents
			rep.TaxCollectedCents += inv.TaxCents
		} else if inv.Status == "unpaid" || inv.Status == "overdue" {
			rep.OutstandingCents += inv.AmountCents
			if inv.Status == "overdue" {
				rep.OverdueCents += inv.AmountCents
			}
		}
	}
	totalSvcs := rep.ActiveServices + rep.SuspendedServices + rep.TerminatedServices
	if totalSvcs > 0 {
		rep.ChurnRateBps = (rep.TerminatedServices * 10000) / totalSvcs
	}
	if len(e.clients) > 0 {
		rep.LTVCents = rep.CollectedCents / int64(len(e.clients))
	}
	return rep
}

func normalizeMonthlyCents(amount int64, cycle string) int64 {
	switch cycle {
	case "quarterly":
		return amount / 3
	case "semiannual":
		return amount / 6
	case "annual":
		return amount / 12
	case "onetime":
		return 0
	default:
		return amount
	}
}

type AutomationSummary struct {
	RenewalsGenerated int `json:"renewalsGenerated"`
	LateFeesApplied   int `json:"lateFeesApplied"`
	ServicesSuspended int `json:"servicesSuspended"`
	ServicesTerminated int `json:"servicesTerminated"`
}

// RunAutomationCycle executes idempotent billing cron actions: renewal invoice
// generation, late fee application, auto-suspend, and auto-terminate.
func (e *Engine) RunAutomationCycle(now time.Time) AutomationSummary {
	var summary AutomationSummary
	type svcAction struct {
		username string
		action   string
	}
	var pendingActions []svcAction

	e.mu.Lock()
	pol := e.policy
	for _, svc := range e.services {
		due, err := time.Parse("2006-01-02", svc.NextDueDate)
		if err != nil {
			continue
		}
		// 1. Generate renewal invoice if within window and none unpaid exists for this due date
		if svc.Status == "active" && !due.After(now.AddDate(0, 0, pol.GenerateRenewalDaysBefore)) {
			hasOpen := false
			for _, inv := range e.invoices {
				if inv.ServiceID == svc.ID && (inv.Status == "unpaid" || inv.Status == "overdue") {
					hasOpen = true
					break
				}
			}
			if !hasOpen && svc.RecurringCents > 0 {
				client := e.clients[svc.ClientID]
				if client != nil {
					seq := e.nextSeq
					e.nextSeq++
					inv := &Invoice{
						ID:            fmt.Sprintf("inv-%d", seq),
						TenantID:      svc.TenantID,
						InvoiceNumber: fmt.Sprintf("INV-%d-%04d", now.Year(), seq),
						ClientID:      client.ID,
						ClientName:    client.FullName,
						ClientEmail:   client.Email,
						ServiceID:     svc.ID,
						SubtotalCents: svc.RecurringCents,
						AmountCents:   svc.RecurringCents,
						Currency:      svc.Currency,
						Status:        "unpaid",
						IssueDate:     now.Format("2006-01-02"),
						DueDate:       due.Format("2006-01-02"),
						Description:   fmt.Sprintf("Renewal %s (%s)", svc.Domain, svc.BillingCycle),
						CreatedAt:     now,
					}
					e.invoices[inv.ID] = inv
					summary.RenewalsGenerated++
				}
			}
		}
	}

	for _, inv := range e.invoices {
		if inv.Status != "unpaid" && inv.Status != "overdue" {
			continue
		}
		due, err := time.Parse("2006-01-02", inv.DueDate)
		if err != nil {
			continue
		}
		overdueDays := int(now.Sub(due).Hours() / 24)
		if overdueDays <= 0 {
			continue
		}
		inv.Status = "overdue"
		if overdueDays >= pol.LateFeeAfterDays && inv.LateFeeCents == 0 && pol.LateFeeBps > 0 {
			fee := applyBasisPoints(inv.AmountCents, pol.LateFeeBps, "half_up")
			if fee > 0 {
				inv.LateFeeCents = fee
				inv.AmountCents += fee
				summary.LateFeesApplied++
			}
		}
		if inv.ServiceID != "" {
			if svc, ok := e.services[inv.ServiceID]; ok {
				if overdueDays >= pol.AutoTerminateDaysOverdue && svc.Status != "terminated" {
					svc.Status = "terminated"
					summary.ServicesTerminated++
					if svc.HostingUsername != "" {
						pendingActions = append(pendingActions, svcAction{username: svc.HostingUsername, action: "terminate"})
					}
				} else if overdueDays >= pol.AutoSuspendDaysOverdue && svc.Status == "active" {
					svc.Status = "suspended"
					summary.ServicesSuspended++
					if svc.HostingUsername != "" {
						pendingActions = append(pendingActions, svcAction{username: svc.HostingUsername, action: "suspend"})
					}
				}
			}
		}
	}
	prov := e.provisioner
	_ = e.persistLocked()
	e.mu.Unlock()

	if prov != nil {
		for _, act := range pendingActions {
			switch act.action {
			case "suspend":
				_ = prov.SuspendAccount(act.username)
			case "terminate":
				_ = prov.TerminateAccount(act.username)
			}
		}
	}
	return summary
}

// --- Pure-Go PDF Invoice Renderer -----------------------------------------

// RenderInvoicePDF generates a valid PDF 1.4 document for an invoice without
// external dependencies.
func (e *Engine) RenderInvoicePDF(invoiceID string) ([]byte, error) {
	e.mu.RLock()
	inv := e.findInvoiceLocked(invoiceID)
	var cp Invoice
	if inv != nil {
		cp = *inv
	}
	e.mu.RUnlock()
	if inv == nil {
		return nil, errors.New("invoice not found")
	}

	esc := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `(`, `\(`)
		s = strings.ReplaceAll(s, `)`, `\)`)
		return s
	}

	stream := fmt.Sprintf(
		"BT /F1 14 Tf 50 760 Td (DigiPanel Billing Invoice: %s) Tj 0 -24 Td /F1 10 Tf "+
			"(Client: %s <%s>) Tj 0 -16 Td "+
			"(Status: %s | Issue: %s | Due: %s) Tj 0 -16 Td "+
			"(Description: %s) Tj 0 -16 Td "+
			"(Subtotal: %d | Discount: %d | Tax: %d | LateFee: %d) Tj 0 -18 Td "+
			"(Total Due: %d minor units %s) Tj ET",
		esc(cp.InvoiceNumber),
		esc(cp.ClientName), esc(cp.ClientEmail),
		esc(strings.ToUpper(cp.Status)), esc(cp.IssueDate), esc(cp.DueDate),
		esc(cp.Description),
		cp.SubtotalCents, cp.DiscountCents, cp.TaxCents, cp.LateFeeCents,
		cp.AmountCents, esc(cp.Currency),
	)

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, 6)
	writeObj := func(idx int, body string) {
		offsets[idx] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", idx, body)
	}
	writeObj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObj(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObj(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>")
	writeObj(4, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	writeObj(5, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

	xrefPos := buf.Len()
	buf.WriteString("xref\n0 6\n0000000000 65535 f \n")
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefPos)
	return buf.Bytes(), nil
}

// --- WHMCS / Blesta Migration Importer ------------------------------------

type MigrationBundle struct {
	SourceSystem string    `json:"sourceSystem"` // whmcs or blesta
	DryRun       bool      `json:"dryRun"`
	Clients      []Client  `json:"clients"`
	Products     []Product `json:"products"`
	Invoices     []Invoice `json:"invoices"`
}

type MigrationResult struct {
	SourceSystem     string `json:"sourceSystem"`
	DryRun           bool   `json:"dryRun"`
	ClientsImported  int    `json:"clientsImported"`
	ProductsImported int    `json:"productsImported"`
	InvoicesImported int    `json:"invoicesImported"`
	Skipped          int    `json:"skipped"`
}

func (e *Engine) ImportMigrationBundle(bundle MigrationBundle) (MigrationResult, error) {
	src := strings.ToLower(strings.TrimSpace(bundle.SourceSystem))
	if src != "whmcs" && src != "blesta" {
		return MigrationResult{}, errors.New("sourceSystem must be whmcs or blesta")
	}
	res := MigrationResult{SourceSystem: src, DryRun: bundle.DryRun}

	if bundle.DryRun {
		res.ClientsImported = len(bundle.Clients)
		res.ProductsImported = len(bundle.Products)
		res.InvoicesImported = len(bundle.Invoices)
		return res, nil
	}

	for _, p := range bundle.Products {
		if _, err := e.CreateProduct(p); err != nil {
			res.Skipped++
		} else {
			res.ProductsImported++
		}
	}
	for _, c := range bundle.Clients {
		if _, err := e.RegisterClient(c); err != nil {
			res.Skipped++
		} else {
			res.ClientsImported++
		}
	}
	e.mu.Lock()
	for _, inv := range bundle.Invoices {
		if inv.InvoiceNumber == "" || inv.AmountCents <= 0 {
			res.Skipped++
			continue
		}
		if e.findInvoiceLocked(inv.InvoiceNumber) != nil {
			res.Skipped++
			continue
		}
		seq := e.nextSeq
		e.nextSeq++
		inv.ID = fmt.Sprintf("inv-%d", seq)
		inv.CreatedAt = time.Now().UTC()
		stored := inv
		e.invoices[inv.ID] = &stored
		res.InvoicesImported++
	}
	_ = e.persistLocked()
	e.mu.Unlock()
	return res, nil
}
