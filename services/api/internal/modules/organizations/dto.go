package organizations

import (
	"time"

	"github.com/google/uuid"
)

type CreateRequest struct {
	Name string `json:"name"`
}

type Response struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"createdAt"`
}

type AuditLogResponse struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID *uuid.UUID     `json:"organizationId,omitempty"`
	ActorUserID    *uuid.UUID     `json:"actorUserId,omitempty"`
	ActorEmail     string         `json:"actorEmail,omitempty"`
	ActorName      string         `json:"actorName,omitempty"`
	Action         string         `json:"action"`
	TargetType     string         `json:"targetType,omitempty"`
	TargetID       string         `json:"targetId,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
}

type AdminCreateOrganizerRequest struct {
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	LeadFullName string `json:"leadFullName"`
	LeadEmail    string `json:"leadEmail"`
	LeadPassword string `json:"leadPassword"`
	LeadPhone    string `json:"leadPhone"`
}

type OrganizationWithStatsResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	CreatedAt   time.Time `json:"createdAt"`
	EventCount  int64     `json:"eventCount"`
	MemberCount int64     `json:"memberCount"`
	LeadEmail   string    `json:"leadEmail"`
	LeadName    string    `json:"leadName"`
}

