// Vitest global setup.
//
// `fake-indexeddb/auto` installs an in-memory IndexedDB implementation onto the
// global scope so the offline-queue persistence tests (task 10) can run under
// jsdom without a real browser.
import 'fake-indexeddb/auto';

// jest-dom custom matchers (toBeInTheDocument, toHaveTextContent, …) for the
// Svelte component tests (task 13.4). @testing-library/svelte registers its own
// auto-cleanup between tests when the global afterEach is available (globals:
// true in vitest.config.ts).
import '@testing-library/jest-dom/vitest';

// `enqueueScan` (offline-db) generates Idempotency-Keys with
// `crypto.randomUUID()`. Some jsdom builds expose a `crypto` global without
// `randomUUID`; fall back to Node's WebCrypto so the offline-queue tests can
// mint keys deterministically in the test environment.
import { webcrypto } from 'node:crypto';

if (typeof globalThis.crypto?.randomUUID !== 'function') {
  Object.defineProperty(globalThis, 'crypto', {
    value: webcrypto,
    configurable: true,
    writable: true,
  });
}

// jsdom in this environment does not expose a functional Web Storage
// (localStorage.setItem/clear are missing), which session.ts relies on for the
// bearer token (Local_Store). Install a minimal Map-backed Storage so the auth
// component tests (task 13.4) can persist and read the token deterministically.
if (typeof globalThis.localStorage?.setItem !== 'function') {
  class MemoryStorage implements Storage {
    #map = new Map<string, string>();
    get length(): number {
      return this.#map.size;
    }
    clear(): void {
      this.#map.clear();
    }
    getItem(key: string): string | null {
      return this.#map.has(key) ? (this.#map.get(key) as string) : null;
    }
    key(index: number): string | null {
      return Array.from(this.#map.keys())[index] ?? null;
    }
    removeItem(key: string): void {
      this.#map.delete(key);
    }
    setItem(key: string, value: string): void {
      this.#map.set(key, String(value));
    }
  }
  Object.defineProperty(globalThis, 'localStorage', {
    value: new MemoryStorage(),
    configurable: true,
    writable: true,
  });
}
