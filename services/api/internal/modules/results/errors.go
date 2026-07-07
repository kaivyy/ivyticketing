package results

import "errors"

var (
	ErrInvalidOrgID       = errors.New("invalid organization id")
	ErrInvalidEventID     = errors.New("invalid event id")
	ErrInvalidPayload     = errors.New("invalid payload")
	ErrInvalidCSV         = errors.New("invalid csv")
	ErrEmptyCSV           = errors.New("csv has no data rows")
	ErrMissingBibColumn   = errors.New("csv missing required bib column")
	ErrResultNotFound     = errors.New("result not found")
	ErrTemplateNotFound   = errors.New("certificate template not found")
	ErrNoActiveTemplate   = errors.New("no active certificate template for this event")
	ErrCertNotEligible    = errors.New("no finished result for this ticket")
	ErrInvalidTimingToken = errors.New("invalid timing vendor token")
)
