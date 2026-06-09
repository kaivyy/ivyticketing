import { authedFetch } from "./api";
import { getToken } from "./auth";

const API_URL = import.meta.env.PUBLIC_API_URL ?? "http://localhost:8080";

export interface CorporateAccount {
  id: string;
  name: string;
  billingEmail: string;
  status: string;
  approvedAt?: string;
}

export interface BulkUploadResult {
  imported: number;
  skipped: number;
  errors?: string[];
}

export interface Invoice {
  account: { name: string; billing_email: string };
  line_items: { description: string; quantity: number; unit_price: number; total: number }[];
  total: number;
  currency: string;
}

export function createCorporateAccount(
  orgId: string,
  data: { name: string; billingEmail: string; invoiceRequired: boolean }
): Promise<CorporateAccount> {
  return authedFetch<CorporateAccount>(`/organizations/${orgId}/access/corporate`, {
    method: "POST",
    body: data,
  });
}

export function listCorporateAccounts(orgId: string): Promise<CorporateAccount[]> {
  return authedFetch<CorporateAccount[]>(`/organizations/${orgId}/access/corporate`);
}

export function approveCorporateAccount(orgId: string, accountId: string): Promise<void> {
  return authedFetch<void>(`/organizations/${orgId}/access/corporate/${accountId}/approve`, {
    method: "POST",
  });
}

export async function bulkUploadMembers(
  orgId: string,
  poolId: string,
  file: File
): Promise<BulkUploadResult> {
  const form = new FormData();
  form.append("file", file);
  const res = await fetch(
    `${API_URL}/api/v1/organizations/${orgId}/access/pools/${poolId}/members`,
    {
      method: "POST",
      headers: { Authorization: `Bearer ${getToken() ?? ""}` },
      credentials: "include",
      body: form,
    }
  );
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `HTTP ${res.status}`);
  }
  return res.json() as Promise<BulkUploadResult>;
}

export function getInvoice(
  orgId: string,
  accountId: string,
  eventId: string,
  unitPrice?: number
): Promise<Invoice> {
  const params = new URLSearchParams({ eventId });
  if (unitPrice != null) params.set("unitPrice", String(unitPrice));
  return authedFetch<Invoice>(
    `/organizations/${orgId}/access/corporate/${accountId}/invoice?${params}`
  );
}
