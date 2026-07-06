package whitelabel

// BrandingResponse is the JSON shape of an organization's branding overrides.
type BrandingResponse struct {
	OrganizationID    string `json:"organizationId"`
	LogoObjectKey     string `json:"logoObjectKey"`
	ThemeColor        string `json:"themeColor"`
	EmailFromName     string `json:"emailFromName"`
	EmailFromAddress  string `json:"emailFromAddress"`
	TermsText         string `json:"termsText"`
	FooterText        string `json:"footerText"`
	WhitelabelEnabled bool   `json:"whitelabelEnabled"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

// UpsertBrandingRequest is the organizer payload for setting branding.
type UpsertBrandingRequest struct {
	LogoObjectKey     string `json:"logoObjectKey"`
	ThemeColor        string `json:"themeColor"`
	EmailFromName     string `json:"emailFromName"`
	EmailFromAddress  string `json:"emailFromAddress"`
	TermsText         string `json:"termsText"`
	FooterText        string `json:"footerText"`
	WhitelabelEnabled bool   `json:"whitelabelEnabled"`
}

// DomainResponse is the JSON shape of a custom domain.
type DomainResponse struct {
	ID                string  `json:"id"`
	OrganizationID    string  `json:"organizationId"`
	Domain            string  `json:"domain"`
	VerificationToken string  `json:"verificationToken"`
	VerificationName  string  `json:"verificationName"`
	Status            string  `json:"status"`
	VerifiedAt        *string `json:"verifiedAt"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
}

// AddDomainRequest is the organizer payload for registering a custom domain.
type AddDomainRequest struct {
	Domain string `json:"domain"`
}

// Domain verification status values (mirror the custom_domains status check).
const (
	DomainPending  = "PENDING"
	DomainVerified = "VERIFIED"
	DomainFailed   = "FAILED"
)
