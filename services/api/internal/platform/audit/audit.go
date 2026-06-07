package audit

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Logger writes audit entries. Failures are logged, never fatal — an audit
// write must not break the user-facing action.
type Logger struct {
	q   *db.Queries
	log *slog.Logger
}

func NewLogger(q *db.Queries, log *slog.Logger) *Logger {
	return &Logger{q: q, log: log}
}

type Entry struct {
	OrganizationID *uuid.UUID
	ActorUserID    *uuid.UUID
	Action         string
	TargetType     string
	TargetID       string
	Metadata       map[string]any
}

func (l *Logger) Record(ctx context.Context, e Entry) {
	var meta []byte
	if e.Metadata != nil {
		meta, _ = json.Marshal(e.Metadata)
	}
	err := l.q.CreateAuditLog(ctx, db.CreateAuditLogParams{
		OrganizationID: e.OrganizationID,
		ActorUserID:    e.ActorUserID,
		Action:         e.Action,
		TargetType:     nullableText(e.TargetType),
		TargetID:       nullableText(e.TargetID),
		Metadata:       meta,
	})
	if err != nil {
		l.log.Error("audit write failed", "action", e.Action, "error", err)
	}
}

func nullableText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}
