package results

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

// Service coordinates result import, ranking, listing and certificate rendering.
type Service struct {
	repo  Repository
	audit *audit.Logger
	log   *slog.Logger
}

// NewService constructs a results Service.
func NewService(repo Repository, auditLog *audit.Logger, log *slog.Logger) *Service {
	return &Service{repo: repo, audit: auditLog, log: log}
}

// ImportCSV parses a results CSV, upserts one row per data line (idempotent on
// (event, bib)), then recomputes all rank columns. The header is matched
// case-insensitively; only bib and name are required. Unparseable rows are
// skipped and reported in the summary rather than aborting the whole import, so
// a single malformed line never loses the rest of a timing export.
//
// Recognized columns (any subset, in any order):
//
//	bib | name | gender | age | age_group | status | chip_time | gun_time | finished_at
//
// Times accept either "H:MM:SS[.mmm]" / "MM:SS" clock format or a raw
// millisecond integer.
func (s *Service) ImportCSV(ctx context.Context, orgID, eventID, userID uuid.UUID, body io.Reader) (ImportSummary, error) {
	reader := csv.NewReader(body)
	reader.FieldsPerRecord = -1 // tolerate ragged rows; we validate per-field
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return ImportSummary{}, ErrEmptyCSV
		}
		return ImportSummary{}, ErrInvalidCSV
	}
	cols := indexColumns(header)
	if _, ok := cols[colBib]; !ok {
		return ImportSummary{}, ErrMissingBibColumn
	}

	var summary ImportSummary
	line := 1 // header consumed
	for {
		line++
		rec, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			summary.Skipped++
			summary.Errors = append(summary.Errors, "baris "+strconv.Itoa(line)+": format tidak valid")
			continue
		}
		params, perr := s.rowToParams(orgID, eventID, cols, rec)
		if perr != nil {
			summary.Skipped++
			summary.Errors = append(summary.Errors, "baris "+strconv.Itoa(line)+": "+perr.Error())
			continue
		}
		if _, err := s.repo.UpsertRaceResult(ctx, params); err != nil {
			summary.Skipped++
			summary.Errors = append(summary.Errors, "baris "+strconv.Itoa(line)+": gagal simpan")
			s.log.Error("upsert race result failed", "event", eventID, "bib", params.BibNumber, "error", err)
			continue
		}
		summary.Imported++
	}

	if summary.Imported > 0 {
		if err := s.recomputeRanks(ctx, eventID); err != nil {
			return summary, err
		}
		summary.Ranked = true
	}

	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			ActorUserID:    &userID,
			Action:         "results.import",
			TargetType:     "event",
			TargetID:       eventID.String(),
			Metadata:       map[string]any{"imported": summary.Imported, "skipped": summary.Skipped, "source": SourceCSV},
		})
	}
	return summary, nil
}

// Recompute re-runs all four ranking passes for an event. Exposed so an
// organizer can force a recompute (e.g. after manually editing a status)
// without re-importing.
func (s *Service) Recompute(ctx context.Context, orgID, eventID, userID uuid.UUID) error {
	if err := s.recomputeRanks(ctx, eventID); err != nil {
		return err
	}
	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			ActorUserID:    &userID,
			Action:         "results.recompute",
			TargetType:     "event",
			TargetID:       eventID.String(),
		})
	}
	return nil
}

func (s *Service) recomputeRanks(ctx context.Context, eventID uuid.UUID) error {
	if err := s.repo.RankOverall(ctx, eventID); err != nil {
		return err
	}
	if err := s.repo.RankGender(ctx, eventID); err != nil {
		return err
	}
	if err := s.repo.RankCategory(ctx, eventID); err != nil {
		return err
	}
	if err := s.repo.RankAgeGroup(ctx, eventID); err != nil {
		return err
	}
	return nil
}

