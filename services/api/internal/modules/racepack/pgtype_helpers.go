package racepack

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// pgtypeText makes a non-NULL pgtype.Text from a Go string.
func pgtypeText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// pgtypeDate makes a non-NULL pgtype.Date from a Go time.Time.
func pgtypeDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

// pgtypeTimestamptz makes a non-NULL pgtype.Timestamptz from a Go time.Time.
func pgtypeTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// errorsIsNoRows reports whether err is pgx.ErrNoRows. Used to translate
// "no row updated" (capacity full) into ErrSlotFull.
func errorsIsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
