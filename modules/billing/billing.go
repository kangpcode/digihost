// DigiPanel Native Billing Engine (modules/billing/billing.go)
// Conceptually compatible with WHMCS workflows without third-party dependencies.
// Rules: All monetary amounts stored strictly as integer minor units (cents),
// double-entry ledger invariant (debits == credits), idempotent payment settlement,
// prorata integer upgrades, anti-fraud scoring, GDPR export/anonymization, and
// autonomous hosting provisioning hooks. Zero emojis, zero mocks.

package billing

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	currencyRegex = regexp.MustCompile(`^[A-Z]{3}$`)
	slugRegex     = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{1,62}$`)
)

// Provisioner connects billing orders and automation cron to the hosting panel.
type Provisioner interface {
	CreateAccount(tenantID, username, domain, pkgName string) error
	SuspendAccount(username string) error
	UnsuspendAccount(username string) error
	TerminateAccount(username string) error
	ChangeAccountPackage(username, pkgName string) error
}

type Product struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenantId"`
	GroupName       string    `json:"groupName"`
	Slug            string    `json:"slug"`
	Name            string    `json:"name"`
	ProductType     string    `json:"productType"` // hosting, reseller, vps, domain, addon, license
	PackageName     string    `json:"packageName"`
	Currency        string    `json:"currency"`
	MonthlyCents    int64     `json:"monthlyCents"`
	QuarterlyCents  int64     `json:"quarterlyCents"`
	SemiannualCents int64     `json:"semiannualCents"`
	AnnualCents     int64     `json:"annualCents"`
	SetupCents      int64     `json:"setupCents"`
	AutoProvision   bool      `json:"autoProvision"`
	Active          bool      `json:"active"`
	CreatedAt       time.Time `json:"createdAt"`
}

type Coupon struct {
	Code          string     `json:"code"`
	DiscountType  string     `json:"discountType"` // percent (bps 1..10000) or fixed (cents)
	DiscountValue int64      `json:"discountValue"`
	MaxUses       int        `json:"maxUses"`
	UsedCount     int        `json:"usedCount"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	Active        bool       `json:"active"`
}

type TaxRule struct {
	Country      string `json:"country"`
	Name         string `json:"name"`
	RateBps      int    `json:"rateBps"`      // e.g. 1100 = 11.00%
	RoundingMode string `json:"roundingMode"` // half_up, floor, ceil
}

type Client struct {
	ID                string            `json:"id"`
	TenantID          string            `json:"tenantId"`
	FullName          string            `json:"fullName"`
	Email             string            `json:"email"`
	Company           string            `json:"company"`
	Country           string            `json:"country"`
	Currency          string            `json:"currency"`
	ClientGroup       string            `json:"clientGroup"`
	EmailVerified     bool              `json:"emailVerified"`
	VerificationToken string            `json:"-"`
	CreditCents       int64             `json:"creditCents"`
	RiskScore         int               `json:"riskScore"`
	CustomFields      map[string]string `json:"customFields,omitempty"`
	SubAccounts       []string          `json:"subAccounts,omitempty"`
	ReferredByCode    string            `json:"referredByCode,omitempty"`
	GDPRAnonymized    bool              `json:"gdprAnonymized"`
	CreatedAt         time.Time         `json:"createdAt"`
}

type Order struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenantId"`
	OrderNumber   string    `json:"orderNumber"`
	ClientID      string    `json:"clientId"`
	ProductID     string    `json:"productId"`
	BillingCycle  string    `json:"billingCycle"`
	Domain        string    `json:"domain"`
	Username      string    `json:"username"`
	CouponCode    string    `json:"couponCode,omitempty"`
	SubtotalCents int64     `json:"subtotalCents"`
	DiscountCents int64     `json:"discountCents"`
	TaxCents      int64     `json:"taxCents"`
	TotalCents    int64     `json:"totalCents"`
	Currency      string    `json:"currency"`
	InvoiceID     string    `json:"invoiceId"`
	RiskScore     int       `json:"riskScore"`
	RiskFlags     []string  `json:"riskFlags,omitempty"`
	Status        string    `json:"status"` // pending, active, fraud, cancelled
	CreatedAt     time.Time `json:"createdAt"`
}

