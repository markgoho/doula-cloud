package authntest_test

import (
	"errors"
	"testing"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
)

// testUID is the identity the fake reports, shared across this file.
const testUID = "test-uid"

func TestVerifyIDToken_ReturnsUID(t *testing.T) {
	verified, err := authntest.Verifier{UID: testUID}.VerifyIDToken(t.Context(), "any-token")
	if err != nil {
		t.Fatalf("VerifyIDToken: %v", err)
	}
	if verified.UID != testUID {
		t.Fatalf("UID = %q, want %q", verified.UID, testUID)
	}
}

func TestVerifyIDToken_ReturnsErr(t *testing.T) {
	wantErr := errors.New("bad token")

	verified, err := authntest.Verifier{UID: testUID, Err: wantErr}.VerifyIDToken(t.Context(), "any-token")
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if verified != nil {
		t.Fatalf("verified = %+v, want nil", verified)
	}
}

// TestSeedSession_IsLive proves the seeder produces a session
// authn.Begin accepts, which is what every other package's tests lean on.
func TestSeedSession_IsLive(t *testing.T) {
	db := testdb.New(t)

	authntest.SeedSession(t, db.App, testUID)

	if got := authntest.CountFor(t, db.App, testUID); got != 1 {
		t.Fatalf("session rows = %d, want 1", got)
	}
}

// TestSeedSessionAt_Expires covers the seeder's reason for taking a mint
// time: a session minted longer ago than SessionLifetime is already
// expired, which is how a test drives the revoked-session case without
// waiting.
func TestSeedSessionAt_Expires(t *testing.T) {
	db := testdb.New(t)

	authntest.SeedSessionAt(t, db.App, testUID, time.Now().Add(-authn.SessionLifetime-time.Hour))

	var live int
	if err := db.App.QueryRowContext(t.Context(),
		`SELECT count(*) FROM sessions WHERE identity_uid = $1 AND expires_at > now()`, testUID,
	).Scan(&live); err != nil {
		t.Fatalf("count live sessions: %v", err)
	}
	if live != 0 {
		t.Fatalf("live session rows = %d, want 0", live)
	}
}

// TestEndSession_RemovesTheRow covers the helper that drives the
// revoked-session case: the row is what makes the token work, so
// deleting it is what revokes it.
func TestEndSession_RemovesTheRow(t *testing.T) {
	db := testdb.New(t)
	token := authntest.SeedSession(t, db.App, testUID)

	authntest.EndSession(t, db.App, token)

	if got := authntest.CountFor(t, db.App, testUID); got != 0 {
		t.Fatalf("session rows = %d, want 0", got)
	}
}
