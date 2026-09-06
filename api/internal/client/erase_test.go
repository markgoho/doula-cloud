package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// newErasureServer mounts the erasure endpoint alongside the reads an
// erasure test needs to prove its effects -- the detail read (for the
// redacted record and the unreadable history), the edit (for the
// post-erasure refusal), and #691's eligibility precheck.
func newErasureServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/clients/{clientId}/erasure",
		staffauth.Middleware(db.App)(client.EraseHandler(tasknudge.NoOpEnqueuer{})))
	mux.Handle("GET /practices/{practiceId}/clients/{clientId}/erasure",
		staffauth.Middleware(db.App)(client.EraseEligibilityHandler()))
	mux.Handle("GET /practices/{practiceId}/clients/{clientId}",
		staffauth.Middleware(db.App)(client.DetailHandler()))
	mux.Handle("PUT /practices/{practiceId}/clients/{clientId}",
		staffauth.Middleware(db.App)(client.EditHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// seedOwner seeds a Practice with an Owner at it, the only seat erasure
// admits.
func seedOwner(t *testing.T, db *testdb.DB, identityUID string) (practiceID, staffID string) {
	t.Helper()
	practiceID = testdb.SeedPractice(t, db, "Erasure Practice")
	staffID = testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, "employee")
	return practiceID, staffID
}

// seedFullClient seeds a Client with every identifying column filled in
// -- the state an erasure has something to do to. staffID is unused
// beyond making each caller state whose Practice she belongs to.
func seedFullClient(t *testing.T, db *testdb.DB, practiceID, staffID string) (clientID string) {
	t.Helper()
	_ = staffID
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, family_name, preferred_name, email, phone,
		     address_line1, address_locality, address_region, address_postal_code, date_of_birth, field_values)
		 VALUES ($1, 'Ada', 'Lovelace', 'Addy', 'ada@example.com', '585-555-0100',
		     '1 Analytical Way', 'Rochester', 'NY', '14607', '1990-12-10', '{"doulaNotes":"first baby"}'::jsonb)
		 RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	return clientID
}

// editOnce puts one edit through the real endpoint, so the activity row
// it writes is genuinely sealed under her key rather than a hand-built
// envelope. The changed field is her phone -- something erasure will
// null out, so the sealed diff really does hold personal data.
func editOnce(t *testing.T, session string, srv *httptest.Server, practiceID, clientID string) {
	t.Helper()
	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{
			GivenName: "Ada", FamilyName: "Lovelace", Email: "ada@example.com", Phone: "585-555-0199",
		}, Override: true})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("seeding edit: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func postErasure(t *testing.T, session string, srv *httptest.Server, practiceID, clientID string) *http.Response {
	t.Helper()
	return authedJSON(t, session, http.MethodPost, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/erasure", nil)
}

// TestEraseHandler_RedactsTheRecordInPlace is the central acceptance
// criterion: every identifying field is gone, erased_at is set, and the
// row's id -- and so every foreign key pointing at it -- is untouched.
func TestEraseHandler_RedactsTheRecordInPlace(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-redacts"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	resp := postErasure(t, session, srv, practiceID, clientID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var row struct {
		id                                                    string
		givenName                                             string
		family, preferred, email, phone, line1, dob, locality *string
		fieldValues                                           string
		erasedAt                                              *time.Time
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT id, given_name, family_name, preferred_name, email, phone, address_line1,
		        date_of_birth::text, address_locality, field_values::text, erased_at
		   FROM clients WHERE id = $1`, clientID,
	).Scan(&row.id, &row.givenName, &row.family, &row.preferred, &row.email, &row.phone,
		&row.line1, &row.dob, &row.locality, &row.fieldValues, &row.erasedAt); err != nil {
		t.Fatalf("read client: %v", err)
	}

	if row.id != clientID {
		t.Fatalf("id = %q, want the row to keep its id %q", row.id, clientID)
	}
	if row.givenName != client.ErasedGivenName {
		t.Fatalf("given_name = %q, want %q", row.givenName, client.ErasedGivenName)
	}
	for name, got := range map[string]*string{
		"family_name": row.family, "preferred_name": row.preferred, "email": row.email,
		"phone": row.phone, "address_line1": row.line1, "date_of_birth": row.dob,
		"address_locality": row.locality,
	} {
		if got != nil {
			t.Errorf("%s = %q, want NULL", name, *got)
		}
	}
	if row.fieldValues != "{}" {
		t.Errorf("field_values = %s, want {}", row.fieldValues)
	}
	if row.erasedAt == nil {
		t.Error("erased_at is NULL, want the timestamp proving the act ran")
	}
}

// TestEraseHandler_ShredsHerHistoryWithoutTouchingIt is the
// crypto-shredding criterion, both halves at once: the diff becomes
// unreadable, and the activity row it lives in is byte-for-byte the row
// that was written -- no UPDATE, no DELETE.
func TestEraseHandler_ShredsHerHistoryWithoutTouchingIt(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-shreds"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	editOnce(t, session, srv, practiceID, clientID)
	before := readActivityRows(t, db, clientID)
	if len(before) != 1 {
		t.Fatalf("seeded activity rows = %d, want 1", len(before))
	}
	if !containsSealed(before[0]) {
		t.Fatalf("seeded activity row is not sealed: %q", before[0])
	}

	resp := postErasure(t, session, srv, practiceID, clientID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	after := readActivityRows(t, db, clientID)
	if len(after) != 2 {
		t.Fatalf("activity rows after erasure = %d, want 2 (the sealed one plus the plaintext 'erased' row)", len(after))
	}
	if after[0] != before[0] {
		t.Fatalf("the pre-erasure activity row changed:\n before %q\n after  %q", before[0], after[0])
	}

	// Her key is gone.
	var keys int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM client_data_keys WHERE client_id = $1`, clientID,
	).Scan(&keys); err != nil {
		t.Fatalf("count keys: %v", err)
	}
	if keys != 0 {
		t.Fatalf("client_data_keys rows = %d, want 0", keys)
	}

	// And the read path says so rather than failing.
	detail := readDetail(t, session, srv, practiceID, clientID)
	var sealed, erasedEntry *client.Event
	for i, entry := range detail.History {
		if entry.ClientEvent == nil {
			continue
		}
		if entry.ClientEvent.EventType == "erased" {
			erasedEntry = detail.History[i].ClientEvent
			continue
		}
		sealed = detail.History[i].ClientEvent
	}
	if sealed == nil || string(sealed.Diff) != `{"erased":true}` {
		t.Fatalf("shredded entry diff = %v, want the unreadable marker", sealed)
	}
	if erasedEntry == nil {
		t.Fatal("no 'erased' entry in her history, want the plaintext record of the act")
	}
	var scope map[string]any
	if err := json.Unmarshal(erasedEntry.Diff, &scope); err != nil {
		t.Fatalf("the 'erased' entry is not readable plaintext: %v", err)
	}
	if erasedEntry.ActorStaffID == nil || *erasedEntry.ActorStaffID == "" {
		t.Fatal("the 'erased' entry names no actor, want the Owner who ran it")
	}
}