// List returns a page of results with optional category/gender filters plus the
// total count for pagination.
func (s *Service) List(ctx context.Context, eventID uuid.UUID, categoryID *uuid.UUID, gender string, limit, offset int32) ([]ResultView, int64, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.repo.ListRaceResults(ctx, db.ListRaceResultsParams{
		EventID:    eventID,
		Limit:      limit,
		Offset:     offset,
		CategoryID: categoryID,
		Gender:     pgText(gender),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountRaceResults(ctx, db.CountRaceResultsParams{
		EventID:    eventID,
		CategoryID: categoryID,
		Gender:     pgText(gender),
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]ResultView, 0, len(rows))
	for _, r := range rows {
		out = append(out, toResultView(r))
	}
	return out, total, nil
}

// GetByBib returns a single result by bib (participant lookup).
func (s *Service) GetByBib(ctx context.Context, eventID uuid.UUID, bib string) (ResultView, error) {
	r, err := s.repo.GetRaceResultByBib(ctx, db.GetRaceResultByBibParams{EventID: eventID, BibNumber: bib})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResultView{}, ErrResultNotFound
		}
		return ResultView{}, err
	}
	return toResultView(r), nil
}

// GetByTicket returns the result linked to a ticket (participant "my result").
func (s *Service) GetByTicket(ctx context.Context, ticketID uuid.UUID) (ResultView, error) {
	tid := ticketID
	r, err := s.repo.GetRaceResultByTicket(ctx, &tid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResultView{}, ErrResultNotFound
		}
		return ResultView{}, err
	}
	return toResultView(r), nil
}

// DeleteAll clears every result for an event (organizer re-import from scratch).
func (s *Service) DeleteAll(ctx context.Context, orgID, eventID, userID uuid.UUID) error {
	if err := s.repo.DeleteRaceResultsByEvent(ctx, eventID); err != nil {
		return err
	}
	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			ActorUserID:    &userID,
			Action:         "results.delete_all",
			TargetType:     "event",
			TargetID:       eventID.String(),
		})
	}
	return nil
}

// rowToParams maps a CSV record to upsert params using the resolved column
// index. bib and name are required; everything else is optional.
func (s *Service) rowToParams(orgID, eventID uuid.UUID, cols map[string]int, rec []string) (db.UpsertRaceResultParams, error) {
	get := func(name string) string {
		if i, ok := cols[name]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}

	bib := get(colBib)
	if bib == "" {
		return db.UpsertRaceResultParams{}, errors.New("bib kosong")
	}
	name := get(colName)
	if name == "" {
		name = bib // fall back to bib so the row is still identifiable
	}

	params := db.UpsertRaceResultParams{
		OrganizationID:  orgID,
		EventID:         eventID,
		BibNumber:       bib,
		ParticipantName: name,
		Status:          StatusFinished,
		Source:          SourceCSV,
		FinishedAt:      pgTimestamptzPtr(nil),
	}

	if g := normalizeGender(get(colGender)); g != "" {
		params.Gender = pgText(g)
	}
	if ag := get(colAgeGroup); ag != "" {
		params.AgeGroup = pgText(ag)
	}
	if a := get(colAge); a != "" {
		if n, err := strconv.Atoi(a); err == nil && n >= 0 && n <= 130 {
			params.Age = pgInt4Ptr(&n)
		}
	}
	if st := normalizeStatus(get(colStatus)); st != "" {
		params.Status = st
	}
	if chip := get(colChipTime); chip != "" {
		ms, err := parseDurationMs(chip)
		if err != nil {
			return db.UpsertRaceResultParams{}, errors.New("chip_time tidak valid")
		}
		params.ChipTimeMs = pgInt8Ptr(&ms)
	}
	if gun := get(colGunTime); gun != "" {
		ms, err := parseDurationMs(gun)
		if err != nil {
			return db.UpsertRaceResultParams{}, errors.New("gun_time tidak valid")
		}
		params.GunTimeMs = pgInt8Ptr(&ms)
	}
	// A finisher with no time recorded is treated as DNF so it is never ranked.
	if params.Status == StatusFinished && !params.ChipTimeMs.Valid && !params.GunTimeMs.Valid {
		params.Status = StatusDNF
	}
	if fa := get(colFinishedAt); fa != "" {
		if t, err := time.Parse(time.RFC3339, fa); err == nil {
			params.FinishedAt = pgTimestamptzPtr(&t)
		}
	}
	return params, nil
}

// --- certificate rendering ---

