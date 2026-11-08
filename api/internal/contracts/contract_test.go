package contracts_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

const mergeFieldProse = "Agreement for {{client_name}} at {{price}}."

const (
	clientNameKey = "client_name"
	priceKey      = "price"
	jamieName     = "Jamie"

	// Shared across contracts_test files: goconst flags repeated literals
	// package-wide, not just within one file.
	testPriceValue = "$1,200"
	// testClientName is SeedEngagement's default Client given name
	// (testdb.SeedNamedEngagement's own "Test Client" argument), and
	// therefore the value resolveMergeFieldValues resolves client_name to
	// for every Contract seeded through the default SeedEngagement.
	testClientName = "Test Client"

	// testRateAmountCents and testRateAmountDollars are the practice_rates
	// amount this file seeds wherever a test creates a Contract through
	// PostContractHandler (#967: no rate set for the Engagement's kind
	// means no Contract at all), and the "price" merge field's resolved
	// rendering of that same amount.
	testRateAmountCents   = 15000
	testRateAmountDollars = "$150.00"
)

func TestPostContractHandler_RefusesAnEngagementItCannotReach(t *testing.T) {
	e := newEnv(t)
	e.refusesEngagementsItCannotReach(http.MethodPost, "")
}

// TestPostContractHandler_NoTemplate proves the "should not happen
// post-#67" missing-template case fails predictably with 404, not a
// crash.
func TestPostContractHandler_NoTemplate(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodPost, e.contractURL(""), http.StatusNotFound, e.signedIn)
}

// TestPostContractHandler_Success proves a Contract is created as a
// Draft, snapshotting the Practice's current template prose, with its
// merge fields parsed out of that prose -- client_name prefilled from the
// Engagement's Client (ADR-0017), every other merge field left blank --
// and that any Staff member (not just an Owner) can create one.
func TestPostContractHandler_Success(t *testing.T) {
	e := newEnv(t)
	e.readyToDraft(mergeFieldProse)

	out := reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.contractURL(""), http.StatusCreated, e.signedIn)

	if out.EngagementID != e.Engagement.ID {
		t.Fatalf("engagementId = %q, want %q", out.EngagementID, e.Engagement.ID)
	}
	if out.Status != statusDraft {
		t.Fatalf("status = %q, want draft", out.Status)
	}
	if out.Prose != mergeFieldProse {
		t.Fatalf("prose = %q, want the seeded template's prose", out.Prose)
	}
	if len(out.MergeFields) != 2 || out.MergeFields[0] != clientNameKey || out.MergeFields[1] != priceKey {
		t.Fatalf("mergeFields = %v, want [client_name price]", out.MergeFields)
	}
	if len(out.Values) != 2 || out.Values[clientNameKey] != testClientName {
		t.Fatalf("values = %+v, want client_name prefilled from the Engagement's Client", out.Values)
	}
	if out.Values[priceKey] != testRateAmountDollars {
		t.Fatalf("values[price] = %q, want %q -- resolved from the practice's rate card, #967", out.Values[priceKey], testRateAmountDollars)
	}
}

// TestPostContractHandler_RecordsCreatedAndPricedSeparately proves #972's
// split: creation writes both a contract_created row (the entity's own
// lifecycle, no price -- no longer in the money set, so its diff must
// never carry one) and a separate contract_priced row (the money set)
// naming the acting Staff member, the time, and the rate-card-derived
// amount as amountCentsAfter, with amountCentsBefore 0 since nothing
// existed to have carried a price before this Contract did.
func TestPostContractHandler_RecordsCreatedAndPricedSeparately(t *testing.T) {
	e := newEnv(t)
	e.readyToDraft(mergeFieldProse)

	reqtest.ExpectStatus(t, http.MethodPost, e.contractURL(""), http.StatusCreated, e.signedIn)

	createdDiff := assertActivityActor(t, e.db, e.Engagement.ID, activity.ActionContractCreated, e.Staff.ID)
	if string(createdDiff) != "{}" {
		t.Fatalf("contract_created diff = %q, want no price carried -- it left the money set (#972)", createdDiff)
	}

	pricedDiff := assertActivityActor(t, e.db, e.Engagement.ID, activity.ActionContractPriced, e.Staff.ID)
	var parsedDiff activity.AmountDiff
	if err := json.Unmarshal(pricedDiff, &parsedDiff); err != nil {
		t.Fatalf("unmarshal contract_priced diff: %v", err)
	}
	if parsedDiff.AmountCentsBefore != 0 || parsedDiff.AmountCentsAfter != testRateAmountCents {
		t.Fatalf("contract_priced diff = %+v, want before=0 after=%d", parsedDiff, testRateAmountCents)
	}
}

