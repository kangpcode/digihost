// DigiPanel Native Billing view.
// Invoices are created through the core billing API with integer minor units.
// Payments settle atomically and post a balanced debit/credit pair into the
// ledger. Nothing on this screen is pre-filled with sample revenue.

import React, { useEffect, useMemo, useState } from 'react';
import { FileText, Download, BookOpen, RefreshCw, Plus, CheckCircle2, AlertTriangle, CreditCard } from 'lucide-react';
import type { Invoice, LedgerResponse, Language } from '../types';
import { getTranslation } from '../i18n/translations';
import { apiFetch } from '../api/client';
import { OpsEmpty, OpsError, OpsHeader, StatusPill } from './opsShared';

const formatMoney = (cents: number, currency: string): string => {
  try {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency,
      maximumFractionDigits: 2
    }).format(cents / 100);
  } catch {
    return `${(cents / 100).toFixed(2)} ${currency}`;
  }
};

const todayIso = (): string => new Date().toISOString().slice(0, 10);

export const BillingLedgerView: React.FC<{ language: Language }> = ({ language }) => {
  const t = (key: any) => getTranslation(language, key);

  const [activeTab, setActiveTab] = useState<'invoices' | 'ledger' | 'catalog' | 'clients' | 'tickets' | 'reports'>('invoices');
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  const [ledger, setLedger] = useState<LedgerResponse | null>(null);
  const [products, setProducts] = useState<Array<{ id: string; slug: string; name: string; productType: string; packageName: string; currency: string; monthlyCents: number }>>([]);
  const [clients, setClients] = useState<Array<{ id: string; fullName: string; email: string; country: string; currency: string; riskScore: number }>>([]);
  const [orders, setOrders] = useState<Array<{ id: string; orderNumber: string; clientId: string; productId: string; domain: string; totalCents: number; currency: string; status: string }>>([]);
  const [tickets, setTickets] = useState<Array<{ id: string; ticketNumber: string; department: string; subject: string; priority: string; status: string }>>([]);
  const [report, setReport] = useState<{ currency: string; mrrCents: number; arrCents: number; collectedCents: number; taxCollectedCents: number; activeServices: number; suspendedServices: number } | null>(null);
  const [prodForm, setProdForm] = useState({ slug: '', name: '', packageName: 'starter', monthly: '150000' });
  const [cliForm, setCliForm] = useState({ fullName: '', email: '', country: 'ID' });
  const [ordForm, setOrdForm] = useState({ clientId: '', productId: '', domain: '', username: '' });
  const [tktForm, setTktForm] = useState({ clientId: '', subject: '', priority: 'medium', message: '' });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [ledgerError, setLedgerError] = useState('');
  const [notice, setNotice] = useState('');
  const [busyInvoice, setBusyInvoice] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form, setForm] = useState({
    clientName: '',
    clientEmail: '',
    amount: '',
    currency: 'IDR',
    dueDate: todayIso(),
    description: ''
  });

  const load = async () => {
    setLoading(true);
    setError('');
    setLedgerError('');
    const [invoiceResult, ledgerResult, prodRes, cliRes, ordRes, tktRes, repRes] = await Promise.allSettled([
      apiFetch<Invoice[]>('/api/v1/billing/invoices'),
      apiFetch<LedgerResponse>('/api/v1/billing/ledger'),
      apiFetch<typeof products>('/api/v1/billing/products'),
      apiFetch<typeof clients>('/api/v1/billing/clients'),
      apiFetch<typeof orders>('/api/v1/billing/orders'),
      apiFetch<typeof tickets>('/api/v1/billing/tickets'),
      apiFetch<typeof report>('/api/v1/billing/reports')
    ]);

    if (invoiceResult.status === 'fulfilled') {
      setInvoices(Array.isArray(invoiceResult.value) ? invoiceResult.value : []);
    } else {
      setError(invoiceResult.reason instanceof Error ? invoiceResult.reason.message : String(invoiceResult.reason));
    }

    if (ledgerResult.status === 'fulfilled') {
      setLedger(ledgerResult.value);
    } else {
      setLedger(null);
      setLedgerError(ledgerResult.reason instanceof Error ? ledgerResult.reason.message : String(ledgerResult.reason));
    }
    if (prodRes.status === 'fulfilled' && Array.isArray(prodRes.value)) setProducts(prodRes.value);
    if (cliRes.status === 'fulfilled' && Array.isArray(cliRes.value)) setClients(cliRes.value);
    if (ordRes.status === 'fulfilled' && Array.isArray(ordRes.value)) setOrders(ordRes.value);
    if (tktRes.status === 'fulfilled' && Array.isArray(tktRes.value)) setTickets(tktRes.value);
    if (repRes.status === 'fulfilled' && repRes.value) setReport(repRes.value);
    setLoading(false);
  };

  useEffect(() => {
    void load();
  }, []);

  const totals = useMemo(() => {
    let invoiced = 0;
    let paid = 0;
    let outstanding = 0;
    for (const invoice of invoices) {
      invoiced += invoice.amountCents;
      if (invoice.status === 'paid') paid += invoice.amountCents;
      if (invoice.status === 'unpaid') outstanding += invoice.amountCents;
    }
    return { invoiced, paid, outstanding };
  }, [invoices]);

  const defaultCurrency = invoices[0]?.currency || 'IDR';

  const createInvoice = async (event: React.FormEvent) => {
    event.preventDefault();
    const amountValue = Number(form.amount);
    if (!Number.isFinite(amountValue) || amountValue <= 0) {
      setError(t('billingAmountInvalid'));
      return;
    }

    setSubmitting(true);
    setError('');
    try {
      await apiFetch<Invoice>('/api/v1/billing/invoices', {
        method: 'POST',
        body: JSON.stringify({
          clientName: form.clientName,
          clientEmail: form.clientEmail,
          amountCents: Math.round(amountValue * 100),
          currency: form.currency,
          dueDate: form.dueDate,
          description: form.description
        })
      });
      setNotice(t('billingInvoiceCreated'));
      setShowForm(false);
      setForm({ clientName: '', clientEmail: '', amount: '', currency: form.currency, dueDate: todayIso(), description: '' });
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setSubmitting(false);
    }
  };

  const settle = async (invoice: Invoice) => {
    if (!window.confirm(`${t('billingMarkPaid')} ${invoice.invoiceNumber} (${formatMoney(invoice.amountCents, invoice.currency)})?`)) {
      return;
    }
    const reference = window.prompt(t('billingReference'), '');
    if (reference === null) return;
    setBusyInvoice(invoice.id);
    setError('');
    try {
      await apiFetch('/api/v1/billing/invoices/pay', {
        method: 'POST',
        body: JSON.stringify({ invoiceId: invoice.id, gateway: 'manual', gatewayReference: reference })
      });
      setNotice(`${invoice.invoiceNumber}: ${t('billingPaid')}`);
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusyInvoice('');
    }
  };

  const exportCsv = () => {
    const header = ['invoice', 'client', 'email', 'amountCents', 'currency', 'status', 'issueDate', 'dueDate', 'description'];
    const rows = invoices.map((invoice) => [
      invoice.invoiceNumber,
      invoice.clientName,
      invoice.clientEmail,
      String(invoice.amountCents),
      invoice.currency,
      invoice.status,
      invoice.issueDate,
      invoice.dueDate,
      invoice.description
    ]);
    const csv = [header, ...rows]
      .map((row) => row.map((cell) => `"${String(cell).replace(/"/g, '""')}"`).join(','))
      .join('\n');
    const blob = new Blob([csv], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = `digipanel-invoices-${new Date().toISOString().slice(0, 10)}.csv`;
    anchor.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <OpsHeader
        title={t('billingTitle')}
        subtitle={t('billingSubtitle')}
        action={
          <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => void load()} disabled={loading}>
              <RefreshCw size={14} />
              {t('refresh')}
            </button>
            <button type="button" className="btn btn-secondary btn-sm" onClick={exportCsv} disabled={invoices.length === 0}>
              <Download size={14} />
              {t('billingExportCsv')}
            </button>
            <button type="button" className="btn btn-primary btn-sm" onClick={() => setShowForm((value) => !value)}>
              <Plus size={14} />
              {t('billingNewInvoice')}
            </button>
          </div>
        }
      />

      {notice ? (
        <div className="glass-card" style={{ padding: '10px 14px', fontSize: '0.82rem', borderColor: 'rgba(16, 185, 129, 0.35)' }}>{notice}</div>
      ) : null}
      {error ? <OpsError message={error} onDismiss={() => setError('')} /> : null}

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(210px, 1fr))', gap: '12px' }}>
        {[
          { label: t('billingTotalInvoiced'), value: formatMoney(totals.invoiced, defaultCurrency), icon: FileText },
          { label: t('billingInvoicesPaid'), value: formatMoney(totals.paid, defaultCurrency), icon: CheckCircle2 },
          { label: t('billingPendingReceivables'), value: formatMoney(totals.outstanding, defaultCurrency), icon: CreditCard }
        ].map((card) => (
          <div key={card.label} className="glass-card" style={{ padding: '16px 18px', display: 'flex', justifyContent: 'space-between', gap: '10px' }}>
            <div>
              <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)', marginBottom: '4px' }}>{card.label}</div>
              <div className="font-mono" style={{ fontSize: '1.3rem', fontWeight: 800 }}>{card.value}</div>
            </div>
            <card.icon size={20} style={{ color: 'var(--brand-primary)' }} />
          </div>
        ))}
        <div className="glass-card" style={{ padding: '16px 18px', display: 'flex', justifyContent: 'space-between', gap: '10px' }}>
          <div>
            <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)', marginBottom: '4px' }}>{t('billingTabLedger')}</div>
            {ledger ? (
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '0.9rem', fontWeight: 700, color: ledger.isBalanced ? 'var(--status-online)' : '#ef4444' }}>
                {ledger.isBalanced ? <CheckCircle2 size={16} /> : <AlertTriangle size={16} />}
                {ledger.isBalanced ? t('billingBalanceOk') : t('billingBalanceBad')}
              </div>
            ) : (
              <div style={{ fontSize: '0.82rem', color: 'var(--text-muted)' }}>{t('billingLedgerRestricted')}</div>
            )}
          </div>
          <BookOpen size={20} style={{ color: 'var(--brand-accent)' }} />
        </div>
      </div>

      {showForm ? (
        <form onSubmit={createInvoice} className="glass-panel" style={{ padding: '18px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <div style={{ fontSize: '0.9rem', fontWeight: 700 }}>{t('billingNewInvoice')}</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(190px, 1fr))', gap: '12px' }}>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.76rem', color: 'var(--text-muted)' }}>
              {t('billingClientName')}
              <input className="input-glass" required value={form.clientName} onChange={(event) => setForm((prev) => ({ ...prev, clientName: event.target.value }))} />
            </label>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.76rem', color: 'var(--text-muted)' }}>
              {t('billingClientEmail')}
              <input className="input-glass" type="email" required value={form.clientEmail} onChange={(event) => setForm((prev) => ({ ...prev, clientEmail: event.target.value }))} />
            </label>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.76rem', color: 'var(--text-muted)' }}>
              {t('billingAmountLabel')}
              <input className="input-glass" type="number" min="0.01" step="0.01" required value={form.amount} onChange={(event) => setForm((prev) => ({ ...prev, amount: event.target.value }))} />
            </label>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.76rem', color: 'var(--text-muted)' }}>
              {t('billingCurrency')}
              <select className="input-glass" value={form.currency} onChange={(event) => setForm((prev) => ({ ...prev, currency: event.target.value }))}>
                <option value="IDR">IDR</option>
                <option value="USD">USD</option>
                <option value="SGD">SGD</option>
                <option value="EUR">EUR</option>
              </select>
            </label>
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.76rem', color: 'var(--text-muted)' }}>
              {t('billingThDue')}
              <input className="input-glass" type="date" value={form.dueDate} onChange={(event) => setForm((prev) => ({ ...prev, dueDate: event.target.value }))} />
            </label>
          </div>
          <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.76rem', color: 'var(--text-muted)' }}>
            {t('billingThDescription')}
            <input className="input-glass" required maxLength={300} value={form.description} onChange={(event) => setForm((prev) => ({ ...prev, description: event.target.value }))} />
          </label>
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => setShowForm(false)} disabled={submitting}>{t('cancel')}</button>
            <button type="submit" className="btn btn-primary btn-sm" disabled={submitting}>{t('create')}</button>
          </div>
        </form>
      ) : null}

      <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
        <button type="button" className={activeTab === 'invoices' ? 'btn btn-primary btn-sm' : 'btn btn-secondary btn-sm'} onClick={() => setActiveTab('invoices')}>
          <FileText size={13} />
          {t('billingTabInvoices')}
        </button>
        <button type="button" className={activeTab === 'ledger' ? 'btn btn-primary btn-sm' : 'btn btn-secondary btn-sm'} onClick={() => setActiveTab('ledger')}>
          <BookOpen size={13} />
          {t('billingTabLedger')}
        </button>
        <button type="button" className={activeTab === 'catalog' ? 'btn btn-primary btn-sm' : 'btn btn-secondary btn-sm'} onClick={() => setActiveTab('catalog')}>
          {t('billingTabCatalog')}
        </button>
        <button type="button" className={activeTab === 'clients' ? 'btn btn-primary btn-sm' : 'btn btn-secondary btn-sm'} onClick={() => setActiveTab('clients')}>
          {t('billingTabClients')}
        </button>
        <button type="button" className={activeTab === 'tickets' ? 'btn btn-primary btn-sm' : 'btn btn-secondary btn-sm'} onClick={() => setActiveTab('tickets')}>
          {t('billingTabTickets')}
        </button>
        <button type="button" className={activeTab === 'reports' ? 'btn btn-primary btn-sm' : 'btn btn-secondary btn-sm'} onClick={() => setActiveTab('reports')}>
          {t('billingTabReports')}
        </button>
      </div>

      {activeTab === 'catalog' ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <form
            className="glass-panel"
            style={{ padding: '16px', display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: '10px', alignItems: 'end' }}
            onSubmit={(e) => {
              e.preventDefault();
              void apiFetch('/api/v1/billing/products', {
                method: 'POST',
                body: JSON.stringify({
                  slug: prodForm.slug,
                  name: prodForm.name,
                  productType: 'hosting',
                  packageName: prodForm.packageName,
                  currency: defaultCurrency,
                  monthlyCents: Number(prodForm.monthly) || 100000,
                  autoProvision: true
                })
              }).then(() => { setProdForm({ slug: '', name: '', packageName: 'starter', monthly: '150000' }); return load(); }).catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)));
            }}
          >
            <input className="input-glass" required placeholder="slug (e.g. cloud-pro)" value={prodForm.slug} onChange={(e) => setProdForm((p) => ({ ...p, slug: e.target.value }))} />
            <input className="input-glass" required placeholder="Product name" value={prodForm.name} onChange={(e) => setProdForm((p) => ({ ...p, name: e.target.value }))} />
            <input className="input-glass" placeholder="Package (starter/pro)" value={prodForm.packageName} onChange={(e) => setProdForm((p) => ({ ...p, packageName: e.target.value }))} />
            <input className="input-glass" type="number" min="1" placeholder="Monthly minor units" value={prodForm.monthly} onChange={(e) => setProdForm((p) => ({ ...p, monthly: e.target.value }))} />
            <button type="submit" className="btn btn-primary btn-sm">{t('billingAddProduct')}</button>
          </form>

          <form
            className="glass-panel"
            style={{ padding: '16px', display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: '10px', alignItems: 'end' }}
            onSubmit={(e) => {
              e.preventDefault();
              void apiFetch('/api/v1/billing/orders', {
                method: 'POST',
                body: JSON.stringify({ clientId: ordForm.clientId, productId: ordForm.productId, billingCycle: 'monthly', domain: ordForm.domain, username: ordForm.username })
              }).then(() => { setOrdForm({ clientId: '', productId: '', domain: '', username: '' }); return load(); }).catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)));
            }}
          >
            <select className="input-glass" required value={ordForm.clientId} onChange={(e) => setOrdForm((o) => ({ ...o, clientId: e.target.value }))}>
              <option value="">Client...</option>
              {clients.map((c) => <option key={c.id} value={c.id}>{c.fullName}</option>)}
            </select>
            <select className="input-glass" required value={ordForm.productId} onChange={(e) => setOrdForm((o) => ({ ...o, productId: e.target.value }))}>
              <option value="">Product...</option>
              {products.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select>
            <input className="input-glass" required placeholder="Domain" value={ordForm.domain} onChange={(e) => setOrdForm((o) => ({ ...o, domain: e.target.value }))} />
            <input className="input-glass" required placeholder="Username" value={ordForm.username} onChange={(e) => setOrdForm((o) => ({ ...o, username: e.target.value }))} />
            <button type="submit" className="btn btn-primary btn-sm">{t('billingPlaceOrder')}</button>
          </form>

          <div className="glass-panel" style={{ overflowX: 'auto' }}>
            <table className="glass-table">
              <thead><tr><th>Order</th><th>Domain</th><th>Total</th><th>Status</th></tr></thead>
              <tbody>
                {orders.map((o) => (
                  <tr key={o.id}>
                    <td className="font-mono" style={{ fontSize: '0.78rem' }}>{o.orderNumber}</td>
                    <td>{o.domain || '-'}</td>
                    <td className="font-mono">{formatMoney(o.totalCents, o.currency)}</td>
                    <td><StatusPill status={o.status} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ) : activeTab === 'clients' ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <form
            className="glass-panel"
            style={{ padding: '16px', display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: '10px', alignItems: 'end' }}
            onSubmit={(e) => {
              e.preventDefault();
              void apiFetch('/api/v1/billing/clients', {
                method: 'POST',
                body: JSON.stringify({ fullName: cliForm.fullName, email: cliForm.email, country: cliForm.country, currency: defaultCurrency })
              }).then(() => { setCliForm({ fullName: '', email: '', country: 'ID' }); return load(); }).catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)));
            }}
          >
            <input className="input-glass" required placeholder={t('billingClientName')} value={cliForm.fullName} onChange={(e) => setCliForm((c) => ({ ...c, fullName: e.target.value }))} />
            <input className="input-glass" required type="email" placeholder={t('billingClientEmail')} value={cliForm.email} onChange={(e) => setCliForm((c) => ({ ...c, email: e.target.value }))} />
            <input className="input-glass" maxLength={2} placeholder="Country (ID)" value={cliForm.country} onChange={(e) => setCliForm((c) => ({ ...c, country: e.target.value }))} />
            <button type="submit" className="btn btn-primary btn-sm">{t('billingAddClient')}</button>
          </form>
          <div className="glass-panel" style={{ overflowX: 'auto' }}>
            <table className="glass-table">
              <thead><tr><th>Name</th><th>Email</th><th>Country</th><th>Risk</th></tr></thead>
              <tbody>
                {clients.map((c) => (
                  <tr key={c.id}>
                    <td>{c.fullName}</td>
                    <td className="font-mono" style={{ fontSize: '0.78rem' }}>{c.email}</td>
                    <td>{c.country}</td>
                    <td className="font-mono">{c.riskScore}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ) : activeTab === 'tickets' ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <form
            className="glass-panel"
            style={{ padding: '16px', display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: '10px', alignItems: 'end' }}
            onSubmit={(e) => {
              e.preventDefault();
              void apiFetch('/api/v1/billing/tickets', {
                method: 'POST',
                body: JSON.stringify({ clientId: tktForm.clientId, department: 'support', subject: tktForm.subject, priority: tktForm.priority, message: tktForm.message })
              }).then(() => { setTktForm({ clientId: '', subject: '', priority: 'medium', message: '' }); return load(); }).catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)));
            }}
          >
            <select className="input-glass" required value={tktForm.clientId} onChange={(e) => setTktForm((tk) => ({ ...tk, clientId: e.target.value }))}>
              <option value="">Client...</option>
              {clients.map((c) => <option key={c.id} value={c.id}>{c.fullName}</option>)}
            </select>
            <input className="input-glass" required placeholder="Subject" value={tktForm.subject} onChange={(e) => setTktForm((tk) => ({ ...tk, subject: e.target.value }))} />
            <select className="input-glass" value={tktForm.priority} onChange={(e) => setTktForm((tk) => ({ ...tk, priority: e.target.value }))}>
              <option value="low">low</option>
              <option value="medium">medium</option>
              <option value="high">high</option>
              <option value="urgent">urgent</option>
            </select>
            <input className="input-glass" required placeholder="Message" value={tktForm.message} onChange={(e) => setTktForm((tk) => ({ ...tk, message: e.target.value }))} />
            <button type="submit" className="btn btn-primary btn-sm">{t('billingOpenTicket')}</button>
          </form>
          <div className="glass-panel" style={{ overflowX: 'auto' }}>
            <table className="glass-table">
              <thead><tr><th>Ticket</th><th>Department</th><th>Subject</th><th>Priority</th><th>Status</th></tr></thead>
              <tbody>
                {tickets.map((tk) => (
                  <tr key={tk.id}>
                    <td className="font-mono">{tk.ticketNumber}</td>
                    <td>{tk.department}</td>
                    <td>{tk.subject}</td>
                    <td>{tk.priority}</td>
                    <td><StatusPill status={tk.status} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ) : activeTab === 'reports' ? (
        <div className="glass-panel" style={{ padding: '18px', display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div style={{ fontWeight: 700 }}>{t('billingTabReports')}</div>
            <button
              type="button"
              className="btn btn-primary btn-sm"
              onClick={() => void apiFetch('/api/v1/billing/automation/run', { method: 'POST' }).then(() => { setNotice('Billing automation cycle completed.'); return load(); })}
            >
              {t('billingRunAutomation')}
            </button>
          </div>
          {report ? (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '12px' }}>
              <div className="glass-card" style={{ padding: '12px' }}><div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{t('billingMrr')}</div><div className="font-mono" style={{ fontWeight: 800 }}>{formatMoney(report.mrrCents, report.currency)}</div></div>
              <div className="glass-card" style={{ padding: '12px' }}><div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{t('billingArr')}</div><div className="font-mono" style={{ fontWeight: 800 }}>{formatMoney(report.arrCents, report.currency)}</div></div>
              <div className="glass-card" style={{ padding: '12px' }}><div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{t('billingCollected')}</div><div className="font-mono" style={{ fontWeight: 800 }}>{formatMoney(report.collectedCents, report.currency)}</div></div>
              <div className="glass-card" style={{ padding: '12px' }}><div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Active / Suspended</div><div className="font-mono" style={{ fontWeight: 800 }}>{report.activeServices} / {report.suspendedServices}</div></div>
            </div>
          ) : null}
        </div>
      ) : null}

      {(activeTab === 'invoices' || activeTab === 'ledger') && (loading ? (
        <div className="glass-panel" style={{ padding: '40px', textAlign: 'center', color: 'var(--text-muted)' }}>{t('appsLoading')}</div>
      ) : activeTab === 'invoices' ? (
        invoices.length === 0 ? (
          <OpsEmpty message={t('billingNoInvoices')} />
        ) : (
          <div className="glass-panel" style={{ overflowX: 'auto' }}>
            <table className="glass-table">
              <thead>
                <tr>
                  <th>{t('billingThInvoice')}</th>
                  <th>{t('billingThClient')}</th>
                  <th>{t('billingThDescription')}</th>
                  <th>{t('billingThAmount')}</th>
                  <th>{t('billingThDue')}</th>
                  <th>{t('thStatus')}</th>
                  <th style={{ textAlign: 'right' }}>{t('thActions')}</th>
                </tr>
              </thead>
              <tbody>
                {invoices.map((invoice) => (
                  <tr key={invoice.id}>
                    <td className="font-mono" style={{ fontSize: '0.78rem', fontWeight: 700 }}>{invoice.invoiceNumber}</td>
                    <td>
                      <div style={{ fontSize: '0.8rem', fontWeight: 600 }}>{invoice.clientName}</div>
                      <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>{invoice.clientEmail}</div>
                    </td>
                    <td style={{ fontSize: '0.78rem', maxWidth: '260px' }}>{invoice.description}</td>
                    <td className="font-mono" style={{ fontSize: '0.8rem' }}>{formatMoney(invoice.amountCents, invoice.currency)}</td>
                    <td className="font-mono" style={{ fontSize: '0.74rem' }}>{invoice.dueDate}</td>
                    <td><StatusPill status={invoice.status} /></td>
                    <td style={{ textAlign: 'right' }}>
                      {invoice.status === 'unpaid' ? (
                        <button type="button" className="btn btn-secondary btn-sm" disabled={busyInvoice === invoice.id} onClick={() => void settle(invoice)}>
                          <CreditCard size={12} />
                          {t('billingMarkPaid')}
                        </button>
                      ) : (
                        <span style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>{invoice.paidAt ? new Date(invoice.paidAt).toLocaleDateString() : '-'}</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      ) : ledgerError ? (
        <OpsError message={ledgerError} />
      ) : !ledger || ledger.entries.length === 0 ? (
        <OpsEmpty message={t('billingNoLedger')} />
      ) : (
        <div className="glass-panel" style={{ overflowX: 'auto' }}>
          <table className="glass-table">
            <thead>
              <tr>
                <th>#</th>
                <th>{t('billingThTxnRef')}</th>
                <th>{t('billingThDebit')}</th>
                <th>{t('billingThCredit')}</th>
                <th>{t('billingThAmount')}</th>
                <th>{t('billingThMemo')}</th>
              </tr>
            </thead>
            <tbody>
              {ledger.entries.map((entry) => (
                <tr key={entry.id}>
                  <td className="font-mono" style={{ fontSize: '0.72rem' }}>{entry.id}</td>
                  <td className="font-mono" style={{ fontSize: '0.74rem' }}>{entry.txRef}</td>
                  <td className="font-mono" style={{ fontSize: '0.72rem' }}>{entry.direction === 'debit' ? entry.accountCode : '-'}</td>
                  <td className="font-mono" style={{ fontSize: '0.72rem' }}>{entry.direction === 'credit' ? entry.accountCode : '-'}</td>
                  <td className="font-mono" style={{ fontSize: '0.78rem' }}>{formatMoney(entry.amountCents, entry.currency)}</td>
                  <td style={{ fontSize: '0.74rem', color: 'var(--text-muted)' }}>{entry.direction}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <div style={{ padding: '10px 14px', fontSize: '0.74rem', color: 'var(--text-muted)' }}>
            {t('billingThDebit')} {formatMoney(ledger.totalDebits, ledger.currency)} &middot; {t('billingThCredit')} {formatMoney(ledger.totalCredits, ledger.currency)}
          </div>
        </div>
      ))}
    </div>
  );
};
