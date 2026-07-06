package scanner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/racepack"
	"github.com/varin/ivyticketing/services/api/internal/modules/tickets/qr"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

// QRVerifier validates a QR token's HMAC signature and returns the decoded
// ticket reference. The tickets/qr.Signer satisfies this interface.
type QRVerifier interface {
	Verify(token string) (qr.TicketRef, error)
}

// TicketReader exposes a read surface over the tickets module for non-sensitive
// display fields. Implemented by an injected adapter so the scanner never
// touches another module's tables directly.
type TicketReader interface {
	// GetDisplayInfo returns non-sensitive participant display fields.
	GetDisplayInfo(ctx context.Context, ticketID uuid.UUID) (DisplayInfo, error)
}

// PickupExecutor is the racepack surface the scanner composes. The scanner
// reads pickup status for the verify duplicate flags and replays offline
// pickups through the existing racepack endpoint. racepack.Service satisfies
// this interface.
type PickupExecutor interface {
	GetPickupStatusByTicket(ctx context.Context, ticketID uuid.UUID) (db.RacepackPickupRecord, error)
}

// Compile-time assertion that racepack.Service satisfies PickupExecutor, so it
// can be injected as the scanner's pickup collaborator at wiring time (task
// 7.2 server.go wiring). The scanner reuses racepack for pickups: it reads
// pickup status here for the Verify duplicate flags, and offline pickup ops are
// replayed to the EXISTING POST /racepack/pickups endpoint (which already
// carries Idempotency-Key support, scope "racepack.execute_pickup"). The
// scanner deliberately owns NO pickup-creation logic of its own.
var _ PickupExecutor = (*racepack.Service)(nil)

// AuditRecorder is the audit surface the service needs. The platform
// audit.Logger satisfies it. The recorder may be nil, in which case audit
// writes are skipped.
type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

// Service composes QR verification, ticket reads, racepack pickups, and the
// check-in transition. It owns no tables of its own except the shared
// idempotency_keys / audit_logs via injected collaborators.
type Service struct {
	qr       QRVerifier
	tickets  TicketReader
	racepack PickupExecutor
	repo     Repository
	audit    AuditRecorder
	log      *slog.Logger
}

// NewService constructs a Service. auditRecorder and log may be nil.
func NewService(
	verifier QRVerifier,
	tickets TicketReader,
	racepack PickupExecutor,
	repo Repository,
	auditRecorder AuditRecorder,
	log *slog.Logger,
) *Service {
	return &Service{
		qr:       verifier,
		tickets:  tickets,
		racepack: racepack,
		repo:     repo,
		audit:    auditRecorder,
		log:      log,
	}
}

// --- idempotency ---
//
// This mirrors the platform idempotency mechanism the racepack module uses for
// POST /pickups (shared idempotency_keys table via GetIdempotencyKey /
// InsertIdempotencyKey). The only difference is the scanner check-in scope
// string (model.IdempotencyScopeCheckin = "scanner.checkin").

// IdempotencyHit is the result of looking up an Idempotency-Key.
type IdempotencyHit struct {
	Found        bool
	RequestHash  string
	Status       int32
	ResponseBody []byte
}

// LookupIdempotency checks if a key was previously stored in `scope`. Returns
// nil if no hit; otherwise returns the cached request hash + response body.
func (s *Service) LookupIdempotency(ctx context.Context, key, scope string) (*IdempotencyHit, error) {
	if key == "" {
		return nil, nil
	}
	row, err := s.repo.GetIdempotencyKey(ctx, db.GetIdempotencyKeyParams{Key: key, Scope: scope})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &IdempotencyHit{
		Found:        true,
		RequestHash:  row.RequestHash,
		Status:       row.ResponseStatus,
		ResponseBody: row.ResponseBody,
	}, nil
}

// StoreIdempotency persists a (key, scope, request_hash, response) tuple.
// Best-effort — failures are logged but do not fail the caller (the operation
// itself already succeeded).
func (s *Service) StoreIdempotency(ctx context.Context, key, scope, requestHash string, status int32, responseBody []byte) {
	if key == "" {
		return
	}
	_, err := s.repo.InsertIdempotencyKey(ctx, db.InsertIdempotencyKeyParams{
		Key:            key,
		Scope:          scope,
		RequestHash:    requestHash,
		ResponseStatus: status,
		ResponseBody:   responseBody,
	})
	if err != nil && s.log != nil {
		s.log.Warn("idempotency: store failed", "scope", scope, "err", err)
	}
}

