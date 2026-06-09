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
	Delta   int  `json:"delta"`
	Visible *bool `json:"visible,omitempty"`
}
