const API_URL = (typeof import.meta !== "undefined" ? import.meta.env.PUBLIC_API_URL : undefined) ?? "http://localhost:8080";

export interface PublicCategory {
  id: string;
  name: string;
  price: number;
  registrationOpensAt: string;
  registrationClosesAt: string;
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
  const res = await fetch(
    `${API_URL}/api/v1/public/organizations/${encodeURIComponent(orgSlug)}/events/${encodeURIComponent(eventSlug)}`
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
  const res = await fetch(
    `${API_URL}/api/v1/public/organizations/${encodeURIComponent(orgSlug)}/events`
  );
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    const msg = body?.error?.message ?? `HTTP ${res.status}`;
    throw new Error(msg);
  }
  return res.json() as Promise<PublicEvent[]>;
}
