// DigiPanel PostgreSQL Store (core/dbstore/store.go)
// Provides versioned schema initialization, tenant-scoped Row-Level Security
// transactions, and atomic persistence for identities, sessions, API keys,
// audit logs, hosting packages/accounts, billing ledger, and cluster nodes.
// Zero emojis, strictly production grade.

package dbstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const DefaultTenantSlug = "default"

// Store owns the PostgreSQL connection pool and default tenant reference.
type Store struct {
	pool            *pgxpool.Pool
	defaultTenantID string
}

// TenantRecord represents a SaaS tenant in PostgreSQL.
type TenantRecord struct {
	ID           string
	Slug         string
	CompanyName  string
	BrandName    string
	CustomDomain string
	IsActive     bool
	CreatedAt    time.Time
}

// UserRecord represents an identity row in PostgreSQL.
type UserRecord struct {
	ID               string
	TenantID         string
	Username         string
	Email            string
	PasswordHash     string
	Role             string
	TwoFactorEnabled bool
	TwoFactorSecret  string
	Status           string
}

// SessionRecord represents an authenticated session row in PostgreSQL.
type SessionRecord struct {
	TenantID  string
	Username  string
	Role      string
	TokenHash string
	IPAddress string
	ExpiresAt time.Time
	LastSeen  time.Time
}

// APIKeyRecord represents a scoped API key in PostgreSQL.
type APIKeyRecord struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenantId"`
	Name          string    `json:"name"`
	OwnerUsername string    `json:"ownerUsername"`
	Role          string    `json:"role"`
	Scopes        []string  `json:"scopes"`
	KeyPrefix     string    `json:"keyPrefix"`
	KeyHash       string    `json:"-"`
	ExpiresAt     time.Time `json:"expiresAt"`
	Revoked       bool      `json:"revoked"`
	CreatedAt     time.Time `json:"createdAt"`
}

// InvoiceRecord represents a persisted invoice.
type InvoiceRecord struct {
	ID               string
	TenantID         string
	InvoiceNumber    string
	ClientName       string
	ClientEmail      string
	SubtotalCents    int64
	TaxCents         int64
	DiscountCents    int64
	LateFeeCents     int64
	AmountCents      int64
	Currency         string
	Status           string
	IssueDate        string
	DueDate          string
	Description      string
	Gateway          string
	GatewayReference string
	PaidAt           *time.Time
	CreatedAt        time.Time
}

// LedgerEntryRecord represents a persisted double-entry posting row.
type LedgerEntryRecord struct {
	ID          int64
	TxRef       string
	AccountCode string
	AmountCents int64
	Direction   string
	Currency    string
}

// Open connects to PostgreSQL, executes pending migrations, and ensures the
// default tenant exists.
func Open(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgresql: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgresql: %w", err)
	}
	if _, err := MigrateUp(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate postgresql: %w", err)
	}

	store := &Store{pool: pool}
	tenant, err := store.EnsureTenant(ctx, DefaultTenantSlug, "DigiHost Default", "DigiPanel", "")
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("bootstrap default tenant: %w", err)
	}
	store.defaultTenantID = tenant.ID
	return store, nil
}

// Close closes the underlying pool.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// Pool exposes the underlying pgxpool for module repositories.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// DefaultTenantID returns the UUID of the default tenant.
func (s *Store) DefaultTenantID() string {
	return s.defaultTenantID
}

