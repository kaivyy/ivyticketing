package events

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	"github.com/varin/ivyticketing/services/api/internal/platform/storage"
)

type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

type Service struct {
	repo  Repository
	store storage.Storage
	audit AuditRecorder
}

func NewService(repo Repository, store storage.Storage, recorder AuditRecorder) *Service {
	return &Service{repo: repo, store: store, audit: recorder}
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, req CreateRequest) (Response, error) {
	if !validEventTypes[req.EventType] {
		return Response{}, ErrInvalidEventType
	}
	slug := slugify(req.Name)
	if _, err := s.repo.GetEventByOrgAndSlug(ctx, db.GetEventByOrgAndSlugParams{OrganizationID: orgID, Slug: slug}); err == nil {
		return Response{}, ErrSlugTaken
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Response{}, err
	}

	e, err := s.repo.CreateEvent(ctx, db.CreateEventParams{
		OrganizationID: orgID,
		Name:           req.Name,
		Slug:           slug,
		Description:    nullText(req.Description),
		EventType:      req.EventType,
		VenueName:      nullText(req.VenueName),
		VenueAddress:   nullText(req.VenueAddress),
		StartsAt:       nullTime(req.StartsAt),
		EndsAt:         nullTime(req.EndsAt),
		Faq:            nullText(req.FAQ),
		Terms:          nullText(req.Terms),
		Waiver:         nullText(req.Waiver),
	})
	if err != nil {
		return Response{}, err
	}
	return s.toResponse(e), nil
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]Response, error) {
	rows, err := s.repo.ListEventsByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]Response, 0, len(rows))
	for _, e := range rows {
		out = append(out, s.toResponse(e))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, orgID, eventID uuid.UUID) (Response, error) {
	e, err := s.loadOrgEvent(ctx, orgID, eventID)
	if err != nil {
		return Response{}, err
	}
	return s.toResponse(e), nil
}

func (s *Service) Update(ctx context.Context, orgID, eventID uuid.UUID, req UpdateRequest) (Response, error) {
	if !validEventTypes[req.EventType] {
		return Response{}, ErrInvalidEventType
	}
	if _, err := s.loadOrgEvent(ctx, orgID, eventID); err != nil {
		return Response{}, err
	}
	e, err := s.repo.UpdateEvent(ctx, db.UpdateEventParams{
		ID:             eventID,
		Name:           req.Name,
		Description:    nullText(req.Description),
		EventType:      req.EventType,
		VenueName:      nullText(req.VenueName),
		VenueAddress:   nullText(req.VenueAddress),
		StartsAt:       nullTime(req.StartsAt),
		EndsAt:         nullTime(req.EndsAt),
		Faq:            nullText(req.FAQ),
		Terms:          nullText(req.Terms),
		Waiver:         nullText(req.Waiver),
		OrganizationID: orgID,
	})
	if err != nil {
		return Response{}, err
	}
	return s.toResponse(e), nil
}

func (s *Service) Publish(ctx context.Context, orgID, eventID uuid.UUID) (Response, error) {
	e, err := s.loadOrgEvent(ctx, orgID, eventID)
	if err != nil {
		return Response{}, err
	}
	if e.Status != StatusDraft {
		return Response{}, ErrInvalidTransition
	}
	n, err := s.repo.CountCategoriesForEvent(ctx, eventID)
	if err != nil {
		return Response{}, err
	}
	if n == 0 {
		return Response{}, ErrNoCategories
	}
	updated, err := s.repo.UpdateEventStatus(ctx, db.UpdateEventStatusParams{
		ID:             eventID,
		Status:         StatusPublished,
		PublishedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		OrganizationID: orgID,
	})
	if err != nil {
		return Response{}, err
	}
	s.record(ctx, orgID, "event.publish", eventID)
	return s.toResponse(updated), nil
}

func (s *Service) Unpublish(ctx context.Context, orgID, eventID uuid.UUID) (Response, error) {
	e, err := s.loadOrgEvent(ctx, orgID, eventID)
	if err != nil {
		return Response{}, err
	}
	if e.Status != StatusPublished {
		return Response{}, ErrInvalidTransition
	}
	updated, err := s.repo.UpdateEventStatus(ctx, db.UpdateEventStatusParams{
		ID:             eventID,
		Status:         StatusDraft,
		PublishedAt:    pgtype.Timestamptz{Valid: false},
		OrganizationID: orgID,
	})
	if err != nil {
		return Response{}, err
	}
	s.record(ctx, orgID, "event.unpublish", eventID)
	return s.toResponse(updated), nil
}

