package staffauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// TestGatedRouter_MountGatedRefusalNamesTheSeat proves the 403 body, not
// only its status, names the seat a route declares (#1031): the sentence
// RequireOwner and RequireOwnerOrAdmin wrote before #970/#990/#1016 moved
// sixteen writes to the mount. Every route goes through
// idempotency.Router.ExemptGated, the way the moved writes are mounted;
// the third declaration is one refusalMessage has no seat sentence for.
func TestGatedRouter_MountGatedRefusalNamesTheSeat(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Refusal Sentence Practice")
	doulaID := testdb.SeedStaff(t, db, "refusal-doula")
	seedMembershipWithRoles(t, db, practiceID, doulaID, "{doula}")

	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	ir.ExemptGated("POST /practices/{practiceId}/owner-only", "test mount", false, staffauth.OwnerOnly, ok)
	ir.ExemptGated("POST /practices/{practiceId}/owner-or-admin", "test mount", false, staffauth.OwnerAndAdmin, ok)
	ir.ExemptGated("POST /practices/{practiceId}/admin-only", "test mount", false, []string{adminRole}, ok)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	session := authntest.SeedSession(t, db.App, "refusal-doula")

	for path, want := range map[string]string{
		"owner-only":     "only a Practice Owner can do that",
		"owner-or-admin": "only a Practice Owner or Admin can do that",
		"admin-only":     "not permitted to do this",
	} {
		resp := postWithSession(t, srv.URL+"/practices/"+practiceID+"/"+path, session)
		var body struct {
			Message string `json:"message"`
		}
		err := json.NewDecoder(resp.Body).Decode(&body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("%s: decode body: %v", path, err)
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("%s: status = %d, want 403", path, resp.StatusCode)
		}
		if body.Message != want {
			t.Errorf("%s: message = %q, want %q", path, body.Message, want)
		}
	}
}
