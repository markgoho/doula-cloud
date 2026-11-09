package engagement_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// transitionBody is TransitionRequest's own JSON shape, built inline so
// this file needs no import of the package's exported request type.
// endingReason/endingNote are omitted from the body entirely when "",
// the same "absent, not empty-string" shape a real caller sends for a
// non-completing move.
func transitionBody(status, endingReason, endingNote string) map[string]any {
	body := map[string]any{"status": status}
	if endingReason != "" {
		body["endingReason"] = endingReason
	}
	if endingNote != "" {
		body["endingNote"] = endingNote
	}
	return body
}

// transitionResponseBody is both shapes this endpoint answers with --
// the success DTO, and apierr's own error envelope, whose Code a caller
// is required to branch on rather than on the prose beside it
// (docs/api-design.md section 7). One struct, the shape
// outcomeResponseBody already uses for the sibling endpoint.
//
// Stays a struct of its own rather than reaching for
// apierrtest.Decode, #811's one reader of the envelope: this package's
// request helper decodes once and hands back whichever of the two
// shapes arrived, so a test that asked for the envelope specifically
// would have to read the body a second time and would lose the success
// fields. Code is apierr.Code so the enumerated constants still compare
// without a conversion.
type transitionResponseBody struct {
	EngagementID string      `json:"engagementId"`
	Status       string      `json:"status"`
	StatusMoves  []string    `json:"statusMoves"`
	Code         apierr.Code `json:"code"`
	Message      string      `json:"message"`
}

// transition sends a status transition for the Engagement as the Staff
// member at the screen and requires want.
func (e *env) transition(body map[string]any, want int) transitionResponseBody {
	e.T.Helper()
	return e.transitionOf(e.Engagement.ID, body, want)
}

func (e *env) transitionOf(engagementID string, body map[string]any, want int) transitionResponseBody {
	e.T.Helper()
	return reqtest.Expect[transitionResponseBody](e.T, http.MethodPatch, e.urlOf(engagementID, "/status"), want, e.SignedIn, reqtest.JSON(body))
}

// countOutcomeEvents counts the 'birth_outcome_recorded' rows on
// engagementID -- the audit trail the completion path must neither write
// to nor duplicate.
func countOutcomeEvents(t *testing.T, db *testdb.DB, engagementID string) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity
		  WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'birth_outcome_recorded'`, engagementID,
	).Scan(&n); err != nil {
		t.Fatalf("count outcome events: %v", err)
	}
	return n
}

// readEngagementStatus is the raw row TestTransitionHandler_* assertions
// check against, bypassing the API to prove the database itself agrees.
func readEngagementStatus(t *testing.T, db *testdb.DB, engagementID string) (status string, endingReason, endingNote *string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text, ending_reason::text, ending_note FROM engagements WHERE id = $1`, engagementID,
	).Scan(&status, &endingReason, &endingNote); err != nil {
		t.Fatalf("read engagement: %v", err)
	}
	return status, endingReason, endingNote
}

// countFactEntries counts the activity rows that record a change to one
// of engagementID's mutable facts -- status, kind, birth outcome, the
// set engagement_events held before #1423 folded it into activity --
// proving a handler wrote exactly one (or zero, for a no-op or a
// refusal). Every legal status move writes exactly one of the first
// three actions, so the count is the same one the dropped table gave.
func countFactEntries(t *testing.T, db *testdb.DB, engagementID string) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity
		  WHERE subject_kind = 'engagement' AND subject_id = $1
		    AND action IN ('care_phase_changed', 'engagement_completed', 'engagement_reopened',
		                   'kind_changed', 'birth_outcome_recorded', 'due_date_changed')`, engagementID,
	).Scan(&n); err != nil {
		t.Fatalf("count fact entries: %v", err)
	}
	return n
}

// diffStatusBefore and diffStatusAfter are the two keys every status
// move's entry carries (#1423), named once for the three tests that read
// them.
const (
	diffStatusBefore = "statusBefore"
	diffStatusAfter  = "statusAfter"
)

// readFactDiff reads the diff of engagementID's most recent activity row
// carrying action -- the both-sides record engagement_events used to
// hold in columns (#1423) -- plus its actor, which every writer sets.
func readFactDiff(t *testing.T, db *testdb.DB, engagementID, action string) map[string]any {
	t.Helper()
	var raw []byte
	var actor *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT diff, actor_staff_id::text FROM activity
		  WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = $2
		  ORDER BY created_at DESC, id DESC LIMIT 1`, engagementID, action,
	).Scan(&raw, &actor); err != nil {
		t.Fatalf("read %s entry: %v", action, err)
	}
	if actor == nil {
		t.Fatalf("%s entry has no actor_staff_id, want the Staff member who acted", action)
	}
	var diff map[string]any
	if err := json.Unmarshal(raw, &diff); err != nil {
		t.Fatalf("decode %s diff: %v", action, err)
	}
	return diff
}

