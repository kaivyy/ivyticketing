package enterprise

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGenerateKey(t *testing.T) {
	raw, err := generateKey()
	if err != nil {
		t.Fatalf("generateKey: %v", err)
	}
	if !strings.HasPrefix(raw, "ivyk_") {
		t.Fatalf("key missing ivyk_ prefix: %q", raw)
	}
	// "ivyk_" + hex(24 bytes) = 5 + 48 chars.
	if want := 5 + keyByteLen*2; len(raw) != want {
		t.Fatalf("key length = %d, want %d", len(raw), want)
	}
	raw2, _ := generateKey()
	if raw == raw2 {
		t.Fatal("two generated keys collided")
	}
}

func TestHashKeyDeterministicAndDistinct(t *testing.T) {
	h1 := hashKey("ivyk_abc")
	h2 := hashKey("ivyk_abc")
	if h1 != h2 {
		t.Fatal("hashKey not deterministic")
	}
	if len(h1) != 64 { // hex sha256
		t.Fatalf("hash length = %d, want 64", len(h1))
	}
	if hashKey("ivyk_abc") == hashKey("ivyk_abd") {
		t.Fatal("distinct inputs hashed to same value")
	}
}

func TestPrefixOf(t *testing.T) {
	if got := prefixOf("ivyk_1234567890"); got != "ivyk_123" {
		t.Fatalf("prefixOf = %q, want ivyk_123", got)
	}
	if got := prefixOf("abc"); got != "abc" {
		t.Fatalf("prefixOf short = %q, want abc", got)
	}
}

func TestConstantTimeEqual(t *testing.T) {
	if !constantTimeEqual("deadbeef", "deadbeef") {
		t.Fatal("equal hashes reported unequal")
	}
	if constantTimeEqual("deadbeef", "deadbee0") {
		t.Fatal("unequal hashes reported equal")
	}
	if constantTimeEqual("short", "longerstring") {
		t.Fatal("different-length inputs reported equal")
	}
}

func TestSignStableAndBound(t *testing.T) {
	body := []byte(`{"a":1}`)
	sig := sign("secret", "1000", body)
	if !strings.HasPrefix(sig, "sha256=") {
		t.Fatalf("signature missing sha256= prefix: %q", sig)
	}
	if sign("secret", "1000", body) != sig {
		t.Fatal("signature not stable for same inputs")
	}
	// Signature is bound to secret, timestamp, and body.
	if sign("other", "1000", body) == sig {
		t.Fatal("signature ignored secret")
	}
	if sign("secret", "1001", body) == sig {
		t.Fatal("signature ignored timestamp")
	}
	if sign("secret", "1000", []byte(`{"a":2}`)) == sig {
		t.Fatal("signature ignored body")
	}
}

func TestBackoff(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 30 * time.Second},
		{2, 1 * time.Minute},
		{3, 2 * time.Minute},
		{4, 4 * time.Minute},
		{5, 8 * time.Minute},
		{6, 16 * time.Minute},
		{7, 30 * time.Minute}, // capped
		{20, 30 * time.Minute},
	}
	for _, c := range cases {
		if got := backoff(c.attempt); got != c.want {
			t.Errorf("backoff(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Fatalf("truncate short = %q", got)
	}
	if got := truncate("hello", 3); got != "hel" {
		t.Fatalf("truncate long = %q", got)
	}
}

func TestEncodeDecodeScopesRoundtrip(t *testing.T) {
	in := []string{"events:read", "orders:read"}
	out := decodeScopes(encodeScopes(in))
	if len(out) != 2 || out[0] != "events:read" || out[1] != "orders:read" {
		t.Fatalf("roundtrip = %v, want %v", out, in)
	}
	// nil encodes to [] not null, and decodes back to empty (never nil).
	if got := string(encodeScopes(nil)); got != "[]" {
		t.Fatalf("encodeScopes(nil) = %q, want []", got)
	}
	if got := decodeScopes(nil); got == nil || len(got) != 0 {
		t.Fatalf("decodeScopes(nil) = %v, want empty non-nil", got)
	}
	if got := decodeScopes([]byte("not json")); got == nil || len(got) != 0 {
		t.Fatalf("decodeScopes(bad) = %v, want empty non-nil", got)
	}
}

func TestSanitizeScopes(t *testing.T) {
	got := sanitizeScopes([]string{" Events:Read ", "events:read", "", "  ", "ORDERS:READ"})
	if len(got) != 2 {
		t.Fatalf("sanitizeScopes = %v, want 2 entries", got)
	}
	if got[0] != "events:read" || got[1] != "orders:read" {
		t.Fatalf("sanitizeScopes = %v, want lowercased+deduped", got)
	}
}

func TestIsValidHTTPSURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://example.com/hook", true},
		{"https://sub.example.com:8443/x", true},
		{"http://example.com/hook", false}, // must be https
		{"ftp://example.com", false},
		{"https://", false}, // no host
		{"not a url", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isValidHTTPSURL(c.url); got != c.want {
			t.Errorf("isValidHTTPSURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestExtractAPIKey(t *testing.T) {
	t.Run("x-api-key header", func(t *testing.T) {
		r, _ := http.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-API-Key", " ivyk_abc ")
		if got := extractAPIKey(r); got != "ivyk_abc" {
			t.Fatalf("extractAPIKey = %q, want ivyk_abc", got)
		}
	})
	t.Run("bearer fallback", func(t *testing.T) {
		r, _ := http.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", "Bearer ivyk_xyz")
		if got := extractAPIKey(r); got != "ivyk_xyz" {
			t.Fatalf("extractAPIKey = %q, want ivyk_xyz", got)
		}
	})
	t.Run("x-api-key wins over bearer", func(t *testing.T) {
		r, _ := http.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-API-Key", "ivyk_header")
		r.Header.Set("Authorization", "Bearer ivyk_bearer")
		if got := extractAPIKey(r); got != "ivyk_header" {
			t.Fatalf("extractAPIKey = %q, want ivyk_header", got)
		}
	})
	t.Run("none", func(t *testing.T) {
		r, _ := http.NewRequest(http.MethodGet, "/", nil)
		if got := extractAPIKey(r); got != "" {
			t.Fatalf("extractAPIKey = %q, want empty", got)
		}
	})
}
