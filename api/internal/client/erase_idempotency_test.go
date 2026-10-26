package client_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

// seedIdempotencyKey inserts one stored response through the superuser.
// Exactly one of staffID and clientID is set, as
// idempotency_keys_one_caller (00129) demands.
func seedIdempotencyKey(t *testing.T, db *testdb.DB, key, practiceID, staffID, clientID string) {
	t.Helper()
	var staff, clientRef any
	if staffID != "" {
		staff = staffID
	}
	if clientID != "" {
		clientRef = clientID
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO idempotency_keys (key, practice_id, staff_id, client_id, status_code, response_body)
		 VALUES ($1, $2, $3, $4, 201, '{"body":"Ada Lovelace, first baby"}')`,
		key, practiceID, staff, clientRef,
	); err != nil {
		t.Fatalf("seed idempotency key %s: %v", key, err)
	}
}

// idempotencyKeysLeft reads every stored key through the superuser, sorted.
func idempotencyKeysLeft(t *testing.T, db *testdb.DB) []string {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(), `SELECT key FROM idempotency_keys ORDER BY key`)
	if err != nil {
		t.Fatalf("read idempotency keys: %v", err)
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatalf("scan idempotency key: %v", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate idempotency keys: %v", err)
	}
	return keys
}

// erasedIdempotencyKeys reads the idempotencyKeys count off the 'erased'
// activity row.
func erasedIdempotencyKeys(t *testing.T, db *testdb.DB, clientID string) int {
	t.Helper()
	var scope struct {
		IdempotencyKeys *int `json:"idempotencyKeys"`
	}
	if err := json.Unmarshal(erasureScopeRaw(t, db, clientID), &scope); err != nil {
		t.Fatalf("decode erasure scope: %v", err)
	}
	if scope.IdempotencyKeys == nil {
		t.Fatal("erasure scope has no idempotencyKeys count")
	}
	return *scope.IdempotencyKeys
}

// TestEraseHandler_DeletesEveryStoredResponseThatCouldHoldHer is #1987's
// Erasure AC. Her own portal rows go. Every Staff row at her Practice goes
// too, whoever sent it: a Staff row names no Client, and its body is
// whatever the handler wrote, so the Practice is the one fact that says
// which rows could hold her. Another Client's portal row at the same
// Practice, and a Staff row at another Practice, cannot hold her, and
// stay. The 'erased' activity row counts what went.
func TestEraseHandler_DeletesEveryStoredResponseThatCouldHoldHer(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-idempotency"
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-erase-idempotency", []string{doulaRole}, "employee")
	clientID := seedFullClient(t, db, practiceID, ownerID)
	otherClientID := testdb.SeedNamedClient(t, db, practiceID, "Grace Hopper", "grace@example.com")
	otherPracticeID, otherStaffID := testdb.SeedStaffAtNewPractice(t, db, "owner-erase-idempotency-other", []string{ownerRole}, "employee")

	seedIdempotencyKey(t, db, "her-portal-message", practiceID, "", clientID)
	seedIdempotencyKey(t, db, "owner-row", practiceID, ownerID, "")
	seedIdempotencyKey(t, db, "doula-row", practiceID, doulaID, "")
	seedIdempotencyKey(t, db, "other-client-portal", practiceID, "", otherClientID)
	seedIdempotencyKey(t, db, "other-practice-staff", otherPracticeID, otherStaffID, "")

	eraseOK(t, db, uid, practiceID, clientID)

	want := []string{"other-client-portal", "other-practice-staff"}
	if got := idempotencyKeysLeft(t, db); !slices.Equal(got, want) {
		t.Fatalf("idempotency keys after the Erasure = %v, want %v", got, want)
	}
	if got := erasedIdempotencyKeys(t, db, clientID); got != 3 {
		t.Fatalf("erasure scope idempotencyKeys = %d, want 3", got)
	}
}

// TestEraseHandler_DeletesStoredResponsesOfAnAbsorbedRecord -- a merge
// moves no idempotency row, so a portal response she was sent under the
// record later absorbed still names that record. The survivor's Erasure
// is the only act that reaches it.
func TestEraseHandler_DeletesStoredResponsesOfAnAbsorbedRecord(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-idempotency-absorbed"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	survivorID, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Iris Bell", "iris@example.com", "active")
	absorbedID := seedFullClient(t, db, practiceID, staffID)
	seedIdempotencyKey(t, db, "absorbed-portal-message", practiceID, "", absorbedID)

	srv, session := newServer(t, db, uid)
	defer srv.Close()
	merge := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+absorbedID+"/merge",
		client.MergeRequest{GivenName: "Iris Bell", OtherClientID: survivorID})
	defer merge.Body.Close()
	if merge.StatusCode != http.StatusOK {
		t.Fatalf("merge status = %d, want %d: %s", merge.StatusCode, http.StatusOK, readBody(t, merge))
	}

	eraseOK(t, db, uid, practiceID, survivorID)

	if got := idempotencyKeysLeft(t, db); len(got) != 0 {
		t.Fatalf("idempotency keys after the Erasure = %v, want none", got)
	}
	if got := erasedIdempotencyKeys(t, db, survivorID); got != 1 {
		t.Fatalf("erasure scope idempotencyKeys = %d, want 1", got)
	}
}

// TestEraseClientIdempotencyKeys_RefusesAnyoneButAnErasedClientOfThisPractice
// is the boundary 00130's Erasure door enforces itself: it is SECURITY
// DEFINER and sees every row, so it must refuse a Client who is not
// erased, and a Client of a Practice other than the one in the
// transaction. Without that, any Staff transaction could empty its whole
// Practice's stored responses, or another Practice's Client's, by naming
// her.
func TestEraseClientIdempotencyKeys_RefusesAnyoneButAnErasedClientOfThisPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA, clientA, clientB, _ := seedTwoPracticeLogin(t, db, "idempotency-door")
	seedIdempotencyKey(t, db, "client-a", practiceA, "", clientA)
	markErased(t, db, clientB)

	const call = `SELECT erase_client_idempotency_keys($1)`
	if !callAsPractice(t, db, practiceA, call, clientA) {
		t.Fatal("erase_client_idempotency_keys admitted a Client who is not erased")
	}
	if !callAsPractice(t, db, practiceA, call, clientB) {
		t.Fatal("erase_client_idempotency_keys admitted another Practice's Client")
	}
	if got := idempotencyKeysLeft(t, db); !slices.Equal(got, []string{"client-a"}) {
		t.Fatalf("idempotency keys after refused calls = %v, want [client-a]", got)
	}
}
