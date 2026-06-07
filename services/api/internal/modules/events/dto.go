package events

import (
	"time"

	"github.com/google/uuid"
)

type CreateRequest struct {
	Name         string     `json:"name"`
	EventType    string     `json:"eventType"`
	Description  string     `json:"description"`
	VenueName    string     `json:"venueName"`
	VenueAddress string     `json:"venueAddress"`
	StartsAt     *time.Time `json:"startsAt"`
	EndsAt       *time.Time `json:"endsAt"`
	FAQ          string     `json:"faq"`
	Terms        string     `json:"terms"`
	Waiver       string     `json:"waiver"`
}

type UpdateRequest struct {
	Name         string     `json:"name"`
	EventType    string     `json:"eventType"`
	Description  string     `json:"description"`
	VenueName    string     `json:"venueName"`
	VenueAddress string     `json:"venueAddress"`
	StartsAt     *time.Time `json:"startsAt"`
	EndsAt       *time.Time `json:"endsAt"`
	FAQ          string     `json:"faq"`
	Terms        string     `json:"terms"`
	Waiver       string     `json:"waiver"`
}

type Response struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	Slug         string     `json:"slug"`
	EventType    string     `json:"eventType"`
	Status       string     `json:"status"`
	Description  string     `json:"description"`
	BannerURL    string     `json:"bannerUrl"`
	LogoURL      string     `json:"logoUrl"`
	VenueName    string     `json:"venueName"`
	VenueAddress string     `json:"venueAddress"`
	StartsAt     *time.Time `json:"startsAt"`
	EndsAt       *time.Time `json:"endsAt"`
	FAQ          string     `json:"faq"`
	Terms        string     `json:"terms"`
	Waiver       string     `json:"waiver"`
	PublishedAt  *time.Time `json:"publishedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
}
