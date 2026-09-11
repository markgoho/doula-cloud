package client_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

func authedGet(t *testing.T, session, url string) *http.Response {
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

func authedJSON(t *testing.T, session, method, url string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, url, bytes.NewReader(raw))
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

// TestCreateHandler_MinimalSavesFreely proves the headline AC: a Client
// can be created with only a first name, and the save is free -- no
// credit_ledger row, and no engagements row.
func TestCreateHandler_MinimalSavesFreely(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-minimal-create"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients",
		client.CreateRequest{Record: client.Record{GivenName: "Jamie"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var rec client.Record
	if err := json.NewDecoder(resp.Body).Decode(&rec); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if rec.GivenName != "Jamie" || rec.ID == "" {
		t.Fatalf("unexpected record: %+v", rec)
	}

	var creditRows, engagementRows int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM credit_ledger WHERE practice_id = $1`, practiceID).Scan(&creditRows); err != nil {
		t.Fatalf("count credit_ledger: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM engagements WHERE practice_id = $1`, practiceID).Scan(&engagementRows); err != nil {
		t.Fatalf("count engagements: %v", err)
	}
	if creditRows != 0 || engagementRows != 0 {
		t.Fatalf("creditRows = %d, engagementRows = %d, want 0 and 0", creditRows, engagementRows)
	}

	var eventCount int
	var eventType string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(action) FROM activity WHERE subject_kind = 'client' AND subject_id = $1`, rec.ID,
	).Scan(&eventCount, &eventType); err != nil {
		t.Fatalf("count activity: %v", err)
	}
	if eventCount != 1 || eventType != "created" {
		t.Fatalf("activity count = %d type = %q, want 1 and \"created\"", eventCount, eventType)
	}
}

func TestCreateHandler_InvalidBodyAndInvalidDateOfBirth(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-create-validation"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	badBody, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients", bytes.NewReader([]byte(`{`)))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(badBody, session)
	badBodyResp, err := http.DefaultClient.Do(badBody)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer badBodyResp.Body.Close()
	if badBodyResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid body status = %d, want %d", badBodyResp.StatusCode, http.StatusBadRequest)
	}

	badDate := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients",
		client.CreateRequest{Record: client.Record{GivenName: "Bad Date", DateOfBirth: "not-a-date"}})
	defer badDate.Body.Close()
	if badDate.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid dateOfBirth status = %d, want %d", badDate.StatusCode, http.StatusBadRequest)
	}
}

func TestCreateHandler_MissingGivenName(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-missing-name"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients", client.CreateRequest{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestCreateHandler_RefusesContractor proves ADR-0017's "a contractor
// originates nothing": a contractor Doula's create is refused at the
// endpoint with a message distinguishable from a bare 403.
func TestCreateHandler_RefusesContractor(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-for-contractor", []string{doulaRole}, "employee")
	const contractorUID = "contractor-creating"
	testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients",
		client.CreateRequest{Record: client.Record{GivenName: "Refused"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	body := readBody(t, resp)
	if body == "" || body == "\n" {
		t.Fatalf("expected a distinguishable error message, got %q", body)
	}
}

// TestCreateHandler_OwnerWithContractorEmploymentTypeMayCreate proves the
// refusal is role-gated, not bare employment_type: the "solo Practice"
// case stays in the Owner column.
func TestCreateHandler_OwnerWithContractorEmploymentTypeMayCreate(t *testing.T) {
	db := testdb.New(t)
	var practiceID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name, timezone) VALUES ('Solo Practice', 'America/New_York') RETURNING id`,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	const uid = "owner-contractor-creating"
	testdb.SeedStaffAtPractice(t, db, practiceID, uid, []string{ownerRole, doulaRole}, "contractor")

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients",
		client.CreateRequest{Record: client.Record{GivenName: "Solo Client"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
}

// TestCreateHandler_RefusesOnMatchAndOverrideProceeds proves
// lookup-before-insert: a create whose details match an existing Client
// is refused with the matches, and Override lets it proceed anyway.
func TestCreateHandler_RefusesOnMatchAndOverrideProceeds(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-dup-create"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	existingID := testdb.SeedNamedClient(t, db, practiceID, "Sarah Beck", "sarah@example.com")
	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients",
		client.CreateRequest{Record: client.Record{GivenName: testSarah, Email: "sarah@example.com"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	var out client.CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Matches) != 1 || out.Matches[0].ID != existingID {
		t.Fatalf("matches = %+v, want one match on %q", out.Matches, existingID)
	}

	overridden := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients",
		client.CreateRequest{Record: client.Record{GivenName: testSarah, Email: "sarah@example.com"}, Override: true})
	defer overridden.Body.Close()
	if overridden.StatusCode != http.StatusCreated {
		t.Fatalf("overridden status = %d, want %d", overridden.StatusCode, http.StatusCreated)
	}
}

// TestSearchHandler_MatchesNameDOBEmailPhoneWithinPractice proves the
// search matches on all four keys and never returns a Client from
// another Practice.
func TestSearchHandler_MatchesNameDOBEmailPhoneWithinPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-searching"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	otherPracticeID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-other-practice-search", []string{doulaRole}, "employee")
	inPractice := testdb.SeedNamedClient(t, db, practiceID, "Nadia Haddad", "nadia@example.com")
	testdb.SeedNamedClient(t, db, otherPracticeID, "Nadia Haddad", "nadia@example.com")
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE clients SET phone = '555-0100', date_of_birth = '1990-03-02' WHERE id = $1`, inPractice); err != nil {
		t.Fatalf("set phone/dob: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	cases := []string{
		"?name=Nadia",
		"?dateOfBirth=1990-03-02",
		"?email=nadia@example.com",
		"?phone=555-0100",
	}
	for _, q := range cases {
		resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/search"+q)
		var out client.SearchResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode response for %q: %v", q, err)
		}
		_ = resp.Body.Close()
		if len(out.Matches) != 1 || out.Matches[0].ID != inPractice {
			t.Fatalf("query %q: matches = %+v, want exactly the in-practice client", q, out.Matches)
		}
	}
}

// TestEditHandler_WhoeverMayReadMayEdit proves the contractor
// attached/unattached split on edit access.
func TestEditHandler_WhoeverMayReadMayEdit(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-for-edit", []string{doulaRole}, "employee")
	clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Edit Client", "edit@example.com", "active")

	const unattachedUID = "contractor-unattached-edit"
	testdb.SeedContractorAtPractice(t, db, practiceID, unattachedUID)
	srvUnattached, sessionUnattached := newServer(t, db, unattachedUID)
	defer srvUnattached.Close()
	respUnattached := authedJSON(t, sessionUnattached, http.MethodPut, srvUnattached.URL+"/api/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{GivenName: "Edit Client"}})
	defer respUnattached.Body.Close()
	if respUnattached.StatusCode != http.StatusNotFound {
		t.Fatalf("unattached contractor status = %d, want %d", respUnattached.StatusCode, http.StatusNotFound)
	}

	const attachedUID = "contractor-attached-edit"
	attachedStaffID := testdb.SeedContractorAtPractice(t, db, practiceID, attachedUID)
	testdb.SeedGrantedAttachment(t, db, engagementID, attachedStaffID)
	srvAttached, sessionAttached := newServer(t, db, attachedUID)
	defer srvAttached.Close()
	respAttached := authedJSON(t, sessionAttached, http.MethodPut, srvAttached.URL+"/api/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{GivenName: "Edited By Attached Contractor"}})
	defer respAttached.Body.Close()
	if respAttached.StatusCode != http.StatusOK {
		t.Fatalf("attached contractor status = %d, want %d", respAttached.StatusCode, http.StatusOK)
	}
}

// TestEditHandler_RefusesOnMatchWithDifferentClientAndOverrideProceeds
// proves #373's name-rule block: an edit that would make the record
// match a different Client is refused, and the single override succeeds.
func TestEditHandler_RefusesOnMatchWithDifferentClientAndOverrideProceeds(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-edit-match"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	testdb.SeedNamedClient(t, db, practiceID, "Nadia Haddad", "nadia@example.com")
	editingID := testdb.SeedNamedClient(t, db, practiceID, "Sara Beck", "sara@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+editingID,
		client.EditRequest{Record: client.Record{GivenName: testNadia, FamilyName: testHaddad, Email: "nadia@example.com"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}

	overridden := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+editingID,
		client.EditRequest{Record: client.Record{GivenName: testNadia, FamilyName: testHaddad, Email: "nadia@example.com"}, Override: true})
	defer overridden.Body.Close()
	if overridden.StatusCode != http.StatusOK {
		t.Fatalf("overridden status = %d, want %d", overridden.StatusCode, http.StatusOK)
	}
}

// TestEditHandler_ChangingEmailRevokesPendingInvite proves ADR-0017's
// rule that editing a Client's address revokes any pending portal
// invite in the same transaction.
func TestEditHandler_ChangingEmailRevokesPendingInvite(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-revoking"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Revoke Client", "old@example.com")
	outboxID := seedPendingOutboxRow(t, db, clientID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{GivenName: "Revoke Client", Email: testNewEmail}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	status, lastErr := outboxStatus(t, db, outboxID)
	if status != deadLetteredStatus || lastErr == "" {
		t.Fatalf("outbox row status = %q lastError = %q, want dead_lettered with a reason", status, lastErr)
	}
}

// TestEditHandler_EveryEditWritesOneClientEvent proves the activity
// AC: an edit that actually changes a fact writes exactly one row, with
// a diff and a named actor.
func TestEditHandler_EveryEditWritesOneClientEvent(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-editing-events"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Event Client", "event@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{GivenName: "Event Client", Phone: "555-0199"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var eventCount int
	var eventType, actorKind string
	var actorStaffID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(action), max(actor_kind::text), max(actor_staff_id::text)
		 FROM activity WHERE subject_kind = 'client' AND subject_id = $1 AND action = 'updated'`, clientID,
	).Scan(&eventCount, &eventType, &actorKind, &actorStaffID); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if eventCount != 1 || eventType != "updated" || actorKind != "staff" || actorStaffID == "" {
		t.Fatalf("eventCount=%d eventType=%q actorKind=%q actorStaffID=%q, want 1 updated staff <id>", eventCount, eventType, actorKind, actorStaffID)
	}
}

// TestEditHandler_NoChangeStillWritesOneEmptyDiffEvent proves ADR-0017's
// "one row per act": even a no-op edit (identical values resubmitted)
// writes exactly one activity row, with an empty diff.
func TestEditHandler_NoChangeStillWritesOneEmptyDiffEvent(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-noop-edit"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Noop Client", "noop@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{GivenName: "Noop Client", Email: "noop@example.com"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE subject_kind = 'client' AND subject_id = $1`, clientID,
	).Scan(&count); err != nil {
		t.Fatalf("count activity: %v", err)
	}
	if count != 1 {
		t.Fatalf("activity count = %d, want 1", count)
	}
	// The stored diff is sealed under her key (ADR-0027), so the empty
	// diff is asserted through the read path that unseals it rather than
	// against the column.
	if diff := readClientEventDiff(t, db, session, srv, practiceID, clientID, "updated"); string(diff) != "{}" {
		t.Fatalf("diff = %s, want an empty diff", diff)
	}
}

// readClientEventDiff reads one client-subject activity entry's diff
// back through the detail endpoint -- the only reader that unseals it.
// Asserting against the activity column directly stopped being possible
// with #394: what is stored there is ciphertext.
func readClientEventDiff(t *testing.T, db *testdb.DB, session string, srv *httptest.Server, practiceID, clientID, action string) json.RawMessage {
	t.Helper()
	_ = db
	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID)
	defer resp.Body.Close()
	var out client.DetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	for _, entry := range out.History {
		if entry.ClientEvent != nil && entry.ClientEvent.EventType == action {
			return entry.ClientEvent.Diff
		}
	}
	t.Fatalf("no %q client event in history %+v", action, out.History)
	return nil
}

