# Phase 7 Plan — Part 5: Frontend participant dashboard

> Part of the Phase 7 implementation plan. Index: [2026-06-08-phase7-participant-dashboard-ticket.md](2026-06-08-phase7-participant-dashboard-ticket.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** New `apps/web` files + additive `lib/api.ts`. Assumes Part 4 endpoints exist and the API runs on `PUBLIC_API_URL` (default `http://localhost:8080`).

**Backend auth facts (verified):**
- `POST /api/v1/auth/login` body `{ "email", "password" }` → `200 { accessToken, expiresIn, user }` + sets HttpOnly `refresh_token` cookie (path `/api/v1/auth`).
- `GET /api/v1/auth/me` (Bearer access token) → current user.
- `POST /api/v1/auth/refresh` → new `{ accessToken, expiresIn }` using the refresh cookie.
- All API routes are under `/api/v1`.

**Frontend auth model (minimal):** Astro runs SSR/dev on port 4321. Store the access token client-side in memory + `sessionStorage` (NOT localStorage long-term — access token is short-lived). The refresh cookie is HttpOnly and handled by the browser. On 401, attempt one `/auth/refresh`, then retry; if that fails, redirect to `/login`. SSR pages that need data fetch client-side after hydration (simplest correct approach for token-in-storage). This keeps the access token out of server logs and avoids SSR cookie plumbing for the MVP.

> SECURITY NOTE for the implementer: storing the access token in `sessionStorage` is acceptable for a short-lived token in this MVP, but it is readable by JS (XSS exposure). The refresh token stays HttpOnly. Do not put the refresh token or any QR secret in client storage. Flag this tradeoff in `PHASE7_DECISIONS.md` (Part 6).

---

## Task 18: Auth foundation — lib/auth.ts + authed api.ts

**Files:**
- Create: `apps/web/src/lib/auth.ts`
- Modify: `apps/web/src/lib/api.ts`
- Modify: `apps/web/package.json` (add `qrcode` dep)

- [ ] **Step 1: Add the QR client dependency**

In `apps/web/`, run:
```bash
cd apps/web && npm install qrcode && npm install -D @types/qrcode; cd ../..
```
Expected: `qrcode` added to `dependencies` in `apps/web/package.json`.

- [ ] **Step 2: Implement lib/auth.ts**

Create `apps/web/src/lib/auth.ts`:
```ts
const API_URL = import.meta.env.PUBLIC_API_URL ?? "http://localhost:8080";
const TOKEN_KEY = "ivy_access_token";

export interface User {
  id: string;
  email: string;
  fullName: string;
}

export function getToken(): string | null {
  if (typeof sessionStorage === "undefined") return null;
  return sessionStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  sessionStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  sessionStorage.removeItem(TOKEN_KEY);
}

export async function login(email: string, password: string): Promise<void> {
  const res = await fetch(`${API_URL}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ email, password }),
  });
  if (!res.ok) {
    throw new Error("Email atau kata sandi salah.");
  }
  const data = await res.json();
  setToken(data.accessToken);
}

export async function refresh(): Promise<boolean> {
  const res = await fetch(`${API_URL}/api/v1/auth/refresh`, {
    method: "POST",
    credentials: "include",
  });
  if (!res.ok) return false;
  const data = await res.json();
  setToken(data.accessToken);
  return true;
}

export async function logout(): Promise<void> {
  await fetch(`${API_URL}/api/v1/auth/logout`, { method: "POST", credentials: "include" }).catch(() => {});
  clearToken();
}

export function redirectToLogin(): void {
  if (typeof window !== "undefined") window.location.href = "/login";
}
```

- [ ] **Step 3: Extend lib/api.ts with an authed fetch**

In `apps/web/src/lib/api.ts`, keep `fetchReadiness` and ADD:
```ts
import { getToken, refresh, redirectToLogin } from "./auth";

const BASE = import.meta.env.PUBLIC_API_URL ?? "http://localhost:8080";

export interface ApiError {
  code: string;
  message: string;
}