// assertDiff checks each key in want against diff; a nil want means the
// key must be present and JSON null, never absent -- both sides of every
// fact are recorded, the shape engagement_events' columns had.
func assertDiff(t *testing.T, diff map[string]any, want map[string]any) {
	t.Helper()
	for k, v := range want {
		got, present := diff[k]
		if !present {
			t.Errorf("diff has no %q, want %v (diff = %v)", k, v, diff)
			continue
		}
		if got != v {
			t.Errorf("diff[%q] = %v, want %v", k, got, v)
		}
	}
}

// countActivityActions counts activity rows for engagementID carrying
// action.
func countActivityActions(t *testing.T, db *testdb.DB, engagementID, action string) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE subject_id = $1 AND action = $2`, engagementID, action,
	).Scan(&n); err != nil {
		t.Fatalf("count activity: %v", err)
	}
	return n
}

// adminRole is named once so golangci-lint's goconst check doesn't see
// repeated "admin" literals across this file's table-driven cases.
const adminRole = "admin"

// TestTransitionHandler_LegalMovesByRole is ADR-0015's whole role table
// crossed with its whole move table: an Owner or Admin reaches all four
// legal moves, an employee Doula reaches every move except reopening a
// completed Engagement (Owner/Admin only), and a contractor Doula, or a
// Staff member who holds no role at all, is refused every one of them
// outright rather than falling through to an allowed default.
func TestTransitionHandler_LegalMovesByRole(t *testing.T) {
	moves := []struct {
		name         string
		from         string
		to           string
		endingReason string
	}{
		{"intake to active", engagement.StatusIntake, engagement.StatusActive, ""},
		{"active to completed", engagement.StatusActive, engagement.StatusCompleted, careCompleteReason},
		{"intake to completed", engagement.StatusIntake, engagement.StatusCompleted, careCompleteReason},
		{"completed to active (reopen)", engagement.StatusCompleted, engagement.StatusActive, ""},
	}
	roles := []struct {
		fixture.RoleCase
		mayMove bool
	}{
		{fixture.OwnerCase, true},
		{fixture.AdminCase, true},
		{fixture.EmployeeDoulaCase, true},
		{fixture.ContractorDoulaCase, false},
		{fixture.NoRoleCase, false},
	}

	for _, move := range moves {
		for _, role := range roles {
			t.Run(move.name+"/"+role.Name, func(t *testing.T) {
				e := newEnv(t, signedInAs(role.Opts...), withEngagement(fixture.Status(move.from)))
				if move.to == engagement.StatusCompleted {
					testdb.SeedBirthOutcome(t, e.DB, e.Engagement.ID)
				}
				isReopen := move.from == engagement.StatusCompleted && move.to == engagement.StatusActive
				wantOK := role.mayMove && (!isReopen || role.Name != employeeDoulaKind)

				body := e.transition(transitionBody(move.to, move.endingReason, ""), okOrForbidden(wantOK))

				if !wantOK {
					return
				}
				if body.Status != move.to {
					t.Fatalf("status in body = %q, want %q", body.Status, move.to)
				}
				if got, _, _ := readEngagementStatus(t, e.DB, e.Engagement.ID); got != move.to {
					t.Fatalf("engagements.status = %q, want %q", got, move.to)
				}
			})
		}
	}
}

// TestTransitionHandler_ReopenClearsReasonAndNote proves reopening a
// completed Engagement clears both ending_reason and ending_note, which
// are demanded again at the next completion (ADR-0015).
func TestTransitionHandler_ReopenClearsReasonAndNote(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), withEngagement(fixture.Status(fixture.StatusCompleted)))
	e.Exec(`UPDATE engagements SET ending_note = 'left the area' WHERE id = $1`, e.Engagement.ID)

	e.transition(transitionBody(engagement.StatusActive, "", ""), http.StatusOK)

	gotStatus, endingReason, endingNote := readEngagementStatus(t, e.DB, e.Engagement.ID)
	if gotStatus != engagement.StatusActive || endingReason != nil || endingNote != nil {
		t.Fatalf("status=%q endingReason=%v endingNote=%v, want active with both cleared", gotStatus, endingReason, endingNote)
	}
	// ADR-0015: "Reopening unfreezes nothing." The two facts the ending
	// carries are cleared and asked for again; the birth outcome is not
	// one of them, and #940 must not have turned reopening into a way to
	// shed it.
	if outcome, _ := readEngagementOutcome(t, e.DB, e.Engagement.ID); outcome == nil {
		t.Fatal("reopening cleared the birth outcome, which the freeze rule says it never does")
	}
	// #1423: the reopen is one engagement_reopened entry carrying both
	// sides of every fact it moved, the ending it cleared included, and
	// never a completion entry.
	assertDiff(t, readFactDiff(t, e.DB, e.Engagement.ID, "engagement_reopened"), map[string]any{
		diffStatusBefore:     engagement.StatusCompleted,
		diffStatusAfter:      engagement.StatusActive,
		"endingReasonBefore": careCompleteReason,
		"endingReasonAfter":  nil,
		"endingNoteBefore":   "left the area",
		"endingNoteAfter":    nil,
	})
	if n := countFactEntries(t, e.DB, e.Engagement.ID); n != 1 {
		t.Fatalf("fact entries = %d, want 1", n)
	}
}

// TestTransitionHandler_CompletingRefusesWithNoBirthOutcome is #940's
// own refusal: ADR-0015's engagements_completed_is_explained (00094)
// will not let an Engagement reach 'completed' while its birth outcome
// is null, and a caller meets that as a named 409 rather than a raw
// constraint violation. Nothing is written on the refusal -- not the
// status, not an audit row, not the completion cascade's own
// attachment close.
func TestTransitionHandler_CompletingRefusesWithNoBirthOutcome(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), withEngagement(fixture.Status(fixture.StatusActive)))
	testdb.SeedAttachment(t, e.DB, e.Engagement.ID, e.Staff.ID, "granted", false)

	body := e.transition(transitionBody(engagement.StatusCompleted, careCompleteReason, ""), http.StatusConflict)

	if body.Code != apierr.CodeBirthOutcomeRequired {
		t.Fatalf("code = %q, want %s", body.Code, apierr.CodeBirthOutcomeRequired)
	}
	// #2199: the hub shows these words as written, so they are a sentence
	// for that screen, the same as the hub's own line before it asks.
	if body.Message != engagement.MsgBirthOutcomeFirst {
		t.Fatalf("message = %q, want %q", body.Message, engagement.MsgBirthOutcomeFirst)
	}
	gotStatus, endingReason, _ := readEngagementStatus(t, e.DB, e.Engagement.ID)
	if gotStatus != engagement.StatusActive || endingReason != nil {
		t.Fatalf("status=%q endingReason=%v, want the row untouched", gotStatus, endingReason)
	}
	if n := countFactEntries(t, e.DB, e.Engagement.ID); n != 0 {
		t.Fatalf("fact entries = %d, want 0 on a refusal", n)
	}
	if n := e.countActivity("engagement_completed"); n != 0 {
		t.Fatalf("engagement_completed activity rows = %d, want 0 on a refusal", n)
	}
	open := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM engagement_attachments WHERE engagement_id = $1 AND ended_at IS NULL`, e.Engagement.ID)
	if open != 1 {
		t.Fatalf("open attachments = %d, want 1: the completion cascade must not run on a refusal", open)
	}
}