// TestEditHandler_FieldValuesChangeIsDiffedAsOneWholeBlob proves
// diffRecords' field_values branch: a changed Practice-defined values
// blob is recorded as one "fieldValues" diff entry.
func TestEditHandler_FieldValuesChangeIsDiffedAsOneWholeBlob(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-field-values"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Field Values Client", "fieldvalues@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{GivenName: "Field Values Client", FieldValues: json.RawMessage(`{"favoriteColor":"blue"}`)}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	diffJSON := readClientEventDiff(t, db, session, srv, practiceID, clientID, "updated")
	var diff map[string]json.RawMessage
	if err := json.Unmarshal(diffJSON, &diff); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if _, ok := diff["fieldValues"]; !ok {
		t.Fatalf("diff = %s, want a fieldValues entry", diffJSON)
	}
}

// TestDetailHandler_ReturnsRecordEngagementsAndHistory proves the detail
// read: her record, her Engagements, and activity merged with
// engagement_requests.
func TestDetailHandler_ReturnsRecordEngagementsAndHistory(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-detail"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Detail Client", "detail@example.com", "active")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out client.DetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ID != clientID || len(out.Engagements) != 1 || out.Engagements[0].EngagementID != engagementID {
		t.Fatalf("unexpected detail: %+v", out)
	}
}

