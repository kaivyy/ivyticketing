package racepack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

// AuditRecorder is the audit surface the service needs. The platform
// audit.Logger satisfies it. recorder may be nil.
type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

// IdempotencyHit is the result of looking up an Idempotency-Key.
type IdempotencyHit struct {
	Found        bool
	RequestHash  string
	Status       int32
	ResponseBody []byte
}

// Service owns the business logic for the racepack module.
type Service struct {
	repo  Repository
	audit AuditRecorder
	log   *slog.Logger
}

// NewService constructs a Service. recorder and log may be nil.
func NewService(repo Repository, auditRecorder AuditRecorder, log *slog.Logger) *Service {
	return &Service{repo: repo, audit: auditRecorder, log: log}
}

// --- idempotency ---

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
// Best-effort — failures are logged but do not fail the caller (the
// operation itself already succeeded).
func (s *Service) StoreIdempotency(ctx context.Context, key, scope, requestHash string, status int32, responseBody []byte) {
	if key == "" {
		return
	}
	_, err := s.repo.InsertIdempotencyKey(ctx, db.InsertIdempotencyKeyParams{
		Key:           key,
		Scope:         scope,
		RequestHash:   requestHash,
		ResponseStatus: status,
		ResponseBody:  responseBody,
	})
	if err != nil && s.log != nil {
		s.log.Warn("idempotency: store failed", "scope", scope, "err", err)
	}
}

// HashRequest canonicalises a JSON body for idempotency comparison.
func HashRequest(method, path string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte{0})
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// --- ownership helpers ---

// AssertEventInOrg verifies that the event in the URL actually belongs to the
// org in the URL. Defense-in-depth — the route middleware already enforces
// org membership, but this catches the case where a staff member's session
// is valid in org A but the URL points at org B's event.
func (s *Service) AssertEventInOrg(ctx context.Context, eventID, expectedOrgID uuid.UUID) error {
	actualOrg, err := s.repo.GetEventOrganizationID(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTicketNotFound // generic not-found so we don't leak org boundaries
		}
		return err
	}
	if actualOrg != expectedOrgID {
		return ErrTicketNotFound
	}
	return nil
}

// AssertTicketInEvent verifies that the ticket belongs to the event.
// Used by ExecutePickup, CreateProxyAuthorization, CreateProblemCase.
func (s *Service) AssertTicketInEvent(ctx context.Context, ticketID, eventID uuid.UUID) error {
	_, eid, _, _, found, err := s.repo.GetTicketStatus(ctx, ticketID)
	if err != nil {
		return err
	}
	if !found {
		return ErrTicketNotFound
	}
	if eid != eventID {
		return ErrTicketEventMismatch
	}
	return nil
}

// AssertCounterInEvent verifies counter.event_id matches the event in the URL.
func (s *Service) AssertCounterInEvent(ctx context.Context, counterID, eventID uuid.UUID) (db.RacepackCounter, error) {
	c, err := s.repo.GetCounterByID(ctx, counterID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.RacepackCounter{}, ErrCounterNotFound
		}
		return db.RacepackCounter{}, err
	}
	if c.EventID != eventID {
		return db.RacepackCounter{}, ErrCounterEventMismatch
	}
	if !c.Active {
		return db.RacepackCounter{}, ErrCounterInactive
	}
	return c, nil
}

// --- counters ---

func (s *Service) CreateCounter(ctx context.Context, orgID, eventID uuid.UUID, name, location string, active bool) (db.RacepackCounter, error) {
	return s.repo.CreateCounter(ctx, db.CreateRacepackCounterParams{
		OrganizationID: orgID,
		EventID:        eventID,
		Name:           name,
		Location:       pgtypeText(location),
		Active:         active,
	})
}

func (s *Service) ListCounters(ctx context.Context, eventID uuid.UUID) ([]db.RacepackCounter, error) {
	return s.repo.ListCountersByEvent(ctx, eventID)
}

func (s *Service) UpdateCounter(ctx context.Context, id uuid.UUID, name, location string, active bool) (db.RacepackCounter, error) {
	return s.repo.UpdateCounter(ctx, db.UpdateRacepackCounterParams{
		ID:       id,
		Name:     name,
		Location: pgtypeText(location),
		Active:   active,
	})
}

