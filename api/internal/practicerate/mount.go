package practicerate

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers a Practice's rate card (#966): the GET every Staff
// member reads, and the per-kind PUT only an Owner or Admin may reach.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router) {
	g.Get("/api/practices/{practiceId}/rates", staffauth.AnyStaff, GetRatesHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/rates/{kind}",
		"full-replace UPDATE of one kind's rate; re-sending the same body is a no-op that records nothing new (see PutRateHandler's own doc comment)",
		false, PutRateHandler())
}
