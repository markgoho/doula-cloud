package authtoken_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/testdb"
)

func TestSpendOrRefuse_ReturnsTheIdentityThenRefusesTheSpentToken(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()
	token, err := authtoken.Mint(t.Context(), db.App, "uid-refuse", authtoken.PurposeStaffPasswordReset, time.Hour, now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/spend", nil)

	w := httptest.NewRecorder()
	uid, ok := authtoken.SpendOrRefuse(w, r, db.App, token, authtoken.PurposeStaffPasswordReset, now)
	if !ok || uid != "uid-refuse" {
		t.Fatalf("first spend = (%q, %v), want (uid-refuse, true)", uid, ok)
	}

	w = httptest.NewRecorder()
	if _, ok := authtoken.SpendOrRefuse(w, r, db.App, token, authtoken.PurposeStaffPasswordReset, now); ok {
		t.Fatal("second spend of the same token was accepted")
	}
	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if got := apierrtest.Decode(t, resp).Message; got != authtoken.MsgLinkUnusable {
		t.Fatalf("message = %q, want %q", got, authtoken.MsgLinkUnusable)
	}
}
