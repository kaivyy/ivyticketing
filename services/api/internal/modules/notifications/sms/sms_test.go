package sms

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMaskPhoneNumber(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"+6281234567890", "+628****7890"},
		{"08123456789", "0812****6789"},
		{"12345", "****"},
		{"12345678", "12****78"},
	}

	for _, c := range cases {
		got := MaskPhoneNumber(c.in)
		if got != c.want {
			t.Errorf("MaskPhoneNumber(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestIdempotencyStore(t *testing.T) {
	store := NewMemoryIdempotencyStore()

	_, ok := store.Get("key-1")
	if ok {
		t.Fatal("expected key-1 not found")
	}

	res := DeliveryResult{
		MessageID:         "m-1",
		ProviderMessageID: "prov-1",
		Status:            "delivered",
	}
	store.Set("key-1", res, 100*time.Millisecond)

	got, ok := store.Get("key-1")
	if !ok || got.ProviderMessageID != "prov-1" {
		t.Fatalf("expected key-1 to return prov-1, got %+v", got)
	}

	// Sleep until expired
	time.Sleep(120 * time.Millisecond)
	_, ok = store.Get("key-1")
	if ok {
		t.Fatal("expected key-1 to be expired")
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewMemoryRateLimiter(2, 50*time.Millisecond)

	if !rl.Allow("+6281111111111") {
		t.Fatal("expected 1st call to be allowed")
	}
	if !rl.Allow("+6281111111111") {
		t.Fatal("expected 2nd call to be allowed")
	}
	if rl.Allow("+6281111111111") {
		t.Fatal("expected 3rd call to be blocked by rate limit")
	}

	// Different phone number should still be allowed
	if !rl.Allow("+6282222222222") {
		t.Fatal("expected different phone number to be allowed")
	}

	// Wait for window to slide
	time.Sleep(60 * time.Millisecond)
	if !rl.Allow("+6281111111111") {
		t.Fatal("expected call to be allowed after window expires")
	}
}

func TestMessagingService_Send(t *testing.T) {
	svc := NewMessagingService(ServiceConfig{
		PrimaryProvider: NewLogProvider(slog.Default()),
	})

	msg := Message{
		To:      "+6281234567890",
		Body:    "Halo, pendaftaran marathon Anda telah berhasil diverifikasi!",
		Channel: ChannelWhatsApp,
	}

	res, err := svc.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected send error: %v", err)
	}

	if res.Status != "delivered" {
		t.Errorf("expected status 'delivered', got %s", res.Status)
	}
	if res.Channel != ChannelWhatsApp {
		t.Errorf("expected channel whatsapp, got %s", res.Channel)
	}
}

func TestMessagingService_Idempotency(t *testing.T) {
	svc := NewMessagingService(ServiceConfig{
		PrimaryProvider: NewLogProvider(slog.Default()),
	})

	msg := Message{
		To:             "+6281234567890",
		Body:           "Kode OTP Anda: 849201",
		Channel:        ChannelSMS,
		IdempotencyKey: "otp-req-9988",
	}

	res1, err := svc.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("first send failed: %v", err)
	}

	res2, err := svc.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("second send failed: %v", err)
	}

	if res1.ProviderMessageID != res2.ProviderMessageID {
		t.Errorf("expected identical provider message ID on idempotency hit: %s vs %s",
			res1.ProviderMessageID, res2.ProviderMessageID)
	}
}

type failingProvider struct {
	name string
}

func (f *failingProvider) Name() string                     { return f.name }
func (f *failingProvider) SupportsChannel(ch Channel) bool  { return true }
func (f *failingProvider) Send(ctx context.Context, msg Message) (DeliveryResult, error) {
	return DeliveryResult{}, errors.New("upstream gateway timeout")
}

func TestMessagingService_Fallback(t *testing.T) {
	primary := &failingProvider{name: "broken_primary"}
	fallback := NewLogProvider(slog.Default())

	svc := NewMessagingService(ServiceConfig{
		PrimaryProvider:  primary,
		FallbackProvider: fallback,
	})

	msg := Message{
		To:      "+6281234567890",
		Body:    "Pemberitahuan perubahan race kit collection.",
		Channel: ChannelSMS,
	}

	res, err := svc.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}

	if res.Provider != fallback.Name() {
		t.Errorf("expected provider %s, got %s", fallback.Name(), res.Provider)
	}
}

func TestWebhookHandler(t *testing.T) {
	secret := "test-secret-key-12345"
	var received DeliveryCallback

	listener := func(cb DeliveryCallback) {
		received = cb
	}

	handler := NewWebhookHandler(secret, listener, slog.Default())

	payload := DeliveryCallback{
		MessageID:         "msg-local-1",
		ProviderMessageID: "msg-ext-99",
		Status:            "delivered",
		Channel:           ChannelWhatsApp,
		Timestamp:         time.Now(),
	}
	body, _ := json.Marshal(payload)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook/delivery", bytes.NewReader(body))
	req.Header.Set("X-Webhook-Signature", sig)
	w := httptest.NewRecorder()

	handler.HandleCallback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d body: %s", w.Code, w.Body.String())
	}
	if received.ProviderMessageID != "msg-ext-99" {
		t.Errorf("expected provider message ID 'msg-ext-99', got %s", received.ProviderMessageID)
	}
}

func TestWebhookHandler_InvalidSignature(t *testing.T) {
	secret := "test-secret-key-12345"
	handler := NewWebhookHandler(secret, nil, slog.Default())

	body := []byte(`{"messageId":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/delivery", bytes.NewReader(body))
	req.Header.Set("X-Webhook-Signature", "invalid-signature")
	w := httptest.NewRecorder()

	handler.HandleCallback(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", w.Code)
	}
}