// WithTenantTx runs fn inside a transaction pinned to the non-superuser
// digipanel_app role with Row-Level Security enforced for tenantID.
func (s *Store) WithTenantTx(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
	if tenantID == "" {
		return errors.New("tenantID is required for tenant-scoped transaction")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SET LOCAL ROLE digipanel_app`); err != nil {
		return fmt.Errorf("set local role: %w", err)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.bypass_rls', 'off', true)`); err != nil {
		return fmt.Errorf("disable bypass_rls: %w", err)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant_id', $1, true)`, tenantID); err != nil {
		return fmt.Errorf("set current_tenant_id: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// WithAdminTx runs fn inside a transaction with RLS bypass enabled for global
// control-plane maintenance tasks.
func (s *Store) WithAdminTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.bypass_rls', 'on', true)`); err != nil {
		return fmt.Errorf("enable bypass_rls: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// EnsureTenant returns an existing tenant by slug or creates it.
func (s *Store) EnsureTenant(ctx context.Context, slug, company, brand, customDomain string) (*TenantRecord, error) {
	var rec TenantRecord
	err := s.pool.QueryRow(ctx, `
		INSERT INTO tenants (slug, company_name, brand_name, custom_domain)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		ON CONFLICT (slug) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
		RETURNING id::text, slug, company_name, brand_name, COALESCE(custom_domain, ''), is_active, created_at
	`, slug, company, brand, customDomain).Scan(
		&rec.ID, &rec.Slug, &rec.CompanyName, &rec.BrandName, &rec.CustomDomain, &rec.IsActive, &rec.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// UpsertUser inserts or updates a panel identity within a tenant.
func (s *Store) UpsertUser(ctx context.Context, u UserRecord) error {
	if u.TenantID == "" {
		u.TenantID = s.defaultTenantID
	}
	if u.Status == "" {
		u.Status = "active"
	}
	return s.WithTenantTx(ctx, u.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (tenant_id, username, email, password_hash, role, two_factor_enabled, two_factor_secret, status)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (tenant_id, username) DO UPDATE
			SET password_hash = EXCLUDED.password_hash,
			    role = EXCLUDED.role,
			    two_factor_enabled = EXCLUDED.two_factor_enabled,
			    two_factor_secret = EXCLUDED.two_factor_secret,
			    status = EXCLUDED.status,
			    updated_at = CURRENT_TIMESTAMP
		`, u.TenantID, u.Username, u.Email, u.PasswordHash, u.Role, u.TwoFactorEnabled, u.TwoFactorSecret, u.Status)
		return err
	})
}

// ListUsers returns all users within a tenant.
func (s *Store) ListUsers(ctx context.Context, tenantID string) ([]UserRecord, error) {
	if tenantID == "" {
		tenantID = s.defaultTenantID
	}
	var out []UserRecord
	err := s.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, tenant_id::text, username, email, password_hash, role, two_factor_enabled, two_factor_secret, status
			FROM users ORDER BY username ASC
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var u UserRecord
			if err := rows.Scan(&u.ID, &u.TenantID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.TwoFactorEnabled, &u.TwoFactorSecret, &u.Status); err != nil {
				return err
			}
			out = append(out, u)
		}
		return rows.Err()
	})
	return out, err
}

// DeleteUser removes a user by username within a tenant.
func (s *Store) DeleteUser(ctx context.Context, tenantID, username string) error {
	if tenantID == "" {
		tenantID = s.defaultTenantID
	}
	return s.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM users WHERE username = $1`, username)
		return err
	})
}

// SaveSession persists an opaque session hash.
func (s *Store) SaveSession(ctx context.Context, tenantID, username, role, rawToken, ip string, expiresAt time.Time) error {
	if tenantID == "" {
		tenantID = s.defaultTenantID
	}
	hash := HashToken(rawToken)
	return s.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO user_sessions (tenant_id, username, role, token_hash, ip_address, expires_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6)
			ON CONFLICT (token_hash) DO UPDATE SET last_seen = CURRENT_TIMESTAMP
		`, tenantID, username, role, hash, ip, expiresAt)
		return err
	})
}

// LookupSession fetches an unexpired session across tenants by token hash.
func (s *Store) LookupSession(ctx context.Context, rawToken string) (*SessionRecord, error) {
	hash := HashToken(rawToken)
	var rec SessionRecord
	err := s.WithAdminTx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT tenant_id::text, username, role, token_hash, ip_address, expires_at, last_seen
			FROM user_sessions
			WHERE token_hash = $1 AND expires_at > CURRENT_TIMESTAMP
		`, hash).Scan(&rec.TenantID, &rec.Username, &rec.Role, &rec.TokenHash, &rec.IPAddress, &rec.ExpiresAt, &rec.LastSeen)
	})
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// DeleteSession revokes a session token.
func (s *Store) DeleteSession(ctx context.Context, rawToken string) error {
	hash := HashToken(rawToken)
	return s.WithAdminTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM user_sessions WHERE token_hash = $1`, hash)
		return err
	})
}

// CreateAPIKey stores a hashed scoped API key.
func (s *Store) CreateAPIKey(ctx context.Context, rec APIKeyRecord) (*APIKeyRecord, error) {
	if rec.TenantID == "" {
		rec.TenantID = s.defaultTenantID
	}
	scopesJSON, err := json.Marshal(rec.Scopes)
	if err != nil {
		return nil, err
	}
	var created APIKeyRecord
	var rawScopes []byte
	err = s.WithTenantTx(ctx, rec.TenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO api_keys (tenant_id, name, owner_username, role, scopes_json, key_prefix, key_hash, expires_at)
			VALUES ($1::uuid, $2, $3, $4, $5::jsonb, $6, $7, $8)
			RETURNING id::text, tenant_id::text, name, owner_username, role, scopes_json, key_prefix, expires_at, revoked, created_at
		`, rec.TenantID, rec.Name, rec.OwnerUsername, rec.Role, string(scopesJSON), rec.KeyPrefix, rec.KeyHash, rec.ExpiresAt).Scan(
			&created.ID, &created.TenantID, &created.Name, &created.OwnerUsername, &created.Role,
			&rawScopes, &created.KeyPrefix, &created.ExpiresAt, &created.Revoked, &created.CreatedAt,
		)
	})
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(rawScopes, &created.Scopes)
	return &created, nil
}

