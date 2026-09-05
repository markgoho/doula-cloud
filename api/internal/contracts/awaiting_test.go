package contracts_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/testdb"
)

const adminRole = "admin"

// seedAdmin seeds a Practice and a Staff member holding the 'admin' role
// there -- the second of the two seats AwaitingSignatureHandler admits.
func seedAdmin(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()
	practiceID = seedPractice(t, db, "Test Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{adminRole}, "employee")
	return practiceID
}

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
	return srv.URL + "/practices/" + practiceID + "/contracts/awaiting-signature"
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
	practiceID := seedOwner(t, db, uid)

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
	practiceID := seedOwner(t, db, uid)

	draftEngagement, draftClient := seedNamedEngagement(t, db, practiceID, "Jamesina", jamieName)
	seedContract(t, db, draftEngagement, statusDraft, mergeFieldProse)
	sentEngagement, _ := seedNamedEngagement(t, db, practiceID, "Renata", "")
	seedContract(t, db, sentEngagement, statusSent, mergeFieldProse)
	seedContract(t, db, seedEngagement(t, db, practiceID), statusSigned, mergeFieldProse)
	voidedEngagement := seedEngagement(t, db, practiceID)
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
	practiceID := seedOwner(t, db, uid)
	otherPracticeID := seedPractice(t, db, "Other Practice")
	seedContract(t, db, seedEngagement(t, db, otherPracticeID), statusSent, mergeFieldProse)
	mine := seedEngagement(t, db, practiceID)
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
	practiceID := seedAdmin(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusSent, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	out := decodeAwaiting(t, session, awaitingURL(srv, practiceID))

	if len(out.Items) != 1 || out.Items[0].EngagementID != engagementID {
		t.Fatalf("items = %+v, want the outstanding Contract", out.Items)
	}
}

// TestAwaitingSignatureHandler_RefusesADoula proves the gate ADR-0008's
// read table draws: the Practice's book of outstanding agreements is not
// a Doula's to read, the same seat the credit balance holds.
func TestAwaitingSignatureHandler_RefusesADoula(t *testing.T) {
	db := testdb.New(t)
	const uid = "awaiting-doula"
	practiceID := seedMember(t, db, uid)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getAwaiting(t, session, awaitingURL(srv, practiceID))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestAwaitingSignatureHandler_RejectsAMalformedCursor proves a cursor
// nobody this endpoint issued is refused rather than silently treated as
// page one, in docs/api-design.md section 7's structured shape.
func TestAwaitingSignatureHandler_RejectsAMalformedCursor(t *testing.T) {
	db := testdb.New(t)
	const uid = "awaiting-bad-cursor"
	practiceID := seedOwner(t, db, uid)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	for _, cursor := range []string{"not!valid!base64!", "YmFkdGltZXxzb21lLWlk"} {
		resp := getAwaiting(t, session, awaitingURL(srv, practiceID)+"?cursor="+cursor)
		var out apierr.APIError
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("cursor %q: decode body: %v", cursor, err)
		}
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
	practiceID := seedOwner(t, db, uid)
	const total = 31 // awaitingPageSize (30) + 1, to force a second page
	for range total {
		seedContract(t, db, seedEngagement(t, db, practiceID), statusSent, mergeFieldProse)
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
