package abuse_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/abuse"
)

type fakeBlockRepo struct {
	blocked map[string]bool // "type:value" → blocked
	rules   []db.IpRule
}

func (f *fakeBlockRepo) GetBlockedSubject(ctx context.Context, arg db.GetBlockedSubjectParams) (db.BlockedSubject, error) {
	if f.blocked[arg.SubjectType+":"+arg.SubjectValue] {
		return db.BlockedSubject{SubjectType: arg.SubjectType, SubjectValue: arg.SubjectValue}, nil
	}
	return db.BlockedSubject{}, pgx.ErrNoRows
}

func (f *fakeBlockRepo) ListIPRules(ctx context.Context) ([]db.IpRule, error) { return f.rules, nil }

func TestIsBlocked_User(t *testing.T) {
	repo := &fakeBlockRepo{blocked: map[string]bool{"user:u1": true}}
	bl := abuse.NewBlocklist(repo)
	if !bl.IsBlocked(context.Background(), "u1", "1.2.3.4") {
		t.Fatal("u1 should be blocked")
	}
	if bl.IsBlocked(context.Background(), "u2", "1.2.3.4") {
		t.Fatal("u2 should not be blocked")
	}
}

func TestIPRule_DenyAndAllow(t *testing.T) {
	repo := &fakeBlockRepo{
		blocked: map[string]bool{},
		rules: []db.IpRule{
			{Cidr: "203.0.113.0/24", Rule: "deny"},
			{Cidr: "203.0.113.5/32", Rule: "allow"},
		},
	}
	bl := abuse.NewBlocklist(repo)
	if !bl.IsBlocked(context.Background(), "", "203.0.113.9") {
		t.Fatal("203.0.113.9 should be denied by CIDR")
	}
	if bl.IsBlocked(context.Background(), "", "203.0.113.5") {
		t.Fatal("203.0.113.5 should be allowed (allow wins)")
	}
	_ = pgtype.Text{}
}