// TestDetailHandler_ResolvedFieldsCoverActiveBlankArchivedHeldAndDropped
// proves #399's AC3 and AC5: an active field always appears (even
// blank), an archived field appears only when the Client holds a stored
// value under it and is marked "No longer collected", and an archived
// field with no stored value does not appear at all.
func TestDetailHandler_ResolvedFieldsCoverActiveBlankArchivedHeldAndDropped(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-resolved-fields"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Resolved Fields Client", "resolved@example.com")

	seedFieldTemplate(t, db, practiceID, `[
		{"id":"active_blank","type":"short_text","label":"Pronouns","order":0,"archived":false},
		{"id":"active_held","type":"short_text","label":"Intake note","order":1,"archived":false},
		{"id":"archived_held","type":"short_text","label":"Old note","order":2,"archived":true},
		{"id":"archived_blank","type":"short_text","label":"Never filled in","order":3,"archived":true}
	]`)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE clients SET field_values = $1 WHERE id = $2`,
		`{"active_held":"She/her","archived_held":"Kept for the record"}`, clientID,
	); err != nil {
		t.Fatalf("seed field values: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID)
	defer resp.Body.Close()
	var out client.DetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	byID := map[string]client.ResolvedField{}
	for _, f := range out.ResolvedFields {
		byID[f.FieldID] = f
	}
	if _, dropped := byID["archived_blank"]; dropped {
		t.Fatalf("resolved fields = %+v, want archived_blank dropped", out.ResolvedFields)
	}
	if len(byID) != 3 {
		t.Fatalf("resolved fields = %+v, want exactly 3", out.ResolvedFields)
	}

	active := byID["active_blank"]
	if active.Note != "" || string(active.Value) != "null" {
		t.Fatalf("active_blank = %+v, want no note and a null value", active)
	}
	held := byID["active_held"]
	if held.Note != "" || string(held.Value) != `"She/her"` {
		t.Fatalf("active_held = %+v, want no note and its stored value", held)
	}
	archived := byID["archived_held"]
	if archived.Note != "No longer collected" || string(archived.Value) != `"Kept for the record"` {
		t.Fatalf("archived_held = %+v, want the \"No longer collected\" note and its stored value", archived)
	}
}

// TestDetailHandler_ResolvedFieldsAreLiveNotSnapshotted proves ADR-0017's
// departure from ADR-0001: editing the Practice's Client Field Template
// after a Client already exists changes what her detail read shows, on
// the very next read -- nothing is snapshotted at intake.
func TestDetailHandler_ResolvedFieldsAreLiveNotSnapshotted(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-live-template"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Live Template Client", "live@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	before := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID)
	var beforeOut client.DetailResponse
	if err := json.NewDecoder(before.Body).Decode(&beforeOut); err != nil {
		t.Fatalf("decode before response: %v", err)
	}
	_ = before.Body.Close()
	if len(beforeOut.ResolvedFields) != 0 {
		t.Fatalf("resolved fields before template exists = %+v, want none", beforeOut.ResolvedFields)
	}

	seedFieldTemplate(t, db, practiceID, `[{"id":"new_field","type":"short_text","label":"Added later","order":0,"archived":false}]`)

	after := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID)
	defer after.Body.Close()
	var afterOut client.DetailResponse
	if err := json.NewDecoder(after.Body).Decode(&afterOut); err != nil {
		t.Fatalf("decode after response: %v", err)
	}
	if len(afterOut.ResolvedFields) != 1 || afterOut.ResolvedFields[0].FieldID != "new_field" {
		t.Fatalf("resolved fields after adding a field to the template = %+v, want one field new_field", afterOut.ResolvedFields)
	}
}

// TestListHandler_ClientShapedDefaultFiltersToWork proves the Clients
// list returns one row per Client (not one per Client+Engagement pair)
// and defaults to Clients with work.
func TestListHandler_ClientShapedDefaultFiltersToWork(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-listing"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	withTwoEngagements, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Two Engagements", "two@example.com", "active")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status, kind, ending_reason, birth_outcome)
		 VALUES ($1, $2, 'completed', 'postpartum', 'care_complete', 'unknown')`,
		withTwoEngagements, practiceID,
	); err != nil {
		t.Fatalf("seed second engagement: %v", err)
	}
	noWork := testdb.SeedNamedClient(t, db, practiceID, "No Work Yet", "nowork@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	list := listResp.Items
	if len(list) != 1 || list[0].ClientID != withTwoEngagements {
		t.Fatalf("default list = %+v, want exactly one row for the two-Engagement Client", list)
	}

	all := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
	defer all.Body.Close()
	var allListResp client.ListResponse
	if err := json.NewDecoder(all.Body).Decode(&allListResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	allList := allListResp.Items
	if len(allList) != 2 {
		t.Fatalf("all=true list length = %d, want 2", len(allList))
	}
	found := false
	for _, item := range allList {
		if item.ClientID == noWork {
			found = true
		}
	}
	if !found {
		t.Fatalf("all=true list %+v missing the no-work Client", allList)
	}
}

// TestListHandler_PendingRequestKindsOnRow proves ADR-0017's "Refusal, and
// the Client it leaves behind" and #499's row-level pending indicator
// together: a pending Request's kind(s) surface on the row, a refused-only
// Client carries none, has no work, and appears only under "see everyone".
func TestListHandler_PendingRequestKindsOnRow(t *testing.T) {
	const birthKind = "birth"

	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	const identityUID = "staff-pending-request"
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{doulaRole}, "employee")

	pendingBirth := testdb.SeedNamedClient(t, db, practiceID, "Pending Birth", "pending-birth@example.com")
	seedPendingRequest(t, db, practiceID, pendingBirth, staffID, birthKind)

	pendingBoth := testdb.SeedNamedClient(t, db, practiceID, "Pending Both", "pending-both@example.com")
	seedPendingRequest(t, db, practiceID, pendingBoth, staffID, birthKind)
	seedPendingRequest(t, db, practiceID, pendingBoth, staffID, "postpartum")

	refusedOnly := testdb.SeedNamedClient(t, db, practiceID, "Refused Only", "refused-only@example.com")
	seedRefusedRequest(t, db, practiceID, refusedOnly, staffID, birthKind)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	all := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
	defer all.Body.Close()
	var allResp client.ListResponse
	if err := json.NewDecoder(all.Body).Decode(&allResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	byID := map[string]client.ListItem{}
	for _, item := range allResp.Items {
		byID[item.ClientID] = item
	}

	if got := byID[pendingBirth].PendingRequestKinds; len(got) != 1 || got[0] != birthKind {
		t.Fatalf("pending-birth kinds = %v, want [birth]", got)
	}
	if got := byID[pendingBoth].PendingRequestKinds; len(got) != 2 || got[0] != birthKind || got[1] != "postpartum" {
		t.Fatalf("pending-both kinds = %v, want [birth postpartum]", got)
	}
	if got := byID[refusedOnly].PendingRequestKinds; len(got) != 0 {
		t.Fatalf("refused-only kinds = %v, want empty", got)
	}
	if byID[refusedOnly].HasWork {
		t.Fatalf("refused-only HasWork = true, want false")
	}

	def := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients")
	defer def.Body.Close()
	var defResp client.ListResponse
	if err := json.NewDecoder(def.Body).Decode(&defResp); err != nil {
		t.Fatalf("decode default response: %v", err)
	}
	defByID := map[string]client.ListItem{}
	for _, item := range defResp.Items {
		defByID[item.ClientID] = item
	}
	if _, ok := defByID[refusedOnly]; ok {
		t.Fatalf("default filter unexpectedly includes the refused-only Client")
	}
	if got := defByID[pendingBirth].PendingRequestKinds; len(got) != 1 || got[0] != birthKind {
		t.Fatalf("default filter's pending-birth kinds = %v, want [birth]", got)
	}
}

// TestListHandler_PortalInviteStatusVariants proves the four shapes
// PortalInviteStatus can take: nil (never invited), "pending" (an
// outbox row), "accepted" (identity_uid set, taking precedence over
// whatever the outbox says), and nil again for a portal user row with no
// outbox row at all (shouldn't occur, per portalInviteStatus's doc
// comment, but reads the same as never invited rather than as a
// distinct state).
func TestListHandler_PortalInviteStatusVariants(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-portal-status"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	neverInvited, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Never Invited", "never@example.com", "active")
	invited, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Invited Client", "invited@example.com", "active")
	seedPendingOutboxRow(t, db, invited)
	accepted, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Accepted Client", "accepted@example.com", "active")
	seedAcceptedPortalUser(t, db, accepted)
	portalUserNoOutbox, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "No Outbox Client", "no-outbox@example.com", "active")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid())`, portalUserNoOutbox,
	); err != nil {
		t.Fatalf("seed portal user with no outbox row: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	byID := map[string]client.ListItem{}
	for _, item := range listResp.Items {
		byID[item.ClientID] = item
	}
	if byID[neverInvited].PortalInviteStatus != nil {
		t.Fatalf("never-invited status = %v, want nil", byID[neverInvited].PortalInviteStatus)
	}
	if got := byID[invited].PortalInviteStatus; got == nil || *got != pendingStatus {
		t.Fatalf("invited status = %v, want \"pending\"", got)
	}
	if got := byID[accepted].PortalInviteStatus; got == nil || *got != "accepted" {
		t.Fatalf("accepted status = %v, want \"accepted\"", got)
	}
	if byID[portalUserNoOutbox].PortalInviteStatus != nil {
		t.Fatalf("no-outbox status = %v, want nil", byID[portalUserNoOutbox].PortalInviteStatus)
	}
}

// TestListHandler_EmailSuppressed proves the #785 column on both list
// queries at once -- the ambient one an Owner gets and the narrowed one
// listAttachedClients builds for a contractor are separate SELECTs, so a
// column added to one proves nothing about the other. It also pins the
// two facts the Clients list's own wording rests on: an address is
// suppressed only while cleared_at is null, and the comparison is
// case-insensitive on the Client's side (00068 stores the address
// lower-cased and the send-time guard compares it lower-cased, so the
// list has to agree with the guard that will refuse the next invite).
func TestListHandler_EmailSuppressed(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "staff-owner-suppression-list"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, ownerUID, []string{doulaRole}, "employee")
	suppressed, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Suppressed Client", "Blocked@Example.com", "active")
	cleared, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Cleared Client", "cleared@example.com", "active")
	untouched, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Untouched Client", "fine@example.com", "active")

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO email_suppressions (address, cause) VALUES ($1, 'bounce'), ($2, 'bounce')`,
		"blocked@example.com", "cleared@example.com",
	); err != nil {
		t.Fatalf("seed suppressions: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE email_suppressions SET cleared_at = now() WHERE address = $1`, "cleared@example.com",
	); err != nil {
		t.Fatalf("clear suppression: %v", err)
	}

	const contractorUID = "contractor-suppression-list"
	contractorStaffID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	testdb.SeedGrantedAttachment(t, db, engagementID, contractorStaffID)

	for _, tc := range []struct {
		name string
		uid  string
		want map[string]bool
	}{
		{
			name: "owner sees every Client's suppression",
			uid:  ownerUID,
			want: map[string]bool{suppressed: true, cleared: false, untouched: false},
		},
		{
			// The contractor reaches only the attached Client, which is
			// the suppressed one -- enough to prove the narrowed query
			// carries the column at all.
			name: "contractor's narrowed list carries it too",
			uid:  contractorUID,
			want: map[string]bool{suppressed: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, session := newServer(t, db, tc.uid)
			defer srv.Close()

			resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
			defer resp.Body.Close()
			var listResp client.ListResponse
			if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			byID := map[string]client.ListItem{}
			for _, item := range listResp.Items {
				byID[item.ClientID] = item
			}
			for clientID, want := range tc.want {
				item, ok := byID[clientID]
				if !ok {
					t.Fatalf("client %s missing from list", clientID)
				}
				if item.EmailSuppressed != want {
					t.Fatalf("client %s EmailSuppressed = %v, want %v", clientID, item.EmailSuppressed, want)
				}
			}
		})
	}
}

