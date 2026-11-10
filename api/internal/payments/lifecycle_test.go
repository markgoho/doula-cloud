package payments_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/invoicestatus"
	"doula-cloud/api/internal/payments"
)

// declaredMove is what TestInvoiceTransitions_TableIsComplete pins for one
// Transition, and where lifecycle_test.go's source scan expects to find
// the route that makes it.
type declaredMove struct {
	varName string
	file    string
	from    []invoicestatus.Status
	to      invoicestatus.Status
	rails   []payments.BillingMode
	by      payments.Mover
}

// The two route files that make more than one declared move.
const (
	manualPaymentFile = "manual_payment.go"
	webhookFile       = "webhook.go"
)

var (
	stripeOnly = []payments.BillingMode{payments.BillingModeStripe}
	byHandOnly = []payments.BillingMode{payments.BillingModeByHand}
	bothRails  = []payments.BillingMode{payments.BillingModeStripe, payments.BillingModeByHand}
)

// declaredMoves is every Invoice lifecycle move, keyed by its Name: the
// table a reader checks the lifecycle against without reading the
// handlers.
var declaredMoves = map[string]declaredMove{
	"finalize": {"TransitionFinalize", "invoice.go",
		[]invoicestatus.Status{invoicestatus.Draft}, invoicestatus.Open, stripeOnly, payments.MovedByStaff},
	"abandon a draft Stripe would not finalize": {"TransitionAbandonDraft", "invoice.go",
		[]invoicestatus.Status{invoicestatus.Draft}, invoicestatus.Void, stripeOnly, payments.MovedByStaff},
	"record a payment": {"TransitionRecordPayment", manualPaymentFile,
		[]invoicestatus.Status{invoicestatus.Open}, invoicestatus.Paid, bothRails, payments.MovedByStaff},
	"void": {"TransitionVoid", manualPaymentFile,
		[]invoicestatus.Status{invoicestatus.Open}, invoicestatus.Void, byHandOnly, payments.MovedByStaff},
	"write off": {"TransitionWriteOff", manualPaymentFile,
		[]invoicestatus.Status{invoicestatus.Open}, invoicestatus.Uncollectible, byHandOnly, payments.MovedByStaff},
	"reverse a payment": {"TransitionReversePayment", manualPaymentFile,
		[]invoicestatus.Status{invoicestatus.Paid}, invoicestatus.Open, byHandOnly, payments.MovedByStaff},
	"refund a payment": {"TransitionRefund", "refund_payment.go",
		[]invoicestatus.Status{invoicestatus.Paid}, invoicestatus.Paid, bothRails, payments.MovedByStaff},
	"pay in the portal": {"TransitionClientPay", "client_pay.go",
		[]invoicestatus.Status{invoicestatus.Open}, invoicestatus.Open, stripeOnly, payments.MovedByClient},
	"invoice.paid": {"TransitionStripePaid", webhookFile,
		[]invoicestatus.Status{invoicestatus.Draft, invoicestatus.Open, invoicestatus.Uncollectible}, invoicestatus.Paid, stripeOnly, payments.MovedByStripe},
	"invoice.marked_uncollectible": {"TransitionStripeUncollectible", webhookFile,
		[]invoicestatus.Status{invoicestatus.Draft, invoicestatus.Open}, invoicestatus.Uncollectible, stripeOnly, payments.MovedByStripe},
	"invoice.voided": {"TransitionStripeVoided", webhookFile,
		[]invoicestatus.Status{invoicestatus.Draft, invoicestatus.Open, invoicestatus.Uncollectible}, invoicestatus.Void, stripeOnly, payments.MovedByStripe},
	"payment_intent.processing": {"TransitionPaymentOnItsWay", webhookFile,
		[]invoicestatus.Status{invoicestatus.Open}, invoicestatus.Open, stripeOnly, payments.MovedByStripe},
}

// TestInvoiceTransitions_TableIsComplete holds payments.InvoiceTransitions
// to exactly the moves above: a Transition added without an entry here,
// or one whose stages or rail drift from it, fails.
func TestInvoiceTransitions_TableIsComplete(t *testing.T) {
	if len(payments.InvoiceTransitions) != len(declaredMoves) {
		t.Fatalf("len(InvoiceTransitions) = %d, want %d", len(payments.InvoiceTransitions), len(declaredMoves))
	}
	for _, tr := range payments.InvoiceTransitions {
		want, known := declaredMoves[tr.Name]
		if !known {
			t.Errorf("InvoiceTransitions has unexpected entry %q", tr.Name)
			continue
		}
		if !slices.Equal(tr.From, want.from) || tr.To != want.to || !slices.Equal(tr.Rails, want.rails) || tr.By != want.by {
			t.Errorf("%q = from %v to %q on %v by %q, want from %v to %q on %v by %q",
				tr.Name, tr.From, tr.To, tr.Rails, tr.By, want.from, want.to, want.rails, want.by)
		}
	}
}

// TestInvoiceTransitions_VoidIsTerminalForMoney is the decision the
// audit asked for in so many words: Stripe's late invoice.paid may move
// an uncollectible Invoice (the money moved, so it is recorded) but never
// a void one, and nothing at all leaves void.
func TestInvoiceTransitions_VoidIsTerminalForMoney(t *testing.T) {
	if ok, _ := payments.TransitionStripePaid.Check(invoicestatus.Uncollectible, payments.BillingModeStripe); !ok {
		t.Error("invoice.paid refuses an uncollectible Invoice, want it recorded")
	}
	for _, tr := range payments.InvoiceTransitions {
		if slices.Contains(tr.From, invoicestatus.Void) {
			t.Errorf("%q starts from void, want void to have no exit", tr.Name)
		}
	}
}

