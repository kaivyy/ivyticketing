# Mobile Bottom Navbar & Corner Styles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a sleek, ergonomic Mobile Bottom Navigation Bar across platform pages (except `/events/[eventId]/*`), support switching between Mobile Bottom Nav and Classic Top Nav in Admin, and implement customizable Corner Radius (defaulting to Sharp Boxy / "Full Kotak") for cards and buttons.

**Architecture:** Extend `HomepageConfig` with `navbar.mobileNavType` and `corners.style`. Implement dynamic mobile bottom nav in `index.astro` and `events/index.astro`, dynamic CSS variable-driven corner styles for cards and buttons, and add intuitive controls in Admin Studio with server-side synchronization via `/platform-config.json`.

**Tech Stack:** Astro, Tailwind CSS, TypeScript, Playwright, Node.js

## Global Constraints

- ZERO em dashes (`\u2014`) allowed in any file, comment, or text (R-02).
- Mobile tap target >= 44px on all interactive elements (R-03).
- WCAG AA contrast standard maintained on all navigation states (R-25).
- Main content has bottom padding (`pb-24`) so bottom nav never covers content (antislop-layoutmobile).
- Event ID pages (`/events/[eventId]/*`) must NOT display the platform bottom navbar.

---

### Task 1: Extend HomepageConfig Data Structure & Defaults

**Files:**
- Modify: `apps/web/src/lib/events-store.ts`
- Modify: `apps/web/src/pages/platform-config.json.ts`

**Interfaces:**
- Produces: `ElementCornerStyle = "square" | "rounded" | "pill"`, `MobileNavType = "bottom_nav" | "top_bar"`
- `DEFAULT_HOMEPAGE_CONFIG.navbar.mobileNavType = "bottom_nav"`
- `DEFAULT_HOMEPAGE_CONFIG.corners = { style: "square" }`

- [ ] **Step 1: Update type definitions in `events-store.ts`**
Add `MobileNavType` and `CornerConfig` interfaces and integrate into `NavbarConfig` and `HomepageConfig`.

- [ ] **Step 2: Update `DEFAULT_HOMEPAGE_CONFIG` in `events-store.ts`**
Set `mobileNavType: "bottom_nav"` and `corners: { style: "square" }`.

- [ ] **Step 3: Update `syncHomepageConfigFromServer` in `events-store.ts`**
Ensure `corners` and `navbar.mobileNavType` are merged properly on server fetch.

---

### Task 2: Implement Mobile Bottom Navigation & Minimal Top Header in Homepage (`index.astro`)

**Files:**
- Modify: `apps/web/src/pages/index.astro`

- [ ] **Step 1: Add Mobile Bottom Navigation Bar markup (`#mobile-bottom-nav`)**
Add fixed bottom bar with 4 ergonomic tabs: Beranda (`/`), Event (`/events`), War & Ballot (`/#section-system`), Akun (`/participant/dashboard`).
Ensure minimum tap height >= 48px and proper safe-area padding.

- [ ] **Step 2: Add dynamic mobile nav switching logic in `applyNavbarConfig()`**
If `mobileNavType === "top_bar"`, show standard top navbar with hamburger drawer.
If `mobileNavType === "bottom_nav"` (default), show `#mobile-bottom-nav` and keep `#main-nav` minimal (logo + status indicator).

- [ ] **Step 3: Add `pb-24` on `#main-content` for mobile**
Ensure bottom content is never obscured by the bottom bar.

---

### Task 3: Implement Mobile Bottom Navigation in Events Catalog (`events/index.astro`)

**Files:**
- Modify: `apps/web/src/pages/events/index.astro`

- [ ] **Step 1: Add Mobile Bottom Navigation Bar markup (`#mobile-bottom-nav`)**
Add fixed bottom bar matching `index.astro` with active state on "Event" tab.

- [ ] **Step 2: Add dynamic mobile nav switching logic in `applyNavbarConfig()`**
Handle `bottom_nav` vs `top_bar` display.

- [ ] **Step 3: Add `pb-24` on `#main-content`**
Ensure catalog filters and cards do not collide with bottom bar.

---

### Task 4: Implement Dynamic Corner Radius Styling (Full Kotak / Square Default)

**Files:**
- Modify: `apps/web/src/pages/index.astro`
- Modify: `apps/web/src/pages/events/index.astro`

- [ ] **Step 1: Implement `applyCornerConfig()` helper in `index.astro`**
Set CSS variables and classes for `--card-radius`, `--btn-radius`, `--badge-radius` (`0px` when `style === "square"`, `12px/8px` when `rounded`, `20px/9999px` when `pill`).
Apply to all `.theme-card`, `.category-card`, `.dist-chip`, and action buttons.

- [ ] **Step 2: Implement `applyCornerConfig()` helper in `events/index.astro`**
Set CSS variables and classes for event catalog cards (`article`), control bar, and filter buttons.

---

### Task 5: Add Admin Studio Controls for Mobile Nav Type & Corner Styles

**Files:**
- Modify: `apps/web/src/pages/admin/events.astro`

- [ ] **Step 1: Add Radio inputs for "Tipe Navigasi Mobile"**
Opsi 1: Mobile Bottom Navbar (Bawaan). Opsi 2: Top Navbar Klasik.

- [ ] **Step 2: Add Radio inputs for "Gaya Sudut Elemen (Corner Radius)"**
Opsi 1: Kotak Tegas / Sharp Boxy (Bawaan). Opsi 2: Melengkung Atletik. Opsi 3: Pill Lengkung.

- [ ] **Step 3: Connect inputs to `initHomepageCustomizer()`, simulator preview, and `saveHomepageConfig()`**
Ensure immediate live preview and persistent saving to `/platform-config.json`.

---

### Task 6: Playwright Verification, Build & Delivery Gate

**Files:**
- Test: Playwright test script executing all scenarios

- [ ] **Step 1: Test mobile viewport (390x844)**
Assert `#mobile-bottom-nav` exists and is visible on `/` and `/events`.
Assert `#mobile-bottom-nav` does NOT exist on `/events/boston-marathon-2026` or queue/checkout.
Assert cards have `borderRadius: "0px"` in default square mode.

- [ ] **Step 2: Test admin toggling**
Switch to `top_bar` in admin, assert top hamburger appears on mobile.
Switch to `rounded`, assert cards receive rounded corners.

- [ ] **Step 3: Build and Code Quality Check**
Verify `assert "\u2014" not in content` on all files.
Run `pnpm --dir apps/web build`.
Run `graphify update .`.
