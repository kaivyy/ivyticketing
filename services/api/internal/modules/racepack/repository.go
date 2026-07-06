package racepack

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository is the data access surface for the racepack module. It also
// satisfies the Lookup interface used by eligibility.go.
type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error

	// Counters.
	CreateCounter(ctx context.Context, arg db.CreateRacepackCounterParams) (db.RacepackCounter, error)
	ListCountersByEvent(ctx context.Context, eventID uuid.UUID) ([]db.RacepackCounter, error)
	GetCounterByID(ctx context.Context, id uuid.UUID) (db.RacepackCounter, error)
	UpdateCounter(ctx context.Context, arg db.UpdateRacepackCounterParams) (db.RacepackCounter, error)
	SetCounterActive(ctx context.Context, id uuid.UUID, active bool) (db.RacepackCounter, error)

	// Slots.
	CreateSlot(ctx context.Context, arg db.CreateRacepackPickupSlotParams) (db.RacepackPickupSlot, error)
	ListSlotsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.RacepackPickupSlot, error)
	ListActiveSlotsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.RacepackPickupSlot, error)
	GetSlotByID(ctx context.Context, id uuid.UUID) (db.RacepackPickupSlot, error)
	UpdateSlot(ctx context.Context, arg db.UpdateRacepackPickupSlotParams) (db.RacepackPickupSlot, error)
	IncrementSlotReserved(ctx context.Context, id uuid.UUID) (db.RacepackPickupSlot, error)
	DecrementSlotReserved(ctx context.Context, id uuid.UUID) error

	// Pickup records.
	CreatePickupRecord(ctx context.Context, arg db.CreateRacepackPickupRecordParams) (db.RacepackPickupRecord, error)
	GetActivePickupByTicket(ctx context.Context, ticketID uuid.UUID) (db.RacepackPickupRecord, error)
	GetPickupRecordByID(ctx context.Context, id uuid.UUID) (db.RacepackPickupRecord, error)
	ListPickupRecordsByEvent(ctx context.Context, arg db.ListRacepackPickupRecordsByEventParams) ([]db.RacepackPickupRecord, error)
	CountPickupRecordsByCounter(ctx context.Context, arg db.CountRacepackPickupRecordsByCounterParams) ([]db.CountRacepackPickupRecordsByCounterRow, error)
	CountPickupRecordsByEvent(ctx context.Context, eventID uuid.UUID) (int64, error)

	// Proxy authorizations.
	CreateProxyAuthorization(ctx context.Context, arg db.CreateRacepackProxyAuthorizationParams) (db.RacepackProxyAuthorization, error)
	ListProxyAuthorizationsByTicket(ctx context.Context, ticketID uuid.UUID) ([]db.RacepackProxyAuthorization, error)
	GetProxyAuthorizationByID(ctx context.Context, id uuid.UUID) (db.RacepackProxyAuthorization, error)

	// Problem cases.
	CreateProblemCase(ctx context.Context, arg db.CreateRacepackProblemCaseParams) (db.RacepackProblemCase, error)
	UpdateProblemCaseStatus(ctx context.Context, arg db.UpdateRacepackProblemCaseStatusParams) (db.RacepackProblemCase, error)
	ListProblemCasesByEvent(ctx context.Context, arg db.ListRacepackProblemCasesByEventParams) ([]db.RacepackProblemCase, error)
	CountProblemCasesByEventAndStatus(ctx context.Context, eventID uuid.UUID, status string) (int64, error)
	GetProblemCaseByID(ctx context.Context, id uuid.UUID) (db.RacepackProblemCase, error)

	// Lookup methods (eligibility + ownership).
	GetTicketStatus(ctx context.Context, ticketID uuid.UUID) (status string, eventID uuid.UUID, participantID uuid.UUID, bibNumber string, found bool, err error)
	GetOrderStatusForTicket(ctx context.Context, ticketID uuid.UUID) (status string, err error)
	HasActivePickup(ctx context.Context, ticketID uuid.UUID) (bool, error)

	// LockTicketForUpdate issues SELECT ... FOR UPDATE on the ticket row.
	// Must be called inside ExecTx; otherwise the lock is meaningless.
	LockTicketForUpdate(ctx context.Context, ticketID uuid.UUID) (db.Ticket, error)

	// GetEventOrganizationID returns the org that owns an event. Used to verify
	// that an event in the URL actually belongs to the org in the URL.
	GetEventOrganizationID(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error)

	// CheckOrganizationMembership reports whether a user is a member of an org.
	CheckOrganizationMembership(ctx context.Context, orgID, userID uuid.UUID) (bool, error)

	// GetUserTicket returns the ticket row with order status joined in one
	// query — used by the eligibility path inside transactions.
	GetUserTicket(ctx context.Context, ticketID uuid.UUID) (db.GetUserTicketByIDRow, error)

	// Idempotency.
	GetIdempotencyKey(ctx context.Context, arg db.GetIdempotencyKeyParams) (db.GetIdempotencyKeyRow, error)
	InsertIdempotencyKey(ctx context.Context, arg db.InsertIdempotencyKeyParams) (db.IdempotencyKey, error)
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// NewRepository constructs a Repository backed by the sqlc-generated Queries.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{pool: pool, q: db.New(pool)}
}

