import { authedFetch } from "./api";

const base = (orgId: string) => `/organizations/${orgId}/reports`;

// Report type identifiers — mirror the backend export_jobs.report_type CHECK.
export type ReportType =
  | "participant"
  | "sales"
  | "payment"
  | "coupon"
  | "queue"
  | "ballot"
  | "racepack"
  | "revenue";

export const REPORT_TYPES: { value: ReportType; label: string }[] = [
  { value: "participant", label: "Peserta" },
  { value: "sales", label: "Penjualan" },
  { value: "payment", label: "Pembayaran" },
  { value: "coupon", label: "Kupon" },
  { value: "queue", label: "Antrian" },
  { value: "ballot", label: "Ballot" },
  { value: "racepack", label: "Racepack" },
  { value: "revenue", label: "Pendapatan" },
];

export type JobStatus = "PENDING" | "PROCESSING" | "READY" | "FAILED";

export interface ExportJob {
  id: string;
  reportType: ReportType;
  format: string;
  status: JobStatus;
  rowCount?: number;
  fileUrl?: string;
  error?: string;
  eventId?: string;
  requestedBy: string;
  createdAt: string;
  completedAt?: string;
}

// getSummary returns the on-screen aggregate for a report type as a loose
// object (shape varies per report type).
export function getSummary(
  orgId: string,
  reportType: ReportType,
  eventId?: string
): Promise<Record<string, unknown>> {
  const q = eventId ? `?eventId=${encodeURIComponent(eventId)}` : "";
  return authedFetch<Record<string, unknown>>(
    `${base(orgId)}/${reportType}/summary${q}`
  );
}

export function listExports(
  orgId: string,
  limit = 50,
  offset = 0
): Promise<ExportJob[]> {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  return authedFetch<ExportJob[]>(`${base(orgId)}/exports?${params}`);
}

export function getExport(orgId: string, jobId: string): Promise<ExportJob> {
  return authedFetch<ExportJob>(`${base(orgId)}/exports/${jobId}`);
}

export function createExport(
  orgId: string,
  reportType: ReportType,
  eventId?: string
): Promise<ExportJob> {
  return authedFetch<ExportJob>(`${base(orgId)}/exports`, {
    method: "POST",
    body: { reportType, eventId },
  });
}
