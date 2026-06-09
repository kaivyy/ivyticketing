package notifications

import "github.com/varin/ivyticketing/services/api/internal/modules/notifications/templates"

// Notification type constants — re-exported from templates to avoid import cycles.
const (
	NotifOrderCreated      = templates.NotifOrderCreated
	NotifPaymentPaid       = templates.NotifPaymentPaid
	NotifPaymentExpired    = templates.NotifPaymentExpired
	NotifQueueAllowed      = templates.NotifQueueAllowed
	NotifBallotWinner      = templates.NotifBallotWinner
	NotifBallotNotSelected = templates.NotifBallotNotSelected
	NotifWaitlistPromoted  = templates.NotifWaitlistPromoted
)

// TemplateData is re-exported from templates for caller convenience.
type TemplateData = templates.TemplateData
