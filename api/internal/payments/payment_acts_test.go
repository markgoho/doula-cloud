package payments_test

import (
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/paymentstanding"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// targetIsAPaymentOnItsInvoice is 00175's refusal of a row pointing at
// something that is not a Payment on its own Invoice.
const targetIsAPaymentOnItsInvoice = "payments_target_is_a_payment_on_its_invoice"

// declaredAct is what TestPaymentActs_TableIsComplete pins for one act.
type declaredAct struct {
	kinds  []string
	from   []paymentstanding.Standing
	to     []paymentstanding.Standing
	writes []string
}

var (
	live           = paymentstanding.Live
	partlyRefunded = paymentstanding.PartlyRefunded
	fullyRefunded  = paymentstanding.FullyRefunded
	reversed       = paymentstanding.Reversed
)

// declaredActs is every act on a payments row, keyed by its Name: the
// table a reader checks the Payment's lifecycle against without reading
// the handlers.
var declaredActs = map[string]declaredAct{
	"record a payment": {nil, nil, []paymentstanding.Standing{live},
		[]string{paymentstanding.KindManual, paymentstanding.KindStripe}},
	"reverse a payment": {[]string{paymentstanding.KindManual}, []paymentstanding.Standing{live},
		[]paymentstanding.Standing{reversed}, []string{paymentstanding.KindReversal}},
	"refund a payment": {[]string{paymentstanding.KindManual, paymentstanding.KindStripe}, []paymentstanding.Standing{live, partlyRefunded},
		[]paymentstanding.Standing{partlyRefunded, fullyRefunded}, []string{paymentstanding.KindRefund}},
}

// TestPaymentActs_TableIsComplete holds payments.PaymentActs to exactly
// the acts above, and every standing an act does not start from to a
// declared refusal.
func TestPaymentActs_TableIsComplete(t *testing.T) {
	if len(payments.PaymentActs) != len(declaredActs) {
		t.Fatalf("len(PaymentActs) = %d, want %d", len(payments.PaymentActs), len(declaredActs))
	}
	for _, a := range payments.PaymentActs {
		want, known := declaredActs[a.Name]
		if !known {
			t.Errorf("PaymentActs has unexpected entry %q", a.Name)
			continue
		}
		if !slices.Equal(a.Kinds, want.kinds) || !slices.Equal(a.From, want.from) || !slices.Equal(a.To, want.to) || !slices.Equal(a.Writes, want.writes) {
			t.Errorf("%q = kinds %v from %v to %v writes %v, want kinds %v from %v to %v writes %v",
				a.Name, a.Kinds, a.From, a.To, a.Writes, want.kinds, want.from, want.to, want.writes)
		}
		if len(a.From) == 0 {
			continue
		}
		for _, s := range paymentstanding.Standings {
			if !slices.Contains(a.From, s) && a.Refusals[s] == "" {
				t.Errorf("%q has no refusal for a Payment that is %s", a.Name, s)
			}
		}
	}
}

// TestPaymentAct_Check pins each refusal a person reads, word for word
// what the routes wrote before the acts were declared.
func TestPaymentAct_Check(t *testing.T) {
	cases := []struct {
		act      payments.PaymentAct
		standing paymentstanding.Standing
		wantOK   bool
		want     string
	}{
		{payments.ActReversePayment, live, true, ""},
		{payments.ActReversePayment, reversed, false, payments.MsgPaymentAlreadyReversed},
		{payments.ActReversePayment, partlyRefunded, false, payments.MsgRefundedPaymentCannotBeReversed},
		{payments.ActReversePayment, fullyRefunded, false, payments.MsgRefundedPaymentCannotBeReversed},
		{payments.ActRefundPayment, live, true, ""},
		{payments.ActRefundPayment, partlyRefunded, true, ""},
		{payments.ActRefundPayment, fullyRefunded, false, payments.MsgRefundExceedsPayment},
		{payments.ActRefundPayment, reversed, false, payments.MsgReversedPaymentCannotBeRefunded},
		{payments.ActRecordPayment, live, false, "record a payment cannot start from a Payment that is live"},
	}
	for _, tc := range cases {
		ok, refusal := tc.act.Check(tc.standing)
		if ok != tc.wantOK || refusal != tc.want {
			t.Errorf("%s.Check(%s) = (%v, %q), want (%v, %q)", tc.act.Name, tc.standing, ok, refusal, tc.wantOK, tc.want)
		}
	}
}

// readStanding derives paymentID's standing through the one derivation.
func (e *env) readStanding(paymentID string) (paymentstanding.Standing, int64) {
	e.T.Helper()
	standing, refunded, err := paymentstanding.Read(e.T.Context(), e.DB.Admin, paymentID)
	if err != nil {
		e.T.Fatalf("read standing of %s: %v", paymentID, err)
	}
	return standing, refunded
}

// invoiceRow is engagementID's listed Invoice invoiceID.
func (e *env) invoiceRow(engagementID, invoiceID string) payments.InvoiceView {
	e.T.Helper()
	for _, it := range e.listInvoices(engagementID).Items {
		if it.ID == invoiceID {
			return it
		}
	}
	e.T.Fatalf("invoice %s is not listed", invoiceID)
	return payments.InvoiceView{}
}

// TestPaymentStanding_EachActMovesTheStandingTheListReads walks one
// by-hand Payment through every standing a Refund can leave it in, and
// another to Reversed, reading each from the one derivation and from the
// Invoice list -- whose RefundableCents and Reversible are what the
// screen offers, so they must change exactly when the routes' answers do.
func TestPaymentStanding_EachActMovesTheStandingTheListReads(t *testing.T) {
	e := newEnv(t)
	engagementID, invoiceID, paymentID := e.paidByHandInvoice()

	check := func(wantStanding paymentstanding.Standing, wantRefunded, wantRefundable int64, wantReversible bool) {
		t.Helper()
		if standing, refunded := e.readStanding(paymentID); standing != wantStanding || refunded != wantRefunded {
			t.Fatalf("standing = (%s, %d), want (%s, %d)", standing, refunded, wantStanding, wantRefunded)
		}
		row := e.invoiceRow(engagementID, invoiceID)
		if row.RefundedCents != wantRefunded || row.RefundableCents != wantRefundable || row.Reversible != wantReversible {
			t.Fatalf("listed = refunded %d refundable %d reversible %v, want %d %d %v",
				row.RefundedCents, row.RefundableCents, row.Reversible, wantRefunded, wantRefundable, wantReversible)
		}
	}

	check(live, 0, seedByHandInvoiceAmountCents, true)
	e.refund(invoiceID, paymentID, checkRefund(1000))
	check(partlyRefunded, 1000, seedByHandInvoiceAmountCents-1000, false)
	e.refund(invoiceID, paymentID, checkRefund(seedByHandInvoiceAmountCents-1000))
	check(fullyRefunded, seedByHandInvoiceAmountCents, 0, false)
	e.refuseRefund(invoiceID, paymentID, checkRefund(1), http.StatusConflict, payments.MsgRefundExceedsPayment)

	otherEngagement, otherInvoice, otherPayment := e.paidByHandInvoice()
	e.reverse(otherInvoice, otherPayment, "the check bounced")
	if standing, _ := e.readStanding(otherPayment); standing != reversed {
		t.Fatalf("standing = %s, want reversed", standing)
	}
	row := e.invoiceRow(otherEngagement, otherInvoice)
	if row.ActivePaymentID != nil || row.RefundableCents != 0 || row.Reversible {
		t.Fatalf("reopened Invoice = %+v, want no standing Payment and nothing to offer", row)
	}
}

// TestPaymentStanding_ACardPaymentIsRefundableButNeverReversible: a
// Stripe-collected Payment can be refunded and is never reversed, so the
// list offers the one and not the other.
func TestPaymentStanding_ACardPaymentIsRefundableButNeverReversible(t *testing.T) {
	e := newEnv(t)
	engagementID, invoiceID := e.seedStripeInvoice("in_card_standing", invoiceStatusOpen, 30000)
	seedStripePayment(t, e.DB, invoiceID, 30000, "pi_card_standing")

	row := e.invoiceRow(engagementID, invoiceID)
	if row.ActivePaymentKind != paymentstanding.KindStripe || row.RefundableCents != 30000 || row.Reversible {
		t.Fatalf("listed = %+v, want a card Payment with 30000 to return and no reversal", row)
	}
}

// TestPaymentsRefuseWhatNoActWrites pins 00175's trigger: each row below
// is one no declared PaymentAct writes, and the database refuses it,
// naming the rule, rather than storing it -- the rules 00103 and 00115
// left to the handlers alone.
func TestPaymentsRefuseWhatNoActWrites(t *testing.T) {
	e := newEnv(t)
	_, paidInvoice, _ := e.paidByHandInvoice()
	_, _, otherPayment := e.paidByHandInvoice()
	_, refundedInvoice, refundedPayment := e.paidByHandInvoice()
	e.refund(refundedInvoice, refundedPayment, checkRefund(500))
	_, reversedInvoice, reversedPayment := e.paidByHandInvoice()
	reversal := e.reverse(reversedInvoice, reversedPayment, "logged on the wrong Invoice").ID
	_, cardInvoice := e.seedStripeInvoice("in_trigger_card", invoiceStatusOpen, 20000)
	cardPayment := seedStripePayment(t, e.DB, cardInvoice, 20000, "pi_trigger_card")

	cases := []struct {
		name       string
		query      string
		args       []any
		constraint string
	}{
		{"a second Payment on a paid Invoice",
			`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, method) VALUES ($1, 100, now(), 'manual', 'cash')`,
			[]any{paidInvoice}, "payments_one_standing_payment_per_invoice"},
		{"a reversal of a reversal",
			`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id, reason) VALUES ($1, -100, now(), 'reversal', $2, 'again')`,
			[]any{reversedInvoice, reversal}, targetIsAPaymentOnItsInvoice},
		{"a Refund of another Invoice's Payment",
			`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id) VALUES ($1, -100, now(), 'refund', $2)`,
			[]any{paidInvoice, otherPayment}, targetIsAPaymentOnItsInvoice},
		{"a Refund of a reversed Payment",
			`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id) VALUES ($1, -100, now(), 'refund', $2)`,
			[]any{reversedInvoice, reversedPayment}, "payments_target_stands"},
		{"a reversal of a card Payment",
			`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id, reason) VALUES ($1, -20000, now(), 'reversal', $2, 'no')`,
			[]any{cardInvoice, cardPayment}, "payments_reversal_of_a_manual_payment"},
		{"a reversal of a refunded Payment",
			`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id, reason) VALUES ($1, $2, now(), 'reversal', $3, 'no')`,
			[]any{refundedInvoice, -seedByHandInvoiceAmountCents, refundedPayment}, "payments_reversal_of_an_unrefunded_payment"},
		{"Refunds summing to more than the Payment",
			`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id) VALUES ($1, $2, now(), 'refund', $3)`,
			[]any{refundedInvoice, -(seedByHandInvoiceAmountCents - 499), refundedPayment}, "payments_refunds_within_payment"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.DB.Admin.ExecContext(t.Context(), tc.query, tc.args...)
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); !ok || pgErr.ConstraintName != tc.constraint {
				t.Fatalf("insert err = %v, want a violation of %s", err, tc.constraint)
			}
		})
	}
	// What the acts do write still goes in: the rest of a partly refunded
	// Payment, and a fresh Payment on an Invoice whose only one was
	// reversed.
	if _, err := e.DB.Admin.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id) VALUES ($1, $2, now(), 'refund', $3)`,
		refundedInvoice, -(seedByHandInvoiceAmountCents - 500), refundedPayment); err != nil {
		t.Fatalf("refund the rest: %v", err)
	}
	if _, err := e.DB.Admin.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, method) VALUES ($1, 100, now(), 'manual', 'cash')`, reversedInvoice); err != nil {
		t.Fatalf("record again after a reversal: %v", err)
	}
}

