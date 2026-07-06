// Typed fetch wrapper for the ivyticketing API.
//
// Responsibilities (task 9.1):
//   - Resolve the API base URL from VITE_API_BASE_URL (default localhost:8080).
//   - Attach `Authorization: Bearer <token>` from the injected session accessor.
//   - Support an optional `Idempotency-Key` header on POSTs (check-in + pickup
//     replay) so the offline sync engine (task 11) can retry exactly-once.
//   - Map non-2xx responses onto a typed ApiError carrying the server's stable
//     error code so the UI/sync layers can classify (2xx vs 409 vs 4xx).
//
// DTO field names mirror the backend Go structs exactly (camelCase JSON).

import { getSessionToken, setSessionToken } from './session';

const DEFAULT_BASE_URL = 'http://localhost:8080';

/** Resolved API base URL without a trailing slash. */
export const API_BASE_URL: string = (
  import.meta.env.VITE_API_BASE_URL ?? DEFAULT_BASE_URL
).replace(/\/+$/, '');

// --- Error taxonomy -------------------------------------------------------

/** Standard error body shape returned by the API. */
export interface ApiErrorBody {
  code?: string;
  message?: string;
  [key: string]: unknown;
}

/** Thrown for any non-2xx response. `code` carries the server's stable code. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly body: ApiErrorBody | null;

  constructor(status: number, code: string, message: string, body: ApiErrorBody | null) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.body = body;
  }
}

// --- Response / request DTOs (match services/api scanner + racepack) ------

/** Non-sensitive participant fields shown during a scan (scanner.DisplayInfo). */
export interface DisplayInfo {
  participantName: string;
  bibNumber: string;
  category: string;
  ticketStatus: string; // VALID | USED | CANCELLED
}

/** Response to POST /scan/verify (scanner.VerifyResult). */
export interface VerifyResult {
  ticketId: string;
  eventId: string;
  display: DisplayInfo;
  alreadyPickedUp: boolean;
  pickedUpAt?: string;
  alreadyCheckedIn: boolean;
  checkedInAt?: string;
}

/** Response to POST /scan/check-in (scanner.CheckInResult). */
export interface CheckInResult {
  ticketId: string;
  status: string; // USED
  checkedInAt: string;
  duplicate: boolean;
}

/** A single Permitted_Event (scanner.PermittedEvent). */
export interface PermittedEvent {
  eventId: string;
  organizationId: string;
  name: string;
  status: string;
}

/** Response to GET /scan/events (scanner.ListPermittedEventsResult). */
export interface ListPermittedEventsResult {
  events: PermittedEvent[];
}

/** Body for POST /scan/check-in. */
export interface CheckInRequest {
  ticketId: string;
  scannedAt?: string; // ISO8601 original offline scan time
}

/** Body for POST /racepack/pickups (subset used by the scanner replay). */
export interface PickupRequest {
  ticketId: string;
  counterId: string;
  slotId?: string;
  method?: string;
  notes?: string;
  scannedAt?: string;
}

/** Response to POST /racepack/pickups (racepack.PickupResponse subset). */
export interface PickupResult {
  organizationId: string;
  eventId: string;
  ticketId: string;
  participantId?: string;
  bibNumber?: string;
  counterId?: string;
  slotId?: string;
  staffId?: string;
  pickedUpAt?: string;
}

// --- Core request helper --------------------------------------------------

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: unknown;
  /** When set, sent as the `Idempotency-Key` header (POST replay support). */
  idempotencyKey?: string;
  signal?: AbortSignal;
}

