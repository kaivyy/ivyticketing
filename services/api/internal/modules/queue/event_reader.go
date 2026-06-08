package queue

import (
	"context"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type dbEventReader struct {
	q *db.Queries
}

// NewDBEventReader returns an EventReader backed by db.Queries.
func NewDBEventReader(q *db.Queries) EventReader {
	return &dbEventReader{q: q}
}

func (r *dbEventReader) GetEventOrgID(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error) {
	ev, err := r.q.GetEventByID(ctx, eventID)
	if err != nil {
		return uuid.Nil, err
	}
	return ev.OrganizationID, nil
}
