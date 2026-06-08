package abuse_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/abuse"
)

type fakeRepRepo struct {
	scores map[string]int32
}

func (f *fakeRepRepo) GetReputation(ctx context.Context, arg db.GetReputationParams) (db.IpReputation, error) {
	if v, ok := f.scores[arg.SubjectType+":"+arg.SubjectValue]; ok {
		return db.IpReputation{SubjectType: arg.SubjectType, SubjectValue: arg.SubjectValue, Score: v}, nil
	}
	return db.IpReputation{}, pgx.ErrNoRows
}

func (f *fakeRepRepo) BumpReputation(ctx context.Context, arg db.BumpReputationParams) (db.IpReputation, error) {
	if f.scores == nil {
		f.scores = map[string]int32{}
	}
	f.scores[arg.SubjectType+":"+arg.SubjectValue] += arg.Score
	return db.IpReputation{Score: f.scores[arg.SubjectType+":"+arg.SubjectValue]}, nil
}

func TestReputation_BumpAndThresholds(t *testing.T) {
	repo := &fakeRepRepo{scores: map[string]int32{}}
	rep := abuse.NewReputation(repo, 10, 25) // challenge=10, deny=25

	rep.Bump(context.Background(), abuse.SubjectIP, "1.2.3.4", abuse.BumpBlockedHit, "blocked")
	if got := rep.Score(context.Background(), abuse.SubjectIP, "1.2.3.4"); got != 5 {
		t.Fatalf("score = %d, want 5", got)
	}
	if rep.ShouldChallenge(context.Background(), abuse.SubjectIP, "1.2.3.4") {
		t.Fatal("score 5 should not trigger challenge (threshold 10)")
	}
	// bump to 11 → challenge
	rep.Bump(context.Background(), abuse.SubjectIP, "1.2.3.4", 6, "x")
	if !rep.ShouldChallenge(context.Background(), abuse.SubjectIP, "1.2.3.4") {
		t.Fatal("score 11 should trigger challenge")
	}
	if rep.ShouldDeny(context.Background(), abuse.SubjectIP, "1.2.3.4") {
		t.Fatal("score 11 should not deny (threshold 25)")
	}
}
