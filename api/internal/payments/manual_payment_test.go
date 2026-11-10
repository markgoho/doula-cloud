package payments_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// seedByHandInvoiceAmountCents is the fixed amount every caller of
// seedByHandInvoice wants -- none of them are testing the amount, so it
// is not a parameter (golangci-lint's unparam).
const seedByHandInvoiceAmountCents = 15000

// seedByHandInvoice inserts an 'open' by-hand invoices row (no
// stripe_invoice_id) directly, bypassing PostInvoiceHandler.
func seedByHandInvoice(t *testing.T, db *testdb.DB, practiceID, contractID string) (invoiceID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, status, amount_cents, currency, reference, due_at)
		 VALUES ($1, $2, 'open', $3, 'usd', 'INV-TEST', now() + interval '30 days') RETURNING id`,
		practiceID, contractID, seedByHandInvoiceAmountCents,
	).Scan(&invoiceID); err != nil {
		t.Fatalf("seed by-hand invoice: %v", err)
	}
	return invoiceID
}

func isoDate(t time.Time) string { return t.Format("2006-01-02") }

// todayInSeededZone is the paidOn fixture value every "accepted, not in
// the future" test in this file wants: today in the fixture Practice's
// own zone (testdb.SeededPracticeZoneName), not UTC's. Building it in
// UTC would flake for exactly the reason #1167 exists -- UTC's day runs
// ahead of Eastern's for several hours of every day, so a UTC "today" is
// sometimes the Practice's tomorrow.
func todayInSeededZone(t *testing.T) string {
	t.Helper()
	zone, err := time.LoadLocation(testdb.SeededPracticeZoneName)
	if err != nil {
		t.Fatalf("load %s: %v", testdb.SeededPracticeZoneName, err)
	}
	return isoDate(time.Now().In(zone))
}

// byHandInvoice seeds an Engagement at the Practice with a signed Contract
// and an open by-hand Invoice against it.
func (e *env) byHandInvoice() (engagementID, invoiceID string) {
	e.T.Helper()
	return e.byHandInvoiceFor()
}

// byHandInvoiceFor is byHandInvoice on an Engagement shaped by opts.
func (e *env) byHandInvoiceFor(opts ...fixture.EngagementOption) (engagementID, invoiceID string) {
	e.T.Helper()
	engagementID, contractID := e.signedContract(opts...)
	return engagementID, seedByHandInvoice(e.T, e.DB, e.Practice.ID, contractID)
}

// signedContract seeds an Engagement at the Practice with a signed Contract.
func (e *env) signedContract(opts ...fixture.EngagementOption) (engagementID, contractID string) {
	e.T.Helper()
	engagementID = fixture.Engagement(e.T, e.DB, e.Practice, opts...).ID
	return engagementID, seedSignedContract(e.T, e.DB, engagementID)
}

// stripeInvoice seeds the same, with an open Invoice that Stripe issued,
// and gives the Practice a Stripe Connect account when accountID is not
// empty.
func (e *env) stripeInvoice(stripeInvoiceID, accountID string) (invoiceID string) {
	e.T.Helper()
	_, contractID := e.signedContract()
	if accountID != "" {
		seedConnectAccount(e.T, e.DB, e.Practice.ID, accountID)
	}
	return seedInvoice(e.T, e.DB, e.Practice.ID, contractID, stripeInvoiceID, invoiceStatusOpen, 15000, time.Now())
}

func (e *env) activityCount(action, engagementID string) int {
	e.T.Helper()
	return testdb.Scalar[int](e.T, e.DB.Admin, `SELECT count(*) FROM activity WHERE action = $1 AND subject_id = $2`, action, engagementID)
}

// paymentRequest is the body recording a Payment sends.
func paymentRequest(method payments.PaymentMethod, note, paidOn string) reqtest.Option {
	return reqtest.JSON(payments.RecordPaymentRequest{Method: method, Note: note, PaidOn: paidOn})
}

func paymentsPath(invoiceID string) string { return "/invoices/" + invoiceID + "/payments" }

// TestPostManualPaymentHandler_ByHandCheckHappyPath proves the core path:
// an Owner records a check against an open by-hand Invoice for its full
// amount, the Invoice flips to paid, and the Activity log carries who and
// when.
func TestPostManualPaymentHandler_ByHandCheckHappyPath(t *testing.T) {
	e := newEnv(t)
	engagementID, invoiceID := e.byHandInvoice()

	out := reqtest.Expect[payments.PaymentView](t, http.MethodPost, e.PracticeURL(paymentsPath(invoiceID)), http.StatusCreated,
		e.SignedIn, paymentRequest(payments.PaymentMethodCheck, "check #204", todayInSeededZone(t)))

	if out.AmountCents != 15000 {
		t.Fatalf("payment.amountCents = %d, want 15000", out.AmountCents)
	}
	if out.Note == nil || *out.Note != "check #204" {
		t.Fatalf("payment.note = %v, want %q", out.Note, "check #204")
	}
	if status := invoiceStatusFor(t, e.DB, invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusPaid)
	}
	if got := e.activityCount("payment_recorded", engagementID); got != 1 {
		t.Fatalf("payment_recorded activity rows = %d, want 1", got)
	}
}

// TestPostManualPaymentHandler_AcceptsTheRequest proves the positive cases
// beside the refusals below: "other" with a note, and a deposit that
// arrived before the Invoice was raised (#271 sets no lower bound).
func TestPostManualPaymentHandler_AcceptsTheRequest(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method payments.PaymentMethod
		note   string
		paidOn string
	}{
		{"other with a note", payments.PaymentMethodOther, "Venmo, screenshot on file", todayInSeededZone(t)},
		{"a date preceding the Invoice", payments.PaymentMethodBankTransfer, "", "2020-01-01"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			_, invoiceID := e.byHandInvoice()

			e.expect(http.MethodPost, paymentsPath(invoiceID), http.StatusCreated, paymentRequest(tc.method, tc.note, tc.paidOn))
		})
	}
}

// TestPostManualPaymentHandler_FieldRefusals proves a request a person
// causes by filling in a form is refused with the field at fault named,
// keyed by the request DTO's own JSON tag (docs/api-design.md section 7
// rule 4, #1037), and that nothing is recorded.
func TestPostManualPaymentHandler_FieldRefusals(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  reqtest.Option
		field string
		want  string
	}{
		{"other without a note", paymentRequest(payments.PaymentMethodOther, "", todayInSeededZone(t)), "note", `Enter a note for "Other"`},
		{"a method outside the closed enum", paymentRequest("venmo", "", todayInSeededZone(t)), "method", "Select how the payment was made"},
		// No "method" key at all, the body a form with nothing chosen would
		// send (#1913): the form's Method opens with nothing chosen, so a
		// send that skips it is refused here as well as on the page.
		{"no method at all", reqtest.RawJSON([]byte(`{"paidOn":"` + todayInSeededZone(t) + `"}`)), "method", "Select how the payment was made"},
		{"a date that does not parse", paymentRequest(payments.PaymentMethodCash, "", "not-a-date"), "paidOn", "Enter the date received as a real date, like 2027-04-23"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			_, invoiceID := e.byHandInvoice()

			out := e.postRefused(paymentsPath(invoiceID), http.StatusBadRequest, tc.body)

			if out.Details[tc.field] != tc.want {
				t.Fatalf("details[%s] = %q, want %q", tc.field, out.Details[tc.field], tc.want)
			}
			if status := invoiceStatusFor(t, e.DB, invoiceID); status != invoiceStatusOpen {
				t.Fatalf("invoice status = %q, want unchanged %q", status, invoiceStatusOpen)
			}
			if got := paymentsForInvoice(t, e.DB, invoiceID); len(got) != 0 {
				t.Fatalf("payments = %d, want 0", len(got))
			}
		})
	}
}

// TestPostManualPaymentHandler_FutureDateRefused proves paidOn cannot be
// in the future, and that the refusal names the field it is about
// (#1062): a client puts a details entry beside the control it is keyed
// by, and this one used to arrive with nothing but a summary. The
// summary message is asserted unchanged at its own #1037 wording; the
// details entry is asserted at its own, separate, #1189 wording -- the
// two read differently now that the details value opens with "The date"
// rather than "paidOn", the wire's own name for the field.
func TestPostManualPaymentHandler_FutureDateRefused(t *testing.T) {
	e := newEnv(t)
	_, invoiceID := e.byHandInvoice()

	out := e.postRefused(paymentsPath(invoiceID), http.StatusBadRequest,
		paymentRequest(payments.PaymentMethodCash, "", isoDate(time.Now().UTC().AddDate(0, 0, 1))))

	const wantMessage = "paidOn cannot be in the future"
	const wantDetail = "The date cannot be in the future"
	if out.Details["paidOn"] != wantDetail {
		t.Fatalf("details[paidOn] = %q, want %q", out.Details["paidOn"], wantDetail)
	}
	if out.Message != wantMessage {
		t.Fatalf("message = %q, want it unchanged at %q", out.Message, wantMessage)
	}
	if out.Code != apierr.CodeInvalidArgument {
		t.Fatalf("code = %q, want %q", out.Code, apierr.CodeInvalidArgument)
	}
}

// TestPostManualPaymentHandler_UnloadablePracticeZoneIsAnError proves the
// future-date guard refuses rather than answering with the wrong day when
// the Practice's zone will not load (#1167, following
// visit.TestListHandler_UnloadablePracticeZoneIsAnError's own case):
// falling back to UTC would reintroduce exactly the misplacement #953 and
// #1166 exist to end, and say nothing about it.
func TestPostManualPaymentHandler_UnloadablePracticeZoneIsAnError(t *testing.T) {
	e := newEnv(t)
	_, invoiceID := e.byHandInvoice()
	testdb.SetPracticeTimezone(t, e.DB, e.Practice.ID, "Nowhere/Atlantis")

	e.expect(http.MethodPost, paymentsPath(invoiceID), http.StatusInternalServerError, paymentRequest(payments.PaymentMethodCash, "", todayInSeededZone(t)))
}

// TestPostManualPaymentHandler_NotOpenInvoiceRefused proves draft, void,
// uncollectible, and already-paid all refuse.
func TestPostManualPaymentHandler_NotOpenInvoiceRefused(t *testing.T) {
	e := newEnv(t)
	_, contractID := e.signedContract()

	for _, status := range []string{invoiceStatusDraft, invoiceStatusVoid, invoiceStatusUncollectible, invoiceStatusPaid} {
		invoiceID := seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_not_open_"+status, status, 15000, time.Now())

		e.expect(http.MethodPost, paymentsPath(invoiceID), http.StatusConflict, paymentRequest(payments.PaymentMethodCash, "", todayInSeededZone(t)))
	}
}

// TestPostManualPaymentHandler_RefusesTheRequest proves an Invoice id that
// is no uuid or names no Invoice (nor another Practice's), and a body that
// is not JSON, are refused.
func TestPostManualPaymentHandler_RefusesTheRequest(t *testing.T) {
	for _, tc := range []struct {
		name      string
		invoiceID string
		body      reqtest.Option
		want      int
	}{
		{caseInvoiceIDNotUUID, notAUUID, paymentRequest(payments.PaymentMethodCash, "", todayInSeededZone(t)), http.StatusBadRequest},
		{"no such Invoice", unknownID, paymentRequest(payments.PaymentMethodCash, "", todayInSeededZone(t)), http.StatusNotFound},
		{caseBodyNotJSON, "", reqtest.RawJSON([]byte("{not json")), http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			invoiceID := tc.invoiceID
			if invoiceID == "" {
				_, invoiceID = e.byHandInvoice()
			}

			e.expect(http.MethodPost, paymentsPath(invoiceID), tc.want, tc.body)
		})
	}
}

// TestPostManualPaymentHandler_StripeBackedCallsPayOutOfBandThenRecords
// proves recording a Payment against a Stripe-backed Invoice calls
// PayOutOfBand and, on success, writes the manual Payment locally.
func TestPostManualPaymentHandler_StripeBackedCallsPayOutOfBandThenRecords(t *testing.T) {
	e := newEnv(t)
	invoiceID := e.stripeInvoice("in_pay_oob", "acct_pay_oob")

	e.expect(http.MethodPost, paymentsPath(invoiceID), http.StatusCreated, paymentRequest(payments.PaymentMethodBankTransfer, "", todayInSeededZone(t)))

	if len(e.Stripe.PayOutOfBandCalls) != 1 {
		t.Fatalf("PayOutOfBand calls = %d, want 1", len(e.Stripe.PayOutOfBandCalls))
	}
	if got := e.Stripe.PayOutOfBandCalls[0]; got.AccountID != "acct_pay_oob" || got.InvoiceID != "in_pay_oob" {
		t.Fatalf("PayOutOfBand call = %+v, want account acct_pay_oob invoice in_pay_oob", got)
	}
	if status := invoiceStatusFor(t, e.DB, invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusPaid)
	}
}

// TestPostManualPaymentHandler_StripePayOutOfBandFailureRecordsNothing
// proves #271's fail-closed rule: a Stripe refusal on the out-of-band
// call leaves the Invoice open and writes no payments row.
func TestPostManualPaymentHandler_StripePayOutOfBandFailureRecordsNothing(t *testing.T) {
	stripe := payments.NewFakeClient()
	stripe.PayOutOfBandErr = errStripeFake
	e := newEnv(t, withStripe(stripe))
	invoiceID := e.stripeInvoice("in_pay_oob_fail", "acct_pay_oob_fail")

	e.expect(http.MethodPost, paymentsPath(invoiceID), http.StatusBadGateway, paymentRequest(payments.PaymentMethodCheck, "", todayInSeededZone(t)))

	if status := invoiceStatusFor(t, e.DB, invoiceID); status != invoiceStatusOpen {
		t.Fatalf("invoice status = %q, want unchanged %q", status, invoiceStatusOpen)
	}
	if got := paymentsForInvoice(t, e.DB, invoiceID); len(got) != 0 {
		t.Fatalf("payments for invoice = %d, want 0", len(got))
	}
}

// TestPostManualPaymentHandler_PaymentOnItsWayRefused proves #2381: while
// a bank debit is on its way, a Payment recorded by hand is refused before
// Stripe is asked, and nothing is written.
func TestPostManualPaymentHandler_PaymentOnItsWayRefused(t *testing.T) {
	e := newEnv(t)
	invoiceID := e.stripeInvoice("in_on_its_way", "acct_on_its_way")
	markProcessing(t, e.DB, invoiceID)

	out := e.postRefused(paymentsPath(invoiceID), http.StatusConflict, paymentRequest(payments.PaymentMethodCheck, "", todayInSeededZone(t)))

	if out.Code != apierr.CodeFailedPrecondition || out.Message != payments.MsgPaymentOnItsWay {
		t.Fatalf("refusal = %+v, want %s %q", out, apierr.CodeFailedPrecondition, payments.MsgPaymentOnItsWay)
	}
	if len(e.Stripe.PayOutOfBandCalls) != 0 {
		t.Fatalf("PayOutOfBand calls = %d, want 0", len(e.Stripe.PayOutOfBandCalls))
	}
	if status := invoiceStatusFor(t, e.DB, invoiceID); status != invoiceStatusOpen {
		t.Fatalf("invoice status = %q, want unchanged %q", status, invoiceStatusOpen)
	}
	if got := paymentsForInvoice(t, e.DB, invoiceID); len(got) != 0 {
		t.Fatalf("payments for invoice = %d, want 0", len(got))
	}
}

// byHandTransitions are the two acts on an open by-hand Invoice that end
// it without a Payment (#282), which share transitionByHandInvoice and so
// share their refusals.
var byHandTransitions = []struct {
	action   string
	staff    fixture.StaffOption
	status   string
	activity string
}{
	{"void", fixture.Admin(), invoiceStatusVoid, "invoice_voided"},
	{"write-off", fixture.Owner(), invoiceStatusUncollectible, "invoice_written_off"},
}

func invoiceActionPath(invoiceID, action string) string {
	return "/invoices/" + invoiceID + "/" + action
}

// TestPostInvoiceTransitionHandler_HappyPath proves an Owner or Admin can
// void or write off an open by-hand Invoice, and that the Activity log
// carries it.
func TestPostInvoiceTransitionHandler_HappyPath(t *testing.T) {
	for _, tc := range byHandTransitions {
		t.Run(tc.action, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.staff))
			engagementID, invoiceID := e.byHandInvoice()

			out := reqtest.Expect[payments.InvoiceTransitionView](t, http.MethodPost, e.PracticeURL(invoiceActionPath(invoiceID, tc.action)), http.StatusOK,
				e.SignedIn, reqtest.RawJSON([]byte("{}")))

			if out.Status != tc.status {
				t.Fatalf("status = %q, want %q", out.Status, tc.status)
			}
			if got := invoiceStatusFor(t, e.DB, invoiceID); got != tc.status {
				t.Fatalf("persisted invoice status = %q, want %q", got, tc.status)
			}
			if got := e.activityCount(tc.activity, engagementID); got != 1 {
				t.Fatalf("%s activity rows = %d, want 1", tc.activity, got)
			}
		})
	}
}

// TestPostInvoiceTransitionHandler_Refusals proves the refusals void and
// write-off share: a Stripe-backed Invoice cannot be ended here, nor one
// that is not open, nor one a Doula asks for (AC7's "a test for each
// refusal" names both acts), and an unknown or malformed Invoice id is
// refused.
func TestPostInvoiceTransitionHandler_Refusals(t *testing.T) {
	for _, tc := range byHandTransitions {
		for _, refusal := range []struct {
			name    string
			staff   fixture.StaffOption
			invoice func(*env) string
			id      string
			want    int
		}{
			{"a Stripe-backed Invoice", tc.staff, func(e *env) string { id := e.stripeInvoice("in_"+tc.action+"_stripe", ""); return id }, "", http.StatusConflict},
			{"an Invoice that is not open", tc.staff, func(e *env) string {
				_, id := e.byHandInvoice()
				if _, err := e.DB.Admin.ExecContext(e.T.Context(), `UPDATE invoices SET status = 'paid', paid_at = now() WHERE id = $1`, id); err != nil {
					e.T.Fatalf("seed paid status: %v", err)
				}
				return id
			}, "", http.StatusConflict},
			{caseDoula, fixture.Doula(), func(e *env) string { _, id := e.byHandInvoice(); return id }, "", http.StatusForbidden},
			{"no such Invoice", tc.staff, nil, unknownID, http.StatusNotFound},
			{caseInvoiceIDNotUUID, tc.staff, nil, notAUUID, http.StatusBadRequest},
		} {
			t.Run(tc.action+"/"+refusal.name, func(t *testing.T) {
				e := newEnv(t, signedInAs(refusal.staff))
				invoiceID := refusal.id
				if refusal.invoice != nil {
					invoiceID = refusal.invoice(e)
				}

				e.expect(http.MethodPost, invoiceActionPath(invoiceID, tc.action), refusal.want, reqtest.RawJSON([]byte("{}")))

				if refusal.invoice != nil && refusal.want == http.StatusConflict {
					if got := invoiceStatusFor(t, e.DB, invoiceID); got == tc.status {
						t.Fatalf("persisted invoice status = %q, want it not to have changed", got)
					}
				}
			})
		}
	}
}

// TestPostManualPaymentHandler_DoulaForbidden proves only Owner and Admin
// may record a Payment.
func TestPostManualPaymentHandler_DoulaForbidden(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()))
	_, invoiceID := e.byHandInvoice()

	e.expect(http.MethodPost, paymentsPath(invoiceID), http.StatusForbidden, paymentRequest(payments.PaymentMethodCash, "", todayInSeededZone(t)))
}

// invoiceStatusFor is invoiceStatus with a name that does not collide
// with webhook_test.go's own helper of nearly the same shape (which also
// returns paid_at).
func invoiceStatusFor(t *testing.T, db *testdb.DB, invoiceID string) string {
	t.Helper()
	return invoiceStatus(t, db, invoiceID)
}
