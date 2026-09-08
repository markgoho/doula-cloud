package contracts_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/testdb"
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

func contractURL(srv *httptest.Server, practiceID, engagementID string) string {
	return srv.URL + "/api/practices/" + practiceID + "/engagements/" + engagementID + "/contract"
}

func postContract(t *testing.T, srv *httptest.Server, session string, practiceID, engagementID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, contractURL(srv, practiceID, engagementID), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func getContract(t *testing.T, srv *httptest.Server, session string, practiceID, engagementID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, contractURL(srv, practiceID, engagementID), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func getContractPDFRaw(t *testing.T, srv *httptest.Server, session string, practiceID, engagementID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, contractURL(srv, practiceID, engagementID)+"/pdf", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if session != "" {
		authntest.AddSessionCookie(req, session)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func getContractPDF(t *testing.T, srv *httptest.Server, session string, practiceID, engagementID string) *http.Response {
	t.Helper()
	return getContractPDFRaw(t, srv, session, practiceID, engagementID)
}

func putContractRaw(t *testing.T, srv *httptest.Server, session string, practiceID, engagementID string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, contractURL(srv, practiceID, engagementID), bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func putContract(t *testing.T, srv *httptest.Server, session string, practiceID, engagementID string, values contracts.MergeFieldValues) *http.Response {
	t.Helper()
	payload, err := json.Marshal(contracts.PutContractRequest{Values: values})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return putContractRaw(t, srv, session, practiceID, engagementID, payload)
}

func TestPostContractHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-invalid-engagement-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postContract(t, srv, session, practiceID, "not-a-uuid")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostContractHandler_EngagementNotFound proves an Engagement id from
// a different Practice (or one that doesn't exist) 404s rather than
// creating a Contract under it.
func TestPostContractHandler_EngagementNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-no-engagement"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	_, otherEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postContract(t, srv, session, practiceID, otherEngagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPostContractHandler_NoTemplate proves the "should not happen
// post-#67" missing-template case fails predictably with 404, not a
// crash.
func TestPostContractHandler_NoTemplate(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-no-template"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPostContractHandler_Success proves a Contract is created as a
// Draft, snapshotting the Practice's current template prose, with its
// merge fields parsed out of that prose -- client_name prefilled from the
// Engagement's Client (ADR-0017), every other merge field left blank --
// and that any Staff member (not just an Owner) can create one.
func TestPostContractHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-success"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractTemplate(t, db, practiceID, mergeFieldProse)
	testdb.SeedPracticeRate(t, db, practiceID, "birth", testRateAmountCents)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.EngagementID != engagementID {
		t.Fatalf("engagementId = %q, want %q", out.EngagementID, engagementID)
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
	db := testdb.New(t)
	const uid = "post-created-and-priced"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractTemplate(t, db, practiceID, mergeFieldProse)
	testdb.SeedPracticeRate(t, db, practiceID, "birth", testRateAmountCents)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var createdActorStaffID string
	var createdDiff []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id, diff FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'contract_created'`,
		engagementID,
	).Scan(&createdActorStaffID, &createdDiff); err != nil {
		t.Fatalf("query contract_created activity: %v", err)
	}
	if createdActorStaffID != staffID {
		t.Fatalf("contract_created actor_staff_id = %q, want %q", createdActorStaffID, staffID)
	}
	if string(createdDiff) != "{}" {
		t.Fatalf("contract_created diff = %q, want no price carried -- it left the money set (#972)", createdDiff)
	}

	var pricedActorStaffID string
	var pricedDiff []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id, diff FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'contract_priced'`,
		engagementID,
	).Scan(&pricedActorStaffID, &pricedDiff); err != nil {
		t.Fatalf("query contract_priced activity: %v", err)
	}
	if pricedActorStaffID != staffID {
		t.Fatalf("contract_priced actor_staff_id = %q, want %q", pricedActorStaffID, staffID)
	}
	var parsedDiff struct {
		AmountCentsBefore int64 `json:"amountCentsBefore"`
		AmountCentsAfter  int64 `json:"amountCentsAfter"`
	}
	if err := json.Unmarshal(pricedDiff, &parsedDiff); err != nil {
		t.Fatalf("unmarshal contract_priced diff: %v", err)
	}
	if parsedDiff.AmountCentsBefore != 0 || parsedDiff.AmountCentsAfter != testRateAmountCents {
		t.Fatalf("contract_priced diff = %+v, want before=0 after=%d", parsedDiff, testRateAmountCents)
	}
}

// TestPostContractHandler_NoClientNameFieldLeavesValuesEmpty proves
// resolveMergeFieldValues's other branch: a Template whose prose never
// asks for client_name or practice_name gets no prefill at all. Prose
// uses a plain scope field rather than {{price}} -- price is no longer
// resolved by resolveMergeFieldValues at all (#967: it is resolved
// separately, from a real column, never stored in merge_field_values),
// so a prose that only asks for it would prove nothing about this
// function's own empty branch.
func TestPostContractHandler_NoClientNameFieldLeavesValuesEmpty(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-no-client-name-field"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractTemplate(t, db, practiceID, "Agreement for {{scope_of_service}}.")
	testdb.SeedPracticeRate(t, db, practiceID, "birth", testRateAmountCents)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Values) != 0 {
		t.Fatalf("values = %+v, want empty -- no client_name field to prefill", out.Values)
	}
}

// TestPostContractHandler_ResolvesPracticeName proves #258's AC: prose
// containing the Practice-name placeholder returns that placeholder
// already filled with the Practice's own name.
func TestPostContractHandler_ResolvesPracticeName(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-resolves-practice-name"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractTemplate(t, db, practiceID, "This agreement is between {{practice_name}} and {{client_name}} for doula services.")
	testdb.SeedPracticeRate(t, db, practiceID, "birth", testRateAmountCents)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Values["practice_name"] != "Test Practice" {
		t.Fatalf("values[practice_name] = %q, want %q", out.Values["practice_name"], "Test Practice")
	}
	if out.Values[clientNameKey] != testClientName {
		t.Fatalf("values[client_name] = %q, want %q -- still resolved alongside practice_name", out.Values[clientNameKey], testClientName)
	}
}

// TestPutContractHandler_OverwrittenResolvedValueSurvivesReload proves a
// resolved value is a normal Draft value: Staff may overwrite it through
// the existing full-replacement update, and the overwritten value
// survives a reload rather than reverting to the resolved one.
func TestPutContractHandler_OverwrittenResolvedValueSurvivesReload(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-overwrite-resolved"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractTemplate(t, db, practiceID, "This agreement is between {{practice_name}} and {{client_name}} for doula services.")
	testdb.SeedPracticeRate(t, db, practiceID, "birth", testRateAmountCents)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	createResp := postContract(t, srv, session, practiceID, engagementID)
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createResp.StatusCode, http.StatusCreated)
	}

	const overwrittenPracticeName = "Overwritten Practice Name"
	putResp := putContract(t, srv, session, practiceID, engagementID,
		contracts.MergeFieldValues{"practice_name": overwrittenPracticeName, clientNameKey: testClientName})
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", putResp.StatusCode, http.StatusOK)
	}

	getResp := getContract(t, srv, session, practiceID, engagementID)
	defer getResp.Body.Close()
	var getOut contracts.ContractResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if getOut.Values["practice_name"] != overwrittenPracticeName {
		t.Fatalf("values[practice_name] after reload = %q, want %q", getOut.Values["practice_name"], overwrittenPracticeName)
	}
}

// TestPostContractHandler_DedupesRepeatedMergeField proves a merge field
// placeholder repeated in prose appears once in MergeFields, not once
// per occurrence.
func TestPostContractHandler_DedupesRepeatedMergeField(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-dedup"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractTemplate(t, db, practiceID, "Hello {{client_name}}, this agreement is for {{client_name}}.")
	testdb.SeedPracticeRate(t, db, practiceID, "birth", testRateAmountCents)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.MergeFields) != 1 || out.MergeFields[0] != clientNameKey {
		t.Fatalf("mergeFields = %v, want [client_name] deduped", out.MergeFields)
	}
}

