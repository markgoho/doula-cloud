package staffauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// TestRequireOwnerOrAdmin covers the widened gate ADR-0008 puts in front
// of running the work (making an Offer, completing an Engagement), as
// opposed to RequireOwner's narrower gate on changing who is at the
// Practice at all.
func TestRequireOwnerOrAdmin(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "OwnerOrAdmin Test Practice")

	people := map[string]string{
		"ooa-owner": "{owner}",
		"ooa-admin": "{admin}",
		"ooa-doula": "{doula}",
	}
	for uid, roles := range people {
		seedMembershipWithRoles(t, db, practiceID, seedStaff(t, db, uid), roles)
	}

	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/thing", staffauth.Middleware(db.App)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, _, ok := staffauth.RequireOwnerOrAdmin(w, r); !ok {
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cases := map[string]int{
		"ooa-owner": http.StatusNoContent,
		"ooa-admin": http.StatusNoContent,
		"ooa-doula": http.StatusForbidden,
	}
	for uid, want := range cases {
		t.Run(uid, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/practices/"+practiceID+"/thing", nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			authntest.AddSessionCookie(req, authntest.SeedSession(t, db.App, uid))
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, want)
			}
		})
	}
}
