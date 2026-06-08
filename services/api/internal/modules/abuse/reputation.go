package abuse

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// ReputationStore is the minimal repo surface reputation needs.
type ReputationStore interface {
	GetReputation(ctx context.Context, arg db.GetReputationParams) (db.IpReputation, error)
	BumpReputation(ctx context.Context, arg db.BumpReputationParams) (db.IpReputation, error)
}

type Reputation struct {
	repo               ReputationStore
	challengeThreshold int
	denyThreshold      int
}

func NewReputation(repo ReputationStore, challenge, deny int) *Reputation {
	return &Reputation{repo: repo, challengeThreshold: challenge, denyThreshold: deny}
}

func (r *Reputation) Score(ctx context.Context, subjectType, subjectValue string) int {
	rec, err := r.repo.GetReputation(ctx, db.GetReputationParams{SubjectType: subjectType, SubjectValue: subjectValue})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0
		}
		return 0 // fail-open on read error
	}
	return int(rec.Score)
}

func (r *Reputation) Bump(ctx context.Context, subjectType, subjectValue string, delta int, reason string) {
	_, _ = r.repo.BumpReputation(ctx, db.BumpReputationParams{
		SubjectType:  subjectType,
		SubjectValue: subjectValue,
		Score:        int32(delta),
	})
}

func (r *Reputation) ShouldChallenge(ctx context.Context, subjectType, subjectValue string) bool {
	return r.Score(ctx, subjectType, subjectValue) >= r.challengeThreshold
}

func (r *Reputation) ShouldDeny(ctx context.Context, subjectType, subjectValue string) bool {
	return r.Score(ctx, subjectType, subjectValue) >= r.denyThreshold
}
