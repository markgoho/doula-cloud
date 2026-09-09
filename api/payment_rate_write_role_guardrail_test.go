package main

import "testing"

// paymentRateWriteRoutes are the payments/practicerate writes #990 exists
// to gate: #970 moved every Contract write's role check from in-handler
// to the mount, but left these three role-gated writes as an
// in-handler-only check (payments.PutBillingModeHandler,
// payments.PostManualPaymentHandler and the by-hand Invoice void/write-off
// sharing transitionByHandInvoice, and practicerate.PutRateHandler) --
// invisible to the startup panic and to a registry-walking test the same
// way Contract void used to be. This is those three handlers' four
// mounted routes (transitionByHandInvoice backs two), the payments/
// practicerate analogue of contractWriteRoutes above. The map's value is
// unused -- only the key set matters.
var paymentRateWriteRoutes = map[string]bool{
	"PUT /api/practices/{practiceId}/payments/billing-mode":                              true,
	"POST /api/practices/{practiceId}/invoices/{invoiceId}/payments":                     true,
	"POST /api/practices/{practiceId}/invoices/{invoiceId}/payments/{paymentId}/reverse": true,
	"POST /api/practices/{practiceId}/invoices/{invoiceId}/void":                         true,
	"POST /api/practices/{practiceId}/invoices/{invoiceId}/write-off":                    true,
	"PUT /api/practices/{practiceId}/rates/{kind}":                                       true,
}

// TestRoutes_PaymentAndRateWritesDeclareRoles is #990's own guardrail,
// mirroring TestRoutes_ContractWritesDeclareRoles: the startup panic in
// staffauth.GatedRouter.GatedWrite catches an empty role list passed to
// the gated door, but it cannot catch one of these writes mounted through
// the role-free Exempt/Replayable door instead -- that path never calls
// GatedWrite at all. This walks the real registry idempotency.Router
// builds and fails if any of paymentRateWriteRoutes' patterns is missing
// a role declaration, or missing from the registry entirely (mounted
// under a different pattern, or not mounted through ExemptGated /
// ReplayableGated).
func TestRoutes_PaymentAndRateWritesDeclareRoles(t *testing.T) {
	_, _, irRoutes := routes(testDeps())

	seen := make(map[string]bool, len(paymentRateWriteRoutes))
	for _, route := range irRoutes {
		if !paymentRateWriteRoutes[route.Pattern] {
			continue
		}
		seen[route.Pattern] = true
		if len(route.Roles) == 0 {
			t.Errorf("payment/rate write %q carries no role declaration -- #990's own gap, reopened", route.Pattern)
		}
	}
	for pattern := range paymentRateWriteRoutes {
		if !seen[pattern] {
			t.Errorf("payment/rate write %q not found in the registry -- did it move, or stop being mounted through ExemptGated/ReplayableGated?", pattern)
		}
	}
}