func (s *Service) SetCounterActive(ctx context.Context, id uuid.UUID, active bool) (db.RacepackCounter, error) {
	return s.repo.SetCounterActive(ctx, id, active)
}

// --- slots ---

func (s *Service) CreateSlot(ctx context.Context, orgID, eventID uuid.UUID, name string, pickupDate time.Time, start, end time.Time, capacity int32) (db.RacepackPickupSlot, error) {
	return s.repo.CreateSlot(ctx, db.CreateRacepackPickupSlotParams{
		OrganizationID: orgID,
		EventID:        eventID,
		Name:           name,
		PickupDate:     pgtypeDate(pickupDate),
		StartTime:      pgtypeTimestamptz(start),
		EndTime:        pgtypeTimestamptz(end),
		Capacity:       capacity,
	})
}

func (s *Service) ListSlots(ctx context.Context, eventID uuid.UUID) ([]db.RacepackPickupSlot, error) {
	return s.repo.ListSlotsByEvent(ctx, eventID)
}

func (s *Service) ListActiveSlots(ctx context.Context, eventID uuid.UUID) ([]db.RacepackPickupSlot, error) {
	return s.repo.ListActiveSlotsByEvent(ctx, eventID)
}

func (s *Service) UpdateSlot(ctx context.Context, id uuid.UUID, name string, pickupDate time.Time, start, end time.Time, capacity int32, active bool) (db.RacepackPickupSlot, error) {
	return s.repo.UpdateSlot(ctx, db.UpdateRacepackPickupSlotParams{
		ID:         id,
		Name:       name,
		PickupDate: pgtypeDate(pickupDate),
		StartTime:  pgtypeTimestamptz(start),
		EndTime:    pgtypeTimestamptz(end),
		Capacity:   capacity,
		Active:     active,
	})
}

// ReserveSlot atomically reserves a slot for a participant. Returns
// ErrSlotFull if reserved_count would exceed capacity. Used by the
// participant-facing reservation endpoint.
func (s *Service) ReserveSlot(ctx context.Context, slotID uuid.UUID) (db.RacepackPickupSlot, error) {
	slot, err := s.repo.IncrementSlotReserved(ctx, slotID)
	if err != nil {
		if errorsIsNoRows(err) {
			return db.RacepackPickupSlot{}, ErrSlotFull
		}
		return db.RacepackPickupSlot{}, err
	}
	return slot, nil
}

// ReleaseSlot decrements reserved_count. No-op if already 0 (DB-level clamp).
func (s *Service) ReleaseSlot(ctx context.Context, slotID uuid.UUID) error {
	return s.repo.DecrementSlotReserved(ctx, slotID)
}

// --- pickup methods (the hot path) ---

// ExecutePickupInput bundles the parameters for ExecutePickup.
type ExecutePickupInput struct {
	OrgID    uuid.UUID
	EventID  uuid.UUID
	TicketID uuid.UUID
	CounterID uuid.UUID
	SlotID    *uuid.UUID // optional — nil means no slot enforcement
	StaffID   uuid.UUID
	Method    string
	Notes     string
}

// CachedPickupResponse is the response body that gets cached for
// idempotent retries.
type CachedPickupResponse struct {
	Record db.RacepackPickupRecord `json:"record"`
}

