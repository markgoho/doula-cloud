package idempotency_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// pngBytes is a minimal valid 1x1 PNG, enough for http.DetectContentType
// to recognize it as image/png -- mirrors message_test's constant of the
// same name, kept as its own copy since that one is unexported.
var pngBytes = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

// countingPutStore wraps a MemoryStore and counts Put calls, so a
// multipart create-Message retry test can prove idempotency.Wrap's replay
// skips the handler entirely -- no second object upload, not just no
// second messages row.
type countingPutStore struct {
	*objectstore.MemoryStore
	puts atomic.Int32
}

func newCountingPutStore() *countingPutStore {
	return &countingPutStore{MemoryStore: objectstore.NewMemoryStore()}
}

func (s *countingPutStore) Put(ctx context.Context, path, contentType string, r io.Reader) error {
	s.puts.Add(1)
	return s.MemoryStore.Put(ctx, path, contentType, r) //nolint:wrapcheck // test double, callers expect the underlying store's raw error
}

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

// seedOwnerStaffWithMembership mirrors seedStaffWithMembership but promotes
// the seeded Staff member to the 'owner' role -- the only role
// staffauth.InviteHandler accepts as authorization, per
// staffauth_test's seedOwnerMembership helper of the same intent.
func seedOwnerStaffWithMembership(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()
	practiceID = seedStaffWithMembership(t, db, identityUID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practice_memberships SET roles = '{owner}' WHERE practice_id = $1`,
		practiceID,
	); err != nil {
		t.Fatalf("promote to owner: %v", err)
	}
	return practiceID
}

// seedSignupBonus grants practiceID the same +3 signup-bonus credit_ledger
// row staffauth.signup writes for a real Practice, mirroring
// engagement_test's helper of the same name.
func seedSignupBonus(t *testing.T, db *testdb.DB, practiceID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', 3)`,
		practiceID,
	); err != nil {
		t.Fatalf("seed signup bonus: %v", err)
	}
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

// seedPushSubscription registers a Web Push subscription for ownerType
// ("staff" or "client") + ownerID, so notifyRecipient (message/push.go) has
// something to deliver to -- without a row here, push.FakePusher never
// records a call regardless of whether the handler actually ran.
func seedPushSubscription(t *testing.T, db *testdb.DB, ownerType, ownerID, endpoint string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO push_subscriptions (owner_type, owner_id, endpoint, p256dh_key, auth_key)
		 VALUES ($1, $2, $3, 'p256dh', 'auth')`,
		ownerType, ownerID, endpoint,
	); err != nil {
		t.Fatalf("seed push subscription: %v", err)
	}
}
