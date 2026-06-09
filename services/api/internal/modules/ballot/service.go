package ballot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

type PoolCreator interface {
	CreatePool(ctx context.Context, orgID, eventID, categoryID uuid.UUID, poolType, name string, slots int, createdBy uuid.UUID) (uuid.UUID, error)
}

type GrantIssuer interface {
	ReserveSlot(ctx context.Context, poolID uuid.UUID) error
	CreateGrant(ctx context.Context, poolID, participantID, eventID, categoryID uuid.UUID, expiresAt time.Time) (uuid.UUID, error)
}

type WaitlistCreator interface {
	CreateWaitlist(ctx context.Context, orgID, eventID, categoryID, createdBy uuid.UUID) (uuid.UUID, error)
	Join(ctx context.Context, waitlistID, participantID uuid.UUID, source string, sourceRefID *uuid.UUID, rank int64) error
}

type Service struct {
	repo     Repository
	audit    AuditRecorder
	pools    PoolCreator
	grants   GrantIssuer
	waitlist WaitlistCreator
}

func NewService(repo Repository, auditRec AuditRecorder, pools PoolCreator, grants GrantIssuer, wl WaitlistCreator) *Service {
	return &Service{repo: repo, audit: auditRec, pools: pools, grants: grants, waitlist: wl}
}

func (s *Service) CreateDraw(ctx context.Context, orgID, eventID, categoryID, createdBy uuid.UUID, req CreateDrawRequest) (db.BallotDraw, error) {
	return s.repo.CreateBallotDraw(ctx, db.CreateBallotDrawParams{
		OrganizationID:      orgID,
		EventID:             eventID,
		CategoryID:          categoryID,
		Quota:               req.Quota,
		WaitlistSize:        pgtype.Int4{Int32: req.WaitlistSize, Valid: true},
		PaymentWindowHours:  req.PaymentWindowHours,
		ApplicationOpensAt:  pgtype.Timestamptz{Time: req.ApplicationOpensAt, Valid: true},
		ApplicationClosesAt: pgtype.Timestamptz{Time: req.ApplicationClosesAt, Valid: true},
		CreatedBy:           createdBy,
	})
}

func (s *Service) OpenDraw(ctx context.Context, drawID, _ uuid.UUID) error {
	draw, err := s.repo.GetBallotDraw(ctx, drawID)
	if err != nil {
		return err
	}
	if draw.Status != DrawStatusPending {
		return ErrBallotClosed
	}
	_, err = s.repo.UpdateBallotDrawStatus(ctx, db.UpdateBallotDrawStatusParams{ID: drawID, Status: DrawStatusOpen})
	return err
}

func (s *Service) CloseDraw(ctx context.Context, drawID, _ uuid.UUID) error {
	draw, err := s.repo.GetBallotDraw(ctx, drawID)
	if err != nil {
		return err
	}
	if draw.Status != DrawStatusOpen {
		return ErrBallotClosed
	}
	_, err = s.repo.UpdateBallotDrawStatus(ctx, db.UpdateBallotDrawStatusParams{ID: drawID, Status: DrawStatusClosed})
	return err
}