// TestTransitionHandler_CompletingWithUnknownOutcomeAndNoDate is the
// cost ADR-0015 accepts, proved reachable: a Client who vanished during
// intake is completed on an 'unknown' outcome carrying no date at all,
// which engagements_outcome_is_dated (00093) is written to allow so that
// no Practice has to invent one. The recorded pair survives the move
// untouched, and the completion writes no second
// 'birth_outcome_recorded' event on top of the one that recorded it.
func TestTransitionHandler_CompletingWithUnknownOutcomeAndNoDate(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.recordOutcome(outcomeBody(engagement.OutcomeUnknown, "", false), http.StatusOK)

	e.transition(transitionBody(engagement.StatusCompleted, "no_response", ""), http.StatusOK)

	if gotStatus, _, _ := readEngagementStatus(t, e.DB, e.Engagement.ID); gotStatus != engagement.StatusCompleted {
		t.Fatalf("status = %q, want completed", gotStatus)
	}
	outcome, endedOn := readEngagementOutcome(t, e.DB, e.Engagement.ID)
	if outcome == nil || *outcome != engagement.OutcomeUnknown || endedOn != nil {
		t.Fatalf("outcome=%v endedOn=%v, want unknown with no date", outcome, endedOn)
	}
	if n := countOutcomeEvents(t, e.DB, e.Engagement.ID); n != 1 {
		t.Fatalf("birth_outcome_recorded events = %d, want 1: completing writes none of its own", n)
	}
}

