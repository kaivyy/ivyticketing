// Offline queue + cached ticket state (IndexedDB via `idb`).
//
// This module is the client's Local_Store for the Scanner PWA (design
// "Client: Offline_Queue" data model). It owns two responsibilities:
//
//   1. Persistence (task 10.1): two IndexedDB object stores — `operations`
//      (queued Scan_Operations, keyPath `idempotencyKey`) and `ticketState`
//      (cached per-ticket pickup/check-in flags, keyPath `ticketId`) — plus
//      CRUD helpers and a startup restore that reloads all non-FAILED ops back
//      into the in-memory queue (Req 7.2, 7.3, 7.6).
//
//   2. Enqueue path (tasks 10.2 + 10.3): pure structural validation that
//      mirrors the SERVER's `qr.DecodeStructure` contract (design D1) followed
//      by offline duplicate detection against cached ticket state, then enqueue
//      with a fresh UUID v4 Idempotency-Key (Req 7.1, 7.2, 7.4, 7.5).
//
// The HMAC signature is NEVER checked here — the secret is server-only (D1).
// Offline acceptance is provisional; the server re-verifies the HMAC at sync.
//
// `validateStructure` is deliberately pure (no IndexedDB) so the property tests
// (task 10.4) can exercise it in isolation.

import { openDB, deleteDB } from 'idb';
import type { DBSchema, IDBPDatabase } from 'idb';

// ---------------------------------------------------------------------------
// Data model (design "Client: Offline_Queue")
// ---------------------------------------------------------------------------

/** Kind of scan operation queued for sync. */
export type ScanKind = 'PICKUP' | 'CHECKIN';

/** Lifecycle status of a queued operation. */
export type OpStatus = 'PENDING' | 'SYNCING' | 'FAILED';

/**
 * A provisional scan captured (online or offline) and queued for exactly-once
 * replay to the server. The raw `qrToken` is retained so the server can
 * re-verify the HMAC at sync time.
 */
export interface ScanOperation {
  /** Client-generated UUID v4 used as the server Idempotency-Key (Req 7.2). */
  idempotencyKey: string;
  kind: ScanKind;
  eventId: string;
  ticketId: string;
  /** Raw token; the server re-verifies its HMAC at sync (design D1). */
  qrToken: string;
  counterId?: string; // pickups
  slotId?: string; // pickups (optional)
  /** ISO8601 original scan time, preserved through sync (Req 10.3). */
  scannedAt: string;
  status: OpStatus;
  attempts: number;
  lastError?: string;
}

/**
 * Cached record of a ticket's processed state, used for offline duplicate
 * detection before enqueue (Req 7.4).
 */
export interface CachedTicketState {
  ticketId: string;
  eventId: string;
  pickedUp: boolean;
  checkedIn: boolean;
  updatedAt: string; // ISO8601
}

// ---------------------------------------------------------------------------
// IndexedDB schema + connection management
// ---------------------------------------------------------------------------

interface ScannerDBSchema extends DBSchema {
  operations: { key: string; value: ScanOperation };
  ticketState: { key: string; value: CachedTicketState };
}

export const DB_NAME = 'ivy-scanner';
export const DB_VERSION = 1;

export type ScannerDB = IDBPDatabase<ScannerDBSchema>;

/** Opens (creating/upgrading as needed) the scanner IndexedDB database. */
export function openScannerDB(): Promise<ScannerDB> {
  return openDB<ScannerDBSchema>(DB_NAME, DB_VERSION, {
    upgrade(db) {
      if (!db.objectStoreNames.contains('operations')) {
        db.createObjectStore('operations', { keyPath: 'idempotencyKey' });
      }
      if (!db.objectStoreNames.contains('ticketState')) {
        db.createObjectStore('ticketState', { keyPath: 'ticketId' });
      }
    },
  });
}

// A process-wide handle is lazily opened for app usage. Tests can call
// `resetScannerDB()` to simulate an application restart (the underlying
// IndexedDB data persists) or `deleteScannerDB()` for full isolation.
let dbPromise: Promise<ScannerDB> | null = null;

/** Returns the shared database handle, opening it on first use. */
export function getDB(): Promise<ScannerDB> {
  if (!dbPromise) {
    dbPromise = openScannerDB();
  }
  return dbPromise;
}

/**
 * Closes the shared handle and forgets it. The persisted data survives; the
 * next `getDB()` reopens it. Useful for simulating an app restart in tests.
 */
export async function resetScannerDB(): Promise<void> {
  if (dbPromise) {
    const db = await dbPromise;
    db.close();
    dbPromise = null;
  }
}

/** Deletes the entire database (test isolation helper). */
export async function deleteScannerDB(): Promise<void> {
  await resetScannerDB();
  await deleteDB(DB_NAME);
}