// TestInvoiceTransition_Check proves Check's three outcomes and their
// order: the rail is asked first, so a Stripe-backed Invoice that is also
// not open reads the rail refusal the void route has always written.
func TestInvoiceTransition_Check(t *testing.T) {
	cases := []struct {
		name       string
		transition payments.InvoiceTransition
		current    invoicestatus.Status
		rail       payments.BillingMode
		wantOK     bool
		want       string
	}{
		{"allowed", payments.TransitionVoid, invoicestatus.Open, payments.BillingModeByHand, true, ""},
		{"wrong rail before wrong stage", payments.TransitionVoid, invoicestatus.Paid, payments.BillingModeStripe, false, payments.MsgStripeInvoiceCannotBeVoidedOrWrittenOff},
		{"wrong stage", payments.TransitionVoid, invoicestatus.Paid, payments.BillingModeByHand, false, payments.MsgInvoiceNotOpen},
		{"a Stripe-only move's stage refusal is for the log", payments.TransitionStripePaid, invoicestatus.Void, payments.BillingModeStripe, false, "invoice.paid cannot move an Invoice that is void"},
		{"a Stripe-only move's rail refusal is for the log", payments.TransitionStripeVoided, invoicestatus.Open, payments.BillingModeByHand, false, "invoice.voided does not apply to a by_hand Invoice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, refusal := tc.transition.Check(tc.current, tc.rail)
			if ok != tc.wantOK || refusal != tc.want {
				t.Fatalf("Check(%q, %q) = (%v, %q), want (%v, %q)", tc.current, tc.rail, ok, refusal, tc.wantOK, tc.want)
			}
		})
	}
}

// TestInvoiceTransition_Reached tells a redelivery (already where the move
// leaves it) from a refusal, and never calls a move that changes no
// status already reached.
func TestInvoiceTransition_Reached(t *testing.T) {
	if !payments.TransitionStripeVoided.Reached(invoicestatus.Void) {
		t.Error("invoice.voided on a void Invoice is not Reached, want a redelivery")
	}
	if payments.TransitionStripeVoided.Reached(invoicestatus.Paid) {
		t.Error("invoice.voided on a paid Invoice is Reached, want a refusal")
	}
	if payments.TransitionPaymentOnItsWay.Reached(invoicestatus.Open) {
		t.Error("payment_intent.processing on an open Invoice is Reached, want it applied")
	}
}

// TestInvoiceMoves_GoThroughTheTable is the source-scan guardrail
// contracts' lifecycle_test.go keeps for Contracts, for Invoices. Before
// it, every move was an inline status comparison beside a literal UPDATE,
// and invoice.paid's guard let a void Invoice be paid. It catches what a
// passing status-code assertion would not: a route that stops consulting
// its Transition, or a new writer of invoices.status that never asks.
//
//  1. Exactly one non-test file under api/internal writes invoices.status:
//     payments/lifecycle.go's moveInvoice, which writes t.To.
//  2. Every payments file that moves an Invoice consults a Check.
//  3. Each declared Transition is named in the file of the route that
//     makes it, so a Transition with no route, or a route that swapped
//     its Transition for a literal, fails.
func TestInvoiceMoves_GoThroughTheTable(t *testing.T) {
	const writesStatus = "UPDATE invoices SET status"
	var writers []string
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// coverage:ignore reason: api/internal is always walkable while its tests run
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path) //nolint:gosec // path comes from WalkDir over this repo's own api/internal tree, never from input
		if err != nil {
			// coverage:ignore reason: a file WalkDir listed a moment ago
			return fmt.Errorf("read %s: %w", path, err)
		}
		if strings.Contains(string(src), writesStatus) {
			writers = append(writers, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk api/internal: %v", err)
	}
	if !slices.Equal(writers, []string{"../payments/lifecycle.go"}) {
		t.Errorf("files writing %q = %v, want only ../payments/lifecycle.go -- an Invoice's status moves through moveInvoice and a declared Transition", writesStatus, writers)
	}

	read := func(name string) string {
		t.Helper()
		// #nosec G304 -- name is one of this package's own file names, fixed in this test
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(src)
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		// coverage:ignore reason: this package's own directory is always readable while its tests run
		t.Fatalf("read package directory: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "lifecycle.go" {
			continue
		}
		src := read(name)
		if strings.Contains(src, "moveInvoice(") && !strings.Contains(src, ".Check(") && !strings.Contains(src, "takeStripeMove(") {
			t.Errorf("%s moves an Invoice but never asks a Transition's Check", name)
		}
	}
	if !strings.Contains(read(webhookFile), "t.Check(current, BillingModeStripe)") {
		t.Error("webhook.go's takeStripeMove no longer asks the Transition's Check")
	}
	for name, move := range declaredMoves {
		if !strings.Contains(read(move.file), move.varName) {
			t.Errorf("%s does not name %s -- the route that makes %q must consult it", move.file, move.varName, name)
		}
	}
}
