package engagement_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
)

// newTransitionServer mounts this package's whole surface, matching the
// old newCompleteServer's shape.
func newTransitionServer(t *testing.T, db *testdb.DB) *httptest.Server {
	t.Helper()
	srv, _ := newServer(t, db, "engagement-transition-mount")
	t.Cleanup(srv.Close)
	return srv
}

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

// transitionAs sends a status transition request as uid and returns its
// status code and decoded body.
func transitionAs(t *testing.T, db *testdb.DB, srv *httptest.Server, uid, practiceID, engagementID string, body map[string]any) (int, transitionResponseBody) {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPatch,
		srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/status", bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	authntest.AddSessionCookie(req, authntest.SeedSession(t, db.App, uid))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out transitionResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
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
		                   'kind_changed', 'birth_outcome_recorded')`, engagementID,
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
// completed Engagement (Owner/Admin only), and a contractor Doula is
// refused every one of them outright.
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
	roleKinds := []struct {
		kind           string
		roles          []string
		employmentType string
	}{
		{ownerRole, []string{ownerRole}, employeeType},
		{adminRole, []string{adminRole}, employeeType},
		{employeeDoulaKind, []string{doulaRole}, employeeType},
		{contractorDoulaKind, []string{doulaRole}, contractorType},
	}

	for _, move := range moves {
		for _, rk := range roleKinds {
			t.Run(move.name+"/"+rk.kind, func(t *testing.T) {
				db := testdb.New(t)
				uid := "legal-moves-" + move.name + "-" + rk.kind
				practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, rk.roles, rk.employmentType)
				_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", uid+"@example.com", move.from)
				if move.to == engagement.StatusCompleted {
					testdb.SeedBirthOutcome(t, db, engagementID)
				}
				srv := newTransitionServer(t, db)

				isReopen := move.from == engagement.StatusCompleted && move.to == engagement.StatusActive
				wantOK := rk.employmentType != contractorType && (!isReopen || rk.kind != employeeDoulaKind)

				status, body := transitionAs(t, db, srv, uid, practiceID, engagementID,
					transitionBody(move.to, move.endingReason, ""))

				if wantOK {
					if status != http.StatusOK {
						t.Fatalf("status = %d, want 200", status)
					}
					if body.Status != move.to {
						t.Fatalf("status in body = %q, want %q", body.Status, move.to)
					}
					gotStatus, _, _ := readEngagementStatus(t, db, engagementID)
					if gotStatus != move.to {
						t.Fatalf("engagements.status = %q, want %q", gotStatus, move.to)
					}
				} else if status != http.StatusForbidden {
					t.Fatalf("status = %d, want 403", status)
				}
			})
		}
	}
}

// TestTransitionHandler_ReopenClearsReasonAndNote proves reopening a
// completed Engagement clears both ending_reason and ending_note, which
// are demanded again at the next completion (ADR-0015).
func TestTransitionHandler_ReopenClearsReasonAndNote(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "reopen-owner", []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "reopen@example.com", engagement.StatusCompleted)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET ending_note = 'left the area' WHERE id = $1`, engagementID); err != nil {
		t.Fatalf("seed ending note: %v", err)
	}
	srv := newTransitionServer(t, db)

	status, _ := transitionAs(t, db, srv, "reopen-owner", practiceID, engagementID, transitionBody(engagement.StatusActive, "", ""))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	gotStatus, endingReason, endingNote := readEngagementStatus(t, db, engagementID)
	if gotStatus != engagement.StatusActive || endingReason != nil || endingNote != nil {
		t.Fatalf("status=%q endingReason=%v endingNote=%v, want active with both cleared", gotStatus, endingReason, endingNote)
	}
	// ADR-0015: "Reopening unfreezes nothing." The two facts the ending
	// carries are cleared and asked for again; the birth outcome is not
	// one of them, and #940 must not have turned reopening into a way to
	// shed it.
	if outcome, _ := readEngagementOutcome(t, db, engagementID); outcome == nil {
		t.Fatal("reopening cleared the birth outcome, which the freeze rule says it never does")
	}
	// #1423: the reopen is one engagement_reopened entry carrying both
	// sides of every fact it moved, the ending it cleared included, and
	// never a completion entry.
	assertDiff(t, readFactDiff(t, db, engagementID, "engagement_reopened"), map[string]any{
		diffStatusBefore:     engagement.StatusCompleted,
		diffStatusAfter:      engagement.StatusActive,
		"endingReasonBefore": careCompleteReason,
		"endingReasonAfter":  nil,
		"endingNoteBefore":   "left the area",
		"endingNoteAfter":    nil,
	})
	if n := countFactEntries(t, db, engagementID); n != 1 {
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
	db := testdb.New(t)
	const uid = "no-outcome-owner"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "no-outcome@example.com", engagement.StatusActive)
	testdb.SeedAttachment(t, db, engagementID, staffID, "granted", false)
	srv := newTransitionServer(t, db)

	status, body := transitionAs(t, db, srv, uid, practiceID, engagementID,
		transitionBody(engagement.StatusCompleted, careCompleteReason, ""))
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	if body.Code != apierr.CodeBirthOutcomeRequired {
		t.Fatalf("code = %q, want %s", body.Code, apierr.CodeBirthOutcomeRequired)
	}
	if body.Message == "" {
		t.Fatal("refusal carried no message")
	}

	gotStatus, endingReason, _ := readEngagementStatus(t, db, engagementID)
	if gotStatus != engagement.StatusActive || endingReason != nil {
		t.Fatalf("status=%q endingReason=%v, want the row untouched", gotStatus, endingReason)
	}
	if n := countFactEntries(t, db, engagementID); n != 0 {
		t.Fatalf("fact entries = %d, want 0 on a refusal", n)
	}
	if n := countActivityActions(t, db, engagementID, "engagement_completed"); n != 0 {
		t.Fatalf("engagement_completed activity rows = %d, want 0 on a refusal", n)
	}
	var openAttachments int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM engagement_attachments WHERE engagement_id = $1 AND ended_at IS NULL`,
		engagementID,
	).Scan(&openAttachments); err != nil {
		t.Fatalf("count attachments: %v", err)
	}
	if openAttachments != 1 {
		t.Fatalf("open attachments = %d, want 1: the completion cascade must not run on a refusal", openAttachments)
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
	db := testdb.New(t)
	const uid = "unknown-outcome-owner"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Vanished Client", "vanished@example.com", engagement.StatusIntake)
	srv := newTransitionServer(t, db)

	if status, _ := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID,
		outcomeBody(engagement.OutcomeUnknown, "", false)); status != http.StatusOK {
		t.Fatalf("recording 'unknown' status = %d, want 200", status)
	}

	status, _ := transitionAs(t, db, srv, uid, practiceID, engagementID,
		transitionBody(engagement.StatusCompleted, "no_response", ""))
	if status != http.StatusOK {
		t.Fatalf("completion status = %d, want 200", status)
	}
	gotStatus, _, _ := readEngagementStatus(t, db, engagementID)
	if gotStatus != engagement.StatusCompleted {
		t.Fatalf("status = %q, want completed", gotStatus)
	}
	outcome, endedOn := readEngagementOutcome(t, db, engagementID)
	if outcome == nil || *outcome != engagement.OutcomeUnknown || endedOn != nil {
		t.Fatalf("outcome=%v endedOn=%v, want unknown with no date", outcome, endedOn)
	}
	if n := countOutcomeEvents(t, db, engagementID); n != 1 {
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
	db := testdb.New(t)
	const uid = "frozen-outcome-owner"
	const endedOnDate = "2026-03-14"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "frozen-complete@example.com", engagement.StatusActive)
	srv := newTransitionServer(t, db)

	if status, _ := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID,
		outcomeBody(engagement.OutcomeLiveBirth, endedOnDate, false)); status != http.StatusOK {
		t.Fatalf("recording status = %d, want 200", status)
	}

	status, _ := transitionAs(t, db, srv, uid, practiceID, engagementID,
		transitionBody(engagement.StatusCompleted, careCompleteReason, ""))
	if status != http.StatusOK {
		t.Fatalf("completion status = %d, want 200", status)
	}
	outcome, endedOn := readEngagementOutcome(t, db, engagementID)
	if outcome == nil || *outcome != engagement.OutcomeLiveBirth {
		t.Fatalf("birth_outcome = %v, want live_birth", outcome)
	}
	if endedOn == nil || *endedOn != endedOnDate {
		t.Fatalf("pregnancy_ended_on = %v, want %s", endedOn, endedOnDate)
	}
	if n := countOutcomeEvents(t, db, engagementID); n != 1 {
		t.Fatalf("birth_outcome_recorded events = %d, want still 1", n)
	}
}

// TestTransitionHandler_CompletingPersistsEndingNote proves the optional
// free-text note travels with the reason on completion -- not only the
// reason enum itself.
func TestTransitionHandler_CompletingPersistsEndingNote(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "note-owner", []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "note@example.com", engagement.StatusActive)
	testdb.SeedBirthOutcome(t, db, engagementID)
	srv := newTransitionServer(t, db)

	status, _ := transitionAs(t, db, srv, "note-owner", practiceID, engagementID,
		transitionBody(engagement.StatusCompleted, careCompleteReason, "Baby arrived safely"))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	_, endingReason, endingNote := readEngagementStatus(t, db, engagementID)
	if endingReason == nil || *endingReason != careCompleteReason {
		t.Fatalf("ending_reason = %v, want care_complete", endingReason)
	}
	if endingNote == nil || *endingNote != "Baby arrived safely" {
		t.Fatalf("ending_note = %v, want %q", endingNote, "Baby arrived safely")
	}
	// #1423: the completion entry carries both sides of the status and of
	// the ending it set -- what engagement_events' columns used to hold.
	assertDiff(t, readFactDiff(t, db, engagementID, "engagement_completed"), map[string]any{
		diffStatusBefore:     engagement.StatusActive,
		diffStatusAfter:      engagement.StatusCompleted,
		"endingReasonBefore": nil,
		"endingReasonAfter":  careCompleteReason,
		"endingNoteBefore":   nil,
		"endingNoteAfter":    "Baby arrived safely",
	})
}

// TestTransitionHandler_CompletingRequiresEndingReason proves the
// handler refuses a request that carries no ending reason, or one
// outside the six-value vocabulary, before it ever reaches the
// database's own CHECK.
func TestTransitionHandler_CompletingRequiresEndingReason(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "reason-owner", []string{ownerRole}, employeeType)
	srv := newTransitionServer(t, db)

	cases := []struct {
		name         string
		endingReason string
	}{
		{"no reason at all", ""},
		{"reason outside the vocabulary", "changed my mind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", tc.name+"@example.com", engagement.StatusActive)
			status, _ := transitionAs(t, db, srv, "reason-owner", practiceID, engagementID,
				transitionBody(engagement.StatusCompleted, tc.endingReason, ""))
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", status)
			}
		})
	}
}

// TestTransitionHandler_RefusesInvalidStatusAndUnknownEngagement covers
// the request-shape refusals: an unrecognized target status, an
// unparseable engagement id, and an engagement id that parses but names
// nothing at this Practice.
func TestTransitionHandler_RefusesInvalidStatusAndUnknownEngagement(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "invalid-owner", []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "invalid@example.com", engagement.StatusIntake)
	srv := newTransitionServer(t, db)

	status, _ := transitionAs(t, db, srv, "invalid-owner", practiceID, engagementID, transitionBody(engagement.StatusIntake, "", ""))
	if status != http.StatusBadRequest {
		t.Fatalf("status(target intake) = %d, want 400", status)
	}
	status, _ = transitionAs(t, db, srv, "invalid-owner", practiceID, "11111111-1111-1111-1111-111111111111",
		transitionBody(engagement.StatusActive, "", ""))
	if status != http.StatusNotFound {
		t.Fatalf("status(unknown engagement) = %d, want 404", status)
	}
	status, _ = transitionAs(t, db, srv, "invalid-owner", practiceID, "not-a-uuid", transitionBody(engagement.StatusActive, "", ""))
	if status != http.StatusBadRequest {
		t.Fatalf("status(bad uuid) = %d, want 400", status)
	}
}

// TestTransitionHandler_RefusesMalformedBody proves a body that fails to
// decode at all -- not just one with an unrecognized status -- is
// refused with apierr.DecodeJSON's own 400.
func TestTransitionHandler_RefusesMalformedBody(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "malformed-owner", []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "malformed@example.com", engagement.StatusIntake)
	srv := newTransitionServer(t, db)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPatch,
		srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/status", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	authntest.AddSessionCookie(req, authntest.SeedSession(t, db.App, "malformed-owner"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestTransitionHandler_ReRequestSameStatusIsNoOp proves re-requesting
// the status an Engagement already holds writes no second audit row and
// no second activity entry for one real event, while still running the
// completion cascade so anything a partial earlier run left behind still
// closes.
func TestTransitionHandler_ReRequestSameStatusIsNoOp(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "noop-owner", []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "noop@example.com", engagement.StatusActive)
	testdb.SeedBirthOutcome(t, db, engagementID)
	srv := newTransitionServer(t, db)

	status, _ := transitionAs(t, db, srv, "noop-owner", practiceID, engagementID, transitionBody(engagement.StatusCompleted, careCompleteReason, ""))
	if status != http.StatusOK {
		t.Fatalf("first completion status = %d, want 200", status)
	}
	if n := countFactEntries(t, db, engagementID); n != 1 {
		t.Fatalf("fact entries after first completion = %d, want 1", n)
	}
	if n := countActivityActions(t, db, engagementID, "engagement_completed"); n != 1 {
		t.Fatalf("engagement_completed activity rows = %d, want 1", n)
	}

	// A leftover open attachment the first run's own cascade should have
	// closed but a caller re-sends the identical request for anyway --
	// proving the cascade still runs on the no-op path.
	var doulaID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT id FROM staff WHERE identity_uid = 'noop-owner'`).Scan(&doulaID); err != nil {
		t.Fatalf("read staff: %v", err)
	}
	testdb.SeedAttachment(t, db, engagementID, doulaID, "granted", false)

	status, body := transitionAs(t, db, srv, "noop-owner", practiceID, engagementID, transitionBody(engagement.StatusCompleted, careCompleteReason, ""))
	if status != http.StatusOK {
		t.Fatalf("re-request status = %d, want 200", status)
	}
	if body.Status != engagement.StatusCompleted {
		t.Fatalf("re-request body status = %q, want completed", body.Status)
	}
	if n := countFactEntries(t, db, engagementID); n != 1 {
		t.Fatalf("fact entries after re-request = %d, want still 1 (no duplicate)", n)
	}
	if n := countActivityActions(t, db, engagementID, "engagement_completed"); n != 1 {
		t.Fatalf("engagement_completed activity rows after re-request = %d, want still 1", n)
	}
	var ended bool
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT ended_at IS NOT NULL FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`,
		engagementID, doulaID,
	).Scan(&ended); err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	if !ended {
		t.Fatal("attachment seeded after first completion was not closed by the re-request's cascade")
	}
}

// TestTransitionHandler_IntakeToActiveWritesCarePhaseChanged proves the
// intake -> active move writes #476's already-reserved care-phase-changed
// activity action, with the acting Staff member as its actor, and no
// such row for a move that isn't intake -> active.
func TestTransitionHandler_IntakeToActiveWritesCarePhaseChanged(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "phase-doula", []string{doulaRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "phase@example.com", engagement.StatusIntake)
	srv := newTransitionServer(t, db)

	status, _ := transitionAs(t, db, srv, "phase-doula", practiceID, engagementID, transitionBody(engagement.StatusActive, "", ""))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if n := countActivityActions(t, db, engagementID, "care_phase_changed"); n != 1 {
		t.Fatalf("care_phase_changed activity rows = %d, want 1", n)
	}
	var actorName string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT s.name FROM activity a JOIN staff s ON s.id = a.actor_staff_id
		  WHERE a.subject_id = $1 AND a.action = 'care_phase_changed'`, engagementID,
	).Scan(&actorName); err != nil {
		t.Fatalf("read activity actor: %v", err)
	}
	if actorName != "Test Staff phase-doula" {
		t.Fatalf("actor name = %q, want the acting Staff member", actorName)
	}
	assertDiff(t, readFactDiff(t, db, engagementID, "care_phase_changed"), map[string]any{
		diffStatusBefore: engagement.StatusIntake,
		diffStatusAfter:  engagement.StatusActive,
	})
}

// TestTransitionHandler_RefusesBareStaffWithNoRole proves a Staff member
// who holds a Membership at the Practice but no role at all -- not in
// ADR-0015's role table -- is refused every move, the same as a
// contractor, rather than falling through to an allowed default.
func TestTransitionHandler_RefusesBareStaffWithNoRole(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "bare-staff", []string{}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "bare@example.com", engagement.StatusIntake)
	srv := newTransitionServer(t, db)

	status, _ := transitionAs(t, db, srv, "bare-staff", practiceID, engagementID, transitionBody(engagement.StatusActive, "", ""))
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", status)
	}
}
