package whitelabel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

// verificationHost is the DNS label organizers add a TXT record under to prove
// domain ownership: TXT _ivyticketing.<domain> = <verification_token>.
const verificationHost = "_ivyticketing"

var (
	themeColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	// A conservative domain matcher: labels of alphanumerics/hyphens joined by
	// dots, at least one dot, no leading/trailing hyphen per label.
	domainRe = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
)

// Resolver looks up DNS TXT records. Abstracted so tests can inject a fake.
type Resolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

type netResolver struct{}

func (netResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return net.DefaultResolver.LookupTXT(ctx, name)
}

// Service coordinates per-org branding overrides and custom-domain verification.
type Service struct {
	repo     Repository
	resolver Resolver
	audit    *audit.Logger
	log      *slog.Logger
}

// NewService constructs a white-label Service using the system DNS resolver.
func NewService(repo Repository, auditLog *audit.Logger, log *slog.Logger) *Service {
	return &Service{repo: repo, resolver: netResolver{}, audit: auditLog, log: log}
}

// WithResolver overrides the DNS resolver (used in tests).
func (s *Service) WithResolver(r Resolver) *Service {
	s.resolver = r
	return s
}

// --- branding ---

// GetBranding returns an org's branding, falling back to defaults when unset.
func (s *Service) GetBranding(ctx context.Context, orgID uuid.UUID) (BrandingResponse, error) {
	b, err := s.repo.GetBranding(ctx, orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return defaultBranding(orgID), nil
		}
		return BrandingResponse{}, err
	}
	return toBrandingResponse(b), nil
}

// UpsertBranding sets an org's branding overrides.
func (s *Service) UpsertBranding(ctx context.Context, actor, orgID uuid.UUID, req UpsertBrandingRequest) (BrandingResponse, error) {
	color := strings.TrimSpace(req.ThemeColor)
	if color == "" {
		color = "#2563eb"
	}
	if !themeColorRe.MatchString(color) {
		return BrandingResponse{}, ErrInvalidBranding
	}
	b, err := s.repo.UpsertBranding(ctx, db.UpsertOrgBrandingParams{
		OrganizationID:    orgID,
		LogoObjectKey:     strings.TrimSpace(req.LogoObjectKey),
		ThemeColor:        color,
		EmailFromName:     strings.TrimSpace(req.EmailFromName),
		EmailFromAddress:  strings.TrimSpace(req.EmailFromAddress),
		TermsText:         req.TermsText,
		FooterText:        req.FooterText,
		WhitelabelEnabled: req.WhitelabelEnabled,
	})
	if err != nil {
		return BrandingResponse{}, err
	}
	s.record(ctx, actor, "whitelabel.branding_updated", "org_branding", orgID.String(), map[string]any{"whitelabel_enabled": req.WhitelabelEnabled})
	return toBrandingResponse(b), nil
}

// --- custom domains ---

// ListDomains returns an org's custom domains, newest first.
func (s *Service) ListDomains(ctx context.Context, orgID uuid.UUID) ([]DomainResponse, error) {
	rows, err := s.repo.ListDomains(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]DomainResponse, 0, len(rows))
	for _, d := range rows {
		out = append(out, toDomainResponse(d))
	}
	return out, nil
}

// AddDomain registers a custom domain for an org and issues a verification token.
func (s *Service) AddDomain(ctx context.Context, actor, orgID uuid.UUID, req AddDomainRequest) (DomainResponse, error) {
	domain := strings.ToLower(strings.TrimSpace(req.Domain))
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" || len(domain) > 253 || !domainRe.MatchString(domain) {
		return DomainResponse{}, ErrInvalidDomain
	}
	if _, err := s.repo.GetDomainByName(ctx, domain); err == nil {
		return DomainResponse{}, ErrDomainTaken
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return DomainResponse{}, err
	}
	d, err := s.repo.CreateDomain(ctx, db.CreateCustomDomainParams{
		OrganizationID:    orgID,
		Domain:            domain,
		VerificationToken: newToken(),
	})
	if err != nil {
		return DomainResponse{}, err
	}
	s.record(ctx, actor, "whitelabel.domain_added", "custom_domain", d.ID.String(), map[string]any{"domain": domain})
	return toDomainResponse(d), nil
}