// HashRequest canonicalises a request (method, path, body) for idempotency
// comparison. Identical to the racepack helper so behaviour matches exactly.
func HashRequest(method, path string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte{0})
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// --- permitted-event resolution & authorization ---

// ListPermittedEvents returns the staff member's Permitted_Events: the events
// (across all of their organizations) for which they hold racepack.execute or
// checkin.execute. The returned DTOs carry only non-sensitive identity fields
// (Req 1.2).
func (s *Service) ListPermittedEvents(ctx context.Context, staffID uuid.UUID) ([]PermittedEvent, error) {
	rows, err := s.repo.ListScannableEventsForUser(ctx, staffID)
	if err != nil {
		return nil, err
	}
	out := make([]PermittedEvent, 0, len(rows))
	for _, e := range rows {
		out = append(out, PermittedEvent{
			EventID:        e.ID.String(),
			OrganizationID: e.OrganizationID.String(),
			Name:           e.Name,
			Status:         e.Status,
		})
	}
	return out, nil
}

// AssertEventInOrg verifies that the event actually belongs to the org in the
// URL. Defense-in-depth mirroring racepack.Service.AssertEventInOrg — the route
// middleware already enforces org membership, but this catches the case where a
// session valid in org A points at org B's event. A missing/mismatched event is
// reported as ErrEventMismatch so org boundaries are not leaked.
func (s *Service) AssertEventInOrg(ctx context.Context, eventID, expectedOrgID uuid.UUID) error {
	actualOrg, err := s.repo.GetEventOrganizationID(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEventMismatch
		}
		return err
	}
	if actualOrg != expectedOrgID {
		return ErrEventMismatch
	}
	return nil
}

// AssertEventPermitted enforces per-operation authorization: the staff member
// must hold racepack.execute or checkin.execute in the org that owns the target
// event, else ErrUnauthorizedEvent (Req 1.4, 1.5). This is defense-in-depth
// behind the route RBAC middleware (task 7.1/7.2), analogous to racepack's
// AssertEventInOrg — a scan operation must never touch a non-permitted event
// even if a route were ever mounted without the middleware.
func (s *Service) AssertEventPermitted(ctx context.Context, staffID, eventID uuid.UUID) error {
	ok, err := s.repo.UserCanScanEvent(ctx, eventID, staffID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrUnauthorizedEvent
	}
	return nil
}

// VerifyInput is the decoded token plus the selected event and actor context.
type VerifyInput struct {
	Token   string
	EventID uuid.UUID
	OrgID   uuid.UUID
	StaffID uuid.UUID
}