// TestPostContractHandler_Duplicate proves a second POST for the same
// Engagement 409s -- POST creates, PUT edits.
func TestPostContractHandler_Duplicate(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-duplicate"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractTemplate(t, db, practiceID, mergeFieldProse)
	testdb.SeedPracticeRate(t, db, practiceID, "birth", testRateAmountCents)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	first := postContract(t, srv, session, practiceID, engagementID)
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first POST status = %d, want %d", first.StatusCode, http.StatusCreated)
	}

	second := postContract(t, srv, session, practiceID, engagementID)
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second POST status = %d, want %d", second.StatusCode, http.StatusConflict)
	}
}

// TestPostContractHandler_ContractorWithoutAttachmentNotFound proves #970's
// AC that the contractor reach rule is unchanged by this ticket's mount
// declaration: staffauth.AnyStaff adds no role restriction beyond what
// AttachingWrite already enforces, so an unattached contractor Doula
// still gets 404, the same reach refusal GetContractHandler's own
// contractor test proves on the read side.
func TestPostContractHandler_ContractorWithoutAttachmentNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-contractor-unattached"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractTemplate(t, db, practiceID, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetContractHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-invalid-engagement-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContract(t, srv, session, practiceID, "not-a-uuid")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestGetContractHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-not-found"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestGetContractHandler_ContractorWithoutAttachmentForbidden proves
// ADR-0008's attachment rule for Contract scope: a contractor Doula with
// no engagement_attachments row gets the same "not found" response an
// out-of-practice engagementId gets -- no partly-open state (#230).
func TestGetContractHandler_ContractorWithoutAttachmentForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-contractor-unattached"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
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
	db := testdb.New(t)
	const uid = "get-contractor-attached"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractWithValues(t, db, engagementID, contracts.MergeFieldValues{clientNameKey: jamieName, priceKey: testPriceValue})
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.EngagementID != engagementID {
		t.Fatalf("engagementId = %q, want %q", out.EngagementID, engagementID)
	}
	if out.Values[clientNameKey] != jamieName {
		t.Fatalf("Values[client_name] = %q, want %q -- scope still reaches a granted-attachment contractor", out.Values[clientNameKey], jamieName)
	}
	if _, present := out.Values[priceKey]; present {
		t.Fatalf("Values = %+v, want no price key reachable at all for a contractor (#967)", out.Values)
	}
}