// TestListHandler_ContractorSeesOnlyAttachedClients proves
// listAttachedClients: a contractor's list narrows to Clients she holds
// a granted attachment to, and an unattached Client never appears.
func TestListHandler_ContractorSeesOnlyAttachedClients(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-for-contractor-list", []string{doulaRole}, "employee")
	attachedClient, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Attached Client", "attached@example.com", "active")
	testdb.SeedEngagementInStatus(t, db, practiceID, "Unattached Client", "unattached@example.com", "active")

	const contractorUID = "contractor-listing"
	staffID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)
	seedPendingRequest(t, db, practiceID, attachedClient, staffID, "birth")

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	list := listResp.Items
	if len(list) != 1 || list[0].ClientID != attachedClient {
		t.Fatalf("contractor list = %+v, want exactly the attached client", list)
	}
	// The narrowed query is a second SELECT, not the same one with an
	// extra predicate, so PendingRequestKinds has to be proved on this
	// path as well as on the Practice-wide one -- the two could drift.
	if len(list[0].PendingRequestKinds) != 1 || list[0].PendingRequestKinds[0] != "birth" {
		t.Fatalf("attached client PendingRequestKinds = %v, want [birth]", list[0].PendingRequestKinds)
	}
}

// TestSearchHandler_RefusesContractorAndEmptyQueryReturnsNoMatches proves
// the contractor refusal and the empty-query short circuit both return
// cleanly rather than erroring.
func TestSearchHandler_RefusesContractorAndEmptyQueryReturnsNoMatches(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-for-search-refusal", []string{doulaRole}, "employee")
	const contractorUID = "contractor-searching"
	testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)

	srvContractor, sessionContractor := newServer(t, db, contractorUID)
	defer srvContractor.Close()
	respContractor := authedGet(t, sessionContractor, srvContractor.URL+"/api/practices/"+practiceID+"/clients/search?name=Anyone")
	defer respContractor.Body.Close()
	if respContractor.StatusCode != http.StatusForbidden {
		t.Fatalf("contractor search status = %d, want %d", respContractor.StatusCode, http.StatusForbidden)
	}

	const ownerUID = "staff-empty-search"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{doulaRole}, "employee")
	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()
	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/search")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("empty query status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out client.SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Matches) != 0 {
		t.Fatalf("matches = %+v, want none for an empty query", out.Matches)
	}
}

func TestEditHandler_InvalidClientIDAndBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-edit-validation"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Validation Client", "validation@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	badID := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/not-a-uuid",
		client.EditRequest{Record: client.Record{GivenName: "Whoever"}})
	defer badID.Body.Close()
	if badID.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid clientId status = %d, want %d", badID.StatusCode, http.StatusBadRequest)
	}

	missingRequest, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		srv.URL+"/api/practices/"+practiceID+"/clients/00000000-0000-0000-0000-000000000000", bytes.NewReader([]byte(`{"givenName":"Ghost"}`)))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(missingRequest, session)
	missingResp, err := http.DefaultClient.Do(missingRequest)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer missingResp.Body.Close()
	if missingResp.StatusCode != http.StatusNotFound {
		t.Fatalf("nonexistent clientId status = %d, want %d", missingResp.StatusCode, http.StatusNotFound)
	}

	badBody, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID, bytes.NewReader([]byte(`{`)))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(badBody, session)
	badBodyResp, err := http.DefaultClient.Do(badBody)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer badBodyResp.Body.Close()
	if badBodyResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid body status = %d, want %d", badBodyResp.StatusCode, http.StatusBadRequest)
	}

	missingGivenName := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{})
	defer missingGivenName.Body.Close()
	if missingGivenName.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing givenName status = %d, want %d", missingGivenName.StatusCode, http.StatusBadRequest)
	}
}

