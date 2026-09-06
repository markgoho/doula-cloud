package mailsuppress

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers ADR-0029's suppression list, narrowed to the addresses
// this Practice is responsible for (#744). OwnerAndAdmin rather than
// AnyStaff: the list is every Client and Staff address at the Practice
// whose mail is failing, which ADR-0008 keeps in the same hands as the
// roster it is drawn from.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, bounceClearer BounceClearer) {
	g.Get("/api/practices/{practiceId}/email-suppressions", staffauth.OwnerAndAdmin, ListHandler())
	ir.Exempt("POST /api/practices/{practiceId}/email-suppressions/clear",
		"state-guarded UPDATE ... WHERE cleared_at IS NULL AND cause = 'bounce', and Mailgun's own DELETE answers 404 for an address already off its list; a retry after the first commit 404s instead of clearing twice",
		false, ClearHandler(bounceClearer))
}
