// DigiPanel API and View Types
// All structures mirror the core REST contract. No sample values live here.

export type UserRole = 'admin' | 'reseller' | 'user';

export type Language = 'en' | 'id';

export type ThemeMode = 'dark' | 'light';

export interface Telemetry {
  hostname: string;
  kernel: string;
  os: string;
  uptime: string;
  uptimeSeconds: number;
  loadAverage: number[];
  cpuUsagePercent: number;
  memoryUsedMb: number;
  memoryTotalMb: number;
  memoryFreeMb: number;
  memoryCachedMb: number;
  diskUsedGb: number;
  diskTotalGb: number;
  diskFreeGb: number;
  activeProcesses: number;
  publicIp: string;
  runtimesDetected: {
    nodeVersion: string;
    pythonVersion: string;
    goVersion: string;
    phpVersion: string;
  };
}

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

export interface ApiAccount {
  id: string;
  tenantId: string;
  username: string;
  primaryDomain: string;
  packageId: string;
  posixUid: number;
  posixGid: number;
  homeDir: string;
  documentRoot: string;
  phpVersion: string;
  diskLimitBytes: number;
  diskUsedBytes: number;
  bandwidthLimitBytes: number;
  bandwidthUsedBytes: number;
  inodesLimit: number;
  inodesUsed: number;
  status: string;
  ipAddress: string;
  cgroupsPath: string;
  createdAt: string;
  updatedAt: string;
}

export interface HostingAccount {
  id: string;
  username: string;
  domain: string;
  plan: string;
  diskUsageMb: number;
  diskLimitMb: number;
  bandwidthUsageMb: number;
  bandwidthLimitMb: number;
  inodesCount: number;
  inodesLimit: number;
  status: 'active' | 'suspended' | 'terminated';
  phpVersion: string;
  ipAddress: string;
  homeDir: string;
  createdAt: string;
}

export interface FleetNode {
  id: string;
  name: string;
  hostname: string;
  ipAddress: string;
  role: string;
  status: string;
  panelVersion: string;
}

export interface FleetResponse {
  totalNodes: number;
  nodes: FleetNode[];
}

export interface ServiceStatus {
  unit: string;
  name: string;
  category: string;
  active: boolean;
  enabled: boolean;
  state: string;
  installed: boolean;
}

export interface ServiceResponse {
  services: ServiceStatus[];
  probedAt: string;
  managedBy: string;
}

export interface Invoice {
  id: string;
  invoiceNumber: string;
  clientName: string;
  clientEmail: string;
  amountCents: number;
  currency: string;
  status: 'unpaid' | 'paid' | 'cancelled';
  issueDate: string;
  dueDate: string;
  description: string;
  gateway?: string;
  gatewayReference?: string;
  createdAt: string;
  paidAt?: string;
}

export interface LedgerEntry {
  id: number;
  txRef: string;
  accountCode: string;
  amountCents: number;
  direction: 'debit' | 'credit';
  currency: string;
}

export interface LedgerResponse {
  entries: LedgerEntry[];
  totalDebits: number;
  totalCredits: number;
  isBalanced: boolean;
  currency: string;
}

export interface DnsRecord {
  id: string;
  domain: string;
  type: string;
  name: string;
  content: string;
  ttl: number;
  priority?: number;
}

export interface HostingToolItem {
  id: string;
  titleKey: string;
  descriptionKey: string;
  category: 'domains' | 'files' | 'databases' | 'emails' | 'metrics' | 'security' | 'software' | 'advanced' | 'preferences';
  icon: string;
  actionId?: string;
}

export interface AiChatMessage {
  id: string;
  sender: 'user' | 'assistant';
  timestamp: string;
  content: string;
  suggestedAction?: {
    label: string;
    actionType: string;
  };
}