func (r *sqlcRepo) ExecTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(&sqlcRepo{pool: r.pool, q: db.New(tx)}); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// --- counters ---

func (r *sqlcRepo) CreateCounter(ctx context.Context, arg db.CreateRacepackCounterParams) (db.RacepackCounter, error) {
	return r.q.CreateRacepackCounter(ctx, arg)
}
func (r *sqlcRepo) ListCountersByEvent(ctx context.Context, eventID uuid.UUID) ([]db.RacepackCounter, error) {
	return r.q.ListRacepackCountersByEvent(ctx, eventID)
}
func (r *sqlcRepo) GetCounterByID(ctx context.Context, id uuid.UUID) (db.RacepackCounter, error) {
	return r.q.GetRacepackCounterByID(ctx, id)
}
func (r *sqlcRepo) UpdateCounter(ctx context.Context, arg db.UpdateRacepackCounterParams) (db.RacepackCounter, error) {
	return r.q.UpdateRacepackCounter(ctx, arg)
}
func (r *sqlcRepo) SetCounterActive(ctx context.Context, id uuid.UUID, active bool) (db.RacepackCounter, error) {
	return r.q.SetRacepackCounterActive(ctx, db.SetRacepackCounterActiveParams{
		ID:     id,
		Active: active,
	})
}

// --- slots ---

func (r *sqlcRepo) CreateSlot(ctx context.Context, arg db.CreateRacepackPickupSlotParams) (db.RacepackPickupSlot, error) {
	return r.q.CreateRacepackPickupSlot(ctx, arg)
}
func (r *sqlcRepo) ListSlotsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.RacepackPickupSlot, error) {
	return r.q.ListRacepackPickupSlotsByEvent(ctx, eventID)
}
func (r *sqlcRepo) ListActiveSlotsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.RacepackPickupSlot, error) {
	return r.q.ListRacepackPickupSlotsActiveByEvent(ctx, eventID)
}
func (r *sqlcRepo) GetSlotByID(ctx context.Context, id uuid.UUID) (db.RacepackPickupSlot, error) {
	return r.q.GetRacepackPickupSlotByID(ctx, id)
}
func (r *sqlcRepo) UpdateSlot(ctx context.Context, arg db.UpdateRacepackPickupSlotParams) (db.RacepackPickupSlot, error) {
	return r.q.UpdateRacepackPickupSlot(ctx, arg)
}
func (r *sqlcRepo) IncrementSlotReserved(ctx context.Context, id uuid.UUID) (db.RacepackPickupSlot, error) {
	return r.q.IncrementRacepackPickupSlotReserved(ctx, id)
}
func (r *sqlcRepo) DecrementSlotReserved(ctx context.Context, id uuid.UUID) error {
	return r.q.DecrementRacepackPickupSlotReserved(ctx, id)
}

// --- pickup records ---

func (r *sqlcRepo) CreatePickupRecord(ctx context.Context, arg db.CreateRacepackPickupRecordParams) (db.RacepackPickupRecord, error) {
	return r.q.CreateRacepackPickupRecord(ctx, arg)
}
func (r *sqlcRepo) GetActivePickupByTicket(ctx context.Context, ticketID uuid.UUID) (db.RacepackPickupRecord, error) {
	return r.q.GetRacepackPickupRecordByTicket(ctx, ticketID)
}
func (r *sqlcRepo) GetPickupRecordByID(ctx context.Context, id uuid.UUID) (db.RacepackPickupRecord, error) {
	return r.q.GetRacepackPickupRecordByID(ctx, id)
}
func (r *sqlcRepo) ListPickupRecordsByEvent(ctx context.Context, arg db.ListRacepackPickupRecordsByEventParams) ([]db.RacepackPickupRecord, error) {
	return r.q.ListRacepackPickupRecordsByEvent(ctx, arg)
}
func (r *sqlcRepo) CountPickupRecordsByCounter(ctx context.Context, arg db.CountRacepackPickupRecordsByCounterParams) ([]db.CountRacepackPickupRecordsByCounterRow, error) {
	return r.q.CountRacepackPickupRecordsByCounter(ctx, arg)
}
func (r *sqlcRepo) CountPickupRecordsByEvent(ctx context.Context, eventID uuid.UUID) (int64, error) {
	return r.q.CountRacepackPickupRecordsByEvent(ctx, eventID)
}

// --- proxy authorizations ---