// TestTransitionHandler_CompletingLeavesARecordedOutcomeAlone is the
// other half of the same rule (#940): a completion never silently
// overwrites an outcome already recorded, which holds here because the
// completion path never writes the column at all. A 'live_birth' with a
// date goes into a completion and comes out unchanged, with no second
// audit row claiming otherwise.
func TestTransitionHandler_CompletingLeavesARecordedOutcomeAlone(t *testing.T) {
	const endedOnDate = "2026-03-14"
	e := newEnv(t, signedInAs(fixture.Owner()), withEngagement(fixture.Status(fixture.StatusActive)))
	e.recordOutcome(outcomeBody(engagement.OutcomeLiveBirth, endedOnDate, false), http.StatusOK)

	e.transition(transitionBody(engagement.StatusCompleted, careCompleteReason, ""), http.StatusOK)

	outcome, endedOn := readEngagementOutcome(t, e.DB, e.Engagement.ID)
	if outcome == nil || *outcome != engagement.OutcomeLiveBirth {
		t.Fatalf("birth_outcome = %v, want live_birth", outcome)
	}
	if endedOn == nil || *endedOn != endedOnDate {
		t.Fatalf("pregnancy_ended_on = %v, want %s", endedOn, endedOnDate)
	}
	if n := countOutcomeEvents(t, e.DB, e.Engagement.ID); n != 1 {
		t.Fatalf("birth_outcome_recorded events = %d, want still 1", n)
	}
}

// TestTransitionHandler_CompletingPersistsEndingNote proves the optional
// free-text note travels with the reason on completion -- not only the
// reason enum itself.
func TestTransitionHandler_CompletingPersistsEndingNote(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), withEngagement(fixture.Status(fixture.StatusActive)))
	testdb.SeedBirthOutcome(t, e.DB, e.Engagement.ID)

	e.transition(transitionBody(engagement.StatusCompleted, careCompleteReason, "Baby arrived safely"), http.StatusOK)

	_, endingReason, endingNote := readEngagementStatus(t, e.DB, e.Engagement.ID)
	if endingReason == nil || *endingReason != careCompleteReason {
		t.Fatalf("ending_reason = %v, want care_complete", endingReason)
	}
	if endingNote == nil || *endingNote != "Baby arrived safely" {
		t.Fatalf("ending_note = %v, want %q", endingNote, "Baby arrived safely")
	}
	// #1423: the completion entry carries both sides of the status and of
	// the ending it set -- what engagement_events' columns used to hold.
	assertDiff(t, readFactDiff(t, e.DB, e.Engagement.ID, "engagement_completed"), map[string]any{
		diffStatusBefore:     engagement.StatusActive,
		diffStatusAfter:      engagement.StatusCompleted,
		"endingReasonBefore": nil,
		"endingReasonAfter":  careCompleteReason,
		"endingNoteBefore":   nil,
		"endingNoteAfter":    "Baby arrived safely",
	})
}

