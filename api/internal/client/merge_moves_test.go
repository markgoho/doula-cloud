package client_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

// clientIDOfEngagement reads an Engagement's client_id off the superuser
// connection -- the fact the whole of #813 turns on, read independently
// of anything an endpoint chooses to report.
func clientIDOfEngagement(t *testing.T, db *testdb.DB, engagementID string) string {
	t.Helper()
	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT client_id FROM engagements WHERE id = $1`, engagementID).Scan(&clientID); err != nil {
		t.Fatalf("read engagement client_id: %v", err)
	}
	return clientID
}

func clientIDOfRequest(t *testing.T, db *testdb.DB, requestID string) string {
	t.Helper()
	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT client_id FROM engagement_requests WHERE id = $1`, requestID).Scan(&clientID); err != nil {
		t.Fatalf("read request client_id: %v", err)
	}
	return clientID
}

func clientIDOfPortalUser(t *testing.T, db *testdb.DB, identityUID string) string {
	t.Helper()
	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT client_id FROM client_portal_users WHERE identity_uid = $1`, identityUID).Scan(&clientID); err != nil {
		t.Fatalf("read portal user client_id: %v", err)
	}
	return clientID
}

// hasDataKey answers whether clientID's data key still exists -- what
// separates "the merge left both sealed histories alone" from "the merge
// quietly shredded one".
func hasDataKey(t *testing.T, db *testdb.DB, clientID string) bool {
	t.Helper()
	var exists bool
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT EXISTS(SELECT 1 FROM client_data_keys WHERE client_id = $1)`, clientID).Scan(&exists); err != nil {
		t.Fatalf("read client data key: %v", err)
	}
	return exists
}

func seedApprovedRequest(t *testing.T, db *testdb.DB, practiceID, clientID, engagementID, staffID, kind string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, state, requested_by, decided_by, decided_at, engagement_id)
		 VALUES ($1, $2, $3, 'approved', $4, $4, now(), $5) RETURNING id`,
		practiceID, clientID, kind, staffID, engagementID,
	).Scan(&id); err != nil {
		t.Fatalf("seed approved request: %v", err)
	}
	return id
}

// markEnteredInError ends an Engagement as "This Engagement should never
// have existed" (00090's ending_reason), which is the Practice's own way
// of saying one pregnancy was typed twice.
func markEnteredInError(t *testing.T, db *testdb.DB, engagementID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET status = 'completed', ending_reason = 'entered_in_error', birth_outcome = 'unknown' WHERE id = $1`,
		engagementID,
	); err != nil {
		t.Fatalf("mark entered_in_error: %v", err)
	}
}

// TestMergeHandler_MovesEngagementsRequestsAndPortalLinks is the whole
// point of #813: two records that both carry history become one, and
// everything that follows the woman moves to the survivor. It is also
// the test that proves 00112's narrowed engagements_freeze_outcome
// trigger admits the merge's central write -- before that migration the
// database refused it outright, whatever CONTEXT.md said.
func TestMergeHandler_MovesEngagementsRequestsAndPortalLinks(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-merge-moves"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	survivorID, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Rosa Lind", "rosa@example.com", "active")
	absorbedID, absorbedEngagementID := testdb.SeedEngagementWithKind(t, db, practiceID, "Rosa Lind", "rosa2@example.com", "postpartum")
	setCreatedAt(t, db, survivorID, "2024-01-01T00:00:00Z")
	setCreatedAt(t, db, absorbedID, "2025-01-01T00:00:00Z")
	requestID := seedApprovedRequest(t, db, practiceID, absorbedID, absorbedEngagementID, staffID, "postpartum")
	testdb.SeedPortalUser(t, db, "portal-rosa-second", absorbedID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+absorbedID+"/merge",
		client.MergeRequest{Record: client.Record{GivenName: "Rosa Lind", Email: "rosa2@example.com"}, OtherClientID: survivorID})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", resp.StatusCode, http.StatusOK, readBody(t, resp))
	}
	var out client.MergeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.MergedFrom != absorbedID {
		t.Fatalf("mergedFrom = %q, want %q", out.MergedFrom, absorbedID)
	}
	if out.Moved.Engagements != 1 || out.Moved.EngagementRequests != 1 || out.Moved.PortalAccounts != 1 {
		t.Fatalf("moved = %+v, want one of each -- a silent zero is exactly what the row counts exist to catch", out.Moved)
	}

	if got := clientIDOfEngagement(t, db, absorbedEngagementID); got != survivorID {
		t.Fatalf("engagement client_id = %q, want the survivor %q", got, survivorID)
	}
	if got := clientIDOfRequest(t, db, requestID); got != survivorID {
		t.Fatalf("request client_id = %q, want the survivor %q -- an approved Request follows its Engagement", got, survivorID)
	}
	if got := clientIDOfPortalUser(t, db, "portal-rosa-second"); got != survivorID {
		t.Fatalf("portal link client_id = %q, want the survivor %q", got, survivorID)
	}

	// The plaintext audit, which has to outlive a shredding of both keys.
	var mergedAt *time.Time
	var mergedBy *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT merged_at, merged_by_staff_id FROM clients WHERE id = $1`, absorbedID,
	).Scan(&mergedAt, &mergedBy); err != nil {
		t.Fatalf("read merge audit: %v", err)
	}
	if mergedAt == nil {
		t.Fatalf("merged_at = nil, want a timestamp -- who merged this and when must survive an erasure")
	}
	if mergedBy == nil || *mergedBy != staffID {
		t.Fatalf("merged_by_staff_id = %v, want %q", mergedBy, staffID)
	}

	// Both keys survive: the two sealed histories may never fold into one.
	if !hasDataKey(t, db, absorbedID) || !hasDataKey(t, db, survivorID) {
		t.Fatalf("a data key was destroyed by the merge -- ADR-0027 keeps both until an erasure shreds them")
	}
}

// TestEngagementFreezeStillRefusesAnUntombstonedMove proves the freeze
// was narrowed rather than dropped. The refusal that #813 had to get
// past lives in the database (00093's engagements_freeze_outcome), and
// the only thing that opens it is a durable tombstone -- not a session
// flag, and not a caller's say-so.
func TestEngagementFreezeStillRefusesAnUntombstonedMove(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-freeze-still-refuses", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Held Fast", "held@example.com")
	otherID := testdb.SeedNamedClient(t, db, practiceID, "Somebody Else", "")

	_, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET client_id = $2 WHERE id = $1`, engagementID, otherID)
	if err == nil {
		t.Fatalf("moving an Engagement with no tombstone succeeded, want the trigger's refusal")
	}
	if got := clientIDOfEngagement(t, db, engagementID); got == otherID {
		t.Fatalf("engagement moved to %q anyway", otherID)
	}
}

