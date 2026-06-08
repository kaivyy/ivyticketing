// Package captcha verifies CAPTCHA tokens (Cloudflare Turnstile).
package captcha

import "context"

// Verifier validates a CAPTCHA response token.
type Verifier interface {
	Verify(ctx context.Context, token, remoteIP string) (bool, error)
}