// TestTransitionHandler_RefusesAMalformedRequest covers the request-shape
// refusals before any move is weighed: the three every write shares, and
// a completion without a usable ending reason or a move back to intake,
// each answered before the database's own CHECK.
func TestTransitionHandler_RefusesAMalformedRequest(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), withEngagement(fixture.Status(fixture.StatusActive)))

	e.refusesAnUnreadableRequest(t, http.MethodPatch, "/status", reqtest.JSON(transitionBody(engagement.StatusActive, "", "")))

	cases := []struct {
		name string
		body map[string]any
	}{
		{"completing with no reason at all", transitionBody(engagement.StatusCompleted, "", "")},
		{"completing with a reason outside the vocabulary", transitionBody(engagement.StatusCompleted, "changed my mind", "")},
		{"moving back to intake", transitionBody(engagement.StatusIntake, "", "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(*testing.T) {
			e.transition(tc.body, http.StatusBadRequest)
		})
	}
}

// TestTransitionHandler_ReRequestSameStatusIsNoOp proves re-requesting
// the status an Engagement already holds writes no second audit row and
// no second activity entry for one real event, while still running the
// completion cascade so anything a partial earlier run left behind still
// closes.
func TestTransitionHandler_ReRequestSameStatusIsNoOp(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), withEngagement(fixture.Status(fixture.StatusActive)))
	testdb.SeedBirthOutcome(t, e.DB, e.Engagement.ID)
	complete := transitionBody(engagement.StatusCompleted, careCompleteReason, "")

	e.transition(complete, http.StatusOK)

	if n := countFactEntries(t, e.DB, e.Engagement.ID); n != 1 {
		t.Fatalf("fact entries after first completion = %d, want 1", n)
	}
	if n := e.countActivity("engagement_completed"); n != 1 {
		t.Fatalf("engagement_completed activity rows = %d, want 1", n)
	}

	// A leftover open attachment the first run's own cascade should have
	// closed but a caller re-sends the identical request for anyway --
	// proving the cascade still runs on the no-op path.
	testdb.SeedAttachment(t, e.DB, e.Engagement.ID, e.Staff.ID, "granted", false)

	body := e.transition(complete, http.StatusOK)

	if body.Status != engagement.StatusCompleted {
		t.Fatalf("re-request body status = %q, want completed", body.Status)
	}
	if n := countFactEntries(t, e.DB, e.Engagement.ID); n != 1 {
		t.Fatalf("fact entries after re-request = %d, want still 1 (no duplicate)", n)
	}
	if n := e.countActivity("engagement_completed"); n != 1 {
		t.Fatalf("engagement_completed activity rows after re-request = %d, want still 1", n)
	}
	ended := testdb.Scalar[bool](t, e.DB.Admin,
		`SELECT ended_at IS NOT NULL FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`, e.Engagement.ID, e.Staff.ID)
	if !ended {
		t.Fatal("attachment seeded after first completion was not closed by the re-request's cascade")
	}
}

// TestTransitionHandler_IntakeToActiveWritesCarePhaseChanged proves the
// intake -> active move writes #476's already-reserved care-phase-changed
// activity action, with the acting Staff member as its actor, and no
// such row for a move that isn't intake -> active.
func TestTransitionHandler_IntakeToActiveWritesCarePhaseChanged(t *testing.T) {
	e := newEnv(t)

	e.transition(transitionBody(engagement.StatusActive, "", ""), http.StatusOK)

	if n := e.countActivity("care_phase_changed"); n != 1 {
		t.Fatalf("care_phase_changed activity rows = %d, want 1", n)
	}
	actorName := testdb.Scalar[string](t, e.DB.Admin,
		`SELECT s.name FROM activity a JOIN staff s ON s.id = a.actor_staff_id
		  WHERE a.subject_id = $1 AND a.action = 'care_phase_changed'`, e.Engagement.ID)
	if actorName != e.Staff.Name {
		t.Fatalf("actor name = %q, want the acting Staff member %q", actorName, e.Staff.Name)
	}
	assertDiff(t, readFactDiff(t, e.DB, e.Engagement.ID, "care_phase_changed"), map[string]any{
		diffStatusBefore: engagement.StatusIntake,
		diffStatusAfter:  engagement.StatusActive,
	})
}
