package xendit

import (
	"net/http"
	"testing"

	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

func TestVerifySignature_Token(t *testing.T) {
	a := New(Config{SecretKey: "sk", CallbackToken: "tok123"})
	h := http.Header{}
	h.Set("x-callback-token", "tok123")
	if !a.VerifySignature(h, nil) {
		t.Fatal("expected valid token")
	}
	h.Set("x-callback-token", "wrong")
	if a.VerifySignature(h, nil) {
		t.Fatal("expected invalid token")
	}
}

func TestParseCallback_JSON(t *testing.T) {
	a := New(Config{SecretKey: "sk", CallbackToken: "tok"})
	body := []byte(`{"external_id":"PAY-1","id":"xnd-123","status":"PAID","amount":100000}`)
	res, err := a.ParseCallback(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if res.MerchantReference != "PAY-1" || res.GatewayReference != "xnd-123" {
		t.Errorf("refs: %+v", res)
	}
	if res.Status != gw.StatusPaid {
		t.Errorf("status = %s, want PAID", res.Status)
	}
	if res.Amount != 100000 {
		t.Errorf("amount = %d", res.Amount)
	}
}
