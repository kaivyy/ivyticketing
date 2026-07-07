package results

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Result status values (mirror race_results.status CHECK constraint).
const (
	StatusFinished = "FINISHED"
	StatusDNF      = "DNF"
	StatusDNS      = "DNS"
)

// Result source values (mirror race_results.source CHECK constraint).
const (
	SourceCSV       = "CSV"
	SourceTimingAPI = "TIMING_API"
)

// ResultView is the API shape of a single race result. Times are surfaced both
// as raw milliseconds (for client-side sorting) and as a pre-formatted
// "HH:MM:SS" string (for direct display).
type ResultView struct {
	ID              uuid.UUID  `json:"id"`
	EventID         uuid.UUID  `json:"eventId"`
	CategoryID      *uuid.UUID `json:"categoryId,omitempty"`
	TicketID        *uuid.UUID `json:"ticketId,omitempty"`
	BibNumber       string     `json:"bibNumber"`
	ParticipantName string     `json:"participantName"`
	Gender          string     `json:"gender,omitempty"`
	Age             *int       `json:"age,omitempty"`
	AgeGroup        string     `json:"ageGroup,omitempty"`
	Status          string     `json:"status"`
	ChipTimeMs      *int64     `json:"chipTimeMs,omitempty"`
	GunTimeMs       *int64     `json:"gunTimeMs,omitempty"`
	ChipTime        string     `json:"chipTime,omitempty"`
	GunTime         string     `json:"gunTime,omitempty"`
	RankOverall     *int       `json:"rankOverall,omitempty"`
	RankGender      *int       `json:"rankGender,omitempty"`
	RankCategory    *int       `json:"rankCategory,omitempty"`
	RankAgeGroup    *int       `json:"rankAgeGroup,omitempty"`
	Source          string     `json:"source"`
	FinishedAt      *time.Time `json:"finishedAt,omitempty"`
}

func toResultView(r db.RaceResult) ResultView {
	v := ResultView{
		ID:              r.ID,
		EventID:         r.EventID,
		CategoryID:      r.CategoryID,
		TicketID:        r.TicketID,
		BibNumber:       r.BibNumber,
		ParticipantName: r.ParticipantName,
		Status:          r.Status,
		Source:          r.Source,
	}
	if r.Gender.Valid {
		v.Gender = r.Gender.String
	}
	if r.AgeGroup.Valid {
		v.AgeGroup = r.AgeGroup.String
	}
	if r.Age.Valid {
		a := int(r.Age.Int32)
		v.Age = &a
	}
	if r.ChipTimeMs.Valid {
		ms := r.ChipTimeMs.Int64
		v.ChipTimeMs = &ms
		v.ChipTime = formatDuration(ms)
	}
	if r.GunTimeMs.Valid {
		ms := r.GunTimeMs.Int64
		v.GunTimeMs = &ms
		v.GunTime = formatDuration(ms)
	}
	if r.RankOverall.Valid {
		n := int(r.RankOverall.Int32)
		v.RankOverall = &n
	}
	if r.RankGender.Valid {
		n := int(r.RankGender.Int32)
		v.RankGender = &n
	}
	if r.RankCategory.Valid {
		n := int(r.RankCategory.Int32)
		v.RankCategory = &n
	}
	if r.RankAgeGroup.Valid {
		n := int(r.RankAgeGroup.Int32)
		v.RankAgeGroup = &n
	}
	if r.FinishedAt.Valid {
		t := r.FinishedAt.Time
		v.FinishedAt = &t
	}
	return v
}

// ImportSummary is returned after a CSV or timing-API import.
type ImportSummary struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
	Ranked   bool     `json:"ranked"`
}

// --- certificate templates ---

// CertificateTemplateView is the API shape of a certificate template.
type CertificateTemplateView struct {
	ID            uuid.UUID `json:"id"`
	EventID       uuid.UUID `json:"eventId"`
	Name          string    `json:"name"`
	Title         string    `json:"title"`
	Subtitle      string    `json:"subtitle"`
	BodyTemplate  string    `json:"bodyTemplate"`
	BackgroundURL string    `json:"backgroundUrl,omitempty"`
	IsActive      bool      `json:"isActive"`
	CreatedAt     time.Time `json:"createdAt"`
}

func toTemplateView(t db.CertificateTemplate) CertificateTemplateView {
	v := CertificateTemplateView{
		ID:           t.ID,
		EventID:      t.EventID,
		Name:         t.Name,
		Title:        t.Title,
		Subtitle:     t.Subtitle,
		BodyTemplate: t.BodyTemplate,
		IsActive:     t.IsActive,
		CreatedAt:    t.CreatedAt.Time,
	}
	if t.BackgroundUrl.Valid {
		v.BackgroundURL = t.BackgroundUrl.String
	}
	return v
}

// CreateTemplateRequest is the organizer payload for creating/updating a
// certificate template.
type CreateTemplateRequest struct {
	Name          string `json:"name"`
	Title         string `json:"title"`
	Subtitle      string `json:"subtitle"`
	BodyTemplate  string `json:"bodyTemplate"`
	BackgroundURL string `json:"backgroundUrl"`
	IsActive      bool   `json:"isActive"`
}

// CertificateRender is a resolved certificate: template text with placeholders
// substituted from the finisher's result, plus the raw result for client-side
// layout. The frontend draws this over the background image.
type CertificateRender struct {
	Title         string     `json:"title"`
	Subtitle      string     `json:"subtitle"`
	Body          string     `json:"body"`
	BackgroundURL string     `json:"backgroundUrl,omitempty"`
	Result        ResultView `json:"result"`
}

// --- pgtype helpers ---

func pgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func pgInt4Ptr(n *int) pgtype.Int4 {
	if n == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: int32(*n), Valid: true}
}

func pgInt8Ptr(n *int64) pgtype.Int8 {
	if n == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: *n, Valid: true}
}

func pgTimestamptzPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// formatDuration renders elapsed milliseconds as "H:MM:SS" (hours not
// zero-padded) — the conventional finish-time display for road races.
func formatDuration(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	totalSec := ms / 1000
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	return itoa2(int(h), false) + ":" + itoa2(int(m), true) + ":" + itoa2(int(s), true)
}

// itoa2 formats n, optionally zero-padded to 2 digits. Kept tiny and
// allocation-light for the hot import/render paths.
func itoa2(n int, pad bool) string {
	if n < 0 {
		n = 0
	}
	if pad && n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
