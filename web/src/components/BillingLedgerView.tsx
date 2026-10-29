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

  const [activeTab, setActiveTab] = useState<'invoices' | 'ledger'>('invoices');
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  const [ledger, setLedger] = useState<LedgerResponse | null>(null);
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
    const [invoiceResult, ledgerResult] = await Promise.allSettled([
      apiFetch<Invoice[]>('/api/v1/billing/invoices'),
      apiFetch<LedgerResponse>('/api/v1/billing/ledger')
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

      <div style={{ display: 'flex', gap: '6px' }}>
        <button type="button" className={activeTab === 'invoices' ? 'btn btn-primary btn-sm' : 'btn btn-secondary btn-sm'} onClick={() => setActiveTab('invoices')}>
          <FileText size={13} />
          {t('billingTabInvoices')}
        </button>
        <button type="button" className={activeTab === 'ledger' ? 'btn btn-primary btn-sm' : 'btn btn-secondary btn-sm'} onClick={() => setActiveTab('ledger')}>
          <BookOpen size={13} />
          {t('billingTabLedger')}
        </button>
      </div>

      {loading ? (
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
      )}
    </div>
  );
};
