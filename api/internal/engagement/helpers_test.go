package engagement_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// fakeVerifier is a test double for authn.Verifier -- see
// staffauth's own middleware_test.go for why: real Identity Platform
// tokens can't be minted without a live GCP project.
type fakeVerifier struct {
	uid string
}

func (f fakeVerifier) VerifyIDToken(_ context.Context, _ string) (*authn.VerifiedToken, error) {
	return &authn.VerifiedToken{UID: f.uid}, nil
}

// newServer mounts the same routes main.go wires up for this package,
// behind staffauth.Middleware.
func newServer(verifier authn.Verifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /practices/{practiceId}/clients",
		staffauth.Middleware(verifier, db.App)(engagement.ListHandler()))
	mux.Handle("POST /practices/{practiceId}/clients",
		staffauth.Middleware(verifier, db.App)(engagement.CreateHandler()))
	mux.Handle("GET /practices/{practiceId}/engagements/{engagementId}",
		staffauth.Middleware(verifier, db.App)(engagement.DetailHandler()))
	return httptest.NewServer(mux)
}

// seedStaffAtPractice inserts a Staff row bound to identityUID and a
// practice_memberships row linking them to an existing practiceID, using
// the superuser Admin connection (which bypasses RLS) so fixture setup
// isn't gated by the policies under test.
func seedStaffAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) {
	t.Helper()

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
}

// seedSignupBonus grants practiceID the same +3 signup-bonus credit_ledger
// row staffauth.signup writes for a real Practice, giving CreateHandler
// tests a balance to spend without going through the signup flow.
func seedSignupBonus(t *testing.T, db *testdb.DB, practiceID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', 3)`,
		practiceID,
	); err != nil {
		t.Fatalf("seed signup bonus: %v", err)
	}
}

// seedStaffWithMembership inserts a new Practice plus a Staff member at
// it, via seedStaffAtPractice.
func seedStaffWithMembership(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ('Test Practice') RETURNING id`,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	seedStaffAtPractice(t, db, practiceID, identityUID)
	return practiceID
}

// seedClientEngagement inserts a Client and an Engagement linking them to
// practiceID, using the superuser Admin connection.
func seedClientEngagement(t *testing.T, db *testdb.DB, practiceID, name, email, status string) (clientID, engagementID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (name, email) VALUES ($1, $2) RETURNING id`,
		name, email,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status) VALUES ($1, $2, $3) RETURNING id`,
		clientID, practiceID, status,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return clientID, engagementID
}
