package staffauth_test

import (
	"database/sql"
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
)

// TestGatedRouter_MountGatedRefusalNamesTheSeat proves the 403 body, not
// only its status, names the seat a route declares (#1031): the sentence
// RequireOwner and RequireOwnerOrAdmin wrote before #970/#990/#1016 moved
// sixteen writes to the mount. Every route goes through
// idempotency.Router.ExemptGated, the way the moved writes are mounted;
// the third declaration is one refusalMessage has no seat sentence for.
func TestGatedRouter_MountGatedRefusalNamesTheSeat(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	s := fixture.NewScene(t, func(_ *staffauth.GatedRouter, ir *idempotency.Router, _ *sql.DB) {
		ir.ExemptGated("POST /practices/{practiceId}/owner-only", "test mount", false, staffauth.OwnerOnly, ok)
		ir.ExemptGated("POST /practices/{practiceId}/owner-or-admin", "test mount", false, staffauth.OwnerAndAdmin, ok)
		ir.ExemptGated("POST /practices/{practiceId}/admin-only", "test mount", false, []string{adminRole}, ok)
	})

	for path, want := range map[string]string{
		"owner-only":     "You need to be an owner of this Practice to do that.",
		"owner-or-admin": "You need to be an owner or admin of this Practice to do that.",
		"admin-only":     "Your role at this Practice does not let you do that.",
	} {
		body := reqtest.Expect[struct {
			Message string `json:"message"`
		}](t, http.MethodPost, s.URL("/practices/"+s.Practice.ID+"/"+path), http.StatusForbidden, s.SignedIn)
		if body.Message != want {
			t.Errorf("%s: message = %q, want %q", path, body.Message, want)
		}
	}
}