// TestGetContractHandler_AmountChangedAtHiddenFromContractor proves
// #968's AC ("whoever opens a re-derived Contract can see that its price
// changed") stays inside ADR-0008's money tier: a contractor never sees
// amountChangedAt, the same "no price key reachable at all" guarantee
// priceForReader already gives the "price" merge field value itself --
// otherwise she would learn "the price changed" with the value redacted,
// which is still a fact about the Practice's money.
func TestGetContractHandler_AmountChangedAtHiddenFromContractor(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-contractor-amount-changed"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE contracts SET amount_changed_at = now() WHERE engagement_id = $1`, engagementID,
	); err != nil {
		t.Fatalf("set amount_changed_at: %v", err)
	}

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.AmountChangedAt != nil {
		t.Fatalf("amountChangedAt = %v, want nil for an ambient contractor (#968, ADR-0008)", out.AmountChangedAt)
	}
}

// TestGetContractHandler_AmountChangedAtVisibleToEmployee proves the
// same read's Owner/Admin/employee side: amountChangedAt round-trips
// once amount_changed_at is set on the row, so a reader can see the
// price changed and when without hunting the activity ledger for it.
func TestGetContractHandler_AmountChangedAtVisibleToEmployee(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-employee-amount-changed"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE contracts SET amount_changed_at = now() WHERE engagement_id = $1`, engagementID,
	); err != nil {
		t.Fatalf("set amount_changed_at: %v", err)
	}

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.AmountChangedAt == nil {
		t.Fatalf("amountChangedAt = nil, want set -- an employed Doula reads Contract money now (#282)")
	}
}

// TestGetContractHandler_EmployedDoulaSeesMoney proves ADR-0008's money
// row as amended by #282: an employed Doula reads the Contract's price,
// resolved from amount_cents (#967), the same as an Owner or Admin --
// previously withheld entirely.
func TestGetContractHandler_EmployedDoulaSeesMoney(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-employee-money"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractWithValues(t, db, engagementID, contracts.MergeFieldValues{clientNameKey: jamieName})

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
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
	db := testdb.New(t)
	const uid = "get-contractor-no-raw-price"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, present := out.Values[priceKey]; present {
		t.Fatalf("Values = %+v, want no price key reachable for a contractor", out.Values)
	}
}

func TestPutContractHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-invalid-engagement-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContract(t, srv, session, practiceID, "not-a-uuid", contracts.MergeFieldValues{})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPutContractHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-not-found"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContract(t, srv, session, practiceID, engagementID, contracts.MergeFieldValues{clientNameKey: jamieName})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPutContractHandler_ContractorWithoutAttachmentNotFound is #970's AC
// for set-values specifically: staffauth.AnyStaff at the mount adds no
// role restriction of its own, so AttachingWrite's reach test still
// refuses an unattached contractor before the handler ever runs -- the
// same 404 TestPutContractHandler_NotFound gets, but for reach rather
// than for having no Contract at all.
func TestPutContractHandler_ContractorWithoutAttachmentNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-contractor-unattached"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContract(t, srv, session, practiceID, engagementID, contracts.MergeFieldValues{clientNameKey: jamieName})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPutContractHandler_NonDraftRejected proves PUT 409s once a Contract
// has moved past 'draft' -- no endpoint in this ticket can drive a
// Contract to that state itself, so the row is seeded directly.
func TestPutContractHandler_NonDraftRejected(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-non-draft"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, "sent", mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContract(t, srv, session, practiceID, engagementID, contracts.MergeFieldValues{clientNameKey: jamieName})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

func TestPutContractHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-invalid-body"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContractRaw(t, srv, session, practiceID, engagementID, []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPutContractHandler_UnknownMergeFieldKeyRejected(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-unknown-key"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContract(t, srv, session, practiceID, engagementID, contracts.MergeFieldValues{"not_a_field": "x"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutContractHandler_StatusFieldIgnored proves no request body field
// can move a Contract's status -- PutContractRequest has no Status field
// at all, so an incoming "status" key is silently dropped by the decoder.
func TestPutContractHandler_StatusFieldIgnored(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-status-ignored"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContractRaw(t, srv, session, practiceID, engagementID,
		[]byte(`{"status":"signed","values":{"client_name":"Jamie"}}`))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Status != statusDraft {
		t.Fatalf("status = %q, want draft (unaffected by the request body)", out.Status)
	}
}

// TestPutContractHandler_EmptyBodyDefaultsToEmptyValues proves a request
// body with no "values" key at all (Values decodes to nil, not an empty
// map) is normalized to an empty map rather than crashing or storing
// JSON null.
func TestPutContractHandler_EmptyBodyDefaultsToEmptyValues(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-empty-body"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContractRaw(t, srv, session, practiceID, engagementID, []byte(`{}`))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// price still resolves even against an otherwise-empty Values map --
	// it is never sourced from what Staff PUTs (#967).
	if len(out.Values) != 1 || out.Values[priceKey] != testRateAmountDollars {
		t.Fatalf("values = %+v, want only the resolved price", out.Values)
	}
}

// TestPutContractHandler_PriceRejected proves #967's AC directly: price
// is a reserved merge field resolved by the product, never a value a
// person fills in on the Contract form -- a PUT that tries anyway is a
// 400, not a silent overwrite the way client_name/practice_name allow.
func TestPutContractHandler_PriceRejected(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-price-rejected"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContract(t, srv, session, practiceID, engagementID,
		contracts.MergeFieldValues{clientNameKey: jamieName, priceKey: "$1"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutContractHandler_Success proves a full Values replace round-trips
// through GET.
func TestPutContractHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-success"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	putResp := putContract(t, srv, session, practiceID, engagementID,
		contracts.MergeFieldValues{clientNameKey: jamieName})
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", putResp.StatusCode, http.StatusOK)
	}

	getResp := getContract(t, srv, session, practiceID, engagementID)
	defer getResp.Body.Close()
	var out contracts.ContractResponse
	if err := json.NewDecoder(getResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
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
	db := testdb.New(t)
	const uid = "snapshot-immutable"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractTemplate(t, db, practiceID, mergeFieldProse)
	testdb.SeedPracticeRate(t, db, practiceID, "birth", testRateAmountCents)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	postResp := postContract(t, srv, session, practiceID, engagementID)
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusCreated {
		t.Fatalf("POST status = %d, want %d", postResp.StatusCode, http.StatusCreated)
	}

	putTemplateResp := putTemplate(t, srv, session, practiceID,
		contracts.TemplateResponse{Prose: "Totally different agreement for {{scope_of_service}}."})
	defer putTemplateResp.Body.Close()
	if putTemplateResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT template status = %d, want %d", putTemplateResp.StatusCode, http.StatusOK)
	}

	getResp := getContract(t, srv, session, practiceID, engagementID)
	defer getResp.Body.Close()
	var out contracts.ContractResponse
	if err := json.NewDecoder(getResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Prose != mergeFieldProse {
		t.Fatalf("prose = %q, want the original snapshot unaffected by the template edit", out.Prose)
	}
	if len(out.MergeFields) != 2 || out.MergeFields[0] != clientNameKey || out.MergeFields[1] != priceKey {
		t.Fatalf("mergeFields = %v, want the original snapshot's fields", out.MergeFields)
	}
}
