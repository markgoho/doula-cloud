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

// lossOn is the date every test in this file records a loss on, named
// once so golangci-lint's goconst check sees one literal.
const lossOn = "2027-03-04"

// outcomeBody is BirthOutcomeRequest's own JSON shape, built inline so
// this file needs no import of the package's exported request type.
// pregnancyEndedOn is omitted entirely when "", the "absent, not
// empty-string" shape a real caller sends for an undated 'unknown'.
// An outcome of "" sends birthOutcome: null, the un-recording shape.
func outcomeBody(outcome, endedOn string, correction bool) map[string]any {
	body := map[string]any{"birthOutcome": nil}
	if outcome != "" {
		body["birthOutcome"] = outcome
	}
	if endedOn != "" {
		body["pregnancyEndedOn"] = endedOn
	}
	if correction {
		body["correction"] = true
	}
	return body
}

// outcomeResponseBody is both shapes this endpoint answers with: the
// success DTO, and apierr's own error envelope, whose Code a caller is
// required to branch on rather than on the prose beside it
// (docs/api-design.md section 7).
type outcomeResponseBody struct {
	EngagementID     string  `json:"engagementId"`
	BirthOutcome     *string `json:"birthOutcome"`
	PregnancyEndedOn *string `json:"pregnancyEndedOn,omitempty"`
	Code             string  `json:"code"`
}

