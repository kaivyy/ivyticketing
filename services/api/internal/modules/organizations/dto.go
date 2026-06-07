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
