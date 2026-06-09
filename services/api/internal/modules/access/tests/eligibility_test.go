package access_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/modules/access"
)

type fakeEligRepo struct {
	orderCount   int64
	membershipID string
}

func (r *fakeEligRepo) CountPaidOrdersByUserInOrg(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return r.orderCount, nil
}
func (r *fakeEligRepo) GetUserMembershipID(_ context.Context, _ uuid.UUID) (string, error) {
	return r.membershipID, nil
}
func (r *fakeEligRepo) HasPaidOrderForEvent(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return r.orderCount > 0, nil
}

func rule(t *testing.T, v map[string]any) json.RawMessage {
	t.Helper()
	b, _ := json.Marshal(v)
	return b
}

func TestEligibility_ReturningRunner(t *testing.T) {
	checker := access.NewEligibilityChecker(&fakeEligRepo{orderCount: 1})
	ok, _, _ := checker.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"returning_runner": true}))
	if !ok {
		t.Fatal("user with 1 paid order should pass returning_runner")
	}

	checker2 := access.NewEligibilityChecker(&fakeEligRepo{orderCount: 0})
	ok2, _, _ := checker2.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"returning_runner": true}))
	if ok2 {
		t.Fatal("user with 0 orders should fail returning_runner")
	}
}

func TestEligibility_MinCompletions(t *testing.T) {
	checker := access.NewEligibilityChecker(&fakeEligRepo{orderCount: 3})
	ok, _, _ := checker.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"min_completions": 3}))
	if !ok {
		t.Fatal("3 completions should pass min_completions=3")
	}

	ok2, _, _ := checker.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"min_completions": 4}))
	if ok2 {
		t.Fatal("3 completions should fail min_completions=4")
	}
}

func TestEligibility_MembershipIDPrefix(t *testing.T) {
	checker := access.NewEligibilityChecker(&fakeEligRepo{membershipID: "MEM-12345"})
	ok, _, _ := checker.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"membership_id_prefix": "MEM"}))
	if !ok {
		t.Fatal("MEM-12345 should pass prefix MEM")
	}

	ok2, _, _ := checker.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"membership_id_prefix": "VIP"}))
	if ok2 {
		t.Fatal("MEM-12345 should fail prefix VIP")
	}
}

func TestEligibility_UnknownKeyIgnored(t *testing.T) {
	checker := access.NewEligibilityChecker(&fakeEligRepo{})
	ok, _, _ := checker.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"future_rule_v2": true}))
	if !ok {
		t.Fatal("unknown rule keys should be ignored (pass)")
	}
}