// recordOutcomeAs PUTs a birth outcome as uid and returns the status
// code and decoded body (zero value on a non-200).
func recordOutcomeAs(t *testing.T, db *testdb.DB, srv *httptest.Server, uid, practiceID, engagementID string, body map[string]any) (int, outcomeResponseBody) {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/birth-outcome", bytes.NewReader(encoded))
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
	var out outcomeResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

// readEngagementOutcome is the raw row every assertion here checks
// against, bypassing the API to prove the database itself agrees.
func readEngagementOutcome(t *testing.T, db *testdb.DB, engagementID string) (outcome, endedOn *string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT birth_outcome::text, pregnancy_ended_on::text FROM engagements WHERE id = $1`, engagementID,
	).Scan(&outcome, &endedOn); err != nil {
		t.Fatalf("read engagement: %v", err)
	}
	return outcome, endedOn
}

// newOutcomeServer mounts this package's whole surface and seeds an
// Engagement in intake at a new Practice, with uid holding roles.
func newOutcomeServer(t *testing.T, db *testdb.DB, uid string, roles []string, employmentType string) (srv *httptest.Server, practiceID, engagementID string) {
	t.Helper()
	practiceID, _ = testdb.SeedStaffAtNewPractice(t, db, uid, roles, employmentType)
	_, engagementID = testdb.SeedEngagementInStatus(t, db, practiceID, "Client", uid+"@example.com", engagement.StatusIntake)
	srv, _ = newServer(t, db, uid)
	t.Cleanup(srv.Close)
	return srv, practiceID, engagementID
}

// TestRecordBirthOutcomeHandler_RolesMayRecord is ADR-0006 as ADR-0015
// applies it: any role that may move a status may record the birth
// outcome, and a contractor Doula -- refused every status move outright
// -- is refused this too.
func TestRecordBirthOutcomeHandler_RolesMayRecord(t *testing.T) {
	cases := []struct {
		kind           string
		roles          []string
		employmentType string
		wantOK         bool
	}{
		{ownerRole, []string{ownerRole}, employeeType, true},
		{adminRole, []string{adminRole}, employeeType, true},
		{employeeDoulaKind, []string{doulaRole}, employeeType, true},
		{"contractor doula", []string{doulaRole}, contractorType, false},
		{"no role", []string{}, employeeType, false},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			db := testdb.New(t)
			uid := "record-outcome-" + tc.kind
			srv, practiceID, engagementID := newOutcomeServer(t, db, uid, tc.roles, tc.employmentType)

			status, body := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID,
				outcomeBody(engagement.OutcomeLoss, lossOn, false))

			if !tc.wantOK {
				if status != http.StatusForbidden {
					t.Fatalf("status = %d, want 403", status)
				}
				gotOutcome, _ := readEngagementOutcome(t, db, engagementID)
				if gotOutcome != nil {
					t.Fatalf("birth_outcome = %v, want it left unrecorded", *gotOutcome)
				}
				return
			}
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200", status)
			}
			if body.BirthOutcome == nil || *body.BirthOutcome != engagement.OutcomeLoss ||
				body.PregnancyEndedOn == nil || *body.PregnancyEndedOn != lossOn {
				t.Fatalf("body = %+v, want loss on %s", body, lossOn)
			}
			gotOutcome, gotEndedOn := readEngagementOutcome(t, db, engagementID)
			if gotOutcome == nil || *gotOutcome != engagement.OutcomeLoss {
				t.Fatalf("birth_outcome = %v, want loss", gotOutcome)
			}
			if gotEndedOn == nil || *gotEndedOn != lossOn {
				t.Fatalf("pregnancy_ended_on = %v, want %s", gotEndedOn, lossOn)
			}
			if n := countEngagementEvents(t, db, engagementID); n != 1 {
				t.Fatalf("engagement_events rows = %d, want 1", n)
			}
		})
	}
}

// TestRecordBirthOutcomeHandler_Validation proves the handler refuses a
// value outside ADR-0015's vocabulary, and enforces
// engagements_outcome_is_dated's "a date unless the outcome is unknown"
// rule itself, before either reaches the database.
func TestRecordBirthOutcomeHandler_Validation(t *testing.T) {
	cases := []struct {
		name    string
		body    map[string]any
		want    int
		wantSet bool
	}{
		{"outside the vocabulary", outcomeBody("stillbirth", lossOn, false), http.StatusBadRequest, false},
		{"a loss with no date", outcomeBody(engagement.OutcomeLoss, "", false), http.StatusBadRequest, false},
		{"a date that is not one", outcomeBody(engagement.OutcomeLoss, "4th March", false), http.StatusBadRequest, false},
		{"unknown needs no date", outcomeBody(engagement.OutcomeUnknown, "", false), http.StatusOK, true},
		{"unknown may carry one", outcomeBody(engagement.OutcomeUnknown, lossOn, false), http.StatusOK, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "outcome-validation-" + tc.name
			srv, practiceID, engagementID := newOutcomeServer(t, db, uid, []string{ownerRole}, employeeType)

			status, _ := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID, tc.body)
			if status != tc.want {
				t.Fatalf("status = %d, want %d", status, tc.want)
			}
			gotOutcome, _ := readEngagementOutcome(t, db, engagementID)
			if (gotOutcome != nil) != tc.wantSet {
				t.Fatalf("birth_outcome = %v, want recorded = %v", gotOutcome, tc.wantSet)
			}
		})
	}
}

// TestRecordBirthOutcomeHandler_FrozenValueRefusesAPlainRecord proves
// ADR-0015's freeze: once recorded, the outcome cannot be overwritten by
// an ordinary record, not even by an Owner. The overwrite is a
// deliberate act or it does not happen.
func TestRecordBirthOutcomeHandler_FrozenValueRefusesAPlainRecord(t *testing.T) {
	db := testdb.New(t)
	const uid = "frozen-outcome-owner"
	srv, practiceID, engagementID := newOutcomeServer(t, db, uid, []string{ownerRole}, employeeType)

	if status, _ := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID,
		outcomeBody(engagement.OutcomeLoss, lossOn, false)); status != http.StatusOK {
		t.Fatalf("first record status = %d, want 200", status)
	}
	status, body := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID,
		outcomeBody(engagement.OutcomeLiveBirth, lossOn, false))
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	// The refusal is a press-through, and #692's rule is that a caller
	// tells it apart by its code, never by its prose -- the other 409
	// this endpoint answers with cannot be pressed through at all.
	if body.Code != string(apierr.CodeBirthOutcomeFrozen) {
		t.Fatalf("code = %q, want %s", body.Code, apierr.CodeBirthOutcomeFrozen)
	}
	gotOutcome, _ := readEngagementOutcome(t, db, engagementID)
	if gotOutcome == nil || *gotOutcome != engagement.OutcomeLoss {
		t.Fatalf("birth_outcome = %v, want the frozen loss", gotOutcome)
	}
	if n := countEngagementEvents(t, db, engagementID); n != 1 {
		t.Fatalf("engagement_events rows = %d, want 1 -- the refusal writes nothing", n)
	}
}

// TestRecordBirthOutcomeHandler_OnlyAnOwnerCorrects walks ADR-0015's
// correction hatch: an Admin and an employee Doula are refused it, an
// Owner passes through it, and the correction lands both sides of both
// facts on engagement_events.
func TestRecordBirthOutcomeHandler_OnlyAnOwnerCorrects(t *testing.T) {
	correctors := []struct {
		kind   string
		roles  []string
		wantOK bool
	}{
		{ownerRole, []string{ownerRole}, true},
		{adminRole, []string{adminRole}, false},
		{employeeDoulaKind, []string{doulaRole}, false},
	}
	for _, tc := range correctors {
		t.Run(tc.kind, func(t *testing.T) {
			db := testdb.New(t)
			uid := "correct-outcome-" + tc.kind
			srv, practiceID, engagementID := newOutcomeServer(t, db, uid, tc.roles, employeeType)
			if _, err := db.Admin.ExecContext(t.Context(),
				`UPDATE engagements SET birth_outcome = 'loss', pregnancy_ended_on = $2::date WHERE id = $1`,
				engagementID, lossOn); err != nil {
				t.Fatalf("seed frozen outcome: %v", err)
			}

			status, _ := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID,
				outcomeBody(engagement.OutcomeLiveBirth, "2027-03-05", true))

			if !tc.wantOK {
				if status != http.StatusForbidden {
					t.Fatalf("status = %d, want 403", status)
				}
				gotOutcome, _ := readEngagementOutcome(t, db, engagementID)
				if gotOutcome == nil || *gotOutcome != engagement.OutcomeLoss {
					t.Fatalf("birth_outcome = %v, want the frozen loss", gotOutcome)
				}
				return
			}
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200", status)
			}
			gotOutcome, gotEndedOn := readEngagementOutcome(t, db, engagementID)
			if gotOutcome == nil || *gotOutcome != engagement.OutcomeLiveBirth {
				t.Fatalf("birth_outcome = %v, want live_birth", gotOutcome)
			}
			if gotEndedOn == nil || *gotEndedOn != "2027-03-05" {
				t.Fatalf("pregnancy_ended_on = %v, want the corrected date", gotEndedOn)
			}
			assertOutcomeEvent(t, db, engagementID, engagement.OutcomeLoss, engagement.OutcomeLiveBirth, lossOn, "2027-03-05")
		})
	}
}

// assertOutcomeEvent reads the single 'birth_outcome_recorded' row this
// Engagement's correction wrote and checks both sides of both facts,
// plus that a human actor is on it -- the cross-cutting audit
// expectation "how did this thing come to be?".
func assertOutcomeEvent(t *testing.T, db *testdb.DB, engagementID, prevOutcome, outcome, prevEndedOn, endedOn string) {
	t.Helper()
	var gotPrevOutcome, gotOutcome, gotPrevEndedOn, gotEndedOn string
	var actor *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT previous_birth_outcome::text, birth_outcome::text,
		        previous_pregnancy_ended_on::text, pregnancy_ended_on::text, actor_staff_id::text
		   FROM engagement_events
		  WHERE engagement_id = $1 AND previous_birth_outcome IS NOT NULL`, engagementID,
	).Scan(&gotPrevOutcome, &gotOutcome, &gotPrevEndedOn, &gotEndedOn, &actor); err != nil {
		t.Fatalf("read correction event: %v", err)
	}
	if gotPrevOutcome != prevOutcome || gotOutcome != outcome {
		t.Fatalf("event outcome = %q -> %q, want %q -> %q", gotPrevOutcome, gotOutcome, prevOutcome, outcome)
	}
	if gotPrevEndedOn != prevEndedOn || gotEndedOn != endedOn {
		t.Fatalf("event date = %q -> %q, want %q -> %q", gotPrevEndedOn, gotEndedOn, prevEndedOn, endedOn)
	}
	if actor == nil {
		t.Fatal("actor_staff_id is null, want the Owner who corrected it")
	}
}

