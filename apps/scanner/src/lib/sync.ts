// Sync_Engine — drains the offline queue to the Scan_API with exactly-once
// semantics (design "lib/sync.ts", Correctness Property 17).
//
// Responsibilities (task 11.1):
//   - On connectivity restore, transmit every PENDING Scan_Operation to the
//     Scan_API, each carrying its client-generated Idempotency-Key (Req 8.1,
//     8.2).
//   - Classify every response against the design Error Handling table:
//       * 2xx / cached idempotent response, and 409 ALREADY_* (already checked
//         in / already picked up) -> DRAIN: the effect is (or was) applied, so
//         remove the op from the queue + Local_Store (Req 8.4).
//       * network error / 5xx / timeout -> RETAIN: keep the op PENDING for a
//         later retry (Req 8.5).
//       * other non-retryable 4xx (forged QR, event mismatch, cancelled,
//         idempotency conflict, ...) -> FAIL: move the op to FAILED and surface
//         it for manual resolution (Req 8.6).
//   - Expose a SyncState store ({ online, pending, failed }) that the
//     OfflineSyncStatus UI (task 13) subscribes to.
//
// The transport is INJECTABLE (a small `SyncTransport` interface): the real
// implementation calls `api.checkIn` / `api.pickup` and maps `ApiError` onto a
// classification, while tests supply a mock transport so no real network is
// touched (design Testing Strategy, Property 17).
//
// This module is deliberately framework-light (a plain class + a subscribe
// callback, no Svelte imports) so it is trivially unit- and property-testable.

import { ApiError, type CheckInResult, type PickupResult } from './api';
import {
  listOperations,
  removeOperation,
  updateStatus,
  getTicketState,
  putTicketState,
  type ScanOperation,
  type CachedTicketState,
} from './offline-db';

// ---------------------------------------------------------------------------
// Response classification (design Error Handling table)
// ---------------------------------------------------------------------------

/**
 * How the Sync_Engine treats a transmitted operation's outcome:
 *   - `DRAIN`  — success (2xx / cached) or a resolved duplicate (409 ALREADY_*);
 *     remove the op from the queue + Local_Store.
 *   - `RETAIN` — transient failure (network error / 5xx / timeout); keep the op
 *     PENDING for a later retry.
 *   - `FAIL`   — non-retryable rejection (other 4xx); move the op to FAILED and
 *     surface it for manual resolution.
 */
export type SyncClassification = 'DRAIN' | 'RETAIN' | 'FAIL';

/** The outcome of transmitting a single {@link ScanOperation}. */
export interface SendResult {
  classification: SyncClassification;
  /** Human-readable detail recorded on the op (RETAIN/FAIL) for diagnostics. */
  error?: string;
}

/**
 * The injectable transport the Sync_Engine drives. Implementations transmit a
 * single operation (including its Idempotency-Key) and return how the engine
 * should treat the result. Implementations MUST NOT throw for expected server
 * or network conditions — they classify instead; an unexpected throw is
 * defensively treated as RETAIN by the engine.
 */
export interface SyncTransport {
  send(op: ScanOperation): Promise<SendResult>;
}

// ---------------------------------------------------------------------------
// SyncState store (design "lib/sync.ts")
// ---------------------------------------------------------------------------

/** Snapshot the OfflineSyncStatus component renders (Req 7.5, 8.6). */
export interface SyncState {
  online: boolean;
  /** Number of PENDING operations still awaiting sync. */
  pending: number;
  /** Non-retryable operations needing manual resolution. */
  failed: ScanOperation[];
}

/** Subscriber invoked with the current {@link SyncState} on every change. */
export type SyncListener = (state: SyncState) => void;

// ---------------------------------------------------------------------------
// Real API transport
// ---------------------------------------------------------------------------

/** Collaborators the API-backed transport needs (kept injectable for tests). */
export interface ApiTransportDeps {
  checkIn: (
    orgId: string,
    eventId: string,
    body: { ticketId: string; scannedAt?: string },
    idempotencyKey: string,
  ) => Promise<CheckInResult>;
  pickup: (
    orgId: string,
    eventId: string,
    body: { ticketId: string; counterId: string; slotId?: string; scannedAt?: string },
    idempotencyKey: string,
  ) => Promise<PickupResult>;
  /**
   * Resolves the organization a queued op belongs to. The scan endpoints are
   * mounted under `/organizations/{orgId}/events/{eventId}`, but a
   * ScanOperation only carries its `eventId`; the app supplies the mapping from
   * the Permitted_Events it already loaded.
   */
  resolveOrgId: (op: ScanOperation) => string;
}

