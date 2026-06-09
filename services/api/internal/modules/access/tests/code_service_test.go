package access_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/access"
)

// codeRepo embeds fakeAccessRepo and overrides the code-related methods.
type codeRepo struct {
	fakeAccessRepo
	codes         map[string]db.AccessCode // keyed by code_value_hash
	useCounts     map[uuid.UUID]int
	incrementErr  error
}

func (r *codeRepo) GetAccessCodeByHash(_ context.Context, arg db.GetAccessCodeByHashParams) (db.AccessCode, error) {
	if c, ok := r.codes[arg.CodeValueHash]; ok {
		return c, nil
	}
	return db.AccessCode{}, pgx.ErrNoRows
}

func (r *codeRepo) IncrementCodeUseCount(_ context.Context, id uuid.UUID) (db.AccessCode, error) {
	if r.incrementErr != nil {
		return db.AccessCode{}, r.incrementErr
	}
	r.useCounts[id]++
	if c, ok := r.codes[r.codeHashByID(id)]; ok {
		if int32(r.useCounts[id]) > c.MaxUses {
			return db.AccessCode{}, pgx.ErrNoRows
		}
		return c, nil
	}
	return db.AccessCode{}, nil
}

func (r *codeRepo) codeHashByID(id uuid.UUID) string {
	for h, c := range r.codes {
		if c.ID == id {
			return h
		}
	}
	return ""
}

func (r *codeRepo) CreateAccessCode(_ context.Context, _ db.CreateAccessCodeParams) (db.AccessCode, error) {
	return db.AccessCode{}, nil
}
func (r *codeRepo) ListAccessCodesByEvent(_ context.Context, _ db.ListAccessCodesByEventParams) ([]db.AccessCode, error) {
	return nil, nil
}
func (r *codeRepo) RevokeAccessCode(_ context.Context, _ uuid.UUID) error { return nil }
func (r *codeRepo) ListActiveGrantsForParticipant(_ context.Context, _ db.ListActiveGrantsForParticipantParams) ([]db.AccessGrant, error) {
	return nil, nil
}

func (r *codeRepo) CreateAccessGrant(_ context.Context, _ db.CreateAccessGrantParams) (db.AccessGrant, error) {
	return db.AccessGrant{ID: uuid.New()}, nil
}

func validCode() db.AccessCode {
	return db.AccessCode{
		ID:            uuid.New(),
		CodeValueHash: access.HashCode("SECRET123"),
		MaxUses:       1,
		UseCount:      0,
		ValidFrom:     pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
		ValidUntil:    pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}
}

func newCodeRepo(code db.AccessCode) *codeRepo {
	return &codeRepo{
		codes:     map[string]db.AccessCode{code.CodeValueHash: code},
		useCounts: map[uuid.UUID]int{},
	}
}

func TestRedeem_ValidCode_IssuesGrant(t *testing.T) {
	code := validCode()
	repo := newCodeRepo(code)
	svc := access.NewCodeService(repo, access.NewEligibilityChecker(&fakeEligRepo{}))
	grant, err := svc.Redeem(context.Background(), uuid.New(), uuid.New(), uuid.New(), "SECRET123")
	if err != nil {
		t.Fatalf("valid redemption should succeed: %v", err)
	}
	if grant.ID == uuid.Nil {
		t.Fatal("grant ID should not be nil")
	}
}

func TestRedeem_WrongCode_ReturnsNotFound(t *testing.T) {
	repo := &codeRepo{codes: map[string]db.AccessCode{}, useCounts: map[uuid.UUID]int{}}
	svc := access.NewCodeService(repo, access.NewEligibilityChecker(&fakeEligRepo{}))
	_, err := svc.Redeem(context.Background(), uuid.New(), uuid.New(), uuid.New(), "WRONG")
	if err == nil {
		t.Fatal("wrong code should return error")
	}
}

func TestRedeem_ExpiredCode_ReturnsError(t *testing.T) {
	code := validCode()
	code.ValidUntil = pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true}
	// Adjust ValidFrom to be before ValidUntil to avoid DB constraint (test only uses in-memory)
	code.ValidFrom = pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Hour), Valid: true}
	repo := newCodeRepo(code)
	svc := access.NewCodeService(repo, access.NewEligibilityChecker(&fakeEligRepo{}))
	_, err := svc.Redeem(context.Background(), uuid.New(), uuid.New(), uuid.New(), "SECRET123")
	if err == nil {
		t.Fatal("expired code should return error")
	}
}

func TestRedeem_ExhaustedCode_ReturnsError(t *testing.T) {
	code := validCode()
	code.UseCount = 1 // already at max_uses=1
	repo := newCodeRepo(code)
	svc := access.NewCodeService(repo, access.NewEligibilityChecker(&fakeEligRepo{}))
	_, err := svc.Redeem(context.Background(), uuid.New(), uuid.New(), uuid.New(), "SECRET123")
	if err == nil {
		t.Fatal("exhausted code should return error")
	}
}

func TestRedeem_IncrementRace_ReturnsExhausted(t *testing.T) {
	code := validCode()
	repo := newCodeRepo(code)
	repo.incrementErr = pgx.ErrNoRows // simulate race: increment returns 0 rows
	svc := access.NewCodeService(repo, access.NewEligibilityChecker(&fakeEligRepo{}))
	_, err := svc.Redeem(context.Background(), uuid.New(), uuid.New(), uuid.New(), "SECRET123")
	if err == nil {
		t.Fatal("race on increment should return error")
	}
}
