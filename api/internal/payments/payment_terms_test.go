package payments_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

const paymentTermsPath = "/payments/payment-terms"

// termsBody is a raw JSON body for the payment-terms route.
func termsBody(body string) reqtest.Option { return reqtest.RawJSON([]byte(body)) }

func (e *env) readTerms() payments.PaymentTermsResponse {
	e.T.Helper()
	return reqtest.Expect[payments.PaymentTermsResponse](e.T, http.MethodGet, e.PracticeURL(paymentTermsPath), http.StatusOK, e.SignedIn)
}

func (e *env) setTerms(body string) payments.PaymentTermsResponse {
	e.T.Helper()
	return reqtest.Expect[payments.PaymentTermsResponse](e.T, http.MethodPut, e.PracticeURL(paymentTermsPath), http.StatusOK, e.SignedIn, termsBody(body))
}

// TestGetPaymentTermsHandler_DefaultsWithoutARow proves #768's decision
// 2: a Practice that has never set terms behaves as 30 days, and says so
// -- the absent row is a documented default, not an unset value a caller
// has to interpret.
func TestGetPaymentTermsHandler_DefaultsWithoutARow(t *testing.T) {
	e := newEnv(t)

	out := e.readTerms()

	if out.NetDays != payments.DefaultPaymentTermsDays || !out.IsDefault {
		t.Fatalf("terms = (%d, isDefault %v), want (%d, true)", out.NetDays, out.IsDefault, payments.DefaultPaymentTermsDays)
	}
}

// TestPutPaymentTermsHandler_SetsAndRecordsWhoChangedIt covers CLAUDE.md's
// audit-trail expectation for this setting: the change is readable back,
// and the Activity log says who made it and when.
func TestPutPaymentTermsHandler_SetsAndRecordsWhoChangedIt(t *testing.T) {
	e := newEnv(t)

	out := e.setTerms(`{"netDays":45}`)
	if out.NetDays != 45 || out.IsDefault {
		t.Fatalf("terms = (%d, isDefault %v), want (45, false)", out.NetDays, out.IsDefault)
	}
	if got := e.readTerms(); got.NetDays != 45 || got.IsDefault {
		t.Fatalf("read back = (%d, isDefault %v), want (45, false)", got.NetDays, got.IsDefault)
	}

	var actor string
	var diff []byte
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id, diff FROM activity
		 WHERE practice_id = $1 AND action = 'payment_terms_changed'`, e.Practice.ID,
	).Scan(&actor, &diff); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	if actor != e.Staff.ID {
		t.Fatalf("actor = %q, want %q", actor, e.Staff.ID)
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
	e := newEnv(t)

	e.setTerms(`{"netDays":15}`)
	e.setTerms(`{"netDays":15}`)

	rows := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM activity WHERE practice_id = $1 AND action = 'payment_terms_changed'`, e.Practice.ID)
	if rows != 1 {
		t.Fatalf("activity rows = %d, want 1", rows)
	}
}

// TestPutPaymentTermsHandler_RefusesAnImpossibleTerm covers both ends of
// the bound the migration's CHECK also carries, so a typo cannot put an
// Invoice's due date past any horizon a Practice would notice.
func TestPutPaymentTermsHandler_RefusesAnImpossibleTerm(t *testing.T) {
	e := newEnv(t)

	for _, body := range []string{`{"netDays":0}`, `{"netDays":-5}`, `{"netDays":400}`} {
		t.Run(body, func(t *testing.T) {
			out := reqtest.Expect[struct {
				Details map[string]string `json:"details"`
			}](t, http.MethodPut, e.PracticeURL(paymentTermsPath), http.StatusBadRequest, e.SignedIn, termsBody(body))

			// docs/api-design.md section 7 rule 4: a refusal a person
			// causes by filling in a form names the field at fault, keyed
			// by the request DTO's own JSON tag, so the app maps it onto a
			// control with no translation table.
			if out.Details["netDays"] != payments.MsgNetDaysOutOfRange {
				t.Fatalf("details[netDays] = %q, want %q", out.Details["netDays"], payments.MsgNetDaysOutOfRange)
			}
		})
	}
}

// TestPutPaymentTermsHandler_RefusesAMalformedBody covers the decode
// refusal ahead of the range check: a body that is not JSON at all is
// answered before anything reads a number out of it.
func TestPutPaymentTermsHandler_RefusesAMalformedBody(t *testing.T) {
	e := newEnv(t)

	e.expect(http.MethodPut, paymentTermsPath, http.StatusBadRequest, termsBody(`{"netDays":`))
}

// TestPutPaymentTermsHandler_RefusesADoula proves the write side is
// narrower than the read side: every Staff member reads the terms, only
// an Owner or Admin sets them (#282's write table). The refusal comes
// from the mount, not the handler (#990).
func TestPutPaymentTermsHandler_RefusesADoula(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()))

	e.expect(http.MethodPut, paymentTermsPath, http.StatusForbidden, termsBody(`{"netDays":45}`))

	// She still reads it: a Doula meets the due date on every Invoice she
	// looks at.
	if got := e.readTerms(); got.NetDays != payments.DefaultPaymentTermsDays {
		t.Fatalf("doula read = %d, want the default %d", got.NetDays, payments.DefaultPaymentTermsDays)
	}
}

// invoiceDueAtOf reads an Invoice's stored due date straight off the row,
// which is the only place the by-hand rail's due date exists at all.
func invoiceDueAtOf(t *testing.T, db *testdb.DB, invoiceID string) time.Time {
	t.Helper()
	dueAt := testdb.Scalar[time.Time](t, db.Admin, `SELECT due_at FROM invoices WHERE id = $1`, invoiceID)
	return dueAt
}
