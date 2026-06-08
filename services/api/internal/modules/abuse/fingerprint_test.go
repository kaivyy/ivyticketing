package abuse

import (
	"net/http/httptest"
	"testing"
)

func TestFingerprint_Stable(t *testing.T) {
	r1 := httptest.NewRequest("GET", "/", nil)
	r1.Header.Set("User-Agent", "UA/1.0")
	r1.Header.Set("Accept-Language", "en")
	r1.RemoteAddr = "1.2.3.4:5555"

	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("User-Agent", "UA/1.0")
	r2.Header.Set("Accept-Language", "en")
	r2.RemoteAddr = "1.2.3.4:6666" // different port, same IP

	if Fingerprint(r1) != Fingerprint(r2) {
		t.Fatal("fingerprint should be stable across ports (same IP+UA+lang)")
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	r.RemoteAddr = "10.0.0.1:1234"
	if got := ClientIP(r); got != "203.0.113.7" {
		t.Fatalf("ClientIP = %q, want 203.0.113.7", got)
	}
}

func TestClientIP_RemoteAddrFallback(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.0.2.5:9999"
	if got := ClientIP(r); got != "192.0.2.5" {
		t.Fatalf("ClientIP = %q, want 192.0.2.5", got)
	}
}
