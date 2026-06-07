package orders

import (
	"time"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

func checkoutEligible(event db.Event, cat db.EventCategory, now time.Time) error {
	if event.Status != "published" {
		return ErrEventNotPublished
	}
	if cat.RegistrationOpensAt.Valid && now.Before(cat.RegistrationOpensAt.Time) {
		return ErrRegistrationClosed
	}
	if cat.RegistrationClosesAt.Valid && now.After(cat.RegistrationClosesAt.Time) {
		return ErrRegistrationClosed
	}
	return nil
}
