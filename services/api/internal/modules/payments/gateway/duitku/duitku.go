package duitku

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

type Config struct {
	MerchantCode string
	APIKey       string
	Env          string
	HTTPClient   *http.Client
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

func (a *Adapter) Name() string { return "duitku" }

func (a *Adapter) VerifySignature(_ http.Header, rawBody []byte) bool {
	form, err := url.ParseQuery(string(rawBody))
	if err != nil {
		return false
	}
	merchantCode := form.Get("merchantCode")
	amount := form.Get("amount")
	orderID := form.Get("merchantOrderId")
	got := form.Get("signature")
	if got == "" {
		return false
	}
	raw := merchantCode + amount + orderID + a.cfg.APIKey
	sum := md5.Sum([]byte(raw))
	want := hex.EncodeToString(sum[:])
	return hmacishEqual(want, got)
}

func (a *Adapter) ParseCallback(rawBody []byte) (gw.CallbackResult, error) {
	form, err := url.ParseQuery(string(rawBody))
	if err != nil {
		return gw.CallbackResult{}, fmt.Errorf("duitku: parse callback: %w", err)
	}
	amount, _ := strconv.ParseInt(form.Get("amount"), 10, 64)
	return gw.CallbackResult{
		MerchantReference: form.Get("merchantOrderId"),
		GatewayReference:  form.Get("reference"),
		Status:            mapStatus(form.Get("resultCode")),
		Amount:            amount,
		EventType:         "duitku.callback",
	}, nil
}

func (a *Adapter) CreateCharge(ctx context.Context, in gw.CreateChargeInput) (gw.CreateChargeResult, error) {
	return gw.CreateChargeResult{}, fmt.Errorf("duitku: CreateCharge not yet implemented")
}

func (a *Adapter) QueryStatus(ctx context.Context, gatewayReference string) (gw.CallbackResult, error) {
	return gw.CallbackResult{}, fmt.Errorf("duitku: QueryStatus not yet implemented")
}

func mapStatus(resultCode string) gw.PaymentStatus {
	switch resultCode {
	case "00":
		return gw.StatusPaid
	case "01":
		return gw.StatusPending
	default:
		return gw.StatusFailed
	}
}

func hmacishEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