// TestRecordBirthOutcomeHandler_ResendingIsANoOp proves the endpoint is
// idempotent by construction, which is what lets it be a PUT carrying no
// Idempotency-Key: the same values again write nothing and add no second
// audit row.
func TestRecordBirthOutcomeHandler_ResendingIsANoOp(t *testing.T) {
	db := testdb.New(t)
	const uid = "resend-outcome-owner"
	srv, practiceID, engagementID := newOutcomeServer(t, db, uid, []string{ownerRole}, employeeType)

	for range 2 {
		status, body := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID,
			outcomeBody(engagement.OutcomeLiveBirth, lossOn, false))
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		if body.BirthOutcome == nil || *body.BirthOutcome != engagement.OutcomeLiveBirth {
			t.Fatalf("birthOutcome = %v, want live_birth", body.BirthOutcome)
		}
	}
	if n := countEngagementEvents(t, db, engagementID); n != 1 {
		t.Fatalf("engagement_events rows = %d, want 1 -- the resend writes nothing", n)
	}
}

// TestRecordBirthOutcomeHandler_ResendingAnUndatedUnknownIsANoOp covers
// the same no-op on the one shape where both dates are null, which the
// dated comparison cannot reach.
func TestRecordBirthOutcomeHandler_ResendingAnUndatedUnknownIsANoOp(t *testing.T) {
	db := testdb.New(t)
	const uid = "resend-unknown-owner"
	srv, practiceID, engagementID := newOutcomeServer(t, db, uid, []string{ownerRole}, employeeType)

	for range 2 {
		if status, _ := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID,
			outcomeBody(engagement.OutcomeUnknown, "", false)); status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
	}
	if n := countEngagementEvents(t, db, engagementID); n != 1 {
		t.Fatalf("engagement_events rows = %d, want 1", n)
	}
}