// TestEraseHandler_RefusesEveryRoleButOwner is the Owner-only criterion.
func TestEraseHandler_RefusesEveryRoleButOwner(t *testing.T) {
	for name, roles := range map[string][]string{
		adminRole: {adminRole},
		doulaRole: {doulaRole},
	} {
		t.Run(name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "erase-role-" + name
			practiceID := testdb.SeedPractice(t, db, "Erasure Practice")
			staffID := testdb.SeedStaffAtPractice(t, db, practiceID, uid, roles, "employee")
			clientID := seedFullClient(t, db, practiceID, staffID)

			srv, session := newErasureServer(t, db, uid)
			defer srv.Close()

			resp := postErasure(t, session, srv, practiceID, clientID)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want %d -- only an Owner may erase", resp.StatusCode, http.StatusForbidden)
			}
			var erasedAt *time.Time
			if err := db.Admin.QueryRowContext(t.Context(),
				`SELECT erased_at FROM clients WHERE id = $1`, clientID,
			).Scan(&erasedAt); err != nil {
				t.Fatalf("read client: %v", err)
			}
			if erasedAt != nil {
				t.Fatal("erased_at is set after a refused erasure")
			}
		})
	}
}

// TestEraseHandler_RefusesASecondErasure -- the act is irreversible, so a
// repeat is a mistake worth naming rather than absorbing.
func TestEraseHandler_RefusesASecondErasure(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-twice"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	first := postErasure(t, session, srv, practiceID, clientID)
	defer first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d, want %d", first.StatusCode, http.StatusOK)
	}
	second := postErasure(t, session, srv, practiceID, clientID)
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second status = %d, want %d", second.StatusCode, http.StatusConflict)
	}
	// The code, not the prose, is what tells this refusal from the
	// unsettled-invoice one -- see EraseHandler's own doc comment.
	if got := readAPIError(t, second); got.Code != string(apierr.CodeConflict) {
		t.Fatalf("second code = %q, want %q", got.Code, apierr.CodeConflict)
	}
}

// TestEraseHandler_RefusesAnUnknownClient keeps a Client at another
// Practice unerasable, and says "not found" rather than "forbidden" --
// the caller learns nothing about whether she exists.
func TestEraseHandler_RefusesAnUnknownClient(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-other-practice"
	practiceID, _ := seedOwner(t, db, uid)
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	otherStaffID := testdb.SeedStaffAtPractice(t, db, otherPracticeID, "other-owner", []string{ownerRole}, "employee")
	otherClientID := seedFullClient(t, db, otherPracticeID, otherStaffID)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	resp := postErasure(t, session, srv, practiceID, otherClientID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestEraseHandler_RefusesAMalformedClientID(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-bad-id"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	resp := postErasure(t, session, srv, practiceID, "not-a-uuid")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestEditHandler_RefusesAnErasedClient -- her record is not editable
// afterwards, and the refusal says why.
func TestEditHandler_RefusesAnErasedClient(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-then-edit"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	erased := postErasure(t, session, srv, practiceID, clientID)
	defer erased.Body.Close()
	if erased.StatusCode != http.StatusOK {
		t.Fatalf("erase status = %d, want %d", erased.StatusCode, http.StatusOK)
	}

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{GivenName: "Ada"}, Override: true})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("edit status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

// containsSealed reports whether a row's text carries a sealed envelope
// rather than a plaintext diff -- and, since it looks for the ciphertext
// key, also proves the phone number that went in is not sitting there in
// the clear.
func containsSealed(row string) bool {
	return strings.Contains(row, `"enc"`) && !strings.Contains(row, "585-555-0199")
}

// readActivityRows reads every client-subject activity row as one opaque
// string, so a test can compare the whole row before and after an erasure
// and prove nothing in it moved.
func readActivityRows(t *testing.T, db *testdb.DB, clientID string) []string {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT id::text || '|' || action || '|' || diff::text || '|' || created_at::text
		   FROM activity WHERE subject_kind = 'client' AND subject_id = $1 ORDER BY created_at`,
		clientID)
	if err != nil {
		t.Fatalf("read activity: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan activity: %v", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate activity: %v", err)
	}
	return out
}

func readDetail(t *testing.T, session string, srv *httptest.Server, practiceID, clientID string) client.DetailResponse {
	t.Helper()
	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients/"+clientID)
	defer resp.Body.Close()
	var out client.DetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	return out
}
