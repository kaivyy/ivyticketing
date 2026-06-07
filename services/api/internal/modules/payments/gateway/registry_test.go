package gateway

import (
	"context"
	"net/http"
	"testing"
)

type fakeGateway struct{ name string }

func (f fakeGateway) Name() string { return f.name }
func (f fakeGateway) CreateCharge(ctx context.Context, in CreateChargeInput) (CreateChargeResult, error) {
	return CreateChargeResult{GatewayReference: "ref"}, nil
}
func (f fakeGateway) VerifySignature(h http.Header, b []byte) bool { return true }
func (f fakeGateway) ParseCallback(b []byte) (CallbackResult, error) {
	return CallbackResult{MerchantReference: "PAY-X", Status: StatusPaid}, nil
}
func (f fakeGateway) QueryStatus(ctx context.Context, ref string) (CallbackResult, error) {
	return CallbackResult{Status: StatusPaid}, nil
}

func TestRegistry_GetAndHas(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeGateway{name: "duitku"})

	if !r.Has("duitku") {
		t.Fatal("expected duitku registered")
	}
	if r.Has("xendit") {
		t.Fatal("xendit should not be registered")
	}
	gw, ok := r.Get("duitku")
	if !ok || gw.Name() != "duitku" {
		t.Fatalf("Get(duitku) = %v, %v", gw, ok)
	}
	if _, ok := r.Get("midtrans"); ok {
		t.Fatal("Get(midtrans) should be false")
	}
}
