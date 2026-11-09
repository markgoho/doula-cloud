package clientkey_test

import (
	"database/sql"
	"encoding/json/jsontext"
	"errors"
	"strings"
	"testing"

	"doula-cloud/api/internal/clientkey"
	"doula-cloud/api/internal/testdb"
)

const aJSONBody = `{"a":1}`

// keyScene is a Client with an open transaction on the admin connection --
// these tests exercise the package's own SQL, not the RLS policy, which
// TestPolicy_KeyIsConfinedToItsPractice covers separately on db.App.
type keyScene struct {
	t          *testing.T
	tx         *sql.Tx
	practiceID string
	clientID   string
}

func newKeyScene(t *testing.T) keyScene {
	t.Helper()
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "")
	return keyScene{t: t, tx: db.BeginAdmin(t), practiceID: practiceID, clientID: clientID}
}

// newKeyedScene is a keyScene whose Client already has a key.
func newKeyedScene(t *testing.T) keyScene {
	t.Helper()
	s := newKeyScene(t)
	if err := clientkey.Ensure(t.Context(), s.tx, s.practiceID, s.clientID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	return s
}

func (s keyScene) seal(plaintext string) jsontext.Value {
	s.t.Helper()
	envelope, err := clientkey.Seal(s.t.Context(), s.tx, s.clientID, []byte(plaintext))
	if err != nil {
		s.t.Fatalf("seal: %v", err)
	}
	return envelope
}

func TestSealAndOpen_RoundTrips(t *testing.T) {
	s := newKeyedScene(t)

	plaintext := []byte(`{"email":{"from":"","to":"ada@example.com"}}`)
	envelope := s.seal(string(plaintext))
	if strings.Contains(string(envelope), "ada@example.com") {
		t.Fatalf("envelope = %s, want the address not to appear in it", envelope)
	}
	if !clientkey.IsSealed(envelope) {
		t.Fatalf("IsSealed(%s) = false, want true", envelope)
	}

	opened, err := clientkey.Open(t.Context(), s.tx, s.clientID, envelope)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(opened) != string(plaintext) {
		t.Fatalf("opened = %s, want %s", opened, plaintext)
	}
}

func TestSeal_IsNotDeterministic(t *testing.T) {
	s := newKeyedScene(t)

	if first, second := s.seal(aJSONBody), s.seal(aJSONBody); string(first) == string(second) {
		t.Fatal("two seals of the same plaintext produced identical envelopes, want a fresh nonce each time")
	}
}

// TestOpen_AfterDestroy is the acceptance criterion in mechanism form:
// a diff sealed before erasure is unreadable after it, and nothing was
// updated or deleted in activity to make that so.
func TestOpen_AfterDestroy(t *testing.T) {
	s := newKeyedScene(t)
	envelope := s.seal(`{"phone":"585-555-0100"}`)

	if err := clientkey.Destroy(t.Context(), s.tx, s.clientID); err != nil {
		t.Fatalf("destroy: %v", err)
	}

	if _, err := clientkey.Open(t.Context(), s.tx, s.clientID, envelope); !errors.Is(err, clientkey.ErrNoKey) {
		t.Fatalf("open after destroy = %v, want ErrNoKey", err)
	}
	if _, err := clientkey.Seal(t.Context(), s.tx, s.clientID, []byte(aJSONBody)); !errors.Is(err, clientkey.ErrNoKey) {
		t.Fatalf("seal after destroy = %v, want ErrNoKey", err)
	}
}

func TestDestroy_IsIdempotent(t *testing.T) {
	s := newKeyedScene(t)

	if err := clientkey.Destroy(t.Context(), s.tx, s.clientID); err != nil {
		t.Fatalf("first destroy: %v", err)
	}
	if err := clientkey.Destroy(t.Context(), s.tx, s.clientID); err != nil {
		t.Fatalf("second destroy: %v", err)
	}
}

// TestEnsure_IsIdempotent proves a second Ensure neither fails nor
// replaces the key -- if it rotated, every diff sealed before it would
// already be unreadable, which is erasure by accident.
func TestEnsure_IsIdempotent(t *testing.T) {
	s := newKeyedScene(t)
	envelope := s.seal(aJSONBody)

	if err := clientkey.Ensure(t.Context(), s.tx, s.practiceID, s.clientID); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if _, err := clientkey.Open(t.Context(), s.tx, s.clientID, envelope); err != nil {
		t.Fatalf("open after a second ensure: %v, want the original key still in place", err)
	}
}

func TestSeal_WithoutAKeyRefuses(t *testing.T) {
	s := newKeyScene(t)

	if _, err := clientkey.Seal(t.Context(), s.tx, s.clientID, []byte(aJSONBody)); !errors.Is(err, clientkey.ErrNoKey) {
		t.Fatalf("seal without a key = %v, want ErrNoKey", err)
	}
}

func TestIsSealed_RejectsAPlaintextDiff(t *testing.T) {
	if clientkey.IsSealed(jsontext.Value(`{"givenName":{"from":"","to":"Ada"}}`)) {
		t.Fatal("IsSealed(plaintext diff) = true, want false")
	}
	if clientkey.IsSealed(jsontext.Value(`not json`)) {
		t.Fatal("IsSealed(garbage) = true, want false")
	}
	if clientkey.IsSealed(jsontext.Value(`{"v":1}`)) {
		t.Fatal("IsSealed(envelope with no ciphertext) = true, want false")
	}
}

func TestOpen_RejectsATamperedEnvelope(t *testing.T) {
	s := newKeyedScene(t)

	for name, envelope := range map[string]string{
		"not an envelope":      `{"givenName":"Ada"}`,
		"undecodable base64":   `{"v":1,"enc":"!!!not base64!!!"}`,
		"shorter than a nonce": `{"v":1,"enc":"AAAA"}`,
		"flipped ciphertext":   `{"v":1,"enc":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := clientkey.Open(t.Context(), s.tx, s.clientID, jsontext.Value(envelope)); err == nil {
				t.Fatalf("open(%s) = nil error, want a refusal", envelope)
			}
		})
	}
}

// TestPolicy_KeyIsConfinedToItsPractice checks the RLS policy on
// client_data_keys the way every other package checks its own: through
// the app_runtime role, with a Practice set that is not the key's.
func TestPolicy_KeyIsConfinedToItsPractice(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	otherPracticeID := testdb.SeedPractice(t, db, "Test Practice")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "")

	mine := db.BeginAs(t, testdb.Practice(practiceID))
	if err := clientkey.Ensure(t.Context(), mine, practiceID, clientID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if _, err := clientkey.Seal(t.Context(), mine, clientID, []byte(aJSONBody)); err != nil {
		t.Fatalf("seal in her own practice: %v", err)
	}
	if err := mine.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	theirs := db.BeginAs(t, testdb.Practice(otherPracticeID))
	if _, err := clientkey.Seal(t.Context(), theirs, clientID, []byte(aJSONBody)); !errors.Is(err, clientkey.ErrNoKey) {
		t.Fatalf("seal from another practice = %v, want ErrNoKey -- the key must not be readable across a tenancy boundary", err)
	}
}
