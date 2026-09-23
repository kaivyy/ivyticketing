package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPProviderConfig configures a generic HTTP REST messaging provider.
type HTTPProviderConfig struct {
	Name             string
	Endpoint         string
	APIKey           string
	SenderID         string
	SupportedChannels []Channel
	Timeout          time.Duration
}

// GenericHTTPProvider sends messages over HTTP POST.
type GenericHTTPProvider struct {
	cfg        HTTPProviderConfig
	httpClient *http.Client
}

// NewGenericHTTPProvider creates a new GenericHTTPProvider.
func NewGenericHTTPProvider(cfg HTTPProviderConfig, client *http.Client) *GenericHTTPProvider {
	if client == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = 10 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &GenericHTTPProvider{
		cfg:        cfg,
		httpClient: client,
	}
}

func (p *GenericHTTPProvider) Name() string {
	if p.cfg.Name != "" {
		return p.cfg.Name
	}
	return "generic_http"
}

func (p *GenericHTTPProvider) SupportsChannel(ch Channel) bool {
	if len(p.cfg.SupportedChannels) == 0 {
		return true
	}
	for _, supported := range p.cfg.SupportedChannels {
		if supported == ch {
			return true
		}
	}
	return false
}

type outboundPayload struct {
	To             string            `json:"to"`
	From           string            `json:"from,omitempty"`
	Message        string            `json:"message"`
	Channel        string            `json:"channel"`
	Template       string            `json:"template,omitempty"`
	TemplateParams map[string]string `json:"templateParams,omitempty"`
	IdempotencyKey string            `json:"idempotencyKey,omitempty"`
}

type outboundResponse struct {
	Success   bool   `json:"success"`
	MessageID string `json:"messageId"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

func (p *GenericHTTPProvider) Send(ctx context.Context, msg Message) (DeliveryResult, error) {
	if !p.SupportsChannel(msg.Channel) {
		return DeliveryResult{}, ErrUnsupportedChannel
	}

	payload := outboundPayload{
		To:             msg.To,
		From:           p.cfg.SenderID,
		Message:        msg.Body,
		Channel:        string(msg.Channel),
		Template:       msg.TemplateName,
		TemplateParams: msg.TemplateParams,
		IdempotencyKey: msg.IdempotencyKey,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return DeliveryResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.Endpoint, bytes.NewReader(raw))
	if err != nil {
		return DeliveryResult{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	if p.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}
	if msg.IdempotencyKey != "" {
		req.Header.Set("X-Idempotency-Key", msg.IdempotencyKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("%w: %v", ErrProviderFailed, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DeliveryResult{}, fmt.Errorf("%w: http status %d body %s", ErrProviderFailed, resp.StatusCode, string(bodyBytes))
	}

	var res outboundResponse
	_ = json.Unmarshal(bodyBytes, &res)

	provMsgID := res.MessageID
	if provMsgID == "" {
		provMsgID = fmt.Sprintf("http-%d", time.Now().UnixNano())
	}

	now := time.Now()
	status := res.Status
	if status == "" {
		status = "sent"
	}

	return DeliveryResult{
		MessageID:         msg.ID,
		ProviderMessageID: provMsgID,
		Status:            status,
		Channel:           msg.Channel,
		Provider:          p.Name(),
		DeliveredAt:       &now,
	}, nil
}
