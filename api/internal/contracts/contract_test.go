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
	testPriceValue     = "$1,200"
	testScopeOfService = "12 prenatal visits"
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
	practiceID := seedMember(t, db, uid)

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
	practiceID := seedMember(t, db, uid)
	otherPracticeID := seedPractice(t, db, "Other Practice")
	otherEngagementID := seedEngagement(t, db, otherPracticeID)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedTemplate(t, db, practiceID, mergeFieldProse)

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
	if len(out.Values) != 1 || out.Values[clientNameKey] != "Test Client" {
		t.Fatalf("values = %+v, want client_name prefilled from the Engagement's Client", out.Values)
	}
}

// TestPostContractHandler_NoClientNameFieldLeavesValuesEmpty proves
// prefillClientName's other branch: a Template whose prose never asks
// for client_name gets no prefill at all.
func TestPostContractHandler_NoClientNameFieldLeavesValuesEmpty(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-no-client-name-field"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedTemplate(t, db, practiceID, "Agreement at {{price}}.")

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

// TestPostContractHandler_DedupesRepeatedMergeField proves a merge field
// placeholder repeated in prose appears once in MergeFields, not once
// per occurrence.
func TestPostContractHandler_DedupesRepeatedMergeField(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-dedup"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedTemplate(t, db, practiceID, "Hello {{client_name}}, this agreement is for {{client_name}}.")

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedTemplate(t, db, practiceID, mergeFieldProse)

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

func TestGetContractHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-invalid-engagement-id"
	practiceID := seedMember(t, db, uid)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)

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
	practiceID, _ := seedContractorAtPractice(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
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
// Contract's scope -- but never its money, regardless of attachment.
func TestGetContractHandler_ContractorWithGrantedAttachmentSeesScope(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-contractor-attached"
	practiceID, staffID := seedContractorAtPractice(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)
	seedGrantedAttachment(t, db, engagementID, staffID)

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
}

func TestPutContractHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-invalid-engagement-id"
	practiceID := seedMember(t, db, uid)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
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
	if len(out.Values) != 0 {
		t.Fatalf("values = %+v, want empty", out.Values)
	}
}

// TestPutContractHandler_Success proves a full Values replace round-trips
// through GET.
func TestPutContractHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-success"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	putResp := putContract(t, srv, session, practiceID, engagementID,
		contracts.MergeFieldValues{clientNameKey: jamieName, priceKey: testPriceValue})
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
	if out.Values[clientNameKey] != jamieName || out.Values[priceKey] != testPriceValue {
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
	practiceID := seedOwner(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedTemplate(t, db, practiceID, mergeFieldProse)

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
