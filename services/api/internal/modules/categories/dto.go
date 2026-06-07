package categories

import (
	"time"

	"github.com/google/uuid"
)

type WriteRequest struct {
	Name                 string    `json:"name"`
	Price                int64     `json:"price"`
	Capacity             int32     `json:"capacity"`
	RegistrationOpensAt  time.Time `json:"registrationOpensAt"`
	RegistrationClosesAt time.Time `json:"registrationClosesAt"`
	BibPrefix            string    `json:"bibPrefix"`
	MinAge               *int32    `json:"minAge"`
	MaxOrderPerUser      int32     `json:"maxOrderPerUser"`
}

type Response struct {
	ID                   uuid.UUID `json:"id"`
	EventID              uuid.UUID `json:"eventId"`
	Name                 string    `json:"name"`
	Price                int64     `json:"price"`
	Capacity             int32     `json:"capacity"`
	RegistrationOpensAt  time.Time `json:"registrationOpensAt"`
	RegistrationClosesAt time.Time `json:"registrationClosesAt"`
	BibPrefix            string    `json:"bibPrefix"`
	MinAge               *int32    `json:"minAge"`
	MaxOrderPerUser      int32     `json:"maxOrderPerUser"`
	CreatedAt            time.Time `json:"createdAt"`
}
