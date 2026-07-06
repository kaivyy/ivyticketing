// Property-based test for the Sync_Engine (`lib/sync.ts`). Covers design
// Correctness Property 17 for the Scanner PWA (task 11.2).
//
// The transport is fully injectable, so this test drives the engine with a
// mock transport (no real network). IndexedDB is provided by
// `fake-indexeddb/auto` (preloaded in `src/test/setup.ts`); each fast-check
// iteration resets the database so state never leaks between runs. The property
// runs a minimum of 100 iterations.

import { describe, it, beforeEach, expect } from 'vitest';
import fc from 'fast-check';
import {
  enqueue,
  listOperations,
  getTicketState,
  deleteScannerDB,
  type ScanOperation,
  type ScanKind,
} from './offline-db';
import { SyncEngine, type SyncTransport, type SendResult } from './sync';

const NUM_RUNS = 100;

// A PENDING operation as it would sit in the Offline_Queue awaiting sync.
const pendingOpArb: fc.Arbitrary<ScanOperation> = fc.record({
  idempotencyKey: fc.uuid(),
  kind: fc.constantFrom<ScanKind>('PICKUP', 'CHECKIN'),
  eventId: fc.uuid(),
  ticketId: fc.uuid(),
  qrToken: fc.string(),
  scannedAt: fc
    .integer({ min: 0, max: 4_102_444_800_000 })
    .map((ms) => new Date(ms).toISOString()),
  status: fc.constant('PENDING' as const),
  attempts: fc.nat({ max: 5 }),
});

const queueArb = fc.uniqueArray(pendingOpArb, {
  selector: (op) => op.idempotencyKey,
  minLength: 1,
  maxLength: 20,
});

/**
 * A mock transport that returns a fixed classification and records how many
 * times each Idempotency-Key was transmitted (and that the key was carried).
 */
function makeMockTransport(result: SendResult): {
  transport: SyncTransport;
  calls: Map<string, number>;
} {
  const calls = new Map<string, number>();
  const transport: SyncTransport = {
    async send(op) {
      // The op MUST carry its Idempotency-Key when transmitted (Req 8.2).
      expect(typeof op.idempotencyKey).toBe('string');
      expect(op.idempotencyKey.length).toBeGreaterThan(0);
      calls.set(op.idempotencyKey, (calls.get(op.idempotencyKey) ?? 0) + 1);
      return result;
    },
  };
  return { transport, calls };
}

async function seedQueue(ops: ScanOperation[]): Promise<void> {
  for (const op of ops) {
    await enqueue(op);
  }
}

describe('sync engine (Property 17)', () => {
  beforeEach(async () => {
    await deleteScannerDB();
  });

  // Feature: scanner-pwa, Property 17: Sync engine transmits, retains, and fails correctly
  it('drains on success, retains on network error, and fails on non-retryable rejection', async () => {
    await fc.assert(
      fc.asyncProperty(queueArb, async (ops) => {
        // -- Succeeding transport: every op transmitted exactly once (with its
        //    key), then removed from the queue + Local_Store; ticket state is
        //    marked processed.
        await deleteScannerDB();
        await seedQueue(ops);
        {
          const engine = new SyncEngine();
          const { transport, calls } = makeMockTransport({ classification: 'DRAIN' });
          await engine.syncNow(transport);

          // Transmitted exactly once per key.
          expect(calls.size).toBe(ops.length);
          for (const op of ops) {
            expect(calls.get(op.idempotencyKey)).toBe(1);
          }
          // Removed from the queue + Local_Store.
          expect(await listOperations()).toHaveLength(0);
          // SyncState reflects an empty queue.
          expect(engine.getState().pending).toBe(0);
          expect(engine.getState().failed).toHaveLength(0);
          // Cached ticket state marks the processed action for dup detection.
          for (const op of ops) {
            const cached = await getTicketState(op.ticketId);
            expect(cached).toBeDefined();
            if (op.kind === 'PICKUP') {
              expect(cached?.pickedUp).toBe(true);
            } else {
              expect(cached?.checkedIn).toBe(true);
            }
          }
        }

        // -- Network-error transport: every op stays PENDING for retry.
        await deleteScannerDB();
        await seedQueue(ops);
        {
          const engine = new SyncEngine();
          const { transport, calls } = makeMockTransport({
            classification: 'RETAIN',
            error: 'network down',
          });
          await engine.syncNow(transport);

          expect(calls.size).toBe(ops.length);
          const remaining = await listOperations();
          expect(remaining).toHaveLength(ops.length);
          for (const op of remaining) {
            expect(op.status).toBe('PENDING');
          }
          expect(engine.getState().pending).toBe(ops.length);
          expect(engine.getState().failed).toHaveLength(0);
        }

        // -- Non-retryable rejection transport: every op moves to FAILED and is
        //    surfaced in SyncState.failed.
        await deleteScannerDB();
        await seedQueue(ops);
        {
          const engine = new SyncEngine();
          const { transport, calls } = makeMockTransport({
            classification: 'FAIL',
            error: 'QR_SIGNATURE_INVALID: forged token',
          });
          await engine.syncNow(transport);

          expect(calls.size).toBe(ops.length);
          const remaining = await listOperations();
          expect(remaining).toHaveLength(ops.length);
          for (const op of remaining) {
            expect(op.status).toBe('FAILED');
          }
          const state = engine.getState();
          expect(state.pending).toBe(0);
          expect(state.failed).toHaveLength(ops.length);
          const failedKeys = new Set(state.failed.map((op) => op.idempotencyKey));
          for (const op of ops) {
            expect(failedKeys.has(op.idempotencyKey)).toBe(true);
          }
        }
      }),
      { numRuns: NUM_RUNS },
    );
  });
});
