package status

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// tsStr renders a timestamptz as RFC3339, or "" when not valid.
func tsStr(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

// tsPtr renders a nullable timestamptz as a *string (nil when not valid).
func tsPtr(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.UTC().Format(time.RFC3339)
	return &s
}
