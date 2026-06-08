package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
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

type Service struct {
	repo        Repository
	store       *Store
	audit       AuditRecorder
	events      EventReader
	defaultRate int32
}

func NewService(repo Repository, store *Store, recorder AuditRecorder, events EventReader, defaultRate int32) *Service {
	return &Service{repo: repo, store: store, audit: recorder, events: events, defaultRate: defaultRate}
}

// Join issues (or returns existing) a queue token for the participant. Idempotent:
// refresh/reconnect/mobile-sleep safe — same token returned on repeated calls.
func (s *Service) Join(ctx context.Context, orgID, eventID, participantID uuid.UUID) (JoinResponse, error) {
	score := FifoScore(time.Now())
	tok, err := s.repo.CreateToken(ctx, db.CreateQueueTokenParams{
		OrganizationID: orgID,
		EventID:        eventID,
		ParticipantID:  participantID,
		Pool:           PoolFifo,
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