// TestDetailHandler_InvalidAndMissing proves the invalid-clientId and
// valid-but-nonexistent-clientId paths, and the contractor-unattached
// carve-out.
func TestDetailHandler_InvalidAndMissing(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-detail-validation", []string{doulaRole}, "employee")
	srv, session := newServer(t, db, "staff-detail-validation")
	defer srv.Close()

	badID := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/not-a-uuid")
	defer badID.Body.Close()
	if badID.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid clientId status = %d, want %d", badID.StatusCode, http.StatusBadRequest)
	}

	missing := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/00000000-0000-0000-0000-000000000000")
	defer missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("nonexistent clientId status = %d, want %d", missing.StatusCode, http.StatusNotFound)
	}

	const contractorUID = "contractor-unattached-client-detail"
	testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	unattachedID := testdb.SeedNamedClient(t, db, practiceID, "Unattached Detail Client", "unattached-cd@example.com")
	srvContractor, sessionContractor := newServer(t, db, contractorUID)
	defer srvContractor.Close()
	forbidden := authedGet(t, sessionContractor, srvContractor.URL+"/api/practices/"+practiceID+"/clients/"+unattachedID)
	defer forbidden.Body.Close()
	if forbidden.StatusCode != http.StatusNotFound {
		t.Fatalf("unattached contractor detail status = %d, want %d", forbidden.StatusCode, http.StatusNotFound)
	}
}

// TestDetailHandler_MergesEventsAndRequestsIntoHistory proves the merged
// history: activity rows (written by an actual create then edit)
// interleaved with engagement_requests rows, newest first.
func TestDetailHandler_MergesEventsAndRequestsIntoHistory(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-history"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	created := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients",
		client.CreateRequest{Record: client.Record{GivenName: "History Client"}})
	var rec client.Record
	if err := json.NewDecoder(created.Body).Decode(&rec); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	_ = created.Body.Close()

	edited := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+rec.ID,
		client.EditRequest{Record: client.Record{GivenName: "History Client", Phone: "555-0177"}})
	_ = edited.Body.Close()

	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "requesting-staff-history", []string{doulaRole}, "employee")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, requested_by, state)
		 VALUES ($1, $2, 'birth', $3, 'pending')`,
		practiceID, rec.ID, staffID,
	); err != nil {
		t.Fatalf("seed pending request: %v", err)
	}
	var approverID string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT id FROM staff WHERE identity_uid = $1`, identityUID).Scan(&approverID); err != nil {
		t.Fatalf("read approver id: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, requested_by, state, decided_by, decided_at, reason)
		 VALUES ($1, $2, 'postpartum', $3, 'refused', $4, now(), 'Practice at capacity')`,
		practiceID, rec.ID, staffID, approverID,
	); err != nil {
		t.Fatalf("seed decided request: %v", err)
	}
	var approvedEngagementID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		rec.ID, practiceID,
	).Scan(&approvedEngagementID); err != nil {
		t.Fatalf("seed engagement for approved request: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, requested_by, state, decided_by, decided_at, engagement_id)
		 VALUES ($1, $2, 'birth', $3, 'approved', $4, now(), $5)`,
		practiceID, rec.ID, staffID, approverID, approvedEngagementID,
	); err != nil {
		t.Fatalf("seed approved request: %v", err)
	}

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/"+rec.ID)
	defer resp.Body.Close()
	var out client.DetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.History) != 5 {
		t.Fatalf("history length = %d, want 5", len(out.History))
	}
	var events, requests int
	var sawDecided, sawApproved bool
	for _, h := range out.History {
		switch h.Type {
		case "client_event":
			events++
			if h.ClientEvent.ActorName == "" {
				t.Fatalf("client event missing actorName: %+v", h.ClientEvent)
			}
		case "engagement_request":
			requests++
			if h.EngagementRequest.RequestedByName != "Test Staff requesting-staff-history" {
				t.Fatalf("requestedByName = %q, want %q", h.EngagementRequest.RequestedByName, "Test Staff requesting-staff-history")
			}
			if h.EngagementRequest.State == "approved" {
				sawApproved = true
				if h.EngagementRequest.EngagementID == nil || *h.EngagementRequest.EngagementID != approvedEngagementID {
					t.Fatalf("approved request engagementId = %v, want %q", h.EngagementRequest.EngagementID, approvedEngagementID)
				}
			}
			if h.EngagementRequest.State == "refused" {
				sawDecided = true
				if h.EngagementRequest.DecidedBy == nil || h.EngagementRequest.Reason == nil {
					t.Fatalf("decided request missing decidedBy/reason: %+v", h.EngagementRequest)
				}
				if h.EngagementRequest.DecidedByName == nil || *h.EngagementRequest.DecidedByName == "" {
					t.Fatalf("decided request missing decidedByName: %+v", h.EngagementRequest)
				}
			}
			if h.EngagementRequest.State == "pending" && h.EngagementRequest.DecidedByName != nil {
				t.Fatalf("pending request should have no decidedByName: %+v", h.EngagementRequest)
			}
		}
	}
	if events != 2 || requests != 3 || !sawDecided || !sawApproved {
		t.Fatalf("events=%d requests=%d sawDecided=%v sawApproved=%v, want 2/3/true/true", events, requests, sawDecided, sawApproved)
	}
	for i := 1; i < len(out.History); i++ {
		if out.History[i-1].At.Before(out.History[i].At) {
			t.Fatalf("history not sorted newest-first: %+v", out.History)
		}
	}
}

// TestListHandler_InvalidCursorRejected mirrors
// message.TestListHandler_InvalidCursorRejected: malformed base64 and
// valid base64 with an unparseable timestamp both 400.
func TestListHandler_InvalidCursorRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-clients-bad-cursor"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	for _, cursor := range []string{"not!valid!base64!", "YmFkdGltZXxzb21lLWlk"} {
		resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?cursor="+cursor)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("cursor %q: status = %d, want %d", cursor, resp.StatusCode, http.StatusBadRequest)
		}
	}
}

// TestListHandler_PaginatesNewestFirst seeds more than one page of
// Clients (?all=true, so no Engagement is needed) and walks the cursor,
// mirroring message.TestListHandler_PaginatesNewestFirst.
func TestListHandler_PaginatesNewestFirst(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-clients-paging"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")

	const total = 31 // pageSize (30) + 1, to force a second page
	for i := range total {
		testdb.SeedNamedClient(t, db, practiceID, "Client", fmt.Sprintf("client-%d@example.com", i))
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	firstResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
	defer firstResp.Body.Close()
	var first client.ListResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	secondResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true&cursor="+*first.NextCursor)
	defer secondResp.Body.Close()
	var second client.ListResponse
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
}

// TestListHandler_ContractorPaginatesNewestFirst hits
// listAttachedClients' cursor branch, the contractor counterpart to
// TestListHandler_PaginatesNewestFirst.
func TestListHandler_ContractorPaginatesNewestFirst(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-for-contractor-paging", []string{doulaRole}, "employee")
	const contractorUID = "contractor-paging"
	staffID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)

	const total = 31
	for i := range total {
		_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", fmt.Sprintf("attached-%d@example.com", i), "active")
		testdb.SeedGrantedAttachment(t, db, engagementID, staffID)
	}

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	firstResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
	defer firstResp.Body.Close()
	var first client.ListResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	secondResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true&cursor="+*first.NextCursor)
	defer secondResp.Body.Close()
	var second client.ListResponse
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
}

