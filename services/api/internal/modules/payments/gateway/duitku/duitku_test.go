package duitku

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"strings"
	"testing"

	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

func TestVerifySignature_Valid(t *testing.T) {
	a := New(Config{MerchantCode: "MC", APIKey: "KEY"})
	raw := "MC" + "100000" + "PAY-1" + "KEY"
	sum := md5.Sum([]byte(raw))
	sig := hex.EncodeToString(sum[:])

	form := url.Values{}
	form.Set("merchantCode", "MC")
	form.Set("amount", "100000")
	form.Set("merchantOrderId", "PAY-1")
	form.Set("resultCode", "00")
	form.Set("signature", sig)
	body := []byte(form.Encode())

	if !a.VerifySignature(nil, body) {
		t.Fatal("expected valid signature")
	}
}

func TestVerifySignature_Invalid(t *testing.T) {
	a := New(Config{MerchantCode: "MC", APIKey: "KEY"})
	body := []byte("merchantCode=MC&amount=100000&merchantOrderId=PAY-1&signature=deadbeef")
	if a.VerifySignature(nil, body) {
		t.Fatal("expected invalid signature")
	}
}

func TestParseCallback_StatusMapping(t *testing.T) {
	a := New(Config{MerchantCode: "MC", APIKey: "KEY"})
	cases := map[string]gw.PaymentStatus{
		"00": gw.StatusPaid,
		"01": gw.StatusPending,
		"02": gw.StatusFailed,
	}
	for code, want := range cases {
		body := []byte("merchantOrderId=PAY-1&amount=100000&resultCode=" + code + "&reference=DTREF")
		res, err := a.ParseCallback(body)
		if err != nil {
			t.Fatalf("code %s: %v", code, err)
		}
		if res.Status != want {
			t.Errorf("code %s: status = %s, want %s", code, res.Status, want)
		}
		if res.MerchantReference != "PAY-1" {
			t.Errorf("code %s: ref = %s", code, res.MerchantReference)
		}
		if !strings.EqualFold(res.GatewayReference, "DTREF") {
			t.Errorf("code %s: gwref = %s", code, res.GatewayReference)
		}
	}
}
