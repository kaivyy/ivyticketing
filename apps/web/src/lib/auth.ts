const TOKEN_KEY = "ivy_access_token";

export function getApiBaseUrl(): string {
  if (typeof window !== "undefined") {
    const envUrl = import.meta.env.PUBLIC_API_URL;
    if (envUrl && !envUrl.includes("localhost") && !envUrl.includes("127.0.0.1") && envUrl.startsWith("http")) {
      return envUrl;
    }
    return "";
  }
  return import.meta.env.PUBLIC_API_URL ?? "http://127.0.0.1:8081";
}

export async function fetchApi(path: string, init?: RequestInit): Promise<Response> {
  const base = getApiBaseUrl();
  try {
    return await fetch(`${base}${path}`, init);
  } catch (err) {
    if (typeof window !== "undefined" && base !== "") {
      try {
        return await fetch(path, init);
      } catch {
        throw err;
      }
    }
    throw err;
  }
}

export interface User {
  id: string;
  email: string;
  fullName: string;
  isPlatformAdmin?: boolean;
}

export interface Membership {
  organizationId: string;
  organizationSlug?: string;
  organizationName?: string;
  memberId: string;
  roles?: string[];
  roleSlugs?: string[];
  permissions: string[];
}

export interface MeResponse {
  user: User;
  memberships: Membership[];
}

export function getToken(): string | null {
  if (typeof sessionStorage === "undefined") return null;
  return sessionStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  sessionStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  sessionStorage.removeItem(TOKEN_KEY);
}

export async function register(fullName: string, email: string, password: string, phone?: string): Promise<void> {
  const res = await fetchApi("/api/v1/auth/register", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ fullName, email, password, phone: phone || "" }),
  });
  if (!res.ok) {
    const errorData = await res.json().catch(() => null);
    throw new Error(errorData?.message || "Pendaftaran gagal. Periksa kembali format data.");
  }
  // Otomatis login setelah pendaftaran berhasil
  await login(email, password);
}

export async function login(email: string, password: string): Promise<void> {
  const res = await fetchApi("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ email, password }),
  });
  if (!res.ok) {
    throw new Error("Email atau kata sandi salah.");
  }
  const data = await res.json();
  setToken(data.accessToken);
}

export async function getMe(): Promise<MeResponse | null> {
  const token = getToken();
  if (!token) return null;
  try {
    const res = await fetchApi("/api/v1/auth/me", {
      headers: { Authorization: `Bearer ${token}` },
      credentials: "include",
    });
    if (!res.ok) return null;
    return await res.json();
  } catch {
    return null;
  }
}

export async function refresh(): Promise<boolean> {
  try {
    const res = await fetchApi("/api/v1/auth/refresh", {
      method: "POST",
      credentials: "include",
    });
    if (!res.ok) return false;
    const data = await res.json();
    setToken(data.accessToken);
    return true;
  } catch {
    return false;
  }
}

export async function logout(): Promise<void> {
  await fetchApi("/api/v1/auth/logout", { method: "POST", credentials: "include" }).catch(() => {});
  clearToken();
}

export function redirectToLogin(): void {
  if (typeof window !== "undefined") window.location.href = "/login";
}