// authedFetch attaches the Bearer token, retries once after refresh on 401,
// and redirects to /login if refresh fails.
export async function authedFetch<T>(path: string): Promise<T> {
  const doFetch = () =>
    fetch(`${BASE}/api/v1${path}`, {
      headers: { Authorization: `Bearer ${getToken() ?? ""}` },
      credentials: "include",
    });

  let res = await doFetch();
  if (res.status === 401) {
    const ok = await refresh();
    if (!ok) {
      redirectToLogin();
      throw new Error("unauthenticated");
    }
    res = await doFetch();
  }
  if (!res.ok) {
    let err: ApiError = { code: "ERROR", message: `HTTP ${res.status}` };
    try {
      const body = await res.json();
      if (body?.error) err = body.error;
    } catch {
      /* ignore */
    }
    throw new Error(err.message);
  }
  return (await res.json()) as T;
}
```

- [ ] **Step 4: Build the web app**

Run:
```bash
cd apps/web && npm run build; cd ../..
```
Expected: build succeeds (TypeScript compiles).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/lib/auth.ts apps/web/src/lib/api.ts apps/web/package.json apps/web/package-lock.json
git commit -m "feat(phase7): web auth foundation (login, token, authed fetch)"
```

---

## Task 19: Login page + ParticipantLayout + route guard

**Files:**
- Create: `apps/web/src/pages/login.astro`
- Create: `apps/web/src/layouts/ParticipantLayout.astro`

- [ ] **Step 1: Implement login.astro**

Create `apps/web/src/pages/login.astro`:
```astro
---
import PublicLayout from "../layouts/PublicLayout.astro";
---
<PublicLayout title="Masuk — ivyticketing">
  <h1 class="text-2xl font-bold mb-6">Masuk</h1>
  <form id="login-form" class="space-y-4 rounded-lg border border-slate-200 bg-white p-6">
    <div>
      <label class="block text-sm font-medium mb-1" for="email">Email</label>
      <input id="email" type="email" required class="w-full rounded border border-slate-300 px-3 py-2" />
    </div>
    <div>
      <label class="block text-sm font-medium mb-1" for="password">Kata sandi</label>
      <input id="password" type="password" required class="w-full rounded border border-slate-300 px-3 py-2" />
    </div>
    <p id="error" class="text-sm text-red-600 hidden"></p>
    <button type="submit" class="w-full rounded bg-slate-900 px-4 py-2 text-white">Masuk</button>
  </form>

  <script>
    import { login } from "../lib/auth";
    const form = document.getElementById("login-form") as HTMLFormElement;
    const errEl = document.getElementById("error") as HTMLParagraphElement;
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      errEl.classList.add("hidden");
      const email = (document.getElementById("email") as HTMLInputElement).value;
      const password = (document.getElementById("password") as HTMLInputElement).value;
      try {
        await login(email, password);
        window.location.href = "/participant/dashboard";
      } catch (err) {
        errEl.textContent = (err as Error).message;
        errEl.classList.remove("hidden");
      }
    });
  </script>
</PublicLayout>
```

- [ ] **Step 2: Implement ParticipantLayout.astro** (client-side guard)

Create `apps/web/src/layouts/ParticipantLayout.astro`:
```astro
---
import "../styles/global.css";
const { title = "ivyticketing" } = Astro.props;
---
<!doctype html>
<html lang="id">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{title}</title>
  </head>
  <body class="min-h-screen bg-slate-50 text-slate-900">
    <header class="border-b border-slate-200 bg-white">
      <nav class="mx-auto flex max-w-3xl items-center justify-between p-4">
        <a href="/participant/dashboard" class="font-semibold">ivyticketing</a>
        <div class="flex gap-4 text-sm">
          <a href="/participant/orders">Pesanan</a>
          <a href="/participant/tickets">Tiket</a>
          <button id="logout-btn" class="text-slate-500">Keluar</button>
        </div>
      </nav>
    </header>
    <main class="mx-auto max-w-3xl p-6"><slot /></main>

    <script>
      import { getToken, logout } from "../lib/auth";
      if (!getToken()) window.location.href = "/login";
      document.getElementById("logout-btn")?.addEventListener("click", async () => {
        await logout();
        window.location.href = "/login";
      });
    </script>
  </body>
</html>
```

- [ ] **Step 3: Build**

Run:
```bash
cd apps/web && npm run build; cd ../..
```
Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/pages/login.astro apps/web/src/layouts/ParticipantLayout.astro
git commit -m "feat(phase7): login page + participant layout with client guard"
```

---

## Task 20: Ticket data lib + QR display + ticket card

**Files:**
- Create: `apps/web/src/lib/tickets.ts`
- Create: `apps/web/src/components/ticket/QrDisplay.astro`
- Create: `apps/web/src/components/ticket/TicketCard.astro`

- [ ] **Step 1: Implement lib/tickets.ts**

Create `apps/web/src/lib/tickets.ts`:
```ts
import { authedFetch } from "./api";

