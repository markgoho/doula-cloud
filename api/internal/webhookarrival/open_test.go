package webhookarrival_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/stripemode"
	"doula-cloud/api/internal/webhookarrival"
)

type openEvent struct {
	id       string
	livemode bool
}

func openRequest(t *testing.T, body string) *http.Request {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", strings.NewReader(body))
	r.Header.Set("Stripe-Signature", "t=1,v1=abc")
	return r
}

func identify(e openEvent) (string, bool) { return e.id, e.livemode }

func TestOpenStripe(t *testing.T) {
	verifyOK := func(body []byte, sig string) (openEvent, error) {
		if sig != "t=1,v1=abc" || string(body) != "{}" {
			t.Fatalf("verify got body %q signature %q", body, sig)
		}
		return openEvent{id: "evt_1", livemode: false}, nil
	}
	verifyBad := func([]byte, string) (openEvent, error) { return openEvent{}, errors.New("bad signature") }

	cases := []struct {
		name   string
		body   string
		mode   stripemode.Mode
		verify func([]byte, string) (openEvent, error)
		wantOK bool
		status int
		msg    string
	}{
		{"verified event in the keys' mode", "{}", stripemode.Test, verifyOK, true, http.StatusOK, ""},
		{"no key configured matches any mode", "{}", "", verifyOK, true, http.StatusOK, ""},
		{"body over the bound", strings.Repeat("x", webhookarrival.MaxBodyBytes+1), stripemode.Test, verifyOK, false, http.StatusBadRequest, "invalid request body"},
		{"signature refused", "{}", stripemode.Test, verifyBad, false, http.StatusBadRequest, "invalid signature"},
		{"livemode mismatch", "{}", stripemode.Live, verifyOK, false, http.StatusBadRequest, "Event mode does not match the Stripe keys"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			_, ok := webhookarrival.OpenStripe(rec, openRequest(t, tc.body), "payments: test webhook", tc.mode, tc.verify, identify)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if !strings.Contains(rec.Body.String(), tc.msg) {
				t.Fatalf("body = %q, want it to contain %q", rec.Body.String(), tc.msg)
			}
		})
	}
}