// TestPaymentsTrigger_ReadsUnderTheInsertersRLS proves the trigger runs as
// the inserting role: a Refund naming another Practice's Payment finds no
// target under this Practice's RLS and is refused, rather than the
// trigger reading across the boundary to judge it.
func TestPaymentsTrigger_ReadsUnderTheInsertersRLS(t *testing.T) {
	e := newEnv(t)
	_, invoiceID, _ := e.paidByHandInvoice()
	otherPractice := testdb.SeedPractice(t, e.DB, "Another Practice")
	_, otherEngagement := testdb.SeedNamedEngagement(t, e.DB, otherPractice, "Other Client", "other@example.com")
	otherInvoice := seedByHandInvoice(t, e.DB, otherPractice, seedSignedContract(t, e.DB, otherEngagement))
	foreignPayment := testdb.Scalar[string](t, e.DB.Admin,
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, method) VALUES ($1, 100, now(), 'manual', 'cash') RETURNING id`, otherInvoice)

	tx := e.DB.BeginAs(t, testdb.Practice(e.Practice.ID))
	defer func() { _ = tx.Rollback() }()
	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id) VALUES ($1, -100, now(), 'refund', $2)`,
		invoiceID, foreignPayment)
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); !ok || pgErr.ConstraintName != targetIsAPaymentOnItsInvoice {
		t.Fatalf("insert err = %v, want the target refused", err)
	}
}

// TestPostReversePaymentHandler_RefusesACardPaymentAsNotThere: the act
// names by-hand Payments only, so a card Payment on a by-hand-rail check
// reads as one that is not there -- the 404 a reversal row or an unknown
// id reads. Reached on a by-hand Invoice whose Payment was seeded as a
// card one, since a Stripe-backed Invoice is refused at its rail first.
func TestPostReversePaymentHandler_RefusesACardPaymentAsNotThere(t *testing.T) {
	e := newEnv(t)
	_, invoiceID := e.byHandInvoice()
	cardPayment := seedStripePayment(t, e.DB, invoiceID, seedByHandInvoiceAmountCents, "pi_on_by_hand")

	got := e.postRefused(paymentsPath(invoiceID)+"/"+cardPayment+"/reverse", http.StatusNotFound,
		reqtest.JSON(payments.ReversePaymentRequest{Reason: "no"}))
	if got.Message != payments.MsgPaymentNotReversible {
		t.Fatalf("message = %q, want %q", got.Message, payments.MsgPaymentNotReversible)
	}
}