async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, idempotencyKey, signal } = opts;

  const headers: Record<string, string> = {
    Accept: 'application/json',
  };

  const token = getSessionToken();
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }

  if (idempotencyKey) {
    headers['Idempotency-Key'] = idempotencyKey;
  }

  const res = await fetch(`${API_BASE_URL}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal,
  });

  if (!res.ok) {
    const errBody = await safeJson<ApiErrorBody>(res);
    const code = errBody?.code ?? `HTTP_${res.status}`;
    const message = errBody?.message ?? res.statusText ?? 'Request failed';
    throw new ApiError(res.status, code, message, errBody);
  }

  // 204 / empty body.
  if (res.status === 204) {
    return undefined as T;
  }
  const data = await safeJson<T>(res);
  return data as T;
}

async function safeJson<T>(res: Response): Promise<T | null> {
  const text = await res.text();
  if (!text) {
    return null;
  }
  try {
    return JSON.parse(text) as T;
  } catch {
    return null;
  }
}

function encode(segment: string): string {
  return encodeURIComponent(segment);
}

// --- Auth -----------------------------------------------------------------

/** Body for POST /api/v1/auth/login. */
export interface LoginRequest {
  email: string;
  password: string;
}

/**
 * Response to POST /api/v1/auth/login (auth.LoginResult subset). The refresh
 * token is delivered as an HttpOnly cookie, never in the JSON body.
 */
export interface LoginResult {
  accessToken: string;
  expiresIn?: number;
  user?: {
    id: string;
    email: string;
    fullName: string;
  };
}

/**
 * Authenticates staff against the platform auth endpoint and persists the
 * returned bearer token in Local_Store (session.ts). Mirrors the web app's
 * convention (apps/web/src/lib/auth.ts): POST {email,password} and read the
 * `accessToken` field. `credentials: 'include'` lets the server set the
 * HttpOnly refresh cookie.
 * POST /api/v1/auth/login
 */
export async function login(body: LoginRequest, signal?: AbortSignal): Promise<LoginResult> {
  const res = await fetch(`${API_BASE_URL}/api/v1/auth/login`, {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    credentials: 'include',
    body: JSON.stringify(body),
    signal,
  });

  if (!res.ok) {
    const errBody = await safeJson<ApiErrorBody>(res);
    const code = errBody?.code ?? `HTTP_${res.status}`;
    const message = errBody?.message ?? res.statusText ?? 'Login failed';
    throw new ApiError(res.status, code, message, errBody);
  }

  const data = (await safeJson<LoginResult>(res)) ?? ({} as LoginResult);
  if (data.accessToken) {
    setSessionToken(data.accessToken);
  }
  return data;
}

// --- Scanner API surface --------------------------------------------------

/**
 * Validate a scanned QR token against the selected event and return the
 * participant display info plus duplicate flags.
 * POST /api/v1/organizations/{orgId}/events/{eventId}/scan/verify
 */
export function verify(
  orgId: string,
  eventId: string,
  qrToken: string,
  signal?: AbortSignal,
): Promise<VerifyResult> {
  return request<VerifyResult>(
    `/api/v1/organizations/${encode(orgId)}/events/${encode(eventId)}/scan/verify`,
    { method: 'POST', body: { qrToken }, signal },
  );
}

/**
 * Idempotently check a participant in (VALID -> USED). The `idempotencyKey`
 * lets the sync engine replay the same operation exactly once.
 * POST /api/v1/organizations/{orgId}/events/{eventId}/scan/check-in
 */
export function checkIn(
  orgId: string,
  eventId: string,
  body: CheckInRequest,
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<CheckInResult> {
  return request<CheckInResult>(
    `/api/v1/organizations/${encode(orgId)}/events/${encode(eventId)}/scan/check-in`,
    { method: 'POST', body, idempotencyKey, signal },
  );
}

/**
 * List the caller's Permitted_Events across every org they belong to.
 * GET /api/v1/scan/events
 */
export function listPermittedEvents(signal?: AbortSignal): Promise<ListPermittedEventsResult> {
  return request<ListPermittedEventsResult>('/api/v1/scan/events', { signal });
}

/**
 * Replay an offline race-pack pickup through the existing racepack endpoint.
 * POST /api/v1/organizations/{orgId}/events/{eventId}/racepack/pickups
 */
export function pickup(
  orgId: string,
  eventId: string,
  body: PickupRequest,
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<PickupResult> {
  return request<PickupResult>(
    `/api/v1/organizations/${encode(orgId)}/events/${encode(eventId)}/racepack/pickups`,
    { method: 'POST', body, idempotencyKey, signal },
  );
}

export const api = {
  baseUrl: API_BASE_URL,
  login,
  verify,
  checkIn,
  listPermittedEvents,
  pickup,
};
