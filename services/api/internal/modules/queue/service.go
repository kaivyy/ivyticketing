package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	notifmod "github.com/varin/ivyticketing/services/api/internal/modules/notifications"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

// EventReader is a thin dependency used by JoinByEvent to look up an event's
// organization_id without importing the full events module.
type EventReader interface {
	GetEventOrgID(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error)
}

// EventModeResolver resolves the registration mode for an event.
// Implemented by registration.Service.
type EventModeResolver interface {
	ResolveEventMode(ctx context.Context, eventID uuid.UUID) (string, error)
}

// Notifier is a local interface satisfied by *notifmod.Service.
type Notifier interface {
	Enqueue(ctx context.Context, participantID uuid.UUID, typ string, data notifmod.TemplateData) error
}

type Service struct {
	repo        Repository
	store       *Store
	audit       AuditRecorder
	notifier    Notifier
	events      EventReader
	resolver    EventModeResolver
	defaultRate int32
}

func NewService(repo Repository, store *Store, recorder AuditRecorder, events EventReader, defaultRate int32, resolver EventModeResolver) *Service {
	return &Service{repo: repo, store: store, audit: recorder, events: events, resolver: resolver, defaultRate: defaultRate}
}

// WithNotifier attaches a Notifier to the service. Called from server.go after construction.
func (s *Service) WithNotifier(n Notifier) { s.notifier = n }

// Join issues (or returns existing) a queue token for the participant. Idempotent:
// refresh/reconnect/mobile-sleep safe — same token returned on repeated calls.
func (s *Service) Join(ctx context.Context, orgID, eventID, participantID uuid.UUID) (JoinResponse, error) {
	pool := PoolFifo
	score := FifoScore(time.Now())

	// Determine mode for pool/score selection.
	mode := "WAR_QUEUE" // default
	if s.resolver != nil {
		m, err := s.resolver.ResolveEventMode(ctx, eventID)
		if err != nil {
			return JoinResponse{}, err
		}
		mode = m
	}

	switch mode {
	case "RANDOMIZED_QUEUE", "HYBRID_QUEUE":
		ctrl, err := s.repo.GetControl(ctx, eventID)
		if err == nil && ctrl.SaleStartAt.Valid && time.Now().Before(ctrl.SaleStartAt.Time) {
			// presale window: use seeded random pool
			seed := ""
			if ctrl.RandomizationSeed.Valid {
				seed = ctrl.RandomizationSeed.String
			}
			pool = PoolPresale
			score = PresaleScore(seed, participantID)
		}
		// after sale start: FIFO (defaults above)
	case "WAR_QUEUE":
		// FIFO always (defaults above)
	default:
		// NORMAL, CLOSED, etc. — not a queue mode
		return JoinResponse{}, ErrNotEnabled
	}

	tok, err := s.repo.CreateToken(ctx, db.CreateQueueTokenParams{
		OrganizationID: orgID,
		EventID:        eventID,
		ParticipantID:  participantID,
		Pool:           pool,
		Score:          score,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// ON CONFLICT DO NOTHING returns no rows — participant already queued; return existing token
		tok, err = s.repo.GetTokenByEventParticipant(ctx, eventID, participantID)
		if err != nil {
			return JoinResponse{}, err
		}
	} else if err != nil {
		return JoinResponse{}, err
	} else {
		// newly issued
		_ = s.store.AddWaiting(ctx, eventID.String(), participantID.String(), tok.Score)
		if s.audit != nil {
			oid := orgID
			aid := participantID
			s.audit.Record(ctx, audit.Entry{
				OrganizationID: &oid,
				ActorUserID:    &aid,
				Action:         "QUEUE_TOKEN_ISSUED",
				TargetType:     "queue_token",
				TargetID:       tok.ID.String(),
				Metadata:       map[string]any{"eventId": eventID.String()},
			})
		}
	}
	pos, _ := s.store.Rank(ctx, eventID.String(), participantID.String())
	return JoinResponse{TokenID: tok.ID.String(), Status: tok.Status, Position: pos}, nil
}

// Status returns the participant's queue position and state.
func (s *Service) Status(ctx context.Context, eventID, participantID uuid.UUID) (StatusResponse, error) {
	tok, err := s.repo.GetTokenByEventParticipant(ctx, eventID, participantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return StatusResponse{}, ErrTokenNotFound
	}
	if err != nil {
		return StatusResponse{}, err
	}

	ctrl, err := s.repo.GetControl(ctx, eventID)
	state := StateRunning
	rate := s.defaultRate
	if err == nil {
		state = ctrl.State
		rate = ctrl.ReleaseRate
	}

	resp := StatusResponse{TokenID: tok.ID.String(), Status: tok.Status, SystemState: state}
	if tok.Status == StatusWaiting {
		pos, _ := s.store.Rank(ctx, eventID.String(), participantID.String())
		resp.Position = pos
		if rate > 0 {
			resp.EstimatedWaitSeconds = pos / int64(rate)
		}
	}
	if tok.Status == StatusAllowed {
		adm, err := s.repo.GetActiveAdmission(ctx, db.GetActiveAdmissionByParticipantParams{
			EventID:       eventID,
			ParticipantID: participantID,
		})
		if err == nil {
			resp.AdmissionToken = adm.ID.String()
			resp.CheckoutExpiresAt = adm.CheckoutExpiresAt.Time.Format(time.RFC3339)
		}
	}
	return resp, nil
}

// JoinByEvent is a convenience wrapper for the HTTP handler. It resolves the
// event's organization_id via EventReader, then delegates to Join.
func (s *Service) JoinByEvent(ctx context.Context, eventID, participantID uuid.UUID) (JoinResponse, error) {
	orgID, err := s.events.GetEventOrgID(ctx, eventID)
	if err != nil {
		return JoinResponse{}, err
	}
	return s.Join(ctx, orgID, eventID, participantID)
}