// ExecutePickup is the atomic pickup method. Steps inside one tx:
//  1. Acquire SELECT ... FOR UPDATE on the ticket (closes TOCTOU window).
//  2. Re-check eligibility using fresh data.
//  3. Validate counter ownership and active status.
//  4. If slotID != nil: validate slot ownership, active, window, and
//     IncrementSlotReserved atomically.
//  5. INSERT pickup record. Unique partial index is the no-duplicate guard.
//
// Audit emits OUTSIDE the tx so audit failures do not roll back the pickup,
// but the operation is recorded in the structured log as well.
func (s *Service) ExecutePickup(ctx context.Context, in ExecutePickupInput) (db.RacepackPickupRecord, error) {
	if err := validatePickupMethod(in.Method); err != nil {
		return db.RacepackPickupRecord{}, err
	}
	var record db.RacepackPickupRecord
	err := s.repo.ExecTx(ctx, func(tx Repository) error {
		// Step 1: lock the ticket row to close the TOCTOU window.
		t, err := tx.LockTicketForUpdate(ctx, in.TicketID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrTicketNotFound
			}
			return err
		}

		// Step 2: re-check eligibility using the locked row.
		if t.Status == TicketStatusCancelled {
			return ErrTicketCancelled
		}
		if !t.BibNumber.Valid || t.BibNumber.String == "" {
			return ErrBibMissing
		}
		if t.EventID != in.EventID {
			return ErrTicketEventMismatch
		}
		hasActive, err := tx.HasActivePickup(ctx, in.TicketID)
		if err != nil {
			return err
		}
		if hasActive {
			return ErrAlreadyPickedUp
		}
		orderStatus, err := tx.GetOrderStatusForTicket(ctx, in.TicketID)
		if err != nil {
			return err
		}
		if orderStatus != OrderStatusPaid {
			return ErrOrderNotPaid
		}

		// Step 3: validate counter ownership and active status.
		if _, err := s.AssertCounterInEventTx(ctx, tx, in.CounterID, in.EventID); err != nil {
			return err
		}

		// Step 4: optional slot enforcement.
		if in.SlotID != nil {
			slot, err := tx.GetSlotByID(ctx, *in.SlotID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrSlotNotFound
				}
				return err
			}
			if slot.EventID != in.EventID {
				return ErrSlotNotFound
			}
			if !slot.Active {
				return ErrSlotInactive
			}
			now := time.Now().UTC()
			if now.Before(slot.StartTime.Time) || now.After(slot.EndTime.Time) {
				return ErrOutsideWindow
			}
			if _, err := tx.IncrementSlotReserved(ctx, slot.ID); err != nil {
				if errorsIsNoRows(err) {
					return ErrSlotFull
				}
				return err
			}
		}

		// Step 5: INSERT pickup record. Unique partial index is the final
		// no-duplicate guard.
		var slotArg *uuid.UUID
		if in.SlotID != nil {
			idCopy := *in.SlotID
			slotArg = &idCopy
		}
		var notesArg = pgtypeText("")
		if in.Notes != "" {
			notesArg = pgtypeText(in.Notes)
		}
		rec, err := tx.CreatePickupRecord(ctx, db.CreateRacepackPickupRecordParams{
			OrganizationID: in.OrgID,
			EventID:        in.EventID,
			TicketID:       in.TicketID,
			ParticipantID:  t.ParticipantID,
			BibNumber:      t.BibNumber.String,
			CounterID:      in.CounterID,
			SlotID:         slotArg,
			StaffID:        in.StaffID,
			PickupMethod:   in.Method,
			Notes:          notesArg,
		})
		if err != nil {
			if IsUniqueViolation(err) {
				return ErrAlreadyPickedUp
			}
			return err
		}
		record = rec
		return nil
	})
	if err != nil {
		return db.RacepackPickupRecord{}, err
	}

	s.recordAudit(ctx, in.OrgID, in.StaffID, "RACEPACK_PICKUP_COMPLETED", "pickup_record", record.ID.String(), map[string]any{
		"ticket_id":     in.TicketID.String(),
		"counter_id":    in.CounterID.String(),
		"pickup_method": in.Method,
		"at":            time.Now().UTC().Format(time.RFC3339),
	})
	if in.SlotID != nil {
		s.recordAudit(ctx, in.OrgID, in.StaffID, "RACEPACK_SLOT_RESERVED", "slot", in.SlotID.String(), map[string]any{
			"pickup_record_id": record.ID.String(),
		})
	}
	return record, nil
}

// AssertCounterInEventTx is the same as AssertCounterInEvent but takes a tx-bound
// repo so it can be called inside ExecutePickup's transaction.
func (s *Service) AssertCounterInEventTx(ctx context.Context, tx Repository, counterID, eventID uuid.UUID) (db.RacepackCounter, error) {
	c, err := tx.GetCounterByID(ctx, counterID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.RacepackCounter{}, ErrCounterNotFound
		}
		return db.RacepackCounter{}, err
	}
	if c.EventID != eventID {
		return db.RacepackCounter{}, ErrCounterEventMismatch
	}
	if !c.Active {
		return db.RacepackCounter{}, ErrCounterInactive
	}
	return c, nil
}

