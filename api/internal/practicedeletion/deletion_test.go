package practicedeletion_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/practicedeletion"
	"doula-cloud/api/internal/testdb"
)

func TestInitiateHandler_StartsThirtyDayWindow(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-initiate-happy"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	before := time.Now().UTC()
	resp := authedRequest(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/deletion", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out practicedeletion.InitiateResponse
	decodeJSON(t, resp, &out)

	wantFinalize := before.Add(practicedeletion.RestoreWindow)
	if diff := out.FinalizeAt.Sub(wantFinalize); diff < -time.Minute || diff > time.Minute {
		t.Fatalf("FinalizeAt = %v, want close to %v", out.FinalizeAt, wantFinalize)
	}

	requestedAt, finalizeAt, deletedAt, requestedBy := practiceDeletionState(t, db, practiceID)
	if requestedAt == nil {
		t.Fatal("deletion_requested_at not set")
	}
	if finalizeAt == nil {
		t.Fatal("deletion_finalize_at not set")
	}
	if deletedAt != nil {
		t.Fatal("deleted_at set on initiation, want nil")
	}
	if requestedBy == nil || *requestedBy != staffID {
		t.Fatalf("deletion_requested_by = %v, want %s", requestedBy, staffID)
	}

	var reminderCount, finalizeCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practice_deletion_outbox WHERE practice_id = $1 AND act = 'reminder' AND status = 'pending'`, practiceID,
	).Scan(&reminderCount); err != nil {
		t.Fatalf("query reminder outbox: %v", err)
	}
	if reminderCount != 1 {
		t.Fatalf("pending reminder rows = %d, want 1", reminderCount)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practice_deletion_outbox WHERE practice_id = $1 AND act = 'finalize' AND status = 'pending'`, practiceID,
	).Scan(&finalizeCount); err != nil {
		t.Fatalf("query finalize outbox: %v", err)
	}
	if finalizeCount != 1 {
		t.Fatalf("pending finalize rows = %d, want 1", finalizeCount)
	}

	var action, actorKind string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_kind::text FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 ORDER BY created_at`,
		practiceID,
	).Scan(&action, &actorKind); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if action != "practice_deletion_requested" || actorKind != "staff" {
		t.Fatalf("activity row = (%s, %s), want (practice_deletion_requested, staff)", action, actorKind)
	}
}

func TestInitiateHandler_RefusesWithoutConfirmation(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-initiate-unconfirmed"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedRequest(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/deletion", false)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestInitiateHandler_RefusesNonOwner(t *testing.T) {
	db := testdb.New(t)
	const uid = "admin-initiate-refused"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedRequest(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/deletion", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestInitiateHandler_SecondCallBlockedByPendingDeletionLockout proves
// the outer gate: once deletion is pending, staffauth.Middleware's own
// lockout refuses a second POST /deletion before InitiateHandler's own
// (race-only) "already pending" check would ever run -- POST is
// deliberately not one of the two routes (GET, DELETE) an Owner keeps
// while pending, per decision 3's "reach the pending-deletion screen to
// restore it," which never includes deleting again.
func TestInitiateHandler_SecondCallBlockedByPendingDeletionLockout(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-initiate-twice"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	first := authedRequest(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/deletion", true)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first initiate status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	second := authedRequest(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/deletion", true)
	defer second.Body.Close()
	if second.StatusCode != http.StatusForbidden {
		t.Fatalf("second initiate status = %d, want %d", second.StatusCode, http.StatusForbidden)
	}
	apiErr := apierrtest.Decode(t, second)
	if apiErr.Code != apierr.CodePracticePendingDeletion {
		t.Fatalf("code = %s, want %s", apiErr.Code, apierr.CodePracticePendingDeletion)
	}
}

func TestInitiateHandler_RefusesUnsettledInvoices(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-initiate-unsettled"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	seedUnsettledInvoice(t, db, practiceID)
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedRequest(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/deletion", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	apiErr := apierrtest.Decode(t, resp)
	if apiErr.Code != apierr.CodeFailedPrecondition {
		t.Fatalf("code = %s, want %s", apiErr.Code, apierr.CodeFailedPrecondition)
	}

	requestedAt, _, _, _ := practiceDeletionState(t, db, practiceID)
	if requestedAt != nil {
		t.Fatal("deletion_requested_at set despite refusal")
	}
}

func TestRestoreHandler_UndoesPendingDeletion(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-restore-happy"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	initiate := authedRequest(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/deletion", true)
	_ = initiate.Body.Close()
	if initiate.StatusCode != http.StatusOK {
		t.Fatalf("initiate status = %d, want %d", initiate.StatusCode, http.StatusOK)
	}

	restore := authedRequest(t, session, http.MethodDelete, srv.URL+"/api/practices/"+practiceID+"/deletion", false)
	defer restore.Body.Close()
	if restore.StatusCode != http.StatusNoContent {
		t.Fatalf("restore status = %d, want %d", restore.StatusCode, http.StatusNoContent)
	}

	requestedAt, finalizeAt, deletedAt, requestedBy := practiceDeletionState(t, db, practiceID)
	if requestedAt != nil || finalizeAt != nil || requestedBy != nil {
		t.Fatalf("deletion state not cleared: requestedAt=%v finalizeAt=%v requestedBy=%v", requestedAt, finalizeAt, requestedBy)
	}
	if deletedAt != nil {
		t.Fatal("deleted_at set by a restore")
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'practice_deletion_restored' AND actor_staff_id = $2`,
		practiceID, staffID,
	).Scan(&count); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if count != 1 {
		t.Fatalf("restored activity rows = %d, want 1", count)
	}
}

