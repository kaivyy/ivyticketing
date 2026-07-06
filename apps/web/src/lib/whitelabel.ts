import { authedFetch } from "./api";

// --- types (mirror services/api/internal/modules/whitelabel/model.go) ---

export interface Branding {
  organizationId: string;
  logoObjectKey: string;
  themeColor: string;
  emailFromName: string;
  emailFromAddress: string;
  termsText: string;
  footerText: string;
  whitelabelEnabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface UpsertBranding {
  logoObjectKey: string;
  themeColor: string;
  emailFromName: string;
  emailFromAddress: string;
  termsText: string;
  footerText: string;
  whitelabelEnabled: boolean;
}

export interface Domain {
  id: string;
  organizationId: string;
  domain: string;
  verificationToken: string;
  verificationName: string;
  status: string;
  verifiedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export const DOMAIN_STATUS_LABELS: Record<string, string> = {
  PENDING: "Menunggu Verifikasi",
  VERIFIED: "Terverifikasi",
  FAILED: "Gagal",
};

// --- organizer endpoints (gated on branding.manage) ---

const base = (orgId: string) => `/organizations/${orgId}/branding`;

export function getBranding(orgId: string): Promise<Branding> {
  return authedFetch<Branding>(base(orgId));
}

export function upsertBranding(
  orgId: string,
  body: UpsertBranding
): Promise<Branding> {
  return authedFetch<Branding>(base(orgId), { method: "PUT", body });
}

export function listDomains(orgId: string): Promise<Domain[]> {
  return authedFetch<Domain[]>(`${base(orgId)}/domains`);
}

export function addDomain(
  orgId: string,
  domain: string
): Promise<Domain> {
  return authedFetch<Domain>(`${base(orgId)}/domains`, {
    method: "POST",
    body: { domain },
  });
}

export function verifyDomain(
  orgId: string,
  domainId: string
): Promise<Domain> {
  return authedFetch<Domain>(
    `${base(orgId)}/domains/${domainId}/verify`,
    { method: "POST" }
  );
}

export function deleteDomain(orgId: string, domainId: string): Promise<void> {
  return authedFetch<void>(`${base(orgId)}/domains/${domainId}`, {
    method: "DELETE",
  });
}
