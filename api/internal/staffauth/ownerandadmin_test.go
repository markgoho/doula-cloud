package staffauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// TestRequireOwner covers the narrow gate directly. Until #1028 it was
// covered incidentally, by the seven staffauth writes that called it in
// their handler bodies; those now declare staffauth.OwnerOnly at the
// mount instead, so the only callers left are GETs in other packages
// (the export, the pending-deletion read, client.EraseEligibilityHandler)
// whose tests cannot reach this package's own coverage. An exported gate
// with no test of its own is one refactor away from silently admitting
// an Admin, so it gets asserted here rather than left to its callers.
func TestRequireOwner(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Owner Test Practice")

	people := map[string]string{
		"ro-owner": "{owner}",
		"ro-admin": "{admin}",
		"ro-doula": "{doula}",
	}
	for uid, roles := range people {
		seedMembershipWithRoles(t, db, practiceID, testdb.SeedStaff(t, db, uid), roles)
	}

	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/thing", staffauth.Middleware(db.App)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, _, ok := staffauth.RequireOwner(w, r); !ok {
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// An Admin is refused here where RequireOwnerOrAdmin admits her:
	// deciding who is at the Practice at all is the Owner's seat alone.
	cases := map[string]int{
		"ro-owner": http.StatusNoContent,
		"ro-admin": http.StatusForbidden,
		"ro-doula": http.StatusForbidden,
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

// TestRequireOwnerOrAdmin covers the widened gate ADR-0008 puts in front
// of running the work (making an Offer, completing an Engagement), as
// opposed to RequireOwner's narrower gate on changing who is at the
// Practice at all.
func TestRequireOwnerOrAdmin(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "OwnerOrAdmin Test Practice")

	people := map[string]string{
		"ooa-owner": "{owner}",
		"ooa-admin": "{admin}",
		"ooa-doula": "{doula}",
	}
	for uid, roles := range people {
		seedMembershipWithRoles(t, db, practiceID, testdb.SeedStaff(t, db, uid), roles)
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
