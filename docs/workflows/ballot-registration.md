# Ballot Registration and Winner Conversion Workflow (`BALLOT` Mode)

This workflow details the complete lifecycle of a lottery-based registration: application, random draw, winner notification, checkout conversion, and the automated waitlist promotion fallback.

---

## 1. Sequence Diagram

```mermaid
sequenceDiagram
    autonumber
    actor Winner as Ballot Winner
    actor WaitlistUser as Waitlisted Applicant
    participant API as Chi Router
    participant Ballot as Ballot Service
    participant Expirer as Winner Expirer Daemon
    participant PG as PostgreSQL 16
    participant Notif as Notification Service

    Note over Winner,API: Phase 1: Application Window
    Winner->>API: POST /api/v1/ballots/{drawId}/apply
    API->>PG: INSERT INTO ballot_entries (status = 'APPLIED')
    WaitlistUser->>API: POST /api/v1/ballots/{drawId}/apply
    API->>PG: INSERT INTO ballot_entries (status = 'APPLIED')

    Note over API,PG: Phase 2: Lottery Draw Execution
    API->>Ballot: ExecuteDraw(drawId)
    Ballot->>PG: Run CSPRNG Selection for N Winners
    Ballot->>PG: UPDATE Winner entries: status = 'WINNER', payment_deadline = NOW() + 48h
    Ballot->>PG: UPDATE Non-Winners: status = 'WAITLISTED', waitlist_rank = 1, 2, ...
    Ballot->>PG: INSERT INTO access_grants (status = 'ACTIVE', expires_at = NOW() + 48h)
    Ballot->>Notif: Send "You Won the Ballot" Notification

    Note over Winner,PG: Phase 3A: Winner Payment (Happy Path)
    Winner->>API: POST /api/v1/orders/checkout (accessGrantId)
    API->>PG: Reserve inventory & INSERT order (PENDING_PAYMENT)
    Winner->>API: Complete Payment (Gateway Webhook)
    API->>PG: BEGIN Tx:
    API->>PG: UPDATE orders SET status = 'PAID'
    API->>PG: UPDATE ballot_entries SET status = 'CONVERTED'
    API->>PG: UPDATE access_grants SET status = 'CONSUMED'
    API->>PG: INSERT INTO tickets (status = 'VALID')
    API->>PG: COMMIT Tx

    Note over Expirer,WaitlistUser: Phase 3B: Unpaid Winner Lapse & Waitlist Cascade
    Note over Expirer: If Winner fails to pay within 48h:
    Expirer->>PG: SELECT * FROM ballot_entries WHERE status = 'WINNER' AND payment_deadline < NOW()
    Expirer->>PG: UPDATE ballot_entries SET status = 'LAPSED' WHERE id = :winner_id
    Expirer->>PG: Promote top WAITLISTED entrant: SET status = 'WINNER', payment_deadline = NOW() + 48h
    Expirer->>PG: INSERT INTO access_grants for newly promoted applicant
    Expirer->>Notif: Send "Waitlist Promotion" Notification to WaitlistUser
```

---

## 2. Invariants and Safety Guarantees

1. **Transactional Conversion**: The ballot winner status is updated to `CONVERTED` within the exact same database transaction that marks the order `PAID` and issues the digital ticket.
2. **Double-Allocation Prevention**: When an unpaid winner's entry lapses, their unused access grant is marked `EXPIRED` immediately before the promoted waitlisted participant's new grant is created.
3. **Auditability**: Every draw execution and promotion event generates an append-only entry in `audit_logs`.
