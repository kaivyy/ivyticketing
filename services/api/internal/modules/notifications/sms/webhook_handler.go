package sms

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// DeliveryCallback contains inbound status reports from messaging providers.
type DeliveryCallback struct {
	MessageID         string    `json:"messageId"`
	ProviderMessageID string    `json:"providerMessageId"`
	Status            string    `json:"status"` // "delivered", "read", "failed", "undelivered"
	ErrorCode         string    `json:"errorCode,omitempty"`
	ErrorMessage      string    `json:"errorMessage,omitempty"`
	Timestamp         time.Time `json:"timestamp"`
	Channel           Channel   `json:"channel"`
}

// CallbackListener is invoked when a verified callback is received.
type CallbackListener func(cb DeliveryCallback)

// WebhookHandler handles inbound delivery receipts and status updates.
type WebhookHandler struct {
	secret   string
	listener CallbackListener
	log      *slog.Logger
}

// NewWebhookHandler creates a WebhookHandler.
func NewWebhookHandler(secret string, listener CallbackListener, log *slog.Logger) *WebhookHandler {
	if log == nil {
		log = slog.Default()
	}
	return &WebhookHandler{
		secret:   secret,
		listener: listener,
		log:      log,
	}
}

// HandleCallback handles HTTP POST webhooks for delivery status updates.
func (h *WebhookHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}

	// Verify signature if secret configured
	if h.secret != "" {
		sig := r.Header.Get("X-Webhook-Signature")
		if sig == "" {
			sig = r.Header.Get("X-Hub-Signature-256")
		}
		if !h.verifySignature(body, sig) {
			h.log.Warn("sms webhook: invalid signature")
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var cb DeliveryCallback
	if err := json.Unmarshal(body, &cb); err != nil {
		h.log.Warn("sms webhook: json unmarshal failed", "err", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	h.log.Info("sms/whatsapp delivery callback received",
		"provider_msg_id", cb.ProviderMessageID,
		"status", cb.Status,
		"channel", string(cb.Channel),
	)

	if h.listener != nil {
		h.listener(cb)
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"received":true}`))
}

func (h *WebhookHandler) verifySignature(body []byte, signature string) bool {
	if signature == "" {
		return false
	}
	// Support optional "sha256=" prefix (e.g. Meta / GitHub style)
	if len(signature) > 7 && signature[:7] == "sha256=" {
		signature = signature[7:]
	}
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}
