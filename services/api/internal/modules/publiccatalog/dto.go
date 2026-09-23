package publiccatalog

import (
	"time"

	"github.com/google/uuid"
)

type EventResponse struct {
	ID               uuid.UUID          `json:"id"`
	Name             string             `json:"name"`
	Slug             string             `json:"slug"`
	EventType        string             `json:"eventType"`
	Description      string             `json:"description"`
	BannerURL        string             `json:"bannerUrl"`
	LogoURL          string             `json:"logoUrl"`
	VenueName        string             `json:"venueName"`
	VenueAddress     string             `json:"venueAddress"`
	StartsAt         *time.Time         `json:"startsAt"`
	EndsAt           *time.Time         `json:"endsAt"`
	OrganizationName string             `json:"organizerName,omitempty"`
	OrganizationSlug string             `json:"organizerSlug,omitempty"`
	Status           string             `json:"status"`
	Categories       []CategoryResponse `json:"categories,omitempty"`
}

type CategoryResponse struct {
	ID                   uuid.UUID `json:"id"`
	Name                 string    `json:"name"`
	Price                int64     `json:"price"`
	Capacity             int32     `json:"capacity"`
	DistanceKm           *float64  `json:"distanceKm,omitempty"`
	CutoffTime           string    `json:"cutoffTime,omitempty"`
	RegistrationOpensAt  time.Time `json:"registrationOpensAt"`
	RegistrationClosesAt time.Time `json:"registrationClosesAt"`
	RegistrationMode     string    `json:"registrationMode"`
}
