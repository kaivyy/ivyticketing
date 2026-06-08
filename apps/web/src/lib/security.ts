const API_URL = import.meta.env.PUBLIC_API_URL ?? "http://localhost:8080";

export interface SecurityConfig {
  turnstileEnabled: boolean;
  siteKey?: string;
}

export async function getSecurityConfig(): Promise<SecurityConfig> {
  try {
    const res = await fetch(`${API_URL}/api/v1/security/config`);
    if (!res.ok) return { turnstileEnabled: false };
    return (await res.json()) as SecurityConfig;
  } catch {
    return { turnstileEnabled: false };
  }
}
