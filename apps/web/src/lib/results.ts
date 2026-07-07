import { authedFetch } from "./api";
import { getToken, refresh, redirectToLogin } from "./auth";

const API_URL = import.meta.env.PUBLIC_API_URL ?? "http://localhost:8080";

export interface ResultRow {
  id: string;
  eventId: string;
  categoryId?: string;
  ticketId?: string;
  bibNumber: string;
  participantName: string;
  gender?: string;
  age?: number;
  ageGroup?: string;
  status: string;
  chipTimeMs?: number;
  gunTimeMs?: number;
  chipTime?: string;
  gunTime?: string;
  rankOverall?: number;
  rankGender?: number;
  rankCategory?: number;
  rankAgeGroup?: number;
  source: string;
  finishedAt?: string;
}

export interface ResultsPage {
  results: ResultRow[];
  total: number;
}

export interface ImportSummary {
  imported: number;
  skipped: number;
  errors?: string[];
  ranked: boolean;
}

export interface CertificateTemplate {
  id: string;
  eventId: string;
  name: string;
  title: string;
  subtitle: string;
  bodyTemplate: string;
  backgroundUrl?: string;
  isActive: boolean;
  createdAt: string;
}

export interface CertificateRender {
  title: string;
  subtitle: string;
  body: string;
  backgroundUrl?: string;
  result: ResultRow;
}

export interface CreateTemplateBody {
  name: string;
  title: string;
  subtitle: string;
  bodyTemplate: string;
  backgroundUrl: string;
  isActive: boolean;
}

const base = (orgId: string, eventId: string) =>
  `/organizations/${orgId}/events/${eventId}/results`;

// Status labels (Indonesian) for the FINISHED/DNF/DNS domain.
export const STATUS_LABELS: Record<string, string> = {
  FINISHED: "Selesai",
  DNF: "DNF",
  DNS: "DNS",
};

// Gender labels (Indonesian) for the M/F/X domain.
export const GENDER_LABELS: Record<string, string> = {
  M: "Pria",
  F: "Wanita",
  X: "Lainnya",
};

export function listResults(
  orgId: string,
  eventId: string,
  opts: { categoryId?: string; gender?: string; limit?: number; offset?: number } = {}
): Promise<ResultsPage> {
  const q = new URLSearchParams();
  if (opts.categoryId) q.set("categoryId", opts.categoryId);
  if (opts.gender) q.set("gender", opts.gender);
  q.set("limit", String(opts.limit ?? 100));
  q.set("offset", String(opts.offset ?? 0));
  return authedFetch<ResultsPage>(`${base(orgId, eventId)}/?${q.toString()}`);
}

// --- participant self-service (authn-level, ticket ownership verified) ---

export function getMyResult(ticketId: string): Promise<ResultRow> {
  return authedFetch<ResultRow>(`/tickets/${encodeURIComponent(ticketId)}/result`);
}

export function getMyCertificate(ticketId: string): Promise<CertificateRender> {
  return authedFetch<CertificateRender>(
    `/tickets/${encodeURIComponent(ticketId)}/certificate`
  );
}

export function getResultByBib(
  orgId: string,
  eventId: string,
  bib: string
): Promise<ResultRow> {
  return authedFetch<ResultRow>(`${base(orgId, eventId)}/bib/${encodeURIComponent(bib)}`);
}

export function getCertificate(
  orgId: string,
  eventId: string,
  ticketId: string
): Promise<CertificateRender> {
  return authedFetch<CertificateRender>(
    `${base(orgId, eventId)}/certificate/${encodeURIComponent(ticketId)}`
  );
}

export function recompute(orgId: string, eventId: string): Promise<{ ranked: boolean }> {
  return authedFetch<{ ranked: boolean }>(`${base(orgId, eventId)}/recompute`, { method: "POST" });
}

export function deleteResults(orgId: string, eventId: string): Promise<void> {
  return authedFetch<void>(`${base(orgId, eventId)}/`, { method: "DELETE" });
}

export function listTemplates(
  orgId: string,
  eventId: string
): Promise<{ templates: CertificateTemplate[] }> {
  return authedFetch<{ templates: CertificateTemplate[] }>(`${base(orgId, eventId)}/templates/`);
}

export function createTemplate(
  orgId: string,
  eventId: string,
  body: CreateTemplateBody
): Promise<CertificateTemplate> {
  return authedFetch<CertificateTemplate>(`${base(orgId, eventId)}/templates/`, {
    method: "POST",
    body,
  });
}

export function updateTemplate(
  orgId: string,
  eventId: string,
  templateId: string,
  body: CreateTemplateBody
): Promise<CertificateTemplate> {
  return authedFetch<CertificateTemplate>(`${base(orgId, eventId)}/templates/${templateId}`, {
    method: "PUT",
    body,
  });
}

export function deleteTemplate(
  orgId: string,
  eventId: string,
  templateId: string
): Promise<void> {
  return authedFetch<void>(`${base(orgId, eventId)}/templates/${templateId}`, {
    method: "DELETE",
  });
}

// importCSV uploads a raw CSV file. authedFetch always JSON-encodes its body,
// so this uses a direct fetch with a text/csv payload and the same 401-refresh
// retry the shared helper provides.
export async function importCSV(
  orgId: string,
  eventId: string,
  csvText: string
): Promise<ImportSummary> {
  const url = `${API_URL}/api/v1${base(orgId, eventId)}/import`;
  const doFetch = () =>
    fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "text/csv",
        Authorization: `Bearer ${getToken() ?? ""}`,
      },
      credentials: "include",
      body: csvText,
    });

  let res = await doFetch();
  if (res.status === 401) {
    const ok = await refresh();
    if (!ok) {
      redirectToLogin();
      throw new Error("unauthenticated");
    }
    res = await doFetch();
  }
  if (!res.ok) {
    let msg = `HTTP ${res.status}`;
    try {
      const body = await res.json();
      if (body?.error?.message) msg = body.error.message;
    } catch {
      /* ignore */
    }
    throw new Error(msg);
  }
  return (await res.json()) as ImportSummary;
}
