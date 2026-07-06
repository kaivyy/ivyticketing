package reporting

import "errors"

var (
	ErrUnknownReportType = errors.New("unknown report type")
	ErrUnsupportedFormat = errors.New("unsupported export format")
	ErrJobNotFound       = errors.New("export job not found")
	ErrJobNotReady       = errors.New("export job not ready")
	ErrInvalidOrgID      = errors.New("invalid organization id")
)
