package orders

import (
	"time"

	"github.com/google/uuid"
)

type OrderResponse struct {
	ID          uuid.UUID  `json:"id"`
	OrderNumber string     `json:"orderNumber"`
	EventID     uuid.UUID  `json:"eventId"`
	CategoryID  uuid.UUID  `json:"categoryId"`
	Status      string     `json:"status"`
	Subtotal    int64      `json:"subtotal"`
	Fee         int64      `json:"fee"`
	Discount    int64      `json:"discount"`
	Total       int64      `json:"total"`
	ExpiredAt   *time.Time `json:"expiredAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}
