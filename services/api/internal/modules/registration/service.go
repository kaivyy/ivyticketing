package registration

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// ResolveForCheckout loads settings for (event, category) and returns the resolved mode.
// Missing rows → NORMAL (regression-safe: no settings = NORMAL = Phase 5 behavior).
func (s *Service) ResolveForCheckout(ctx context.Context, eventID, categoryID uuid.UUID) (Mode, error) {
	in := ModeInput{}
	ev, err := s.repo.GetEventSettings(ctx, eventID)
	if err == nil {
		in.EventModeSet = true
		in.EventMode = Mode(ev.DefaultMode)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ModeNormal, err
	}
	cat, err := s.repo.GetCategorySettings(ctx, categoryID)
	if err == nil {
		in.CategoryOverride = cat.OverrideEnabled
		if cat.RegistrationMode.Valid {
			in.CategoryModeSet = true
			in.CategoryMode = Mode(cat.RegistrationMode.String)
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ModeNormal, err
	}
	return ResolveMode(in), nil
}

// ResolveEventMode returns the event's default mode (ignoring category override).
// Used by the queue module to check if an event has a queue mode active.
func (s *Service) ResolveEventMode(ctx context.Context, eventID uuid.UUID) (string, error) {
	ev, err := s.repo.GetEventSettings(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return string(ModeNormal), nil
		}
		return "", err
	}
	return ev.DefaultMode, nil
}

// assertEvent confirms the event exists and belongs to orgID (tenant guard).
func (s *Service) assertEvent(ctx context.Context, orgID, eventID uuid.UUID) error {
	e, err := s.repo.GetEventByID(ctx, eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEventNotFound
	} else if err != nil {
		return err
	}
	if orgID != uuid.Nil && e.OrganizationID != orgID {
		return ErrEventNotFound
	}
	return nil
}

func (s *Service) SetEventSettings(ctx context.Context, orgID, eventID uuid.UUID, req EventSettingsRequest) error {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return err
	}
	if !Valid(Mode(req.DefaultMode)) {
		return ErrInvalidMode
	}
	_, err := s.repo.UpsertEventSettings(ctx, db.UpsertEventRegistrationSettingsParams{
		EventID:         eventID,
		DefaultMode:     req.DefaultMode,
		QueueEnabled:    req.QueueEnabled,
		BallotEnabled:   req.BallotEnabled,
		PriorityEnabled: req.PriorityEnabled,
		WaitlistEnabled: req.WaitlistEnabled,
	})
	return err
}

func (s *Service) SetCategorySettings(ctx context.Context, orgID, eventID, categoryID uuid.UUID, req CategorySettingsRequest) error {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return err
	}
	cat, err := s.repo.GetCategoryByID(ctx, categoryID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && cat.EventID != eventID) {
		return ErrCategoryNotFound
	}
	if err != nil {
		return err
	}

	var mode pgtype.Text
	if req.RegistrationMode != nil {
		if !Valid(Mode(*req.RegistrationMode)) {
			return ErrInvalidMode
		}
		mode = pgtype.Text{String: *req.RegistrationMode, Valid: true}
	}
	_, err = s.repo.UpsertCategorySettings(ctx, db.UpsertCategoryRegistrationSettingsParams{
		CategoryID:       categoryID,
		RegistrationMode: mode,
		OverrideEnabled:  req.OverrideEnabled,
	})
	return err
}

func (s *Service) GetEventSettings(ctx context.Context, orgID, eventID uuid.UUID) (SettingsResponse, error) {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return SettingsResponse{}, err
	}
	evSettings, evErr := s.repo.GetEventSettings(ctx, eventID)
	resp := SettingsResponse{EventID: eventID.String()}
	if evErr == nil {
		resp.DefaultMode = evSettings.DefaultMode
		resp.QueueEnabled = evSettings.QueueEnabled
		resp.BallotEnabled = evSettings.BallotEnabled
		resp.PriorityEnabled = evSettings.PriorityEnabled
		resp.WaitlistEnabled = evSettings.WaitlistEnabled
	} else {
		resp.DefaultMode = string(ModeNormal)
	}

	catSettings, err := s.repo.ListCategorySettingsByEvent(ctx, eventID)
	if err != nil {
		return SettingsResponse{}, err
	}
	for _, cs := range catSettings {
		c := CategorySettingsResponse{
			CategoryID:      cs.CategoryID.String(),
			OverrideEnabled: cs.OverrideEnabled,
		}
		if cs.RegistrationMode.Valid {
			m := cs.RegistrationMode.String
			c.RegistrationMode = &m
		}
		resp.Categories = append(resp.Categories, c)
	}
	return resp, nil
}
