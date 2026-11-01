package main

import (
	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/sitebuild"
	"doula-cloud/api/internal/staffauth"
)

// The endpoints nobody signs in to reach: ADR-0010's outboxes, #443's
// page verifier and sweeps. Every operator act is in doula-ops.
//
// Cloud Scheduler and Cloud Tasks call these on a cadence, authenticated
// by a caller identity rather than by a session (ADR-0037), so they sit
// outside staffauth.Middleware and GatedRouter entirely -- the same
// position the webhooks hold, and for the same reason.
func registerInternalRoutes(g *staffauth.GatedRouter, d Deps) {
	// Every outbox, from the one list in outboxes.go (ADR-0010), each
	// authenticated by d.InternalAuth rather than a session so it sits
	// outside staffauth.Middleware/GatedRouter like the Stripe webhooks.
	//
	// This used to be thirteen mux.Handle calls whose only differences
	// were the path and the worker, every one of them routed through a
	// pass-through ProcessOutboxHandler in the worker's own package.
	outbox.Register(g, d.DB, d.InternalAuth, outboxRegistrations(d))
	// #443's page verifier, behind the same guard and under
	// /api/internal/site alongside that ticket's rebuild outbox above.
	// Deliberately not an outbox itself and so not in the list: it
	// processes no queue, it probes every published page and records
	// whether it resolved. Two callers, and the same behavior for both --
	// Cloud Scheduler on a cadence, and the last step of the deploy
	// workflow. The cadence is what covers a build that fails and
	// produces no deploy and no callback at all.
	g.Write("POST /api/internal/site/verify-pages", sitebuild.VerifyHandler(d.DB, d.PageVerifier, d.InternalAuth))
}

// legalDocuments is the version history the BFF publishes and mails
// from: the two documents in api/internal/legal, the one place it holds
// them (#1556). Deps.LegalDocuments replaces it on the end-to-end stack
// only (#1833).
var legalDocuments = []legal.Document{legal.Terms, legal.Privacy}