export interface Ticket {
  id: string;
  ticketNumber: string;
  status: string;
  orderId: string;
  eventId: string;
  categoryId: string;
  holderName: string;
  holderEmail: string;
  eventTitle: string;
  categoryName: string;
  issuedAt: string;
  usedAt?: string;
}

export interface TicketWithQR extends Ticket {
  qrToken: string;
}

export function listMyTickets(): Promise<Ticket[]> {
  return authedFetch<Ticket[]>("/tickets");
}

export function getTicket(id: string): Promise<TicketWithQR> {
  return authedFetch<TicketWithQR>(`/tickets/${id}`);
}

export function getTicketByOrder(orderId: string): Promise<TicketWithQR> {
  return authedFetch<TicketWithQR>(`/orders/${orderId}/ticket`);
}
```

- [ ] **Step 2: Implement QrDisplay.astro** (renders a token to a canvas)

Create `apps/web/src/components/ticket/QrDisplay.astro`:
```astro
---
// Renders a QR token string into a canvas client-side. The token comes from the API.
const { token } = Astro.props;
---
<canvas data-qr-token={token} class="qr-canvas mx-auto" width="220" height="220"></canvas>
<script>
  import QRCode from "qrcode";
  document.querySelectorAll<HTMLCanvasElement>(".qr-canvas").forEach((c) => {
    const token = c.dataset.qrToken;
    if (token) QRCode.toCanvas(c, token, { width: 220 });
  });
</script>
```

- [ ] **Step 3: Implement TicketCard.astro**

Create `apps/web/src/components/ticket/TicketCard.astro`:
```astro
---
const { ticket } = Astro.props;
const statusColor = ticket.status === "VALID" ? "text-green-600" : "text-slate-500";
---
<a href={`/participant/tickets/${ticket.id}`} class="block rounded-lg border border-slate-200 bg-white p-4 hover:border-slate-400">
  <div class="flex items-center justify-between">
    <span class="font-medium">{ticket.eventTitle}</span>
    <span class={statusColor}>{ticket.status}</span>
  </div>
  <p class="text-sm text-slate-600">{ticket.categoryName} · {ticket.ticketNumber}</p>
</a>
```

- [ ] **Step 4: Build**

Run:
```bash
cd apps/web && npm run build; cd ../..
```
Expected: build succeeds.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/lib/tickets.ts apps/web/src/components/ticket/QrDisplay.astro apps/web/src/components/ticket/TicketCard.astro
git commit -m "feat(phase7): ticket lib + QR display + ticket card"
```

---

## Task 21: Tickets pages (list + detail with QR)

**Files:**
- Create: `apps/web/src/pages/participant/tickets.astro`
- Create: `apps/web/src/pages/participant/tickets/[ticketId].astro`

- [ ] **Step 1: Implement tickets.astro** (list, client-fetched)

Create `apps/web/src/pages/participant/tickets.astro`:
```astro
---
import ParticipantLayout from "../../layouts/ParticipantLayout.astro";
---
<ParticipantLayout title="Tiket Saya">
  <h1 class="text-xl font-bold mb-4">Tiket Saya</h1>
  <div id="list" class="space-y-3"><p class="text-slate-500">Memuat…</p></div>

  <script>
    import { listMyTickets } from "../../lib/tickets";
    const list = document.getElementById("list")!;
    try {
      const tickets = await listMyTickets();
      if (tickets.length === 0) {
        list.innerHTML = '<p class="text-slate-500">Belum ada tiket.</p>';
      } else {
        list.innerHTML = tickets
          .map(
            (t) => `<a href="/participant/tickets/${t.id}" class="block rounded-lg border border-slate-200 bg-white p-4 hover:border-slate-400">
              <div class="flex items-center justify-between"><span class="font-medium">${t.eventTitle}</span><span class="${t.status === "VALID" ? "text-green-600" : "text-slate-500"}">${t.status}</span></div>
              <p class="text-sm text-slate-600">${t.categoryName} · ${t.ticketNumber}</p></a>`
          )
          .join("");
      }
    } catch (e) {
      list.innerHTML = `<p class="text-red-600">${(e as Error).message}</p>`;
    }
  </script>
</ParticipantLayout>
```

