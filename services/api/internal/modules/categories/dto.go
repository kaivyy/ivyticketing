package categories

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type WriteRequest struct {
	Name                 string          `json:"name"`
	Price                int64           `json:"price"`
	Capacity             int32           `json:"capacity"`
	RegistrationOpensAt  time.Time       `json:"registrationOpensAt"`
	RegistrationClosesAt time.Time       `json:"registrationClosesAt"`
	BibPrefix            string          `json:"bibPrefix"`
	MinAge               *int32          `json:"minAge"`
	MaxOrderPerUser      int32           `json:"maxOrderPerUser"`
	DistanceKm           *float64        `json:"distanceKm,omitempty"`
	CutoffTime           string          `json:"cutoffTime,omitempty"`
	PricingTiers         json.RawMessage `json:"pricingTiers,omitempty"`
}

type Response struct {
	ID                   uuid.UUID       `json:"id"`
	EventID              uuid.UUID       `json:"eventId"`
	Name                 string          `json:"name"`
	Price                int64           `json:"price"`
	Capacity             int32           `json:"capacity"`
	RegistrationOpensAt  time.Time       `json:"registrationOpensAt"`
	RegistrationClosesAt time.Time       `json:"registrationClosesAt"`
	BibPrefix            string          `json:"bibPrefix"`
	MinAge               *int32          `json:"minAge"`
	MaxOrderPerUser      int32           `json:"maxOrderPerUser"`
	DistanceKm           *float64        `json:"distanceKm,omitempty"`
	CutoffTime           string          `json:"cutoffTime,omitempty"`
	PricingTiers         json.RawMessage `json:"pricingTiers,omitempty"`
	CreatedAt            time.Time       `json:"createdAt"`
}
