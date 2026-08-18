package idempotency_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// fakeVerifier is a test double for authn.Verifier, mirroring
// portalinvite_test's -- real Identity Platform tokens can't be minted
// without a live GCP project.
type fakeVerifier struct {
	uid string
}

func (f fakeVerifier) VerifyIDToken(_ context.Context, _ string) (*authn.VerifiedToken, error) {
	return &authn.VerifiedToken{UID: f.uid}, nil
}

// countingHandler returns an http.Handler that increments a counter on
// every invocation and writes a JSON body reporting the call number, at
// the given status. Used to prove whether Wrap actually re-ran the
// wrapped handler or replayed a stored response instead.
func countingHandler(calls *int, status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"n":` + strconv.Itoa(*calls) + `}`))
	})
}

// newIdempotencyServer wires countingHandler behind
// staffauth.Middleware(...)(idempotency.Wrap(...)), the same composition
// main.go uses for portal-invite.
func newIdempotencyServer(db *testdb.DB, uid string, calls *int, status int) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/widgets",
		staffauth.Middleware(fakeVerifier{uid: uid}, db.App)(idempotency.Wrap(countingHandler(calls, status))))
	return httptest.NewServer(mux)
}

func postWidget(t *testing.T, srv *httptest.Server, practiceID, idempotencyKey string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/practices/"+practiceID+"/widgets", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// seedStaffWithMembership seeds a Practice and a Staff member holding a
// membership there, mirroring portalinvite_test's helper of the same name.
func seedStaffWithMembership(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()
	practiceID = seedPractice(t, db, "Idempotency Test Practice")
	var staffID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, 'Test Staff', 'staff@example.com') RETURNING id`,
		identityUID,
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles) VALUES ($1, $2, '{doula}')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return practiceID
}

// seedClientEngagement inserts a Client and an Engagement linking them to
// practiceID, mirroring portalinvite_test's helper of the same name.
func seedClientEngagement(t *testing.T, db *testdb.DB, practiceID, name, email string) (clientID, engagementID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (name, email) VALUES ($1, $2) RETURNING id`,
		name, email,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id) VALUES ($1, $2) RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return clientID, engagementID
}
