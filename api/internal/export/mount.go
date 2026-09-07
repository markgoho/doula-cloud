// Package export is #288's whole-Practice export: one ZIP archive of
// UTF-8 CSVs, one file per domain entity the Practice owns, so an Owner
// who asks "can I get out again?" gets back an answer a spreadsheet can
// open and a machine can read without Doula Cloud.
//
// Owner-only, the same seat as erasure (ADR-0027) and the MFA switch --
// a whole-Practice export is a bulk read across every attachment and
// role boundary ADR-0008's read table draws, so it sits with the acts
// that cross a Practice's own internal walls rather than the acts any
// Staff member may run.
//
// Every entity is one row in entities(): a filename, a header, and a
// query that casts every selected column to text so one generic
// streamer (writeEntity) can copy any of them into a CSV without
// knowing what any single column means. Adding an entity later is
// adding one row to that slice and one test, not editing a monolith.
package export

import (
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the one export route. GET, because an export reads a
// Practice and changes nothing about it -- docs/api-design.md's own
// rule for when a bulk read may skip idempotency.Router entirely.
func Mount(g *staffauth.GatedRouter) {
	g.Get("/api/practices/{practiceId}/export", staffauth.OwnerOnly, Handler())
}