type Service struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenantId"`
	ClientID        string    `json:"clientId"`
	OrderID         string    `json:"orderId"`
	ProductID       string    `json:"productId"`
	PackageName     string    `json:"packageName"`
	HostingUsername string    `json:"hostingUsername"`
	Domain          string    `json:"domain"`
	BillingCycle    string    `json:"billingCycle"`
	RecurringCents  int64     `json:"recurringCents"`
	Currency        string    `json:"currency"`
	NextDueDate     string    `json:"nextDueDate"`
	Status          string    `json:"status"` // pending, active, suspended, terminated, cancelled
	CreatedAt       time.Time `json:"createdAt"`
}

type Invoice struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenantId"`
	InvoiceNumber    string     `json:"invoiceNumber"`
	ClientID         string     `json:"clientId,omitempty"`
	ClientName       string     `json:"clientName"`
	ClientEmail      string     `json:"clientEmail"`
	ServiceID        string     `json:"serviceId,omitempty"`
	OrderID          string     `json:"orderId,omitempty"`
	SubtotalCents    int64      `json:"subtotalCents"`
	DiscountCents    int64      `json:"discountCents"`
	TaxCents         int64      `json:"taxCents"`
	LateFeeCents     int64      `json:"lateFeeCents"`
	AmountCents      int64      `json:"amountCents"`
	RefundedCents    int64      `json:"refundedCents"`
	Currency         string     `json:"currency"`
	Status           string     `json:"status"` // unpaid, paid, overdue, cancelled, refunded
	IssueDate        string     `json:"issueDate"`
	DueDate          string     `json:"dueDate"`
	Description      string     `json:"description"`
	Gateway          string     `json:"gateway,omitempty"`
	GatewayReference string     `json:"gatewayReference,omitempty"`
	PaidAt           *time.Time `json:"paidAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}

type LedgerEntry struct {
	ID          int64     `json:"id"`
	TxRef       string    `json:"txRef"`
	AccountCode string    `json:"accountCode"`
	AmountCents int64     `json:"amountCents"`
	Direction   string    `json:"direction"` // debit or credit
	Currency    string    `json:"currency"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type AutomationPolicy struct {
	GenerateRenewalDaysBefore int   `json:"generateRenewalDaysBefore"`
	LateFeeAfterDays          int   `json:"lateFeeAfterDays"`
	LateFeeBps                int   `json:"lateFeeBps"`
	AutoSuspendDaysOverdue    int   `json:"autoSuspendDaysOverdue"`
	AutoTerminateDaysOverdue  int   `json:"autoTerminateDaysOverdue"`
}

type Engine struct {
	mu            sync.RWMutex
	statePath     string
	provisioner   Provisioner
	products      map[string]*Product
	coupons       map[string]*Coupon
	taxRules      map[string]*TaxRule
	clients       map[string]*Client
	orders        map[string]*Order
	services      map[string]*Service
	invoices      map[string]*Invoice
	ledger        []LedgerEntry
	webhooksSeen  map[string]time.Time
	domains       map[string]*DomainRegistration
	tickets       map[string]*Ticket
	affiliates    map[string]*Affiliate
	kbArticles    map[string]*KBArticle
	policy        AutomationPolicy
	nextSeq       int64
	nextLedgerID  int64
	gatewaySecret map[string]string
}

func NewEngine(stateDir string, provisioner Provisioner) *Engine {
	var path string
	if strings.TrimSpace(stateDir) != "" {
		_ = os.MkdirAll(stateDir, 0750)
		path = filepath.Join(stateDir, "billing_state.json")
	}
	e := &Engine{
		statePath:    path,
		provisioner:  provisioner,
		products:     make(map[string]*Product),
		coupons:      make(map[string]*Coupon),
		taxRules:     make(map[string]*TaxRule),
		clients:      make(map[string]*Client),
		orders:       make(map[string]*Order),
		services:     make(map[string]*Service),
		invoices:     make(map[string]*Invoice),
		webhooksSeen: make(map[string]time.Time),
		domains:      make(map[string]*DomainRegistration),
		tickets:      make(map[string]*Ticket),
		affiliates:   make(map[string]*Affiliate),
		kbArticles:   make(map[string]*KBArticle),
		policy: AutomationPolicy{
			GenerateRenewalDaysBefore: 7,
			LateFeeAfterDays:          5,
			LateFeeBps:                500, // 5%
			AutoSuspendDaysOverdue:    7,
			AutoTerminateDaysOverdue:  30,
		},
		nextSeq:       1,
		nextLedgerID:  1,
		gatewaySecret: make(map[string]string),
	}
	e.taxRules["ID"] = &TaxRule{Country: "ID", Name: "PPN 11%", RateBps: 1100, RoundingMode: "half_up"}
	e.taxRules["SG"] = &TaxRule{Country: "SG", Name: "GST 9%", RateBps: 900, RoundingMode: "half_up"}
	e.taxRules["US"] = &TaxRule{Country: "US", Name: "Tax Exempt", RateBps: 0, RoundingMode: "half_up"}
	if path != "" {
		_ = e.load()
	}
	return e
}

