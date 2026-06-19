package notifications

import (
	"context"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// ParticipantLookup resolves a participant ID to email + name.
type ParticipantLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (email string, name string, err error)
}

type userLookup struct{ q *db.Queries }

// NewParticipantLookup creates a lookup backed by the users table.
func NewParticipantLookup(queries *db.Queries) ParticipantLookup {
	return &userLookup{q: queries}
}

func (u *userLookup) GetByID(ctx context.Context, id uuid.UUID) (string, string, error) {
	user, err := u.q.GetUserByID(ctx, id)
	if err != nil {
		return "", "", err
	}
	return user.Email, user.FullName, nil
}