// GetPickupStatusByTicket returns the active pickup for a ticket, or
// ErrTicketNotFound.
func (s *Service) GetPickupStatusByTicket(ctx context.Context, ticketID uuid.UUID) (db.RacepackPickupRecord, error) {
	rec, err := s.repo.GetActivePickupByTicket(ctx, ticketID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.RacepackPickupRecord{}, ErrTicketNotFound
		}
		return db.RacepackPickupRecord{}, err
	}
	return rec, nil
}

// GetPickupRecordByID returns a single pickup record by primary key.
func (s *Service) GetPickupRecordByID(ctx context.Context, id uuid.UUID) (db.RacepackPickupRecord, error) {
	rec, err := s.repo.GetPickupRecordByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.RacepackPickupRecord{}, ErrTicketNotFound
		}
		return db.RacepackPickupRecord{}, err
	}
	return rec, nil
}

func (s *Service) ListPickupRecordsByEvent(ctx context.Context, eventID uuid.UUID, limit, offset int32) ([]db.RacepackPickupRecord, error) {
	return s.repo.ListPickupRecordsByEvent(ctx, db.ListRacepackPickupRecordsByEventParams{
		EventID: eventID,
		Limit:   limit,
		Offset:  offset,
	})
}

// --- proxy authorizations ---

func (s *Service) CreateProxyAuthorization(ctx context.Context, orgID, eventID, ticketID, createdBy uuid.UUID, pickupRecordID *uuid.UUID, proxyName, proxyPhone, proxyIdentity, doc string) (db.RacepackProxyAuthorization, error) {
	// Verify the ticket belongs to this event.
	if err := s.AssertTicketInEvent(ctx, ticketID, eventID); err != nil {
		return db.RacepackProxyAuthorization{}, err
	}
	rec, err := s.repo.CreateProxyAuthorization(ctx, db.CreateRacepackProxyAuthorizationParams{
		OrganizationID:        orgID,
		EventID:               eventID,
		TicketID:              ticketID,
		PickupRecordID:        pickupRecordID,
		ProxyName:             proxyName,
		ProxyPhone:            pgtypeText(proxyPhone),
		ProxyIdentity:         proxyIdentity,
		AuthorizationDocument: pgtypeText(doc),
		CreatedBy:             createdBy,
	})
	if err != nil {
		return db.RacepackProxyAuthorization{}, err
	}
	s.recordAudit(ctx, orgID, createdBy, "RACEPACK_PROXY_AUTHORIZED", "proxy_authorization", rec.ID.String(), map[string]any{
		"ticket_id":   ticketID.String(),
		"proxy_name":  proxyName,
		"at":          time.Now().UTC().Format(time.RFC3339),
	})
	return rec, nil
}

func (s *Service) ListProxyAuthorizationsByTicket(ctx context.Context, ticketID uuid.UUID) ([]db.RacepackProxyAuthorization, error) {
	return s.repo.ListProxyAuthorizationsByTicket(ctx, ticketID)
}

// --- problem cases ---

func (s *Service) CreateProblemCase(ctx context.Context, orgID, eventID, createdBy uuid.UUID, ticketID, participantID *uuid.UUID, reason string) (db.RacepackProblemCase, error) {
	// At least one of ticket_id or participant_id must be provided.
	if ticketID == nil && participantID == nil {
		return db.RacepackProblemCase{}, ErrNoProblemTarget
	}
	// If ticket_id is provided, verify it belongs to this event.
	if ticketID != nil {
		if err := s.AssertTicketInEvent(ctx, *ticketID, eventID); err != nil {
			return db.RacepackProblemCase{}, err
		}
	}
	rec, err := s.repo.CreateProblemCase(ctx, db.CreateRacepackProblemCaseParams{
		OrganizationID: orgID,
		EventID:        eventID,
		TicketID:       ticketID,
		ParticipantID:  participantID,
		Reason:         reason,
		CreatedBy:      createdBy,
	})
	if err != nil {
		return db.RacepackProblemCase{}, err
	}
	s.recordAudit(ctx, orgID, createdBy, "RACEPACK_PROBLEM_OPENED", "problem_case", rec.ID.String(), map[string]any{
		"reason": reason,
		"at":    time.Now().UTC().Format(time.RFC3339),
	})
	return rec, nil
}

