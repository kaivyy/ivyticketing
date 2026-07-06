package whitelabel_test

import (
	"context"
	"errors"
	"log/slog"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/whitelabel"
)

// --- fakes ---

type fakeRepo struct {
	branding    map[uuid.UUID]db.OrgBranding
	domains     map[uuid.UUID]db.CustomDomain
	byName      map[string]db.CustomDomain
	createErr   error
	upsertErr   error
	deletedID   uuid.UUID
	verifiedID  uuid.UUID
	failedID    uuid.UUID
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		branding: map[uuid.UUID]db.OrgBranding{},
		domains:  map[uuid.UUID]db.CustomDomain{},
		byName:   map[string]db.CustomDomain{},
	}
}

func (r *fakeRepo) GetBranding(ctx context.Context, orgID uuid.UUID) (db.OrgBranding, error) {
	b, ok := r.branding[orgID]
	if !ok {
		return db.OrgBranding{}, pgx.ErrNoRows
	}
	return b, nil
}

func (r *fakeRepo) UpsertBranding(ctx context.Context, arg db.UpsertOrgBrandingParams) (db.OrgBranding, error) {
	if r.upsertErr != nil {
		return db.OrgBranding{}, r.upsertErr
	}
	b := db.OrgBranding{
		OrganizationID:    arg.OrganizationID,
		LogoObjectKey:     arg.LogoObjectKey,
		ThemeColor:        arg.ThemeColor,
		EmailFromName:     arg.EmailFromName,
		EmailFromAddress:  arg.EmailFromAddress,
		TermsText:         arg.TermsText,
		FooterText:        arg.FooterText,
		WhitelabelEnabled: arg.WhitelabelEnabled,
	}
	r.branding[arg.OrganizationID] = b
	return b, nil
}

