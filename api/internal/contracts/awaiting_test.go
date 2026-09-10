package contracts_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/testdb"
)

const adminRole = "admin"

// seedNamedEngagement seeds a Client carrying both a legal given name and
// the preferred name she is actually called, plus her Engagement --
// seedEngagement's Client has no preferred name, and the roll-up is
// supposed to print the one a person answers to.
func seedNamedEngagement(t *testing.T, db *testdb.DB, practiceID, givenName, preferredName string) (engagementID, clientID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, preferred_name, email)
		      VALUES ($1, $2, $3, 'client@example.com') RETURNING id`,
		practiceID, givenName, preferredName,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return engagementID, clientID
}

func awaitingURL(srv *httptest.Server, practiceID string) string {
	return srv.URL + "/api/practices/" + practiceID + "/contracts/awaiting-signature"
}

func getAwaiting(t *testing.T, session, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
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

// decodeAwaiting fetches one page and decodes it, failing the test unless
// the status is 200 -- every caller of this one is asserting the body,
// not the refusal. It owns the whole exchange rather than taking a
// response, so the body is closed in the same scope it was opened in.
func decodeAwaiting(t *testing.T, session, url string) contracts.AwaitingResponse {
	t.Helper()
	resp := getAwaiting(t, session, url)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out contracts.AwaitingResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return out
}

// TestAwaitingSignatureHandler_EmptyPracticeAnswersWithAnEmptyList proves
// a Practice owing no signatures gets an empty list rather than JSON
// null -- asserted on the raw bytes, because a []AwaitingItem field
// decodes null and [] into the same nil slice and only one of the two is
// a list the screen can render.
func TestAwaitingSignatureHandler_EmptyPracticeAnswersWithAnEmptyList(t *testing.T) {
	db := testdb.New(t)
	const uid = "awaiting-empty"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getAwaiting(t, session, awaitingURL(srv, practiceID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), `"items":[]`) {
		t.Fatalf("body = %s, want an empty items array", body)
	}
	if strings.Contains(string(body), "nextCursor") {
		t.Fatalf("body = %s, want no cursor on a list with nothing after it", body)
	}
}

// TestAwaitingSignatureHandler_ListsOnlyOutstandingContractsOldestFirst
// is #426's whole point: every Contract still waiting on somebody, in one
// read, oldest first -- and nothing else. A signed Contract is done and a
// voided one is a superseded record; neither is work anybody can chase.
func TestAwaitingSignatureHandler_ListsOnlyOutstandingContractsOldestFirst(t *testing.T) {
	db := testdb.New(t)
	const uid = "awaiting-mixed"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	draftEngagement, draftClient := seedNamedEngagement(t, db, practiceID, "Jamesina", jamieName)
	seedContract(t, db, draftEngagement, statusDraft, mergeFieldProse)
	sentEngagement, _ := seedNamedEngagement(t, db, practiceID, "Renata", "")
	seedContract(t, db, sentEngagement, statusSent, mergeFieldProse)
	_, signedEngagement := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, signedEngagement, statusSigned, mergeFieldProse)
	_, voidedEngagement := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, voidedEngagement, statusVoided, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	out := decodeAwaiting(t, session, awaitingURL(srv, practiceID))

	if len(out.Items) != 2 || out.HasMore || out.NextCursor != nil {
		t.Fatalf("page = %d items, hasMore=%v, cursor=%v; want the two outstanding Contracts on one page",
			len(out.Items), out.HasMore, out.NextCursor)
	}
	first, second := out.Items[0], out.Items[1]
	if first.EngagementID != draftEngagement || second.EngagementID != sentEngagement {
		t.Fatalf("order = %s, %s; want the longest wait first", first.EngagementID, second.EngagementID)
	}
	if first.Status != statusDraft || second.Status != statusSent {
		t.Fatalf("statuses = %q, %q; want the Practice's own unsent work told apart from the Client's",
			first.Status, second.Status)
	}
	if first.ClientID != draftClient || first.ClientName != jamieName {
		t.Fatalf("client = %s/%q, want the Client named as she is called", first.ClientID, first.ClientName)
	}
	if second.ClientName != "Renata" {
		t.Fatalf("client name = %q, want the legal given name where no preferred one was given", second.ClientName)
	}
	if first.ContractID == "" || first.CreatedAt.IsZero() {
		t.Fatalf("row = %+v, want the Contract identified and dated", first)
	}
}

// TestAwaitingSignatureHandler_IgnoresAnotherPracticesContracts proves
// the roll-up is scoped to the caller's Practice: an outstanding Contract
// somewhere else is not this Practice's work to chase.
func TestAwaitingSignatureHandler_IgnoresAnotherPracticesContracts(t *testing.T) {
	db := testdb.New(t)
	const uid = "awaiting-scope"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	_, otherEngagement := testdb.SeedEngagement(t, db, otherPracticeID)
	seedContract(t, db, otherEngagement, statusSent, mergeFieldProse)
	_, mine := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, mine, statusSent, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	out := decodeAwaiting(t, session, awaitingURL(srv, practiceID))

	if len(out.Items) != 1 || out.Items[0].EngagementID != mine {
		t.Fatalf("items = %+v, want only this Practice's outstanding Contract", out.Items)
	}
}

// TestAwaitingSignatureHandler_AdmitsAnAdmin proves the second of the two
// seats: the person who actually chases signatures at a fourteen-doula
// agency is an Admin, not the Owner.
func TestAwaitingSignatureHandler_AdmitsAnAdmin(t *testing.T) {
	db := testdb.New(t)
	const uid = "awaiting-admin"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusSent, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	out := decodeAwaiting(t, session, awaitingURL(srv, practiceID))

	if len(out.Items) != 1 || out.Items[0].EngagementID != engagementID {
		t.Fatalf("items = %+v, want the outstanding Contract", out.Items)
	}
}

// TestAwaitingSignatureHandler_AdmitsAnEmployedDoula proves ADR-0008's
// Engagements/Visits/Messages row, not the money row, governs this
// roll-up (#973): an employed Doula reaches every Engagement at the
// Practice, so she sees every outstanding Contract, the same as an Owner
// or Admin.
func TestAwaitingSignatureHandler_AdmitsAnEmployedDoula(t *testing.T) {
	db := testdb.New(t)
	const uid = "awaiting-employee-doula"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusSent, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	out := decodeAwaiting(t, session, awaitingURL(srv, practiceID))

	if len(out.Items) != 1 || out.Items[0].EngagementID != engagementID {
		t.Fatalf("items = %+v, want the outstanding Contract", out.Items)
	}
}

// TestAwaitingSignatureHandler_ContractorSeesOnlyHerAttachedEngagements
// is #973's own security boundary: a contractor Doula reaches only an
// Engagement she holds an open, granted attachment on, so an outstanding
// Contract on an Engagement she is not attached to never reaches her.
func TestAwaitingSignatureHandler_ContractorSeesOnlyHerAttachedEngagements(t *testing.T) {
	db := testdb.New(t)
	const contractorUID = "awaiting-contractor-narrowed"
	practiceID := testdb.SeedPractice(t, db, "Awaiting Contractor Practice")
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)

	_, attachedEngagement := testdb.SeedEngagement(t, db, practiceID)
	testdb.SeedGrantedAttachment(t, db, attachedEngagement, contractorID)
	seedContract(t, db, attachedEngagement, statusSent, mergeFieldProse)

	_, unattachedEngagement := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, unattachedEngagement, statusSent, mergeFieldProse)

	srv, session := newContractServer(t, db, contractorUID)
	defer srv.Close()

	out := decodeAwaiting(t, session, awaitingURL(srv, practiceID))

	if len(out.Items) != 1 || out.Items[0].EngagementID != attachedEngagement {
		t.Fatalf("items = %+v, want exactly the one attached Engagement (no leak of the unattached one)", out.Items)
	}
}

// TestAwaitingSignatureHandler_ContractorWithNoAttachmentSeesNothing
// proves the narrow case #973's acceptance criteria calls out by name: a
// contractor who holds no granted attachment at all gets an empty list,
// not an error and not the Practice's whole roll-up.
func TestAwaitingSignatureHandler_ContractorWithNoAttachmentSeesNothing(t *testing.T) {
	db := testdb.New(t)
	const contractorUID = "awaiting-contractor-unattached"
	practiceID := testdb.SeedPractice(t, db, "Awaiting Contractor No Attachment Practice")
	testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)

	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusSent, mergeFieldProse)

	srv, session := newContractServer(t, db, contractorUID)
	defer srv.Close()

	out := decodeAwaiting(t, session, awaitingURL(srv, practiceID))

	if len(out.Items) != 0 {
		t.Fatalf("items = %+v, want none for a contractor with no granted attachment", out.Items)
	}
}

// TestAwaitingSignatureHandler_ContractorPaginatesAcrossPages exercises
// listAttachedAwaiting's own cursor branch (the `after != nil` path),
// the contractor mirror of
// TestAwaitingSignatureHandler_WalksTheCursor below.
func TestAwaitingSignatureHandler_ContractorPaginatesAcrossPages(t *testing.T) {
	db := testdb.New(t)
	const contractorUID = "awaiting-contractor-pages"
	practiceID := testdb.SeedPractice(t, db, "Awaiting Contractor Pages Practice")
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)

	const total = 31 // awaitingPageSize (30) + 1, to force a second page
	for range total {
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)
		testdb.SeedGrantedAttachment(t, db, engagementID, contractorID)
		seedContract(t, db, engagementID, statusSent, mergeFieldProse)
	}

	srv, session := newContractServer(t, db, contractorUID)
	defer srv.Close()

	first := decodeAwaiting(t, session, awaitingURL(srv, practiceID))
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	second := decodeAwaiting(t, session, awaitingURL(srv, practiceID)+"?cursor="+*first.NextCursor)
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
	if second.Items[0].ContractID == first.Items[29].ContractID {
		t.Fatal("the second page repeated the first page's last row")
	}
}

// TestAwaitingSignatureHandler_RejectsAMalformedCursor proves a cursor
// nobody this endpoint issued is refused rather than silently treated as
// page one, in docs/api-design.md section 7's structured shape.
func TestAwaitingSignatureHandler_RejectsAMalformedCursor(t *testing.T) {
	db := testdb.New(t)
	const uid = "awaiting-bad-cursor"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	for _, cursor := range []string{"not!valid!base64!", "YmFkdGltZXxzb21lLWlk"} {
		resp := getAwaiting(t, session, awaitingURL(srv, practiceID)+"?cursor="+cursor)
		out := apierrtest.Decode(t, resp)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("cursor %q: status = %d, want %d", cursor, resp.StatusCode, http.StatusBadRequest)
		}
		if out.Code != "INVALID_ARGUMENT" || out.Message != contracts.MsgInvalidCursor {
			t.Fatalf("cursor %q: error = %+v, want the structured refusal", cursor, out)
		}
	}
}

// TestAwaitingSignatureHandler_WalksTheCursor proves the envelope
// docs/api-design.md section 4 asks for: a full first page carries a
// cursor, and that cursor resumes at the row after the last one rather
// than repeating it.
func TestAwaitingSignatureHandler_WalksTheCursor(t *testing.T) {
	db := testdb.New(t)
	const uid = "awaiting-cursor"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	const total = 31 // awaitingPageSize (30) + 1, to force a second page
	for range total {
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)
		seedContract(t, db, engagementID, statusSent, mergeFieldProse)
	}

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	first := decodeAwaiting(t, session, awaitingURL(srv, practiceID))
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	second := decodeAwaiting(t, session, awaitingURL(srv, practiceID)+"?cursor="+*first.NextCursor)
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
	if second.Items[0].ContractID == first.Items[29].ContractID {
		t.Fatal("the second page repeated the first page's last row")
	}
}
