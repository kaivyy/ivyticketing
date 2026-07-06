import { authedFetch } from "./api";

// --- shared types (mirror services/api/internal/modules/billing/model.go) ---

export interface Package {
  id: string;
  slug: string;
  name: string;
  description: string;
  priceMonthly: number;
  maxEvents: number | null;
  feeBps: number;
  features: string[];
  isActive: boolean;
  sortOrder: number;
}

export interface Subscription {
  id: string;
  organizationId: string;
  status: string;
  startedAt: string;
  expiresAt: string | null;
  package: Package;
}

export interface Invoice {
  id: string;
  organizationId: string;
  invoiceNumber: string;
  periodStart: string;
  periodEnd: string;
  subscriptionAmount: number;
  feeAmount: number;
  totalAmount: number;
  status: string;
  issuedAt: string | null;
  paidAt: string | null;
  createdAt: string;
}

export interface FeeSummary {
  entries: number;
  grossOrders: number;
  totalFees: number;
}

export interface RevenueRow {
  organizationId: string;
  organizationName: string;
  totalFees: number;
  grossOrders: number;
  feeEntries: number;
}

// Feature keys mirror the backend Feature* consts.
export const FEATURE_LABELS: Record<string, string> = {
  basic_registration: "Registrasi Dasar",
  basic_payment: "Pembayaran Dasar",
  queue: "Antrian Virtual",
  ballot: "Ballot / Undian",
  racepack: "Racepack",
  custom_branding: "Branding Kustom",
  whitelabel: "White Label",
  custom_domain: "Domain Kustom",
  custom_payment: "Gateway Pembayaran Sendiri",
  dedicated_queue: "Antrian Terdedikasi",
  api: "Akses API",
};

export const SUB_STATUS_LABELS: Record<string, string> = {
  ACTIVE: "Aktif",
  CANCELLED: "Dibatalkan",
  EXPIRED: "Kedaluwarsa",
};

export const INVOICE_STATUS_LABELS: Record<string, string> = {
  DRAFT: "Draf",
  ISSUED: "Terbit",
  PAID: "Lunas",
  VOID: "Batal",
};

// --- organizer endpoints (gated on billing.view) ---

const orgBase = (orgId: string) => `/organizations/${orgId}/billing`;

export function getSubscription(orgId: string): Promise<Subscription> {
  return authedFetch<Subscription>(`${orgBase(orgId)}/subscription`);
}

export function listActivePackages(orgId: string): Promise<Package[]> {
  return authedFetch<Package[]>(`${orgBase(orgId)}/packages`);
}

export function getFeeSummary(orgId: string): Promise<FeeSummary> {
  return authedFetch<FeeSummary>(`${orgBase(orgId)}/fees/summary`);
}

export function listInvoices(
  orgId: string,
  limit = 50,
  offset = 0
): Promise<Invoice[]> {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  return authedFetch<Invoice[]>(`${orgBase(orgId)}/invoices?${params}`);
}

// --- super-admin endpoints (RequirePlatformAdmin) ---

const adminBase = "/admin/billing";

export function adminListPackages(): Promise<Package[]> {
  return authedFetch<Package[]>(`${adminBase}/packages`);
}

export interface UpsertPackage {
  slug: string;
  name: string;
  description: string;
  priceMonthly: number;
  maxEvents: number | null;
  feeBps: number;
  features: string[];
  isActive: boolean;
  sortOrder: number;
}

export function adminCreatePackage(body: UpsertPackage): Promise<Package> {
  return authedFetch<Package>(`${adminBase}/packages`, {
    method: "POST",
    body,
  });
}

export function adminUpdatePackage(
  packageId: string,
  body: UpsertPackage
): Promise<Package> {
  return authedFetch<Package>(`${adminBase}/packages/${packageId}`, {
    method: "PUT",
    body,
  });
}

export function adminRevenue(): Promise<RevenueRow[]> {
  return authedFetch<RevenueRow[]>(`${adminBase}/revenue`);
}

export function adminAssignSubscription(
  orgId: string,
  packageId: string,
  expiresAt?: string | null
): Promise<Subscription> {
  return authedFetch<Subscription>(
    `${adminBase}/organizations/${orgId}/subscription`,
    { method: "PUT", body: { packageId, expiresAt: expiresAt ?? null } }
  );
}

export interface GenerateInvoice {
  periodStart: string;
  periodEnd: string;
  subscriptionAmount: number;
  feeAmount: number;
}

export function adminGenerateInvoice(
  orgId: string,
  body: GenerateInvoice
): Promise<Invoice> {
  return authedFetch<Invoice>(
    `${adminBase}/organizations/${orgId}/invoices`,
    { method: "POST", body }
  );
}

export function adminMarkInvoicePaid(invoiceId: string): Promise<Invoice> {
  return authedFetch<Invoice>(`${adminBase}/invoices/${invoiceId}/paid`, {
    method: "POST",
  });
}
