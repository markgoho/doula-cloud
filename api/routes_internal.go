package main

import (
	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/legalnotice"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/sitebuild"
	"doula-cloud/api/internal/staffauth"
)

// The endpoints nobody signs in to reach: ADR-0010's outboxes, #443's
// page verifier, and the operator endpoints.
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
	// #449's founding grant moved to doula-ops (#2482, ADR-0062), and
	// #605's support-clear with it (#2483), #1394's monthly Credits read
	// (#2476), #420's dormant-Practices list (#2477) and the refund
	// (#2481).
	//
	// #1790's pilot gate, on the same guard: it opens the gate for
	// everyone -- the one deliberate act ADR-0043 asks for -- and records
	// who did it and when. Adding an address to the allowlist moved to
	// doula-ops (#2484).
	g.Write("POST /api/internal/staffauth/pilot-gate/open", staffauth.PilotGateOpenHandler(d.DB, d.InternalAuth))
	// #1548's publish act, on the same guard and in the pilot gate's
	// shape: a new version of the Terms or the Privacy Policy is written
	// into api/internal/legal, and the founder then publishes it by name.
	// A material version queues the email to each Owner, and is refused
	// fewer than 30 days before it takes effect.
	g.Write("POST /api/internal/legal/publish", legalnotice.PublishHandler(d.DB, d.InternalAuth, d.legalHistory(), d.StripeClient))
}

// legalDocuments is the version history the BFF publishes and mails
// from: the two documents in api/internal/legal, the one place it holds
// them (#1556). Deps.LegalDocuments replaces it on the end-to-end stack
// only (#1833).
var legalDocuments = []legal.Document{legal.Terms, legal.Privacy}
