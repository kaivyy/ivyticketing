# Registration Modes

## Overview

The registration module defines **9 modes** that govern how participants enter an event's
registration flow. The mode is resolved at checkout time via the `RegistrationGate`
interface, which is a dependency-inversion seam injected into the orders module. Orders
does not import registration or queue -- it only knows about the interface.

## Mode Catalog

| Mode | Enum | Phase | Behavior |
|---|---|---|---|
| NORMAL | `NORMAL` | 5 | Standard checkout; no queue, no gate. Phase 5 behaviour preserved. |
| WAR_QUEUE | `WAR_QUEUE` | 8 | Pure FIFO queue; first-join-first-serve ordering. |
| RANDOMIZED_QUEUE | `RANDOMIZED_QUEUE` | 8 | Presale: seeded random pool. Post-sale: FIFO. |
| HYBRID_QUEUE | `HYBRID_QUEUE` | 8 | Same ordering as RANDOMIZED_QUEUE (presale pool + post-sale FIFO). |
| BALLOT | `BALLOT` | 10 | Random draw after registration window closes. Deferred. |
| INVITATION_ONLY | `INVITATION_ONLY` | 11 | Access via invitation code. Deferred. |
| PRIORITY_ACCESS | `PRIORITY_ACCESS` | 11 | Tiered access (e.g. member early access). Deferred. |
| WAITLIST_ONLY | `WAITLIST_ONLY` | 11 | Waitlist registration; no immediate checkout. Deferred. |
| CLOSED | `CLOSED` | 8 | Registration is closed; checkout returns `REGISTRATION_CLOSED`. |

## Mode Resolution

Resolution follows a **category-overrides-event** logic. The resolver is a pure function
in `registration/resolver.go`:

```
ResolveMode(ModeInput):
  1. If category override is enabled AND category has a mode set → return category mode
  2. If event has a mode set → return event mode
  3. Otherwise → return NORMAL (default)
```

This means:
- An event can run `NORMAL` but a single VIP category can run `BALLOT`.
- An event can run `WAR_QUEUE` globally and no category overrides are needed.
- If no settings rows exist at all, the default `NORMAL` preserves Phase 5 behaviour.

## RegistrationGate Seam

The `RegistrationGate` interface is defined in the orders package:

```go
// orders/gate.go
type RegistrationGate interface {
    Admit(ctx, participantID, eventID, categoryID, admissionToken) error
}
```

The orders `Service.Checkout` method calls `s.gate.Admit(...)` as the first step before
any inventory check or order creation. If admission is denied, the error propagates
immediately to the caller -- no resources are consumed, no transaction is opened.

The concrete implementation lives in `registration/gate.go`:

```
registration.Gate.Admit:
  1. ResolveForCheckout(eventID, categoryID) → mode
  2. Switch on mode:
     - NORMAL      → return nil (allow)
     - CLOSED      → return ErrClosed
     - WAR_QUEUE, RANDOMIZED_QUEUE, HYBRID_QUEUE → delegate to QueueAdmitter.CheckAdmission
     - BALLOT, INVITATION_ONLY, PRIORITY_ACCESS, WAITLIST_ONLY → return ErrModeNotAvailable
```

**NORMAL = regression-safe.** When mode resolves to `NORMAL` (no settings rows, or
explicit NORMAL), the gate returns `nil` immediately. This is the exact Phase 5 path:
no queue token needed, no queue check, no overhead.

## Error Codes

| Error | HTTP | Code | Trigger |
|---|---|---|---|
| `ErrClosed` | 409 | `REGISTRATION_CLOSED` | Mode = CLOSED |
| `ErrModeNotAvailable` | 409 | `REGISTRATION_MODE_NOT_AVAILABLE` | Mode = BALLOT, INVITATION_ONLY, PRIORITY_ACCESS, WAITLIST_ONLY |
| `ErrAdmissionRequired` | 403 | `ADMISSION_REQUIRED` | Queue mode but no valid admission token |
| `ErrAdmissionExpired` | 403 | `ADMISSION_EXPIRED` | Queue admission window expired |

## Deferred Modes

BALLOT, INVITATION_ONLY, PRIORITY_ACCESS, and WAITLIST_ONLY are defined in the enum and
validated as legal values in event/category settings (they can be saved), but the gate's
`Admit` method returns `REGISTRATION_MODE_NOT_AVAILABLE` for all of them. This unblocks
early configuration while preventing use before implementation.

## Event Settings Endpoints

```
PUT  /organizations/{orgId}/events/{eventId}/registration         — set event default mode
PUT  /organizations/{orgId}/events/{eventId}/registration/category — set per-category override
GET  /organizations/{orgId}/events/{eventId}/registration         — read settings
```

All endpoints require `registration.manage` permission.

### Event Settings Request

```json
{
  "defaultMode": "WAR_QUEUE",
  "queueEnabled": true,
  "ballotEnabled": false,
  "priorityEnabled": false,
  "waitlistEnabled": false
}
```

### Category Settings Request

```json
{
  "categoryId": "<uuid>",
  "registrationMode": "BALLOT",
  "overrideEnabled": true
}
```

`registrationMode` can be `null` to inherit the event default. `overrideEnabled` must
be `true` for the category mode to take effect -- settings rows where
`overrideEnabled = false` act as if no category override exists.

## Database

- `event_registration_settings` (migration 00020): one row per event, stores
  `default_mode`, feature-flag booleans, timestamps.
- `category_registration_settings` (migration 00020): per-category rows with
  `registration_mode` (nullable) and `override_enabled`.
- Permission `registration.manage` (migration 00021): assigned to Owner + Manager
  role templates.
