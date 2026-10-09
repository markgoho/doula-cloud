package practicename

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the one write a Practice's name has after signup
// (#1540). Owner only, declared here rather than checked in the handler
// (#990's form): the name is what a Client, a Contract and the Practice
// Page call the business, so it is the founder's to state, a notch
// narrower than the timezone and the rate card. There is no GET: the
// name already rides on the Practice session every screen reads.
func Mount(ir *idempotency.Router) {
	ir.ExemptGated("PUT /api/practices/{practiceId}/name",
		"full-replace UPDATE of the Practice's one name; re-sending the same body is a no-op that records nothing new (see PutHandler's own doc comment)",
		false, staffauth.OwnerOnly, PutHandler())
}