// TestPostContractHandler_ResolvesMergeFields proves what
// resolveMergeFieldValues prefills from the prose a Template asks for:
// the Practice's own name for its placeholder (#258's AC), alongside the
// Client's name, and nothing at all for a Template that asks for neither.
// The last row's prose uses a plain scope field rather than {{price}} --
// price is no longer resolved by resolveMergeFieldValues at all (#967: it
// is resolved separately, from a real column, never stored in
// merge_field_values), so a prose that only asks for it would prove
// nothing about the empty branch.
func TestPostContractHandler_ResolvesMergeFields(t *testing.T) {
	cases := []struct {
		name  string
		prose string
		want  contracts.MergeFieldValues
	}{
		{
			"the Practice's name beside the Client's",
			"This agreement is between {{practice_name}} and {{client_name}} for doula services.",
			contracts.MergeFieldValues{"practice_name": "Test Practice", clientNameKey: testClientName},
		},
		{"no field to prefill", "Agreement for {{scope_of_service}}.", contracts.MergeFieldValues{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.readyToDraft(c.prose)

			out := reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.contractURL(""), http.StatusCreated, e.signedIn)

			if len(out.Values) != len(c.want) {
				t.Fatalf("values = %+v, want %+v", out.Values, c.want)
			}
			for key, want := range c.want {
				if out.Values[key] != want {
					t.Fatalf("values[%s] = %q, want %q", key, out.Values[key], want)
				}
			}
		})
	}
}

// TestPutContractHandler_OverwrittenResolvedValueSurvivesReload proves a
// resolved value is a normal Draft value: Staff may overwrite it through
// the existing full-replacement update, and the overwritten value
// survives a reload rather than reverting to the resolved one.
func TestPutContractHandler_OverwrittenResolvedValueSurvivesReload(t *testing.T) {
	e := newEnv(t)
	e.readyToDraft("This agreement is between {{practice_name}} and {{client_name}} for doula services.")
	reqtest.ExpectStatus(t, http.MethodPost, e.contractURL(""), http.StatusCreated, e.signedIn)

	const overwrittenPracticeName = "Overwritten Practice Name"
	reqtest.ExpectStatus(t, http.MethodPut, e.contractURL(""), http.StatusOK, e.signedIn,
		putting(contracts.MergeFieldValues{"practice_name": overwrittenPracticeName, clientNameKey: testClientName}))

	if got := e.contract().Values["practice_name"]; got != overwrittenPracticeName {
		t.Fatalf("values[practice_name] after reload = %q, want %q", got, overwrittenPracticeName)
	}
}

// TestPostContractHandler_DedupesRepeatedMergeField proves a merge field
// placeholder repeated in prose appears once in MergeFields, not once
// per occurrence.
func TestPostContractHandler_DedupesRepeatedMergeField(t *testing.T) {
	e := newEnv(t)
	e.readyToDraft("Hello {{client_name}}, this agreement is for {{client_name}}.")

	out := reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.contractURL(""), http.StatusCreated, e.signedIn)

	if len(out.MergeFields) != 1 || out.MergeFields[0] != clientNameKey {
		t.Fatalf("mergeFields = %v, want [client_name] deduped", out.MergeFields)
	}
}

