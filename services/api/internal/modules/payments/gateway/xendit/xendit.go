package xendit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

type Config struct {
	SecretKey     string
	CallbackToken string
	Env           string
	HTTPClient    *http.Client
}

type Adapter struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) *Adapter {
	c := cfg.HTTPClient
	if c == nil {
		c = http.DefaultClient
	}
	return &Adapter{cfg: cfg, client: c}
}

func (a *Adapter) Name() string { return "xendit" }

func (a *Adapter) VerifySignature(headers http.Header, _ []byte) bool {
	if headers == nil {
		return false
	}
	got := headers.Get("x-callback-token")
	return got != "" && constantEqual(got, a.cfg.CallbackToken)
}

type callbackPayload struct {
	ExternalID string `json:"external_id"`
	ID         string `json:"id"`
	Status     string `json:"status"`
	Amount     int64  `json:"amount"`
}

func (a *Adapter) ParseCallback(rawBody []byte) (gw.CallbackResult, error) {
	var p callbackPayload
	if err := json.Unmarshal(rawBody, &p); err != nil {
		return gw.CallbackResult{}, fmt.Errorf("xendit: parse callback: %w", err)
	}
	return gw.CallbackResult{
		MerchantReference: p.ExternalID,
		GatewayReference:  p.ID,
		Status:            mapStatus(p.Status),
		Amount:            p.Amount,
		EventType:         "xendit.callback",
	}, nil
}

func (a *Adapter) CreateCharge(ctx context.Context, in gw.CreateChargeInput) (gw.CreateChargeResult, error) {
	return gw.CreateChargeResult{}, fmt.Errorf("xendit: CreateCharge not yet implemented")
}

func (a *Adapter) QueryStatus(ctx context.Context, gatewayReference string) (gw.CallbackResult, error) {
	return gw.CallbackResult{}, fmt.Errorf("xendit: QueryStatus not yet implemented")
}

func mapStatus(s string) gw.PaymentStatus {
	switch strings.ToUpper(s) {
	case "PAID", "SETTLED", "SUCCEEDED", "COMPLETED":
		return gw.StatusPaid
	case "PENDING", "ACTIVE":
		return gw.StatusPending
	case "EXPIRED":
		return gw.StatusExpired
	default:
		return gw.StatusFailed
	}
}

func constantEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