func (r *sqlcRepo) CreateProxyAuthorization(ctx context.Context, arg db.CreateRacepackProxyAuthorizationParams) (db.RacepackProxyAuthorization, error) {
	return r.q.CreateRacepackProxyAuthorization(ctx, arg)
}
func (r *sqlcRepo) ListProxyAuthorizationsByTicket(ctx context.Context, ticketID uuid.UUID) ([]db.RacepackProxyAuthorization, error) {
	return r.q.ListRacepackProxyAuthorizationsByTicket(ctx, ticketID)
}
func (r *sqlcRepo) GetProxyAuthorizationByID(ctx context.Context, id uuid.UUID) (db.RacepackProxyAuthorization, error) {
	return r.q.GetRacepackProxyAuthorizationByID(ctx, id)
}

// --- problem cases ---

func (r *sqlcRepo) CreateProblemCase(ctx context.Context, arg db.CreateRacepackProblemCaseParams) (db.RacepackProblemCase, error) {
	return r.q.CreateRacepackProblemCase(ctx, arg)
}
func (r *sqlcRepo) UpdateProblemCaseStatus(ctx context.Context, arg db.UpdateRacepackProblemCaseStatusParams) (db.RacepackProblemCase, error) {
	return r.q.UpdateRacepackProblemCaseStatus(ctx, arg)
}
func (r *sqlcRepo) ListProblemCasesByEvent(ctx context.Context, arg db.ListRacepackProblemCasesByEventParams) ([]db.RacepackProblemCase, error) {
	return r.q.ListRacepackProblemCasesByEvent(ctx, arg)
}
func (r *sqlcRepo) CountProblemCasesByEventAndStatus(ctx context.Context, eventID uuid.UUID, status string) (int64, error) {
	return r.q.CountRacepackProblemCasesByEventAndStatus(ctx, db.CountRacepackProblemCasesByEventAndStatusParams{
		EventID: eventID,
		Status:  status,
	})
}
func (r *sqlcRepo) GetProblemCaseByID(ctx context.Context, id uuid.UUID) (db.RacepackProblemCase, error) {
	return r.q.GetRacepackProblemCaseByID(ctx, id)
}

// --- lookup methods (eligibility + ownership) ---

func (r *sqlcRepo) GetTicketStatus(ctx context.Context, ticketID uuid.UUID) (string, uuid.UUID, uuid.UUID, string, bool, error) {
	t, err := r.q.GetTicketByID(ctx, ticketID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", uuid.Nil, uuid.Nil, "", false, nil
		}
		return "", uuid.Nil, uuid.Nil, "", false, err
	}
	bib := ""
	if t.BibNumber.Valid {
		bib = t.BibNumber.String
	}
	return t.Status, t.EventID, t.ParticipantID, bib, true, nil
}

func (r *sqlcRepo) GetOrderStatusForTicket(ctx context.Context, ticketID uuid.UUID) (string, error) {
	t, err := r.q.GetTicketByID(ctx, ticketID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrTicketNotFound
		}
		return "", err
	}
	o, err := r.q.GetOrderByID(ctx, t.OrderID)
	if err != nil {
		return "", err
	}
	return o.Status, nil
}

func (r *sqlcRepo) HasActivePickup(ctx context.Context, ticketID uuid.UUID) (bool, error) {
	_, err := r.q.GetRacepackPickupRecordByTicket(ctx, ticketID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *sqlcRepo) LockTicketForUpdate(ctx context.Context, ticketID uuid.UUID) (db.Ticket, error) {
	return r.q.LockTicketForUpdate(ctx, ticketID)
}

func (r *sqlcRepo) GetEventOrganizationID(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error) {
	return r.q.GetEventOrganizationID(ctx, eventID)
}

func (r *sqlcRepo) CheckOrganizationMembership(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	row, err := r.q.CheckOrganizationMembership(ctx, db.CheckOrganizationMembershipParams{
		OrganizationID: orgID,
		UserID:        userID,
	})
	if err != nil {
		return false, err
	}
	return row, nil
}

func (r *sqlcRepo) GetUserTicket(ctx context.Context, ticketID uuid.UUID) (db.GetUserTicketByIDRow, error) {
	return r.q.GetUserTicketByID(ctx, ticketID)
}

// --- idempotency ---

func (r *sqlcRepo) GetIdempotencyKey(ctx context.Context, arg db.GetIdempotencyKeyParams) (db.GetIdempotencyKeyRow, error) {
	return r.q.GetIdempotencyKey(ctx, arg)
}
func (r *sqlcRepo) InsertIdempotencyKey(ctx context.Context, arg db.InsertIdempotencyKeyParams) (db.IdempotencyKey, error) {
	return r.q.InsertIdempotencyKey(ctx, arg)
}

// IsUniqueViolation returns true if err is a Postgres unique-constraint
// violation (SQLSTATE 23505) — used to translate pickup-record races into
// ErrAlreadyPickedUp.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}