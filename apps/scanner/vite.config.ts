import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { VitePWA } from 'vite-plugin-pwa';

// Vite config for the Scanner PWA (Vite + Svelte 5 + vite-plugin-pwa).
//
// The PWA layer (task 9.2) gives the scanner installability and a zero-network
// launch: Workbox precaches the built app shell + static assets so a
// home-screen launch renders the UI offline. API calls are deliberately NOT
// cached — the server is the single source of truth for scan/verify/check-in
// and pickup, and a POST must never be served from cache (correctness over
// speed). navigateFallback serves the precached shell for offline navigations.
export default defineConfig({
  plugins: [
    svelte(),
    VitePWA({
      // Auto-update keeps the installed shell fresh without a manual reload
      // prompt — appropriate for an internal staff tool.
      registerType: 'autoUpdate',
      // `injectRegister: 'auto'` injects the service-worker registration into
      // the app entry automatically (no manual registerSW import needed).
      injectRegister: 'auto',
      // GenerateSW (Workbox) is the default strategy; state it explicitly.
      strategies: 'generateSW',
      includeAssets: ['favicon.svg', 'maskable-icon.svg'],
      manifest: {
        name: 'Ivy Scanner',
        short_name: 'Ivy Scanner',
        description:
          'Offline-capable check-in & race-pack pickup scanner for event staff.',
        lang: 'en',
        display: 'standalone',
        orientation: 'portrait',
        start_url: '/',
        scope: '/',
        theme_color: '#0f172a',
        background_color: '#0f172a',
        icons: [
          {
            src: 'icon-192.png',
            sizes: '192x192',
            type: 'image/png',
            purpose: 'any',
          },
          {
            src: 'icon-512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'any',
          },
          {
            src: 'maskable-icon.svg',
            sizes: 'any',
            type: 'image/svg+xml',
            purpose: 'maskable',
          },
        ],
      },
      workbox: {
        // Precache the built app shell + static assets so the scanner launches
        // with zero network from the home screen.
        globPatterns: ['**/*.{js,css,html,svg,png,ico,woff,woff2}'],
        // Offline navigations fall back to the precached shell (index.html).
        navigateFallback: '/index.html',
        // Never let the SW intercept API traffic — the sync engine (task 11)
        // owns network-only replay of scan/check-in/pickup POSTs.
        navigateFallbackDenylist: [/^\/api\//],
        // Clean up outdated precaches on activation.
        cleanupOutdatedCaches: true,
        clientsClaim: true,
        // No runtimeCaching for the API: requests to the backend stay
        // network-only (uncached) so a stale scan/verify is never served and
        // no POST response is ever cached.
      },
      devOptions: {
        // Keep the SW disabled in `vite dev` (default). It is generated only on
        // `vite build`, so `pnpm build` emits sw.js + manifest.webmanifest.
        enabled: false,
      },
    }),
  ],
  server: {
    port: 5174,
  },
});
