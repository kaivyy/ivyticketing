package payments

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrPaymentNotFound   = apperr.New(http.StatusNotFound, "PAYMENT_NOT_FOUND", "payment not found")
	ErrOrderNotPayable   = apperr.New(http.StatusConflict, "ORDER_NOT_PAYABLE", "order is not pending payment")
	ErrPaymentActive     = apperr.New(http.StatusConflict, "PAYMENT_ALREADY_ACTIVE", "order already has an active payment")
	ErrGatewayNotAvail   = apperr.New(http.StatusBadRequest, "GATEWAY_NOT_AVAILABLE", "gateway or method not available")
	ErrUnsupportedMethod = apperr.New(http.StatusBadRequest, "UNSUPPORTED_METHOD", "payment method not supported")
	ErrGatewayError      = apperr.New(http.StatusBadGateway, "GATEWAY_ERROR", "payment gateway error")
	ErrAmountMismatch    = apperr.New(http.StatusBadRequest, "PAYMENT_AMOUNT_MISMATCH", "callback amount does not match")
	ErrInvalidSignature  = apperr.New(http.StatusUnauthorized, "INVALID_SIGNATURE", "invalid callback signature")
	ErrReconcileFailed   = apperr.New(http.StatusBadGateway, "RECONCILE_FAILED", "reconcile failed")
	ErrMerchantRefGen    = apperr.New(http.StatusInternalServerError, "MERCHANT_REF_GENERATION_FAILED", "could not generate merchant reference")
)
