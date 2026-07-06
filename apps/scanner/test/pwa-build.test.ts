import { execSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { beforeAll, describe, expect, it } from 'vitest';

// Feature: scanner-pwa, Task 14.2 (smoke: manifest + service worker installability)
// Feature: scanner-pwa, Task 14.3 (integration: offline app shell loads from cache)
//
// The PWA artifacts (manifest.webmanifest, sw.js, workbox-*.js) only exist AFTER
// `vite build`, so these are build-output assertions: we run a real production
// build in beforeAll and then assert against the emitted files. This exercises
// the vite-plugin-pwa / Workbox config from task 9.2 exactly as shipped.

const appRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const distDir = resolve(appRoot, 'dist');
const manifestPath = resolve(distDir, 'manifest.webmanifest');
const swPath = resolve(distDir, 'sw.js');

let manifest: Record<string, unknown>;
let swSource: string;

beforeAll(() => {
  // Run a real production build so the assertions always reflect the shipped
  // Workbox output rather than a stale dist/. `vite build` regenerates
  // manifest.webmanifest + sw.js + the precache manifest deterministically.
  execSync('pnpm build', { cwd: appRoot, stdio: 'inherit' });

  expect(existsSync(manifestPath), 'dist/manifest.webmanifest should be emitted').toBe(true);
  expect(existsSync(swPath), 'dist/sw.js should be emitted').toBe(true);

  manifest = JSON.parse(readFileSync(manifestPath, 'utf8')) as Record<string, unknown>;
  swSource = readFileSync(swPath, 'utf8');
});

describe('Task 14.2 — smoke: manifest + service worker installability (Req 9.1)', () => {
  it('emits a manifest with the required installability fields', () => {
    expect(typeof manifest.name).toBe('string');
    expect((manifest.name as string).length).toBeGreaterThan(0);

    expect(typeof manifest.short_name).toBe('string');
    expect((manifest.short_name as string).length).toBeGreaterThan(0);

    // display must be an installable mode (standalone/fullscreen/minimal-ui).
    expect(typeof manifest.display).toBe('string');
    expect(['standalone', 'fullscreen', 'minimal-ui']).toContain(manifest.display as string);

    expect(typeof manifest.start_url).toBe('string');
    expect((manifest.start_url as string).length).toBeGreaterThan(0);
  });

  it('declares icons including the required 192px and 512px sizes', () => {
    const icons = manifest.icons as Array<{ src?: string; sizes?: string; type?: string }>;
    expect(Array.isArray(icons)).toBe(true);
    expect(icons.length).toBeGreaterThan(0);

    const sizes = icons.map((i) => i.sizes ?? '');
    expect(sizes.some((s) => s.split(' ').includes('192x192'))).toBe(true);
    expect(sizes.some((s) => s.split(' ').includes('512x512'))).toBe(true);

    // Each declared icon must reference a src and a type.
    for (const icon of icons) {
      expect(typeof icon.src).toBe('string');
      expect((icon.src as string).length).toBeGreaterThan(0);
      expect(typeof icon.type).toBe('string');
    }
  });

  it('generates a Workbox service worker with a precache present', () => {
    // Workbox GenerateSW output wires precaching via precacheAndRoute([...]).
    expect(swSource).toContain('precacheAndRoute');
    // The SW loads the Workbox runtime chunk it depends on.
    expect(swSource).toMatch(/workbox-[0-9a-f]+/);
    // The precache list must be non-empty.
    expect(swSource).toMatch(/precacheAndRoute\(\[\{/);
  });
});

describe('Task 14.3 — integration: offline app shell loads from cache (Req 9.2)', () => {
  it('precaches the app shell (index.html) so an offline launch has the shell cached', () => {
    expect(swSource).toContain('index.html');
    // index.html is registered as a precache entry (Workbox emits either the
    // minified `url:"index.html"` or the expanded `"url": "index.html"` form).
    expect(swSource).toMatch(/["']?url["']?\s*:\s*["']index\.html["']/);
  });

  it('precaches the built JS and CSS assets that make up the shell', () => {
    // Hashed asset bundles emitted by vite must be in the precache manifest so
    // the offline shell renders without any network fetch.
    expect(swSource).toMatch(/assets\/[^"']+\.js/);
    expect(swSource).toMatch(/assets\/[^"']+\.css/);
  });

  it('configures navigateFallback to the precached shell for offline navigations', () => {
    // navigateFallback => a NavigationRoute bound to the precached /index.html.
    expect(swSource).toContain('NavigationRoute');
    expect(swSource).toMatch(/createHandlerBoundToURL\(["']\/index\.html["']\)/);
  });

  it('denylists /api/ so API navigations are never served the cached shell', () => {
    // The API stays network-only; the shell fallback must not shadow /api/*.
    expect(swSource).toMatch(/denylist:\s*\[\/\^\\\/api\\\//);
  });
});
