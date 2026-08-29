package client_test

import (
	"bytes"
	"encoding/json"
	"net/http"
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
	practiceID := seedStaffWithMembership(t, db, identityUID)
	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/practices/"+practiceID+"/clients",
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
		`SELECT count(*), max(event_type::text) FROM client_events WHERE client_id = $1`, rec.ID,
	).Scan(&eventCount, &eventType); err != nil {
		t.Fatalf("count client_events: %v", err)
	}
	if eventCount != 1 || eventType != "created" {
		t.Fatalf("client_events count = %d type = %q, want 1 and \"created\"", eventCount, eventType)
	}
}

func TestCreateHandler_InvalidBodyAndInvalidDateOfBirth(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-create-validation"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	badBody, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/practices/"+practiceID+"/clients", bytes.NewReader([]byte(`{`)))
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

	badDate := authedJSON(t, session, http.MethodPost, srv.URL+"/practices/"+practiceID+"/clients",
		client.CreateRequest{Record: client.Record{GivenName: "Bad Date", DateOfBirth: "not-a-date"}})
	defer badDate.Body.Close()
	if badDate.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid dateOfBirth status = %d, want %d", badDate.StatusCode, http.StatusBadRequest)
	}
}

func TestCreateHandler_MissingGivenName(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-missing-name"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/practices/"+practiceID+"/clients", client.CreateRequest{})
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
	practiceID := seedStaffWithMembership(t, db, "staff-owner-for-contractor")
	const contractorUID = "contractor-creating"
	seedContractorAtPractice(t, db, practiceID, contractorUID)
	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/practices/"+practiceID+"/clients",
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
		`INSERT INTO practices (name) VALUES ('Solo Practice') RETURNING id`,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	const uid = "owner-contractor-creating"
	seedOwnerContractorAtPractice(t, db, practiceID, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/practices/"+practiceID+"/clients",
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
	practiceID := seedStaffWithMembership(t, db, identityUID)
	existingID := seedClient(t, db, practiceID, "Sarah Beck", "sarah@example.com")
	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/practices/"+practiceID+"/clients",
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

	overridden := authedJSON(t, session, http.MethodPost, srv.URL+"/practices/"+practiceID+"/clients",
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
	practiceID := seedStaffWithMembership(t, db, identityUID)
	otherPracticeID := seedStaffWithMembership(t, db, "staff-other-practice-search")
	inPractice := seedClient(t, db, practiceID, "Nadia Haddad", "nadia@example.com")
	seedClient(t, db, otherPracticeID, "Nadia Haddad", "nadia@example.com")
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
		resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients/search"+q)
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
	practiceID := seedStaffWithMembership(t, db, "staff-owner-for-edit")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Edit Client", "edit@example.com")

	const unattachedUID = "contractor-unattached-edit"
	seedContractorAtPractice(t, db, practiceID, unattachedUID)
	srvUnattached, sessionUnattached := newServer(t, db, unattachedUID)
	defer srvUnattached.Close()
	respUnattached := authedJSON(t, sessionUnattached, http.MethodPut, srvUnattached.URL+"/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{GivenName: "Edit Client"}})
	defer respUnattached.Body.Close()
	if respUnattached.StatusCode != http.StatusNotFound {
		t.Fatalf("unattached contractor status = %d, want %d", respUnattached.StatusCode, http.StatusNotFound)
	}

	const attachedUID = "contractor-attached-edit"
	attachedStaffID := seedContractorAtPractice(t, db, practiceID, attachedUID)
	seedGrantedAttachment(t, db, engagementID, attachedStaffID)
	srvAttached, sessionAttached := newServer(t, db, attachedUID)
	defer srvAttached.Close()
	respAttached := authedJSON(t, sessionAttached, http.MethodPut, srvAttached.URL+"/practices/"+practiceID+"/clients/"+clientID,
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
	practiceID := seedStaffWithMembership(t, db, identityUID)
	seedClient(t, db, practiceID, "Nadia Haddad", "nadia@example.com")
	editingID := seedClient(t, db, practiceID, "Sara Beck", "sara@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/practices/"+practiceID+"/clients/"+editingID,
		client.EditRequest{Record: client.Record{GivenName: "Nadia", FamilyName: "Haddad", Email: "nadia@example.com"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}

	overridden := authedJSON(t, session, http.MethodPut, srv.URL+"/practices/"+practiceID+"/clients/"+editingID,
		client.EditRequest{Record: client.Record{GivenName: "Nadia", FamilyName: "Haddad", Email: "nadia@example.com"}, Override: true})
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
	practiceID := seedStaffWithMembership(t, db, identityUID)
	clientID := seedClient(t, db, practiceID, "Revoke Client", "old@example.com")
	outboxID := seedPendingOutboxRow(t, db, clientID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{GivenName: "Revoke Client", Email: "new@example.com"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	status, lastErr := outboxStatus(t, db, outboxID)
	if status != "dead_lettered" || lastErr == "" {
		t.Fatalf("outbox row status = %q lastError = %q, want dead_lettered with a reason", status, lastErr)
	}
}

// TestEditHandler_EveryEditWritesOneClientEvent proves the client_events
// AC: an edit that actually changes a fact writes exactly one row, with
// a diff and a named actor.
func TestEditHandler_EveryEditWritesOneClientEvent(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-editing-events"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	clientID := seedClient(t, db, practiceID, "Event Client", "event@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{GivenName: "Event Client", Phone: "555-0199"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var eventCount int
	var eventType, actorKind string
	var actorStaffID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(event_type::text), max(actor_kind::text), max(actor_staff_id::text)
		 FROM client_events WHERE client_id = $1 AND event_type = 'updated'`, clientID,
	).Scan(&eventCount, &eventType, &actorKind, &actorStaffID); err != nil {
		t.Fatalf("query client_events: %v", err)
	}
	if eventCount != 1 || eventType != "updated" || actorKind != "staff" || actorStaffID == "" {
		t.Fatalf("eventCount=%d eventType=%q actorKind=%q actorStaffID=%q, want 1 updated staff <id>", eventCount, eventType, actorKind, actorStaffID)
	}
}

// TestEditHandler_NoChangeStillWritesOneEmptyDiffEvent proves ADR-0017's
// "one row per act": even a no-op edit (identical values resubmitted)
// writes exactly one client_events row, with an empty diff.
func TestEditHandler_NoChangeStillWritesOneEmptyDiffEvent(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-noop-edit"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	clientID := seedClient(t, db, practiceID, "Noop Client", "noop@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{GivenName: "Noop Client", Email: "noop@example.com"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var count int
	var diffJSON string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(diff::text) FROM client_events WHERE client_id = $1`, clientID,
	).Scan(&count, &diffJSON); err != nil {
		t.Fatalf("count client_events: %v", err)
	}
	if count != 1 || diffJSON != "{}" {
		t.Fatalf("client_events count = %d diff = %q, want 1 row with an empty diff", count, diffJSON)
	}
}

// TestEditHandler_FieldValuesChangeIsDiffedAsOneWholeBlob proves
// diffRecords' field_values branch: a changed Practice-defined values
// blob is recorded as one "fieldValues" diff entry.
func TestEditHandler_FieldValuesChangeIsDiffedAsOneWholeBlob(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-field-values"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	clientID := seedClient(t, db, practiceID, "Field Values Client", "fieldvalues@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{GivenName: "Field Values Client", FieldValues: json.RawMessage(`{"favoriteColor":"blue"}`)}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var diffJSON []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT diff FROM client_events WHERE client_id = $1 AND event_type = 'updated'`, clientID,
	).Scan(&diffJSON); err != nil {
		t.Fatalf("read diff: %v", err)
	}
	var diff map[string]json.RawMessage
	if err := json.Unmarshal(diffJSON, &diff); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if _, ok := diff["fieldValues"]; !ok {
		t.Fatalf("diff = %s, want a fieldValues entry", diffJSON)
	}
}

// TestDetailHandler_ReturnsRecordEngagementsAndHistory proves the detail
// read: her record, her Engagements, and client_events merged with
// engagement_requests.
func TestDetailHandler_ReturnsRecordEngagementsAndHistory(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-detail"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Detail Client", "detail@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients/"+clientID)
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
	practiceID := seedStaffWithMembership(t, db, identityUID)
	clientID := seedClient(t, db, practiceID, "Resolved Fields Client", "resolved@example.com")

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
	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients/"+clientID)
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
	practiceID := seedStaffWithMembership(t, db, identityUID)
	clientID := seedClient(t, db, practiceID, "Live Template Client", "live@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	before := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients/"+clientID)
	var beforeOut client.DetailResponse
	if err := json.NewDecoder(before.Body).Decode(&beforeOut); err != nil {
		t.Fatalf("decode before response: %v", err)
	}
	_ = before.Body.Close()
	if len(beforeOut.ResolvedFields) != 0 {
		t.Fatalf("resolved fields before template exists = %+v, want none", beforeOut.ResolvedFields)
	}

	seedFieldTemplate(t, db, practiceID, `[{"id":"new_field","type":"short_text","label":"Added later","order":0,"archived":false}]`)

	after := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients/"+clientID)
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
	practiceID := seedStaffWithMembership(t, db, identityUID)
	withTwoEngagements, _ := seedClientEngagement(t, db, practiceID, "Two Engagements", "two@example.com")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status, kind) VALUES ($1, $2, 'completed', 'postpartum')`,
		withTwoEngagements, practiceID,
	); err != nil {
		t.Fatalf("seed second engagement: %v", err)
	}
	noWork := seedClient(t, db, practiceID, "No Work Yet", "nowork@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients")
	defer resp.Body.Close()
	var list []client.ListItem
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 1 || list[0].ClientID != withTwoEngagements {
		t.Fatalf("default list = %+v, want exactly one row for the two-Engagement Client", list)
	}

	all := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients?all=true")
	defer all.Body.Close()
	var allList []client.ListItem
	if err := json.NewDecoder(all.Body).Decode(&allList); err != nil {
		t.Fatalf("decode response: %v", err)
	}
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
	practiceID := seedStaffWithMembership(t, db, identityUID)
	neverInvited, _ := seedClientEngagement(t, db, practiceID, "Never Invited", "never@example.com")
	invited, _ := seedClientEngagement(t, db, practiceID, "Invited Client", "invited@example.com")
	seedPendingOutboxRow(t, db, invited)
	accepted, _ := seedClientEngagement(t, db, practiceID, "Accepted Client", "accepted@example.com")
	seedAcceptedPortalUser(t, db, accepted)
	portalUserNoOutbox, _ := seedClientEngagement(t, db, practiceID, "No Outbox Client", "no-outbox@example.com")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid())`, portalUserNoOutbox,
	); err != nil {
		t.Fatalf("seed portal user with no outbox row: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients")
	defer resp.Body.Close()
	var list []client.ListItem
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	byID := map[string]client.ListItem{}
	for _, item := range list {
		byID[item.ClientID] = item
	}
	if byID[neverInvited].PortalInviteStatus != nil {
		t.Fatalf("never-invited status = %v, want nil", byID[neverInvited].PortalInviteStatus)
	}
	if got := byID[invited].PortalInviteStatus; got == nil || *got != "pending" {
		t.Fatalf("invited status = %v, want \"pending\"", got)
	}
	if got := byID[accepted].PortalInviteStatus; got == nil || *got != "accepted" {
		t.Fatalf("accepted status = %v, want \"accepted\"", got)
	}
	if byID[portalUserNoOutbox].PortalInviteStatus != nil {
		t.Fatalf("no-outbox status = %v, want nil", byID[portalUserNoOutbox].PortalInviteStatus)
	}
}

// TestListHandler_ContractorSeesOnlyAttachedClients proves
// listAttachedClients: a contractor's list narrows to Clients she holds
// a granted attachment to, and an unattached Client never appears.
func TestListHandler_ContractorSeesOnlyAttachedClients(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-owner-for-contractor-list")
	attachedClient, engagementID := seedClientEngagement(t, db, practiceID, "Attached Client", "attached@example.com")
	seedClientEngagement(t, db, practiceID, "Unattached Client", "unattached@example.com")

	const contractorUID = "contractor-listing"
	staffID := seedContractorAtPractice(t, db, practiceID, contractorUID)
	seedGrantedAttachment(t, db, engagementID, staffID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients?all=true")
	defer resp.Body.Close()
	var list []client.ListItem
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 1 || list[0].ClientID != attachedClient {
		t.Fatalf("contractor list = %+v, want exactly the attached client", list)
	}
}

// TestSearchHandler_RefusesContractorAndEmptyQueryReturnsNoMatches proves
// the contractor refusal and the empty-query short circuit both return
// cleanly rather than erroring.
func TestSearchHandler_RefusesContractorAndEmptyQueryReturnsNoMatches(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-owner-for-search-refusal")
	const contractorUID = "contractor-searching"
	seedContractorAtPractice(t, db, practiceID, contractorUID)

	srvContractor, sessionContractor := newServer(t, db, contractorUID)
	defer srvContractor.Close()
	respContractor := authedGet(t, sessionContractor, srvContractor.URL+"/practices/"+practiceID+"/clients/search?name=Anyone")
	defer respContractor.Body.Close()
	if respContractor.StatusCode != http.StatusForbidden {
		t.Fatalf("contractor search status = %d, want %d", respContractor.StatusCode, http.StatusForbidden)
	}

	const ownerUID = "staff-empty-search"
	seedStaffAtPractice(t, db, practiceID, ownerUID)
	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()
	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients/search")
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
	practiceID := seedStaffWithMembership(t, db, identityUID)
	clientID := seedClient(t, db, practiceID, "Validation Client", "validation@example.com")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	badID := authedJSON(t, session, http.MethodPut, srv.URL+"/practices/"+practiceID+"/clients/not-a-uuid",
		client.EditRequest{Record: client.Record{GivenName: "Whoever"}})
	defer badID.Body.Close()
	if badID.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid clientId status = %d, want %d", badID.StatusCode, http.StatusBadRequest)
	}

	missingRequest, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		srv.URL+"/practices/"+practiceID+"/clients/00000000-0000-0000-0000-000000000000", bytes.NewReader([]byte(`{"givenName":"Ghost"}`)))
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

	badBody, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/practices/"+practiceID+"/clients/"+clientID, bytes.NewReader([]byte(`{`)))
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

	missingGivenName := authedJSON(t, session, http.MethodPut, srv.URL+"/practices/"+practiceID+"/clients/"+clientID,
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
	practiceID := seedStaffWithMembership(t, db, "staff-detail-validation")
	srv, session := newServer(t, db, "staff-detail-validation")
	defer srv.Close()

	badID := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients/not-a-uuid")
	defer badID.Body.Close()
	if badID.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid clientId status = %d, want %d", badID.StatusCode, http.StatusBadRequest)
	}

	missing := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients/00000000-0000-0000-0000-000000000000")
	defer missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("nonexistent clientId status = %d, want %d", missing.StatusCode, http.StatusNotFound)
	}

	const contractorUID = "contractor-unattached-client-detail"
	seedContractorAtPractice(t, db, practiceID, contractorUID)
	unattachedID := seedClient(t, db, practiceID, "Unattached Detail Client", "unattached-cd@example.com")
	srvContractor, sessionContractor := newServer(t, db, contractorUID)
	defer srvContractor.Close()
	forbidden := authedGet(t, sessionContractor, srvContractor.URL+"/practices/"+practiceID+"/clients/"+unattachedID)
	defer forbidden.Body.Close()
	if forbidden.StatusCode != http.StatusNotFound {
		t.Fatalf("unattached contractor detail status = %d, want %d", forbidden.StatusCode, http.StatusNotFound)
	}
}

// TestDetailHandler_MergesEventsAndRequestsIntoHistory proves the merged
// history: client_events rows (written by an actual create then edit)
// interleaved with engagement_requests rows, newest first.
func TestDetailHandler_MergesEventsAndRequestsIntoHistory(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-history"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	created := authedJSON(t, session, http.MethodPost, srv.URL+"/practices/"+practiceID+"/clients",
		client.CreateRequest{Record: client.Record{GivenName: "History Client"}})
	var rec client.Record
	if err := json.NewDecoder(created.Body).Decode(&rec); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	_ = created.Body.Close()

	edited := authedJSON(t, session, http.MethodPut, srv.URL+"/practices/"+practiceID+"/clients/"+rec.ID,
		client.EditRequest{Record: client.Record{GivenName: "History Client", Phone: "555-0177"}})
	_ = edited.Body.Close()

	staffID := seedStaffAtPractice(t, db, practiceID, "requesting-staff-history")
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

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients/"+rec.ID)
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
		case "engagement_request":
			requests++
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

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return buf.String()
}
