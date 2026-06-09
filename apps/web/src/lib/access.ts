import { authedFetch } from "./api";

export interface AccessGrant {
  id: string;
  token: string;
  categoryId: string;
  expiresAt: string;
}

export function redeemCode(
  eventId: string,
  categoryId: string,
  code: string
): Promise<AccessGrant> {
  return authedFetch<AccessGrant>(`/events/${eventId}/access/redeem`, {
    method: "POST",
    body: { code, categoryId },
  });
}

export function getMyGrants(
  eventId: string
): Promise<AccessGrant[]> {
  return authedFetch<AccessGrant[]>(`/events/${eventId}/access/my-grants`);
}

export interface WaitlistPosition {
  position: number;
  status: "WAITING" | "PROMOTED" | "EXPIRED" | "NOT_ELIGIBLE";
}

export function joinWaitlist(
  eventId: string,
  categoryId: string
): Promise<WaitlistPosition> {
  return authedFetch<WaitlistPosition>(
    `/events/${eventId}/categories/${categoryId}/waitlist/join`,
    { method: "POST", body: {} }
  );
}

export function getWaitlistPosition(
  eventId: string,
  categoryId: string
): Promise<WaitlistPosition> {
  return authedFetch<WaitlistPosition>(
    `/events/${eventId}/categories/${categoryId}/waitlist/my-position`
  );
}
