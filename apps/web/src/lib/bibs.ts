import { authedFetch } from "./api";

const base = (orgId: string, eventId: string) =>
  `/organizations/${orgId}/events/${eventId}/tickets`;

export interface BibPreview {
  eventId: string;
  nextBib: string;
  prefix?: string;
  numericNext: number;
  assignedCount: number;
}

export interface BulkAssignResult {
  eventId: string;
  assigned: number;
  failed: number;
  skipped: number;
  lastBib?: string;
}

export async function previewNextBib(orgId: string, eventId: string): Promise<BibPreview> {
  return authedFetch<BibPreview>(`${base(orgId, eventId)}/bib/next`);
}

export async function assignNextBib(orgId: string, eventId: string, ticketId: string): Promise<void> {
  await authedFetch(`${base(orgId, eventId)}/${ticketId}/bib/assign`, { method: "POST" });
}

export async function setBib(orgId: string, eventId: string, ticketId: string, bibNumber: string): Promise<void> {
  await authedFetch(`${base(orgId, eventId)}/${ticketId}/bib`, {
    method: "PUT",
    body: { bibNumber },
  });
}

export async function clearBib(orgId: string, eventId: string, ticketId: string): Promise<void> {
  await authedFetch(`${base(orgId, eventId)}/${ticketId}/bib`, { method: "DELETE" });
}

export async function bulkAssignBibs(orgId: string, eventId: string): Promise<BulkAssignResult> {
  return authedFetch<BulkAssignResult>(`${base(orgId, eventId)}/bib/bulk-assign`, {
    method: "POST",
  });
}

export function bibsExportUrl(orgId: string, eventId: string): string {
  return `${base(orgId, eventId)}/bib/export`;
}