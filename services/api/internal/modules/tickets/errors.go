package tickets

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrTicketNotFound      = apperr.New(http.StatusNotFound, "TICKET_NOT_FOUND", "Ticket not found.")
	ErrTicketNotAvailable  = apperr.New(http.StatusConflict, "TICKET_NOT_AVAILABLE", "Ticket is not available for this order yet.")
	ErrInvoiceNotAvailable = apperr.New(http.StatusConflict, "INVOICE_NOT_AVAILABLE", "Invoice is only available for paid orders.")
)
