import { authedFetch } from "./api";

export interface BallotEntry {
  id: string;
  draw_id: string;
  status: "APPLIED" | "WINNER" | "WAITLISTED" | "NOT_SELECTED" | "CONVERTED" | "WITHDRAWN";
  waitlist_rank?: number;
  payment_deadline?: string; // ISO-8601
  converted_at?: string;
}

export function applyBallot(
  eventId: string,
  categoryId: string,
  drawId: string
): Promise<BallotEntry> {
  return authedFetch<BallotEntry>(
    `/events/${eventId}/categories/${categoryId}/ballot/apply`,
    { method: "POST", body: { draw_id: drawId } }
  );
}

export function getMyBallotEntry(
  eventId: string,
  categoryId: string
): Promise<BallotEntry | null> {
  return authedFetch<BallotEntry>(
    `/events/${eventId}/categories/${categoryId}/ballot/my-entry`
  ).catch((err: Error) => {
    if (err.message.startsWith("HTTP 404") || err.message === "HTTP 404") {
      return null;
    }
    throw err;
  });
}

export function withdrawBallot(
  eventId: string,
  categoryId: string
): Promise<void> {
  return authedFetch<void>(
    `/events/${eventId}/categories/${categoryId}/ballot/my-entry`,
    { method: "DELETE" }
  );
}
