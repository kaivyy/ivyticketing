package access

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/abuse"
	waitlistmod "github.com/varin/ivyticketing/services/api/internal/modules/waitlist"
	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	codes        *CodeService
	pools        *PoolService
	corporate    *CorporateService
	priority     *PriorityChecker
	waitlistRepo waitlistmod.Repository
	waitlistSvc  *waitlistmod.Service
	guard        *abuse.Guard
}

func NewHandler(codes *CodeService, pools *PoolService, corp *CorporateService) *Handler {
	return &Handler{codes: codes, pools: pools, corporate: corp}
}

// WithPriorityAndWaitlist attaches the priority checker and waitlist dependencies.
func (h *Handler) WithPriorityAndWaitlist(pc *PriorityChecker, wlRepo waitlistmod.Repository, wlSvc *waitlistmod.Service) *Handler {
	h.priority = pc
	h.waitlistRepo = wlRepo
	h.waitlistSvc = wlSvc
	return h
}

// WithGuard attaches the abuse guard used for brute-force tracking and reputation bumps.
func (h *Handler) WithGuard(g *abuse.Guard) *Handler {
	h.guard = g
	return h
}

// clientIP extracts the real client IP from the request, preferring X-Forwarded-For.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// take only the first (leftmost) address
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	// strip port from RemoteAddr
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

func caller(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

// Redeem POST /events/{eventId}/access/redeem
func (h *Handler) Redeem(w http.ResponseWriter, r *http.Request) {
	uid, ok := caller(w, r)
	if !ok {
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	var req RedeemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	categoryID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return
	}
	grant, err := h.codes.Redeem(r.Context(), uid, eventID, categoryID, req.Code)
	if err != nil {
		if errors.Is(err, ErrCodeNotFound) || errors.Is(err, ErrCodeExhausted) {
			if h.guard != nil {
				ip := clientIP(r)
				h.guard.BumpReputation(r.Context(), ip, 2)
				h.guard.TrackCodeFailure(r.Context(), ip)
			}
		}
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, AccessGrantDTO{
		ID:         grant.ID.String(),
		Token:      grant.ID.String(),
		CategoryID: grant.CategoryID.String(),
		ExpiresAt:  grant.ExpiresAt.Time,
	})
}

// MyGrants GET /events/{eventId}/access/my-grants
func (h *Handler) MyGrants(w http.ResponseWriter, r *http.Request) {
	uid, ok := caller(w, r)
	if !ok {
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	grants, err := h.codes.repo.ListActiveGrantsForParticipant(r.Context(), db.ListActiveGrantsForParticipantParams{
		ParticipantID: uid,
		EventID:       eventID,
	})
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, grants)
}

// CreateCode POST /org/{orgId}/events/{eventId}/access/codes
func (h *Handler) CreateCode(w http.ResponseWriter, r *http.Request) {
	uid, ok := caller(w, r)
	if !ok {
		return
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid org id"))
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	var req CreateCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	var catID *uuid.UUID
	if req.CategoryID != nil {
		id, err := uuid.Parse(*req.CategoryID)
		if err != nil {
			apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
			return
		}
		catID = &id
	}
	var poolID *uuid.UUID
	if req.PoolID != nil {
		id, err := uuid.Parse(*req.PoolID)
		if err != nil {
			apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_POOL_ID", "invalid pool id"))
			return
		}
		poolID = &id
	}
	code, err := h.codes.Create(r.Context(), orgID, eventID, catID, req.CodeType, req.Code, req.MaxUses, req.ValidFrom, req.ValidUntil, poolID, uid)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, AccessCodeDTO{
		ID:         code.ID.String(),
		CodeType:   code.CodeType,
		MaxUses:    code.MaxUses,
		UseCount:   code.UseCount,
		ValidFrom:  code.ValidFrom.Time,
		ValidUntil: code.ValidUntil.Time,
	})
}

// ListCodes GET /org/{orgId}/events/{eventId}/access/codes
func (h *Handler) ListCodes(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	limit := int32(50)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = int32(n)
		}
	}
	offset := int32(0)
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	codes, err := h.codes.repo.ListAccessCodesByEvent(r.Context(), db.ListAccessCodesByEventParams{
		EventID: eventID,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, codes)
}

