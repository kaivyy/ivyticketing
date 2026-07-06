import { authedFetch } from "./api";

const base = (orgId: string, eventId: string) =>
  `/organizations/${orgId}/events/${eventId}/racepack`;

// ── Counters ──────────────────────────────────────────────────────────────────

export interface Counter {
  id: string;
  name: string;
  location?: string;
  active: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export function listCounters(orgId: string, eventId: string): Promise<Counter[]> {
  return authedFetch<Counter[]>(`${base(orgId, eventId)}/counters`);
}

export function createCounter(
  orgId: string,
  eventId: string,
  name: string,
  location?: string
): Promise<Counter> {
  return authedFetch<Counter>(`${base(orgId, eventId)}/counters`, {
    method: "POST",
    body: { name, location },
  });
}

export function updateCounter(
  orgId: string,
  eventId: string,
  id: string,
  updates: { name?: string; location?: string; active?: boolean }
): Promise<Counter> {
  return authedFetch<Counter>(`${base(orgId, eventId)}/counters/${id}`, {
    method: "PUT",
    body: updates,
  });
}

export function setCounterActive(
  orgId: string,
  eventId: string,
  id: string,
  active: boolean
): Promise<void> {
  return authedFetch<void>(`${base(orgId, eventId)}/counters/${id}/activate`, {
    method: "POST",
    body: { active },
  });
}

// ── Slots ─────────────────────────────────────────────────────────────────────

export interface Slot {
  id: string;
  name: string;
  pickupDate: string;
  startTime: string;
  endTime: string;
  capacity: number;
  reservedCount?: number;
  active: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export interface SlotInput {
  name: string;
  pickupDate: string;
  startTime: string;
  endTime: string;
  capacity: number;
}

export function listSlots(orgId: string, eventId: string): Promise<Slot[]> {
  return authedFetch<Slot[]>(`${base(orgId, eventId)}/slots`);
}

export function createSlot(
  orgId: string,
  eventId: string,
  slot: SlotInput
): Promise<Slot> {
  return authedFetch<Slot>(`${base(orgId, eventId)}/slots`, {
    method: "POST",
    body: slot,
  });
}

export function updateSlot(
  orgId: string,
  eventId: string,
  id: string,
  slot: Partial<SlotInput> & { active?: boolean }
): Promise<Slot> {
  return authedFetch<Slot>(`${base(orgId, eventId)}/slots/${id}`, {
    method: "PUT",
    body: slot,
  });
}

// ── Pickups ───────────────────────────────────────────────────────────────────

export type PickupMethod = "SELF" | "PROXY" | "MANUAL_OVERRIDE";

export interface PickupRecord {
  id: string;
  ticketId: string;
  bibNumber?: string;
  counterId: string;
  slotId?: string;
  staffId?: string;
  staffName?: string;
  pickupMethod: PickupMethod;
  pickupTimestamp: string;
  notes?: string;
  status: string;
}

export function executePickup(
  orgId: string,
  eventId: string,
  ticketId: string,
  counterId: string,
  method: PickupMethod,
  notes?: string,
  opts?: { slotId?: string; idempotencyKey?: string }
): Promise<PickupRecord> {
  return authedFetch<PickupRecord>(`${base(orgId, eventId)}/pickups`, {
    method: "POST",
    body: { ticketId, counterId, slotId: opts?.slotId, method, notes },
    headers: opts?.idempotencyKey
      ? { "Idempotency-Key": opts.idempotencyKey }
      : undefined,
  });
}

export function getPickupStatusByTicket(
  orgId: string,
  eventId: string,
  ticketId: string
): Promise<PickupRecord | null> {
  const params = new URLSearchParams({ ticketId });
  return authedFetch<PickupRecord | null>(
    `${base(orgId, eventId)}/pickups/status?${params}`
  );
}

export function listPickups(
  orgId: string,
  eventId: string,
  limit = 50,
  offset = 0
): Promise<PickupRecord[]> {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  return authedFetch<PickupRecord[]>(`${base(orgId, eventId)}/pickups?${params}`);
}

// ── Proxy authorizations ──────────────────────────────────────────────────────

export interface ProxyAuthorizationInput {
  ticketId: string;
  proxyName: string;
  proxyPhone?: string;
  proxyIdentity: string;
  authorizationDocument?: string;
  pickupRecordId?: string;
}

export interface ProxyAuthorization {
  id: string;
  ticketId: string;
  proxyName: string;
  proxyPhone?: string;
  proxyIdentity: string;
  authorizationDocument?: string;
  pickupRecordId?: string;
  createdAt: string;
}

export function createProxyAuthorization(
  orgId: string,
  eventId: string,
  ticketId: string,
  payload: Omit<ProxyAuthorizationInput, "ticketId">,
  opts?: { idempotencyKey?: string }
): Promise<ProxyAuthorization> {
  return authedFetch<ProxyAuthorization>(
    `${base(orgId, eventId)}/proxy-authorizations`,
    {
      method: "POST",
      body: { ticketId, ...payload },
      headers: opts?.idempotencyKey
        ? { "Idempotency-Key": opts.idempotencyKey }
        : undefined,
    }
  );
}

// ── Problem cases ─────────────────────────────────────────────────────────────

export type ProblemStatus = "OPEN" | "UNDER_REVIEW" | "RESOLVED" | "ESCALATED";

export interface ProblemCase {
  id: string;
  ticketId?: string;
  participantId?: string;
  reason: string;
  status: ProblemStatus;
  resolution?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ProblemCaseInput {
  ticketId?: string;
  participantId?: string;
  reason: string;
}

export function createProblemCase(
  orgId: string,
  eventId: string,
  payload: ProblemCaseInput,
  opts?: { idempotencyKey?: string }
): Promise<ProblemCase> {
  return authedFetch<ProblemCase>(`${base(orgId, eventId)}/problem-cases`, {
    method: "POST",
    body: payload,
    headers: opts?.idempotencyKey
      ? { "Idempotency-Key": opts.idempotencyKey }
      : undefined,
  });
}

export function updateProblemCase(
  orgId: string,
  eventId: string,
  caseId: string,
  status: ProblemStatus,
  resolution?: string
): Promise<ProblemCase> {
  return authedFetch<ProblemCase>(
    `${base(orgId, eventId)}/problem-cases/${caseId}`,
    {
      method: "PUT",
      body: { status, resolution },
    }
  );
}

export function listProblemCases(
  orgId: string,
  eventId: string
): Promise<ProblemCase[]> {
  return authedFetch<ProblemCase[]>(`${base(orgId, eventId)}/problem-cases`);
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

export interface DashboardCounterCount {
  counter_id: string;
  counter_name?: string;
  count: number;
  active?: boolean;
}

export interface Dashboard {
  totalPickups: number;
  byCounter: DashboardCounterCount[];
  openCases: number;
  totalCounters?: number;
  activeCounters?: number;
}

export function getDashboard(orgId: string, eventId: string): Promise<Dashboard> {
  return authedFetch<Dashboard>(`${base(orgId, eventId)}/dashboard`);
}