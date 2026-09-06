package authn_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/testdb"
)

// evictionRequest builds a bare POST carrying token as the __session
// cookie, or none at all when token is empty -- the sign-in shape
// EvictionFor reads, unlike authn_test.go's own always-signed-in GET
// helper.
func evictionRequest(t *testing.T, token string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	if token != "" {
		authntest.AddSessionCookie(req, token)
	}
	return req
}

func TestTierOf(t *testing.T) {
	tests := []struct {
		name        string
		identityUID string
		want        authn.Tier
	}{
		{"identity platform uid", "abcDEF123456789012345678901X", authn.TierStaff},
		{"portal identifier", portalaccount.NewIdentifier(), authn.TierPortal},
		// The prefix is the namespace, not a substring match: a uid that
		// merely contains it is still Staff.
		{"prefix not at the start", "xportal_abc", authn.TierStaff},
		{"empty", "", authn.TierStaff},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := authn.TierOf(tt.identityUID); got != tt.want {
				t.Fatalf("TierOf(%q) = %q, want %q", tt.identityUID, got, tt.want)
			}
		})
	}
}

func TestEvictionFor_NoCookieEvictsNothing(t *testing.T) {
	db := testdb.New(t)
	ev, found, err := authn.EvictionFor(t.Context(), db.App, evictionRequest(t, ""), authn.TierStaff, time.Now())
	if err != nil {
		t.Fatalf("EvictionFor: %v", err)
	}
	if found {
		t.Fatalf("eviction = %+v, want none for a request with no session cookie", ev)
	}
}

func TestEvictionFor_DeadTokenEvictsNothing(t *testing.T) {
	db := testdb.New(t)
	ev, found, err := authn.EvictionFor(t.Context(), db.App, evictionRequest(t, "never-issued"), authn.TierStaff, time.Now())
	if err != nil {
		t.Fatalf("EvictionFor: %v", err)
	}
	if found {
		t.Fatalf("eviction = %+v, want none for a token naming no live session", ev)
	}
}

func TestEvictionFor_ExpiredSessionEvictsNothing(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()
	// Seeded far enough in the past that its row expired before now.
	token := authntest.SeedSessionAt(t, db.App, "staff-uid", now.Add(-2*authn.SessionLifetime))

	ev, found, err := authn.EvictionFor(t.Context(), db.App, evictionRequest(t, token), authn.TierPortal, now)
	if err != nil {
		t.Fatalf("EvictionFor: %v", err)
	}
	if found {
		t.Fatalf("eviction = %+v, want none for an expired session", ev)
	}
}

func TestEvictionFor_SameTierEvictsNothing(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()
	token := authntest.SeedSessionAt(t, db.App, "staff-uid", now)

	// Signing in as Staff again with a live Staff session is an ordinary
	// re-sign-in, not a cross-population eviction.
	ev, found, err := authn.EvictionFor(t.Context(), db.App, evictionRequest(t, token), authn.TierStaff, now)
	if err != nil {
		t.Fatalf("EvictionFor: %v", err)
	}
	if found {
		t.Fatalf("eviction = %+v, want none for a same-population re-sign-in", ev)
	}
}

func TestEvictionFor_OtherTierIsEvicted(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()
	portalUID := portalaccount.NewIdentifier()
	token := authntest.SeedSessionAt(t, db.App, portalUID, now)

	ev, found, err := authn.EvictionFor(t.Context(), db.App, evictionRequest(t, token), authn.TierStaff, now)
	if err != nil {
		t.Fatalf("EvictionFor: %v", err)
	}
	if !found {
		t.Fatal("no eviction found, want the live portal session")
	}
	if ev.Token != token || ev.IdentityUID != portalUID || ev.Tier != authn.TierPortal {
		t.Fatalf("eviction = %+v, want the seeded portal session", ev)
	}
}

func TestEvictionWarning_NamesOnlyThePopulation(t *testing.T) {
	staff := authn.EvictionWarning(authn.TierStaff)
	portal := authn.EvictionWarning(authn.TierPortal)
	if staff == portal {
		t.Fatalf("both warnings read %q; each has to name the population being left", staff)
	}
	for _, message := range []string{staff, portal} {
		if message == "" {
			t.Fatal("warning is empty")
		}
	}
}

func TestRefuseUnconfirmed_NothingToEvictPassesThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	if !authn.RefuseUnconfirmed(rec, evictionRequest(t, ""), authn.Eviction{}, false) {
		t.Fatal("ok = false, want true when there is nothing to evict")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want no response written", rec.Code)
	}
}

func TestRefuseUnconfirmed_ConfirmedPassesThrough(t *testing.T) {
	req := evictionRequest(t, "token")
	req.Header.Set("X-Confirmed", "true")
	rec := httptest.NewRecorder()

	ev := authn.Eviction{Token: "token", IdentityUID: "staff-uid", Tier: authn.TierStaff}
	if !authn.RefuseUnconfirmed(rec, req, ev, true) {
		t.Fatal("ok = false, want true once the caller has confirmed")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want no response written", rec.Code)
	}
}

func TestRefuseUnconfirmed_WarnsWithItsOwnCode(t *testing.T) {
	rec := httptest.NewRecorder()
	ev := authn.Eviction{Token: "token", IdentityUID: "staff-uid", Tier: authn.TierStaff}

	if authn.RefuseUnconfirmed(rec, evictionRequest(t, "token"), ev, true) {
		t.Fatal("ok = true, want false for an unconfirmed eviction")
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}

	// Told apart by code rather than by matching English prose (#692),
	// which is what lets the page render this as a warning with a
	// press-through rather than as an error.
	var out apierr.APIError
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if out.Code != string(authn.EvictionUnconfirmed) {
		t.Fatalf("code = %q, want %q", out.Code, authn.EvictionUnconfirmed)
	}
	if out.Message != authn.EvictionWarning(authn.TierStaff) {
		t.Fatalf("message = %q, want the Staff warning", out.Message)
	}
}