// RevokeCode DELETE /org/{orgId}/access/codes/{codeId}
func (h *Handler) RevokeCode(w http.ResponseWriter, r *http.Request) {
	codeID, err := uuid.Parse(chi.URLParam(r, "codeId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CODE_ID", "invalid code id"))
		return
	}
	if err := h.codes.Revoke(r.Context(), codeID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListPools GET /org/{orgId}/events/{eventId}/access/pools
func (h *Handler) ListPools(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	categoryID, _ := uuid.Parse(r.URL.Query().Get("categoryId"))
	pools, err := h.pools.repo.ListVisiblePoolsByCategory(r.Context(), db.ListVisiblePoolsByCategoryParams{
		EventID:    eventID,
		CategoryID: categoryID,
	})
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, pools)
}

// AdjustPool PUT /org/{orgId}/access/pools/{poolId}
func (h *Handler) AdjustPool(w http.ResponseWriter, r *http.Request) {
	poolID, err := uuid.Parse(chi.URLParam(r, "poolId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_POOL_ID", "invalid pool id"))
		return
	}
	var req AdjustPoolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if req.Delta != 0 {
		if err := h.pools.AdjustTotalSlots(r.Context(), poolID, req.Delta); err != nil {
			apperr.WriteError(w, r, err)
			return
		}
	}
	if req.Visible != nil {
		if err := h.pools.SetVisible(r.Context(), poolID, *req.Visible); err != nil {
			apperr.WriteError(w, r, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// PriorityWindow GET /events/{eventId}/access/priority-window?categoryId=...
// Checks the priority window and auto-issues a grant if eligible, then returns it.
func (h *Handler) PriorityWindow(w http.ResponseWriter, r *http.Request) {
	uid, ok := caller(w, r)
	if !ok {
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	categoryID, err := uuid.Parse(r.URL.Query().Get("categoryId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return
	}
	if h.priority == nil {
		apperr.WriteError(w, r, apperr.New(http.StatusServiceUnavailable, "PRIORITY_NOT_CONFIGURED", "priority checker not configured"))
		return
	}
	if err := h.priority.CheckPriorityAdmission(r.Context(), uid, eventID, categoryID, ""); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	grant, err := h.codes.repo.GetActiveGrantForParticipant(r.Context(), db.GetActiveGrantForParticipantParams{
		ParticipantID: uid,
		CategoryID:    categoryID,
	})
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, AccessGrantDTO{
		ID:         grant.ID.String(),
		Token:      grant.ID.String(),
		CategoryID: grant.CategoryID.String(),
		ExpiresAt:  grant.ExpiresAt.Time,
	})
}

// WaitlistJoin POST /events/{eventId}/categories/{categoryId}/waitlist/join
func (h *Handler) WaitlistJoin(w http.ResponseWriter, r *http.Request) {
	uid, ok := caller(w, r)
	if !ok {
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	categoryID, err := uuid.Parse(chi.URLParam(r, "categoryId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return
	}
	if h.waitlistRepo == nil || h.waitlistSvc == nil {
		apperr.WriteError(w, r, apperr.New(http.StatusServiceUnavailable, "WAITLIST_NOT_CONFIGURED", "waitlist not configured"))
		return
	}
	wl, err := h.waitlistRepo.GetWaitlistByCategory(r.Context(), db.GetWaitlistByCategoryParams{
		EventID:    eventID,
		CategoryID: categoryID,
	})
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "WAITLIST_NOT_FOUND", "no waitlist for this category"))
		return
	}
	entry, err := h.waitlistSvc.Join(r.Context(), wl.ID, uid, "QUOTA_RELEASE", nil)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, map[string]any{
		"waitlistEntryId": entry.ID.String(),
		"rank":            entry.Rank,
	})
}

// WaitlistPosition GET /events/{eventId}/categories/{categoryId}/waitlist/my-position
func (h *Handler) WaitlistPosition(w http.ResponseWriter, r *http.Request) {
	uid, ok := caller(w, r)
	if !ok {
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	categoryID, err := uuid.Parse(chi.URLParam(r, "categoryId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return
	}
	if h.waitlistRepo == nil {
		apperr.WriteError(w, r, apperr.New(http.StatusServiceUnavailable, "WAITLIST_NOT_CONFIGURED", "waitlist not configured"))
		return
	}
	wl, err := h.waitlistRepo.GetWaitlistByCategory(r.Context(), db.GetWaitlistByCategoryParams{
		EventID:    eventID,
		CategoryID: categoryID,
	})
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "WAITLIST_NOT_FOUND", "no waitlist for this category"))
		return
	}
	entry, err := h.waitlistRepo.GetWaitlistEntry(r.Context(), db.GetWaitlistEntryParams{
		WaitlistID:    wl.ID,
		ParticipantID: uid,
	})
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "NOT_ON_WAITLIST", "not on waitlist"))
		return
	}
	position, _ := h.waitlistRepo.CountWaitlistPosition(r.Context(), db.CountWaitlistPositionParams{
		WaitlistID: wl.ID,
		Rank:       entry.Rank,
	})
	apperr.WriteJSON(w, http.StatusOK, map[string]any{
		"position": position + 1,
		"rank":     entry.Rank,
		"status":   entry.Status,
	})
}

