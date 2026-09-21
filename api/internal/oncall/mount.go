package oncall

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// Mount registers the on-call surface (#1093).
//
// The gap writes are attaching (AttachingWrite): that is what refuses a
// contractor an Engagement she is not on with the read gate's own 404,
// before any on-call fact is consulted. Who may name whom is the
// handlers' own self-versus-colleague rule, because it is decided per
// gap rather than per route.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, enq tasknudge.Enqueuer) {
	g.Get("/api/practices/{practiceId}/on-call", staffauth.AnyStaff, RosterHandler())
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/on-call", staffauth.AnyStaff, EngagementHandler())

	g.Get("/api/practices/{practiceId}/on-call-settings", staffauth.OwnerAndAdmin, GetSettingsHandler())
	ir.ExemptGated("PUT /api/practices/{practiceId}/on-call-settings",
		"full-replace UPDATE of the Practice's one on-call rule; re-sending the same body writes and records nothing (see PutSettingsHandler)",
		false, staffauth.OwnerAndAdmin, PutSettingsHandler())
	ir.ExemptGated("PUT /api/practices/{practiceId}/engagements/{engagementId}/on-call-rule",
		"full-replace UPDATE of one Engagement's start-rule override; re-sending the same body writes and records nothing",
		true, staffauth.OwnerAndAdmin, PutRuleHandler())
	ir.ExemptGated("PUT /api/practices/{practiceId}/engagements/{engagementId}/attachments/{staffId}/on-call",
		"full-replace UPDATE of one Doula's narrowing; re-sending the same body writes and records nothing",
		true, staffauth.OwnerAndAdmin, PutNarrowingHandler())

	ir.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/coverage-gaps", true, CreateGapHandler(enq))
	ir.Exempt("PUT /api/practices/{practiceId}/engagements/{engagementId}/coverage-gaps/{gapId}",
		"full replace of one gap's facts; re-sending the same body writes the same row, and at most one Notification is ever pending per gap (00114's partial unique index)",
		true, UpdateGapHandler(enq))
	ir.Exempt("DELETE /api/practices/{practiceId}/engagements/{engagementId}/coverage-gaps/{gapId}",
		"clearing is terminal: a second clear finds no live gap and answers 404 with nothing written",
		true, ClearGapHandler())
}