// TestPostContractHandler_Duplicate proves a second POST for the same
// Engagement 409s -- POST creates, PUT edits.
func TestPostContractHandler_Duplicate(t *testing.T) {
	e := newEnv(t)
	e.readyToDraft(mergeFieldProse)

	reqtest.ExpectStatus(t, http.MethodPost, e.contractURL(""), http.StatusCreated, e.signedIn)
	reqtest.ExpectStatus(t, http.MethodPost, e.contractURL(""), http.StatusConflict, e.signedIn)
}

// TestPostContractHandler_ContractorWithoutAttachmentNotFound proves #970's
// AC that the contractor reach rule is unchanged by this ticket's mount
// declaration: staffauth.AnyStaff adds no role restriction beyond what
// AttachingWrite already enforces, so an unattached contractor Doula
// still gets 404, the same reach refusal GetContractHandler's own
// contractor test proves on the read side.
func TestPostContractHandler_ContractorWithoutAttachmentNotFound(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))
	e.seedTemplate(mergeFieldProse)

	reqtest.ExpectStatus(t, http.MethodPost, e.contractURL(""), http.StatusNotFound, e.signedIn)
}

func TestGetContractHandler_RefusesAnEngagementItCannotReach(t *testing.T) {
	e := newEnv(t)
	e.refusesEngagementsItCannotReach(http.MethodGet, "")
}

func TestGetContractHandler_NotFound(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodGet, e.contractURL(""), http.StatusNotFound, e.signedIn)
}

// TestGetContractHandler_ContractorWithoutAttachmentForbidden proves
// ADR-0008's attachment rule for Contract scope: a contractor Doula with
// no engagement_attachments row gets the same "not found" response an
// out-of-practice engagementId gets -- no partly-open state (#230).
func TestGetContractHandler_ContractorWithoutAttachmentForbidden(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))
	e.seedDraft()

	reqtest.ExpectStatus(t, http.MethodGet, e.contractURL(""), http.StatusNotFound, e.signedIn)
}

// TestGetContractHandler_ContractorWithGrantedAttachmentSeesScope proves
// the other half of that rule: a granted, open attachment reaches the
// Contract's scope -- but never its price. #969 could not close this
// gap on its own (no single reliable key to gate a contractor's read on,
// with money no longer tagged at all); #967's real amount_cents column
// gives price back exactly one reserved key, and priceForReader deletes
// it outright for an ambient contractor, even though seedContractWithValues
// stored a raw value under it directly (a pre-#967 fixture shape) --
// proving the guarantee is "no price key reachable", not "an unresolved
// one".
func TestGetContractHandler_ContractorWithGrantedAttachmentSeesScope(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()), attachedToEngagement())
	seedContractWithValues(t, e.db, e.Engagement.ID, contracts.MergeFieldValues{clientNameKey: jamieName, priceKey: testPriceValue})

	out := e.contract()

	if out.EngagementID != e.Engagement.ID {
		t.Fatalf("engagementId = %q, want %q", out.EngagementID, e.Engagement.ID)
	}
	if out.Values[clientNameKey] != jamieName {
		t.Fatalf("Values[client_name] = %q, want %q -- scope still reaches a granted-attachment contractor", out.Values[clientNameKey], jamieName)
	}
	if _, present := out.Values[priceKey]; present {
		t.Fatalf("Values = %+v, want no price key reachable at all for a contractor (#967)", out.Values)
	}
}

// TestGetContractHandler_AmountChangedAt proves #968's AC ("whoever opens
// a re-derived Contract can see that its price changed") stays inside
// ADR-0008's money tier: a contractor never sees amountChangedAt, the
// same "no price key reachable at all" guarantee priceForReader already
// gives the "price" merge field value itself -- otherwise she would
// learn "the price changed" with the value redacted, which is still a
// fact about the Practice's money. An employed Doula reads Contract money
// (#282), so amountChangedAt round-trips for her once amount_changed_at
// is set on the row, and she can see the price changed and when without
// hunting the activity ledger for it.
func TestGetContractHandler_AmountChangedAt(t *testing.T) {
	cases := []struct {
		name        string
		staff       fixture.StaffOption
		wantVisible bool
	}{
		{"hidden from a contractor", fixture.Contractor(), false},
		{"visible to an employee", fixture.Doula(), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(c.staff), attachedToEngagement())
			e.seedDraft()
			e.exec(`UPDATE contracts SET amount_changed_at = now() WHERE engagement_id = $1`, e.Engagement.ID)

			if got := e.contract().AmountChangedAt != nil; got != c.wantVisible {
				t.Fatalf("amountChangedAt visible = %v, want %v (#968, #282, ADR-0008)", got, c.wantVisible)
			}
		})
	}
}

