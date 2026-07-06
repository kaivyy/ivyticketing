// Property-based tests for the offline queue + structural validation
// (`lib/offline-db.ts`). Covers design Correctness Properties 12–16 for the
// Scanner PWA (tasks 10.4–10.8).
//
// IndexedDB is provided by `fake-indexeddb/auto` (preloaded in
// `src/test/setup.ts`). Because a single fast-check property runs many
// iterations inside one Vitest test, each predicate resets the database up
// front (`deleteScannerDB()`) so iterations never leak state into each other.
// Every property runs a minimum of 100 iterations.

import { describe, it, beforeEach, expect } from 'vitest';
import fc from 'fast-check';
import {
  validateStructure,
  enqueueScan,
  enqueue,
  listOperations,
  restoreQueue,
  pendingCount,
  putTicketState,
  resetScannerDB,
  deleteScannerDB,
  SUPPORTED_QR_VERSION,
  type ScanOperation,
  type OpStatus,
  type ScanKind,
  type StructuralErrorKind,
} from './offline-db';

const NUM_RUNS = 100;

// ---------------------------------------------------------------------------
// Token construction helpers (mirror the server's signed-token shape:
// `<version>.<base64url(payload)>.<signature>`; the signature segment is
// irrelevant to structural validation, which never inspects it).
// ---------------------------------------------------------------------------

const textEncoder = new TextEncoder();