func (r *fakeRepo) ListDomains(ctx context.Context, orgID uuid.UUID) ([]db.CustomDomain, error) {
	var out []db.CustomDomain
	for _, d := range r.domains {
		if d.OrganizationID == orgID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (r *fakeRepo) GetDomain(ctx context.Context, id uuid.UUID) (db.CustomDomain, error) {
	d, ok := r.domains[id]
	if !ok {
		return db.CustomDomain{}, pgx.ErrNoRows
	}
	return d, nil
}

func (r *fakeRepo) GetDomainByName(ctx context.Context, domain string) (db.CustomDomain, error) {
	d, ok := r.byName[domain]
	if !ok {
		return db.CustomDomain{}, pgx.ErrNoRows
	}
	return d, nil
}

func (r *fakeRepo) CreateDomain(ctx context.Context, arg db.CreateCustomDomainParams) (db.CustomDomain, error) {
	if r.createErr != nil {
		return db.CustomDomain{}, r.createErr
	}
	d := db.CustomDomain{
		ID:                uuid.New(),
		OrganizationID:    arg.OrganizationID,
		Domain:            arg.Domain,
		VerificationToken: arg.VerificationToken,
		Status:            whitelabel.DomainPending,
	}
	r.domains[d.ID] = d
	r.byName[d.Domain] = d
	return d, nil
}

func (r *fakeRepo) MarkDomainVerified(ctx context.Context, id uuid.UUID) (db.CustomDomain, error) {
	r.verifiedID = id
	d := r.domains[id]
	d.Status = whitelabel.DomainVerified
	r.domains[id] = d
	return d, nil
}

func (r *fakeRepo) MarkDomainFailed(ctx context.Context, id uuid.UUID) (db.CustomDomain, error) {
	r.failedID = id
	d := r.domains[id]
	d.Status = whitelabel.DomainFailed
	r.domains[id] = d
	return d, nil
}

func (r *fakeRepo) DeleteDomain(ctx context.Context, arg db.DeleteCustomDomainParams) error {
	r.deletedID = arg.ID
	delete(r.domains, arg.ID)
	return nil
}

type fakeResolver struct {
	txts map[string][]string
	err  error
}

func (f fakeResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.txts[name], nil
}

func newSvc(r whitelabel.Repository) *whitelabel.Service {
	return whitelabel.NewService(r, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// --- branding ---

func TestGetBranding_DefaultsWhenUnset(t *testing.T) {
	svc := newSvc(newFakeRepo())
	org := uuid.New()
	b, err := svc.GetBranding(context.Background(), org)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if b.ThemeColor != "#2563eb" {
		t.Fatalf("expected default theme color, got %q", b.ThemeColor)
	}
	if b.OrganizationID != org.String() {
		t.Fatalf("org id mismatch")
	}
}

func TestUpsertBranding_RejectsBadColor(t *testing.T) {
	svc := newSvc(newFakeRepo())
	_, err := svc.UpsertBranding(context.Background(), uuid.New(), uuid.New(), whitelabel.UpsertBrandingRequest{ThemeColor: "blue"})
	if !errors.Is(err, whitelabel.ErrInvalidBranding) {
		t.Fatalf("expected ErrInvalidBranding, got %v", err)
	}
}

func TestUpsertBranding_DefaultsEmptyColor(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	org := uuid.New()
	b, err := svc.UpsertBranding(context.Background(), uuid.New(), org, whitelabel.UpsertBrandingRequest{ThemeColor: ""})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if b.ThemeColor != "#2563eb" {
		t.Fatalf("expected default color, got %q", b.ThemeColor)
	}
}

func TestUpsertBranding_RoundTrips(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	org := uuid.New()
	b, err := svc.UpsertBranding(context.Background(), uuid.New(), org, whitelabel.UpsertBrandingRequest{
		ThemeColor:        "#Aa11FF",
		EmailFromName:     " Acme ",
		WhitelabelEnabled: true,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if b.ThemeColor != "#Aa11FF" {
		t.Fatalf("color mismatch: %q", b.ThemeColor)
	}
	if b.EmailFromName != "Acme" {
		t.Fatalf("expected trimmed name, got %q", b.EmailFromName)
	}
	if !b.WhitelabelEnabled {
		t.Fatalf("expected whitelabel enabled")
	}
}

// --- domains ---

func TestAddDomain_RejectsInvalid(t *testing.T) {
	svc := newSvc(newFakeRepo())
	for _, bad := range []string{"", "nodot", "-bad.com", "spaces here.com"} {
		if _, err := svc.AddDomain(context.Background(), uuid.New(), uuid.New(), whitelabel.AddDomainRequest{Domain: bad}); !errors.Is(err, whitelabel.ErrInvalidDomain) {
			t.Fatalf("expected ErrInvalidDomain for %q, got %v", bad, err)
		}
	}
}

func TestAddDomain_RejectsTaken(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	org := uuid.New()
	if _, err := svc.AddDomain(context.Background(), uuid.New(), org, whitelabel.AddDomainRequest{Domain: "events.acme.com"}); err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if _, err := svc.AddDomain(context.Background(), uuid.New(), org, whitelabel.AddDomainRequest{Domain: "events.acme.com"}); !errors.Is(err, whitelabel.ErrDomainTaken) {
		t.Fatalf("expected ErrDomainTaken, got %v", err)
	}
}

func TestAddDomain_NormalizesAndIssuesToken(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	d, err := svc.AddDomain(context.Background(), uuid.New(), uuid.New(), whitelabel.AddDomainRequest{Domain: "  Events.Acme.COM.  "})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if d.Domain != "events.acme.com" {
		t.Fatalf("expected normalized domain, got %q", d.Domain)
	}
	if d.VerificationToken == "" {
		t.Fatalf("expected a verification token")
	}
	if d.Status != whitelabel.DomainPending {
		t.Fatalf("expected PENDING, got %q", d.Status)
	}
	if d.VerificationName != "_ivyticketing.events.acme.com" {
		t.Fatalf("unexpected verification name: %q", d.VerificationName)
	}
}

func TestVerifyDomain_SuccessOnMatch(t *testing.T) {
	repo := newFakeRepo()
	org := uuid.New()
	svc := newSvc(repo)
	d, _ := svc.AddDomain(context.Background(), uuid.New(), org, whitelabel.AddDomainRequest{Domain: "events.acme.com"})
	id := uuid.MustParse(d.ID)
	svc = svc.WithResolver(fakeResolver{txts: map[string][]string{
		"_ivyticketing.events.acme.com": {"other", d.VerificationToken},
	}})
	out, err := svc.VerifyDomain(context.Background(), uuid.New(), org, id)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != whitelabel.DomainVerified {
		t.Fatalf("expected VERIFIED, got %q", out.Status)
	}
	if repo.verifiedID != id {
		t.Fatalf("expected MarkDomainVerified called")
	}
}

func TestVerifyDomain_MismatchMarksFailed(t *testing.T) {
	repo := newFakeRepo()
	org := uuid.New()
	svc := newSvc(repo)
	d, _ := svc.AddDomain(context.Background(), uuid.New(), org, whitelabel.AddDomainRequest{Domain: "events.acme.com"})
	id := uuid.MustParse(d.ID)
	svc = svc.WithResolver(fakeResolver{txts: map[string][]string{
		"_ivyticketing.events.acme.com": {"wrong-token"},
	}})
	if _, err := svc.VerifyDomain(context.Background(), uuid.New(), org, id); !errors.Is(err, whitelabel.ErrDNSMismatch) {
		t.Fatalf("expected ErrDNSMismatch, got %v", err)
	}
	if repo.failedID != id {
		t.Fatalf("expected MarkDomainFailed called")
	}
}

func TestVerifyDomain_WrongOrgNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	d, _ := svc.AddDomain(context.Background(), uuid.New(), uuid.New(), whitelabel.AddDomainRequest{Domain: "events.acme.com"})
	id := uuid.MustParse(d.ID)
	if _, err := svc.VerifyDomain(context.Background(), uuid.New(), uuid.New(), id); !errors.Is(err, whitelabel.ErrDomainNotFound) {
		t.Fatalf("expected ErrDomainNotFound for wrong org, got %v", err)
	}
}

func TestVerifyDomain_NotFound(t *testing.T) {
	svc := newSvc(newFakeRepo())
	if _, err := svc.VerifyDomain(context.Background(), uuid.New(), uuid.New(), uuid.New()); !errors.Is(err, whitelabel.ErrDomainNotFound) {
		t.Fatalf("expected ErrDomainNotFound, got %v", err)
	}
}

func TestDeleteDomain(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	org := uuid.New()
	d, _ := svc.AddDomain(context.Background(), uuid.New(), org, whitelabel.AddDomainRequest{Domain: "events.acme.com"})
	id := uuid.MustParse(d.ID)
	if err := svc.DeleteDomain(context.Background(), uuid.New(), org, id); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.deletedID != id {
		t.Fatalf("expected DeleteDomain called with id")
	}
}