- [ ] **Step 2: Implement tickets/[ticketId].astro** (detail + QR)

Create `apps/web/src/pages/participant/tickets/[ticketId].astro`:
```astro
---
import ParticipantLayout from "../../../layouts/ParticipantLayout.astro";
// Client-rendered; no SSR data. Astro needs the param to exist.
export function getStaticPaths() {
  return [];
}
---
<ParticipantLayout title="Detail Tiket">
  <a href="/participant/tickets" class="text-sm text-slate-500">← Kembali</a>
  <div id="detail" class="mt-4"><p class="text-slate-500">Memuat…</p></div>

  <script>
    import { getTicket } from "../../../lib/tickets";
    import QRCode from "qrcode";

    const id = window.location.pathname.split("/").pop()!;
    const detail = document.getElementById("detail")!;
    try {
      const t = await getTicket(id);
      detail.innerHTML = `
        <div class="rounded-lg border border-slate-200 bg-white p-6 text-center">
          <h1 class="text-lg font-bold">${t.eventTitle}</h1>
          <p class="text-slate-600">${t.categoryName} · ${t.ticketNumber}</p>
          <p class="mt-1 text-sm ${t.status === "VALID" ? "text-green-600" : "text-slate-500"}">${t.status}</p>
          <canvas id="qr" class="mx-auto mt-4" width="220" height="220"></canvas>
          <p class="mt-4 text-sm text-slate-600">${t.holderName}</p>
        </div>`;
      await QRCode.toCanvas(document.getElementById("qr"), t.qrToken, { width: 220 });
    } catch (e) {
      detail.innerHTML = `<p class="text-red-600">${(e as Error).message}</p>`;
    }
  </script>
</ParticipantLayout>
```

> Astro routing note: a dynamic route with `output: "static"` (default) needs `getStaticPaths`. Returning `[]` plus client-side param parsing works only if the route is reachable. If the build complains, set `export const prerender = false;` in this page and add the Node/standalone adapter, OR switch the project to `output: "server"`. Simplest for MVP: add `export const prerender = false;` to this page (Astro will server-render it on demand in dev). Verify in Step 3; pick whichever the toolchain accepts and note it.

- [ ] **Step 3: Build + dev sanity**

Run:
```bash
cd apps/web && npm run build; cd ../..
```
Expected: build succeeds. If the dynamic route errors, apply the routing note (add `export const prerender = false;`) and rebuild.

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/pages/participant/tickets.astro "apps/web/src/pages/participant/tickets/[ticketId].astro"
git commit -m "feat(phase7): participant ticket list + detail with QR"
```

---

## Task 22: Orders pages + timeline + invoice

**Files:**
- Create: `apps/web/src/lib/invoice.ts`
- Create: `apps/web/src/components/ticket/OrderTimeline.astro`
- Create: `apps/web/src/components/ticket/InvoiceView.astro`
- Create: `apps/web/src/pages/participant/orders.astro`
- Create: `apps/web/src/pages/participant/orders/[orderId].astro`
- Create: `apps/web/src/pages/participant/dashboard.astro`

- [ ] **Step 1: Implement lib/invoice.ts**

Create `apps/web/src/lib/invoice.ts`:
```ts
import { authedFetch } from "./api";

export interface Order {
  id: string;
  orderNumber: string;
  status: string;
  total: number;
  createdAt: string;
}

export interface Invoice {
  orderId: string;
  orderNumber: string;
  status: string;
  eventTitle: string;
  categoryName: string;
  holderName: string;
  holderEmail: string;
  subtotal: number;
  fee: number;
  discount: number;
  total: number;
  currency: string;
  issuedAt: string;
}

export function listMyOrders(): Promise<Order[]> {
  return authedFetch<Order[]>("/orders");
}

