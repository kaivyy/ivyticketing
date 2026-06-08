package captcha

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTurnstileEndpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// Turnstile is the Cloudflare Turnstile adapter.
type Turnstile struct {
	Secret   string
	Endpoint string       // defaults to Cloudflare siteverify
	HTTP     *http.Client // defaults to a 5s-timeout client
}

func NewTurnstile(secret string) *Turnstile {
	return &Turnstile{Secret: secret, Endpoint: defaultTurnstileEndpoint, HTTP: &http.Client{Timeout: 5 * time.Second}}
}

type siteverifyResp struct {
	Success bool `json:"success"`
}

func (t *Turnstile) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	endpoint := t.Endpoint
	if endpoint == "" {
		endpoint = defaultTurnstileEndpoint
	}
	client := t.HTTP
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	form := url.Values{}
	form.Set("secret", t.Secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	var sv siteverifyResp
	if err := json.NewDecoder(resp.Body).Decode(&sv); err != nil {
		return false, err
	}
	return sv.Success, nil
}