// TestListHandler_OpenEngagementsRollup_MultipleOpenNoneDroppedCompletedExcluded
// proves #264's headline AC directly: a Client with two concurrent open
// Engagements shows both (ADR-0017), a completed one is excluded, an
// Engagement with no Contract or Doula yet renders those fields nil
// rather than erroring, and Owner/Admin see Invoice status/money on a
// line that has one.
func TestListHandler_OpenEngagementsRollup_MultipleOpenNoneDroppedCompletedExcluded(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	const ownerUID = "staff-rollup-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-for-rollup", []string{doulaRole}, "employee")

	clientID, birthEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Rollup Client", "rollup@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, birthEngagement, doulaID)
	contractID := seedClientContract(t, db, birthEngagement, "sent")
	seedClientInvoice(t, db, practiceID, contractID, openInvoiceStatus, 50000)

	postpartumEngagement := seedEngagementForClient(t, db, clientID, practiceID, "intake", "postpartum")
	// No Contract, no Doula on this one -- proves the nil/absent case.

	seedEngagementForClient(t, db, clientID, practiceID, "completed", "birth")

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].ClientID != clientID {
		t.Fatalf("list = %+v, want exactly the rollup Client", listResp.Items)
	}
	rollup := listResp.Items[0].OpenEngagements
	if len(rollup) != 2 {
		t.Fatalf("OpenEngagements = %+v, want 2 lines (completed Engagement excluded)", rollup)
	}

	byID := map[string]client.OpenEngagement{}
	for _, line := range rollup {
		byID[line.EngagementID] = line
	}

	birth := byID[birthEngagement]
	if birth.EngagementStatus != "active" {
		t.Fatalf("birth line status = %q, want active", birth.EngagementStatus)
	}
	if birth.ContractStatus == nil || *birth.ContractStatus != "sent" {
		t.Fatalf("birth line contract status = %v, want \"sent\"", birth.ContractStatus)
	}
	if birth.DoulaName == nil || *birth.DoulaName != "Test Staff doula-for-rollup" {
		t.Fatalf("birth line doula name = %v, want the seeded Doula's name", birth.DoulaName)
	}
	if birth.InvoiceStatus == nil || *birth.InvoiceStatus != openInvoiceStatus {
		t.Fatalf("birth line invoice status = %v, want \"open\" (Owner reads it)", birth.InvoiceStatus)
	}
	if birth.InvoiceAmountCents == nil || *birth.InvoiceAmountCents != 50000 {
		t.Fatalf("birth line invoice amount = %v, want 50000", birth.InvoiceAmountCents)
	}
	if birth.FeeCents != nil {
		t.Fatalf("birth line fee = %v, want nil -- Owner is not a contractor", birth.FeeCents)
	}

	postpartum, ok := byID[postpartumEngagement]
	if !ok {
		t.Fatalf("postpartum Engagement missing from rollup: %+v", rollup)
	}
	if postpartum.EngagementStatus != "intake" {
		t.Fatalf("postpartum line status = %q, want intake", postpartum.EngagementStatus)
	}
	if postpartum.ContractStatus != nil {
		t.Fatalf("postpartum line contract status = %v, want nil (no Contract created)", postpartum.ContractStatus)
	}
	if postpartum.DoulaName != nil {
		t.Fatalf("postpartum line doula name = %v, want nil (no attachment)", postpartum.DoulaName)
	}
	if postpartum.InvoiceStatus != nil || postpartum.InvoiceAmountCents != nil {
		t.Fatalf("postpartum line invoice = %v/%v, want nil/nil (no Invoice)", postpartum.InvoiceStatus, postpartum.InvoiceAmountCents)
	}
}

// TestListHandler_OpenEngagementsRollup_AdminSeesInvoiceAndMoney proves the
// other half of "Owner and Admin views include Invoice status and money"
// (#264's AC): `reader.Has("owner") || reader.Has("admin")` short-circuits
// on Owner alone in every other test here, so this is the one place the
// Admin branch is actually exercised.
func TestListHandler_OpenEngagementsRollup_AdminSeesInvoiceAndMoney(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	const adminUID = "staff-rollup-admin"
	testdb.SeedStaffAtPractice(t, db, practiceID, adminUID, []string{adminRole}, "employee")

	clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Admin View Client", "admin-view@example.com", "active")
	contractID := seedClientContract(t, db, engagementID, "sent")
	seedClientInvoice(t, db, practiceID, contractID, openInvoiceStatus, 30000)

	srv, session := newServer(t, db, adminUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	byID := map[string]client.ListItem{}
	for _, item := range listResp.Items {
		byID[item.ClientID] = item
	}
	rollup := byID[clientID].OpenEngagements
	if len(rollup) != 1 {
		t.Fatalf("OpenEngagements = %+v, want exactly one line", rollup)
	}
	line := rollup[0]
	if line.InvoiceStatus == nil || *line.InvoiceStatus != openInvoiceStatus {
		t.Fatalf("invoice status = %v, want \"open\" -- an Admin reads it", line.InvoiceStatus)
	}
	if line.InvoiceAmountCents == nil || *line.InvoiceAmountCents != 30000 {
		t.Fatalf("invoice amount = %v, want 30000", line.InvoiceAmountCents)
	}
}

// TestListHandler_OpenEngagementsRollup_UnpaidDepositOutranksLaterPaidBalance
// is #741's regression case: a Contract can hold more than one Invoice,
// so a deposit Invoice that is still open (billed, unpaid) must not be
// hidden by a balance Invoice raised later and already paid. Decided on
// #741's own issue comment -- the rollup reports whether anything is
// outstanding under the Contract, never merely the newest Invoice's
// status.
func TestListHandler_OpenEngagementsRollup_UnpaidDepositOutranksLaterPaidBalance(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	const ownerUID = "staff-rollup-deposit-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")

	clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Deposit Client", "deposit@example.com", "active")
	contractID := seedClientContract(t, db, engagementID, "sent")
	seedClientInvoice(t, db, practiceID, contractID, openInvoiceStatus, 20000)
	seedClientInvoice(t, db, practiceID, contractID, "paid", 80000)

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	byID := map[string]client.ListItem{}
	for _, item := range listResp.Items {
		byID[item.ClientID] = item
	}
	rollup := byID[clientID].OpenEngagements
	if len(rollup) != 1 {
		t.Fatalf("OpenEngagements = %+v, want exactly one line", rollup)
	}
	line := rollup[0]
	if line.InvoiceStatus == nil || *line.InvoiceStatus != openInvoiceStatus {
		t.Fatalf("invoice status = %v, want %q -- the unpaid deposit must outrank the later paid balance, never read \"paid\"", line.InvoiceStatus, openInvoiceStatus)
	}
	if line.InvoiceAmountCents == nil || *line.InvoiceAmountCents != 20000 {
		t.Fatalf("invoice amount = %v, want 20000 (the deposit's amount, not the 80000 balance)", line.InvoiceAmountCents)
	}
}

// TestListHandler_OpenEngagementsRollup_NoClientsSkipsTheRollupQuery proves
// attachOpenEngagements' empty-list short circuit: a Practice with no
// Clients at all returns an empty page rather than erroring.
func TestListHandler_OpenEngagementsRollup_NoClientsSkipsTheRollupQuery(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "staff-rollup-no-clients"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, ownerUID, []string{doulaRole}, "employee")

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(listResp.Items) != 0 {
		t.Fatalf("items = %+v, want none", listResp.Items)
	}
}

