package tickets

import "time"

// TicketResponse is the participant/organizer-facing ticket view.
type TicketResponse struct {
	ID           string     `json:"id"`
	TicketNumber string     `json:"ticketNumber"`
	Status       string     `json:"status"`
	OrderID      string     `json:"orderId"`
	EventID      string     `json:"eventId"`
	CategoryID   string     `json:"categoryId"`
	HolderName   string     `json:"holderName"`
	HolderEmail  string     `json:"holderEmail"`
	EventTitle   string     `json:"eventTitle"`
	CategoryName string     `json:"categoryName"`
	IssuedAt     time.Time  `json:"issuedAt"`
	UsedAt       *time.Time `json:"usedAt,omitempty"`

	// BIB fields (Phase 13). All optional; hide section when nil.
	BibNumber           *string    `json:"bibNumber,omitempty"`
	BibAssignedAt       *time.Time `json:"bibAssignedAt,omitempty"`
	BibAssignmentMethod *string    `json:"bibAssignmentMethod,omitempty"`
}

// TicketWithQR adds the signed QR token to a ticket view.
type TicketWithQR struct {
	TicketResponse
	QRToken string `json:"qrToken"`
}

// QRResponse is the QR-only endpoint payload.
type QRResponse struct {
	QRToken string `json:"qrToken"`
}

// InvoiceResponse is the JSON invoice for a paid order.
type InvoiceResponse struct {
	OrderID      string    `json:"orderId"`
	OrderNumber  string    `json:"orderNumber"`
	Status       string    `json:"status"`
	EventTitle   string    `json:"eventTitle"`
	CategoryName string    `json:"categoryName"`
	HolderName   string    `json:"holderName"`
	HolderEmail  string    `json:"holderEmail"`
	Subtotal     int64     `json:"subtotal"`
	Fee          int64     `json:"fee"`
	Discount     int64     `json:"discount"`
	Total        int64     `json:"total"`
	Currency     string    `json:"currency"`
	IssuedAt     time.Time `json:"issuedAt"`
}