// TestGetContractHandler_EmployedDoulaSeesMoney proves ADR-0008's money
// row as amended by #282: an employed Doula reads the Contract's price,
// resolved from amount_cents (#967), the same as an Owner or Admin --
// previously withheld entirely.
func TestGetContractHandler_EmployedDoulaSeesMoney(t *testing.T) {
	e := newEnv(t)
	seedContractWithValues(t, e.db, e.Engagement.ID, contracts.MergeFieldValues{clientNameKey: jamieName})

	out := e.contract()

	if out.Values[priceKey] != testRateAmountDollars {
		t.Fatalf("Values[price] = %q, want %q -- an employed Doula reads Contract money now, resolved from amount_cents", out.Values[priceKey], testRateAmountDollars)
	}
}

// TestGetContractHandler_ContractorWithGrantedAttachmentNoRawPriceStored
// proves priceForReader's other branch: a real, post-#967 Contract never
// stores a raw price in merge_field_values at all (price is resolved,
// never persisted -- withResolvedPrice's own doc comment), so a
// contractor's read hits the "not present" early return rather than the
// delete path TestGetContractHandler_ContractorWithGrantedAttachmentSeesScope
// exercises -- both must leave no price key reachable.
func TestGetContractHandler_ContractorWithGrantedAttachmentNoRawPriceStored(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()), attachedToEngagement())
	e.seedDraft()

	out := e.contract()

	if _, present := out.Values[priceKey]; present {
		t.Fatalf("Values = %+v, want no price key reachable for a contractor", out.Values)
	}
}

func TestPutContractHandler_RefusesAnEngagementItCannotReach(t *testing.T) {
	e := newEnv(t)
	e.refusesEngagementsItCannotReach(http.MethodPut, "", putting(contracts.MergeFieldValues{}))
}

func TestPutContractHandler_NotFound(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodPut, e.contractURL(""), http.StatusNotFound, e.signedIn,
		putting(contracts.MergeFieldValues{clientNameKey: jamieName}))
}

// TestPutContractHandler_ContractorWithoutAttachmentNotFound is #970's AC
// for set-values specifically: staffauth.AnyStaff at the mount adds no
// role restriction of its own, so AttachingWrite's reach test still
// refuses an unattached contractor before the handler ever runs -- the
// same 404 TestPutContractHandler_NotFound gets, but for reach rather
// than for having no Contract at all.
func TestPutContractHandler_ContractorWithoutAttachmentNotFound(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))
	e.seedDraft()

	reqtest.ExpectStatus(t, http.MethodPut, e.contractURL(""), http.StatusNotFound, e.signedIn,
		putting(contracts.MergeFieldValues{clientNameKey: jamieName}))
}

// TestPutContractHandler_Refusals proves what a PUT refuses: a Contract
// that has moved past 'draft' (409 -- no endpoint in this ticket can
// drive a Contract to that state itself, so the row is seeded directly),
// a body that is not JSON, a merge field key the prose does not ask for,
// and a price. Price is a reserved merge field resolved by the product
// (#967), never a value a person fills in on the Contract form, so a PUT
// that tries anyway is a 400, not a silent overwrite the way
// client_name/practice_name allow.
func TestPutContractHandler_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		status string
		body   reqtest.Option
		want   int
	}{
		{"contract already sent", statusSent, putting(contracts.MergeFieldValues{clientNameKey: jamieName}), http.StatusConflict},
		{"body is not JSON", statusDraft, reqtest.RawJSON([]byte("not json")), http.StatusBadRequest},
		{"unknown merge field key", statusDraft, putting(contracts.MergeFieldValues{"not_a_field": "x"}), http.StatusBadRequest},
		{"price", statusDraft, putting(contracts.MergeFieldValues{clientNameKey: jamieName, priceKey: "$1"}), http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.seedContract(c.status)

			reqtest.ExpectStatus(t, http.MethodPut, e.contractURL(""), c.want, e.signedIn, c.body)
		})
	}
}

