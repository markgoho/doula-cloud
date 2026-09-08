package payments_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

const paymentTermsPath = "/payments/payment-terms"

func requestPaymentTerms(t *testing.T, srv *httptest.Server, session, practiceID, method, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method,
		srv.URL+"/api/practices/"+practiceID+paymentTermsPath, bytes.NewBufferString(body))
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

func readPaymentTerms(t *testing.T, srv *httptest.Server, session, practiceID, method, body string) payments.PaymentTermsResponse {
	t.Helper()
	resp := requestPaymentTerms(t, srv, session, practiceID, method, body)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out payments.PaymentTermsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

// TestGetPaymentTermsHandler_DefaultsWithoutARow proves #768's decision
// 2: a Practice that has never set terms behaves as 30 days, and says so
// -- the absent row is a documented default, not an unset value a caller
// has to interpret.
func TestGetPaymentTermsHandler_DefaultsWithoutARow(t *testing.T) {
	db := testdb.New(t)
	const uid = "payment-terms-default"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	out := readPaymentTerms(t, srv, session, practiceID, http.MethodGet, "")
	if out.NetDays != payments.DefaultPaymentTermsDays || !out.IsDefault {
		t.Fatalf("terms = (%d, isDefault %v), want (%d, true)", out.NetDays, out.IsDefault, payments.DefaultPaymentTermsDays)
	}
}

// TestPutPaymentTermsHandler_SetsAndRecordsWhoChangedIt covers CLAUDE.md's
// audit-trail expectation for this setting: the change is readable back,
// and the Activity log says who made it and when.
func TestPutPaymentTermsHandler_SetsAndRecordsWhoChangedIt(t *testing.T) {
	db := testdb.New(t)
	const uid = "payment-terms-set"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	out := readPaymentTerms(t, srv, session, practiceID, http.MethodPut, `{"netDays":45}`)
	if out.NetDays != 45 || out.IsDefault {
		t.Fatalf("terms = (%d, isDefault %v), want (45, false)", out.NetDays, out.IsDefault)
	}
	if got := readPaymentTerms(t, srv, session, practiceID, http.MethodGet, ""); got.NetDays != 45 || got.IsDefault {
		t.Fatalf("read back = (%d, isDefault %v), want (45, false)", got.NetDays, got.IsDefault)
	}

	var actor string
	var diff []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id, diff FROM activity
		 WHERE practice_id = $1 AND action = 'payment_terms_changed'`, practiceID,
	).Scan(&actor, &diff); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	if actor != staffID {
		t.Fatalf("actor = %q, want %q", actor, staffID)
	}
	var recorded struct {
		NetDaysBefore *int `json:"netDaysBefore"`
		NetDaysAfter  int  `json:"netDaysAfter"`
	}
	if err := json.Unmarshal(diff, &recorded); err != nil {
		t.Fatalf("decode diff: %v", err)
	}
	if recorded.NetDaysBefore != nil || recorded.NetDaysAfter != 45 {
		t.Fatalf("diff = %+v, want before nil (the Practice was on the default) and after 45", recorded)
	}
}

// TestPutPaymentTermsHandler_RepeatRecordsNothingNew pins the idempotency
// practicerate.PutRateHandler already has: a retry with the same body is
// not a second change, so it writes no second Activity row.
func TestPutPaymentTermsHandler_RepeatRecordsNothingNew(t *testing.T) {
	db := testdb.New(t)
	const uid = "payment-terms-repeat"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	readPaymentTerms(t, srv, session, practiceID, http.MethodPut, `{"netDays":15}`)
	readPaymentTerms(t, srv, session, practiceID, http.MethodPut, `{"netDays":15}`)

	var rows int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE practice_id = $1 AND action = 'payment_terms_changed'`, practiceID,
	).Scan(&rows); err != nil {
		t.Fatalf("count activity rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("activity rows = %d, want 1", rows)
	}
}

// TestPutPaymentTermsHandler_RefusesAnImpossibleTerm covers both ends of
// the bound the migration's CHECK also carries, so a typo cannot put an
// Invoice's due date past any horizon a Practice would notice.
func TestPutPaymentTermsHandler_RefusesAnImpossibleTerm(t *testing.T) {
	db := testdb.New(t)
	const uid = "payment-terms-invalid"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	for _, body := range []string{`{"netDays":0}`, `{"netDays":-5}`, `{"netDays":400}`} {
		func() {
			resp := requestPaymentTerms(t, srv, session, practiceID, http.MethodPut, body)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("%s status = %d, want %d", body, resp.StatusCode, http.StatusBadRequest)
			}
			// docs/api-design.md section 7 rule 4: a refusal a person
			// causes by filling in a form names the field at fault, keyed
			// by the request DTO's own JSON tag, so the app maps it onto a
			// control with no translation table.
			var out struct {
				Details map[string]string `json:"details"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatalf("%s decode response: %v", body, err)
			}
			if out.Details["netDays"] != payments.MsgNetDaysOutOfRange {
				t.Fatalf("%s details[netDays] = %q, want %q", body, out.Details["netDays"], payments.MsgNetDaysOutOfRange)
			}
		}()
	}
}

// TestPutPaymentTermsHandler_RefusesAMalformedBody covers the decode
// refusal ahead of the range check: a body that is not JSON at all is
// answered before anything reads a number out of it.
func TestPutPaymentTermsHandler_RefusesAMalformedBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "payment-terms-malformed"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := requestPaymentTerms(t, srv, session, practiceID, http.MethodPut, `{"netDays":`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutPaymentTermsHandler_RefusesADoula proves the write side is
// narrower than the read side: every Staff member reads the terms, only
// an Owner or Admin sets them (#282's write table). The refusal comes
// from the mount, not the handler (#990).
func TestPutPaymentTermsHandler_RefusesADoula(t *testing.T) {
	db := testdb.New(t)
	const uid = "payment-terms-doula"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{"doula"}, "employee")

	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := requestPaymentTerms(t, srv, session, practiceID, http.MethodPut, `{"netDays":45}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}

	// She still reads it: a Doula meets the due date on every Invoice she
	// looks at.
	if got := readPaymentTerms(t, srv, session, practiceID, http.MethodGet, ""); got.NetDays != payments.DefaultPaymentTermsDays {
		t.Fatalf("doula read = %d, want the default %d", got.NetDays, payments.DefaultPaymentTermsDays)
	}
}

// invoiceDueAtOf reads an Invoice's stored due date straight off the row,
// which is the only place the by-hand rail's due date exists at all.
func invoiceDueAtOf(t *testing.T, db *testdb.DB, invoiceID string) time.Time {
	t.Helper()
	var dueAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT due_at FROM invoices WHERE id = $1`, invoiceID,
	).Scan(&dueAt); err != nil {
		t.Fatalf("read due_at: %v", err)
	}
	return dueAt
}