/**
 * Maps an {@link ApiError} (or a thrown network error) onto a
 * {@link SyncClassification}, per the design Error Handling table.
 */
export function classifyFailure(err: unknown): SendResult {
  if (err instanceof ApiError) {
    // Resolved duplicate: the server already has the effect recorded.
    if (
      err.status === 409 &&
      (err.code === 'ALREADY_CHECKED_IN' || err.code === 'ALREADY_PICKED_UP')
    ) {
      return { classification: 'DRAIN', error: err.message };
    }
    // Server-side transient failure — safe to retry with the same key.
    if (err.status >= 500) {
      return { classification: 'RETAIN', error: err.message };
    }
    // Any other 4xx is a definitive, non-retryable rejection.
    return { classification: 'FAIL', error: `${err.code}: ${err.message}` };
  }
  // Not an ApiError => the request never got a response (network error /
  // timeout / offline). Retain for retry.
  return {
    classification: 'RETAIN',
    error: err instanceof Error ? err.message : String(err),
  };
}

/**
 * Builds a {@link SyncTransport} backed by the real API client. Each op is
 * transmitted with its `idempotencyKey`; CHECKIN ops hit `/scan/check-in` and
 * PICKUP ops replay to `/racepack/pickups`, both preserving the original
 * `scannedAt` (Req 10.3). A 2xx resolves to DRAIN; failures are classified by
 * {@link classifyFailure}.
 */
export function createApiTransport(deps: ApiTransportDeps): SyncTransport {
  return {
    async send(op: ScanOperation): Promise<SendResult> {
      const orgId = deps.resolveOrgId(op);
      try {
        if (op.kind === 'CHECKIN') {
          await deps.checkIn(
            orgId,
            op.eventId,
            { ticketId: op.ticketId, scannedAt: op.scannedAt },
            op.idempotencyKey,
          );
        } else {
          await deps.pickup(
            orgId,
            op.eventId,
            {
              ticketId: op.ticketId,
              counterId: op.counterId ?? '',
              slotId: op.slotId,
              scannedAt: op.scannedAt,
            },
            op.idempotencyKey,
          );
        }
        return { classification: 'DRAIN' };
      } catch (err) {
        return classifyFailure(err);
      }
    },
  };
}

// ---------------------------------------------------------------------------
// Sync engine
// ---------------------------------------------------------------------------

/** Minimal window surface the engine wires connectivity listeners onto. */
export interface ConnectivityHost {
  addEventListener(type: 'online' | 'offline', listener: () => void): void;
  removeEventListener(type: 'online' | 'offline', listener: () => void): void;
  navigatorOnLine?: () => boolean;
}

/**
 * Drives the offline queue: subscribable {@link SyncState}, a {@link syncNow}
 * run that drains/retains/fails each PENDING op, and optional wiring to the
 * browser's `online`/`offline` events so a run kicks off automatically when
 * connectivity returns.
 */
export class SyncEngine {
  #state: SyncState = { online: true, pending: 0, failed: [] };
  readonly #listeners = new Set<SyncListener>();
  readonly #defaultTransport?: SyncTransport;
  #syncing = false;

  #host?: ConnectivityHost;
  #onOnline?: () => void;
  #onOffline?: () => void;

  constructor(defaultTransport?: SyncTransport) {
    this.#defaultTransport = defaultTransport;
  }

  /** Current snapshot (read-only copy). */
  getState(): SyncState {
    return { ...this.#state, failed: [...this.#state.failed] };
  }

  /**
   * Subscribes to state changes. The listener is invoked immediately with the
   * current state and on every subsequent change. Returns an unsubscribe fn.
   */
  subscribe(listener: SyncListener): () => void {
    this.#listeners.add(listener);
    listener(this.getState());
    return () => {
      this.#listeners.delete(listener);
    };
  }

  #emit(): void {
    const snapshot = this.getState();
    for (const listener of this.#listeners) {
      listener(snapshot);
    }
  }

  /** Updates the cached online flag and notifies subscribers. */
  setOnline(online: boolean): void {
    if (this.#state.online !== online) {
      this.#state = { ...this.#state, online };
      this.#emit();
    }
  }

