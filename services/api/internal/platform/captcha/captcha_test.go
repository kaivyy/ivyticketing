package captcha

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFakeVerifier(t *testing.T) {
	if ok, _ := (FakeVerifier{Pass: true}).Verify(context.Background(), "tok", "1.2.3.4"); !ok {
		t.Fatal("fake pass should return true")
	}
	if ok, _ := (FakeVerifier{Pass: false}).Verify(context.Background(), "tok", "1.2.3.4"); ok {
		t.Fatal("fake fail should return false")
	}
}

func TestTurnstileVerify_SuccessAndFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("response") == "good" {
			w.Write([]byte(`{"success":true}`))
		} else {
			w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
		}
	}))
	defer srv.Close()

	v := &Turnstile{Secret: "s", Endpoint: srv.URL, HTTP: srv.Client()}
	if ok, err := v.Verify(context.Background(), "good", "1.2.3.4"); err != nil || !ok {
		t.Fatalf("good token should pass: ok=%v err=%v", ok, err)
	}
	if ok, _ := v.Verify(context.Background(), "bad", "1.2.3.4"); ok {
		t.Fatal("bad token should fail")
	}
}
