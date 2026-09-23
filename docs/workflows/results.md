# Race Results Processing and Finisher Certificate Workflow

This workflow traces the ingestion of official timing data, category ranking calculations, and participant self-service certificate generation.

---

## 1. Sequence Diagram

```mermaid
sequenceDiagram
    autonumber
    actor Timing as Timing Operator
    actor Runner as Participant
    participant API as Chi HTTP Router
    participant Results as Results Service
    participant Tickets as Tickets Service
    participant PG as PostgreSQL 16
    participant Storage as Object Storage / Media

    Timing->>API: POST .../results/import (Multipart CSV file)
    API->>Results: IngestResultsCSV(eventId, csvReader)

    Results->>PG: BEGIN Transaction
    loop For each row in CSV
        Results->>PG: Match bib_number to tickets.id and participant_id
        Results->>PG: INSERT / UPDATE race_results (gun_time_ms, chip_time_ms, status)
        Results->>PG: INSERT INTO race_split_times (checkpoint_name, split_time_ms)
    end
    Results->>PG: COMMIT Transaction

    Results->>Results: RecalculateRankings(eventId)
    Results->>PG: Compute Overall, Gender, and Category Rankings by chip_time_ms
    Results-->>Timing: 200 OK (Imported 3,420 Results)

    Note over Runner,API: Participant Self-Service Certificate Generation
    Runner->>API: GET /api/v1/results/my/certificates/{ticketId}
    API->>Tickets: VerifyTicketOwnership(userId, ticketId)
    Tickets-->>API: Verified (eventId returned)

    API->>Results: GenerateCertificate(ticketId)
    Results->>PG: Fetch runner race_result, name, bib, category, times, and ranks
    Results->>Storage: Load Certificate SVG/PDF Template
    Results->>Results: Render dynamic text and cryptographic verification stamp
    Results-->>Runner: 200 OK (PDF Binary Stream)
```

---

## 2. Invariants and Safety Guarantees

1. **Ticket Ownership Gate**: Participants cannot access or generate certificates for other athletes. Handlers enforce ownership by calling [`ticketsSvc.GetTicketForUser`](file:///root/ivyticketing/services/api/internal/app/server.go#L314) before accessing results.
2. **Deterministic Ranking**: Rankings are calculated server-side based strictly on chip time for category and age group placements, and gun time for official podium placements.
3. **Audit Trail**: Every results file import or ranking adjustment is permanently logged in `audit_logs` with the operator's user ID and file checksum.
