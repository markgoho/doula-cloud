package engagement_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

type transitionResponseBody struct {
	EngagementID string   `json:"engagementId"`
	Status       string   `json:"status"`
	StatusMoves  []string `json:"statusMoves"`
}

// transitionAs sends a status transition request as uid and returns its
// status code and decoded body (zero value on a non-200).
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
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return resp.StatusCode, out
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

// countEngagementEvents counts engagement_events rows for engagementID,
// proving TransitionHandler wrote exactly one (or zero, for a no-op).
func countEngagementEvents(t *testing.T, db *testdb.DB, engagementID string) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM engagement_events WHERE engagement_id = $1`, engagementID,
	).Scan(&n); err != nil {
		t.Fatalf("count engagement_events: %v", err)
	}
	return n
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
		{"owner", []string{ownerRole}, employeeType},
		{"admin", []string{adminRole}, employeeType},
		{"employee doula", []string{doulaRole}, employeeType},
		{"contractor doula", []string{doulaRole}, "contractor"},
	}

	for _, move := range moves {
		for _, rk := range roleKinds {
			t.Run(move.name+"/"+rk.kind, func(t *testing.T) {
				db := testdb.New(t)
				uid := "legal-moves-" + move.name + "-" + rk.kind
				practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, rk.roles, rk.employmentType)
				_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", uid+"@example.com", move.from)
				srv := newTransitionServer(t, db)

				isReopen := move.from == engagement.StatusCompleted && move.to == engagement.StatusActive
				wantOK := rk.employmentType != "contractor" && (!isReopen || rk.kind != "employee doula")

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
}

// TestTransitionHandler_CompletingPersistsEndingNote proves the optional
// free-text note travels with the reason on completion -- not only the
// reason enum itself.
func TestTransitionHandler_CompletingPersistsEndingNote(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "note-owner", []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "note@example.com", engagement.StatusActive)
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
// the request-shape refusals: an unrecognised target status, an
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
// decode at all -- not just one with an unrecognised status -- is
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
	srv := newTransitionServer(t, db)

	status, _ := transitionAs(t, db, srv, "noop-owner", practiceID, engagementID, transitionBody(engagement.StatusCompleted, careCompleteReason, ""))
	if status != http.StatusOK {
		t.Fatalf("first completion status = %d, want 200", status)
	}
	if n := countEngagementEvents(t, db, engagementID); n != 1 {
		t.Fatalf("engagement_events after first completion = %d, want 1", n)
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
	if n := countEngagementEvents(t, db, engagementID); n != 1 {
		t.Fatalf("engagement_events after re-request = %d, want still 1 (no duplicate)", n)
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
