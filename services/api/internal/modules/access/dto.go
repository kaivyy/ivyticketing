package access

import "time"

type RedeemRequest struct {
	Code       string `json:"code"`
	CategoryID string `json:"categoryId"`
}

type AccessGrantDTO struct {
	ID         string    `json:"id"`
	Token      string    `json:"token"` // same as ID — used as admissionToken at checkout
	CategoryID string    `json:"categoryId"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type CreateCodeRequest struct {
	CodeType   string    `json:"codeType"`
	Code       string    `json:"code"`
	MaxUses    int32     `json:"maxUses"`
	ValidFrom  time.Time `json:"validFrom"`
	ValidUntil time.Time `json:"validUntil"`
	CategoryID *string   `json:"categoryId,omitempty"`
	PoolID     *string   `json:"poolId,omitempty"`
}

type AccessCodeDTO struct {
	ID         string    `json:"id"`
	CodeType   string    `json:"codeType"`
	MaxUses    int32     `json:"maxUses"`
	UseCount   int32     `json:"useCount"`
	ValidFrom  time.Time `json:"validFrom"`
	ValidUntil time.Time `json:"validUntil"`
}

type AdjustPoolRequest struct {
	Delta   int   `json:"delta"`
	Visible *bool `json:"visible,omitempty"`
}

type CreateCorporateAccountRequest struct {
	Name            string `json:"name"`
	BillingEmail    string `json:"billingEmail"`
	InvoiceRequired bool   `json:"invoiceRequired"`
}

type CorporateAccountDTO struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	BillingEmail string  `json:"billingEmail"`
	Status       string  `json:"status"`
	ApprovedAt   *string `json:"approvedAt,omitempty"`
}

type BulkUploadResultDTO struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

type MemberDTO struct {
	ID           string  `json:"id"`
	Email        string  `json:"email"`
	MemberStatus string  `json:"memberStatus"`
	RegisteredAt *string `json:"registeredAt,omitempty"`
}
