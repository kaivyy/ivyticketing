package ballot

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrBallotClosed             = apperr.New(http.StatusConflict, "BALLOT_CLOSED", "ballot application window is not open")
	ErrAlreadyApplied           = apperr.New(http.StatusConflict, "BALLOT_ALREADY_APPLIED", "already applied to this ballot")
	ErrNotWinner                = apperr.New(http.StatusForbidden, "BALLOT_NOT_WINNER", "ballot entry is not a winner")
	ErrDrawNotAnnounced         = apperr.New(http.StatusConflict, "BALLOT_DRAW_NOT_ANNOUNCED", "ballot results not yet announced")
	ErrPaymentWindowExpired     = apperr.New(http.StatusConflict, "BALLOT_PAYMENT_WINDOW_EXPIRED", "winner payment window has expired")
	ErrDrawAlreadyRun           = apperr.New(http.StatusConflict, "BALLOT_DRAW_ALREADY_RUN", "draw has already been executed")
	ErrBallotWithdrawNotAllowed = apperr.New(http.StatusConflict, "BALLOT_WITHDRAW_NOT_ALLOWED", "can only withdraw while ballot is open")
)
