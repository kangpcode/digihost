-- 0001_core_schema.up.sql
-- DigiPanel Core Foundation Schema (PostgreSQL 16+)
-- Integer minor currency units, hash-chained audit logs, RBAC, API keys, and hosting accounts.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(64) UNIQUE NOT NULL,
    company_name VARCHAR(255) NOT NULL,
    brand_name VARCHAR(128) NOT NULL DEFAULT 'DigiPanel',
    custom_domain VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    username VARCHAR(64) NOT NULL,
    email VARCHAR(255) NOT NULL DEFAULT '',
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(32) NOT NULL CHECK (role IN ('admin', 'reseller', 'user', 'staff')),
    two_factor_enabled BOOLEAN NOT NULL DEFAULT false,
    two_factor_secret TEXT NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'pending')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_username UNIQUE (tenant_id, username)
);

CREATE TABLE IF NOT EXISTS user_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    username VARCHAR(64) NOT NULL,
    role VARCHAR(32) NOT NULL CHECK (role IN ('admin', 'reseller', 'user', 'staff')),
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    ip_address TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    last_seen TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    owner_username VARCHAR(64) NOT NULL,
    role VARCHAR(32) NOT NULL CHECK (role IN ('admin', 'reseller', 'user', 'staff')),
    scopes_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    key_prefix VARCHAR(16) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT false,
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    actor_username VARCHAR(64) NOT NULL,
    action VARCHAR(128) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(128) NOT NULL,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address TEXT NOT NULL,
    prev_entry_hash VARCHAR(64) NOT NULL,
    entry_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_created ON audit_logs(tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS hosting_packages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(64) NOT NULL,
    disk_limit_bytes BIGINT NOT NULL,
    bandwidth_limit_bytes BIGINT NOT NULL,
    inodes_limit INT NOT NULL DEFAULT 100000,
    max_domains INT NOT NULL DEFAULT 1,
    max_databases INT NOT NULL DEFAULT 5,
    max_email_accounts INT NOT NULL DEFAULT 10,
    default_php_version VARCHAR(16) NOT NULL DEFAULT '8.3',
    memory_limit_mb INT NOT NULL DEFAULT 256,
    pids_limit INT NOT NULL DEFAULT 50,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_package_name UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS hosting_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    username VARCHAR(32) UNIQUE NOT NULL,
    primary_domain VARCHAR(255) UNIQUE NOT NULL,
    package_name VARCHAR(64) NOT NULL,
    posix_uid INT UNIQUE NOT NULL,
    posix_gid INT NOT NULL,
    home_dir VARCHAR(512) NOT NULL,
    document_root VARCHAR(512) NOT NULL,
    php_version VARCHAR(16) NOT NULL DEFAULT '8.3',
    disk_limit_bytes BIGINT NOT NULL,
    disk_used_bytes BIGINT NOT NULL DEFAULT 0,
    bandwidth_limit_bytes BIGINT NOT NULL,
    bandwidth_used_bytes BIGINT NOT NULL DEFAULT 0,
    inodes_limit INT NOT NULL,
    inodes_used INT NOT NULL DEFAULT 0,
    cgroups_path VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'terminated')),
    ip_address TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ledger_accounts (
    code VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(32) NOT NULL CHECK (type IN ('asset', 'liability', 'equity', 'revenue', 'expense'))
);

INSERT INTO ledger_accounts (code, name, type) VALUES
('1010-CASH-GATEWAY-MANUAL', 'Manual Settlement Account', 'asset'),
('1010-CASH-GATEWAY-BANK_TRANSFER', 'Bank Transfer Settlement Account', 'asset'),
('1010-CASH-GATEWAY-MIDTRANS', 'Midtrans Gateway Settlement Account', 'asset'),
('1010-CASH-GATEWAY-XENDIT', 'Xendit Gateway Settlement Account', 'asset'),
('1010-CASH-GATEWAY-STRIPE', 'Stripe Gateway Settlement Account', 'asset'),
('1010-CASH-GATEWAY-PAYPAL', 'PayPal Gateway Settlement Account', 'asset'),
('1200-ACCOUNTS-RECEIVABLE', 'Accounts Receivable Client Invoices', 'asset'),
('2010-ACCOUNTS-PAYABLE', 'Accounts Payable Upstream Infrastructure', 'liability'),
('2200-CLIENT-CREDITS', 'Client Prepaid Credit Liability', 'liability'),
('4010-REVENUE-HOSTING-PLANS', 'Hosting Services Recurring Revenue', 'revenue'),
('4020-REVENUE-DOMAINS', 'Domain Registration and Renewals', 'revenue'),
('5010-REFUNDS-CONTRA', 'Refunds and Credit Adjustments', 'expense')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE IF NOT EXISTS billing_invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    invoice_number VARCHAR(64) UNIQUE NOT NULL,
    client_id UUID,
    client_name VARCHAR(255) NOT NULL,
    client_email VARCHAR(255) NOT NULL,
    subtotal_cents BIGINT NOT NULL DEFAULT 0,
    tax_cents BIGINT NOT NULL DEFAULT 0,
    discount_cents BIGINT NOT NULL DEFAULT 0,
    late_fee_cents BIGINT NOT NULL DEFAULT 0,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    currency VARCHAR(8) NOT NULL DEFAULT 'IDR',
    status VARCHAR(32) NOT NULL DEFAULT 'unpaid' CHECK (status IN ('unpaid', 'paid', 'overdue', 'cancelled', 'refunded')),
    issue_date DATE NOT NULL,
    due_date DATE NOT NULL,
    description TEXT NOT NULL,
    gateway VARCHAR(32) NOT NULL DEFAULT '',
    gateway_reference VARCHAR(160) NOT NULL DEFAULT '',
    paid_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ledger_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    transaction_ref VARCHAR(64) NOT NULL,
    description TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_tx_ref UNIQUE (tenant_id, transaction_ref)
);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    transaction_id UUID NOT NULL REFERENCES ledger_transactions(id) ON DELETE CASCADE,
    tx_ref VARCHAR(64) NOT NULL,
    account_code VARCHAR(64) NOT NULL REFERENCES ledger_accounts(code),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    direction VARCHAR(8) NOT NULL CHECK (direction IN ('debit', 'credit')),
    currency VARCHAR(8) NOT NULL DEFAULT 'IDR',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cluster_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    node_name VARCHAR(128) NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    endpoint_url VARCHAR(512) NOT NULL DEFAULT '',
    ip_address TEXT NOT NULL,
    role VARCHAR(32) NOT NULL CHECK (role IN ('hub', 'edge', 'database')),
    status VARCHAR(32) NOT NULL DEFAULT 'online' CHECK (status IN ('online', 'degraded', 'offline')),
    panel_version VARCHAR(32) NOT NULL DEFAULT '1.0.0',
    mtls_cert_fingerprint VARCHAR(64) NOT NULL,
    services_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_poll_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_node_name UNIQUE (tenant_id, node_name)
);

CREATE TABLE IF NOT EXISTS system_modules (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    version VARCHAR(32) NOT NULL,
    is_core BOOLEAN NOT NULL DEFAULT false,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    manifest_json JSONB NOT NULL,
    installed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