/** base64url (RawURLEncoding: url alphabet, no `=` padding). */
function base64UrlEncode(bytes: Uint8Array): string {
  let binary = '';
  for (let i = 0; i < bytes.length; i += 1) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function encodePayloadObject(obj: unknown): string {
  return base64UrlEncode(textEncoder.encode(JSON.stringify(obj)));
}

function encodeRaw(raw: string): string {
  return base64UrlEncode(textEncoder.encode(raw));
}

/** Builds a well-formed, signed-shaped token for `{tid, eid, v:1}`. */
function buildValidToken(tid: string, eid: string, sig: string): string {
  return `${SUPPORTED_QR_VERSION}.${encodePayloadObject({ tid, eid, v: SUPPORTED_QR_VERSION })}.${sig}`;
}

// A dot-free, non-empty segment (a signature segment or a generic segment for
// wrong-segment-count tokens). Dots are replaced so segment counts stay exact.
const segmentArb = fc
  .string({ minLength: 1, maxLength: 12 })
  .map((s) => s.replace(/\./g, 'x'));

// ---------------------------------------------------------------------------
// Property 12 — Offline structural validation (task 10.4)
// ---------------------------------------------------------------------------

type ExpectedOutcome =
  | { ok: true; tid: string; eid: string }
  | { ok: false; kind: StructuralErrorKind };

interface StructuralCase {
  token: string;
  selectedEventId: string;
  expected: ExpectedOutcome;
}

// Accepted: version 1, valid base64url payload with uuid tid/eid, eid matches.
const acceptedCase = fc
  .record({ tid: fc.uuid(), eid: fc.uuid(), sig: segmentArb })
  .map(({ tid, eid, sig }): StructuralCase => ({
    token: buildValidToken(tid, eid, sig),
    selectedEventId: eid,
    expected: { ok: true, tid, eid },
  }));

// Event mismatch: well-formed token whose eid differs from the selected event.
const eventMismatchCase = fc
  .record({ tid: fc.uuid(), eid: fc.uuid(), selected: fc.uuid(), sig: segmentArb })
  .filter(({ eid, selected }) => eid !== selected)
  .map(({ tid, eid, selected, sig }): StructuralCase => ({
    token: buildValidToken(tid, eid, sig),
    selectedEventId: selected,
    expected: { ok: false, kind: 'EVENT_MISMATCH' },
  }));

// Wrong segment count: N dot-free segments joined, N !== 3.
const wrongSegmentCountCase = fc
  .record({
    parts: fc
      .array(segmentArb, { minLength: 1, maxLength: 6 })
      .filter((parts) => parts.length !== 3),
    selected: fc.uuid(),
  })
  .map(({ parts, selected }): StructuralCase => ({
    token: parts.join('.'),
    selectedEventId: selected,
    expected: { ok: false, kind: 'MALFORMED' },
  }));

// Bad base64url payload: version 1 but the payload segment carries a character
// outside the base64url alphabet, so it cannot be decoded.
const badBase64Case = fc
  .record({ junk: segmentArb, sig: segmentArb, selected: fc.uuid() })
  .map(({ junk, sig, selected }): StructuralCase => ({
    token: `${SUPPORTED_QR_VERSION}.${junk}%.${sig}`,
    selectedEventId: selected,
    expected: { ok: false, kind: 'MALFORMED' },
  }));

// Bad payload content: version 1, valid base64url, but the decoded JSON is not
// an object carrying uuid tid/eid (invalid JSON, non-object JSON, or non-uuid
// ids). All of these reduce to MALFORMED.
const badPayloadContentCase = fc
  .record({
    payloadSeg: fc.oneof(
      // Non-uuid string ids.
      fc
        .record({ tid: fc.string(), eid: fc.string() })
        .filter(({ tid, eid }) => !isUuidLike(tid) || !isUuidLike(eid))
        .map((o) => encodePayloadObject(o)),
      // JSON that is not an object.
      fc.integer().map((n) => encodePayloadObject(n)),
      fc.string().map((s) => encodePayloadObject(s)),
      fc.constant(encodePayloadObject([1, 2, 3])),
      fc.constant(encodePayloadObject(null)),
      // Structurally-valid base64url that is not valid JSON at all.
      fc.constant(encodeRaw('{not valid json')),
      fc.constant(encodeRaw('plain text, definitely not json')),
    ),
    sig: segmentArb,
    selected: fc.uuid(),
  })
  .map(({ payloadSeg, sig, selected }): StructuralCase => ({
    token: `${SUPPORTED_QR_VERSION}.${payloadSeg}.${sig}`,
    selectedEventId: selected,
    expected: { ok: false, kind: 'MALFORMED' },
  }));

// Unsupported version: a version segment that is not the supported version,
// with an otherwise-valid payload (version is checked before the payload).
const unsupportedVersionCase = fc
  .record({
    ver: fc.constantFrom('0', '2', '3', '-1', '99', 'abc', ''),
    tid: fc.uuid(),
    eid: fc.uuid(),
    sig: segmentArb,
    selected: fc.uuid(),
  })
  .map(({ ver, tid, eid, sig, selected }): StructuralCase => ({
    token: `${ver}.${encodePayloadObject({ tid, eid, v: 1 })}.${sig}`,
    selectedEventId: selected,
    expected: { ok: false, kind: 'UNSUPPORTED_VERSION' },
  }));

const UUID_RE =
  /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;
function isUuidLike(value: string): boolean {
  return UUID_RE.test(value);
}

const structuralCaseArb: fc.Arbitrary<StructuralCase> = fc.oneof(
  acceptedCase,
  eventMismatchCase,
  wrongSegmentCountCase,
  badBase64Case,
  badPayloadContentCase,
  unsupportedVersionCase,
);

describe('offline-db structural validation', () => {
  beforeEach(async () => {
    await deleteScannerDB();
  });

  // Feature: scanner-pwa, Property 12: Offline structural validation
  it('accepts a token iff structure/version/payload/event all hold, and only accepted tokens enqueue', async () => {
    await fc.assert(
      fc.asyncProperty(structuralCaseArb, async ({ token, selectedEventId, expected }) => {
        // Reset per iteration so the enqueue assertions never leak state.
        await deleteScannerDB();

        const result = validateStructure(token, selectedEventId);
        const outcome = await enqueueScan({ token, selectedEventId, kind: 'CHECKIN' });

        if (expected.ok) {
          expect(result.ok).toBe(true);
          if (result.ok) {
            expect(result.ticketId).toBe(expected.tid);
            expect(result.eventId).toBe(expected.eid);
            expect(result.version).toBe(SUPPORTED_QR_VERSION);
          }
          // Accepted tokens are enqueueable.
          expect(outcome.status).toBe('ENQUEUED');
          expect(await listOperations()).toHaveLength(1);
        } else {
          expect(result.ok).toBe(false);
          if (!result.ok) {
            expect(result.kind).toBe(expected.kind);
          }
          // Rejected tokens are not enqueued.
          expect(outcome.status).toBe('REJECTED');
          expect(await listOperations()).toHaveLength(0);
        }
      }),
      { numRuns: NUM_RUNS },
    );
  });
});

// ---------------------------------------------------------------------------
// Property 13 — Unique idempotency key per queued operation (task 10.5)
// ---------------------------------------------------------------------------

const scanRequestArb = fc.record({
  tid: fc.uuid(),
  eid: fc.uuid(),
  kind: fc.constantFrom<ScanKind>('PICKUP', 'CHECKIN'),
  sig: segmentArb,
});

describe('offline-db idempotency key uniqueness', () => {
  beforeEach(async () => {
    await deleteScannerDB();
  });

  // Feature: scanner-pwa, Property 13: Unique idempotency key per queued operation
  it('assigns pairwise-distinct idempotency keys across an enqueued sequence', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.array(scanRequestArb, { minLength: 1, maxLength: 25 }),
        async (requests) => {
          await deleteScannerDB();

          let enqueued = 0;
          for (const req of requests) {
            const outcome = await enqueueScan({
              token: buildValidToken(req.tid, req.eid, req.sig),
              selectedEventId: req.eid,
              kind: req.kind,
            });
            // No cached state exists, so every valid scan enqueues.
            expect(outcome.status).toBe('ENQUEUED');
            enqueued += 1;
          }

          const ops = await listOperations();
          const keys = ops.map((op) => op.idempotencyKey);
          const distinct = new Set(keys);
          expect(distinct.size).toBe(keys.length);
          expect(keys.length).toBe(enqueued);
        },
      ),
      { numRuns: NUM_RUNS },
    );
  });
});