// VerifyDomain checks the DNS TXT record for the domain's verification token and
// transitions the domain to VERIFIED or FAILED. The domain must belong to orgID.
func (s *Service) VerifyDomain(ctx context.Context, actor, orgID, domainID uuid.UUID) (DomainResponse, error) {
	d, err := s.repo.GetDomain(ctx, domainID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DomainResponse{}, ErrDomainNotFound
		}
		return DomainResponse{}, err
	}
	if d.OrganizationID != orgID {
		return DomainResponse{}, ErrDomainNotFound
	}

	txts, lookupErr := s.resolver.LookupTXT(ctx, verificationHost+"."+d.Domain)
	matched := false
	for _, txt := range txts {
		if strings.TrimSpace(txt) == d.VerificationToken {
			matched = true
			break
		}
	}

	if !matched {
		if _, err := s.repo.MarkDomainFailed(ctx, domainID); err != nil {
			return DomainResponse{}, err
		}
		s.record(ctx, actor, "whitelabel.domain_verify_failed", "custom_domain", domainID.String(), map[string]any{"domain": d.Domain})
		if lookupErr != nil {
			s.log.Info("custom domain TXT lookup failed", "domain", d.Domain, "err", lookupErr)
		}
		return DomainResponse{}, ErrDNSMismatch
	}

	updated, err := s.repo.MarkDomainVerified(ctx, domainID)
	if err != nil {
		return DomainResponse{}, err
	}
	s.record(ctx, actor, "whitelabel.domain_verified", "custom_domain", domainID.String(), map[string]any{"domain": d.Domain})
	return toDomainResponse(updated), nil
}

// DeleteDomain removes a custom domain owned by orgID.
func (s *Service) DeleteDomain(ctx context.Context, actor, orgID, domainID uuid.UUID) error {
	if err := s.repo.DeleteDomain(ctx, db.DeleteCustomDomainParams{ID: domainID, OrganizationID: orgID}); err != nil {
		return err
	}
	s.record(ctx, actor, "whitelabel.domain_deleted", "custom_domain", domainID.String(), nil)
	return nil
}

// --- helpers ---

func (s *Service) record(ctx context.Context, actor uuid.UUID, action, targetType, targetID string, meta map[string]any) {
	if s.audit == nil {
		return
	}
	a := actor
	s.audit.Record(ctx, audit.Entry{
		ActorUserID: &a,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Metadata:    meta,
	})
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "ivy-verify=" + hex.EncodeToString(b)
}

func defaultBranding(orgID uuid.UUID) BrandingResponse {
	return BrandingResponse{
		OrganizationID: orgID.String(),
		ThemeColor:     "#2563eb",
	}
}

func toBrandingResponse(b db.OrgBranding) BrandingResponse {
	return BrandingResponse{
		OrganizationID:    b.OrganizationID.String(),
		LogoObjectKey:     b.LogoObjectKey,
		ThemeColor:        b.ThemeColor,
		EmailFromName:     b.EmailFromName,
		EmailFromAddress:  b.EmailFromAddress,
		TermsText:         b.TermsText,
		FooterText:        b.FooterText,
		WhitelabelEnabled: b.WhitelabelEnabled,
		CreatedAt:         tsStr(b.CreatedAt),
		UpdatedAt:         tsStr(b.UpdatedAt),
	}
}

func toDomainResponse(d db.CustomDomain) DomainResponse {
	return DomainResponse{
		ID:                d.ID.String(),
		OrganizationID:    d.OrganizationID.String(),
		Domain:            d.Domain,
		VerificationToken: d.VerificationToken,
		VerificationName:  verificationHost + "." + d.Domain,
		Status:            d.Status,
		VerifiedAt:        tsPtr(d.VerifiedAt),
		CreatedAt:         tsStr(d.CreatedAt),
		UpdatedAt:         tsStr(d.UpdatedAt),
	}
}
