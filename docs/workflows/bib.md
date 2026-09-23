# BIB Allocation and Transponder Assignment Workflow

This workflow documents the batch sequential assignment of BIB numbers to paid participants, manual VIP overrides, and RFID transponder mapping.

---

## 1. Sequence Diagram: Batch Sequential Auto-Assignment

```mermaid
sequenceDiagram
    autonumber
    actor Organizer as Event Director
    participant API as Chi HTTP Router
    participant Tickets as Tickets Service
    participant PG as PostgreSQL 16

    Organizer->>API: POST .../tickets/auto-assign-bib (categoryId, rangeStart, rangeEnd, prefix)
    API->>Tickets: AutoAssignBIBs(categoryId, config)

    Tickets->>PG: BEGIN Transaction
    Tickets->>PG: SELECT current_bib FROM category_bib_sequences WHERE category_id = :id FOR UPDATE

    Tickets->>PG: SELECT id FROM tickets WHERE category_id = :id AND status = 'VALID' AND bib_number IS NULL ORDER BY created_at ASC FOR UPDATE

    loop For each unassigned ticket
        Tickets->>PG: Format BIB string: prefix + sequenceNumber
        Tickets->>PG: UPDATE tickets SET bib_number = :formatted_bib, bib_assignment_method = 'AUTO' WHERE id = :ticket_id
        Tickets->>PG: sequenceNumber = sequenceNumber + 1
    end

    Tickets->>PG: UPDATE category_bib_sequences SET current_bib = :sequenceNumber WHERE category_id = :id
    Tickets->>PG: COMMIT Transaction

    Tickets-->>API: Total Assigned Count
    API-->>Organizer: 200 OK (assignedCount: 1450)
```

---

## 2. Manual VIP and Elite BIB Override

For elite runners, pacers, or on-site expo replacements:

1. Organizer sends `PUT /api/v1/organizations/{orgId}/events/{eventId}/tickets/{ticketId}/bib` with payload `{"bibNumber": "ELITE-01"}`.
2. The service executes an event-wide uniqueness check:
   ```sql
   SELECT id FROM tickets
   WHERE event_id = :event_id
     AND bib_number = 'ELITE-01'
     AND id != :ticket_id
     AND status != 'CANCELLED';
   ```
3. If an existing ticket holds that number, returns `409 BIB_CONFLICT`.
4. Otherwise updates `tickets.bib_number = 'ELITE-01'` and `bib_assignment_method = 'MANUAL'`.
5. An audit log event is recorded with the previous and new BIB numbers.