export function getInvoice(orderId: string): Promise<Invoice> {
  return authedFetch<Invoice>(`/orders/${orderId}/invoice`);
}
```

> Confirm the Phase 5 `GET /api/v1/orders` response field names (`id`, `orderNumber`, `status`, `total`, `createdAt`). If they differ, align the `Order` interface to the actual JSON.

- [ ] **Step 2: Implement InvoiceView.astro** (printable)

Create `apps/web/src/components/ticket/InvoiceView.astro`:
```astro
---
const { invoice } = Astro.props;
const fmt = (n: number) => `Rp${(n / 100).toLocaleString("id-ID")}`;
---
<div class="rounded-lg border border-slate-200 bg-white p-6 print:border-0">
  <div class="flex items-center justify-between">
    <h2 class="text-lg font-bold">Invoice</h2>
    <button onclick="window.print()" class="rounded border px-3 py-1 text-sm print:hidden">Cetak / PDF</button>
  </div>
  <p class="text-sm text-slate-600">{invoice.orderNumber} · {invoice.status}</p>
  <dl class="mt-4 space-y-1 text-sm">
    <div class="flex justify-between"><dt>Event</dt><dd>{invoice.eventTitle}</dd></div>
    <div class="flex justify-between"><dt>Kategori</dt><dd>{invoice.categoryName}</dd></div>
    <div class="flex justify-between"><dt>Subtotal</dt><dd>{fmt(invoice.subtotal)}</dd></div>
    <div class="flex justify-between"><dt>Biaya</dt><dd>{fmt(invoice.fee)}</dd></div>
    <div class="flex justify-between"><dt>Diskon</dt><dd>-{fmt(invoice.discount)}</dd></div>
    <div class="flex justify-between font-semibold"><dt>Total</dt><dd>{fmt(invoice.total)}</dd></div>
  </dl>
</div>
```

> Amount unit: Phase 5/6 store amounts in the smallest currency unit (sen). `fmt` divides by 100. Verify against an actual order; if amounts are already in rupiah, drop the `/100`.

- [ ] **Step 3: Implement OrderTimeline.astro**

Create `apps/web/src/components/ticket/OrderTimeline.astro`:
```astro
---
// Derives a simple timeline from order status + timestamps passed in.
const { order } = Astro.props;
const steps = [
  { key: "created", label: "Pesanan dibuat", done: true },
  { key: "pending", label: "Menunggu pembayaran", done: order.status !== "DRAFT" },
  { key: "paid", label: "Lunas", done: order.status === "PAID" },
];
---
<ol class="space-y-2">
  {steps.map((s) => (
    <li class="flex items-center gap-2">
      <span class={s.done ? "text-green-600" : "text-slate-300"}>●</span>
      <span class={s.done ? "" : "text-slate-400"}>{s.label}</span>
    </li>
  ))}
</ol>
```

- [ ] **Step 4: Implement orders.astro (list)**

Create `apps/web/src/pages/participant/orders.astro`:
```astro
---
import ParticipantLayout from "../../layouts/ParticipantLayout.astro";
---
<ParticipantLayout title="Pesanan Saya">
  <h1 class="text-xl font-bold mb-4">Pesanan Saya</h1>
  <div id="list" class="space-y-3"><p class="text-slate-500">Memuat…</p></div>
  <script>
    import { listMyOrders } from "../../lib/invoice";
    const list = document.getElementById("list")!;
    try {
      const orders = await listMyOrders();
      list.innerHTML = orders.length
        ? orders.map((o) => `<a href="/participant/orders/${o.id}" class="block rounded-lg border border-slate-200 bg-white p-4 hover:border-slate-400">
            <div class="flex justify-between"><span class="font-medium">${o.orderNumber}</span><span>${o.status}</span></div></a>`).join("")
        : '<p class="text-slate-500">Belum ada pesanan.</p>';
    } catch (e) {
      list.innerHTML = `<p class="text-red-600">${(e as Error).message}</p>`;
    }
  </script>