// TestRecordBirthOutcomeHandler_CorrectingAnUnrecordedOutcome proves the
// correction flag is refused where there is nothing to correct, so the
// deliberate act never becomes a caller's default.
func TestRecordBirthOutcomeHandler_CorrectingAnUnrecordedOutcome(t *testing.T) {
	db := testdb.New(t)
	const uid = "correct-nothing-owner"
	srv, practiceID, engagementID := newOutcomeServer(t, db, uid, []string{ownerRole}, employeeType)

	status, body := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID,
		outcomeBody(engagement.OutcomeLoss, lossOn, true))
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	if body.Code == string(apierr.CodeBirthOutcomeFrozen) {
		t.Fatalf("code = %s, want the plain conflict -- nothing here can be pressed through", body.Code)
	}
}

// TestRecordBirthOutcomeHandler_UnknownEngagement covers both doors an
// engagement id can fail at: one that is not a UUID at all, and a
// well-formed one belonging to nobody this caller can reach.
func TestRecordBirthOutcomeHandler_UnknownEngagement(t *testing.T) {
	cases := []struct {
		name         string
		engagementID string
		want         int
	}{
		{"not a uuid", "not-a-uuid", http.StatusBadRequest},
		{"no such engagement", "00000000-0000-0000-0000-000000000000", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "missing-engagement-" + tc.name
			srv, practiceID, _ := newOutcomeServer(t, db, uid, []string{ownerRole}, employeeType)

			status, _ := recordOutcomeAs(t, db, srv, uid, practiceID, tc.engagementID,
				outcomeBody(engagement.OutcomeLoss, lossOn, false))
			if status != tc.want {
				t.Fatalf("status = %d, want %d", status, tc.want)
			}
		})
	}
}

