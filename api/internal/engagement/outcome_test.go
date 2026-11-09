package engagement_test

import (
	"encoding/json/jsontext"
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
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
//
// Stays a struct of its own rather than reaching for apierrtest.Decode,
// #811's one reader of the envelope, for the reason
// transitionResponseBody records: one decode has to serve both shapes.
type outcomeResponseBody struct {
	EngagementID     string      `json:"engagementId"`
	BirthOutcome     *string     `json:"birthOutcome"`
	PregnancyEndedOn *string     `json:"pregnancyEndedOn,omitempty"`
	Code             apierr.Code `json:"code"`
	Message          string      `json:"message"`
}

// recordOutcome PUTs a birth outcome for the Engagement as the Staff
// member at the screen and requires want.
func (e *env) recordOutcome(body map[string]any, want int) outcomeResponseBody {
	e.T.Helper()
	return e.recordOutcomeOf(e.Engagement.ID, body, want)
}

func (e *env) recordOutcomeOf(engagementID string, body map[string]any, want int) outcomeResponseBody {
	e.T.Helper()
	return reqtest.Expect[outcomeResponseBody](e.T, http.MethodPut, e.urlOf(engagementID, "/birth-outcome"), want, e.SignedIn, reqtest.JSON(body))
}

// outcome is the recorded outcome and date, read past the API.
func (e *env) outcome() (outcome, endedOn *string) {
	e.T.Helper()
	return readEngagementOutcome(e.T, e.DB, e.Engagement.ID)
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

// An Owner at the screen, with an intake Engagement: what almost every
// test below starts from.
func newOwnerEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	return newEnv(t, append([]envOption{signedInAs(fixture.Owner())}, opts...)...)
}

// TestRecordBirthOutcomeHandler_RolesMayRecord is ADR-0006 as ADR-0015
// applies it: any role that may move a status may record the birth
// outcome, and a contractor Doula -- refused every status move outright
// -- is refused this too.
func TestRecordBirthOutcomeHandler_RolesMayRecord(t *testing.T) {
	cases := []struct {
		role   roleCase
		wantOK bool
	}{
		{ownerCase, true},
		{adminCase, true},
		{employeeDoulaCase, true},
		{contractorDoulaCase, false},
		{noRoleCase, false},
	}
	for _, tc := range cases {
		t.Run(tc.role.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.role.opts...))

			body := e.recordOutcome(outcomeBody(engagement.OutcomeLoss, lossOn, false), okOrForbidden(tc.wantOK))

			gotOutcome, gotEndedOn := e.outcome()
			if !tc.wantOK {
				if gotOutcome != nil {
					t.Fatalf("birth_outcome = %v, want it left unrecorded", *gotOutcome)
				}
				return
			}
			if body.BirthOutcome == nil || *body.BirthOutcome != engagement.OutcomeLoss ||
				body.PregnancyEndedOn == nil || *body.PregnancyEndedOn != lossOn {
				t.Fatalf("body = %+v, want loss on %s", body, lossOn)
			}
			if gotOutcome == nil || *gotOutcome != engagement.OutcomeLoss {
				t.Fatalf("birth_outcome = %v, want loss", gotOutcome)
			}
			if gotEndedOn == nil || *gotEndedOn != lossOn {
				t.Fatalf("pregnancy_ended_on = %v, want %s", gotEndedOn, lossOn)
			}
			if n := countFactEntries(t, e.DB, e.Engagement.ID); n != 1 {
				t.Fatalf("fact entries = %d, want 1", n)
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
			e := newOwnerEnv(t)

			e.recordOutcome(tc.body, tc.want)

			if gotOutcome, _ := e.outcome(); (gotOutcome != nil) != tc.wantSet {
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
	e := newOwnerEnv(t)
	e.recordOutcome(outcomeBody(engagement.OutcomeLoss, lossOn, false), http.StatusOK)

	body := e.recordOutcome(outcomeBody(engagement.OutcomeLiveBirth, lossOn, false), http.StatusConflict)

	// The refusal is a press-through, and #692's rule is that a caller
	// tells it apart by its code, never by its prose -- the other 409
	// this endpoint answers with cannot be pressed through at all.
	if body.Code != apierr.CodeBirthOutcomeFrozen {
		t.Fatalf("code = %q, want %s", body.Code, apierr.CodeBirthOutcomeFrozen)
	}
	if gotOutcome, _ := e.outcome(); gotOutcome == nil || *gotOutcome != engagement.OutcomeLoss {
		t.Fatalf("birth_outcome = %v, want the frozen loss", gotOutcome)
	}
	if n := countFactEntries(t, e.DB, e.Engagement.ID); n != 1 {
		t.Fatalf("fact entries = %d, want 1 -- the refusal writes nothing", n)
	}
}

// TestRecordBirthOutcomeHandler_OnlyAnOwnerCorrects walks ADR-0015's
// correction hatch: an Admin and an employee Doula are refused it, an
// Owner passes through it, and the correction lands both sides of both
// facts on its birth_outcome_recorded activity entry.
func TestRecordBirthOutcomeHandler_OnlyAnOwnerCorrects(t *testing.T) {
	correctors := []struct {
		role   roleCase
		wantOK bool
	}{
		{ownerCase, true},
		{adminCase, false},
		{employeeDoulaCase, false},
	}
	for _, tc := range correctors {
		t.Run(tc.role.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.role.opts...))
			e.Exec(`UPDATE engagements SET birth_outcome = 'loss', pregnancy_ended_on = $2::date WHERE id = $1`, e.Engagement.ID, lossOn)

			e.recordOutcome(outcomeBody(engagement.OutcomeLiveBirth, "2027-03-05", true), okOrForbidden(tc.wantOK))

			gotOutcome, gotEndedOn := e.outcome()
			if !tc.wantOK {
				if gotOutcome == nil || *gotOutcome != engagement.OutcomeLoss {
					t.Fatalf("birth_outcome = %v, want the frozen loss", gotOutcome)
				}
				return
			}
			if gotOutcome == nil || *gotOutcome != engagement.OutcomeLiveBirth {
				t.Fatalf("birth_outcome = %v, want live_birth", gotOutcome)
			}
			if gotEndedOn == nil || *gotEndedOn != "2027-03-05" {
				t.Fatalf("pregnancy_ended_on = %v, want the corrected date", gotEndedOn)
			}
			assertOutcomeEvent(t, e.DB, e.Engagement.ID, engagement.OutcomeLoss, engagement.OutcomeLiveBirth, lossOn, "2027-03-05")
		})
	}
}

