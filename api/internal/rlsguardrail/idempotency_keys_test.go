package rlsguardrail_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests hold idempotency_keys_own_scope
// (00129_idempotency_keys_client_scope.sql, #1924) to both tiers at once:
// a portal key is visible only to its own Client, under her own Practice,
// and never to a Staff session, and a Staff key is never visible to a
// Client session.

// clientIDVar is the session variable clientauth.Middleware sets.
const clientIDVar = "app.current_client_id"

// seedIdempotencyStaff inserts a Staff row for a Staff-scoped key.
func seedIdempotencyStaff(t *testing.T, db *testdb.DB, uid string) (staffID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, first_name, last_name, email, work_state) VALUES ($1, 'Key', 'Staff', $1 || '@example.com', 'NY') RETURNING id`,
		uid,
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	return staffID
}

// seedClientKey inserts a Client-scoped key through the superuser.
func seedClientKey(t *testing.T, db *testdb.DB, key, practiceID, clientID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO idempotency_keys (key, practice_id, client_id, status_code, response_body) VALUES ($1, $2, $3, 201, '{}')`,
		key, practiceID, clientID,
	); err != nil {
		t.Fatalf("seed client key: %v", err)
	}
}

// inSession runs fn in one transaction on db.App with each name in vars
// set to its value, the way clientauth.Middleware and
// staffauth.Middleware set them.
func inSession(t *testing.T, db *testdb.DB, vars map[string]string, fn func(tx *sql.Tx)) {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	for name, value := range vars {
		if _, err := tx.ExecContext(t.Context(), `SELECT set_config($1, $2, true)`, name, value); err != nil {
			t.Fatalf("set_config %s: %v", name, err)
		}
	}
	fn(tx)
}

func visibleKeys(t *testing.T, tx *sql.Tx) []string {
	t.Helper()
	rows, err := tx.QueryContext(t.Context(), `SELECT key FROM idempotency_keys ORDER BY key`)
	if err != nil {
		t.Fatalf("query idempotency_keys: %v", err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatalf("scan key: %v", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate keys: %v", err)
	}
	return keys
}

// TestRLS_GuardrailIdempotencyKeysClientTierIsolation: Client A sees her
// own key and nothing of Client B's or a Staff member's; a Staff session
// at Client A's Practice sees its own key and nothing of Client A's.
func TestRLS_GuardrailIdempotencyKeysClientTierIsolation(t *testing.T) {
	db := testdb.New(t)
	practiceA, clientA, _ := seedEngagementAt(t, db, "Key Client A")
	practiceB, clientB, _ := seedEngagementAt(t, db, "Key Client B")
	staffA := seedIdempotencyStaff(t, db, "key-staff-a")

	seedClientKey(t, db, "client-a", practiceA, clientA)
	seedClientKey(t, db, "client-b", practiceB, clientB)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO idempotency_keys (key, practice_id, staff_id, status_code, response_body) VALUES ('staff-a', $1, $2, 201, '{}')`,
		practiceA, staffA,
	); err != nil {
		t.Fatalf("seed staff key: %v", err)
	}

	inSession(t, db, map[string]string{clientIDVar: clientA}, func(tx *sql.Tx) {
		if got := visibleKeys(t, tx); len(got) != 1 || got[0] != "client-a" {
			t.Fatalf("Client A's session sees %v, want [client-a]", got)
		}
	})
	inSession(t, db, map[string]string{"app.current_practice_id": practiceA, "app.current_staff_id": staffA}, func(tx *sql.Tx) {
		if got := visibleKeys(t, tx); len(got) != 1 || got[0] != "staff-a" {
			t.Fatalf("Staff A's session sees %v, want [staff-a]", got)
		}
	})
}

// TestRLS_GuardrailIdempotencyKeysRefusesAClientKeyUnderAnotherPractice:
// a Client's session may write a key for herself only under her own
// Practice -- naming her own Client id under another Practice is refused,
// and so is naming another Client.
func TestRLS_GuardrailIdempotencyKeysRefusesAClientKeyUnderAnotherPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA, clientA, _ := seedEngagementAt(t, db, "Key Insert A")
	practiceB, clientB, _ := seedEngagementAt(t, db, "Key Insert B")

	insert := func(tx *sql.Tx, key, practiceID, clientID string) error {
		_, err := tx.ExecContext(t.Context(),
			`INSERT INTO idempotency_keys (key, practice_id, client_id, status_code, response_body) VALUES ($1, $2, $3, 201, '{}')`,
			key, practiceID, clientID,
		)
		return err //nolint:wrapcheck // test helper; the caller only checks nil-ness
	}

	inSession(t, db, map[string]string{clientIDVar: clientA}, func(tx *sql.Tx) {
		if err := insert(tx, "other-practice", practiceB, clientA); err == nil {
			t.Fatal("Client A wrote a key under Practice B, want it refused")
		}
	})
	inSession(t, db, map[string]string{clientIDVar: clientA}, func(tx *sql.Tx) {
		if err := insert(tx, "other-client", practiceB, clientB); err == nil {
			t.Fatal("Client A wrote a key for Client B, want it refused")
		}
	})
	inSession(t, db, map[string]string{clientIDVar: clientA}, func(tx *sql.Tx) {
		if err := insert(tx, "own", practiceA, clientA); err != nil {
			t.Fatalf("Client A could not write her own key under her own Practice: %v", err)
		}
	})
}
