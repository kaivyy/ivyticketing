import { getToken, refresh, redirectToLogin, getApiBaseUrl, fetchApi } from "./auth";

export interface ReadyResponse {
  status: "ready" | "not_ready";
  checks: Record<string, string>;
}

export interface ApiError {
  code: string;
  message: string;
}

export async function fetchReadiness(): Promise<ReadyResponse | null> {
  try {
    const res = await fetchApi("/readyz");
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
    fetchApi(`/api/v1${path}`, {
      method: opts?.method ?? "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${getToken() ?? ""}`,
        ...opts?.headers,
      },
      credentials: "include",
      body: opts?.body != null ? (typeof opts.body === "string" ? opts.body : JSON.stringify(opts.body)) : undefined,
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

export async function fetchPublicEvents(): Promise<any[]> {
  try {
    const res = await fetchApi("/api/v1/public/events");
    if (!res.ok) return [];
    return (await res.json()) || [];
  } catch {
    return [];
  }
}

export async function fetchPublicEvent(idOrSlug: string): Promise<any | null> {
  try {
    const res = await fetchApi(`/api/v1/public/events/${encodeURIComponent(idOrSlug)}`);
    if (!res.ok) return null;
    return await res.json();
  } catch {
    return null;
  }
}

export async function fetchPublicPaymentChannels(eventId: string): Promise<any[]> {
  try {
    const res = await fetchApi(`/api/v1/public/events/${encodeURIComponent(eventId)}/payment-channels`);
    if (!res.ok) return [];
    return (await res.json()) || [];
  } catch {
    return [];
  }
}

export async function fetchPublicForm(eventId: string): Promise<any | null> {
  try {
    const res = await fetchApi(`/api/v1/public/events/${encodeURIComponent(eventId)}/form`);
    if (!res.ok) return null;
    return await res.json();
  } catch {
    return null;
  }
}
