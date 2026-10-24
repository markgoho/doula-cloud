package contracts_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// noMergeFieldProse is Contract prose that holds no {{merge_field}} at
// all -- the template a Practice may write that #1776 found broke the
// Engagement page, because mergeFields marshaled as null.
const noMergeFieldProse = "This agreement has no fields to fill in."

// assertEmptyMergeFields fails the test unless resp has wantStatus and
// its body carries `"mergeFields": []` -- the literal JSON, not only a
// decoded empty slice, because null decodes to an empty slice too and
// null is what the screens cannot read.
func assertEmptyMergeFields(t *testing.T, resp *http.Response, wantStatus int) {
	t.Helper()
	if resp.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d", resp.StatusCode, wantStatus)
	}
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := string(raw["mergeFields"]); got != "[]" {
		t.Fatalf("mergeFields = %s, want [] for prose with no merge fields (#1776)", got)
	}
}

// TestContractResponses_NoMergeFieldsMarshalAsEmptyList proves #1776's
// fix on every response that carries mergeFields: a Contract whose prose
// holds no merge field answers with an empty list, never null.
func TestContractResponses_NoMergeFieldsMarshalAsEmptyList(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		db := testdb.New(t)
		const uid = "no-fields-create"
		practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, employeeType)
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)
		seedContractTemplate(t, db, practiceID, noMergeFieldProse)
		testdb.SeedPracticeRate(t, db, practiceID, "birth", testRateAmountCents)
		srv, session := newContractServer(t, db, uid)
		defer srv.Close()

		resp := postContract(t, srv, session, practiceID, engagementID)
		defer resp.Body.Close()
		assertEmptyMergeFields(t, resp, http.StatusCreated)
	})

	t.Run("staff read", func(t *testing.T) {
		db := testdb.New(t)
		const uid = "no-fields-get"
		practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)
		seedContract(t, db, engagementID, statusDraft, noMergeFieldProse)
		srv, session := newContractServer(t, db, uid)
		defer srv.Close()

		resp := getContract(t, srv, session, practiceID, engagementID)
		defer resp.Body.Close()
		assertEmptyMergeFields(t, resp, http.StatusOK)
	})

	t.Run("edit", func(t *testing.T) {
		db := testdb.New(t)
		const uid = "no-fields-put"
		practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)
		seedContract(t, db, engagementID, statusDraft, noMergeFieldProse)
		srv, session := newContractServer(t, db, uid)
		defer srv.Close()

		resp := putContract(t, srv, session, practiceID, engagementID, nil)
		defer resp.Body.Close()
		assertEmptyMergeFields(t, resp, http.StatusOK)
	})

	t.Run("send", func(t *testing.T) {
		db := testdb.New(t)
		const uid = "no-fields-send"
		practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, employeeType)
		clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
		seedContract(t, db, engagementID, statusDraft, noMergeFieldProse)
		testdb.SeedPendingPortalInvite(t, db, clientID)
		srv, session := newContractServer(t, db, uid)
		defer srv.Close()

		resp := postSendContract(t, srv, session, practiceID, engagementID)
		defer resp.Body.Close()
		assertEmptyMergeFields(t, resp, http.StatusOK)
	})

	t.Run("void", func(t *testing.T) {
		db := testdb.New(t)
		const uid = "no-fields-void"
		practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)
		seedContract(t, db, engagementID, statusSigned, noMergeFieldProse)
		srv, session := newContractServer(t, db, uid)
		defer srv.Close()

		resp := postVoidContract(t, srv, session, practiceID, engagementID)
		defer resp.Body.Close()
		assertEmptyMergeFields(t, resp, http.StatusOK)
	})

	t.Run("void request", func(t *testing.T) {
		db := testdb.New(t)
		const uid = "no-fields-void-request"
		practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, employeeType)
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)
		seedContract(t, db, engagementID, statusSigned, noMergeFieldProse)
		srv, session := newContractServer(t, db, uid)
		defer srv.Close()

		resp := postVoidRequest(t, srv, session, practiceID, engagementID, "the plan changed")
		defer resp.Body.Close()
		assertEmptyMergeFields(t, resp, http.StatusCreated)
	})

	t.Run("void request decline", func(t *testing.T) {
		db := testdb.New(t)
		const uid = "no-fields-void-decline"
		practiceID, adminID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, employeeType)
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)
		seedContract(t, db, engagementID, statusSigned, noMergeFieldProse)
		requestID := seedVoidRequest(t, db, engagementID, adminID, "asking")
		srv, session := newContractServer(t, db, uid)
		defer srv.Close()

		resp := postVoidRequestDecline(t, srv, session, practiceID, engagementID, requestID, "not yet")
		defer resp.Body.Close()
		assertEmptyMergeFields(t, resp, http.StatusOK)
	})

	t.Run("client read", func(t *testing.T) {
		db := testdb.New(t)
		const identityUID = "client-no-fields-get"
		practiceID := testdb.SeedPractice(t, db, "Practice")
		clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
		testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)
		seedContract(t, db, engagementID, statusSent, noMergeFieldProse)
		srv, session := newPortalServer(t, db, identityUID)
		defer srv.Close()

		resp := getClientContract(t, srv, session, engagementID)
		defer resp.Body.Close()
		assertEmptyMergeFields(t, resp, http.StatusOK)
	})

	t.Run("client sign", func(t *testing.T) {
		db := testdb.New(t)
		const identityUID = "client-no-fields-sign"
		practiceID := testdb.SeedPractice(t, db, "Practice")
		clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
		testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)
		seedContract(t, db, engagementID, statusSent, noMergeFieldProse)
		srv, session := newPortalServer(t, db, identityUID)
		defer srv.Close()

		resp := postSignContract(t, srv, session, engagementID, "Jordan Client", true)
		defer resp.Body.Close()
		assertEmptyMergeFields(t, resp, http.StatusOK)
	})
}
