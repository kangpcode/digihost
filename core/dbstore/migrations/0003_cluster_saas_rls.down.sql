-- 0003_cluster_saas_rls.down.sql
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
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_policy ON %I', tbl);
        EXECUTE format('ALTER TABLE %I NO FORCE ROW LEVEL SECURITY', tbl);
        EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', tbl);
    END LOOP;
END
$$;

DROP TABLE IF EXISTS cluster_migrations;
DROP TABLE IF EXISTS tenant_saas_subscriptions;
