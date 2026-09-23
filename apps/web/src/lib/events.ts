import { getApiBaseUrl } from "./auth";

export interface PublicCategory {
  id: string;
  name: string;
  price: number;
  registrationOpensAt: string;
  registrationClosesAt: string;
  registrationMode: string;
}

export interface PublicEvent {
  id: string;
  name: string;
  slug: string;
  eventType: string;
  description: string;
  bannerUrl: string;
  logoUrl: string;
  venueName: string;
  startsAt: string | null;
  endsAt: string | null;
  categories: PublicCategory[];
}

/**
 * Fetch a public event by org slug + event slug.
 * Uses the public (unauthenticated) catalog endpoint:
 * GET /public/organizations/{orgSlug}/events/{eventSlug}
 */
export async function fetchPublicEvent(
  orgSlug: string,
  eventSlug: string
): Promise<PublicEvent> {
  const base = getApiBaseUrl();
  const res = await fetch(
    `${base}/api/v1/public/organizations/${encodeURIComponent(orgSlug)}/events/${encodeURIComponent(eventSlug)}`
  );
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    const msg = body?.error?.message ?? `HTTP ${res.status}`;
    throw new Error(msg);
  }
  return res.json() as Promise<PublicEvent>;
}

/**
 * Fetch all published events for an org.
 * GET /public/organizations/{orgSlug}/events
 */
export async function fetchPublicEvents(orgSlug: string): Promise<PublicEvent[]> {
  const base = getApiBaseUrl();
  const res = await fetch(
    `${base}/api/v1/public/organizations/${encodeURIComponent(orgSlug)}/events`
  );
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    const msg = body?.error?.message ?? `HTTP ${res.status}`;
    throw new Error(msg);
  }
  return res.json() as Promise<PublicEvent[]>;
}