// TestRecordBirthOutcomeHandler_MalformedBody covers the decode refusal.
func TestRecordBirthOutcomeHandler_MalformedBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "malformed-outcome-owner"
	srv, practiceID, engagementID := newOutcomeServer(t, db, uid, []string{ownerRole}, employeeType)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/birth-outcome",
		bytes.NewReader([]byte("{")))
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
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestDetailHandler_BirthOutcome proves the Staff-side read carries both
// facts once recorded, and omits both while they are not -- the same
// `omitempty` shape Detail.DueDate already uses, so a page can tell
// "nothing recorded" apart from a fetch that broke.
func TestDetailHandler_BirthOutcome(t *testing.T) {
	db := testdb.New(t)
	const uid = "detail-birth-outcome"
	srv, practiceID, engagementID := newOutcomeServer(t, db, uid, []string{ownerRole}, employeeType)
	detailURL := srv.URL + "/api/practices/" + practiceID + "/engagements/" + engagementID

	resp := authedGet(t, authntest.SeedSession(t, db.App, uid), detailURL)
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	_ = resp.Body.Close()
	for _, key := range []string{"birthOutcome", "pregnancyEndedOn"} {
		if _, present := raw[key]; present {
			t.Fatalf("%q present before anything was recorded, want omitted", key)
		}
	}

	if status, _ := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID,
		outcomeBody(engagement.OutcomeLoss, lossOn, false)); status != http.StatusOK {
		t.Fatalf("record status = %d, want 200", status)
	}

	resp = authedGet(t, authntest.SeedSession(t, db.App, uid), detailURL)
	defer func() { _ = resp.Body.Close() }()
	var d engagement.Detail
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if d.BirthOutcome == nil || *d.BirthOutcome != engagement.OutcomeLoss {
		t.Fatalf("birthOutcome = %v, want loss", d.BirthOutcome)
	}
	if d.PregnancyEndedOn == nil || *d.PregnancyEndedOn != lossOn {
		t.Fatalf("pregnancyEndedOn = %v, want %s", d.PregnancyEndedOn, lossOn)
	}
}

// TestRecordBirthOutcomeHandler_AnOwnerCanUnrecord is ADR-0015's own
// motivating typo: a loss entered on the wrong Engagement. The right
// state for that Engagement is never-recorded, not 'unknown' -- which
// means the Practice looked and never learned -- so the correction hatch
// takes a null outcome, and the audit row says what was cleared.
func TestRecordBirthOutcomeHandler_AnOwnerCanUnrecord(t *testing.T) {
	db := testdb.New(t)
	const uid = "unrecord-owner"
	srv, practiceID, engagementID := newOutcomeServer(t, db, uid, []string{ownerRole}, employeeType)
	if status, _ := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID,
		outcomeBody(engagement.OutcomeLoss, lossOn, false)); status != http.StatusOK {
		t.Fatalf("first record status = %d, want 200", status)
	}

	status, body := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID, outcomeBody("", "", true))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if body.BirthOutcome != nil {
		t.Fatalf("birthOutcome = %v, want null", *body.BirthOutcome)
	}
	gotOutcome, gotEndedOn := readEngagementOutcome(t, db, engagementID)
	if gotOutcome != nil || gotEndedOn != nil {
		t.Fatalf("row = (%v, %v), want both cleared", gotOutcome, gotEndedOn)
	}
	if n := countEngagementEvents(t, db, engagementID); n != 2 {
		t.Fatalf("engagement_events rows = %d, want 2 -- the record and the un-recording", n)
	}
}

// TestRecordBirthOutcomeHandler_ClearingIsAlwaysACorrection proves a
// null outcome is refused unless the caller says it is a correction, and
// that it cannot carry a date -- engagements_outcome_is_dated's own rule,
// answered as a named 400 rather than a constraint violation.
func TestRecordBirthOutcomeHandler_ClearingIsAlwaysACorrection(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
	}{
		{"no correction flag", outcomeBody("", "", false)},
		{"a date with no outcome", outcomeBody("", lossOn, true)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "clear-refused-" + tc.name
			srv, practiceID, engagementID := newOutcomeServer(t, db, uid, []string{ownerRole}, employeeType)
			if status, _ := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID,
				outcomeBody(engagement.OutcomeLoss, lossOn, false)); status != http.StatusOK {
				t.Fatalf("first record status = %d, want 200", status)
			}

			status, _ := recordOutcomeAs(t, db, srv, uid, practiceID, engagementID, tc.body)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", status)
			}
			gotOutcome, _ := readEngagementOutcome(t, db, engagementID)
			if gotOutcome == nil {
				t.Fatal("birth_outcome was cleared, want the frozen loss left alone")
			}
		})
	}
}
