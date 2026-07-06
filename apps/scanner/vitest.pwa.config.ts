import { defineConfig } from 'vitest/config';

// Separate Vitest config for the PWA build-artifact assertions (tasks 14.2 / 14.3).
//
// These tests read the production `dist/` output (manifest.webmanifest + Workbox
// sw.js), so they run in a plain `node` environment and are deliberately kept
// OUT of the fast unit suite (`pnpm test`, which only globs `src/**`). They are
// invoked explicitly via `pnpm test:pwa`, which builds first and then asserts
// against the real build output.
export default defineConfig({
  test: {
    environment: 'node',
    include: ['test/**/*.{test,spec}.ts'],
    // A production build runs in beforeAll; give it room on cold machines.
    testTimeout: 180_000,
    hookTimeout: 180_000,
  },
});
