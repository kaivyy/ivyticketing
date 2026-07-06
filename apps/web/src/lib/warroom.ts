import { authedFetch } from "./api";

// Snapshot mirrors metrics.Snapshot in the Go API (GET /admin/warroom).
export interface WarRoomSnapshot {
  capturedAt: string;

  // Gauges (current values).
  activeQueueUsers: number;
  dbConnsInUse: number;
  dbConnsIdle: number;
  httpInFlight: number;

  // Counters (cumulative totals since start).
  queueReleased: number;
  checkoutStarted: number;
  checkoutSucceeded: number;
  checkoutFailed: number;
  paymentSucceeded: number;
  paymentFailed: number;
  racepackScans: number;

  // Derived rates / health.
  httpRequests: number;
  httpErrors: number;
  errorRate: number;
  checkoutSuccessRate: number;
  paymentSuccessRate: number;
  httpP95Seconds: number;
}

export function fetchWarRoom(): Promise<WarRoomSnapshot> {
  return authedFetch<WarRoomSnapshot>("/admin/warroom");
}
