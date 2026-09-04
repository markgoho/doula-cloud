package clientkey_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"doula-cloud/api/internal/clientkey"
	"doula-cloud/api/internal/testdb"
)

func seedPractice(t *testing.T, db *testdb.DB) (practiceID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ('Test Practice') RETURNING id`,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	return practiceID
}

func seedClient(t *testing.T, db *testdb.DB, practiceID string) (clientID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name) VALUES ($1, 'Test Client') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	return clientID
}

// begin opens a transaction on the admin connection -- these tests
// exercise the package's own SQL, not the RLS policy, which
// TestPolicy_KeyIsConfinedToItsPractice covers separately on db.App.
func begin(t *testing.T, db *testdb.DB) *sql.Tx {
	t.Helper()
	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

func TestSealAndOpen_RoundTrips(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	clientID := seedClient(t, db, practiceID)
	tx := begin(t, db)

	if err := clientkey.Ensure(t.Context(), tx, practiceID, clientID); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	plaintext := []byte(`{"email":{"from":"","to":"ada@example.com"}}`)
	envelope, err := clientkey.Seal(t.Context(), tx, clientID, plaintext)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if strings.Contains(string(envelope), "ada@example.com") {
		t.Fatalf("envelope = %s, want the address not to appear in it", envelope)
	}
	if !clientkey.IsSealed(envelope) {
		t.Fatalf("IsSealed(%s) = false, want true", envelope)
	}

	opened, err := clientkey.Open(t.Context(), tx, clientID, envelope)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(opened) != string(plaintext) {
		t.Fatalf("opened = %s, want %s", opened, plaintext)
	}
}

func TestSeal_IsNotDeterministic(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	clientID := seedClient(t, db, practiceID)
	tx := begin(t, db)

	if err := clientkey.Ensure(t.Context(), tx, practiceID, clientID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	first, err := clientkey.Seal(t.Context(), tx, clientID, []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("seal first: %v", err)
	}
	second, err := clientkey.Seal(t.Context(), tx, clientID, []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("seal second: %v", err)
	}
	if string(first) == string(second) {
		t.Fatal("two seals of the same plaintext produced identical envelopes, want a fresh nonce each time")
	}
}

// TestOpen_AfterDestroy is the acceptance criterion in mechanism form:
// a diff sealed before erasure is unreadable after it, and nothing was
// updated or deleted in activity to make that so.
func TestOpen_AfterDestroy(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	clientID := seedClient(t, db, practiceID)
	tx := begin(t, db)

	if err := clientkey.Ensure(t.Context(), tx, practiceID, clientID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	envelope, err := clientkey.Seal(t.Context(), tx, clientID, []byte(`{"phone":"585-555-0100"}`))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	if err := clientkey.Destroy(t.Context(), tx, clientID); err != nil {
		t.Fatalf("destroy: %v", err)
	}

	if _, err := clientkey.Open(t.Context(), tx, clientID, envelope); !errors.Is(err, clientkey.ErrNoKey) {
		t.Fatalf("open after destroy = %v, want ErrNoKey", err)
	}
	if _, err := clientkey.Seal(t.Context(), tx, clientID, []byte(`{"a":1}`)); !errors.Is(err, clientkey.ErrNoKey) {
		t.Fatalf("seal after destroy = %v, want ErrNoKey", err)
	}
}

func TestDestroy_IsIdempotent(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	clientID := seedClient(t, db, practiceID)
	tx := begin(t, db)

	if err := clientkey.Ensure(t.Context(), tx, practiceID, clientID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if err := clientkey.Destroy(t.Context(), tx, clientID); err != nil {
		t.Fatalf("first destroy: %v", err)
	}
	if err := clientkey.Destroy(t.Context(), tx, clientID); err != nil {
		t.Fatalf("second destroy: %v", err)
	}
}

// TestEnsure_IsIdempotent proves a second Ensure neither fails nor
// replaces the key -- if it rotated, every diff sealed before it would
// already be unreadable, which is erasure by accident.
func TestEnsure_IsIdempotent(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	clientID := seedClient(t, db, practiceID)
	tx := begin(t, db)

	if err := clientkey.Ensure(t.Context(), tx, practiceID, clientID); err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	envelope, err := clientkey.Seal(t.Context(), tx, clientID, []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if err := clientkey.Ensure(t.Context(), tx, practiceID, clientID); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if _, err := clientkey.Open(t.Context(), tx, clientID, envelope); err != nil {
		t.Fatalf("open after a second ensure: %v, want the original key still in place", err)
	}
}

func TestSeal_WithoutAKeyRefuses(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	clientID := seedClient(t, db, practiceID)
	tx := begin(t, db)

	if _, err := clientkey.Seal(t.Context(), tx, clientID, []byte(`{"a":1}`)); !errors.Is(err, clientkey.ErrNoKey) {
		t.Fatalf("seal without a key = %v, want ErrNoKey", err)
	}
}

func TestIsSealed_RejectsAPlaintextDiff(t *testing.T) {
	if clientkey.IsSealed(json.RawMessage(`{"givenName":{"from":"","to":"Ada"}}`)) {
		t.Fatal("IsSealed(plaintext diff) = true, want false")
	}
	if clientkey.IsSealed(json.RawMessage(`not json`)) {
		t.Fatal("IsSealed(garbage) = true, want false")
	}
	if clientkey.IsSealed(json.RawMessage(`{"v":1}`)) {
		t.Fatal("IsSealed(envelope with no ciphertext) = true, want false")
	}
}

func TestOpen_RejectsATamperedEnvelope(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	clientID := seedClient(t, db, practiceID)
	tx := begin(t, db)

	if err := clientkey.Ensure(t.Context(), tx, practiceID, clientID); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	for name, envelope := range map[string]string{
		"not an envelope":      `{"givenName":"Ada"}`,
		"undecodable base64":   `{"v":1,"enc":"!!!not base64!!!"}`,
		"shorter than a nonce": `{"v":1,"enc":"AAAA"}`,
		"flipped ciphertext":   `{"v":1,"enc":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := clientkey.Open(t.Context(), tx, clientID, json.RawMessage(envelope)); err == nil {
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
	practiceID := seedPractice(t, db)
	otherPracticeID := seedPractice(t, db)
	clientID := seedClient(t, db, practiceID)

	mine := beginScoped(t, db, practiceID)
	if err := clientkey.Ensure(t.Context(), mine, practiceID, clientID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if _, err := clientkey.Seal(t.Context(), mine, clientID, []byte(`{"a":1}`)); err != nil {
		t.Fatalf("seal in her own practice: %v", err)
	}
	if err := mine.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	theirs := beginScoped(t, db, otherPracticeID)
	if _, err := clientkey.Seal(t.Context(), theirs, clientID, []byte(`{"a":1}`)); !errors.Is(err, clientkey.ErrNoKey) {
		t.Fatalf("seal from another practice = %v, want ErrNoKey -- the key must not be readable across a tenancy boundary", err)
	}
}

func beginScoped(t *testing.T, db *testdb.DB, practiceID string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(context.Background(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("scope tx: %v", err)
	}
	return tx
}
