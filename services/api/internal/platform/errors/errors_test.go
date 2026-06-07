package errors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError_ShapeAndStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "req_123")

	WriteError(rec, req, New(http.StatusForbidden, "FORBIDDEN", "no access"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code != "FORBIDDEN" || body.Error.Message != "no access" {
		t.Errorf("unexpected body: %+v", body.Error)
	}
	if body.Error.RequestID != "req_123" {
		t.Errorf("requestId = %q, want req_123", body.Error.RequestID)
	}
}

func TestWriteError_NonAPIErrorBecomes500(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	WriteError(rec, req, errString("boom"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code != "INTERNAL" {
		t.Errorf("code = %q, want INTERNAL", body.Error.Code)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