// CreateCorporateAccount POST /org/{orgId}/access/corporate
func (h *Handler) CreateCorporateAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid org id"))
		return
	}
	var req CreateCorporateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request"))
		return
	}
	account, err := h.corporate.Create(r.Context(), orgID, req.Name, req.BillingEmail, req.InvoiceRequired, actor.UserID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, CorporateAccountDTO{
		ID:           account.ID.String(),
		Name:         account.Name,
		BillingEmail: account.BillingEmail,
		Status:       account.Status,
	})
}

// ListCorporateAccounts GET /org/{orgId}/access/corporate
func (h *Handler) ListCorporateAccounts(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid org id"))
		return
	}
	accounts, err := h.corporate.repo.ListCorporateAccounts(r.Context(), db.ListCorporateAccountsParams{
		OrganizationID: orgID,
		Limit:          50,
		Offset:         0,
	})
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, accounts)
}

// ApproveCorporateAccount POST /org/{orgId}/access/corporate/{accountId}/approve
func (h *Handler) ApproveCorporateAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return
	}
	accountID, err := uuid.Parse(chi.URLParam(r, "accountId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ACCOUNT_ID", "invalid account id"))
		return
	}
	if err := h.corporate.Approve(r.Context(), accountID, actor.UserID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// BulkUploadMembers POST /org/{orgId}/access/pools/{poolId}/members
func (h *Handler) BulkUploadMembers(w http.ResponseWriter, r *http.Request) {
	poolID, err := uuid.Parse(chi.URLParam(r, "poolId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_POOL_ID", "invalid pool id"))
		return
	}
	actor, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_MULTIPART", "expected multipart form"))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "MISSING_FILE", "file field required"))
		return
	}
	defer file.Close()
	result, err := h.corporate.BulkUploadMembers(r.Context(), poolID, actor.UserID, file)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, BulkUploadResultDTO{Imported: result.Imported, Skipped: result.Skipped})
}

// ListMembers GET /org/{orgId}/access/pools/{poolId}/members
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	poolID, err := uuid.Parse(chi.URLParam(r, "poolId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_POOL_ID", "invalid pool id"))
		return
	}
	members, err := h.corporate.repo.ListPoolMembers(r.Context(), db.ListPoolMembersParams{
		PoolID: poolID,
		Limit:  100,
		Offset: 0,
	})
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, members)
}

// GetInvoice GET /org/{orgId}/access/corporate/{accountId}/invoice?eventId=...&unitPrice=...
func (h *Handler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	accountID, err := uuid.Parse(chi.URLParam(r, "accountId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ACCOUNT_ID", "invalid account id"))
		return
	}
	eventID, _ := uuid.Parse(r.URL.Query().Get("eventId"))
	unitPrice := int64(150000)
	if v := r.URL.Query().Get("unitPrice"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			unitPrice = n
		}
	}
	invoice, err := h.corporate.GenerateInvoice(r.Context(), accountID, eventID, unitPrice)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, invoice)
}

// AdminListCodes GET /admin/access/codes?eventId=...&limit=...&offset=...
func (h *Handler) AdminListCodes(w http.ResponseWriter, r *http.Request) {
	limit := int32(100)
	offset := int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = int32(n)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	eventID, err := uuid.Parse(r.URL.Query().Get("eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	codes, err := h.codes.repo.ListAccessCodesByEvent(r.Context(), db.ListAccessCodesByEventParams{
		EventID: eventID,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, codes)
}

// AdminAdjustQuota POST /admin/access/pools/{poolId}/adjust
func (h *Handler) AdminAdjustQuota(w http.ResponseWriter, r *http.Request) {
	poolID, err := uuid.Parse(chi.URLParam(r, "poolId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_POOL_ID", "invalid pool id"))
		return
	}
	var body struct {
		Delta int `json:"delta"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request"))
		return
	}
	if err := h.pools.AdjustTotalSlots(r.Context(), poolID, body.Delta); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RegisterAdminRoutes mounts admin-only access routes.
// Must be called inside a RequirePlatformAdmin middleware group.
func (h *Handler) RegisterAdminRoutes(r chi.Router) {
	r.Route("/admin/access", func(r chi.Router) {
		r.Get("/codes", h.AdminListCodes)
		r.Post("/pools/{poolId}/adjust", h.AdminAdjustQuota)
	})
}
