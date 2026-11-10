package paymentstanding_test

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/lifecycletest"
	"doula-cloud/api/internal/paymentstanding"
)

// TestStandings_AreTheLabelsTheDerivationWrites holds Standings to the
// labels Lateral's CASE writes, both ways: a standing the SQL never
// writes, or a label Go has no name for, fails here.
func TestStandings_AreTheLabelsTheDerivationWrites(t *testing.T) {
	var labels []paymentstanding.Standing
	for _, m := range regexp.MustCompile(`THEN '([a-z_]+)'|ELSE '([a-z_]+)'`).FindAllStringSubmatch(paymentstanding.Lateral, -1) {
		labels = append(labels, paymentstanding.Standing(m[1]+m[2]))
	}
	declared := slices.Clone(paymentstanding.Standings)
	slices.Sort(labels)
	slices.Sort(declared)
	if !slices.Equal(labels, declared) {
		t.Fatalf("Lateral writes %v, Standings declares %v", labels, paymentstanding.Standings)
	}
}

// TestStanding_Stands: only a reversal takes a Payment off its Invoice. A
// fully refunded one still stands -- its Invoice stays paid (00115).
func TestStanding_Stands(t *testing.T) {
	for _, s := range paymentstanding.Standings {
		if got, want := s.Stands(), s != paymentstanding.Reversed; got != want {
			t.Errorf("%s.Stands() = %v, want %v", s, got, want)
		}
	}
}

// sources is every non-test Go file under api/internal outside this
// package, keyed by its path relative to api/internal.
func sources(t *testing.T) map[string]string {
	t.Helper()
	out := lifecycletest.Sources(t, "..", ".")
	maps.DeleteFunc(out, func(file, _ string) bool { return strings.HasPrefix(file, "paymentstanding/") })
	return out
}

// derivations are the pieces of a Payment's or a Fee Payment's standing
// that only this package may spell: reading the rows that point at one,
// summing Refunds, and naming the kinds that record money arriving.
var derivations = []*regexp.Regexp{
	regexp.MustCompile(`target_(payment_)?id\s*=`),
	regexp.MustCompile(`-SUM\(`),
	regexp.MustCompile(`kind IN \('manual', ?'stripe'\)`),
}

// TestStanding_IsDerivedOnlyHere is the source-scan guardrail. Before
// this package the rule for "the Payment that stands" was retyped at five
// readers and four guards, one of them a variant, and the Fee Payment's
// at four; a reader or guard that spells it again, rather than reading
// Lateral, InvoicePaymentJoin, RefundedSumExpr, ArrivedExpr or
// FeeStandsExpr, fails here.
func TestStanding_IsDerivedOnlyHere(t *testing.T) {
	for file, src := range sources(t) {
		for _, re := range derivations {
			if loc := re.FindStringIndex(src); loc != nil {
				t.Errorf("%s spells %q -- a Payment's standing is derived only by paymentstanding", file, src[loc[0]:loc[1]])
			}
		}
	}
}

// refundWebhookFile reads, writes, and asks its act: it is named thrice.
const refundWebhookFile = "payments/refund_webhook.go"

// readers are the files that read a Payment's or a Fee Payment's
// standing. Each must read it from here.
var readers = []string{
	"payments/invoice.go", "payments/client.go", "payments/practice_invoices.go",
	refundWebhookFile, "payments/payment_acts.go",
	"feepayment/feepayment.go", "offer/list.go",
}

// writers names, for each table, the files that may insert into it: the
// routes of the declared acts (payments.PaymentActs; feepayment's record
// and reverse).
var writers = map[string][]string{
	"INSERT INTO payments": {
		"payments/manual_payment.go", "payments/refund_payment.go",
		refundWebhookFile, "payments/webhook.go",
	},
	"INSERT INTO attachment_fee_payments": {"feepayment/feepayment.go"},
}

// TestStanding_EveryReaderAndWriterIsTheDeclaredOne holds the readers to
// this package, the writers to the declared acts' routes, and every
// route that writes a row pointing at a Payment to the lock that asks
// its act (lockPaymentFor, or resolveRefundTarget, which calls it).
func TestStanding_EveryReaderAndWriterIsTheDeclaredOne(t *testing.T) {
	src := sources(t)
	for _, file := range readers {
		if s, ok := src[file]; !ok || !strings.Contains(s, "paymentstanding.") && !strings.Contains(s, "activePaymentJoin") {
			t.Errorf("%s does not read a standing from paymentstanding", file)
		}
	}
	for stmt, want := range writers {
		var got []string
		for file, s := range src {
			if strings.Contains(s, stmt) {
				got = append(got, file)
			}
		}
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("files holding %q = %v, want %v -- a row is written only by a declared act's route", stmt, got, want)
		}
	}
	pointing := map[string]string{
		"payments/manual_payment.go": "lockPaymentFor(r.Context(), tx, ActReversePayment,",
		"payments/refund_payment.go": "lockPaymentFor(ctx, tx, ActRefundPayment,",
		refundWebhookFile:            "resolveRefundTarget(",
	}
	for file, call := range pointing {
		if !strings.Contains(src[file], call) {
			t.Errorf("%s no longer asks its act before writing (want %q)", file, call)
		}
	}
}