// TestPutContractHandler_StatusFieldIgnored proves no request body field
// can move a Contract's status -- PutContractRequest has no Status field
// at all, so an incoming "status" key is silently dropped by the decoder.
func TestPutContractHandler_StatusFieldIgnored(t *testing.T) {
	e := newEnv(t)
	e.seedDraft()

	out := reqtest.Expect[contracts.ContractResponse](t, http.MethodPut, e.contractURL(""), http.StatusOK, e.signedIn,
		reqtest.RawJSON([]byte(`{"status":"signed","values":{"client_name":"Jamie"}}`)))

	if out.Status != statusDraft {
		t.Fatalf("status = %q, want draft (unaffected by the request body)", out.Status)
	}
}

// TestPutContractHandler_EmptyBodyDefaultsToEmptyValues proves a request
// body with no "values" key at all (Values decodes to nil, not an empty
// map) is normalized to an empty map rather than crashing or storing
// JSON null.
func TestPutContractHandler_EmptyBodyDefaultsToEmptyValues(t *testing.T) {
	e := newEnv(t)
	e.seedDraft()

	out := reqtest.Expect[contracts.ContractResponse](t, http.MethodPut, e.contractURL(""), http.StatusOK, e.signedIn,
		reqtest.RawJSON([]byte(`{}`)))

	// price still resolves even against an otherwise-empty Values map --
	// it is never sourced from what Staff PUTs (#967).
	if len(out.Values) != 1 || out.Values[priceKey] != testRateAmountDollars {
		t.Fatalf("values = %+v, want only the resolved price", out.Values)
	}
}

// TestPutContractHandler_Success proves a full Values replace round-trips
// through GET.
func TestPutContractHandler_Success(t *testing.T) {
	e := newEnv(t)
	e.seedDraft()

	reqtest.ExpectStatus(t, http.MethodPut, e.contractURL(""), http.StatusOK, e.signedIn,
		putting(contracts.MergeFieldValues{clientNameKey: jamieName}))

	out := e.contract()
	// price is resolved from amount_cents, never from what was PUT (#967);
	// seedContract's own hardcoded amount_cents (template_test.go) is
	// what this renders as.
	if out.Values[clientNameKey] != jamieName || out.Values[priceKey] != testRateAmountDollars {
		t.Fatalf("values = %+v, want the just-written values", out.Values)
	}
}

// TestContract_TemplateEditDoesNotAlterExistingSnapshot proves editing
// the Contract Template's prose after a Contract instance exists does
// not change that instance's stored (and derived-merge-field) snapshot --
// the AC's explicit coverage requirement.
func TestContract_TemplateEditDoesNotAlterExistingSnapshot(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.readyToDraft(mergeFieldProse)
	reqtest.ExpectStatus(t, http.MethodPost, e.contractURL(""), http.StatusCreated, e.signedIn)

	reqtest.ExpectStatus(t, http.MethodPut, e.templateURL(), http.StatusOK, e.signedIn,
		reqtest.JSON(contracts.TemplateResponse{Prose: "Totally different agreement for {{scope_of_service}}."}))

	out := e.contract()
	if out.Prose != mergeFieldProse {
		t.Fatalf("prose = %q, want the original snapshot unaffected by the template edit", out.Prose)
	}
	if len(out.MergeFields) != 2 || out.MergeFields[0] != clientNameKey || out.MergeFields[1] != priceKey {
		t.Fatalf("mergeFields = %v, want the original snapshot's fields", out.MergeFields)
	}
}
