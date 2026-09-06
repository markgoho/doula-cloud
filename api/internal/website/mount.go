package website

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// Mount registers the website a Practice declares to Stripe (#440). Read
// by every Staff member, because the payments screen has to tell a Doula
// who opens it what is outstanding rather than show her an empty panel,
// and nothing here is secret -- the whole point of the answer is that it
// is published. Written by an Owner alone (PutHandler).
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, nudge tasknudge.Enqueuer) {
	g.Get("/api/practices/{practiceId}/website", staffauth.AnyStaff, GetHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/website",
		"one declaration per Practice, replaced whole (PUT semantics) -- the handler's own doc comment already says re-sending the same body is safe -- and the rebuild nudge only fires when the page becomes newly stale",
		false, PutHandler(nudge))
}
