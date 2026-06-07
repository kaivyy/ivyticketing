package categories

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

func validate(req WriteRequest) error {
	if req.Price < 0 {
		return ErrInvalidPrice
	}
	if req.Capacity <= 0 {
		return ErrInvalidCapacity
	}
	if !req.RegistrationOpensAt.Before(req.RegistrationClosesAt) {
		return ErrInvalidWindow
	}
	if req.MinAge != nil && *req.MinAge < 0 {
		return ErrInvalidAge
	}
	if req.MaxOrderPerUser < 1 {
		return ErrInvalidMaxOrder
	}
	return nil
}

// assertEvent confirms the event exists and belongs to orgID (tenant guard).
func (s *Service) assertEvent(ctx context.Context, orgID, eventID uuid.UUID) error {
	e, err := s.repo.GetEventByID(ctx, eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEventNotFound
	} else if err != nil {
		return err
	}
	if e.OrganizationID != orgID {
		return ErrEventNotFound
	}
	return nil
}

func (s *Service) Create(ctx context.Context, orgID, eventID uuid.UUID, req WriteRequest) (Response, error) {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return Response{}, err
	}
	if err := validate(req); err != nil {
		return Response{}, err
	}
	c, err := s.repo.CreateCategory(ctx, db.CreateCategoryParams{
		OrganizationID:       orgID,
		EventID:              eventID,
		Name:                 req.Name,
		Price:                req.Price,
		Capacity:             req.Capacity,
		RegistrationOpensAt:  pgtype.Timestamptz{Time: req.RegistrationOpensAt, Valid: true},
		RegistrationClosesAt: pgtype.Timestamptz{Time: req.RegistrationClosesAt, Valid: true},
		BibPrefix:            nullText(req.BibPrefix),
		MinAge:               nullInt4(req.MinAge),
		MaxOrderPerUser:      req.MaxOrderPerUser,
	})
	if err != nil {
		return Response{}, err
	}
	return toResponse(c), nil
}

func (s *Service) List(ctx context.Context, orgID, eventID uuid.UUID) ([]Response, error) {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListCategoriesByEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	out := make([]Response, 0, len(rows))
	for _, c := range rows {
		out = append(out, toResponse(c))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, orgID, eventID, categoryID uuid.UUID) (Response, error) {
	c, err := s.loadCategory(ctx, orgID, eventID, categoryID)
	if err != nil {
		return Response{}, err
	}
	return toResponse(c), nil
}

func (s *Service) Update(ctx context.Context, orgID, eventID, categoryID uuid.UUID, req WriteRequest) (Response, error) {
	if _, err := s.loadCategory(ctx, orgID, eventID, categoryID); err != nil {
		return Response{}, err
	}
	if err := validate(req); err != nil {
		return Response{}, err
	}
	c, err := s.repo.UpdateCategory(ctx, db.UpdateCategoryParams{
		ID:                   categoryID,
		Name:                 req.Name,
		Price:                req.Price,
		Capacity:             req.Capacity,
		RegistrationOpensAt:  pgtype.Timestamptz{Time: req.RegistrationOpensAt, Valid: true},
		RegistrationClosesAt: pgtype.Timestamptz{Time: req.RegistrationClosesAt, Valid: true},
		BibPrefix:            nullText(req.BibPrefix),
		MinAge:               nullInt4(req.MinAge),
		MaxOrderPerUser:      req.MaxOrderPerUser,
		EventID:              eventID,
	})
	if err != nil {
		return Response{}, err
	}
	return toResponse(c), nil
}

func (s *Service) Delete(ctx context.Context, orgID, eventID, categoryID uuid.UUID) error {
	if _, err := s.loadCategory(ctx, orgID, eventID, categoryID); err != nil {
		return err
	}
	return s.repo.DeleteCategory(ctx, db.DeleteCategoryParams{ID: categoryID, EventID: eventID})
}

// loadCategory confirms the category belongs to the event AND the event to the org.
func (s *Service) loadCategory(ctx context.Context, orgID, eventID, categoryID uuid.UUID) (db.EventCategory, error) {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return db.EventCategory{}, err
	}
	c, err := s.repo.GetCategoryByID(ctx, categoryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.EventCategory{}, ErrNotFound
	} else if err != nil {
		return db.EventCategory{}, err
	}
	if c.EventID != eventID || c.OrganizationID != orgID {
		return db.EventCategory{}, ErrNotFound
	}
	return c, nil
}

func toResponse(c db.EventCategory) Response {
	r := Response{
		ID: c.ID, EventID: c.EventID, Name: c.Name, Price: c.Price, Capacity: c.Capacity,
		RegistrationOpensAt:  c.RegistrationOpensAt.Time,
		RegistrationClosesAt: c.RegistrationClosesAt.Time,
		BibPrefix:            c.BibPrefix.String,
		MaxOrderPerUser:      c.MaxOrderPerUser,
		CreatedAt:            c.CreatedAt.Time,
	}
	if c.MinAge.Valid {
		v := c.MinAge.Int32
		r.MinAge = &v
	}
	return r
}

func nullText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func nullInt4(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}