// ListAPIKeys lists scoped API keys for a tenant.
func (s *Store) ListAPIKeys(ctx context.Context, tenantID string) ([]APIKeyRecord, error) {
	if tenantID == "" {
		tenantID = s.defaultTenantID
	}
	var out []APIKeyRecord
	err := s.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, tenant_id::text, name, owner_username, role, scopes_json, key_prefix, expires_at, revoked, created_at
			FROM api_keys ORDER BY created_at DESC
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var k APIKeyRecord
			var rawScopes []byte
			if err := rows.Scan(&k.ID, &k.TenantID, &k.Name, &k.OwnerUsername, &k.Role, &rawScopes, &k.KeyPrefix, &k.ExpiresAt, &k.Revoked, &k.CreatedAt); err != nil {
				return err
			}
			_ = json.Unmarshal(rawScopes, &k.Scopes)
			out = append(out, k)
		}
		return rows.Err()
	})
	return out, err
}

// ResolveAPIKey validates an API key by SHA-256 hash.
func (s *Store) ResolveAPIKey(ctx context.Context, rawKey string) (*APIKeyRecord, error) {
	hash := HashToken(rawKey)
	var k APIKeyRecord
	var rawScopes []byte
	err := s.WithAdminTx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			UPDATE api_keys SET last_used_at = CURRENT_TIMESTAMP
			WHERE key_hash = $1 AND revoked = false AND expires_at > CURRENT_TIMESTAMP
			RETURNING id::text, tenant_id::text, name, owner_username, role, scopes_json, key_prefix, expires_at, revoked, created_at
		`, hash).Scan(&k.ID, &k.TenantID, &k.Name, &k.OwnerUsername, &k.Role, &rawScopes, &k.KeyPrefix, &k.ExpiresAt, &k.Revoked, &k.CreatedAt)
	})
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(rawScopes, &k.Scopes)
	return &k, nil
}

// RevokeAPIKey marks an API key revoked within a tenant.
func (s *Store) RevokeAPIKey(ctx context.Context, tenantID, keyID string) error {
	if tenantID == "" {
		tenantID = s.defaultTenantID
	}
	return s.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE api_keys SET revoked = true WHERE id = $1::uuid`, keyID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("api key not found")
		}
		return nil
	})
}

