// DigiPanel Payment Gateway Plugin Adapters (modules/billing/gateways.go)
// Implements cryptographically signed, idempotent webhook verification for
// Midtrans, Xendit, Stripe, PayPal, and manual bank transfers.
// Zero card data is ever accepted or persisted; only external references/tokens.

package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type WebhookEvent struct {
	Gateway        string `json:"gateway"`
	IdempotencyKey string `json:"idempotencyKey"`
	InvoiceID      string `json:"invoiceId"`
	Reference      string `json:"reference"`
	EventType      string `json:"eventType"`
	AmountCents    int64  `json:"amountCents"`
	Status         string `json:"status"` // paid, failed
}

type GatewayAdapter interface {
	Name() string
	VerifyWebhook(headers http.Header, body []byte, secret string) (WebhookEvent, error)
}

func resolveGatewayAdapter(name string) (GatewayAdapter, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "midtrans":
		return midtransAdapter{}, nil
	case "xendit":
		return xenditAdapter{}, nil
	case "stripe":
		return stripeAdapter{}, nil
	case "paypal":
		return paypalAdapter{}, nil
	default:
		return nil, fmt.Errorf("unsupported webhook gateway %q", name)
	}
}

type midtransAdapter struct{}

func (midtransAdapter) Name() string { return "midtrans" }

// Midtrans signature: SHA512(order_id + status_code + gross_amount + serverKey)
func (midtransAdapter) VerifyWebhook(headers http.Header, body []byte, secret string) (WebhookEvent, error) {
	if secret == "" {
		return WebhookEvent{}, errors.New("midtrans webhook secret not configured")
	}
	var payload struct {
		OrderID           string `json:"order_id"`
		StatusCode        string `json:"status_code"`
		GrossAmount       string `json:"gross_amount"`
		SignatureKey      string `json:"signature_key"`
		TransactionStatus string `json:"transaction_status"`
		TransactionID     string `json:"transaction_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookEvent{}, err
	}
	sum := sha512.Sum512([]byte(payload.OrderID + payload.StatusCode + payload.GrossAmount + secret))
	expected := hex.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(payload.SignatureKey)), []byte(expected)) != 1 {
		return WebhookEvent{}, errors.New("invalid midtrans webhook signature")
	}
	status := "failed"
	if payload.TransactionStatus == "settlement" || payload.TransactionStatus == "capture" {
		status = "paid"
	}
	return WebhookEvent{
		Gateway:        "midtrans",
		IdempotencyKey: payload.TransactionID + ":" + payload.TransactionStatus,
		InvoiceID:      payload.OrderID,
		Reference:      payload.TransactionID,
		EventType:      "payment." + payload.TransactionStatus,
		Status:         status,
	}, nil
}

type xenditAdapter struct{}

func (xenditAdapter) Name() string { return "xendit" }

func (xenditAdapter) VerifyWebhook(headers http.Header, body []byte, secret string) (WebhookEvent, error) {
	if secret == "" {
		return WebhookEvent{}, errors.New("xendit callback token not configured")
	}
	token := strings.TrimSpace(headers.Get("X-Callback-Token"))
	if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
		return WebhookEvent{}, errors.New("invalid xendit callback token")
	}
	var payload struct {
		ID         string `json:"id"`
		ExternalID string `json:"external_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookEvent{}, err
	}
	status := "failed"
	if strings.EqualFold(payload.Status, "PAID") || strings.EqualFold(payload.Status, "SETTLED") {
		status = "paid"
	}
	return WebhookEvent{
		Gateway:        "xendit",
		IdempotencyKey: payload.ID + ":" + payload.Status,
		InvoiceID:      payload.ExternalID,
		Reference:      payload.ID,
		EventType:      "invoice." + strings.ToLower(payload.Status),
		Status:         status,
	}, nil
}

type stripeAdapter struct{}

func (stripeAdapter) Name() string { return "stripe" }

// Stripe-Signature: t=<timestamp>,v1=<hmac_sha256(t + "." + body)>
func (stripeAdapter) VerifyWebhook(headers http.Header, body []byte, secret string) (WebhookEvent, error) {
	if secret == "" {
		return WebhookEvent{}, errors.New("stripe webhook secret not configured")
	}
	sigHeader := headers.Get("Stripe-Signature")
	var ts, v1 string
	for _, part := range strings.Split(sigHeader, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		if kv[0] == "t" {
			ts = kv[1]
		} else if kv[0] == "v1" {
			v1 = kv[1]
		}
	}
	if ts == "" || v1 == "" {
		return WebhookEvent{}, errors.New("missing Stripe-Signature timestamp or v1 signature")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(v1)), []byte(expected)) != 1 {
		return WebhookEvent{}, errors.New("invalid stripe webhook signature")
	}

	var payload struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID       string            `json:"id"`
				Metadata map[string]string `json:"metadata"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookEvent{}, err
	}
	status := "failed"
	if payload.Type == "invoice.paid" || payload.Type == "checkout.session.completed" || payload.Type == "payment_intent.succeeded" {
		status = "paid"
	}
	return WebhookEvent{
		Gateway:        "stripe",
		IdempotencyKey: payload.ID,
		InvoiceID:      payload.Data.Object.Metadata["invoiceId"],
		Reference:      payload.Data.Object.ID,
		EventType:      payload.Type,
		Status:         status,
	}, nil
}

type paypalAdapter struct{}

func (paypalAdapter) Name() string { return "paypal" }

func (paypalAdapter) VerifyWebhook(headers http.Header, body []byte, secret string) (WebhookEvent, error) {
	if secret == "" {
		return WebhookEvent{}, errors.New("paypal webhook secret not configured")
	}
	sig := strings.TrimSpace(headers.Get("X-Paypal-Transmission-Sig"))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(sig)), []byte(expected)) != 1 {
		return WebhookEvent{}, errors.New("invalid paypal webhook signature")
	}
	var payload struct {
		ID        string `json:"id"`
		EventType string `json:"event_type"`
		Resource  struct {
			ID        string `json:"id"`
			InvoiceID string `json:"invoice_id"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookEvent{}, err
	}
	status := "failed"
	if payload.EventType == "PAYMENT.CAPTURE.COMPLETED" {
		status = "paid"
	}
	return WebhookEvent{
		Gateway:        "paypal",
		IdempotencyKey: payload.ID,
		InvoiceID:      payload.Resource.InvoiceID,
		Reference:      payload.Resource.ID,
		EventType:      payload.EventType,
		Status:         status,
	}, nil
}

// ProcessWebhook verifies the gateway signature, enforces idempotency, and settles
// the matching invoice atomically.
func (e *Engine) ProcessWebhook(gateway string, headers http.Header, body []byte) (WebhookEvent, bool, error) {
	adapter, err := resolveGatewayAdapter(gateway)
	if err != nil {
		return WebhookEvent{}, false, err
	}
	e.mu.RLock()
	secret := e.gatewaySecret[adapter.Name()]
	e.mu.RUnlock()

	evt, err := adapter.VerifyWebhook(headers, body, secret)
	if err != nil {
		return WebhookEvent{}, false, err
	}
	if evt.InvoiceID == "" {
		return evt, false, errors.New("webhook payload missing invoice identifier")
	}
	if evt.Status != "paid" {
		return evt, false, nil
	}
	_, replayed, err := e.SettleInvoice(evt.InvoiceID, adapter.Name(), evt.Reference, evt.IdempotencyKey)
	return evt, replayed, err
}