// TestListHandler_OpenEngagementsRollup_ZeroOpenEngagementsShowsNoLines
// proves the third headline AC: a Client whose only Engagement is
// completed shows an empty rollup and the screen does not error.
func TestListHandler_OpenEngagementsRollup_ZeroOpenEngagementsShowsNoLines(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "staff-rollup-zero-open"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, ownerUID, []string{doulaRole}, "employee")
	clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Done Client", "done@example.com", "active")
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET status = 'completed', ending_reason = 'care_complete', birth_outcome = 'unknown'
		  WHERE id = $1`, engagementID,
	); err != nil {
		t.Fatalf("complete engagement: %v", err)
	}

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	byID := map[string]client.ListItem{}
	for _, item := range listResp.Items {
		byID[item.ClientID] = item
	}
	if got := byID[clientID].OpenEngagements; len(got) != 0 {
		t.Fatalf("OpenEngagements = %+v, want none", got)
	}
}

// TestListHandler_OpenEngagementsRollup_EmployeeDoulaSeesInvoiceAndMoney
// proves ADR-0008 as amended by #282: an employee Doula reads Contract
// status, Doula name, Engagement status, and now Invoice status/money
// too -- the same as an Owner or Admin -- since employment type, not
// role, is the boundary. #282's own mapping found this exact rollup
// silently giving an employee neither an Invoice status nor a fee; this
// proves that gap is closed.
func TestListHandler_OpenEngagementsRollup_EmployeeDoulaSeesInvoiceAndMoney(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	const employeeUID = "employee-doula-rollup"
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, employeeUID, []string{doulaRole}, "employee")

	clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Employee View Client", "employee-view@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)
	contractID := seedClientContract(t, db, engagementID, "signed")
	seedClientInvoice(t, db, practiceID, contractID, "paid", 75000)

	srv, session := newServer(t, db, employeeUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	byID := map[string]client.ListItem{}
	for _, item := range listResp.Items {
		byID[item.ClientID] = item
	}
	rollup := byID[clientID].OpenEngagements
	if len(rollup) != 1 {
		t.Fatalf("OpenEngagements = %+v, want exactly one line", rollup)
	}
	line := rollup[0]
	if line.ContractStatus == nil || *line.ContractStatus != "signed" {
		t.Fatalf("contract status = %v, want \"signed\"", line.ContractStatus)
	}
	if line.DoulaName == nil {
		t.Fatalf("doula name = nil, want the attached Doula's name")
	}
	if line.InvoiceStatus == nil || *line.InvoiceStatus != "paid" {
		t.Fatalf("invoice status = %v, want \"paid\" -- an employee Doula reads it now", line.InvoiceStatus)
	}
	if line.InvoiceAmountCents == nil || *line.InvoiceAmountCents != 75000 {
		t.Fatalf("invoice amount = %v, want 75000", line.InvoiceAmountCents)
	}
	if line.FeeCents != nil {
		t.Fatalf("fee = %v, want nil -- an employee Doula has no fee", line.FeeCents)
	}
}

// TestListHandler_OpenEngagementsRollup_ContractorSeesOwnFeeOnlyOnAttachedEngagement
// proves ADR-0008's sharpest edge: a contractor Doula's rollup shows her
// own fee, only on the Engagement she actually holds an open, granted
// attachment on -- a second open Engagement under the same Client that
// she is not attached to never appears in her rollup at all, matching
// staffauth.Reader.CanAccessEngagement's per-Engagement (not per-Client)
// gate.
func TestListHandler_OpenEngagementsRollup_ContractorSeesOwnFeeOnlyOnAttachedEngagement(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-for-contractor-rollup", []string{doulaRole}, "employee")
	const contractorUID = "contractor-rollup"
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)

	clientID, attachedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Contractor Rollup Client", "contractor-rollup@example.com", "active")
	seedGrantedAttachmentWithFee(t, db, attachedEngagement, contractorID, 120000)
	contractID := seedClientContract(t, db, attachedEngagement, "signed")
	// An Invoice exists, but a contractor never reads Invoice money.
	seedClientInvoice(t, db, practiceID, contractID, openInvoiceStatus, 120000)

	// A second open Engagement on the same Client she holds no
	// attachment on at all.
	unattachedEngagement := seedEngagementForClient(t, db, clientID, practiceID, "active", "postpartum")

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].ClientID != clientID {
		t.Fatalf("list = %+v, want exactly the attached Client", listResp.Items)
	}
	rollup := listResp.Items[0].OpenEngagements
	if len(rollup) != 1 || rollup[0].EngagementID != attachedEngagement {
		t.Fatalf("rollup = %+v, want exactly the attached Engagement %q (unattached %q must not appear)",
			rollup, attachedEngagement, unattachedEngagement)
	}
	line := rollup[0]
	if line.FeeCents == nil || *line.FeeCents != 120000 {
		t.Fatalf("fee = %v, want 120000", line.FeeCents)
	}
	if line.InvoiceStatus != nil || line.InvoiceAmountCents != nil {
		t.Fatalf("invoice = %v/%v, want both nil -- a contractor never reads Invoice money", line.InvoiceStatus, line.InvoiceAmountCents)
	}
	if line.ContractStatus == nil || *line.ContractStatus != "signed" {
		t.Fatalf("contract status = %v, want \"signed\" -- ungated scope field", line.ContractStatus)
	}
}

// TestListHandler_OpenEngagementsRollup_EndedAttachmentShowsNoFee proves
// fetchOpenEngagements' own "att.ended_at IS NULL" condition (#742),
// distinct from the identical one staffauth.Reader.CanAccessEngagement
// enforces: a contractor's attachment that has since ended must not
// surface her old fee, or the Engagement line it sat on, even though the
// same Client still shows because of a separate, still-open attachment.
func TestListHandler_OpenEngagementsRollup_EndedAttachmentShowsNoFee(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-for-ended-rollup", []string{doulaRole}, "employee")
	const contractorUID = "contractor-ended-rollup"
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)

	clientID, openEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Ended Rollup Client", "ended-rollup@example.com", "active")
	seedGrantedAttachmentWithFee(t, db, openEngagement, contractorID, 90000)

	endedEngagement := seedEngagementForClient(t, db, clientID, practiceID, "active", "postpartum")
	seedAttachmentWithFee(t, db, endedEngagement, contractorID, "granted", true, 40000)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].ClientID != clientID {
		t.Fatalf("list = %+v, want exactly the still-attached Client", listResp.Items)
	}
	rollup := listResp.Items[0].OpenEngagements
	if len(rollup) != 1 || rollup[0].EngagementID != openEngagement {
		t.Fatalf("rollup = %+v, want exactly the open-attachment Engagement %q (ended attachment's %q must not appear)",
			rollup, openEngagement, endedEngagement)
	}
	if rollup[0].FeeCents == nil || *rollup[0].FeeCents != 90000 {
		t.Fatalf("fee = %v, want 90000", rollup[0].FeeCents)
	}
}

// TestListHandler_OpenEngagementsRollup_AccruedAttachmentShowsNoFee
// proves fetchOpenEngagements' own "att.origin = 'granted'" condition
// (#742): an accrued-only attachment (#228 -- a record of work, never a
// key) must not surface a fee or its Engagement line either, even though
// the same contractor holds a separate open, granted attachment
// elsewhere on the same Client.
func TestListHandler_OpenEngagementsRollup_AccruedAttachmentShowsNoFee(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-for-accrued-rollup", []string{doulaRole}, "employee")
	const contractorUID = "contractor-accrued-rollup"
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)

	clientID, openEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Accrued Rollup Client", "accrued-rollup@example.com", "active")
	seedGrantedAttachmentWithFee(t, db, openEngagement, contractorID, 85000)

	accruedEngagement := seedEngagementForClient(t, db, clientID, practiceID, "active", "postpartum")
	seedAttachmentWithFee(t, db, accruedEngagement, contractorID, "accrued", false, 5000)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].ClientID != clientID {
		t.Fatalf("list = %+v, want exactly the granted-attachment Client", listResp.Items)
	}
	rollup := listResp.Items[0].OpenEngagements
	if len(rollup) != 1 || rollup[0].EngagementID != openEngagement {
		t.Fatalf("rollup = %+v, want exactly the granted Engagement %q (accrued-only %q must not appear)",
			rollup, openEngagement, accruedEngagement)
	}
	if rollup[0].FeeCents == nil || *rollup[0].FeeCents != 85000 {
		t.Fatalf("fee = %v, want 85000", rollup[0].FeeCents)
	}
}

// TestListHandler_OpenEngagementsRollup_AnotherDoulasFeeNeverSurfaces
// proves fetchOpenEngagements' own "att.staff_id = $n" condition (#742):
// two contractors both hold an open, granted attachment on the same
// Engagement (a co-attended birth), each with her own agreed fee -- the
// reader must see only her own, never the other Doula's.
func TestListHandler_OpenEngagementsRollup_AnotherDoulasFeeNeverSurfaces(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-for-cofee-rollup", []string{doulaRole}, "employee")
	const readerUID = "contractor-cofee-reader"
	readerID := testdb.SeedContractorAtPractice(t, db, practiceID, readerUID)
	const otherUID = "contractor-cofee-other"
	otherID := testdb.SeedContractorAtPractice(t, db, practiceID, otherUID)

	clientID, sharedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Co-Attended Client", "cofee-rollup@example.com", "active")
	seedGrantedAttachmentWithFee(t, db, sharedEngagement, readerID, 70000)
	seedGrantedAttachmentWithFee(t, db, sharedEngagement, otherID, 130000)

	srv, session := newServer(t, db, readerUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].ClientID != clientID {
		t.Fatalf("list = %+v, want exactly the shared Client", listResp.Items)
	}
	rollup := listResp.Items[0].OpenEngagements
	if len(rollup) != 1 || rollup[0].EngagementID != sharedEngagement {
		t.Fatalf("rollup = %+v, want exactly one line for the shared Engagement", rollup)
	}
	if rollup[0].FeeCents == nil || *rollup[0].FeeCents != 70000 {
		t.Fatalf("fee = %v, want the reader's own 70000, never the other Doula's 130000", rollup[0].FeeCents)
	}
}

// TestListHandler_OwnerContractorSeesWholePracticeNotOnlyHerAttachments
// resolves fetchOpenEngagements' own doc comment (#742): an
// owner-contractor (ADR-0017's "solo Practice") is real and reachable --
// nothing in staffauth ties employment_type to role -- so both
// ListHandler's own query choice and attachOpenEngagements' attachedOnly
// must key off staffauth.Reader.IsAmbientContractor, never bare
// IsContractor, or an Owner whose own membership happens to read
// "contractor" would have her Clients list silently narrowed to just her
// own attachments.
func TestListHandler_OwnerContractorSeesWholePracticeNotOnlyHerAttachments(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Solo Practice Rollup")
	const uid = "owner-contractor-rollup"
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, uid, []string{ownerRole, doulaRole}, "contractor")

	// An Engagement the Owner holds no attachment on at all -- still
	// hers to see, because the owner role's ambient reach does not
	// depend on her own employment_type.
	unattachedClientID, unattachedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Unattached Owner Client", "unattached-owner-rollup@example.com", "active")

	// An Engagement she does hold a fee on, to prove that still joins
	// through unconditionally alongside the ambient reach above.
	attachedClientID, attachedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Attached Owner Client", "attached-owner-rollup@example.com", "active")
	seedGrantedAttachmentWithFee(t, db, attachedEngagement, ownerID, 60000)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	byID := map[string]client.ListItem{}
	for _, item := range listResp.Items {
		byID[item.ClientID] = item
	}
	unattached, ok := byID[unattachedClientID]
	if !ok {
		t.Fatalf("list = %+v, want the unattached Client too -- an owner-contractor is not narrowed", listResp.Items)
	}
	if rollup := unattached.OpenEngagements; len(rollup) != 1 || rollup[0].EngagementID != unattachedEngagement {
		t.Fatalf("unattached rollup = %+v, want the one Engagement she holds no attachment on", rollup)
	}

	attached, ok := byID[attachedClientID]
	if !ok {
		t.Fatalf("list = %+v, want the attached Client", listResp.Items)
	}
	rollup := attached.OpenEngagements
	if len(rollup) != 1 || rollup[0].FeeCents == nil || *rollup[0].FeeCents != 60000 {
		t.Fatalf("attached rollup = %+v, want her own 60000 fee to join through", rollup)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return buf.String()
}

// TestListHandler_OpenEngagementsRollup_VoidedContractDoesNotDuplicateLine
// pins the regression that 00020_contracts_recreate_after_void.sql makes
// possible. That migration dropped contracts' table-wide UNIQUE
// (engagement_id) and replaced it with a partial unique index over
// non-voided rows, so #72's void-then-recreate flow leaves an Engagement
// holding a voided Contract AND a fresh Draft. The rollup's first
// implementation joined contracts plainly, returned both rows, and emitted
// two OpenEngagements sharing one EngagementID -- which the route renders
// with {#each ... (line.engagementId)}, where a duplicate key is a hard
// error in production rather than a degraded row.
//
// The second Client here is the other half of the rule: an Engagement
// whose ONLY Contract is voided must still report `voided`, because
// "no Contract yet" is a different answer for the reader, so the fix
// cannot simply filter voided rows out.
func TestListHandler_OpenEngagementsRollup_VoidedContractDoesNotDuplicateLine(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	const ownerUID = "staff-voided-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")

	recreatedClient, recreatedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Recreated Contract", "recreated@example.com", "active")
	seedClientContract(t, db, recreatedEngagement, "voided")
	seedClientContract(t, db, recreatedEngagement, "draft")

	voidedOnlyClient, voidedOnlyEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Voided Only", "voided@example.com", "active")
	seedClientContract(t, db, voidedOnlyEngagement, "voided")

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients")
	defer resp.Body.Close()
	var listResp client.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	byClient := map[string][]client.OpenEngagement{}
	for _, item := range listResp.Items {
		byClient[item.ClientID] = item.OpenEngagements
	}

	recreated := byClient[recreatedClient]
	if len(recreated) != 1 {
		t.Fatalf("OpenEngagements for the recreated-Contract Client = %+v, want exactly 1 line; a voided Contract must not duplicate its Engagement", recreated)
	}
	if recreated[0].EngagementID != recreatedEngagement {
		t.Fatalf("recreated line engagement = %q, want %q", recreated[0].EngagementID, recreatedEngagement)
	}
	if recreated[0].ContractStatus == nil || *recreated[0].ContractStatus != "draft" {
		t.Fatalf("recreated line contract status = %v, want \"draft\" (the live Contract wins over the voided one)", recreated[0].ContractStatus)
	}

	voidedOnly := byClient[voidedOnlyClient]
	if len(voidedOnly) != 1 {
		t.Fatalf("OpenEngagements for the voided-only Client = %+v, want exactly 1 line", voidedOnly)
	}
	if voidedOnly[0].ContractStatus == nil || *voidedOnly[0].ContractStatus != "voided" {
		t.Fatalf("voided-only line contract status = %v, want \"voided\" rather than nil: a voided Contract is not the absence of one", voidedOnly[0].ContractStatus)
	}
}
