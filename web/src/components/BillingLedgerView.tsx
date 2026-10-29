// DigiPanel Native Billing Engine & Double-Entry Ledger (Section 7)
// Zero emojis, strictly Lucide SVG iconography, modern glass styling

import React, { useState } from 'react';
import { 
  FileText, 
  CreditCard, 
  DollarSign, 
  TrendingUp, 
  CheckCircle2, 
  Clock, 
  Plus, 
  Download,
  BookOpen,
  ArrowUpRight,
  ArrowDownLeft,
  ChevronRight
} from 'lucide-react';
import { BillingInvoice, LedgerEntry, Language } from '../types';
import { getTranslation } from '../i18n/translations';

interface BillingLedgerViewProps {
  language: Language;
}

export const BillingLedgerView: React.FC<BillingLedgerViewProps> = ({ language }) => {
  const t = (key: any) => getTranslation(language, key);

  const [activeTab, setActiveTab] = useState<'invoices' | 'ledger' | 'plans'>('invoices');

  const [invoices, setInvoices] = useState<BillingInvoice[]>([
    {
      id: 'inv-1001',
      invoiceNumber: 'INV-2026-0042',
      clientName: 'PT Nusantara Digital',
      clientEmail: 'billing@nusantara.id',
      amountCents: 15000000, // IDR 150,000
      currency: 'IDR',
      status: 'paid',
      issueDate: '2026-10-01',
      dueDate: '2026-10-15',
      itemDescription: 'Standard Cloud Hosting (Annual + Let\'s Encrypt Wildcard)'
    },
    {
      id: 'inv-1002',
      invoiceNumber: 'INV-2026-0043',
      clientName: 'testuser',
      clientEmail: 'testuser@digipanel.live',
      amountCents: 5000000, // IDR 50,000
      currency: 'IDR',
      status: 'paid',
      issueDate: '2026-10-15',
      dueDate: '2026-10-29',
      itemDescription: 'Starter Cloud Package (Monthly Plan)'
    },
    {
      id: 'inv-1003',
      invoiceNumber: 'INV-2026-0044',
      clientName: 'Artha Media Studio',
      clientEmail: 'finance@arthamedia.co.id',
      amountCents: 45000000, // IDR 450,000
      currency: 'IDR',
      status: 'unpaid',
      issueDate: '2026-10-24',
      dueDate: '2026-11-07',
      itemDescription: 'Reseller Enterprise Tier (Monthly 100 Accounts Quota)'
    }
  ]);

  const ledgerEntries: LedgerEntry[] = [
    {
      id: 'led-1',
      timestamp: '2026-10-15 10:24:00',
      transactionRef: 'TXN-98402',
      accountDebit: '1010-CASH-GATEWAY-MIDTRANS',
      accountCredit: '4010-REVENUE-HOSTING-PLANS',
      amountCents: 5000000,
      currency: 'IDR',
      description: 'Invoice #INV-2026-0043 settlement via QRIS/VA'
    },
    {
      id: 'led-2',
      timestamp: '2026-10-01 08:12:00',
      transactionRef: 'TXN-98214',
      accountDebit: '1010-CASH-GATEWAY-XENDIT',
      accountCredit: '4010-REVENUE-HOSTING-PLANS',
      amountCents: 15000000,
      currency: 'IDR',
      description: 'Invoice #INV-2026-0042 annual renewal'
    }
  ];

  const formatCurrency = (cents: number, cur: string) => {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: cur,
      maximumFractionDigits: 0
    }).format(cents / 100);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      {/* Header */}
      <div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '0.82rem', color: 'var(--text-muted)', marginBottom: '4px' }}>
          <span>{t('adminPanel')}</span>
          <ChevronRight size={14} />
          <span style={{ color: 'var(--text-main)', fontWeight: 600 }}>{t('navBilling')}</span>
        </div>
        <h1 style={{ fontSize: '1.5rem', fontWeight: 800, color: 'var(--text-main)', letterSpacing: '-0.02em' }}>
          Native Billing Engine & Double-Entry Ledger
        </h1>
        <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
          Autonomous provisioning, idempotent webhooks, integer accounting, and compliance ledger.
        </p>
      </div>

      {/* Summary Cards */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
        gap: '16px'
      }}>
        <div className="glass-card" style={{ padding: '20px', display: 'flex', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '4px' }}>Monthly Recurring (MRR)</div>
            <div className="font-mono" style={{ fontSize: '1.4rem', fontWeight: 800, color: 'var(--text-main)' }}>
              {formatCurrency(65000000, 'IDR')}
            </div>
            <div style={{ fontSize: '0.74rem', color: 'var(--status-online)', marginTop: '4px' }}>
              &uarr; 14.2% vs last month
            </div>
          </div>
          <div style={{
            width: '42px',
            height: '42px',
            borderRadius: '12px',
            background: 'rgba(16, 185, 129, 0.12)',
            color: 'var(--status-online)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <TrendingUp size={20} />
          </div>
        </div>

        <div className="glass-card" style={{ padding: '20px', display: 'flex', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '4px' }}>Total Invoices Paid</div>
            <div className="font-mono" style={{ fontSize: '1.4rem', fontWeight: 800, color: 'var(--text-main)' }}>
              {formatCurrency(20000000, 'IDR')}
            </div>
            <div style={{ fontSize: '0.74rem', color: 'var(--text-muted)', marginTop: '4px' }}>
              2 invoices settled
            </div>
          </div>
          <div style={{
            width: '42px',
            height: '42px',
            borderRadius: '12px',
            background: 'rgba(2, 132, 199, 0.12)',
            color: 'var(--status-info)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <CheckCircle2 size={20} />
          </div>
        </div>

        <div className="glass-card" style={{ padding: '20px', display: 'flex', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '4px' }}>Pending Receivables</div>
            <div className="font-mono" style={{ fontSize: '1.4rem', fontWeight: 800, color: 'var(--text-main)' }}>
              {formatCurrency(45000000, 'IDR')}
            </div>
            <div style={{ fontSize: '0.74rem', color: 'var(--status-warning)', marginTop: '4px' }}>
              1 invoice awaiting payment
            </div>
          </div>
          <div style={{
            width: '42px',
            height: '42px',
            borderRadius: '12px',
            background: 'rgba(245, 158, 11, 0.12)',
            color: 'var(--status-warning)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}>
            <Clock size={20} />
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div style={{ display: 'flex', gap: '8px', borderBottom: '1px solid var(--glass-border)', paddingBottom: '8px' }}>
        <button
          onClick={() => setActiveTab('invoices')}
          className={`btn btn-sm ${activeTab === 'invoices' ? 'btn-primary' : 'btn-secondary'}`}
        >
          <FileText size={14} />
          <span>Invoices & Auto-Provisioning</span>
        </button>
        <button
          onClick={() => setActiveTab('ledger')}
          className={`btn btn-sm ${activeTab === 'ledger' ? 'btn-primary' : 'btn-secondary'}`}
        >
          <BookOpen size={14} />
          <span>Double-Entry General Ledger</span>
        </button>
      </div>

      {/* Invoices Tab */}
      {activeTab === 'invoices' && (
        <div className="glass-panel" style={{ padding: '24px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <h3 style={{ fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-main)' }}>
              Recent Orders & Automated Invoices
            </h3>
            <button className="btn btn-secondary btn-sm">
              <Download size={14} />
              <span>Export CSV</span>
            </button>
          </div>

          <div style={{ overflowX: 'auto' }}>
            <table className="glass-table">
              <thead>
                <tr>
                  <th>INVOICE #</th>
                  <th>CLIENT</th>
                  <th>DESCRIPTION</th>
                  <th>AMOUNT</th>
                  <th>STATUS</th>
                  <th>DUE DATE</th>
                  <th style={{ textAlign: 'right' }}>ACTION</th>
                </tr>
              </thead>
              <tbody>
                {invoices.map(inv => (
                  <tr key={inv.id}>
                    <td className="font-mono" style={{ fontWeight: 700 }}>
                      {inv.invoiceNumber}
                    </td>
                    <td>
                      <div style={{ fontWeight: 600 }}>{inv.clientName}</div>
                      <div className="font-mono" style={{ fontSize: '0.72rem', color: 'var(--text-dim)' }}>{inv.clientEmail}</div>
                    </td>
                    <td style={{ fontSize: '0.82rem', color: 'var(--text-muted)' }}>
                      {inv.itemDescription}
                    </td>
                    <td className="font-mono" style={{ fontWeight: 700 }}>
                      {formatCurrency(inv.amountCents, inv.currency)}
                    </td>
                    <td>
                      {inv.status === 'paid' ? (
                        <span className="badge-online">Paid</span>
                      ) : (
                        <span className="badge-warning">Unpaid</span>
                      )}
                    </td>
                    <td className="font-mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                      {inv.dueDate}
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      <button className="btn btn-secondary btn-sm" style={{ padding: '4px 8px', fontSize: '0.74rem' }}>
                        View PDF
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Ledger Tab */}
      {activeTab === 'ledger' && (
        <div className="glass-panel" style={{ padding: '24px' }}>
          <div style={{ marginBottom: '16px' }}>
            <h3 style={{ fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-main)', marginBottom: '4px' }}>
              Immutable Double-Entry Ledger (Balanced Trial Balance)
            </h3>
            <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
              Every billing transaction writes two balanced debit/credit records with cryptographic transaction reference.
            </p>
          </div>

          <div style={{ overflowX: 'auto' }}>
            <table className="glass-table">
              <thead>
                <tr>
                  <th>TIMESTAMP</th>
                  <th>TXN REF</th>
                  <th>DEBIT ACCOUNT</th>
                  <th>CREDIT ACCOUNT</th>
                  <th>AMOUNT</th>
                  <th>MEMO</th>
                </tr>
              </thead>
              <tbody>
                {ledgerEntries.map(entry => (
                  <tr key={entry.id}>
                    <td className="font-mono" style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                      {entry.timestamp}
                    </td>
                    <td className="font-mono" style={{ fontWeight: 600, color: 'var(--brand-primary)' }}>
                      {entry.transactionRef}
                    </td>
                    <td className="font-mono" style={{ fontSize: '0.78rem' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                        <ArrowUpRight size={13} style={{ color: 'var(--status-online)' }} />
                        <span>{entry.accountDebit}</span>
                      </div>
                    </td>
                    <td className="font-mono" style={{ fontSize: '0.78rem' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                        <ArrowDownLeft size={13} style={{ color: 'var(--brand-accent)' }} />
                        <span>{entry.accountCredit}</span>
                      </div>
                    </td>
                    <td className="font-mono" style={{ fontWeight: 700 }}>
                      {formatCurrency(entry.amountCents, entry.currency)}
                    </td>
                    <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                      {entry.description}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
};