func (s *Service) RunDraw(ctx context.Context, drawID, actorID uuid.UUID) error {
	draw, err := s.repo.GetBallotDraw(ctx, drawID)
	if err != nil {
		return err
	}
	if draw.Status != DrawStatusClosed {
		return ErrBallotClosed
	}

	// Idempotency: check if results already exist
	n, err := s.repo.CountBallotDrawResults(ctx, db.CountBallotDrawResultsParams{
		DrawID:  drawID,
		Outcome: OutcomeWinner,
	})
	if err != nil {
		return err
	}
	if n > 0 {
		return nil // already run
	}

	// 1. Commit seed before draw
	nonce := uuid.New()
	seedInput := fmt.Sprintf("%s|%s|%s", draw.EventID, draw.CategoryID, nonce)
	seedHash := sha256.Sum256([]byte(seedInput))
	seed := hex.EncodeToString(seedHash[:])
	if _, err := s.repo.SetBallotDrawSeed(ctx, db.SetBallotDrawSeedParams{
		ID:        drawID,
		Seed:      pgtype.Text{String: seed, Valid: true},
		DrawNonce: &nonce,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // seed already set — idempotent
		}
		return err
	}

	// 2. Load entries (ordered by id ASC — deterministic)
	dbEntries, err := s.repo.ListAppliedEntriesForDraw(ctx, drawID)
	if err != nil {
		return err
	}
	entries := make([]DrawEntry, len(dbEntries))
	for i, e := range dbEntries {
		entries[i] = DrawEntry{ID: e.ID.String()}
	}

	// 3. Shuffle + Assign
	quota := int(draw.Quota)
	waitlistSize := 0
	if draw.WaitlistSize.Valid {
		waitlistSize = int(draw.WaitlistSize.Int32)
	}
	shuffled := Shuffle(seed, entries)
	results := Assign(seed, shuffled, quota, waitlistSize)

	// 4. Write results + collect outcome groups
	winnerIDs, waitlistedIDs, notSelectedIDs := []uuid.UUID{}, []uuid.UUID{}, []uuid.UUID{}
	for i, r := range results {
		entryID := dbEntries[i].ID
		if err := s.repo.InsertBallotDrawResult(ctx, db.InsertBallotDrawResultParams{
			DrawID:        drawID,
			BallotEntryID: entryID,
			Outcome:       r.Outcome,
			Rank:          int32(r.Rank),
			ResultHash:    r.ResultHash,
		}); err != nil {
			return err
		}
		switch r.Outcome {
		case OutcomeWinner:
			winnerIDs = append(winnerIDs, entryID)
		case OutcomeWaitlisted:
			waitlistedIDs = append(waitlistedIDs, entryID)
		default:
			notSelectedIDs = append(notSelectedIDs, entryID)
		}
	}
	if len(winnerIDs) > 0 {
		_ = s.repo.BulkUpdateBallotOutcome(ctx, db.BulkUpdateBallotOutcomeParams{Column1: winnerIDs, Status: OutcomeWinner})
	}
	if len(waitlistedIDs) > 0 {
		_ = s.repo.BulkUpdateBallotOutcome(ctx, db.BulkUpdateBallotOutcomeParams{Column1: waitlistedIDs, Status: OutcomeWaitlisted})
	}
	if len(notSelectedIDs) > 0 {
		_ = s.repo.BulkUpdateBallotOutcome(ctx, db.BulkUpdateBallotOutcomeParams{Column1: notSelectedIDs, Status: OutcomeNotSelected})
	}

	// 5. Create RESERVED pool for winners
	poolID, err := s.pools.CreatePool(ctx, draw.OrganizationID, draw.EventID, draw.CategoryID,
		"RESERVED", fmt.Sprintf("Ballot winners — draw %s", drawID), quota, actorID)
	if err != nil {
		return err
	}

	// 6. Create waitlist for waitlisted entries
	var waitlistID uuid.UUID
	if waitlistSize > 0 {
		waitlistID, err = s.waitlist.CreateWaitlist(ctx, draw.OrganizationID, draw.EventID, draw.CategoryID, actorID)
		if err != nil {
			return err
		}
	}

	// 7. Set pool + waitlist on draw, advance status
	winnerPoolIDPtr := &poolID
	var waitlistIDPtr *uuid.UUID
	if waitlistSize > 0 {
		waitlistIDPtr = &waitlistID
	}
	_ = s.repo.SetBallotDrawPools(ctx, db.SetBallotDrawPoolsParams{
		ID:           drawID,
		WinnerPoolID: winnerPoolIDPtr,
		WaitlistID:   waitlistIDPtr,
	})
	_, err = s.repo.UpdateBallotDrawStatus(ctx, db.UpdateBallotDrawStatusParams{ID: drawID, Status: DrawStatusDrawn})
	return err
}

func (s *Service) AnnounceDraw(ctx context.Context, drawID, _ uuid.UUID) error {
	draw, err := s.repo.GetBallotDraw(ctx, drawID)
	if err != nil {
		return err
	}
	if draw.Status != DrawStatusDrawn {
		return ErrDrawAlreadyRun
	}

	winners, err := s.repo.ListWinnerEntries(ctx, drawID)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(time.Duration(draw.PaymentWindowHours) * time.Hour)
	for _, w := range winners {
		if draw.WinnerPoolID == nil {
			continue
		}
		poolID := *draw.WinnerPoolID
		if err := s.grants.ReserveSlot(ctx, poolID); err != nil {
			continue
		}
		grantID, err := s.grants.CreateGrant(ctx, poolID, w.ParticipantID, draw.EventID, draw.CategoryID, deadline)
		if err != nil {
			continue
		}
		_, _ = s.repo.UpdateBallotEntryStatus(ctx, db.UpdateBallotEntryStatusParams{
			ID:              w.ID,
			Status:          StatusWinner,
			PaymentDeadline: pgtype.Timestamptz{Time: deadline, Valid: true},
			AccessGrantID:   &grantID,
		})
	}

	// Add waitlisted entries to waitlist engine
	if draw.WaitlistID != nil && s.waitlist != nil {
		wlID := *draw.WaitlistID
		waitlisted, _ := s.repo.ListAppliedEntriesForDraw(ctx, drawID)
		for _, e := range waitlisted {
			if e.Status != StatusWaitlisted {
				continue
			}
			_ = s.waitlist.Join(ctx, wlID, e.ParticipantID, "BALLOT", &e.ID, int64(e.ID.ID()))
		}
	}

	_, err = s.repo.UpdateBallotDrawStatus(ctx, db.UpdateBallotDrawStatusParams{ID: drawID, Status: DrawStatusAnnounced})
	return err
}

func (s *Service) ListResults(ctx context.Context, drawID uuid.UUID, limit, offset int32) ([]db.ListBallotDrawResultsRow, error) {
	return s.repo.ListBallotDrawResults(ctx, db.ListBallotDrawResultsParams{DrawID: drawID, Limit: limit, Offset: offset})
}

func (s *Service) ExportResultsCSV(ctx context.Context, drawID uuid.UUID) ([]byte, error) {
	rows, err := s.repo.ListAllDrawResults(ctx, drawID)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"rank", "outcome", "ballot_entry_id", "participant_id", "result_hash"})
	for _, r := range rows {
		_ = w.Write([]string{
			fmt.Sprintf("%d", r.Rank),
			r.Outcome,
			r.BallotEntryID.String(),
			r.ParticipantID.String(),
			r.ResultHash,
		})
	}
	w.Flush()
	return buf.Bytes(), nil
}

func (s *Service) PromoteWaitlist(_ context.Context, _ uuid.UUID) error {
	// waitlist integration wired in server.go via WinnerExpirer
	return nil
}
