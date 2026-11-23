-- 0003_cluster_saas_rls.up.sql
-- Multi-tenant SaaS subscriptions, white-label branding, cluster operations,
-- and mandatory Row-Level Security (RLS) policies on all tenant-scoped tables.

CREATE TABLE IF NOT EXISTS tenant_saas_subscriptions (
    tenant_id UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    plan_name VARCHAR(64) NOT NULL DEFAULT 'starter',
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('trial', 'active', 'suspended', 'cancelled')),
    max_accounts INT NOT NULL DEFAULT 50,
    max_nodes INT NOT NULL DEFAULT 1,
    features_json JSONB NOT NULL DEFAULT '["hosting","dns","ssl","backups"]'::jsonb,
    trial_ends_at TIMESTAMP WITH TIME ZONE,
    branding_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cluster_migrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    username VARCHAR(32) NOT NULL,
    source_node_id UUID NOT NULL,
    target_node_id UUID NOT NULL,
    archive_checksum VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN ('pending', 'transferring', 'verified', 'completed', 'rolled_back', 'failed')),
    detail TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'digipanel_app') THEN
        CREATE ROLE digipanel_app NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
    END IF;
END
$$;

GRANT USAGE ON SCHEMA public TO digipanel_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO digipanel_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO digipanel_app;

DO $$
DECLARE
    tbl TEXT;
    tenant_tables TEXT[] := ARRAY[
        'users',
        'user_sessions',
        'api_keys',
        'audit_logs',
        'hosting_packages',
        'hosting_accounts',
        'billing_invoices',
        'ledger_transactions',
        'ledger_entries',
        'cluster_nodes',
        'billing_products',
        'billing_clients',
        'billing_coupons',
        'billing_tax_rules',
        'billing_orders',
        'billing_services',
        'billing_webhook_events',
        'billing_tickets',
        'billing_affiliates',
        'tenant_saas_subscriptions',
        'cluster_migrations'
    ];
BEGIN
    FOREACH tbl IN ARRAY tenant_tables LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', tbl);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', tbl);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_policy ON %I', tbl);
        EXECUTE format(
            'CREATE POLICY tenant_isolation_policy ON %I
             USING (
                 current_setting(''app.bypass_rls'', true) = ''on''
                 OR tenant_id = NULLIF(current_setting(''app.current_tenant_id'', true), '''')::uuid
             )
             WITH CHECK (
                 current_setting(''app.bypass_rls'', true) = ''on''
                 OR tenant_id = NULLIF(current_setting(''app.current_tenant_id'', true), '''')::uuid
             )',
            tbl
        );
    END LOOP;
END
$$;