func (s *Service) Archive(ctx context.Context, orgID, eventID uuid.UUID) (Response, error) {
	e, err := s.loadOrgEvent(ctx, orgID, eventID)
	if err != nil {
		return Response{}, err
	}
	if e.Status == StatusArchived {
		return Response{}, ErrInvalidTransition
	}
	updated, err := s.repo.UpdateEventStatus(ctx, db.UpdateEventStatusParams{
		ID:             eventID,
		Status:         StatusArchived,
		PublishedAt:    e.PublishedAt,
		OrganizationID: orgID,
	})
	if err != nil {
		return Response{}, err
	}
	s.record(ctx, orgID, "event.archive", eventID)
	return s.toResponse(updated), nil
}

func (s *Service) SetMedia(ctx context.Context, orgID, eventID uuid.UUID, kind, objectKey string) (Response, error) {
	if _, err := s.loadOrgEvent(ctx, orgID, eventID); err != nil {
		return Response{}, err
	}
	arg := db.SetEventMediaKeyParams{ID: eventID, OrganizationID: orgID}
	if kind == "banner" {
		arg.BannerObjectKey = pgtype.Text{String: objectKey, Valid: true}
	} else {
		arg.LogoObjectKey = pgtype.Text{String: objectKey, Valid: true}
	}
	e, err := s.repo.SetEventMediaKey(ctx, arg)
	if err != nil {
		return Response{}, err
	}
	return s.toResponse(e), nil
}

func (s *Service) Delete(ctx context.Context, orgID, eventID uuid.UUID) error {
	e, err := s.loadOrgEvent(ctx, orgID, eventID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteEvent(ctx, db.DeleteEventParams{ID: eventID, OrganizationID: orgID}); err != nil {
		return err
	}
	// best-effort media cleanup
	if s.store != nil {
		if e.BannerObjectKey.Valid {
			_ = s.store.Delete(ctx, e.BannerObjectKey.String)
		}
		if e.LogoObjectKey.Valid {
			_ = s.store.Delete(ctx, e.LogoObjectKey.String)
		}
	}
	s.record(ctx, orgID, "event.delete", eventID)
	return nil
}

// loadOrgEvent fetches an event and confirms tenant ownership (mismatch → ErrNotFound).
func (s *Service) loadOrgEvent(ctx context.Context, orgID, eventID uuid.UUID) (db.Event, error) {
	e, err := s.repo.GetEventByID(ctx, eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Event{}, ErrNotFound
	} else if err != nil {
		return db.Event{}, err
	}
	if e.OrganizationID != orgID {
		return db.Event{}, ErrNotFound
	}
	return e, nil
}

func (s *Service) record(ctx context.Context, orgID uuid.UUID, action string, eventID uuid.UUID) {
	if s.audit == nil {
		return
	}
	oid := orgID
	s.audit.Record(ctx, audit.Entry{
		OrganizationID: &oid,
		Action:         action,
		TargetType:     "event",
		TargetID:       eventID.String(),
	})
}

func (s *Service) toResponse(e db.Event) Response {
	r := Response{
		ID: e.ID, Name: e.Name, Slug: e.Slug, EventType: e.EventType, Status: e.Status,
		Description:  e.Description.String,
		VenueName:    e.VenueName.String,
		VenueAddress: e.VenueAddress.String,
		FAQ:          e.Faq.String,
		Terms:        e.Terms.String,
		Waiver:       e.Waiver.String,
		StartsAt:     timePtr(e.StartsAt),
		EndsAt:       timePtr(e.EndsAt),
		PublishedAt:  timePtr(e.PublishedAt),
		CreatedAt:    e.CreatedAt.Time,
	}
	if s.store != nil {
		if e.BannerObjectKey.Valid {
			r.BannerURL = s.store.PublicURL(e.BannerObjectKey.String)
		}
		if e.LogoObjectKey.Valid {
			r.LogoURL = s.store.PublicURL(e.LogoObjectKey.String)
		}
	}
	return r
}

func nullText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func nullTime(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
