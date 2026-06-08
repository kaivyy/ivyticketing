import { getToken, refresh, redirectToLogin } from "./auth";

export interface ReadyResponse {
  status: "ready" | "not_ready";
  checks: Record<string, string>;
}

export interface ApiError {
  code: string;
  message: string;
}

const API_URL = import.meta.env.PUBLIC_API_URL ?? "http://localhost:8080";
const BASE = API_URL;

export async function fetchReadiness(): Promise<ReadyResponse | null> {
  try {
    const res = await fetch(`${API_URL}/readyz`);
    return (await res.json()) as ReadyResponse;
  } catch {
    return null;
  }
}

export async function authedFetch<T>(
  path: string,
  opts?: { method?: string; body?: unknown; headers?: Record<string, string> }
): Promise<T> {
  const doFetch = () =>
    fetch(`${BASE}/api/v1${path}`, {
      method: opts?.method ?? "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${getToken() ?? ""}`,
        ...opts?.headers,
      },
      credentials: "include",
      body: opts?.body != null ? JSON.stringify(opts.body) : undefined,
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
    let err: ApiError = { code: "ERROR", message: `HTTP ${res.status}` };
    try {
      const body = await res.json();
      if (body?.error) err = body.error;
    } catch { /* ignore */ }
    throw new Error(err.message);
  }
  return (await res.json()) as T;
}
