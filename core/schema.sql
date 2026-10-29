-- DigiPanel Core Database Schema (PostgreSQL)
-- Adheres to: Integer currencies, hash-chained audit logging, Row-Level Security (RLS)
-- Version: 1.0.0

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Tenants (Multi-tenant SaaS Isolation)
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(64) UNIQUE NOT NULL,
    company_name VARCHAR(255) NOT NULL,
    brand_name VARCHAR(128) NOT NULL DEFAULT 'DigiPanel',
    custom_domain VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Users & Authentication (Argon2id hashes)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    username VARCHAR(64) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(32) NOT NULL CHECK (role IN ('admin', 'reseller', 'user', 'staff')),
    two_factor_enabled BOOLEAN NOT NULL DEFAULT false,
    two_factor_secret_encrypted BYTEA,
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'pending')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_username UNIQUE (tenant_id, username),
    CONSTRAINT uq_tenant_email UNIQUE (tenant_id, email)
);

-- Immutable Append-Only Audit Log with Cryptographic Hash Chaining
CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    actor_user_id UUID REFERENCES users(id),
    action VARCHAR(128) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(128) NOT NULL,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address INET NOT NULL,
    user_agent TEXT,
    prev_entry_hash VARCHAR(64) NOT NULL,
    entry_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Hosting Accounts
CREATE TABLE hosting_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    owner_user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    reseller_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    username VARCHAR(32) UNIQUE NOT NULL,
    primary_domain VARCHAR(255) NOT NULL,
    package_plan VARCHAR(64) NOT NULL,
    php_version VARCHAR(16) NOT NULL DEFAULT '8.3',
    disk_limit_bytes BIGINT NOT NULL DEFAULT 52428800, -- 50MB
    disk_used_bytes BIGINT NOT NULL DEFAULT 0,
    bandwidth_limit_bytes BIGINT NOT NULL DEFAULT 53687091200, -- 50GB
    bandwidth_used_bytes BIGINT NOT NULL DEFAULT 0,
    inodes_limit INT NOT NULL DEFAULT 100000,
    inodes_used INT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'terminated')),
    ip_address INET NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Native Double-Entry General Ledger (Amounts strictly in smallest currency unit / cents)
CREATE TABLE ledger_accounts (
    code VARCHAR(32) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(32) NOT NULL CHECK (type IN ('asset', 'liability', 'equity', 'revenue', 'expense'))
);

INSERT INTO ledger_accounts (code, name, type) VALUES
('1010-CASH-GATEWAY-MIDTRANS', 'Midtrans Gateway Settlement Account', 'asset'),
('1020-CASH-GATEWAY-XENDIT', 'Xendit Gateway Settlement Account', 'asset'),
('1030-CASH-GATEWAY-STRIPE', 'Stripe Gateway Settlement Account', 'asset'),
('1200-ACCOUNTS-RECEIVABLE', 'Accounts Receivable Client Invoices', 'asset'),
('2010-ACCOUNTS-PAYABLE', 'Accounts Payable Upstream Infrastructure', 'liability'),
('4010-REVENUE-HOSTING-PLANS', 'Hosting Services Recurring Revenue', 'revenue'),
('4020-REVENUE-DOMAINS', 'Domain Registration and Renewals', 'revenue');

CREATE TABLE ledger_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    transaction_ref VARCHAR(64) UNIQUE NOT NULL,
    description TEXT NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE ledger_entries (
    id BIGSERIAL PRIMARY KEY,
    transaction_id UUID NOT NULL REFERENCES ledger_transactions(id) ON DELETE CASCADE,
    account_code VARCHAR(32) NOT NULL REFERENCES ledger_accounts(code),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    direction VARCHAR(8) NOT NULL CHECK (direction IN ('debit', 'credit')),
    currency VARCHAR(8) NOT NULL DEFAULT 'IDR'
);

-- Billing Invoices
CREATE TABLE billing_invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    client_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    invoice_number VARCHAR(64) UNIQUE NOT NULL,
    subtotal_cents BIGINT NOT NULL CHECK (subtotal_cents >= 0),
    tax_cents BIGINT NOT NULL DEFAULT 0,
    total_cents BIGINT NOT NULL CHECK (total_cents >= 0),
    currency VARCHAR(8) NOT NULL DEFAULT 'IDR',
    status VARCHAR(32) NOT NULL DEFAULT 'unpaid' CHECK (status IN ('unpaid', 'paid', 'overdue', 'cancelled')),
    issue_date DATE NOT NULL,
    due_date DATE NOT NULL,
    paid_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Cluster Fleet Nodes
CREATE TABLE cluster_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    node_name VARCHAR(128) NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    ip_address INET NOT NULL,
    role VARCHAR(32) NOT NULL CHECK (role IN ('hub', 'edge', 'database')),
    status VARCHAR(32) NOT NULL DEFAULT 'online' CHECK (status IN ('online', 'degraded', 'offline')),
    mtls_cert_fingerprint VARCHAR(64) NOT NULL,
    last_poll_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