// ---------------------------------------------------------------------------
// operations store CRUD
// ---------------------------------------------------------------------------

/** Persists (inserts or replaces) a queued operation. */
export async function enqueue(op: ScanOperation): Promise<void> {
  const db = await getDB();
  await db.put('operations', op);
}

/** Returns every queued operation. */
export async function listOperations(): Promise<ScanOperation[]> {
  const db = await getDB();
  return db.getAll('operations');
}

/** Returns a single operation by its Idempotency-Key, or undefined. */
export async function getOperation(idempotencyKey: string): Promise<ScanOperation | undefined> {
  const db = await getDB();
  return db.get('operations', idempotencyKey);
}

/** Removes an operation from the queue (e.g. after a successful sync). */
export async function removeOperation(idempotencyKey: string): Promise<void> {
  const db = await getDB();
  await db.delete('operations', idempotencyKey);
}

/**
 * Updates an operation's status (and optional `attempts`/`lastError`) in place.
 * Returns the updated operation, or undefined if it no longer exists.
 */
export async function updateStatus(
  idempotencyKey: string,
  status: OpStatus,
  patch: { attempts?: number; lastError?: string } = {},
): Promise<ScanOperation | undefined> {
  const db = await getDB();
  const existing = await db.get('operations', idempotencyKey);
  if (!existing) {
    return undefined;
  }
  const updated: ScanOperation = { ...existing, status };
  if (patch.attempts !== undefined) {
    updated.attempts = patch.attempts;
  }
  if (patch.lastError !== undefined) {
    updated.lastError = patch.lastError;
  }
  await db.put('operations', updated);
  return updated;
}

/**
 * Restores all non-FAILED operations into the in-memory queue on startup
 * (Req 7.6). FAILED ops are excluded because they need manual resolution and
 * must not be auto-replayed.
 */
export async function restoreQueue(): Promise<ScanOperation[]> {
  const ops = await listOperations();
  return ops.filter((op) => op.status !== 'FAILED');
}

/** Number of operations currently in the PENDING state (Req 7.5). */
export async function pendingCount(): Promise<number> {
  const ops = await listOperations();
  return ops.reduce((n, op) => (op.status === 'PENDING' ? n + 1 : n), 0);
}

// ---------------------------------------------------------------------------
// ticketState store
// ---------------------------------------------------------------------------

/** Reads the cached processed-state for a ticket, or undefined. */
export async function getTicketState(ticketId: string): Promise<CachedTicketState | undefined> {
  const db = await getDB();
  return db.get('ticketState', ticketId);
}

/** Writes (inserts or replaces) the cached processed-state for a ticket. */
export async function putTicketState(state: CachedTicketState): Promise<void> {
  const db = await getDB();
  await db.put('ticketState', state);
}

// ---------------------------------------------------------------------------
// Structural validation (pure — mirrors server qr.DecodeStructure, design D1)
// ---------------------------------------------------------------------------

/** Supported QR schema version (server `qr.CurrentVersion`). */
export const SUPPORTED_QR_VERSION = 1;

/** Reason a token was rejected by structural validation. */
export type StructuralErrorKind =
  | 'MALFORMED' // wrong segment count, bad base64url, bad JSON, unparseable ids
  | 'UNSUPPORTED_VERSION' // version segment absent or not a supported version
  | 'EVENT_MISMATCH'; // event_id does not match the selected Permitted_Event

/**
 * Discriminated result of {@link validateStructure}. On success it carries the
 * parsed identifiers; on failure it carries the rejection `kind`.
 */
export type StructuralValidation =
  | { ok: true; ticketId: string; eventId: string; version: number }
  | { ok: false; kind: StructuralErrorKind };

// Canonical UUID (8-4-4-4-12); matches the form emitted by the server's
// uuid.String() and carried in signed tokens.
const UUID_RE = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;
const INT_RE = /^[+-]?\d+$/;

function isUuid(value: unknown): value is string {
  return typeof value === 'string' && UUID_RE.test(value);
}

/**
 * Decodes a base64url (RawURLEncoding, i.e. no padding) segment into bytes,
 * returning null when the input is not valid base64url. Mirrors Go's
 * base64.RawURLEncoding: url alphabet, no `=` padding, and a length remainder
 * of 1 (mod 4) is invalid.
 */
function base64UrlDecode(segment: string): Uint8Array | null {
  if (!/^[A-Za-z0-9_-]*$/.test(segment)) {
    return null;
  }
  const remainder = segment.length % 4;
  if (remainder === 1) {
    return null;
  }
  let b64 = segment.replace(/-/g, '+').replace(/_/g, '/');
  if (remainder !== 0) {
    b64 += '='.repeat(4 - remainder);
  }
  try {
    const binary = atob(b64);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) {
      bytes[i] = binary.charCodeAt(i);
    }
    return bytes;
  } catch {
    return null;
  }
}

