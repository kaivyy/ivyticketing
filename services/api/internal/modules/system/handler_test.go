package system

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeChecker struct{ err error }

func (f fakeChecker) Ping(context.Context) error { return f.err }

func TestHealthz(t *testing.T) {
	h := NewHandler(nil, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	h.Healthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestReadyz_AllHealthy(t *testing.T) {
	h := NewHandler(fakeChecker{nil}, fakeChecker{nil})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	h.Readyz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body readyResponse
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Status != "ready" || body.Checks["postgres"] != "ok" || body.Checks["redis"] != "ok" {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestReadyz_PostgresDown(t *testing.T) {
	h := NewHandler(fakeChecker{errors.New("conn refused")}, fakeChecker{nil})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	h.Readyz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body readyResponse
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Status != "not_ready" || body.Checks["postgres"] != "down" || body.Checks["redis"] != "ok" {
		t.Errorf("unexpected body: %+v", body)
	}
}
