package scanner

import "errors"

// Sentinel errors for the scanner module. Handlers map these to stable HTTP
// codes in writeError (task 7.3). Signature-related sentinels wrap the qr
// package's own errors at the service boundary so callers of the scanner use
// a single error vocabulary.
var (
	// ErrSignatureInvalid is returned when a QR token's HMAC signature does not
	// match (forged or tampered token).
	ErrSignatureInvalid = errors.New("scanner: qr signature invalid")
	// ErrMalformedToken is returned when a QR token does not have the expected
	// structure or its payload cannot be decoded.
	ErrMalformedToken = errors.New("scanner: qr token malformed")
	// ErrUnsupportedVersion is returned when a QR token's schema version is not
	// supported by the server.
	ErrUnsupportedVersion = errors.New("scanner: qr token version unsupported")
	// ErrEventMismatch is returned when the ticket's embedded event does not
	// match the selected Permitted_Event.
	ErrEventMismatch = errors.New("scanner: ticket does not belong to this event")
	// ErrTicketNotFound is returned when the referenced ticket does not exist.
	ErrTicketNotFound = errors.New("scanner: ticket not found")
	// ErrTicketCancelled is returned when a scan targets a CANCELLED ticket.
	ErrTicketCancelled = errors.New("scanner: ticket cancelled")
	// ErrAlreadyCheckedIn is returned when a ticket has already been checked in
	// (status USED). This is a duplicate, not a hard failure — the service may
	// surface it as a duplicate result instead of an error.
	ErrAlreadyCheckedIn = errors.New("scanner: ticket already checked in")
	// ErrUnauthorizedEvent is returned when staff attempt an operation on an
	// event they do not hold a scanning permission for.
	ErrUnauthorizedEvent = errors.New("scanner: not authorized for this event")
	// ErrIdempotencyConflict is returned when an Idempotency-Key is reused with
	// a different request payload.
	ErrIdempotencyConflict = errors.New("scanner: idempotency key reused with different payload")
)