func (s *Service) UpdateProblemCaseStatus(ctx context.Context, caseID, actorID uuid.UUID, to, resolution string) (db.RacepackProblemCase, error) {
	cur, err := s.repo.GetProblemCaseByID(ctx, caseID)
	if err != nil {
		return db.RacepackProblemCase{}, err
	}
	if err := ValidateStateTransition(cur.Status, to); err != nil {
		return db.RacepackProblemCase{}, err
	}
	rec, err := s.repo.UpdateProblemCaseStatus(ctx, db.UpdateRacepackProblemCaseStatusParams{
		ID:         caseID,
		Status:     to,
		Resolution: pgtypeText(resolution),
		ResolvedBy: &actorID,
	})
	if err != nil {
		return db.RacepackProblemCase{}, err
	}
	s.recordAudit(ctx, cur.OrganizationID, actorID, "RACEPACK_PROBLEM_STATUS", "problem_case", rec.ID.String(), map[string]any{
		"from":   cur.Status,
		"to":     to,
		"at":     time.Now().UTC().Format(time.RFC3339),
	})
	return rec, nil
}

func (s *Service) ListProblemCases(ctx context.Context, eventID uuid.UUID, limit, offset int32) ([]db.RacepackProblemCase, error) {
	return s.repo.ListProblemCasesByEvent(ctx, db.ListRacepackProblemCasesByEventParams{
		EventID: eventID,
		Limit:   limit,
		Offset:  offset,
	})
}

// --- dashboard ---

// DashboardSummary is the read model the organizer dashboard consumes.
// Returned shape matches the frontend Dashboard interface (byCounter, openCases).
func (s *Service) DashboardSummary(ctx context.Context, eventID uuid.UUID) (DashboardResponse, error) {
	total, err := s.repo.CountPickupRecordsByEvent(ctx, eventID)
	if err != nil {
		return DashboardResponse{}, err
	}
	counters, err := s.repo.ListCountersByEvent(ctx, eventID)
	if err != nil {
		return DashboardResponse{}, err
	}
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	rows, err := s.repo.CountPickupRecordsByCounter(ctx, db.CountRacepackPickupRecordsByCounterParams{
		EventID:           eventID,
		PickupTimestamp:   pgtypeTimestamptz(dayStart),
		PickupTimestamp_2: pgtypeTimestamptz(dayEnd),
	})
	if err != nil {
		return DashboardResponse{}, err
	}
	byCounter := make(map[uuid.UUID]int64, len(rows))
	for _, r := range rows {
		byCounter[r.CounterID] = r.PickupCount
	}
	per := make([]DashboardCounterCount, 0, len(counters))
	active := 0
	for _, c := range counters {
		if c.Active {
			active++
		}
		per = append(per, DashboardCounterCount{
			CounterID:   c.ID,
			CounterName: c.Name,
			Pickups:     byCounter[c.ID],
			Active:      c.Active,
		})
	}
	openCases, err := s.repo.CountProblemCasesByEventAndStatus(ctx, eventID, ProblemCaseStatusOpen)
	if err != nil {
		return DashboardResponse{}, err
	}
	return DashboardResponse{
		TotalPickups:   total,
		ByCounter:      per,
		OpenCases:      openCases,
		TotalCounters:  len(counters),
		ActiveCounters: active,
	}, nil
}

// recordAudit is a tiny helper that no-ops when audit is nil.
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

// validatePickupMethod enforces the strict allowlist.
func validatePickupMethod(method string) error {
	m := strings.ToUpper(strings.TrimSpace(method))
	switch m {
	case PickupMethodSelf, PickupMethodProxy, PickupMethodManualOverride:
		return nil
	default:
		return fmt.Errorf("%w: got %q", ErrInvalidMethod, method)
	}
}

// Suppress unused warnings for some pgtype helpers that may not be used here
// but are useful to keep exported.
var _ = pgtype.Text{}

// Suppress unused warnings for fmt import (used in validatePickupMethod).
var _ = fmt.Sprintf
var _ = errors.Is
var _ = pgx.ErrNoRows