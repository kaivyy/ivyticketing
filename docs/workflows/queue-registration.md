# High-Traffic Queue Registration Workflow (`WAR_QUEUE` Mode)

This workflow traces the admission, waiting room, release, and checkout sequence for high-demand ticket sales operating under the `WAR_QUEUE` mode.

---

## 1. Sequence Diagram

```mermaid
sequenceDiagram
    autonumber
    actor User as Participant
    participant Web as Web Frontend (Astro Island)
    participant API as Chi Router (/api/v1)
    participant AG as AbuseGuard
    participant Redis as Redis 7 Engine
    participant Worker as Queue Release Daemon
    participant PG as PostgreSQL 16

    User->>Web: Click "Enter Registration"
    Web->>API: POST /api/v1/queue/{eventId}/join
    API->>AG: Verify Turnstile Token & IP Reputation
    AG-->>API: Verified
    API->>Redis: ZADD queue:{eventId}:waiting {timestamp} {userId}
    API->>PG: INSERT INTO queue_tokens (status = 'WAITING')
    API-->>Web: 200 OK (queueToken, position)

    loop Polling Every 2s (Redis Cached)
        Web->>API: GET /api/v1/queue/{eventId}/status?token=...
        API->>Redis: GET queue:status:{eventId}:{userId}
        Redis-->>API: Status: WAITING, Position: 42
        API-->>Web: 200 OK (position: 42, estimatedWaitSec: 25)
    end

    Note over Worker,Redis: Every 10s Release Cycle
    Worker->>Redis: Multi-Pop Top N: ZREM waiting -> ZADD allowed (TTL: 5m)
    Worker->>PG: UPDATE queue_tokens SET status = 'ALLOWED'

    Web->>API: GET /api/v1/queue/{eventId}/status?token=...
    API->>Redis: GET queue:status (Now ALLOWED)
    API-->>Web: 200 OK (status: ALLOWED, admissionToken, window: 300s)

    User->>Web: Select Category & Submit Checkout Form
    Web->>API: POST /api/v1/orders/checkout (Header: X-Admission-Token)
    API->>PG: BEGIN Transaction
    API->>PG: Validate & Consume Admission Token (status = 'CONSUMED')
    API->>PG: Lock Quota: SELECT ... FOR UPDATE
    API->>PG: INSERT INTO inventory_reservations (15m hold)
    API->>PG: INSERT INTO orders (status = 'PENDING_PAYMENT')
    API->>PG: COMMIT Transaction
    API->>Redis: ZREM queue:{eventId}:allowed {userId}
    API-->>Web: 201 Created (Order PENDING_PAYMENT, 15m payment timer)
```

---

## 2. Invariants and Safety Guarantees

1. **Admission Token Single-Use Guarantee**: An admission token can only be consumed once. Attempting to use the same token for multiple checkout calls returns `403 ADMISSION_EXPIRED`.
2. **5-Minute Checkout Expiration**: If a participant is released into the checkout room but takes longer than 5 minutes to submit their details, the admission token expires automatically and the slot is released to waiting participants.
3. **Sub-Millisecond Polling Defense**: Queue status requests are cached in Redis with a 2-second TTL. The database is never queried during the waiting room polling loop.
