import { authedFetch } from "./api";

export interface JoinResponse {
  tokenId: string;
  status: string;
  position: number;
}

export interface QueueStatusResponse {
  tokenId: string;
  status: string;
  position: number;
  estimatedWaitSeconds: number;
  systemState: string;
  admissionToken?: string;
  checkoutExpiresAt?: string;
}

export function joinQueue(eventId: string): Promise<JoinResponse> {
  const token =
    typeof window !== "undefined"
      ? ((window as any).__ivyTurnstileToken as string | undefined)
      : undefined;
  const headers: Record<string, string> = {};
  if (token) headers["X-Turnstile-Token"] = token;
  return authedFetch<JoinResponse>(`/events/${eventId}/queue/join`, {
    method: "POST",
    headers: Object.keys(headers).length > 0 ? headers : undefined,
  });
}

export function getQueueStatus(eventId: string): Promise<QueueStatusResponse> {
  return authedFetch<QueueStatusResponse>(`/events/${eventId}/queue/status`);
}
