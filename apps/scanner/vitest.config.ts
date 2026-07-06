import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// Vitest configuration for the Scanner PWA.
//
// Uses jsdom so DOM-dependent code (and later Svelte component tests) can run,
// and preloads `fake-indexeddb/auto` so the offline-db tests (task 10) have a
// working IndexedDB implementation without a browser.
export default defineConfig({
  plugins: [svelte({ hot: false })],
  // Resolve Svelte's browser (client) build under test so mounting real
  // components with @testing-library/svelte works — without the `browser`
  // condition Svelte 5 resolves its server entry and `mount()` is unavailable.
  resolve: {
    conditions: ['browser'],
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.ts'],
  },
});
