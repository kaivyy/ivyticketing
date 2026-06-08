# Participant Dashboard

## Overview

Phase 7 adds participant-facing endpoints for viewing tickets and orders, an organizer
endpoint for listing event tickets, and an `apps/web` participant dashboard with
client-side QR rendering and invoice printing.

---

## Endpoint Reference

### Participant endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/tickets` | List all tickets for the authenticated participant |
| `GET` | `/api/v1/tickets/{ticketId}` | Ticket detail including QR token |
| `GET` | `/api/v1/tickets/{ticketId}/qr` | QR token only (for refresh without full detail) |
| `GET` | `/api/v1/orders/{orderId}/ticket` | Ticket for a specific order |
| `GET` | `/api/v1/orders/{orderId}/invoice` | Invoice JSON (PAID orders only) |

### Organizer endpoints

| Method | Path | Permission |
|--------|------|------------|
| `GET` | `/api/v1/organizations/{orgId}/events/{eventId}/tickets` | `ticket.view` |

---

## Ownership Model

All participant resources are filtered by `participant_id = caller` at the query
level. A participant cannot see another participant's tickets or orders by guessing
a UUID.

When a ticket or order is found but belongs to a different participant, the handler
returns **404** (not 403). This prevents enumeration — the caller cannot distinguish
"not found" from "found but not yours."

The organizer ticket list endpoint is scoped to a specific event and gated by the
`ticket.view` permission, which is assigned to the Owner, Manager, and Customer
Service role templates.

---

## Invoice Gating

`GET /api/v1/orders/{orderId}/invoice` only returns data for orders with status
`PAID`. Any other status — `PENDING_PAYMENT`, `EXPIRED`, `CANCELLED` — returns the
error code `INVOICE_NOT_AVAILABLE`.

The invoice is returned as JSON. PDF generation is deferred; the browser's
`@media print` CSS handles print formatting (see Frontend below).

---

## Frontend — `apps/web` Participant Pages

### Pages

| Route | Description |
|-------|-------------|
| `/login` | Login form; on success stores access token and redirects |
| `/dashboard` | Overview: upcoming events, recent orders |
| `/orders` | Order list with status badges |
| `/orders/{orderId}` | Order detail: items, payment status, link to ticket |
| `/tickets` | Ticket list |
| `/tickets/{ticketId}` | Ticket detail with QR code |

### Auth model

The frontend uses a minimal auth approach:

- **Access token**: stored in `sessionStorage`. Sent as `Authorization: Bearer <token>`
  on API requests. Cleared on tab close.
- **Refresh token**: stored in an HttpOnly cookie (set by the API, path
  `/api/v1/auth`). The frontend never reads it directly; the browser sends it
  automatically on refresh calls.
- On 401 responses the frontend attempts a silent token refresh via
  `POST /api/v1/auth/refresh`, then retries the original request once.

See `PHASE7_DECISIONS.md` for the sessionStorage vs. cookie tradeoff.

### QR rendering

The QR code image is generated client-side using the `qrcode` npm library. The
ticket detail page calls `GET /api/v1/tickets/{ticketId}/qr` to fetch the raw token
string, then renders it into a `<canvas>` element. No QR image is stored server-side.

### Invoice printing

The invoice page is a standard HTML page styled for screen. A `@media print` CSS
block hides navigation chrome and formats the invoice for A4. The participant prints
via the browser's native print dialog (`Ctrl+P` / `Cmd+P`). No server-side PDF
generation is required.