  /**
   * Recomputes `pending`/`failed` from Local_Store and notifies subscribers.
   * Called after every sync run so the UI reflects the queue exactly (Req 7.5,
   * 8.6).
   */
  async refresh(): Promise<void> {
    const ops = await listOperations();
    this.#state = {
      ...this.#state,
      pending: ops.filter((op) => op.status === 'PENDING').length,
      failed: ops.filter((op) => op.status === 'FAILED'),
    };
    this.#emit();
  }

  /**
   * Drains the queue once: transmits every PENDING operation exactly once
   * (carrying its Idempotency-Key) and applies the classified outcome —
   * DRAIN removes the op (and marks the ticket processed in cached state so
   * later offline scans detect the duplicate), RETAIN keeps it PENDING, FAIL
   * moves it to FAILED. Concurrent runs are coalesced (a run in progress is a
   * no-op). Returns the per-op classifications for the run.
   */
  async syncNow(
    transport: SyncTransport | undefined = this.#defaultTransport,
  ): Promise<SendResult[]> {
    if (!transport) {
      throw new Error('SyncEngine.syncNow: no transport provided');
    }
    if (this.#syncing) {
      return [];
    }
    this.#syncing = true;
    const results: SendResult[] = [];
    try {
      const ops = await listOperations();
      const pending = ops.filter((op) => op.status === 'PENDING');

      for (const op of pending) {
        let result: SendResult;
        try {
          result = await transport.send(op);
        } catch (err) {
          // A transport that throws unexpectedly is treated as transient.
          result = classifyFailure(err);
        }
        results.push(result);

        switch (result.classification) {
          case 'DRAIN':
            await this.#markProcessed(op);
            await removeOperation(op.idempotencyKey);
            break;
          case 'RETAIN':
            await updateStatus(op.idempotencyKey, 'PENDING', {
              attempts: op.attempts + 1,
              lastError: result.error,
            });
            break;
          case 'FAIL':
            await updateStatus(op.idempotencyKey, 'FAILED', {
              attempts: op.attempts + 1,
              lastError: result.error,
            });
            break;
        }
      }
    } finally {
      this.#syncing = false;
    }

    await this.refresh();
    return results;
  }

  /**
   * Wires `online`/`offline` listeners on the host (defaults to `window`). On
   * `online` the engine flips the flag and kicks off a sync run with the
   * default transport. Call {@link stop} to tear the listeners down.
   */
  start(host?: ConnectivityHost): void {
    const target =
      host ?? (typeof window !== 'undefined' ? toConnectivityHost(window) : undefined);
    if (!target) {
      return;
    }
    this.stop();
    this.#host = target;

    this.#onOnline = () => {
      this.setOnline(true);
      void this.syncNow().catch(() => {
        /* surfaced through op state; never throw from the listener */
      });
    };
    this.#onOffline = () => {
      this.setOnline(false);
    };

    target.addEventListener('online', this.#onOnline);
    target.addEventListener('offline', this.#onOffline);
    this.setOnline(target.navigatorOnLine ? target.navigatorOnLine() : true);
  }

  /** Removes previously-wired connectivity listeners. */
  stop(): void {
    if (this.#host && this.#onOnline && this.#onOffline) {
      this.#host.removeEventListener('online', this.#onOnline);
      this.#host.removeEventListener('offline', this.#onOffline);
    }
    this.#host = undefined;
    this.#onOnline = undefined;
    this.#onOffline = undefined;
  }

  /**
   * Marks the ticket processed in cached state after a successful drain so a
   * subsequent offline scan of the same ticket is detected as a duplicate
   * (Req 7.4). Preserves the other action's flag.
   */
  async #markProcessed(op: ScanOperation): Promise<void> {
    const existing = await getTicketState(op.ticketId);
    const state: CachedTicketState = {
      ticketId: op.ticketId,
      eventId: op.eventId,
      pickedUp: existing?.pickedUp ?? false,
      checkedIn: existing?.checkedIn ?? false,
      updatedAt: new Date().toISOString(),
    };
    if (op.kind === 'PICKUP') {
      state.pickedUp = true;
    } else {
      state.checkedIn = true;
    }
    await putTicketState(state);
  }
}

/** Adapts a DOM `Window` to the minimal {@link ConnectivityHost} surface. */
function toConnectivityHost(win: Window): ConnectivityHost {
  return {
    addEventListener: (type, listener) => win.addEventListener(type, listener),
    removeEventListener: (type, listener) => win.removeEventListener(type, listener),
    navigatorOnLine: () => win.navigator.onLine,
  };
}

/**
 * Process-wide engine the app wires up (task 13). Construct with the real API
 * transport at bootstrap via {@link createApiTransport} and call `start()`.
 */
export const syncEngine = new SyncEngine();
