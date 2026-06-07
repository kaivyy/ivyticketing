package webhookhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/varin/ivyticketing/services/api/internal/modules/payments"
	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

func TestWebhookServer_Healthz(t *testing.T) {
	reg := gw.NewRegistry()
	proc := payments.NewProcessor(nil, nil)
	srv := NewServer(proc, reg)
	h := srv.Router()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d, want 200", rec.Code)
	}
}

func TestWebhookServer_UnknownGateway(t *testing.T) {
	reg := gw.NewRegistry()
	proc := payments.NewProcessor(nil, nil)
	srv := NewServer(proc, reg)
	h := srv.Router()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/duitku", strings.NewReader("test"))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown gateway = %d, want 404", rec.Code)
	}
}