</ParticipantLayout>
```

- [ ] **Step 5: Implement orders/[orderId].astro (detail + timeline + invoice + ticket link)**

Create `apps/web/src/pages/participant/orders/[orderId].astro`:
```astro
---
import ParticipantLayout from "../../../layouts/ParticipantLayout.astro";
export const prerender = false;
---
<ParticipantLayout title="Detail Pesanan">
  <a href="/participant/orders" class="text-sm text-slate-500">← Kembali</a>
  <div id="detail" class="mt-4 space-y-6"><p class="text-slate-500">Memuat…</p></div>
  <script>
    import { getInvoice } from "../../../lib/invoice";
    import { getTicketByOrder } from "../../../lib/tickets";
    const orderId = window.location.pathname.split("/").pop()!;
    const detail = document.getElementById("detail")!;
    const fmt = (n: number) => `Rp${(n / 100).toLocaleString("id-ID")}`;
    try {
      let html = "";
      try {
        const inv = await getInvoice(orderId);
        html += `<div class="rounded-lg border border-slate-200 bg-white p-6">
          <div class="flex justify-between"><h2 class="font-bold">Invoice</h2><button onclick="window.print()" class="rounded border px-3 py-1 text-sm print:hidden">Cetak / PDF</button></div>
          <p class="text-sm text-slate-600">${inv.orderNumber} · ${inv.status}</p>
          <dl class="mt-3 space-y-1 text-sm">
            <div class="flex justify-between"><dt>Event</dt><dd>${inv.eventTitle}</dd></div>
            <div class="flex justify-between"><dt>Kategori</dt><dd>${inv.categoryName}</dd></div>
            <div class="flex justify-between font-semibold"><dt>Total</dt><dd>${fmt(inv.total)}</dd></div>
          </dl></div>`;
      } catch {
        html += `<p class="text-slate-500">Invoice tersedia setelah pembayaran lunas.</p>`;
      }
      try {
        const t = await getTicketByOrder(orderId);
        html += `<a href="/participant/tickets/${t.id}" class="block rounded-lg border border-slate-200 bg-white p-4 hover:border-slate-400">Lihat tiket: ${t.ticketNumber} (${t.status})</a>`;
      } catch {
        /* no ticket yet */
      }
      detail.innerHTML = html;
    } catch (e) {
      detail.innerHTML = `<p class="text-red-600">${(e as Error).message}</p>`;
    }
  </script>
</ParticipantLayout>
```

- [ ] **Step 6: Implement dashboard.astro**

Create `apps/web/src/pages/participant/dashboard.astro`:
```astro
---
import ParticipantLayout from "../../layouts/ParticipantLayout.astro";
---
<ParticipantLayout title="Dashboard">
  <h1 class="text-xl font-bold mb-4">Dashboard</h1>
  <div class="grid grid-cols-2 gap-4">
    <a href="/participant/orders" class="rounded-lg border border-slate-200 bg-white p-6 hover:border-slate-400">
      <h2 class="font-semibold">Pesanan Saya</h2>
      <p class="text-sm text-slate-600">Lihat status & invoice</p>
    </a>
    <a href="/participant/tickets" class="rounded-lg border border-slate-200 bg-white p-6 hover:border-slate-400">
      <h2 class="font-semibold">Tiket Saya</h2>
      <p class="text-sm text-slate-600">Lihat tiket & QR</p>
    </a>
  </div>
</ParticipantLayout>
```

- [ ] **Step 7: Build**

Run:
```bash
cd apps/web && npm run build; cd ../..
```
Expected: build succeeds (apply `prerender = false` notes if dynamic routes complain).

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/lib/invoice.ts apps/web/src/components/ticket apps/web/src/pages/participant
git commit -m "feat(phase7): orders pages, timeline, invoice, dashboard"
```

---

## Task 23: Browser verification (golden path)

**Files:** none (manual verification).

- [ ] **Step 1: Start API + DB + web**

In separate terminals (or background):
```bash
make api        # API on :8080 (requires DATABASE_URL, REDIS_URL, JWT_SECRET, TICKET_QR_SECRET set)
make web        # Astro dev on :4321
```
Ensure a participant user exists with a PAID order + issued ticket (seed via the integration flow or a manual checkout→payment→callback in a dev script).

- [ ] **Step 2: Walk the golden path in a browser**

1. Visit `http://localhost:4321/login`, log in as the participant.
2. Land on `/participant/dashboard`.
3. Open `/participant/tickets` → ticket appears.
4. Open the ticket detail → QR renders as an image.
5. Open `/participant/orders/{orderId}` → invoice shows; click Cetak/PDF → print preview works.
6. Verify ownership: log in as a different user → the first user's ticket/invoice is not visible (404).

- [ ] **Step 3: Record verification outcome**

State explicitly in the commit/PR what was verified in the browser and what could not be (e.g., "QR scan/verify not testable in Phase 7 — deferred to Phase 15"). If any step fails, fix before proceeding.

- [ ] **Step 4: Commit (any fixups)**

```bash
git add -A apps/web
git commit -m "fix(phase7): browser verification fixups" || echo "nothing to commit"
```

---

Part 5 complete. Next: [Part 6 — Docs + final verification](2026-06-08-phase7-part6-docs-verify.md).
