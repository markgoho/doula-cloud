package staffauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const gateNotFound = "gate: engagement not found"

// TestRequireEngagementGates covers the two shared engagement gates: an
// Engagement at another Practice (or one that does not exist) is a 404
// for every caller, and a contractor Doula reaches only an Engagement she
// holds a granted attachment on.
func TestRequireEngagementGates(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Engagement Gate Practice")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	otherPracticeID := testdb.SeedPractice(t, db, "Engagement Gate Other Practice")
	_, otherEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)

	const (
		ownerUID      = "gate-owner"
		attachedUID   = "gate-contractor-attached"
		unattachedUID = "gate-contractor-unattached"
	)
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{"owner"}, "employee")
	attachedID := testdb.SeedStaffAtPractice(t, db, practiceID, attachedUID, []string{"doula"}, "contractor")
	testdb.SeedStaffAtPractice(t, db, practiceID, unattachedUID, []string{"doula"}, "contractor")
	testdb.SeedAttachment(t, db, engagementID, attachedID, "granted", false)

	mux := http.NewServeMux()
	gate := func(w http.ResponseWriter, r *http.Request) {
		tx, pid, ok := staffauth.RequireTx(w, r)
		if !ok {
			return
		}
		id := r.PathValue("engagementId")
		if !staffauth.RequireEngagementAtPractice(w, r, tx, id, pid) {
			return
		}
		if !staffauth.RequireEngagementAccess(w, r, tx, id, gateNotFound) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
	mux.Handle("GET /practices/{practiceId}/engagements/{engagementId}", staffauth.Middleware(db.App)(http.HandlerFunc(gate)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cases := []struct {
		name         string
		uid          string
		engagementID string
		want         int
	}{
		{"owner, own engagement", ownerUID, engagementID, http.StatusNoContent},
		{"owner, engagement at another practice", ownerUID, otherEngagementID, http.StatusNotFound},
		{"contractor, attached", attachedUID, engagementID, http.StatusNoContent},
		{"contractor, not attached", unattachedUID, engagementID, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/"+practiceID+"/engagements/"+tc.engagementID, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			authntest.AddSessionCookie(req, authntest.SeedSession(t, db.App, tc.uid))
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}