// Verify validates the QR signature (authoritative HMAC), confirms the ticket
// belongs to the selected event, and returns non-sensitive display info plus
// duplicate flags.
//
// Steps:
//  1. qr.Verify(token) → reject with a typed sentinel; a rejection during an
//     online scan writes a best-effort SCANNER_QR_REJECTED audit entry.
//  2. Assert ref.EventID == in.EventID, else ErrEventMismatch.
//  3. Load DisplayInfo (whitelisted fields; maps ErrTicketNotFound through).
//  4. Compute alreadyPickedUp (+ pickedUpAt) from the racepack pickup status and
//     alreadyCheckedIn (+ checkedInAt) from the ticket status/used_at.
func (s *Service) Verify(ctx context.Context, in VerifyInput) (VerifyResult, error) {
	// Per-operation authorization (Req 1.4, 1.5): the staff must hold a scanning
	// permission for the selected event, else reject before any read. This is
	// defense-in-depth behind the route RBAC middleware (task 7.1/7.2).
	if err := s.AssertEventPermitted(ctx, in.StaffID, in.EventID); err != nil {
		return VerifyResult{}, err
	}

	ref, err := s.qr.Verify(in.Token)
	if err != nil {
		mapped := mapQRError(err)
		// Best-effort rejection audit for the online scan path (Req 10.4).
		// The token failed HMAC verification so there is no trusted ticket id;
		// we record the actor (ActorUserID), org, selected event, the rejection
		// reason, and the time so the rejected attempt can be traced.
		s.recordAudit(ctx, in.OrgID, in.StaffID, AuditActionQRRejected, "ticket", "", map[string]any{
			"reason":   mapped.Error(),
			"event_id": in.EventID.String(),
			"at":       time.Now().UTC().Format(time.RFC3339),
		})
		return VerifyResult{}, mapped
	}

	// The ticket must belong to the event the staff selected.
	if ref.EventID != in.EventID {
		return VerifyResult{}, ErrEventMismatch
	}

	// Whitelisted display fields (name, BIB, category, ticket status).
	display, err := s.tickets.GetDisplayInfo(ctx, ref.TicketID)
	if err != nil {
		return VerifyResult{}, err
	}

	// Read the ticket row for the check-in flag + original timestamp.
	ticket, err := s.repo.GetTicketByID(ctx, ref.TicketID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VerifyResult{}, ErrTicketNotFound
		}
		return VerifyResult{}, err
	}
	alreadyCheckedIn := ticket.Status == TicketStatusUsed
	var checkedInAt *time.Time
	if alreadyCheckedIn && ticket.UsedAt.Valid {
		t := ticket.UsedAt.Time
		checkedInAt = &t
	}

	// Pickup duplicate flag. GetPickupStatusByTicket returns the active
	// PICKED_UP record, or ErrTicketNotFound / pgx.ErrNoRows when none exists.
	alreadyPickedUp := false
	var pickedUpAt *time.Time
	rec, err := s.racepack.GetPickupStatusByTicket(ctx, ref.TicketID)
	switch {
	case err == nil:
		alreadyPickedUp = true
		if rec.PickupTimestamp.Valid {
			t := rec.PickupTimestamp.Time
			pickedUpAt = &t
		}
	case errors.Is(err, racepack.ErrTicketNotFound), errors.Is(err, pgx.ErrNoRows):
		// No active pickup record → not picked up.
	default:
		return VerifyResult{}, err
	}

	return VerifyResult{
		TicketID:         ref.TicketID.String(),
		EventID:          ref.EventID.String(),
		Display:          display,
		AlreadyPickedUp:  alreadyPickedUp,
		PickedUpAt:       pickedUpAt,
		AlreadyCheckedIn: alreadyCheckedIn,
		CheckedInAt:      checkedInAt,
	}, nil
}

// mapQRError translates the qr package's sentinel errors into the scanner's
// error vocabulary so callers use a single set of sentinels.
func mapQRError(err error) error {
	switch {
	case errors.Is(err, qr.ErrInvalidSignature):
		return ErrSignatureInvalid
	case errors.Is(err, qr.ErrUnsupportedVersion):
		return ErrUnsupportedVersion
	case errors.Is(err, qr.ErrMalformedToken):
		return ErrMalformedToken
	default:
		// Unknown verification failures are treated as malformed rather than
		// leaking an internal error to the client.
		return ErrMalformedToken
	}
}

// CheckInInput bundles the parameters for CheckIn.
type CheckInInput struct {
	OrgID     uuid.UUID
	EventID   uuid.UUID
	TicketID  uuid.UUID
	StaffID   uuid.UUID
	ScannedAt *time.Time // original offline scan time; nil = server now()
}

