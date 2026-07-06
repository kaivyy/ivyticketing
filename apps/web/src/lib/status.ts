import { authedFetch } from "./api";

// --- types (mirror services/api/internal/modules/status/dto.go) ---

export interface Component {
  key: string;
  name: string;
  status: string;
  sortOrder: number;
  updatedAt: string;
}

export interface IncidentUpdate {
  id: string;
  status: string;
  body: string;
  createdAt: string;
}

export interface Incident {
  id: string;
  title: string;
  impact: string;
  status: string;
  startedAt: string;
  resolvedAt: string | null;
  createdAt: string;
  updatedAt: string;
  updates: IncidentUpdate[];
}

export interface StatusPage {
  overall: string;
  components: Component[];
  incidents: Incident[];
  lastUpdated: string;
}

export const COMPONENT_STATUS_LABELS: Record<string, string> = {
  OPERATIONAL: "Beroperasi",
  DEGRADED: "Terganggu",
  DOWN: "Gangguan",
};

export const OVERALL_LABELS: Record<string, string> = {
  OPERATIONAL: "Semua Sistem Beroperasi",
  DEGRADED: "Sebagian Sistem Terganggu",
  DOWN: "Gangguan Sistem",
};

export const IMPACT_LABELS: Record<string, string> = {
  NONE: "Tidak Ada",
  MINOR: "Ringan",
  MAJOR: "Berat",
  CRITICAL: "Kritis",
};

export const INCIDENT_STATUS_LABELS: Record<string, string> = {
  INVESTIGATING: "Investigasi",
  IDENTIFIED: "Teridentifikasi",
  MONITORING: "Pemantauan",
  RESOLVED: "Selesai",
};

const API_URL =
  (import.meta.env.PUBLIC_API_URL as string | undefined) ??
  "http://localhost:8080";

// --- public endpoints (no auth) ---

export async function fetchPublicStatus(): Promise<StatusPage> {
  const res = await fetch(`${API_URL}/api/v1/public/status`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return (await res.json()) as StatusPage;
}

export async function fetchPublicIncidents(
  limit = 20,
  offset = 0,
): Promise<Incident[]> {
  const res = await fetch(
    `${API_URL}/api/v1/public/status/incidents?limit=${limit}&offset=${offset}`,
  );
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return (await res.json()) as Incident[];
}

// --- super-admin endpoints (RequirePlatformAdmin) ---

export function listIncidentsAdmin(
  limit = 50,
  offset = 0,
): Promise<Incident[]> {
  return authedFetch<Incident[]>(
    `/admin/status/incidents?limit=${limit}&offset=${offset}`,
  );
}

export function createIncident(body: {
  title: string;
  impact: string;
  body: string;
}): Promise<Incident> {
  return authedFetch<Incident>(`/admin/status/incidents`, {
    method: "POST",
    body,
  });
}

export function addIncidentUpdate(
  incidentId: string,
  body: { status: string; body: string },
): Promise<Incident> {
  return authedFetch<Incident>(
    `/admin/status/incidents/${incidentId}/updates`,
    { method: "POST", body },
  );
}

export function updateComponent(
  key: string,
  status: string,
): Promise<Component> {
  return authedFetch<Component>(`/admin/status/components/${key}`, {
    method: "PUT",
    body: { status },
  });
}