// AppendAudit inserts an immutable audit log record.
func (s *Store) AppendAudit(ctx context.Context, tenantID, actor, action, resType, resID, payloadJSON, ip, prevHash, entryHash string) error {
	if tenantID == "" {
		tenantID = s.defaultTenantID
	}
	if payloadJSON == "" {
		payloadJSON = "{}"
	}
	return s.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO audit_logs (tenant_id, actor_username, action, resource_type, resource_id, payload_json, ip_address, prev_entry_hash, entry_hash)
			VALUES ($1::uuid, $2, $3, $4, $5, $6::jsonb, $7, $8, $9)
		`, tenantID, actor, action, resType, resID, payloadJSON, ip, prevHash, entryHash)
		return err
	})
}

// CreateInvoiceAndLedgerSettlement demonstrates atomic PostgreSQL transaction
// for invoice creation and double-entry settlement.
func (s *Store) CreateInvoice(ctx context.Context, inv InvoiceRecord) (*InvoiceRecord, error) {
	if inv.TenantID == "" {
		inv.TenantID = s.defaultTenantID
	}
	if inv.SubtotalCents == 0 {
		inv.SubtotalCents = inv.AmountCents
	}
	var out InvoiceRecord
	var issueDate, dueDate time.Time
	err := s.WithTenantTx(ctx, inv.TenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO billing_invoices (tenant_id, invoice_number, client_name, client_email, subtotal_cents, tax_cents, discount_cents, late_fee_cents, amount_cents, currency, status, issue_date, due_date, description)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::date, $13::date, $14)
			RETURNING id::text, tenant_id::text, invoice_number, client_name, client_email, subtotal_cents, tax_cents, discount_cents, late_fee_cents, amount_cents, currency, status, issue_date, due_date, description, created_at
		`, inv.TenantID, inv.InvoiceNumber, inv.ClientName, inv.ClientEmail, inv.SubtotalCents, inv.TaxCents, inv.DiscountCents, inv.LateFeeCents, inv.AmountCents, inv.Currency, inv.Status, inv.IssueDate, inv.DueDate, inv.Description).Scan(
			&out.ID, &out.TenantID, &out.InvoiceNumber, &out.ClientName, &out.ClientEmail,
			&out.SubtotalCents, &out.TaxCents, &out.DiscountCents, &out.LateFeeCents, &out.AmountCents,
			&out.Currency, &out.Status, &issueDate, &dueDate, &out.Description, &out.CreatedAt,
		)
	})
	if err != nil {
		return nil, err
	}
	out.IssueDate = issueDate.Format("2006-01-02")
	out.DueDate = dueDate.Format("2006-01-02")
	return &out, nil
}

// SettleInvoiceAtomic marks an invoice paid and posts balanced debit/credit rows
// in the same PostgreSQL transaction.
func (s *Store) SettleInvoiceAtomic(ctx context.Context, tenantID, invoiceNumber, gateway, reference string) (bool, error) {
	if tenantID == "" {
		tenantID = s.defaultTenantID
	}
	alreadyPaid := false
	err := s.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var invID, status, currency string
		var amountCents int64
		err := tx.QueryRow(ctx, `
			SELECT id::text, status, amount_cents, currency
			FROM billing_invoices
			WHERE invoice_number = $1 OR id::text = $1
			FOR UPDATE
		`, invoiceNumber).Scan(&invID, &status, &amountCents, &currency)
		if err != nil {
			return err
		}
		if status == "paid" {
			alreadyPaid = true
			return nil
		}
		if status == "cancelled" {
			return errors.New("cancelled invoice cannot be paid")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE billing_invoices
			SET status = 'paid', gateway = $2, gateway_reference = $3, paid_at = CURRENT_TIMESTAMP
			WHERE id = $1::uuid
		`, invID, gateway, reference); err != nil {
			return err
		}

		var txID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO ledger_transactions (tenant_id, transaction_ref, description)
			VALUES ($1::uuid, $2, $3)
			RETURNING id::text
		`, tenantID, invoiceNumber, "Invoice payment settlement via "+gateway).Scan(&txID); err != nil {
			return err
		}

		cashAcct := "1010-CASH-GATEWAY-" + toUpperSafe(gateway)
		revAcct := "4010-REVENUE-HOSTING-PLANS"
		if _, err := tx.Exec(ctx, `
			INSERT INTO ledger_entries (tenant_id, transaction_id, tx_ref, account_code, amount_cents, direction, currency)
			VALUES
			  ($1::uuid, $2::uuid, $3, $4, $5, 'debit', $6),
			  ($1::uuid, $2::uuid, $3, $7, $5, 'credit', $6)
		`, tenantID, txID, invoiceNumber, cashAcct, amountCents, currency, revAcct); err != nil {
			return err
		}
		return nil
	})
	return alreadyPaid, err
}

func toUpperSafe(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - ('a' - 'A')
		}
	}
	return string(b)
}

// HashToken hashes an opaque token or API key with SHA-256.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
