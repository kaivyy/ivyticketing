# Requirements Document

## Introduction

Phase 15 delivers the **Scanner PWA**: a fast, installable, offline-capable
Progressive Web App used by on-site staff to process participants during
racepack distribution and event check-in. Staff point a phone camera at a
participant's signed QR ticket, see the participant's information, and confirm
either a racepack pickup or a check-in. The app must keep working when the
venue network is unreliable — scans are queued locally and synchronised to the
server when connectivity returns, without ever producing duplicate pickups or
check-ins.

The Scanner PWA builds on existing platform building blocks:

- **Phase 7 tickets/qr**: HMAC-SHA256 signed QR tokens of the form
  `<version>.<base64url(payload)>.<base64url(hmac)>`, carrying only
  `ticket_id`, `event_id`, and a schema `version` (no PII). Ticket states are
  `VALID → USED` (check-in scan) and `VALID → CANCELLED` (refund). The QR
  verify/scan endpoint was explicitly deferred to Phase 15.
- **Phase 14/14.1 racepack**: `ExecutePickup` provides a TOCTOU-safe pickup
  path with `Idempotency-Key` support, slot enforcement, and a unique partial
  index that prevents duplicate `PICKED_UP` records.
- **RBAC (Phase 2)**: `racepack.execute` and `racepack.problemdesk`
  permissions, plus multi-tenant isolation ensuring staff only reach events
  their organization membership permits.

This document defines the requirements only. Design and implementation tasks
follow in later phases.

Constitutional constraints that shape these requirements:

- Scanner lives in `apps/scanner`; technology is **Vite + Svelte 5 + PWA**
  (React/Next/Vue/SvelteKit forbidden).
- QR signature validation is **server-side**; forged or tampered QR is
  rejected.
- Every scan, pickup, and check-in is **audited**.
- Correctness and no-duplicates take priority over raw speed.

## Glossary

- **Scanner_PWA**: The client-side Progressive Web App in `apps/scanner`
  (Vite + Svelte 5) that staff install and run on a phone.
- **Scan_API**: The server-side API surface that verifies QR tokens and
  records racepack pickups and check-ins.
- **QR_Verifier**: The server-side component that validates the HMAC-SHA256
  signature and decodes a QR token into a ticket reference.
- **QR_Token**: The signed string encoded in a participant's QR ticket,
  containing `ticket_id`, `event_id`, and a schema version only.
- **Staff_User**: An authenticated organization member holding the
  `racepack.execute` permission for at least one event.
- **Permitted_Event**: An event belonging to an organization in which the
  Staff_User holds the required scanning permission.
- **Racepack_Pickup**: The action of recording that a participant collected
  their racepack, creating a `PICKED_UP` pickup record.
- **Check_In**: The action of marking a `VALID` ticket as `USED` at event
  entry.
- **Scan_Operation**: A single queued unit of work representing either a
  Racepack_Pickup or a Check_In for one ticket.
- **Offline_Queue**: The ordered local store of Scan_Operations awaiting
  synchronisation to the Scan_API.
- **Local_Store**: The browser-persistent storage (e.g. IndexedDB) that holds
  the Offline_Queue and cached data across app restarts.
- **Sync_Engine**: The Scanner_PWA component that transmits queued
  Scan_Operations to the Scan_API when connectivity is available.
- **Idempotency_Key**: A client-generated unique identifier attached to each
  Scan_Operation so that retried transmissions do not create duplicates.
- **Duplicate_Warning**: A user-facing notice shown when a ticket has already
  been picked up or already been checked in.

## Requirements

### Requirement 1: Staff Authentication Scoped to Permitted Events

**User Story:** As a racepack staff member, I want to log in and see only the
events I am allowed to scan, so that I cannot accidentally or intentionally
process participants for events outside my organization.

#### Acceptance Criteria

1. WHEN a Staff_User submits valid credentials, THE Scanner_PWA SHALL establish an authenticated session and store the session token in Local_Store.
2. WHEN an authenticated session is established, THE Scan_API SHALL return only the set of Permitted_Events for which the Staff_User holds the `racepack.execute` permission.
3. IF a Staff_User submits invalid credentials, THEN THE Scanner_PWA SHALL reject the login and display an authentication error message.
4. WHEN a Staff_User selects an event to scan, THE Scan_API SHALL confirm the event is a Permitted_Event before allowing any Scan_Operation.
5. IF a Scan_Operation targets an event that is not a Permitted_Event for the Staff_User, THEN THE Scan_API SHALL reject the operation with an authorization error.
6. WHEN a Staff_User logs out, THE Scanner_PWA SHALL clear the session token from Local_Store.

