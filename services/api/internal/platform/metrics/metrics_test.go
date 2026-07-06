package metrics

import (
	"math"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSnapshot_DerivedRates(t *testing.T) {
	m := New()

	// 8 checkout successes, 2 failures → 0.8 success rate.
	for i := 0; i < 8; i++ {
		m.IncCheckoutSucceeded()
		m.IncPaymentSucceeded()
	}
	for i := 0; i < 2; i++ {
		m.IncCheckoutFailed()
		m.IncPaymentFailed()
	}
	m.IncQueueReleased(5)
	m.IncRacepackScans()
	m.SetActiveQueueUsers(42)
	m.SetDBConns(3, 7)

	// HTTP: 9 ok + 1 server error → 0.1 error rate.
	for i := 0; i < 9; i++ {
		m.ObserveHTTP("GET", "/api/v1/events", 200, 20*time.Millisecond)
	}
	m.ObserveHTTP("POST", "/api/v1/orders", 500, 40*time.Millisecond)

	s := m.Snapshot()

	if s.ActiveQueueUsers != 42 {
		t.Errorf("ActiveQueueUsers = %v, want 42", s.ActiveQueueUsers)
	}
	if s.DBConnsInUse != 3 || s.DBConnsIdle != 7 {
		t.Errorf("DBConns = %v/%v, want 3/7", s.DBConnsInUse, s.DBConnsIdle)
	}
	if s.QueueReleased != 5 {
		t.Errorf("QueueReleased = %v, want 5", s.QueueReleased)
	}
	if s.RacepackScans != 1 {
		t.Errorf("RacepackScans = %v, want 1", s.RacepackScans)
	}
	if !approxEq(s.CheckoutSuccessRate, 0.8) {
		t.Errorf("CheckoutSuccessRate = %v, want 0.8", s.CheckoutSuccessRate)
	}
	if !approxEq(s.PaymentSuccessRate, 0.8) {
		t.Errorf("PaymentSuccessRate = %v, want 0.8", s.PaymentSuccessRate)
	}
	if s.HTTPRequests != 10 {
		t.Errorf("HTTPRequests = %v, want 10", s.HTTPRequests)
	}
	if !approxEq(s.ErrorRate, 0.1) {
		t.Errorf("ErrorRate = %v, want 0.1", s.ErrorRate)
	}
	if s.HTTPP95Seconds <= 0 {
		t.Errorf("HTTPP95Seconds = %v, want > 0", s.HTTPP95Seconds)
	}
}

func TestSnapshot_EmptyIsZero(t *testing.T) {
	s := New().Snapshot()
	if s.ErrorRate != 0 || s.CheckoutSuccessRate != 0 || s.HTTPP95Seconds != 0 {
		t.Errorf("empty snapshot should have zero rates, got %+v", s)
	}
}

func TestMetricsEndpoint_Exposes(t *testing.T) {
	m := New()
	m.IncCheckoutSucceeded()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	m.SnapshotHandler(rec, req)

	if rec.Code != 200 {
		t.Fatalf("snapshot handler status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Error("expected Content-Type header on snapshot response")
	}
}

func approxEq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