// CheckIn transitions a VALID ticket to USED atomically and idempotently.
//
// It mirrors racepack.ExecutePickup's structure: inside one transaction it
// locks the ticket row (closing the TOCTOU window), re-checks status/event on
// the locked row, and performs the guarded transition. The audit write happens
// AFTER the transaction commits so an audit failure never rolls back the
// check-in (best-effort, Req 10.2/10.3).
//
// Steps inside the transaction:
//  1. LockTicketForUpdate the ticket row; missing row → ErrTicketNotFound.
//  2. status == CANCELLED → ErrTicketCancelled.
//  3. ticket.EventID != in.EventID → ErrEventMismatch.
//  4. status == USED → duplicate: no transition, return existing used_at.
//  5. status == VALID → MarkTicketUsed with used_at = COALESCE(ScannedAt, now()).
func (s *Service) CheckIn(ctx context.Context, in CheckInInput) (CheckInResult, error) {
	// Per-operation authorization (Req 1.4, 1.5): reject check-ins targeting a
	// non-permitted event before any state change. Defense-in-depth behind the
	// route RBAC middleware (task 7.1/7.2).
	if err := s.AssertEventPermitted(ctx, in.StaffID, in.EventID); err != nil {
		return CheckInResult{}, err
	}

	var (
		checkedInAt time.Time
		duplicate   bool
	)

	err := s.repo.ExecTx(ctx, func(tx Repository) error {
		// Step 1: lock the ticket row to close the TOCTOU window.
		t, err := tx.LockTicketForUpdate(ctx, in.TicketID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrTicketNotFound
			}
			return err
		}

		// Step 2: cancelled tickets can never be checked in.
		if t.Status == TicketStatusCancelled {
			return ErrTicketCancelled
		}

		// Step 3: the ticket must belong to the selected event.
		if t.EventID != in.EventID {
			return ErrEventMismatch
		}

		// Step 4: already USED → treat as a duplicate. Do NOT transition again;
		// surface the existing used_at so the UI shows a Duplicate_Warning.
		if t.Status == TicketStatusUsed {
			duplicate = true
			if t.UsedAt.Valid {
				checkedInAt = t.UsedAt.Time
			}
			return nil
		}

		// Step 5: VALID → USED. used_at = COALESCE(ScannedAt, now()); a nil
		// ScannedAt yields an invalid Timestamptz so the SQL COALESCE falls to
		// now().
		usedAt := pgtype.Timestamptz{Valid: false}
		if in.ScannedAt != nil {
			usedAt = pgtype.Timestamptz{Time: *in.ScannedAt, Valid: true}
		}
		updated, err := tx.MarkTicketUsed(ctx, db.MarkTicketUsedParams{
			ID:     in.TicketID,
			UsedAt: usedAt,
		})
		if err != nil {
			// The guarded UPDATE (WHERE status='VALID') returns no rows if the
			// row was not VALID. Since we locked + checked status==VALID above
			// this should not happen, but handle it defensively rather than
			// leaking pgx.ErrNoRows.
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAlreadyCheckedIn
			}
			return err
		}
		if updated.UsedAt.Valid {
			checkedInAt = updated.UsedAt.Time
		}
		return nil
	})
	if err != nil {
		return CheckInResult{}, err
	}

	// Best-effort audit OUTSIDE the tx so an audit failure never rolls back the
	// check-in (Req 10.2). The entry carries the ticket id (metadata + TargetID),
	// the staff id (ActorUserID), and a timestamp. For offline-synced ops the
	// metadata "at" is the ORIGINAL scan time (ScannedAt), matching the ticket's
	// used_at set inside the tx above; online scans default to server now()
	// (Req 10.3).
	at := time.Now().UTC()
	if in.ScannedAt != nil {
		at = in.ScannedAt.UTC()
	}
	s.recordAudit(ctx, in.OrgID, in.StaffID, AuditActionCheckinCompleted, "ticket", in.TicketID.String(), map[string]any{
		"ticket_id": in.TicketID.String(),
		"duplicate": duplicate,
		"at":        at.Format(time.RFC3339),
	})

	return CheckInResult{
		TicketID:    in.TicketID.String(),
		Status:      TicketStatusUsed,
		CheckedInAt: checkedInAt,
		Duplicate:   duplicate,
	}, nil
}

// recordAudit is a tiny helper that no-ops when audit is nil. Best-effort:
// audit failures never roll back the user-facing action.
func (s *Service) recordAudit(ctx context.Context, orgID, actorID uuid.UUID, action, targetType, targetID string, meta map[string]any) {
	if s.audit == nil {
		return
	}
	org := orgID
	actor := actorID
	s.audit.Record(ctx, audit.Entry{
		OrganizationID: &org,
		ActorUserID:    &actor,
		Action:         action,
		TargetType:     targetType,
		TargetID:       targetID,
		Metadata:       meta,
	})
}