### Requirement 2: QR Scanning and Server-Side Signature Validation

**User Story:** As a racepack staff member, I want the app to read a
participant's QR ticket and confirm it is genuine, so that forged or altered
tickets are rejected.

#### Acceptance Criteria

1. WHEN a Staff_User points the device camera at a QR_Token, THE Scanner_PWA SHALL decode the QR_Token string from the camera image.
2. WHEN a QR_Token is decoded and connectivity is available, THE QR_Verifier SHALL validate the HMAC-SHA256 signature server-side before returning participant information.
3. IF the QR_Token signature is invalid, THEN THE QR_Verifier SHALL reject the QR_Token and return a signature-invalid error.
4. IF the QR_Token is malformed or uses an unsupported schema version, THEN THE QR_Verifier SHALL reject the QR_Token and return a validation error.
5. IF a decoded QR_Token references a ticket that does not belong to the selected Permitted_Event, THEN THE Scan_API SHALL reject the Scan_Operation with an event-mismatch error.
6. WHEN a QR_Token is rejected for any reason, THE Scanner_PWA SHALL display a rejection message and SHALL NOT record a Racepack_Pickup or Check_In.

### Requirement 3: Participant Information Display

**User Story:** As a racepack staff member, I want to see the participant's key
details after a successful scan, so that I can verify identity and hand over
the correct racepack.

#### Acceptance Criteria

1. WHEN a QR_Token is validated successfully, THE Scanner_PWA SHALL display the participant name, BIB number, category, and current ticket status.
2. WHEN participant information is displayed, THE Scanner_PWA SHALL indicate whether the ticket has already been picked up.
3. WHEN participant information is displayed, THE Scanner_PWA SHALL indicate whether the ticket has already been checked in.
4. THE Scanner_PWA SHALL NOT display payment card data, passwords, or full contact details beyond what is required to confirm identity for pickup.

### Requirement 4: Racepack Pickup Confirmation

**User Story:** As a racepack staff member, I want to confirm a racepack pickup
after verifying the participant, so that collection is recorded accurately.

#### Acceptance Criteria

1. WHEN a Staff_User confirms a Racepack_Pickup for an eligible ticket, THE Scan_API SHALL create exactly one `PICKED_UP` pickup record for that ticket.
2. IF a ticket presented for Racepack_Pickup is `CANCELLED`, THEN THE Scan_API SHALL reject the pickup with a ticket-cancelled error.
3. IF a ticket presented for Racepack_Pickup has no assigned BIB number, THEN THE Scan_API SHALL reject the pickup with a BIB-missing error.
4. IF the order associated with a ticket presented for Racepack_Pickup is not `PAID`, THEN THE Scan_API SHALL reject the pickup with an order-not-paid error.
5. WHEN a Racepack_Pickup is confirmed, THE Scanner_PWA SHALL display a success confirmation to the Staff_User.

### Requirement 5: Check-In Confirmation

**User Story:** As an event check-in staff member, I want to check a
participant in at event entry, so that attendance is recorded and re-entry
misuse is detectable.

#### Acceptance Criteria

1. WHEN a Staff_User confirms a Check_In for a `VALID` ticket, THE Scan_API SHALL transition the ticket status from `VALID` to `USED`.
2. IF a ticket presented for Check_In is `CANCELLED`, THEN THE Scan_API SHALL reject the Check_In with a ticket-cancelled error.
3. WHEN a Check_In is confirmed, THE Scanner_PWA SHALL display a success confirmation to the Staff_User.
4. WHERE the selected scanning mode is Check_In, THE Scanner_PWA SHALL present Check_In as the confirmation action rather than Racepack_Pickup.

### Requirement 6: Duplicate Detection and Warning

**User Story:** As a racepack staff member, I want to be warned when a ticket
has already been processed, so that I do not hand out a racepack twice or admit
a participant twice.

#### Acceptance Criteria

