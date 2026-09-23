package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/notifications/email"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

type BroadcastHandler struct {
	queries  *db.Queries
	sender   email.Sender
	auditLog *audit.Logger
	log      *slog.Logger
}

func NewBroadcastHandler(queries *db.Queries, sender email.Sender, auditLog *audit.Logger, log *slog.Logger) *BroadcastHandler {
	return &BroadcastHandler{
		queries:  queries,
		sender:   sender,
		auditLog: auditLog,
		log:      log,
	}
}

func (h *BroadcastHandler) RegisterOrgRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.With(middleware.RequirePermission(loader, "broadcast.send")).Post("/broadcast", h.SendBroadcast)
	r.With(middleware.RequirePermission(loader, "broadcast.send")).Get("/broadcast/preview", h.PreviewBroadcast)
}

func (h *BroadcastHandler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.With(middleware.RequirePermission(loader, "broadcast.send")).Post("/broadcast", h.SendEventBroadcast)
	r.With(middleware.RequirePermission(loader, "broadcast.send")).Get("/broadcast/preview", h.PreviewEventBroadcast)
}

type sendBroadcastPayload struct {
	EventID    uuid.UUID  `json:"eventId"`
	CategoryID *uuid.UUID `json:"categoryId,omitempty"`
	Subject    string     `json:"subject"`
	Message    string     `json:"message"`
}

func (h *BroadcastHandler) PreviewBroadcast(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return
	}

	eventIDStr := r.URL.Query().Get("eventId")
	if eventIDStr == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "EVENT_ID_REQUIRED", "eventId query parameter is required"))
		return
	}
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}

	var categoryID *uuid.UUID
	if catStr := r.URL.Query().Get("categoryId"); catStr != "" {
		if cID, err := uuid.Parse(catStr); err == nil {
			categoryID = &cID
		}
	}

	recipients, err := h.queries.ListTicketsForBroadcast(r.Context(), db.ListTicketsForBroadcastParams{
		OrganizationID: orgID,
		EventID:        eventID,
		CategoryID:     categoryID,
	})
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}

	apperr.WriteJSON(w, http.StatusOK, map[string]any{
		"recipientCount": len(recipients),
		"eventId":        eventID,
		"categoryId":     categoryID,
	})
}

func (h *BroadcastHandler) PreviewEventBroadcast(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}

	var categoryID *uuid.UUID
	if catStr := r.URL.Query().Get("categoryId"); catStr != "" {
		if cID, err := uuid.Parse(catStr); err == nil {
			categoryID = &cID
		}
	}

	recipients, err := h.queries.ListTicketsForBroadcast(r.Context(), db.ListTicketsForBroadcastParams{
		OrganizationID: orgID,
		EventID:        eventID,
		CategoryID:     categoryID,
	})
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}

	apperr.WriteJSON(w, http.StatusOK, map[string]any{
		"recipientCount": len(recipients),
		"eventId":        eventID,
		"categoryId":     categoryID,
	})
}

func (h *BroadcastHandler) SendEventBroadcast(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}

	var payload sendBroadcastPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	payload.EventID = eventID
	h.executeBroadcast(w, r, payload)
}

func (h *BroadcastHandler) SendBroadcast(w http.ResponseWriter, r *http.Request) {
	var payload sendBroadcastPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if payload.EventID == uuid.Nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "EVENT_ID_REQUIRED", "eventId is required"))
		return
	}
	h.executeBroadcast(w, r, payload)
}

func (h *BroadcastHandler) executeBroadcast(w http.ResponseWriter, r *http.Request, payload sendBroadcastPayload) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return
	}

	subject := strings.TrimSpace(payload.Subject)
	message := strings.TrimSpace(payload.Message)
	if subject == "" || message == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "FIELDS_REQUIRED", "subject and message cannot be empty"))
		return
	}

	recipients, err := h.queries.ListTicketsForBroadcast(r.Context(), db.ListTicketsForBroadcastParams{
		OrganizationID: orgID,
		EventID:        payload.EventID,
		CategoryID:     payload.CategoryID,
	})
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}

	id, _ := authctx.FromContext(r.Context())

	// Dispatch emails asynchronously in background
	go func(items []db.ListTicketsForBroadcastRow, subj, msg string) {
		bgCtx := context.Background()
		escapedMsg := html.EscapeString(msg)
		escapedMsg = strings.ReplaceAll(escapedMsg, "\n", "<br/>")

		for _, item := range items {
			if item.HolderEmail == "" {
				continue
			}
			htmlBody := fmt.Sprintf(`<div style="font-family:sans-serif;max-width:600px;margin:0 auto;padding:24px;color:#1e293b;">
				<h2 style="color:#0f172a;margin-bottom:16px;">%s</h2>
				<p style="color:#475569;font-size:14px;margin-bottom:16px;">Halo <strong>%s</strong>,</p>
				<div style="background-color:#f8fafc;border-left:4px solid #f97316;padding:16px;margin-bottom:20px;border-radius:4px;font-size:14px;line-height:1.6;color:#334155;">
					%s
				</div>
				<p style="font-size:12px;color:#94a3b8;border-top:1px solid #e2e8f0;padding-top:12px;margin-top:24px;">Pesan ini dikirim oleh penyelenggara event melalui platform IvyTicketing.</p>
			</div>`, html.EscapeString(subj), html.EscapeString(item.HolderName), escapedMsg)

			if sendErr := h.sender.Send(bgCtx, item.HolderEmail, subj, htmlBody, msg); sendErr != nil {
				h.log.Warn("broadcast email delivery failed", "to", item.HolderEmail, "err", sendErr)
			}
		}
	}(recipients, subject, message)

	// Record audit entry
	if h.auditLog != nil {
		h.auditLog.Record(r.Context(), audit.Entry{
			OrganizationID: &orgID,
			ActorUserID:    &id.UserID,
			Action:         "BROADCAST_SENT",
			TargetType:     "event",
			TargetID:       payload.EventID.String(),
			Metadata: map[string]any{
				"subject":         subject,
				"recipient_count": len(recipients),
				"category_id":     payload.CategoryID,
			},
		})
	}

	apperr.WriteJSON(w, http.StatusOK, map[string]any{
		"status":         "sent",
		"recipientCount": len(recipients),
		"eventId":        payload.EventID,
		"subject":        subject,
	})
}
