package client_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

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
func (e *eraseEnv) editOnce() {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, http.MethodPut, e.clientURL(e.ClientID, ""), http.StatusOK, e.SignedIn, reqtest.JSON(
		client.EditRequest{
			GivenName: "Ada", FamilyName: "Lovelace", Email: "ada@example.com", Phone: "585-555-0199", Override: true}))
}

func postErasure(t *testing.T, session string, srv *httptest.Server, practiceID, clientID string) *http.Response {
	t.Helper()
	return authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID+"/erasure", nil)
}

// manualCheckPayment is a manually recorded check Payment on the invoice.
func (e *eraseEnv) manualCheckPayment(invoiceID string) (paymentID string) {
	e.T.Helper()
	return testdb.Scalar[string](e.T, e.DB.Admin,
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, method) VALUES ($1, 15000, now(), 'manual', 'check') RETURNING id`,
		invoiceID)
}

// TestEraseHandler_RedactsTheRecordInPlace is the central acceptance
// criterion: every identifying field is gone, erased_at is set, and the
// row's id -- and so every foreign key pointing at it -- is untouched.
func TestEraseHandler_RedactsTheRecordInPlace(t *testing.T) {
	e := newEraseEnv(t)

	e.erase()

	var row struct {
		id                                                    string
		givenName                                             string
		family, preferred, email, phone, line1, dob, locality *string
		fieldValues                                           string
		erasedAt                                              *time.Time
	}
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT id, given_name, family_name, preferred_name, email, phone, address_line1,
		        date_of_birth::text, address_locality, field_values::text, erased_at
		   FROM clients WHERE id = $1`, e.ClientID,
	).Scan(&row.id, &row.givenName, &row.family, &row.preferred, &row.email, &row.phone,
		&row.line1, &row.dob, &row.locality, &row.fieldValues, &row.erasedAt); err != nil {
		t.Fatalf("read client: %v", err)
	}

	if row.id != e.ClientID {
		t.Fatalf("id = %q, want the row to keep its id %q", row.id, e.ClientID)
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
	e := newEraseEnv(t)

	e.editOnce()
	before := readActivityRows(t, e.DB, e.ClientID)
	if len(before) != 1 {
		t.Fatalf("seeded activity rows = %d, want 1", len(before))
	}
	if !containsSealed(before[0]) {
		t.Fatalf("seeded activity row is not sealed: %q", before[0])
	}

	e.erase()

	after := readActivityRows(t, e.DB, e.ClientID)
	if len(after) != 2 {
		t.Fatalf("activity rows after erasure = %d, want 2 (the sealed one plus the plaintext 'erased' row)", len(after))
	}
	if after[0] != before[0] {
		t.Fatalf("the pre-erasure activity row changed:\n before %q\n after  %q", before[0], after[0])
	}

	// Her key is gone.
	if hasDataKey(t, e.DB, e.ClientID) {
		t.Fatal("her data key survived the erasure")
	}

	// And the read path says so rather than failing.
	detail := e.detail()
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

// TestEraseHandler_RedactsPaymentReversalReasons proves erasure empties
// the free-text reason on a reversal row (#945) against one of her
// Invoices -- the same personal-data sweep #271 already runs for a
// manually recorded Payment's own note, on the same table's other
// free-text column.
func TestEraseHandler_RedactsPaymentReversalReasons(t *testing.T) {
	e := newEraseEnv(t)
	invoiceID := e.paidInvoice()

	paymentID := e.manualCheckPayment(invoiceID)
	e.Exec(`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id, reason)
	        VALUES ($1, -15000, now(), 'reversal', $2, 'logged against the wrong invoice')`,
		invoiceID, paymentID)

	e.erase()
	var scope struct {
		PaymentReversalReasons int `json:"paymentReversalReasons"`
	}
	e.readErasureScope(&scope)
	if scope.PaymentReversalReasons != 1 {
		t.Fatalf("erasureScope.paymentReversalReasons = %d, want 1", scope.PaymentReversalReasons)
	}

	reason := testdb.Scalar[*string](t, e.DB.Admin, `SELECT reason FROM payments WHERE invoice_id = $1 AND kind = 'reversal'`, invoiceID)
	if reason != nil {
		t.Fatalf("reversal reason = %v, want NULL after erasure", *reason)
	}
}

// TestEraseHandler_RedactsRefundNote proves erasure reaches the note on a
// Refund (#1009) -- money a Practice returned by hand, whose note is the
// same free text a recorded Payment's is ("check #2201 mailed to her home
// address"). The note must go while the Refund row itself stays: it is
// the record that money went back, and nothing about her.
func TestEraseHandler_RedactsRefundNote(t *testing.T) {
	e := newEraseEnv(t)
	invoiceID := e.paidInvoice()

	paymentID := e.manualCheckPayment(invoiceID)
	e.Exec(`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id, method, note)
	        VALUES ($1, -15000, now(), 'refund', $2, 'check', 'check #2201 mailed to 12 Elm St')`,
		invoiceID, paymentID)

	e.erase()

	if note := testdb.Scalar[*string](t, e.DB.Admin, `SELECT note FROM payments WHERE invoice_id = $1 AND kind = 'refund'`, invoiceID); note != nil {
		t.Fatalf("refund note = %q, want NULL after erasure", *note)
	}
	if amount := testdb.Scalar[int64](t, e.DB.Admin, `SELECT amount_cents FROM payments WHERE invoice_id = $1 AND kind = 'refund'`, invoiceID); amount != -15000 {
		t.Fatalf("refund amount = %d, want the row kept at -15000", amount)
	}
}

// TestEraseHandler_RedactsOtherMethodPaymentNote proves erasure succeeds,
// and still empties the note, on a manually recorded Payment (#271) whose
// method is 'other' -- the one shape 00095's now-dropped
// payments_other_method_requires_note CHECK used to refuse, since that
// CHECK required note to stay non-null forever once method = 'other'
// (#1034). Every other erasure test's payment fixture uses method =
// 'check', which never exercised this path.
func TestEraseHandler_RedactsOtherMethodPaymentNote(t *testing.T) {
	e := newEraseEnv(t)
	invoiceID := e.paidInvoice()

	e.Exec(`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, method, note)
	        VALUES ($1, 15000, now(), 'manual', 'other', 'wire transfer via her employer')`,
		invoiceID)

	e.erase()
	var scope struct {
		PaymentNotes int `json:"paymentNotes"`
	}
	e.readErasureScope(&scope)
	if scope.PaymentNotes != 1 {
		t.Fatalf("erasureScope.paymentNotes = %d, want 1", scope.PaymentNotes)
	}

	if note := testdb.Scalar[*string](t, e.DB.Admin, `SELECT note FROM payments WHERE invoice_id = $1 AND method = 'other'`, invoiceID); note != nil {
		t.Fatalf("other-method payment note = %v, want NULL after erasure", *note)
	}
}

// eraseRoutes are the two routes at /clients/{clientId}/erasure: the
// eligibility precheck (#691) and the act it precedes.
var eraseRoutes = []struct{ name, method string }{
	{"the eligibility precheck", http.MethodGet},
	{"the erasure", http.MethodPost},
}

// TestEraseRoutes_RefuseEveryRoleButOwner is the Owner-only criterion, for
// the act and for the precheck that answers the same question (ADR-0027 and
// #394).
func TestEraseRoutes_RefuseEveryRoleButOwner(t *testing.T) {
	for _, route := range eraseRoutes {
		for _, role := range []string{adminRole, doulaRole} {
			t.Run(route.name+"/"+role, func(t *testing.T) {
				e := newEraseEnv(t, signedInAs(fixture.Roles(role)))

				reqtest.ExpectStatus(t, route.method, e.erasureURL(), http.StatusForbidden, e.SignedIn)

				if erasedAt := testdb.Scalar[*time.Time](t, e.DB.Admin, `SELECT erased_at FROM clients WHERE id = $1`, e.ClientID); erasedAt != nil {
					t.Fatal("erased_at is set after a refused erasure")
				}
			})
		}
	}
}

// TestEraseRoutes_RefuseWhatTheyCannotReach keeps a Client at another
// Practice unerasable, and says "not found" rather than "forbidden" -- the
// caller learns nothing about whether she exists -- and refuses an id that
// is no id.
func TestEraseRoutes_RefuseWhatTheyCannotReach(t *testing.T) {
	for _, route := range eraseRoutes {
		t.Run(route.name, func(t *testing.T) {
			e := newEraseEnv(t)
			otherPractice := fixture.Practice(t, e.DB, fixture.PracticeName("Other Practice"))
			otherClientID := seedFullClient(t, e.DB, otherPractice.ID, "")

			reqtest.ExpectStatus(t, route.method, e.clientURL(otherClientID, "/erasure"), http.StatusNotFound, e.SignedIn)
			reqtest.ExpectStatus(t, route.method, e.clientURL("not-a-uuid", "/erasure"), http.StatusBadRequest, e.SignedIn)
		})
	}
}

// TestEraseHandler_RefusesASecondErasure -- the act is irreversible, so a
// repeat is a mistake worth naming rather than absorbing.
func TestEraseHandler_RefusesASecondErasure(t *testing.T) {
	e := newEraseEnv(t)

	e.erase()
	second := reqtest.Post(t, e.erasureURL(), e.SignedIn)
	defer second.Body.Close()
	reqtest.RequireStatus(t, second, http.StatusConflict)
	// The code, not the prose, is what tells this refusal from the
	// unsettled-invoice one -- see EraseHandler's own doc comment.
	if got := apierrtest.Decode(t, second); got.Code != apierr.CodeConflict {
		t.Fatalf("second code = %q, want %q", got.Code, apierr.CodeConflict)
	}
}

// TestEditHandler_RefusesAnErasedClient -- her record is not editable
// afterwards, and the refusal says why.
func TestEditHandler_RefusesAnErasedClient(t *testing.T) {
	e := newEraseEnv(t)

	e.erase()

	reqtest.ExpectStatus(t, http.MethodPut, e.clientURL(e.ClientID, ""), http.StatusConflict, e.SignedIn,
		reqtest.JSON(client.EditRequest{GivenName: "Ada", Override: true}))
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
