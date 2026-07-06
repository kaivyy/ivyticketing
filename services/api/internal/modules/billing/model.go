package billing

// Package slugs — the three masterplan tiers.
const (
	PackageStarter      = "starter"
	PackageProfessional = "professional"
	PackageEnterprise   = "enterprise"
)

// Subscription statuses.
const (
	SubActive    = "ACTIVE"
	SubCancelled = "CANCELLED"
	SubExpired   = "EXPIRED"
)

// Invoice statuses.
const (
	InvoiceDraft  = "DRAFT"
	InvoiceIssued = "ISSUED"
	InvoicePaid   = "PAID"
	InvoiceVoid   = "VOID"
)

// Feature keys. A package's `features` jsonb array holds a subset of these; the
// PackageGate consults them to allow/deny gated capabilities.
const (
	FeatureBasicRegistration = "basic_registration"
	FeatureBasicPayment      = "basic_payment"
	FeatureQueue             = "queue"
	FeatureBallot            = "ballot"
	FeatureRacepack          = "racepack"
	FeatureCustomBranding    = "custom_branding"
	FeatureWhitelabel        = "whitelabel"
	FeatureCustomDomain      = "custom_domain"
	FeatureCustomPayment     = "custom_payment"
	FeatureDedicatedQueue    = "dedicated_queue"
	FeatureAPI               = "api"
)

// PackageResponse is the API view of a subscription package.
type PackageResponse struct {
	ID           string   `json:"id"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	PriceMonthly int64    `json:"priceMonthly"`
	MaxEvents    *int32   `json:"maxEvents"`
	FeeBps       int32    `json:"feeBps"`
	Features     []string `json:"features"`
	IsActive     bool     `json:"isActive"`
	SortOrder    int32    `json:"sortOrder"`
}

// SubscriptionResponse is the API view of an org's subscription + its package.
type SubscriptionResponse struct {
	ID             string           `json:"id"`
	OrganizationID string           `json:"organizationId"`
	Status         string           `json:"status"`
	StartedAt      string           `json:"startedAt"`
	ExpiresAt      *string          `json:"expiresAt"`
	Package        PackageResponse  `json:"package"`
}

// InvoiceResponse is the API view of a platform invoice.
type InvoiceResponse struct {
	ID                 string  `json:"id"`
	OrganizationID     string  `json:"organizationId"`
	InvoiceNumber      string  `json:"invoiceNumber"`
	PeriodStart        string  `json:"periodStart"`
	PeriodEnd          string  `json:"periodEnd"`
	SubscriptionAmount int64   `json:"subscriptionAmount"`
	FeeAmount          int64   `json:"feeAmount"`
	TotalAmount        int64   `json:"totalAmount"`
	Status             string  `json:"status"`
	IssuedAt           *string `json:"issuedAt"`
	PaidAt             *string `json:"paidAt"`
	CreatedAt          string  `json:"createdAt"`
}

// FeeSummaryResponse aggregates an org's platform-fee ledger.
type FeeSummaryResponse struct {
	Entries     int64 `json:"entries"`
	GrossOrders int64 `json:"grossOrders"`
	TotalFees   int64 `json:"totalFees"`
}

// RevenueRow is one org's contribution to platform revenue (super-admin view).
type RevenueRow struct {
	OrganizationID   string `json:"organizationId"`
	OrganizationName string `json:"organizationName"`
	TotalFees        int64  `json:"totalFees"`
	GrossOrders      int64  `json:"grossOrders"`
	FeeEntries       int64  `json:"feeEntries"`
}

// --- request bodies ---

// UpsertPackageRequest creates or updates a subscription package (super-admin).
type UpsertPackageRequest struct {
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	PriceMonthly int64    `json:"priceMonthly"`
	MaxEvents    *int32   `json:"maxEvents"`
	FeeBps       int32    `json:"feeBps"`
	Features     []string `json:"features"`
	IsActive     bool     `json:"isActive"`
	SortOrder    int32    `json:"sortOrder"`
}

// AssignSubscriptionRequest assigns/upgrades an org to a package (super-admin).
type AssignSubscriptionRequest struct {
	PackageID string  `json:"packageId"`
	ExpiresAt *string `json:"expiresAt"` // RFC3339, nil = no expiry
}

// GenerateInvoiceRequest generates a platform invoice for an org (super-admin).
type GenerateInvoiceRequest struct {
	PeriodStart        string `json:"periodStart"` // YYYY-MM-DD
	PeriodEnd          string `json:"periodEnd"`   // YYYY-MM-DD
	SubscriptionAmount int64  `json:"subscriptionAmount"`
	FeeAmount          int64  `json:"feeAmount"`
}
