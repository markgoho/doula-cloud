package contracts_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/practicerate"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// noBirthRate is the refusal for a Contract of a kind whose rate is not
// set (#1885): the full sentence, so a reword is a deliberate test change.
const noBirthRate = "Your Practice has no birth rate yet. Set one on the Rates page in Settings before you create a Contract."

// amounting is the body of a PUT that overrides a Contract's amount.
func amounting(amountCents int64) reqtest.Option {
	return reqtest.JSON(contracts.PutContractAmountRequest{AmountCents: amountCents})
}

func (e *env) amountURL() string {
	return e.contractURL("/amount")
}

// TestPostContractHandler_NoRateSet proves #967's AC: creating a Contract
// on a Practice with no rate set for the Engagement's kind fails in a
// way a person can act on, naming the missing kind -- a Contract
// Template exists (so this refusal isn't NoTemplate's), but no
// practice_rates row does.
func TestPostContractHandler_NoRateSet(t *testing.T) {
	e := newEnv(t)
	e.seedTemplate(mergeFieldProse)

	out := apierrtest.Expect(t, http.MethodPost, e.contractURL(""), http.StatusConflict, e.signedIn)

	if out.Code != apierr.CodeFailedPrecondition {
		t.Fatalf("code = %q, want %q", out.Code, apierr.CodeFailedPrecondition)
	}
	if out.Message != noBirthRate {
		t.Fatalf("message = %q, want %q", out.Message, noBirthRate)
	}
}

// TestPostContractHandler_RemovedRateRefusedLikeNoRate proves #2080's AC
// that a new Contract of a kind whose rate an Owner removed is refused
// exactly as it is for a Practice that never set one -- through the real
// DELETE, not a hand-deleted row, so the two packages agree on what "no
// rate" means.
func TestPostContractHandler_RemovedRateRefusedLikeNoRate(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), alsoMounting(practicerate.Mount))
	e.readyToDraft(mergeFieldProse)

	reqtest.ExpectStatus(t, http.MethodDelete, e.srv.URL+"/api/practices/"+e.Practice.ID+"/rates/birth", http.StatusOK, e.signedIn)

	out := apierrtest.Expect(t, http.MethodPost, e.contractURL(""), http.StatusConflict, e.signedIn)
	if out.Message != noBirthRate {
		t.Fatalf("message = %q, want %q", out.Message, noBirthRate)
	}
}

// TestPutContractAmountHandler_RefusedByRole table-drives #967's AC that
// the API refuses a Doula, employee or contractor, and admits an Admin,
// not only an Owner, to override a Contract's amount.
func TestPutContractAmountHandler_RefusedByRole(t *testing.T) {
	cases := []struct {
		name       string
		staff      []fixture.StaffOption
		wantStatus int
	}{
		{caseAdmin, []fixture.StaffOption{fixture.Admin()}, http.StatusOK},
		{"employee doula", []fixture.StaffOption{fixture.Doula()}, http.StatusForbidden},
		{"contractor doula", []fixture.StaffOption{fixture.Doula(), fixture.Contractor()}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.staff...))
			e.seedDraft()

			reqtest.ExpectStatus(t, http.MethodPut, e.amountURL(), tc.wantStatus, e.signedIn, amounting(20000))
		})
	}
}

// TestPutContractAmountHandler_Success proves an Owner's override
// persists, round-trips through GET (the "price" merge field renders the
// new amount), and is recorded in the activity ledger with before/after
// amounts and the acting Staff member.
func TestPutContractAmountHandler_Success(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedDraft()

	out := reqtest.Expect[contracts.ContractAmountResponse](t, http.MethodPut, e.amountURL(), http.StatusOK, e.signedIn, amounting(30000))

	if out.AmountCents != 30000 {
		t.Fatalf("amountCents = %d, want 30000", out.AmountCents)
	}
	got := e.contract()
	if got.Values[priceKey] != "$300.00" {
		t.Fatalf("values[price] after override = %q, want %q", got.Values[priceKey], "$300.00")
	}

	diff := assertActivityActor(t, e.db, e.Engagement.ID, activity.ActionContractAmountOverridden, e.Staff.ID)
	var parsedDiff activity.AmountDiff
	if err := json.Unmarshal(diff, &parsedDiff); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if parsedDiff.AmountCentsBefore != 15000 || parsedDiff.AmountCentsAfter != 30000 {
		t.Fatalf("diff = %+v, want before=15000 after=30000", parsedDiff)
	}

	// #968's AC: an overridden Contract keeps the override and never
	// re-derives when a later rate change comes through -- amount_overridden
	// is the column practicerate.PutRateHandler's reprice pass excludes on.
	overridden := testdb.Scalar[bool](t, e.db.Admin, `SELECT amount_overridden FROM contracts WHERE engagement_id = $1`, e.Engagement.ID)
	if !overridden {
		t.Fatalf("amount_overridden = false, want true after an Owner/Admin override")
	}
	if got.AmountChangedAt == nil {
		t.Fatalf("GET amountChangedAt = nil, want set -- an Owner/Admin can see the price changed and when (#968)")
	}
}

// TestPutContractAmountHandler_RepeatedIdenticalValueRecordsNoNewActivity
// proves the idempotent-PUT shape: a retry with the same amount writes
// no second activity row, mirroring practicerate's own repeat case.
func TestPutContractAmountHandler_RepeatedIdenticalValueRecordsNoNewActivity(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedDraft()

	reqtest.ExpectStatus(t, http.MethodPut, e.amountURL(), http.StatusOK, e.signedIn, amounting(15000))
	reqtest.ExpectStatus(t, http.MethodPut, e.amountURL(), http.StatusOK, e.signedIn, amounting(15000))

	count := testdb.Scalar[int](t, e.db.Admin, `SELECT count(*) FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'contract_amount_overridden'`, e.Engagement.ID)
	if count != 0 {
		t.Fatalf("activity rows = %d, want 0 (the amount never actually changed, 15000 == 15000)", count)
	}
}

// TestPutContractAmountHandler_Refusals table-drives what an override is
// refused for. Zero and negative amounts are both invalid, and malformed
// JSON is refused. #967's AC is that "a signed Contract's amount never
// changes for any reason" -- TransitionOverrideAmount refuses everything
// but Draft, so sent, signed and voided all 409 -- and an Engagement with
// no Contract at all 404s.
func TestPutContractAmountHandler_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		status string
		body   reqtest.Option
		want   int
	}{
		{"zero", statusDraft, amounting(0), http.StatusBadRequest},
		{"negative", statusDraft, amounting(-100), http.StatusBadRequest},
		{caseBodyNotJSON, statusDraft, reqtest.RawJSON([]byte("not json")), http.StatusBadRequest},
		{"sent", statusSent, amounting(20000), http.StatusConflict},
		{statusSigned, statusSigned, amounting(20000), http.StatusConflict},
		{"voided", statusVoided, amounting(20000), http.StatusConflict},
		{caseNoContract, "", amounting(20000), http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Owner()))
			if tc.status != "" {
				e.seedContract(tc.status)
			}

			reqtest.ExpectStatus(t, http.MethodPut, e.amountURL(), tc.want, e.signedIn, tc.body)
		})
	}
}

// TestPutContractAmountHandler_RefusesAnEngagementItCannotReach proves a
// malformed path segment is refused before any query runs, and an
// Engagement belonging to another Practice 404s rather than leaking that
// it exists.
func TestPutContractAmountHandler_RefusesAnEngagementItCannotReach(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.refusesEngagementsItCannotReach(http.MethodPut, "/amount", amounting(20000))
}
