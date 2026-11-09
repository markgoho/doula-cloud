package practicedeletion_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/practicedeletion"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func pendingCount(t *testing.T, e *env, act string) int {
	t.Helper()
	return testdb.Scalar[int](t, e.DB.Admin,
		`SELECT count(*) FROM practice_deletion_outbox WHERE practice_id = $1 AND act = $2 AND status = 'pending'`, e.Practice.ID, act)
}

func TestInitiateHandler_StartsThirtyDayWindow(t *testing.T) {
	e := newEnv(t, fixture.Owner())

	before := time.Now().UTC()
	resp := e.call(http.MethodPost, true)
	reqtest.RequireStatus(t, resp, http.StatusOK)
	out := reqtest.Decode[practicedeletion.InitiateResponse](t, resp)

	wantFinalize := before.Add(practicedeletion.RestoreWindow)
	if diff := out.FinalizeAt.Sub(wantFinalize); diff < -time.Minute || diff > time.Minute {
		t.Fatalf("FinalizeAt = %v, want close to %v", out.FinalizeAt, wantFinalize)
	}

	requestedAt, finalizeAt, deletedAt, requestedBy := practiceDeletionState(t, e.DB, e.Practice.ID)
	if requestedAt == nil {
		t.Fatal("deletion_requested_at not set")
	}
	if finalizeAt == nil {
		t.Fatal("deletion_finalize_at not set")
	}
	if deletedAt != nil {
		t.Fatal("deleted_at set on initiation, want nil")
	}
	if requestedBy == nil || *requestedBy != e.Staff.ID {
		t.Fatalf("deletion_requested_by = %v, want %s", requestedBy, e.Staff.ID)
	}

	for _, act := range []string{"reminder", "finalize"} {
		if n := pendingCount(t, e, act); n != 1 {
			t.Fatalf("pending %s rows = %d, want 1", act, n)
		}
	}

	var action, actorKind string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_kind::text FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 ORDER BY created_at`,
		e.Practice.ID,
	).Scan(&action, &actorKind); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if action != "practice_deletion_requested" || actorKind != "staff" {
		t.Fatalf("activity row = (%s, %s), want (practice_deletion_requested, staff)", action, actorKind)
	}
}

// TestHandlers_Refusals holds each write to its refusal: the
// confirmation header, the Owner-only gate, and a restore with nothing
// pending.
func TestHandlers_Refusals(t *testing.T) {
	cases := []struct {
		name      string
		role      fixture.StaffOption
		method    string
		confirmed bool
		want      int
	}{
		{"initiate without confirmation", fixture.Owner(), http.MethodPost, false, http.StatusBadRequest},
		{"initiate by a non-Owner", fixture.Admin(), http.MethodPost, true, http.StatusForbidden},
		{"restore by a non-Owner", fixture.Admin(), http.MethodDelete, false, http.StatusForbidden},
		{"restore with nothing pending", fixture.Owner(), http.MethodDelete, false, http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newEnv(t, tc.role).expect(tc.method, tc.confirmed, tc.want)
		})
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
	e := newEnv(t, fixture.Owner())
	e.initiate()

	second := e.call(http.MethodPost, true)
	defer second.Body.Close()
	reqtest.RequireStatus(t, second, http.StatusForbidden)
	apiErr := apierrtest.Decode(t, second)
	if apiErr.Code != apierr.CodePracticePendingDeletion {
		t.Fatalf("code = %s, want %s", apiErr.Code, apierr.CodePracticePendingDeletion)
	}
}

func TestInitiateHandler_RefusesUnsettledInvoices(t *testing.T) {
	e := newEnv(t, fixture.Owner())
	seedUnsettledInvoice(t, e.DB, e.Practice.ID)

	resp := e.call(http.MethodPost, true)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusConflict)
	apiErr := apierrtest.Decode(t, resp)
	if apiErr.Code != apierr.CodeFailedPrecondition {
		t.Fatalf("code = %s, want %s", apiErr.Code, apierr.CodeFailedPrecondition)
	}

	requestedAt, _, _, _ := practiceDeletionState(t, e.DB, e.Practice.ID)
	if requestedAt != nil {
		t.Fatal("deletion_requested_at set despite refusal")
	}
}

func TestRestoreHandler_UndoesPendingDeletion(t *testing.T) {
	e := newEnv(t, fixture.Owner())
	e.initiate()
	e.restore()

	requestedAt, finalizeAt, deletedAt, requestedBy := practiceDeletionState(t, e.DB, e.Practice.ID)
	if requestedAt != nil || finalizeAt != nil || requestedBy != nil {
		t.Fatalf("deletion state not cleared: requestedAt=%v finalizeAt=%v requestedBy=%v", requestedAt, finalizeAt, requestedBy)
	}
	if deletedAt != nil {
		t.Fatal("deleted_at set by a restore")
	}

	count := testdb.Scalar[int](t, e.DB.Admin,
		`SELECT count(*) FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'practice_deletion_restored' AND actor_staff_id = $2`,
		e.Practice.ID, e.Staff.ID)
	if count != 1 {
		t.Fatalf("restored activity rows = %d, want 1", count)
	}
}

// TestInitiateHandler_ReinitiateAfterRestoreMovesTheSamePendingRow proves
// enqueue()'s upsert: a restore during the window followed by a fresh
// initiate must not collide with the stale reminder/finalize rows the
// first initiate already queued, and must not leave them behind at their
// original (now-wrong) deadline for the skip-at-send recheck to act on.
func TestInitiateHandler_ReinitiateAfterRestoreMovesTheSamePendingRow(t *testing.T) {
	e := newEnv(t, fixture.Owner())
	e.initiate()
	firstFinalizeAt, _ := pendingOutboxRow(t, e.DB, e.Practice.ID, "finalize")
	e.restore()
	e.initiate()

	reminderAt, reminderAttempts := pendingOutboxRow(t, e.DB, e.Practice.ID, "reminder")
	finalizeAt, finalizeAttempts := pendingOutboxRow(t, e.DB, e.Practice.ID, "finalize")
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
	e := newEnv(t, fixture.Owner())
	seedUnsettledInvoice(t, e.DB, e.Practice.ID)

	resp := e.call(http.MethodGet, false)
	reqtest.RequireStatus(t, resp, http.StatusOK)
	out := reqtest.Decode[practicedeletion.StatusResponse](t, resp)
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
	e := newEnv(t, fixture.Owner())
	e.initiate()

	resp := e.call(http.MethodGet, false)
	reqtest.RequireStatus(t, resp, http.StatusOK)
	out := reqtest.Decode[practicedeletion.StatusResponse](t, resp)
	if !out.Pending || out.DeletionRequestedAt == nil || out.FinalizeAt == nil {
		t.Fatalf("status = %+v, want Pending true with both timestamps set", out)
	}
}