1. IF a ticket presented for Racepack_Pickup already has a `PICKED_UP` record, THEN THE Scanner_PWA SHALL display a Duplicate_Warning identifying the ticket as already picked up.
2. IF a ticket presented for Check_In already has status `USED`, THEN THE Scanner_PWA SHALL display a Duplicate_Warning identifying the ticket as already checked in.
3. WHEN a Duplicate_Warning is displayed, THE Scan_API SHALL NOT create an additional pickup record or additional status transition for the ticket.
4. WHEN a Duplicate_Warning is displayed, THE Scanner_PWA SHALL show the timestamp of the original Racepack_Pickup or Check_In.

### Requirement 7: Offline Mode with Local Queue and Persistence

**User Story:** As a racepack staff member working in a venue with poor
connectivity, I want scans to keep working offline, so that participant
processing is not blocked by network outages.

#### Acceptance Criteria

1. WHILE the device is offline, THE Scanner_PWA SHALL validate the QR_Token signature locally using a cached verification key and enqueue the resulting Scan_Operation in the Offline_Queue.
2. WHEN a Scan_Operation is enqueued, THE Scanner_PWA SHALL assign a unique Idempotency_Key to that Scan_Operation.
3. WHEN a Scan_Operation is enqueued, THE Local_Store SHALL persist the Scan_Operation so that it survives an application restart.
4. WHILE the device is offline, THE Scanner_PWA SHALL detect duplicates against locally cached ticket state and display a Duplicate_Warning when a ticket has already been processed on the same device.
5. WHILE the device is offline, THE Scanner_PWA SHALL display the count of pending Scan_Operations in the Offline_Queue.
6. WHEN the application restarts, THE Scanner_PWA SHALL restore all unsynchronised Scan_Operations from Local_Store into the Offline_Queue.

### Requirement 8: Background Synchronisation Without Duplicates

**User Story:** As an event operator, I want offline scans to sync
automatically when the network returns, so that server records are complete and
never duplicated.

#### Acceptance Criteria

1. WHEN connectivity is restored, THE Sync_Engine SHALL transmit each pending Scan_Operation from the Offline_Queue to the Scan_API.
2. WHEN the Sync_Engine transmits a Scan_Operation, THE Sync_Engine SHALL include the Scan_Operation's Idempotency_Key in the request.
3. IF the same Scan_Operation is transmitted more than once with the same Idempotency_Key, THEN THE Scan_API SHALL record the operation exactly once and return the original result for subsequent transmissions.
4. WHEN the Scan_API acknowledges a Scan_Operation, THE Sync_Engine SHALL remove that Scan_Operation from the Offline_Queue and Local_Store.
5. IF transmission of a Scan_Operation fails due to a network error, THEN THE Sync_Engine SHALL retain the Scan_Operation in the Offline_Queue for a later retry.
6. IF the Scan_API rejects a Scan_Operation for a non-retryable reason, THEN THE Sync_Engine SHALL move the Scan_Operation to a failed state and surface it to the Staff_User for manual resolution.

### Requirement 9: PWA Installability and Performance

**User Story:** As a racepack staff member, I want to install the scanner on a
normal phone and process each participant quickly, so that queues move fast on
event day.

#### Acceptance Criteria

1. THE Scanner_PWA SHALL provide a web app manifest and service worker so that the application is installable on a standard mobile device.
2. WHEN the Scanner_PWA is installed and launched, THE Scanner_PWA SHALL load the scanning interface using cached assets without requiring a network connection.
3. WHEN a QR_Token is validated online, THE Scan_API SHALL return participant information within 3 seconds under normal event-day load.
4. THE Scanner_PWA SHALL enable a Staff_User to complete a scan-to-confirmation cycle for one participant within 10 seconds.

### Requirement 10: Audit Logging of Scans, Pickups, and Check-Ins

**User Story:** As an event operator, I want every scan action recorded, so
that pickup and check-in activity is traceable and disputes can be resolved.

#### Acceptance Criteria

1. WHEN a Racepack_Pickup is recorded, THE Scan_API SHALL write an audit entry containing the ticket identifier, the Staff_User identifier, and the timestamp.
2. WHEN a Check_In is recorded, THE Scan_API SHALL write an audit entry containing the ticket identifier, the Staff_User identifier, and the timestamp.
3. WHEN an offline Scan_Operation is synchronised, THE Scan_API SHALL record the audit entry with the timestamp of the original offline scan.
4. IF a QR_Token is rejected for an invalid signature during an online scan, THEN THE Scan_API SHALL write an audit entry recording the rejected scan attempt.
