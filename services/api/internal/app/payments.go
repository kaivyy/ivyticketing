package app

import (
	paygw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
	"github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway/duitku"
	"github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway/xendit"
)

// BuildPaymentRegistry constructs a gateway Registry from app config.
// Only gateways that are enabled with complete credentials are included.
func BuildPaymentRegistry(cfg Config) *paygw.Registry {
	r := paygw.NewRegistry()
	if cfg.DuitkuEnabled {
		r.Register(duitku.New(duitku.Config{
			MerchantCode: cfg.DuitkuMerchantCode,
			APIKey:       cfg.DuitkuAPIKey,
			Env:          cfg.DuitkuEnv,
		}))
	}
	if cfg.XenditEnabled {
		r.Register(xendit.New(xendit.Config{
			SecretKey:     cfg.XenditSecretKey,
			CallbackToken: cfg.XenditCallbackToken,
			Env:           cfg.XenditEnv,
		}))
	}
	return r
}