// assertOutcomeEvent reads the most recent birth_outcome_recorded
// activity entry and checks both sides of both facts, plus that a human
// actor is on it -- the cross-cutting audit expectation "how did this
// thing come to be?".
func assertOutcomeEvent(t *testing.T, db *testdb.DB, engagementID, prevOutcome, outcome, prevEndedOn, endedOn string) {
	t.Helper()
	assertDiff(t, readFactDiff(t, db, engagementID, "birth_outcome_recorded"), map[string]any{
		"birthOutcomeBefore":     prevOutcome,
		"birthOutcomeAfter":      outcome,
		"pregnancyEndedOnBefore": prevEndedOn,
		"pregnancyEndedOnAfter":  endedOn,
	})
}

// TestRecordBirthOutcomeHandler_ResendingIsANoOp proves the endpoint is
// idempotent by construction, which is what lets it be a PUT carrying no
// Idempotency-Key: the same values again write nothing and add no second
// audit row. An undated 'unknown' is its own row, because it is the one
// shape where both dates are null, which the dated comparison cannot reach.
func TestRecordBirthOutcomeHandler_ResendingIsANoOp(t *testing.T) {
	cases := []struct {
		name    string
		outcome string
		endedOn string
	}{
		{"a dated live birth", engagement.OutcomeLiveBirth, lossOn},
		{"an undated unknown", engagement.OutcomeUnknown, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newOwnerEnv(t)

			for range 2 {
				body := e.recordOutcome(outcomeBody(tc.outcome, tc.endedOn, false), http.StatusOK)
				if body.BirthOutcome == nil || *body.BirthOutcome != tc.outcome {
					t.Fatalf("birthOutcome = %v, want %s", body.BirthOutcome, tc.outcome)
				}
			}

			if n := countFactEntries(t, e.DB, e.Engagement.ID); n != 1 {
				t.Fatalf("fact entries = %d, want 1 -- the resend writes nothing", n)
			}
		})
	}
}

// TestRecordBirthOutcomeHandler_CorrectingAnUnrecordedOutcome proves the
// correction flag is refused where there is nothing to correct, so the
// deliberate act never becomes a caller's default.
func TestRecordBirthOutcomeHandler_CorrectingAnUnrecordedOutcome(t *testing.T) {
	e := newOwnerEnv(t)

	body := e.recordOutcome(outcomeBody(engagement.OutcomeLoss, lossOn, true), http.StatusConflict)

	if body.Code == apierr.CodeBirthOutcomeFrozen {
		t.Fatalf("code = %s, want the plain conflict -- nothing here can be pressed through", body.Code)
	}
}

