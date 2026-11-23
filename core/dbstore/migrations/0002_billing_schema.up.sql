-- 0002_billing_schema.up.sql
-- Native Billing Subsystem: catalog, clients, orders, services, coupons, taxes,
-- webhooks, domains, support tickets, affiliates, and knowledge base.

CREATE TABLE IF NOT EXISTS billing_products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    group_name VARCHAR(64) NOT NULL,
    slug VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    product_type VARCHAR(32) NOT NULL CHECK (product_type IN ('hosting', 'reseller', 'vps', 'domain', 'addon', 'license')),
    package_name VARCHAR(64) NOT NULL DEFAULT '',
    currency VARCHAR(8) NOT NULL DEFAULT 'IDR',
    monthly_cents BIGINT NOT NULL DEFAULT 0,
    quarterly_cents BIGINT NOT NULL DEFAULT 0,
    semiannual_cents BIGINT NOT NULL DEFAULT 0,
    annual_cents BIGINT NOT NULL DEFAULT 0,
    setup_cents BIGINT NOT NULL DEFAULT 0,
    auto_provision BOOLEAN NOT NULL DEFAULT true,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_billing_product_slug UNIQUE (tenant_id, slug)
);

CREATE TABLE IF NOT EXISTS billing_clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    full_name VARCHAR(128) NOT NULL,
    email VARCHAR(255) NOT NULL,
    company VARCHAR(255) NOT NULL DEFAULT '',
    country VARCHAR(8) NOT NULL DEFAULT 'ID',
    currency VARCHAR(8) NOT NULL DEFAULT 'IDR',
    client_group VARCHAR(64) NOT NULL DEFAULT 'standard',
    email_verified BOOLEAN NOT NULL DEFAULT false,
    credit_cents BIGINT NOT NULL DEFAULT 0,
    risk_score INT NOT NULL DEFAULT 0,
    custom_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    sub_accounts_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    gdpr_anonymized BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_billing_client_email UNIQUE (tenant_id, email)
);

CREATE TABLE IF NOT EXISTS billing_coupons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    code VARCHAR(64) NOT NULL,
    discount_type VARCHAR(16) NOT NULL CHECK (discount_type IN ('percent', 'fixed')),
    discount_value BIGINT NOT NULL CHECK (discount_value > 0),
    max_uses INT NOT NULL DEFAULT 0,
    used_count INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN NOT NULL DEFAULT true,
    CONSTRAINT uq_billing_coupon_code UNIQUE (tenant_id, code)
);

CREATE TABLE IF NOT EXISTS billing_tax_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    country VARCHAR(8) NOT NULL,
    name VARCHAR(64) NOT NULL,
    rate_bps INT NOT NULL CHECK (rate_bps >= 0 AND rate_bps <= 10000),
    rounding_mode VARCHAR(16) NOT NULL DEFAULT 'half_up' CHECK (rounding_mode IN ('half_up', 'floor', 'ceil')),
    CONSTRAINT uq_billing_tax_country UNIQUE (tenant_id, country)
);

CREATE TABLE IF NOT EXISTS billing_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    order_number VARCHAR(64) UNIQUE NOT NULL,
    client_id UUID NOT NULL REFERENCES billing_clients(id) ON DELETE RESTRICT,
    product_id UUID NOT NULL REFERENCES billing_products(id) ON DELETE RESTRICT,
    billing_cycle VARCHAR(16) NOT NULL,
    domain VARCHAR(255) NOT NULL DEFAULT '',
    username VARCHAR(32) NOT NULL DEFAULT '',
    coupon_code VARCHAR(64) NOT NULL DEFAULT '',
    subtotal_cents BIGINT NOT NULL,
    discount_cents BIGINT NOT NULL DEFAULT 0,
    tax_cents BIGINT NOT NULL DEFAULT 0,
    total_cents BIGINT NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT 'IDR',
    invoice_id UUID,
    status VARCHAR(32) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'fraud', 'cancelled')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS billing_services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES billing_clients(id) ON DELETE RESTRICT,
    order_id UUID NOT NULL REFERENCES billing_orders(id) ON DELETE RESTRICT,
    product_id UUID NOT NULL REFERENCES billing_products(id) ON DELETE RESTRICT,
    hosting_username VARCHAR(32) NOT NULL DEFAULT '',
    domain VARCHAR(255) NOT NULL DEFAULT '',
    billing_cycle VARCHAR(16) NOT NULL,
    recurring_cents BIGINT NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT 'IDR',
    next_due_date DATE NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'suspended', 'terminated', 'cancelled')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS billing_webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    gateway VARCHAR(32) NOT NULL,
    idempotency_key VARCHAR(160) NOT NULL,
    invoice_id UUID,
    event_type VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    payload_hash VARCHAR(64) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_webhook_idempotency UNIQUE (gateway, idempotency_key)
);

CREATE TABLE IF NOT EXISTS billing_tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    ticket_number VARCHAR(32) UNIQUE NOT NULL,
    client_id UUID NOT NULL REFERENCES billing_clients(id) ON DELETE CASCADE,
    department VARCHAR(64) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    priority VARCHAR(16) NOT NULL CHECK (priority IN ('low', 'medium', 'high', 'urgent')),
    status VARCHAR(24) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'answered', 'customer_reply', 'closed')),
    sla_due_at TIMESTAMP WITH TIME ZONE NOT NULL,
    replies_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS billing_affiliates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES billing_clients(id) ON DELETE CASCADE,
    referral_code VARCHAR(32) UNIQUE NOT NULL,
    commission_bps INT NOT NULL DEFAULT 1000,
    balance_cents BIGINT NOT NULL DEFAULT 0,
    withdrawn_cents BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
