import { getApiBaseUrl } from "./auth";

export interface SecurityConfig {
  turnstileEnabled: boolean;
  siteKey?: string;
}

export async function getSecurityConfig(): Promise<SecurityConfig> {
  try {
    const base = getApiBaseUrl();
    const res = await fetch(`${base}/api/v1/security/config`);
    if (!res.ok) return { turnstileEnabled: false };
    return (await res.json()) as SecurityConfig;
  } catch {
    return { turnstileEnabled: false };
  }
}
