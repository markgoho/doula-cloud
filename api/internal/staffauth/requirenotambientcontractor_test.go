package staffauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// TestRequireNotAmbientContractor covers #969's gate: ADR-0008's money
// row as amended by #282 opens to an Owner, an Admin, and an employed
// Doula alike, and refuses only a plain contractor Doula. Mirrors
// TestRequireOwnerOrAdmin's own shape, one layer down in roles.go.
func TestRequireNotAmbientContractor(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "RequireNotAmbientContractor Test Practice")

	const (
		ownerUID      = "rnac-owner"
		adminUID      = "rnac-admin"
		employeeUID   = "rnac-employee"
		contractorUID = "rnac-contractor"
	)
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{"owner"}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, adminUID, []string{"admin"}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, employeeUID, []string{"doula"}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, contractorUID, []string{"doula"}, "contractor")

	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/thing", staffauth.Middleware(db.App)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, _, ok := staffauth.RequireNotAmbientContractor(w, r); !ok {
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cases := map[string]int{
		ownerUID:      http.StatusNoContent,
		adminUID:      http.StatusNoContent,
		employeeUID:   http.StatusNoContent,
		contractorUID: http.StatusForbidden,
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
