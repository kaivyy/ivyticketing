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
  return authedFetch<JoinResponse>(`/events/${eventId}/queue/join`, { method: "POST" });
}

export function getQueueStatus(eventId: string): Promise<QueueStatusResponse> {
  return authedFetch<QueueStatusResponse>(`/events/${eventId}/queue/status`);
}
