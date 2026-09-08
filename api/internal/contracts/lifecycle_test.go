package contracts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"doula-cloud/api/internal/contracts"
)

// TestTransitions_DeclaresEveryContractRoute is the guardrail #275's
// third acceptance criterion asks for: every route that changes or bills
// a Contract must consult contracts.Transitions' one declaration rather
// than restating its own status comparison. It is a source scan, not a
// behavior check -- the per-handler Rejected/Success tests in this
// package (and payments' own invoice tests) already prove each route's
// refusal fires correctly; what this test alone catches is a route that
// stops calling its declared Transition (or a new route that is added
// without ever calling one), which a passing status-code assertion would
// not notice if the literal comparison it replaced quietly came back.
//
// The scan is self-updating for anything inside this package: any
// non-test file that writes to the contracts row (`UPDATE contracts
// SET`) must also call some Transition's Check, so a sixth in-package
// route that mutates a Contract is caught the moment it is added, the
// same "walk the directory, don't hardcode the file list" shape
// main package's packageSources uses. It can't reach the same way into
// payments -- an Invoice-create never writes the contracts row at all,
// which is exactly how #275 slipped through -- so that one entry stays
// named explicitly.
func TestTransitions_DeclaresEveryContractRoute(t *testing.T) {
	const mutatesContractStatement = "UPDATE contracts SET"
	const checksATransition = ".Check("

	entries, err := os.ReadDir(".")
	if err != nil {
		// coverage:ignore reason: this package's own directory is always readable while its tests run
		t.Fatalf("read package directory: %v", err)
	}
	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// #nosec G304 -- name comes from ReadDir on this package's own
		// directory, not from anything a caller supplies
		src, err := os.ReadFile(name)
		if err != nil {
			// coverage:ignore reason: a file ReadDir listed a moment ago
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(src), mutatesContractStatement) {
			continue
		}
		scanned++
		if !strings.Contains(string(src), checksATransition) {
			t.Errorf("%s writes %q but never calls a Transition's Check -- a route that changes a Contract's status must consult contracts.Transitions' declaration, not restate its own comparison", name, mutatesContractStatement)
		}
	}
	if scanned == 0 {
		t.Fatal("no in-package file writes 'UPDATE contracts SET' -- did contract.go/send.go/sign.go/void.go move or stop mutating the row this scan looks for?")
	}

	// Invoice-create lives in the payments package and never writes the
	// contracts row itself (#275's whole point), so it is the one entry
	// that reaches outside this package's own directory rather than
	// falling out of the scan above.
	invoicePath := filepath.Join("..", "payments", "invoice.go")
	// #nosec G304 -- invoicePath is a fixed, hardcoded relative path, not
	// anything a caller supplies
	src, err := os.ReadFile(invoicePath)
	if err != nil {
		t.Fatalf("read %s: %v", invoicePath, err)
	}
	if !strings.Contains(string(src), "contracts.TransitionBill.Check(") {
		t.Errorf("%s does not call contracts.TransitionBill.Check -- the Invoice-create route must consult contracts.Transitions' declaration", invoicePath)
	}
}

// TestTransitions_TableIsComplete proves contracts.Transitions -- the
// registry the guardrail above exists to keep honest -- carries exactly
// the declared preconditions, each requiring the status its route's doc
// comment says it does. A new Transition added to the var block without
// a matching entry here (or a matching route above) is exactly the drift
// #275 found once, on the Invoice route that had no entry at all.
func TestTransitions_TableIsComplete(t *testing.T) {
	want := map[string]contracts.Status{
		"edited":            contracts.StatusDraft,
		statusSent:          contracts.StatusDraft,
		"signed":            contracts.StatusSent,
		"voided":            contracts.StatusSigned,
		"billed":            contracts.StatusSigned,
		"amount overridden": contracts.StatusDraft,
	}
	if len(contracts.Transitions) != len(want) {
		t.Fatalf("len(Transitions) = %d, want %d", len(contracts.Transitions), len(want))
	}
	for _, tr := range contracts.Transitions {
		requires, known := want[tr.PastTense]
		if !known {
			t.Errorf("Transitions has unexpected entry %+v", tr)
			continue
		}
		if tr.Requires != requires {
			t.Errorf("Transition %q requires %q, want %q", tr.PastTense, tr.Requires, requires)
		}
	}
}

// TestTransition_Check proves Check's two outcomes: satisfied (no
// refusal) and refused, with a message naming the state the Contract is
// actually in -- #275's acceptance criterion for the Invoice route,
// generalized to every Transition since they now share one
// implementation.
func TestTransition_Check(t *testing.T) {
	if ok, refusal := contracts.TransitionBill.Check(contracts.StatusSigned); !ok || refusal != "" {
		t.Fatalf("Check(signed) = (%v, %q), want (true, \"\")", ok, refusal)
	}
	ok, refusal := contracts.TransitionBill.Check(contracts.StatusVoided)
	if ok {
		t.Fatal("Check(voided) = true, want false")
	}
	if !strings.Contains(refusal, "voided") {
		t.Errorf("refusal = %q, want it to name the actual status %q", refusal, contracts.StatusVoided)
	}
}
