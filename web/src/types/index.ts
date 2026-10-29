// DigiPanel Type Definitions
// Strictly adhering to: no emojis, fully typed, enterprise hosting semantics.

export type UserRole = 'admin' | 'reseller' | 'user';

export type Language = 'en' | 'id';

export type ThemeMode = 'dark' | 'light';

export interface SystemMetrics {
  hostname: string;
  kernel: string;
  os: string;
  uptime: string;
  loadAverage: [number, number, number];
  cpuUsagePercent: number;
  memoryUsedMb: number;
  memoryTotalMb: number;
  diskUsedGb: number;
  diskTotalGb: number;
  activeProcesses: number;
}

export interface HostingAccount {
  id: string;
  username: string;
  domain: string;
  plan: string;
  resellerOwner?: string;
  diskUsageMb: number;
  diskLimitMb: number;
  bandwidthUsageMb: number;
  bandwidthLimitMb: number;
  databasesCount: number;
  databasesLimit: number;
  emailCount: number;
  emailLimit: number;
  inodesCount: number;
  inodesLimit: number;
  status: 'active' | 'suspended' | 'pending';
  phpVersion: string;
  ipAddress: string;
  createdAt: string;
  sslActive: boolean;
}

export interface ClusterNode {
  id: string;
  name: string;
  hostname: string;
  ipAddress: string;
  role: 'hub' | 'edge' | 'database';
  status: 'online' | 'degraded' | 'offline';
  panelVersion: string;
  cpuPercent: number;
  ramPercent: number;
  diskPercent: number;
  inventory: {
    accounts: number;
    domains: number;
    databases: number;
    resellers: number;
  };
  services: {
    web: boolean; // nginx / openlitespeed
    ftp: boolean; // pure-ftpd / vsftpd
    dns: boolean; // pdns
    db: boolean; // mariadb / postgresql
    fail2ban: boolean;
    nftables: boolean;
    securebox: boolean;
    postfix: boolean;
    dovecot: boolean;
    rspamd: boolean;
    mlmmj: boolean;
  };
}

export interface DnsRecord {
  id: string;
  domain: string;
  type: 'A' | 'AAAA' | 'CNAME' | 'MX' | 'TXT' | 'SRV' | 'CAA';
  name: string;
  content: string;
  ttl: number;
  priority?: number;
}

export interface AiChatMessage {
  id: string;
  sender: 'user' | 'assistant';
  timestamp: string;
  content: string;
  suggestedAction?: {
    label: string;
    actionType: 'open_dns' | 'open_ssl' | 'open_terminal' | 'open_files' | 'open_php';
  };
}

export interface HostingToolItem {
  id: string;
  titleKey: string;
  descriptionKey: string;
  category: 'domains' | 'files' | 'databases' | 'emails' | 'metrics' | 'security' | 'software' | 'advanced' | 'preferences';
  icon: string;
  badge?: string;
  actionId?: string;
}

export interface BillingInvoice {
  id: string;
  invoiceNumber: string;
  clientName: string;
  clientEmail: string;
  amountCents: number;
  currency: string;
  status: 'paid' | 'unpaid' | 'overdue' | 'cancelled';
  issueDate: string;
  dueDate: string;
  itemDescription: string;
}

export interface LedgerEntry {
  id: string;
  timestamp: string;
  transactionRef: string;
  accountDebit: string;
  accountCredit: string;
  amountCents: number;
  currency: string;
  description: string;
}
