package clientfieldtemplate

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers ADR-0017's Client Field Template settings screen
// (#399): the field list an Owner or Admin defines for a Client's
// Practice-defined layer. Read by any Staff member (the definitions
// carry nothing secret), written by an Owner or Admin alone
// (client_field_templates_insert/_update, 00050, enforce the same rule in
// RLS).
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router) {
	g.Get("/api/practices/{practiceId}/client-field-template", staffauth.AnyStaff, GetHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/client-field-template",
		"upsert (ON CONFLICT ... DO UPDATE); replaces the template wholesale, so re-sending the same body is a no-op",
		false, PutHandler())
}