// TestMergeHandler_RefusesTwoPendingRequestsOfOneKind proves the merge
// asks before it writes. engagement_requests_one_pending (00042) is a
// unique index on (client_id, kind) where state = 'pending', so moving
// one pending Request onto a record that already holds one of that kind
// would raise a constraint violation the endpoint could only report as
// a 500.
func TestMergeHandler_RefusesTwoPendingRequestsOfOneKind(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-merge-two-pending"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	survivorID, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Wren Ash", "wren@example.com", "active")
	otherID := testdb.SeedNamedClient(t, db, practiceID, "Wren Ash", "wren2@example.com")
	seedPendingRequest(t, db, practiceID, survivorID, staffID, "birth")
	seedPendingRequest(t, db, practiceID, otherID, staffID, "birth")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+otherID+"/merge",
		client.MergeRequest{Record: client.Record{GivenName: "Wren Ash"}, OtherClientID: survivorID})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", resp.StatusCode, http.StatusConflict, readBody(t, resp))
	}
	if got := mergedIntoOf(t, db, otherID); got != nil {
		t.Fatalf("merged_into = %v, want nil -- the refusal must be asked before anything is written", got)
	}
}

// TestMergeHandler_EnteredInErrorDoesNotCountAsAttached proves the
// second half of #813's work. "This Engagement should never have
// existed" (00090) is the Practice saying one pregnancy was typed
// twice, so the record holding only that is the one absorbed rather
// than the one that wins -- and the approved Request that produced the
// Engagement is discounted alongside it, since an Engagement exists
// only because a Request was approved and discounting the Engagement
// alone would have changed nothing.
func TestMergeHandler_EnteredInErrorDoesNotCountAsAttached(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-merge-in-error"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	realID, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Iris Vale", "iris@example.com", "active")
	typoID, typoEngagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Iris Vale", "iris2@example.com")
	seedApprovedRequest(t, db, practiceID, typoID, typoEngagementID, staffID, "birth")
	markEnteredInError(t, db, typoEngagementID)
	// The typo record is deliberately the OLDER of the two, so the
	// older-survives tiebreak would keep it if the discount did not
	// apply. Attachment is the only thing that can decide this one.
	setCreatedAt(t, db, realID, "2025-06-01T00:00:00Z")
	setCreatedAt(t, db, typoID, "2024-06-01T00:00:00Z")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+typoID+"/merge",
		client.MergeRequest{Record: client.Record{GivenName: "Iris Vale"}, OtherClientID: realID})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", resp.StatusCode, http.StatusOK, readBody(t, resp))
	}
	if got := mergedIntoOf(t, db, typoID); got == nil || *got != realID {
		t.Fatalf("typo merged_into = %v, want %q -- an in-error Engagement is not history worth keeping", got, realID)
	}
}
