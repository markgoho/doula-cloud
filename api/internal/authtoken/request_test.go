package authtoken_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/authtoken"
)

type spendBody struct {
	Token string `json:"token"`
}

func decodeSpendBody(t *testing.T, body string) (spendBody, bool, int) {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/spend", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	req, ok := authtoken.DecodeSpend(w, r, func(q *spendBody) *string { return &q.Token })
	return req, ok, w.Code
}

func TestDecodeSpend(t *testing.T) {
	if req, ok, _ := decodeSpendBody(t, `{"token":"  abc  "}`); !ok || req.Token != "abc" {
		t.Fatalf("got (%q, %v), want the trimmed token accepted", req.Token, ok)
	}
	if _, ok, code := decodeSpendBody(t, `{"token":"   "}`); ok || code != http.StatusBadRequest {
		t.Fatalf("blank token: ok=%v status=%d, want refused with 400", ok, code)
	}
	if _, ok, code := decodeSpendBody(t, `not json`); ok || code != http.StatusBadRequest {
		t.Fatalf("bad body: ok=%v status=%d, want refused with 400", ok, code)
	}
}
