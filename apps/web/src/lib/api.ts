export interface ReadyResponse {
  status: "ready" | "not_ready";
  checks: Record<string, string>;
}

const API_URL = import.meta.env.PUBLIC_API_URL ?? "http://localhost:8080";

export async function fetchReadiness(): Promise<ReadyResponse | null> {
  try {
    const res = await fetch(`${API_URL}/readyz`);
    return (await res.json()) as ReadyResponse;
  } catch {
    return null;
  }
}