func TestRestoreHandler_RefusesNonOwner(t *testing.T) {
	db := testdb.New(t)
	const uid = "admin-restore-refused"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedRequest(t, session, http.MethodDelete, srv.URL+"/api/practices/"+practiceID+"/deletion", false)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestRestoreHandler_RefusesNothingPending(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-restore-nothing-pending"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedRequest(t, session, http.MethodDelete, srv.URL+"/api/practices/"+practiceID+"/deletion", false)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

// TestInitiateHandler_ReinitiateAfterRestoreMovesTheSamePendingRow proves
// enqueue()'s upsert: a restore during the window followed by a fresh
// initiate must not collide with the stale reminder/finalize rows the
// first initiate already queued, and must not leave them behind at their
// original (now-wrong) deadline for the skip-at-send recheck to act on.
func TestInitiateHandler_ReinitiateAfterRestoreMovesTheSamePendingRow(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-reinitiate"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	first := authedRequest(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/deletion", true)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first initiate status = %d, want %d", first.StatusCode, http.StatusOK)
	}
	firstFinalizeAt, _ := pendingOutboxRow(t, db, practiceID, "finalize")

	restore := authedRequest(t, session, http.MethodDelete, srv.URL+"/api/practices/"+practiceID+"/deletion", false)
	_ = restore.Body.Close()
	if restore.StatusCode != http.StatusNoContent {
		t.Fatalf("restore status = %d, want %d", restore.StatusCode, http.StatusNoContent)
	}

	second := authedRequest(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/deletion", true)
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second initiate status = %d, want %d", second.StatusCode, http.StatusOK)
	}

	reminderAt, reminderAttempts := pendingOutboxRow(t, db, practiceID, "reminder")
	finalizeAt, finalizeAttempts := pendingOutboxRow(t, db, practiceID, "finalize")
	if reminderAttempts != 0 || finalizeAttempts != 0 {
		t.Fatalf("attempt_count after reinitiate = %d/%d, want 0/0", reminderAttempts, finalizeAttempts)
	}
	if !finalizeAt.After(firstFinalizeAt) {
		t.Fatalf("second finalize deadline %v did not move past the first %v", finalizeAt, firstFinalizeAt)
	}
	if !reminderAt.Before(finalizeAt) {
		t.Fatalf("reminder %v did not land before finalize %v", reminderAt, finalizeAt)
	}
}

func TestStatusHandler_ReadsPendingAndUnsettledState(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-status-read"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	seedUnsettledInvoice(t, db, practiceID)
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/deletion")
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out practicedeletion.StatusResponse
	decodeJSON(t, resp, &out)
	if out.Pending {
		t.Fatal("Pending = true before any initiation")
	}
	if !out.HasUnsettledInvoices {
		t.Fatal("HasUnsettledInvoices = false, want true")
	}
}

// TestStatusHandler_ReadsPendingState covers StatusResponse's Pending
// and FinalizeAt fields, both set together at initiation.
func TestStatusHandler_ReadsPendingState(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-status-pending"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	initiate := authedRequest(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/deletion", true)
	_ = initiate.Body.Close()
	if initiate.StatusCode != http.StatusOK {
		t.Fatalf("initiate status = %d, want %d", initiate.StatusCode, http.StatusOK)
	}

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/deletion")
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out practicedeletion.StatusResponse
	decodeJSON(t, resp, &out)
	if !out.Pending || out.DeletionRequestedAt == nil || out.FinalizeAt == nil {
		t.Fatalf("status = %+v, want Pending true with both timestamps set", out)
	}
}