func (e *Engine) SetProvisioner(p Provisioner) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.provisioner = p
}

func (e *Engine) SetGatewaySecret(gateway, secret string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.gatewaySecret[strings.ToLower(strings.TrimSpace(gateway))] = secret
}

func (e *Engine) nextID(prefix string) string {
	seq := e.nextSeq
	e.nextSeq++
	return fmt.Sprintf("%s-%d", prefix, seq)
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (e *Engine) CreateProduct(p Product) (*Product, error) {
	p.Slug = strings.ToLower(strings.TrimSpace(p.Slug))
	if !slugRegex.MatchString(p.Slug) {
		return nil, errors.New("product slug must be 2-63 lowercase alphanumeric or dash characters")
	}
	p.Name = strings.TrimSpace(p.Name)
	if len(p.Name) < 2 {
		return nil, errors.New("product name is required")
	}
	switch p.ProductType {
	case "hosting", "reseller", "vps", "domain", "addon", "license":
	default:
		return nil, errors.New("unsupported productType")
	}
	p.Currency = normalizeCurrency(p.Currency)
	if !currencyRegex.MatchString(p.Currency) {
		return nil, errors.New("invalid ISO 4217 currency")
	}
	if p.MonthlyCents < 0 || p.QuarterlyCents < 0 || p.SemiannualCents < 0 || p.AnnualCents < 0 || p.SetupCents < 0 {
		return nil, errors.New("product prices cannot be negative")
	}
	if p.GroupName == "" {
		p.GroupName = "hosting"
	}
	if p.TenantID == "" {
		p.TenantID = "default"
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	for _, existing := range e.products {
		if existing.Slug == p.Slug && existing.TenantID == p.TenantID {
			return nil, errors.New("product slug already exists")
		}
	}
	p.ID = e.nextID("prod")
	p.Active = true
	p.CreatedAt = time.Now().UTC()
	stored := p
	e.products[p.ID] = &stored
	_ = e.persistLocked()
	copied := stored
	return &copied, nil
}

func (e *Engine) ListProducts() []*Product {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Product, 0, len(e.products))
	for _, p := range e.products {
		c := *p
		out = append(out, &c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (e *Engine) UpsertCoupon(c Coupon) (*Coupon, error) {
	c.Code = strings.ToUpper(strings.TrimSpace(c.Code))
	if len(c.Code) < 3 || len(c.Code) > 32 {
		return nil, errors.New("coupon code must be 3-32 characters")
	}
	if c.DiscountType != "percent" && c.DiscountType != "fixed" {
		return nil, errors.New("discountType must be percent or fixed")
	}
	if c.DiscountValue <= 0 {
		return nil, errors.New("discountValue must be positive")
	}
	if c.DiscountType == "percent" && c.DiscountValue > 10000 {
		return nil, errors.New("percent discountValue is in basis points (max 10000)")
	}
	c.Active = true

	e.mu.Lock()
	defer e.mu.Unlock()
	stored := c
	e.coupons[c.Code] = &stored
	_ = e.persistLocked()
	copied := stored
	return &copied, nil
}

func (e *Engine) RegisterClient(c Client) (*Client, error) {
	c.FullName = strings.TrimSpace(c.FullName)
	if len(c.FullName) < 2 {
		return nil, errors.New("fullName is required")
	}
	addr, err := mail.ParseAddress(strings.TrimSpace(c.Email))
	if err != nil {
		return nil, errors.New("valid email is required")
	}
	c.Email = strings.ToLower(addr.Address)
	c.Currency = normalizeCurrency(c.Currency)
	if !currencyRegex.MatchString(c.Currency) {
		return nil, errors.New("invalid currency")
	}
	if c.Country == "" {
		c.Country = "ID"
	} else {
		c.Country = strings.ToUpper(strings.TrimSpace(c.Country))
	}
	if c.ClientGroup == "" {
		c.ClientGroup = "standard"
	}
	if c.TenantID == "" {
		c.TenantID = "default"
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	for _, existing := range e.clients {
		if existing.Email == c.Email && existing.TenantID == c.TenantID {
			return nil, errors.New("client email already registered")
		}
	}
	c.ID = e.nextID("cli")
	c.VerificationToken = randomHex(16)
	c.RiskScore, _ = evaluateRiskLocked(c.Email, c.Country, "")
	c.CreatedAt = time.Now().UTC()
	stored := c
	e.clients[c.ID] = &stored
	_ = e.persistLocked()
	copied := stored
	return &copied, nil
}

func (e *Engine) VerifyClientEmail(clientID, token string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	c, ok := e.clients[clientID]
	if !ok {
		return errors.New("client not found")
	}
	if c.VerificationToken != token || token == "" {
		return errors.New("invalid verification token")
	}
	c.EmailVerified = true
	c.VerificationToken = ""
	_ = e.persistLocked()
	return nil
}

func (e *Engine) ListClients() []*Client {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Client, 0, len(e.clients))
	for _, c := range e.clients {
		cp := *c
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (e *Engine) ExportClientGDPR(clientID string) (map[string]any, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	c, ok := e.clients[clientID]
	if !ok {
		return nil, errors.New("client not found")
	}
	var orders []*Order
	for _, o := range e.orders {
		if o.ClientID == clientID {
			cp := *o
			orders = append(orders, &cp)
		}
	}
	var invoices []*Invoice
	for _, inv := range e.invoices {
		if inv.ClientID == clientID {
			cp := *inv
			invoices = append(invoices, &cp)
		}
	}
	return map[string]any{
		"exportedAt": time.Now().UTC(),
		"client":     *c,
		"orders":     orders,
		"invoices":   invoices,
	}, nil
}

func (e *Engine) AnonymizeClientGDPR(clientID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	c, ok := e.clients[clientID]
	if !ok {
		return errors.New("client not found")
	}
	c.FullName = "Anonymized Client " + c.ID
	c.Email = c.ID + "@anonymized.invalid"
	c.Company = ""
	c.CustomFields = nil
	c.SubAccounts = nil
	c.GDPRAnonymized = true
	for _, inv := range e.invoices {
		if inv.ClientID == clientID {
			inv.ClientName = c.FullName
			inv.ClientEmail = c.Email
		}
	}
	_ = e.persistLocked()
	return nil
}

type PlaceOrderRequest struct {
	ClientID     string `json:"clientId"`
	ProductID    string `json:"productId"`
	BillingCycle string `json:"billingCycle"`
	Domain       string `json:"domain"`
	Username     string `json:"username"`
	CouponCode   string `json:"couponCode"`
	ClientIP     string `json:"clientIp"`
}

func (e *Engine) PlaceOrder(req PlaceOrderRequest) (*Order, *Invoice, *Service, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	client, ok := e.clients[req.ClientID]
	if !ok {
		return nil, nil, nil, errors.New("client not found")
	}
	product, ok := e.products[req.ProductID]
	if !ok || !product.Active {
		return nil, nil, nil, errors.New("product not found or inactive")
	}
	cyclePrice, months, err := cycleAmount(product, req.BillingCycle)
	if err != nil {
		return nil, nil, nil, err
	}
	subtotal := cyclePrice + product.SetupCents
	if subtotal <= 0 {
		return nil, nil, nil, errors.New("order subtotal must be positive")
	}

	var discount int64
	couponCode := strings.ToUpper(strings.TrimSpace(req.CouponCode))
	if couponCode != "" {
		coupon, ok := e.coupons[couponCode]
		if !ok || !coupon.Active {
			return nil, nil, nil, errors.New("coupon is invalid or inactive")
		}
		if coupon.ExpiresAt != nil && time.Now().UTC().After(*coupon.ExpiresAt) {
			return nil, nil, nil, errors.New("coupon has expired")
		}
		if coupon.MaxUses > 0 && coupon.UsedCount >= coupon.MaxUses {
			return nil, nil, nil, errors.New("coupon usage limit reached")
		}
		if coupon.DiscountType == "percent" {
			discount = applyBasisPoints(subtotal, int(coupon.DiscountValue), "half_up")
		} else {
			discount = coupon.DiscountValue
		}
		if discount > subtotal {
			discount = subtotal
		}
		coupon.UsedCount++
	}

	taxable := subtotal - discount
	var taxCents int64
	if rule, ok := e.taxRules[client.Country]; ok && rule.RateBps > 0 {
		taxCents = applyBasisPoints(taxable, rule.RateBps, rule.RoundingMode)
	}
	total := taxable + taxCents
	if total <= 0 {
		total = 1 // minimum 1 minor unit when fully discounted unless explicit free flow
	}

	riskScore, riskFlags := evaluateRiskLocked(client.Email, client.Country, req.Domain)
	orderStatus := "pending"
	if riskScore >= 75 {
		orderStatus = "fraud"
	}

	now := time.Now().UTC()
	orderID := e.nextID("ord")
	serviceID := e.nextID("svc")
	invSeq := e.nextSeq
	e.nextSeq++
	invoiceID := fmt.Sprintf("inv-%d", invSeq)
	invoiceNumber := fmt.Sprintf("INV-%d-%04d", now.Year(), invSeq)

	due := now.AddDate(0, months, 0)
	svc := &Service{
		ID:              serviceID,
		TenantID:        client.TenantID,
		ClientID:        client.ID,
		OrderID:         orderID,
		ProductID:       product.ID,
		PackageName:     product.PackageName,
		HostingUsername: strings.ToLower(strings.TrimSpace(req.Username)),
		Domain:          strings.ToLower(strings.TrimSpace(req.Domain)),
		BillingCycle:    req.BillingCycle,
		RecurringCents:  cyclePrice,
		Currency:        product.Currency,
		NextDueDate:     due.Format("2006-01-02"),
		Status:          "pending",
		CreatedAt:       now,
	}

	inv := &Invoice{
		ID:            invoiceID,
		TenantID:      client.TenantID,
		InvoiceNumber: invoiceNumber,
		ClientID:      client.ID,
		ClientName:    client.FullName,
		ClientEmail:   client.Email,
		ServiceID:     serviceID,
		OrderID:       orderID,
		SubtotalCents: subtotal,
		DiscountCents: discount,
		TaxCents:      taxCents,
		AmountCents:   total,
		Currency:      product.Currency,
		Status:        "unpaid",
		IssueDate:     now.Format("2006-01-02"),
		DueDate:       now.AddDate(0, 0, 14).Format("2006-01-02"),
		Description:   fmt.Sprintf("%s (%s) - %s", product.Name, req.BillingCycle, svc.Domain),
		CreatedAt:     now,
	}

	order := &Order{
		ID:            orderID,
		TenantID:      client.TenantID,
		OrderNumber:   fmt.Sprintf("ORD-%d-%04d", now.Year(), invSeq),
		ClientID:      client.ID,
		ProductID:     product.ID,
		BillingCycle:  req.BillingCycle,
		Domain:        svc.Domain,
		Username:      svc.HostingUsername,
		CouponCode:    couponCode,
		SubtotalCents: subtotal,
		DiscountCents: discount,
		TaxCents:      taxCents,
		TotalCents:    total,
		Currency:      product.Currency,
		InvoiceID:     invoiceID,
		RiskScore:     riskScore,
		RiskFlags:     riskFlags,
		Status:        orderStatus,
		CreatedAt:     now,
	}

	e.services[svc.ID] = svc
	e.invoices[inv.ID] = inv
	e.orders[order.ID] = order
	_ = e.persistLocked()

	oCopy, iCopy, sCopy := *order, *inv, *svc
	return &oCopy, &iCopy, &sCopy, nil
}

// SettleInvoice atomically marks an invoice paid, posts balanced double-entry
// ledger rows, credits affiliate commission if applicable, and provisions or
// unsuspends the linked hosting service.
func (e *Engine) SettleInvoice(invoiceID, gateway, reference, idempotencyKey string) (*Invoice, bool, error) {
	gateway = strings.ToLower(strings.TrimSpace(gateway))
	if gateway == "" {
		gateway = "manual"
	}

	e.mu.Lock()
	if idempotencyKey != "" {
		dedupKey := gateway + ":" + idempotencyKey
		if _, seen := e.webhooksSeen[dedupKey]; seen {
			inv := e.findInvoiceLocked(invoiceID)
			e.mu.Unlock()
			if inv != nil {
				cp := *inv
				return &cp, true, nil
			}
			return nil, true, nil
		}
	}

	inv := e.findInvoiceLocked(invoiceID)
	if inv == nil {
		e.mu.Unlock()
		return nil, false, errors.New("invoice not found")
	}
	if inv.Status == "paid" {
		cp := *inv
		e.mu.Unlock()
		return &cp, true, nil
	}
	if inv.Status == "cancelled" || inv.Status == "refunded" {
		e.mu.Unlock()
		return nil, false, errors.New("cancelled or refunded invoice cannot be paid")
	}

	now := time.Now().UTC()
	inv.Status = "paid"
	inv.Gateway = gateway
	inv.GatewayReference = strings.TrimSpace(reference)
	inv.PaidAt = &now
	if idempotencyKey != "" {
		e.webhooksSeen[gateway+":"+idempotencyKey] = now
	}

	e.postLedgerPairLocked(inv.InvoiceNumber,
		"1010-CASH-GATEWAY-"+strings.ToUpper(gateway),
		"4010-REVENUE-HOSTING-PLANS",
		inv.AmountCents, inv.Currency, "Invoice settlement "+inv.InvoiceNumber)

	// Accrue affiliate commission if client was referred
	if inv.ClientID != "" {
		if client, ok := e.clients[inv.ClientID]; ok && client.ReferredByCode != "" {
			if aff, ok := e.affiliates[client.ReferredByCode]; ok {
				comm := applyBasisPoints(inv.SubtotalCents-inv.DiscountCents, aff.CommissionBps, "floor")
				aff.BalanceCents += comm
			}
		}
	}

	// Provision or unsuspend linked service
	var svcCopy *Service
	var prodCopy *Product
	var orderRef *Order
	if inv.ServiceID != "" {
		if svc, ok := e.services[inv.ServiceID]; ok {
			if inv.OrderID != "" {
				orderRef = e.orders[inv.OrderID]
			}
			if orderRef == nil || orderRef.Status != "fraud" {
				if p, ok := e.products[svc.ProductID]; ok {
					cp := *p
					prodCopy = &cp
				}
				sc := *svc
				svcCopy = &sc
			}
		}
	}
	prov := e.provisioner
	_ = e.persistLocked()
	invCopy := *inv
	e.mu.Unlock()

	if svcCopy != nil && prodCopy != nil && prodCopy.AutoProvision && prov != nil && svcCopy.HostingUsername != "" {
		var provErr error
		if svcCopy.Status == "pending" && svcCopy.Domain != "" {
			provErr = prov.CreateAccount(svcCopy.TenantID, svcCopy.HostingUsername, svcCopy.Domain, svcCopy.PackageName)
		} else if svcCopy.Status == "suspended" {
			provErr = prov.UnsuspendAccount(svcCopy.HostingUsername)
		}
		if provErr == nil {
			e.mu.Lock()
			if liveSvc, ok := e.services[svcCopy.ID]; ok {
				liveSvc.Status = "active"
			}
			if orderRef != nil {
				if liveOrd, ok := e.orders[orderRef.ID]; ok {
					liveOrd.Status = "active"
				}
			}
			_ = e.persistLocked()
			e.mu.Unlock()
		}
	}

	return &invCopy, false, nil
}

// IssueRefund posts a balanced contra-revenue refund ledger pair and updates invoice status.
func (e *Engine) IssueRefund(invoiceID string, refundCents int64, reason string) (*Invoice, error) {
	if refundCents <= 0 {
		return nil, errors.New("refundCents must be positive")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	inv := e.findInvoiceLocked(invoiceID)
	if inv == nil {
		return nil, errors.New("invoice not found")
	}
	if inv.Status != "paid" {
		return nil, errors.New("only paid invoices can be refunded")
	}
	if inv.RefundedCents+refundCents > inv.AmountCents {
		return nil, errors.New("refund exceeds remaining paid amount")
	}
	inv.RefundedCents += refundCents
	if inv.RefundedCents == inv.AmountCents {
		inv.Status = "refunded"
	}
	txRef := inv.InvoiceNumber + "-RFND-" + fmt.Sprintf("%d", inv.RefundedCents)
	e.postLedgerPairLocked(txRef,
		"5010-REFUNDS-CONTRA",
		"1010-CASH-GATEWAY-"+strings.ToUpper(inv.Gateway),
		refundCents, inv.Currency, "Refund: "+strings.TrimSpace(reason))
	_ = e.persistLocked()
	cp := *inv
	return &cp, nil
}

// UpgradeServicePackage calculates integer prorata charge or credit and applies package change.
func (e *Engine) UpgradeServicePackage(serviceID, newProductID string, totalCycleDays, remainingDays int) (int64, *Invoice, error) {
	if totalCycleDays <= 0 || remainingDays < 0 || remainingDays > totalCycleDays {
		return 0, nil, errors.New("invalid prorata day window")
	}
	e.mu.Lock()
	svc, ok := e.services[serviceID]
	if !ok {
		e.mu.Unlock()
		return 0, nil, errors.New("service not found")
	}
	newProd, ok := e.products[newProductID]
	if !ok {
		e.mu.Unlock()
		return 0, nil, errors.New("target product not found")
	}
	newPrice, _, err := cycleAmount(newProd, svc.BillingCycle)
	if err != nil {
		e.mu.Unlock()
		return 0, nil, err
	}
	diff := newPrice - svc.RecurringCents
	prorataCents := (diff * int64(remainingDays)) / int64(totalCycleDays)
	prov := e.provisioner
	username := svc.HostingUsername
	targetPkg := newProd.PackageName

	var createdInv *Invoice
	if prorataCents > 0 {
		client := e.clients[svc.ClientID]
		now := time.Now().UTC()
		seq := e.nextSeq
		e.nextSeq++
		inv := &Invoice{
			ID:            fmt.Sprintf("inv-%d", seq),
			TenantID:      svc.TenantID,
			InvoiceNumber: fmt.Sprintf("INV-%d-%04d", now.Year(), seq),
			ClientID:      svc.ClientID,
			ClientName:    client.FullName,
			ClientEmail:   client.Email,
			ServiceID:     svc.ID,
			SubtotalCents: prorataCents,
			AmountCents:   prorataCents,
			Currency:      svc.Currency,
			Status:        "unpaid",
			IssueDate:     now.Format("2006-01-02"),
			DueDate:       now.AddDate(0, 0, 7).Format("2006-01-02"),
			Description:   fmt.Sprintf("Prorata upgrade %s -> %s (%d/%d days)", svc.PackageName, newProd.Name, remainingDays, totalCycleDays),
			CreatedAt:     now,
		}
		e.invoices[inv.ID] = inv
		cp := *inv
		createdInv = &cp
	} else if prorataCents < 0 {
		credit := -prorataCents
		if client, ok := e.clients[svc.ClientID]; ok {
			client.CreditCents += credit
		}
		e.postLedgerPairLocked("PRORATA-"+svc.ID, "5010-REFUNDS-CONTRA", "2200-CLIENT-CREDITS", credit, svc.Currency, "Prorata downgrade credit")
	}

	svc.ProductID = newProd.ID
	svc.PackageName = newProd.PackageName
	svc.RecurringCents = newPrice
	_ = e.persistLocked()
	e.mu.Unlock()

	if prov != nil && username != "" && targetPkg != "" {
		if err := prov.ChangeAccountPackage(username, targetPkg); err != nil {
			return prorataCents, createdInv, err
		}
	}
	return prorataCents, createdInv, nil
}

func (e *Engine) LedgerSummary() (entries []LedgerEntry, totalDebits, totalCredits int64, balanced bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	entries = make([]LedgerEntry, len(e.ledger))
	copy(entries, e.ledger)
	for _, entry := range entries {
		if entry.Direction == "debit" {
			totalDebits += entry.AmountCents
		} else if entry.Direction == "credit" {
			totalCredits += entry.AmountCents
		}
	}
	return entries, totalDebits, totalCredits, totalDebits == totalCredits
}

func (e *Engine) ListInvoices() []*Invoice {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Invoice, 0, len(e.invoices))
	for _, inv := range e.invoices {
		cp := *inv
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (e *Engine) ListOrders() []*Order {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Order, 0, len(e.orders))
	for _, o := range e.orders {
		cp := *o
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (e *Engine) ListServices() []*Service {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Service, 0, len(e.services))
	for _, s := range e.services {
		cp := *s
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (e *Engine) postLedgerPairLocked(txRef, debitAccount, creditAccount string, amountCents int64, currency, desc string) {
	now := time.Now().UTC()
	id1 := e.nextLedgerID
	id2 := id1 + 1
	e.nextLedgerID += 2
	e.ledger = append(e.ledger,
		LedgerEntry{ID: id1, TxRef: txRef, AccountCode: debitAccount, AmountCents: amountCents, Direction: "debit", Currency: currency, Description: desc, CreatedAt: now},
		LedgerEntry{ID: id2, TxRef: txRef, AccountCode: creditAccount, AmountCents: amountCents, Direction: "credit", Currency: currency, Description: desc, CreatedAt: now},
	)
}

func (e *Engine) findInvoiceLocked(id string) *Invoice {
	if inv, ok := e.invoices[id]; ok {
		return inv
	}
	for _, inv := range e.invoices {
		if inv.InvoiceNumber == id {
			return inv
		}
	}
	return nil
}

func normalizeCurrency(raw string) string {
	c := strings.ToUpper(strings.TrimSpace(raw))
	if c == "" {
		return "IDR"
	}
	return c
}

func cycleAmount(p *Product, cycle string) (int64, int, error) {
	switch strings.ToLower(strings.TrimSpace(cycle)) {
	case "monthly":
		return p.MonthlyCents, 1, nil
	case "quarterly":
		return p.QuarterlyCents, 3, nil
	case "semiannual":
		return p.SemiannualCents, 6, nil
	case "annual":
		return p.AnnualCents, 12, nil
	case "onetime":
		return p.MonthlyCents, 0, nil
	default:
		return 0, 0, fmt.Errorf("unsupported billingCycle %q", cycle)
	}
}

func applyBasisPoints(amount int64, bps int, rounding string) int64 {
	num := amount * int64(bps)
	switch rounding {
	case "floor":
		return num / 10000
	case "ceil":
		return (num + 9999) / 10000
	default: // half_up
		return (num + 5000) / 10000
	}
}

func evaluateRiskLocked(email, country, domain string) (int, []string) {
	score := 0
	var flags []string
	lowerEmail := strings.ToLower(email)
	for _, disposable := range []string{"mailinator.com", "tempmail", "10minutemail", "guerrillamail"} {
		if strings.Contains(lowerEmail, disposable) {
			score += 55
			flags = append(flags, "disposable_email")
			break
		}
	}
	if strings.TrimSpace(country) == "" {
		score += 15
		flags = append(flags, "missing_country")
	}
	if strings.Contains(strings.ToLower(domain), "phish") || strings.Contains(strings.ToLower(domain), "fraud") {
		score += 80
		flags = append(flags, "high_risk_domain")
	}
	if score > 100 {
		score = 100
	}
	return score, flags
}

type persistedBillingState struct {
	Products     map[string]*Product            `json:"products"`
	Coupons      map[string]*Coupon             `json:"coupons"`
	TaxRules     map[string]*TaxRule            `json:"taxRules"`
	Clients      map[string]*Client             `json:"clients"`
	Orders       map[string]*Order              `json:"orders"`
	Services     map[string]*Service            `json:"services"`
	Invoices     map[string]*Invoice            `json:"invoices"`
	Ledger       []LedgerEntry                  `json:"ledger"`
	WebhooksSeen map[string]time.Time           `json:"webhooksSeen"`
	Domains      map[string]*DomainRegistration `json:"domains"`
	Tickets      map[string]*Ticket             `json:"tickets"`
	Affiliates   map[string]*Affiliate          `json:"affiliates"`
	NextSeq      int64                          `json:"nextSeq"`
	NextLedgerID int64                          `json:"nextLedgerId"`
}

func (e *Engine) persistLocked() error {
	if e.statePath == "" {
		return nil
	}
	snap := persistedBillingState{
		Products:     e.products,
		Coupons:      e.coupons,
		TaxRules:     e.taxRules,
		Clients:      e.clients,
		Orders:       e.orders,
		Services:     e.services,
		Invoices:     e.invoices,
		Ledger:       e.ledger,
		WebhooksSeen: e.webhooksSeen,
		Domains:      e.domains,
		Tickets:      e.tickets,
		Affiliates:   e.affiliates,
		NextSeq:      e.nextSeq,
		NextLedgerID: e.nextLedgerID,
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(e.statePath, data, 0600)
}

func (e *Engine) load() error {
	data, err := os.ReadFile(e.statePath)
	if err != nil {
		return err
	}
	var snap persistedBillingState
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.Products != nil {
		e.products = snap.Products
	}
	if snap.Coupons != nil {
		e.coupons = snap.Coupons
	}
	if snap.TaxRules != nil {
		e.taxRules = snap.TaxRules
	}
	if snap.Clients != nil {
		e.clients = snap.Clients
	}
	if snap.Orders != nil {
		e.orders = snap.Orders
	}
	if snap.Services != nil {
		e.services = snap.Services
	}
	if snap.Invoices != nil {
		e.invoices = snap.Invoices
	}
	if snap.Ledger != nil {
		e.ledger = snap.Ledger
	}
	if snap.WebhooksSeen != nil {
		e.webhooksSeen = snap.WebhooksSeen
	}
	if snap.Domains != nil {
		e.domains = snap.Domains
	}
	if snap.Tickets != nil {
		e.tickets = snap.Tickets
	}
	if snap.Affiliates != nil {
		e.affiliates = snap.Affiliates
	}
	if snap.NextSeq > e.nextSeq {
		e.nextSeq = snap.NextSeq
	}
	if snap.NextLedgerID > e.nextLedgerID {
		e.nextLedgerID = snap.NextLedgerID
	}
	return nil
}