/**
 * Pure structural validation of a QR token against the selected event. Mirrors
 * the server `qr.DecodeStructure` contract exactly (design D1): the token MUST
 * have three dot-separated segments, a supported version, a base64url-decodable
 * JSON payload with parseable `tid`/`eid` UUIDs, and `eid` MUST equal the
 * selected event. It performs NO HMAC verification (the secret is server-only).
 *
 * Validation order matches the server (version is checked before the payload is
 * decoded) so rejection reasons line up with the authoritative contract.
 */
export function validateStructure(token: string, selectedEventId: string): StructuralValidation {
  const parts = token.split('.');
  if (parts.length !== 3) {
    return { ok: false, kind: 'MALFORMED' };
  }

  const [verSeg, payloadSeg] = parts;

  // Version segment first (matches server decodePayload ordering).
  const version = INT_RE.test(verSeg) ? Number.parseInt(verSeg, 10) : Number.NaN;
  if (version !== SUPPORTED_QR_VERSION) {
    return { ok: false, kind: 'UNSUPPORTED_VERSION' };
  }

  const bytes = base64UrlDecode(payloadSeg);
  if (!bytes) {
    return { ok: false, kind: 'MALFORMED' };
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(new TextDecoder().decode(bytes));
  } catch {
    return { ok: false, kind: 'MALFORMED' };
  }
  if (typeof parsed !== 'object' || parsed === null) {
    return { ok: false, kind: 'MALFORMED' };
  }

  const { tid, eid } = parsed as { tid?: unknown; eid?: unknown };
  if (!isUuid(tid) || !isUuid(eid)) {
    return { ok: false, kind: 'MALFORMED' };
  }

  if (eid !== selectedEventId) {
    return { ok: false, kind: 'EVENT_MISMATCH' };
  }

  return { ok: true, ticketId: tid, eventId: eid, version };
}

// ---------------------------------------------------------------------------
// Enqueue path (validation + duplicate detection + enqueue)
// ---------------------------------------------------------------------------

/** Input to {@link enqueueScan}. */
export interface EnqueueScanInput {
  token: string;
  selectedEventId: string;
  kind: ScanKind;
  /** ISO8601 original scan time; defaults to now (Req 10.3). */
  scannedAt?: string;
  counterId?: string; // pickups
  slotId?: string; // pickups
}

/**
 * Outcome of {@link enqueueScan}:
 *   - `ENQUEUED`  — accepted and persisted to the queue.
 *   - `DUPLICATE` — a Duplicate_Warning: cached state already records this
 *     action for the ticket, so no second op is enqueued (Req 7.4).
 *   - `REJECTED`  — failed structural validation; not enqueued (Req 7.1).
 */
export type EnqueueOutcome =
  | { status: 'ENQUEUED'; operation: ScanOperation }
  | { status: 'DUPLICATE'; ticketId: string; kind: ScanKind }
  | { status: 'REJECTED'; kind: StructuralErrorKind };

/**
 * Validates a scanned token, checks cached ticket state for a duplicate, and —
 * when accepted and not a duplicate — enqueues a Scan_Operation with a fresh
 * UUID v4 Idempotency-Key. Rejected or duplicate scans are NOT enqueued.
 */
export async function enqueueScan(input: EnqueueScanInput): Promise<EnqueueOutcome> {
  const validation = validateStructure(input.token, input.selectedEventId);
  if (!validation.ok) {
    return { status: 'REJECTED', kind: validation.kind };
  }

  // Offline duplicate detection against cached ticket state (Req 7.4).
  const cached = await getTicketState(validation.ticketId);
  if (cached) {
    if (input.kind === 'PICKUP' && cached.pickedUp) {
      return { status: 'DUPLICATE', ticketId: validation.ticketId, kind: 'PICKUP' };
    }
    if (input.kind === 'CHECKIN' && cached.checkedIn) {
      return { status: 'DUPLICATE', ticketId: validation.ticketId, kind: 'CHECKIN' };
    }
  }

  const operation: ScanOperation = {
    idempotencyKey: crypto.randomUUID(),
    kind: input.kind,
    eventId: validation.eventId,
    ticketId: validation.ticketId,
    qrToken: input.token,
    scannedAt: input.scannedAt ?? new Date().toISOString(),
    status: 'PENDING',
    attempts: 0,
  };
  if (input.counterId !== undefined) {
    operation.counterId = input.counterId;
  }
  if (input.slotId !== undefined) {
    operation.slotId = input.slotId;
  }

  await enqueue(operation);
  return { status: 'ENQUEUED', operation };
}
