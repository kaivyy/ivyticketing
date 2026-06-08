const API_URL = import.meta.env.PUBLIC_API_URL ?? "http://localhost:8080";
const TOKEN_KEY = "ivy_access_token";

export interface User {
  id: string;
  email: string;
  fullName: string;
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

export async function login(email: string, password: string): Promise<void> {
  const res = await fetch(`${API_URL}/api/v1/auth/login`, {
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

export async function refresh(): Promise<boolean> {
  const res = await fetch(`${API_URL}/api/v1/auth/refresh`, {
    method: "POST",
    credentials: "include",
  });
  if (!res.ok) return false;
  const data = await res.json();
  setToken(data.accessToken);
  return true;
}

export async function logout(): Promise<void> {
  await fetch(`${API_URL}/api/v1/auth/logout`, { method: "POST", credentials: "include" }).catch(() => {});
  clearToken();
}

export function redirectToLogin(): void {
  if (typeof window !== "undefined") window.location.href = "/login";
}