// GetCertificate returns the rendered certificate for a ticket: the active
// template with placeholders substituted from the ticket's finisher row.
func (s *Service) GetCertificate(ctx context.Context, eventID, ticketID uuid.UUID) (CertificateRender, error) {
	tid := ticketID
	result, err := s.repo.GetRaceResultByTicket(ctx, &tid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CertificateRender{}, ErrCertNotEligible
		}
		return CertificateRender{}, err
	}
	if result.Status != StatusFinished {
		return CertificateRender{}, ErrCertNotEligible
	}
	tmpl, err := s.repo.GetActiveCertificateTemplate(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CertificateRender{}, ErrNoActiveTemplate
		}
		return CertificateRender{}, err
	}
	view := toResultView(result)
	subs := certificateSubstitutions(view)
	return CertificateRender{
		Title:         applyPlaceholders(tmpl.Title, subs),
		Subtitle:      applyPlaceholders(tmpl.Subtitle, subs),
		Body:          applyPlaceholders(tmpl.BodyTemplate, subs),
		BackgroundURL: pgTextValue(tmpl.BackgroundUrl),
		Result:        view,
	}, nil
}

// --- certificate templates CRUD ---

func (s *Service) CreateTemplate(ctx context.Context, orgID, eventID, userID uuid.UUID, req CreateTemplateRequest) (CertificateTemplateView, error) {
	if strings.TrimSpace(req.Name) == "" {
		return CertificateTemplateView{}, ErrInvalidPayload
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Certificate of Completion"
	}
	if req.IsActive {
		if err := s.repo.DeactivateCertificateTemplatesForEvent(ctx, eventID); err != nil {
			return CertificateTemplateView{}, err
		}
	}
	tmpl, err := s.repo.CreateCertificateTemplate(ctx, db.CreateCertificateTemplateParams{
		OrganizationID: orgID,
		EventID:        eventID,
		Name:           strings.TrimSpace(req.Name),
		Title:          title,
		Subtitle:       req.Subtitle,
		BodyTemplate:   req.BodyTemplate,
		BackgroundUrl:  pgText(strings.TrimSpace(req.BackgroundURL)),
		IsActive:       req.IsActive,
	})
	if err != nil {
		return CertificateTemplateView{}, err
	}
	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			ActorUserID:    &userID,
			Action:         "results.template_created",
			TargetType:     "certificate_template",
			TargetID:       tmpl.ID.String(),
		})
	}
	return toTemplateView(tmpl), nil
}

func (s *Service) UpdateTemplate(ctx context.Context, orgID, templateID, userID uuid.UUID, req CreateTemplateRequest) (CertificateTemplateView, error) {
	existing, err := s.repo.GetCertificateTemplateByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CertificateTemplateView{}, ErrTemplateNotFound
		}
		return CertificateTemplateView{}, err
	}
	if existing.OrganizationID != orgID {
		return CertificateTemplateView{}, ErrTemplateNotFound
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Certificate of Completion"
	}
	if req.IsActive {
		if err := s.repo.DeactivateCertificateTemplatesForEvent(ctx, existing.EventID); err != nil {
			return CertificateTemplateView{}, err
		}
	}
	tmpl, err := s.repo.UpdateCertificateTemplate(ctx, db.UpdateCertificateTemplateParams{
		ID:             templateID,
		OrganizationID: orgID,
		Name:           strings.TrimSpace(req.Name),
		Title:          title,
		Subtitle:       req.Subtitle,
		BodyTemplate:   req.BodyTemplate,
		BackgroundUrl:  pgText(strings.TrimSpace(req.BackgroundURL)),
		IsActive:       req.IsActive,
	})
	if err != nil {
		return CertificateTemplateView{}, err
	}
	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			ActorUserID:    &userID,
			Action:         "results.template_updated",
			TargetType:     "certificate_template",
			TargetID:       tmpl.ID.String(),
		})
	}
	return toTemplateView(tmpl), nil
}

func (s *Service) ListTemplates(ctx context.Context, eventID uuid.UUID) ([]CertificateTemplateView, error) {
	rows, err := s.repo.ListCertificateTemplatesByEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	out := make([]CertificateTemplateView, 0, len(rows))
	for _, t := range rows {
		out = append(out, toTemplateView(t))
	}
	return out, nil
}

func (s *Service) DeleteTemplate(ctx context.Context, orgID, templateID, userID uuid.UUID) error {
	if err := s.repo.DeleteCertificateTemplate(ctx, db.DeleteCertificateTemplateParams{ID: templateID, OrganizationID: orgID}); err != nil {
		return err
	}
	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			ActorUserID:    &userID,
			Action:         "results.template_deleted",
			TargetType:     "certificate_template",
			TargetID:       templateID.String(),
		})
	}
	return nil
}
