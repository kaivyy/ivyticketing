package reporting

import "github.com/jackc/pgx/v5/pgtype"

func pgInt4(v int32) pgtype.Int4 { return pgtype.Int4{Int32: v, Valid: true} }

func pgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}