// ---------------------------------------------------------------------------
// Property 14 — Offline queue persistence round-trip (task 10.6)
// ---------------------------------------------------------------------------

const opStatusArb = fc.constantFrom<OpStatus>('PENDING', 'SYNCING', 'FAILED');

const scanOperationArb: fc.Arbitrary<ScanOperation> = fc.record({
  idempotencyKey: fc.uuid(),
  kind: fc.constantFrom<ScanKind>('PICKUP', 'CHECKIN'),
  eventId: fc.uuid(),
  ticketId: fc.uuid(),
  qrToken: fc.string(),
  scannedAt: fc
    .integer({ min: 0, max: 4_102_444_800_000 })
    .map((ms) => new Date(ms).toISOString()),
  status: opStatusArb,
  attempts: fc.nat({ max: 10 }),
});

function sortByKey(ops: ScanOperation[]): ScanOperation[] {
  return [...ops].sort((a, b) => a.idempotencyKey.localeCompare(b.idempotencyKey));
}

describe('offline-db persistence round-trip', () => {
  beforeEach(async () => {
    await deleteScannerDB();
  });

  // Feature: scanner-pwa, Property 14: Offline queue persistence round-trip
  it('restores exactly the non-FAILED operations after an app restart', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.uniqueArray(scanOperationArb, {
          selector: (op) => op.idempotencyKey,
          maxLength: 25,
        }),
        async (ops) => {
          // Full isolation for this iteration (clears persisted data).
          await deleteScannerDB();

          for (const op of ops) {
            await enqueue(op);
          }

          // Simulate an application restart: close the handle but KEEP the
          // persisted IndexedDB data (do not delete it).
          await resetScannerDB();

          const restored = await restoreQueue();
          const expected = ops.filter((op) => op.status !== 'FAILED');

          expect(sortByKey(restored)).toEqual(sortByKey(expected));
        },
      ),
      { numRuns: NUM_RUNS },
    );
  });
});

// ---------------------------------------------------------------------------
// Property 15 — Offline duplicate detection against cached state (task 10.7)
// ---------------------------------------------------------------------------

describe('offline-db duplicate detection', () => {
  beforeEach(async () => {
    await deleteScannerDB();
  });

  // Feature: scanner-pwa, Property 15: Offline duplicate detection against cached state
  it('returns DUPLICATE and enqueues nothing when cached state already records the action', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.record({
          tid: fc.uuid(),
          eid: fc.uuid(),
          kind: fc.constantFrom<ScanKind>('PICKUP', 'CHECKIN'),
          otherFlag: fc.boolean(),
          sig: segmentArb,
        }),
        async ({ tid, eid, kind, otherFlag, sig }) => {
          await deleteScannerDB();

          // Record the ticket as already processed for the scanned action.
          await putTicketState({
            ticketId: tid,
            eventId: eid,
            pickedUp: kind === 'PICKUP' ? true : otherFlag,
            checkedIn: kind === 'CHECKIN' ? true : otherFlag,
            updatedAt: new Date().toISOString(),
          });

          const outcome = await enqueueScan({
            token: buildValidToken(tid, eid, sig),
            selectedEventId: eid,
            kind,
          });

          expect(outcome.status).toBe('DUPLICATE');
          if (outcome.status === 'DUPLICATE') {
            expect(outcome.ticketId).toBe(tid);
            expect(outcome.kind).toBe(kind);
          }
          // No second operation is enqueued for the duplicate action.
          expect(await listOperations()).toHaveLength(0);
        },
      ),
      { numRuns: NUM_RUNS },
    );
  });
});

// ---------------------------------------------------------------------------
// Property 16 — Pending count reflects the queue (task 10.8)
// ---------------------------------------------------------------------------

describe('offline-db pending count', () => {
  beforeEach(async () => {
    await deleteScannerDB();
  });

  // Feature: scanner-pwa, Property 16: Pending count reflects the queue
  it('reports pendingCount equal to the number of PENDING operations', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.uniqueArray(scanOperationArb, {
          selector: (op) => op.idempotencyKey,
          maxLength: 30,
        }),
        async (ops) => {
          await deleteScannerDB();

          for (const op of ops) {
            await enqueue(op);
          }

          const expectedPending = ops.filter((op) => op.status === 'PENDING').length;
          expect(await pendingCount()).toBe(expectedPending);
        },
      ),
      { numRuns: NUM_RUNS },
    );
  });
});
