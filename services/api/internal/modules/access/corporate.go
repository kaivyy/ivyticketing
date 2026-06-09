package access

import (
	"context"
	"encoding/csv"
	"io"
	"strings"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// BulkUploadResult holds the outcome of a CSV bulk member upload.
type BulkUploadResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

// CorporateService manages corporate accounts and their pool member operations.
type CorporateService struct{ repo Repository }

// NewCorporateService returns a new CorporateService.
func NewCorporateService(repo Repository) *CorporateService {
	return &CorporateService{repo: repo}
}

// Create registers a new corporate account under an organization.
func (s *CorporateService) Create(ctx context.Context, orgID uuid.UUID, name, billingEmail string, invoiceRequired bool, createdBy uuid.UUID) (db.CorporateAccount, error) {
	return s.repo.CreateCorporateAccount(ctx, db.CreateCorporateAccountParams{
		OrganizationID:  orgID,
		Name:            name,
		BillingEmail:    billingEmail,
		InvoiceRequired: invoiceRequired,
		CreatedBy:       createdBy,
	})
}

// Approve marks a pending corporate account as ACTIVE.
func (s *CorporateService) Approve(ctx context.Context, accountID, approvedBy uuid.UUID) error {
	_, err := s.repo.ApproveCorporateAccount(ctx, db.ApproveCorporateAccountParams{
		ID:         accountID,
		ApprovedBy: &approvedBy,
	})
	return err
}

// BulkUploadMembers parses a CSV (header row must include "email") and creates
// AccessPoolMember rows. Deduplicates by email. Rejects the entire upload if
// the unique email count exceeds the pool's available slots.
func (s *CorporateService) BulkUploadMembers(ctx context.Context, poolID, _ uuid.UUID, r io.Reader) (BulkUploadResult, error) {
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		return BulkUploadResult{}, err
	}
	emailIdx := -1
	for i, h := range header {
		if strings.EqualFold(strings.TrimSpace(h), "email") {
			emailIdx = i
			break
		}
	}
	if emailIdx < 0 {
		return BulkUploadResult{}, ErrPoolExhausted // reuse as sentinel for invalid CSV
	}

	seen := map[string]bool{}
	var emails []string
	skipped := 0
	rows, _ := cr.ReadAll()
	for _, row := range rows {
		if emailIdx >= len(row) {
			continue
		}
		email := strings.TrimSpace(strings.ToLower(row[emailIdx]))
		if email == "" {
			continue
		}
		if seen[email] {
			skipped++
			continue
		}
		seen[email] = true
		emails = append(emails, email)
	}

	// Quota check: reject whole upload if it exceeds available pool slots.
	pool, err := s.repo.GetAccessPool(ctx, poolID)
	if err != nil {
		return BulkUploadResult{}, err
	}
	available := int(pool.TotalSlots - pool.ReservedSlots - pool.UsedSlots)
	if len(emails) > available {
		return BulkUploadResult{}, ErrPoolExhausted
	}

	imported := 0
	for _, email := range emails {
		_, err := s.repo.AddPoolMember(ctx, db.AddPoolMemberParams{
			PoolID: poolID,
			Email:  email,
		})
		if err != nil {
			skipped++
			continue
		}
		imported++
	}
	return BulkUploadResult{Imported: imported, Skipped: skipped}, nil
}

// GenerateInvoice builds an invoice summary for a corporate account's pool members.
func (s *CorporateService) GenerateInvoice(ctx context.Context, accountID, _ uuid.UUID, unitPrice int64) (map[string]any, error) {
	account, err := s.repo.GetCorporateAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	members, err := s.repo.ListPoolMembers(ctx, db.ListPoolMembersParams{
		PoolID: uuid.Nil,
		Limit:  10000,
		Offset: 0,
	})
	if err != nil {
		return nil, err
	}
	n := int64(len(members))
	return map[string]any{
		"account": map[string]any{
			"name":          account.Name,
			"billing_email": account.BillingEmail,
		},
		"line_items": []map[string]any{
			{
				"description": "Corporate slots",
				"quantity":    n,
				"unit_price":  unitPrice,
				"total":       n * unitPrice,
			},
		},
		"total":    n * unitPrice,
		"currency": "IDR",
	}, nil
}
