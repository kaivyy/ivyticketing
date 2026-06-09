package access

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

// EligibilityRepo provides the data queries needed by EligibilityChecker.
type EligibilityRepo interface {
	CountPaidOrdersByUserInOrg(ctx context.Context, userID, orgID uuid.UUID) (int64, error)
	GetUserMembershipID(ctx context.Context, userID uuid.UUID) (string, error)
	HasPaidOrderForEvent(ctx context.Context, userID, eventID uuid.UUID) (bool, error)
}

// EligibilityChecker evaluates jsonb eligibility rules against a user.
type EligibilityChecker struct{ repo EligibilityRepo }

// NewEligibilityChecker returns a new EligibilityChecker backed by the given repo.
func NewEligibilityChecker(repo EligibilityRepo) *EligibilityChecker {
	return &EligibilityChecker{repo: repo}
}

// Check evaluates rule (jsonb) for userID. orgID is used for org-scoped queries.
// Returns (eligible bool, reason string, error).
// Unknown rule keys are ignored for forward-compatibility.
func (e *EligibilityChecker) Check(ctx context.Context, userID, orgID uuid.UUID, rule json.RawMessage) (bool, string, error) {
	if len(rule) == 0 {
		return true, "", nil
	}
	var r map[string]any
	if err := json.Unmarshal(rule, &r); err != nil {
		return false, "invalid_rule", nil
	}
	for key, val := range r {
		switch key {
		case "returning_runner":
			if b, _ := val.(bool); b {
				n, err := e.repo.CountPaidOrdersByUserInOrg(ctx, userID, orgID)
				if err != nil {
					return false, "db_error", err
				}
				if n < 1 {
					return false, "not_returning_runner", nil
				}
			}
		case "min_completions":
			var min int64
			switch v := val.(type) {
			case float64:
				min = int64(v)
			case int64:
				min = v
			}
			n, err := e.repo.CountPaidOrdersByUserInOrg(ctx, userID, orgID)
			if err != nil {
				return false, "db_error", err
			}
			if n < min {
				return false, "insufficient_completions", nil
			}
		case "membership_id_prefix":
			prefix, _ := val.(string)
			mid, err := e.repo.GetUserMembershipID(ctx, userID)
			if err != nil {
				return false, "db_error", err
			}
			if !strings.HasPrefix(mid, prefix) {
				return false, "membership_id_mismatch", nil
			}
		case "event_completed":
			eventIDStr, _ := val.(string)
			eventID, err := uuid.Parse(eventIDStr)
			if err != nil {
				return false, "invalid_event_id", nil
			}
			ok, err := e.repo.HasPaidOrderForEvent(ctx, userID, eventID)
			if err != nil {
				return false, "db_error", err
			}
			if !ok {
				return false, "event_not_completed", nil
			}
		// unknown keys: ignored for forward-compatibility
		}
	}
	return true, "", nil
}
