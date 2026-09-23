package publiccatalog

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var ErrNotFound = apperr.New(http.StatusNotFound, "EVENT_NOT_FOUND", "event not found")

// urlBuilder is the subset of storage.Storage this module needs.
type urlBuilder interface {
	PublicURL(key string) string
}

type Service struct {
	repo  Repository
	store urlBuilder
}

func NewService(repo Repository, store urlBuilder) *Service {
	return &Service{repo: repo, store: store}
}

func (s *Service) ListAllEvents(ctx context.Context) ([]EventResponse, error) {
	rows, err := s.repo.ListAllPublishedEvents(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]EventResponse, 0, len(rows))
	for _, row := range rows {
		cats, _ := s.repo.ListCategoriesByEventForPublicWithMode(ctx, row.ID)
		e := db.Event{
			ID:              row.ID,
			OrganizationID:  row.OrganizationID,
			Name:            row.Name,
			Slug:            row.Slug,
			Description:     row.Description,
			EventType:       row.EventType,
			Status:          row.Status,
			BannerObjectKey: row.BannerObjectKey,
			LogoObjectKey:   row.LogoObjectKey,
			VenueName:       row.VenueName,
			VenueAddress:    row.VenueAddress,
			StartsAt:        row.StartsAt,
			EndsAt:          row.EndsAt,
			Faq:             row.Faq,
			Terms:           row.Terms,
			Waiver:          row.Waiver,
			PublishedAt:     row.PublishedAt,
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
		}
		resp := s.toEventResponse(e, cats)
		resp.OrganizationName = row.OrganizationName
		resp.OrganizationSlug = row.OrganizationSlug
		resp.Status = row.Status
		resp.VenueAddress = row.VenueAddress.String
		out = append(out, resp)
	}
	return out, nil
}

func (s *Service) GetEventByIDOrSlug(ctx context.Context, identifier string) (EventResponse, error) {
	row, err := s.repo.GetPublishedEventByIDOrSlug(ctx, identifier)
	if errors.Is(err, pgx.ErrNoRows) {
		return EventResponse{}, ErrNotFound
	} else if err != nil {
		return EventResponse{}, err
	}
	cats, err := s.repo.ListCategoriesByEventForPublicWithMode(ctx, row.ID)
	if err != nil {
		return EventResponse{}, err
	}
	e := db.Event{
		ID:              row.ID,
		OrganizationID:  row.OrganizationID,
		Name:            row.Name,
		Slug:            row.Slug,
		Description:     row.Description,
		EventType:       row.EventType,
		Status:          row.Status,
		BannerObjectKey: row.BannerObjectKey,
		LogoObjectKey:   row.LogoObjectKey,
		VenueName:       row.VenueName,
		VenueAddress:    row.VenueAddress,
		StartsAt:        row.StartsAt,
		EndsAt:          row.EndsAt,
		Faq:             row.Faq,
		Terms:           row.Terms,
		Waiver:          row.Waiver,
		PublishedAt:     row.PublishedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
	resp := s.toEventResponse(e, cats)
	resp.OrganizationName = row.OrganizationName
	resp.OrganizationSlug = row.OrganizationSlug
	resp.Status = row.Status
	resp.VenueAddress = row.VenueAddress.String
	return resp, nil
}

func (s *Service) ListEvents(ctx context.Context, orgSlug string) ([]EventResponse, error) {
	rows, err := s.repo.ListPublishedEventsByOrgSlug(ctx, orgSlug)
	if err != nil {
		return nil, err
	}
	out := make([]EventResponse, 0, len(rows))
	for _, e := range rows {
		cats, _ := s.repo.ListCategoriesByEventForPublicWithMode(ctx, e.ID)
		out = append(out, s.toEventResponse(e, cats))
	}
	return out, nil
}

func (s *Service) GetEvent(ctx context.Context, orgSlug, eventSlug string) (EventResponse, error) {
	e, err := s.repo.GetPublishedEventByOrgAndSlug(ctx, db.GetPublishedEventByOrgAndSlugParams{Slug: orgSlug, Slug_2: eventSlug})
	if errors.Is(err, pgx.ErrNoRows) {
		return EventResponse{}, ErrNotFound
	} else if err != nil {
		return EventResponse{}, err
	}
	cats, err := s.repo.ListCategoriesByEventForPublicWithMode(ctx, e.ID)
	if err != nil {
		return EventResponse{}, err
	}
	return s.toEventResponse(e, cats), nil
}

func (s *Service) toEventResponse(e db.Event, cats []db.EventCategoryWithMode) EventResponse {
	r := EventResponse{
		ID:           e.ID,
		Name:         e.Name,
		Slug:         e.Slug,
		EventType:    e.EventType,
		Description:  e.Description.String,
		VenueName:    e.VenueName.String,
		VenueAddress: e.VenueAddress.String,
		StartsAt:     tptr(e.StartsAt),
		EndsAt:       tptr(e.EndsAt),
		Status:       e.Status,
		Categories:   []CategoryResponse{},
	}
	if e.BannerObjectKey.Valid {
		r.BannerURL = s.store.PublicURL(e.BannerObjectKey.String)
	}
	if e.LogoObjectKey.Valid {
		r.LogoURL = s.store.PublicURL(e.LogoObjectKey.String)
	}
	for _, c := range cats {
		mode := "NORMAL"
		if c.RegistrationMode.Valid && c.RegistrationMode.String != "" {
			mode = c.RegistrationMode.String
		}
		r.Categories = append(r.Categories, CategoryResponse{
			ID:                   c.ID,
			Name:                 c.Name,
			Price:                c.Price,
			Capacity:             c.Capacity,
			RegistrationOpensAt:  c.RegistrationOpensAt.Time,
			RegistrationClosesAt: c.RegistrationClosesAt.Time,
			RegistrationMode:     mode,
		})
	}
	return r
}

func tptr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