// TestRecordBirthOutcomeHandler_RefusesARequestItCannotRead covers the
// doors a request can fail at before any outcome is weighed.
func TestRecordBirthOutcomeHandler_RefusesARequestItCannotRead(t *testing.T) {
	e := newOwnerEnv(t)

	e.refusesAnUnreadableRequest(t, http.MethodPut, "/birth-outcome", reqtest.JSON(outcomeBody(engagement.OutcomeLoss, lossOn, false)))
}

// TestDetailHandler_BirthOutcome proves the Staff-side read carries both
// facts once recorded, and omits both while they are not -- the same
// `omitempty` shape Detail.DueDate already uses, so a page can tell
// "nothing recorded" apart from a fetch that broke.
func TestDetailHandler_BirthOutcome(t *testing.T) {
	e := newOwnerEnv(t)

	raw := reqtest.Expect[map[string]jsontext.Value](t, http.MethodGet, e.engagementURL(""), http.StatusOK, e.SignedIn)
	for _, key := range []string{"birthOutcome", "pregnancyEndedOn"} {
		if _, present := raw[key]; present {
			t.Fatalf("%q present before anything was recorded, want omitted", key)
		}
	}

	e.recordOutcome(outcomeBody(engagement.OutcomeLoss, lossOn, false), http.StatusOK)

	d := e.detail()
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
	e := newOwnerEnv(t)
	e.recordOutcome(outcomeBody(engagement.OutcomeLoss, lossOn, false), http.StatusOK)

	body := e.recordOutcome(outcomeBody("", "", true), http.StatusOK)

	if body.BirthOutcome != nil {
		t.Fatalf("birthOutcome = %v, want null", *body.BirthOutcome)
	}
	gotOutcome, gotEndedOn := e.outcome()
	if gotOutcome != nil || gotEndedOn != nil {
		t.Fatalf("row = (%v, %v), want both cleared", gotOutcome, gotEndedOn)
	}
	if n := countFactEntries(t, e.DB, e.Engagement.ID); n != 2 {
		t.Fatalf("fact entries = %d, want 2 -- the record and the un-recording", n)
	}
	assertDiff(t, readFactDiff(t, e.DB, e.Engagement.ID, "birth_outcome_recorded"), map[string]any{
		"birthOutcomeBefore":     engagement.OutcomeLoss,
		"birthOutcomeAfter":      nil,
		"pregnancyEndedOnBefore": lossOn,
		"pregnancyEndedOnAfter":  nil,
	})
}

// TestRecordBirthOutcomeHandler_ClearingIsRefusedOnACompletedEngagement
// is the other side of #940's constraint: once an Engagement is
// 'completed', engagements_completed_is_explained (00094) forbids a row
// that carries no outcome, so the un-recording above cannot be applied
// where it would leave one. The caller is told the order that works
// rather than meeting the raw constraint as a 500, and the recorded pair
// is left exactly as it was.
func TestRecordBirthOutcomeHandler_ClearingIsRefusedOnACompletedEngagement(t *testing.T) {
	e := newOwnerEnv(t)
	e.recordOutcome(outcomeBody(engagement.OutcomeLoss, lossOn, false), http.StatusOK)
	e.Exec(`UPDATE engagements SET status = 'completed', ending_reason = 'care_complete' WHERE id = $1`, e.Engagement.ID)

	body := e.recordOutcome(outcomeBody("", "", true), http.StatusConflict)

	// #2308: the hub shows these words as written, so they are a sentence
	// for that screen, the same as the section's own line before it asks.
	if body.Code != apierr.CodeBirthOutcomeRequired {
		t.Fatalf("code = %q, want %s", body.Code, apierr.CodeBirthOutcomeRequired)
	}
	if body.Message != engagement.MsgOutcomeKeptOnCompleted {
		t.Fatalf("message = %q, want %q", body.Message, engagement.MsgOutcomeKeptOnCompleted)
	}
	gotOutcome, gotEndedOn := e.outcome()
	if gotOutcome == nil || *gotOutcome != engagement.OutcomeLoss || gotEndedOn == nil {
		t.Fatalf("row = (%v, %v), want the recorded loss left alone", gotOutcome, gotEndedOn)
	}
	// Correcting in place is untouched by that refusal: only the null
	// case can leave a 'completed' row with no outcome at all.
	e.recordOutcome(outcomeBody(engagement.OutcomeUnknown, "", true), http.StatusOK)
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
			e := newOwnerEnv(t)
			e.recordOutcome(outcomeBody(engagement.OutcomeLoss, lossOn, false), http.StatusOK)

			e.recordOutcome(tc.body, http.StatusBadRequest)

			if gotOutcome, _ := e.outcome(); gotOutcome == nil {
				t.Fatal("birth_outcome was cleared, want the frozen loss left alone")
			}
		})
	}
}
