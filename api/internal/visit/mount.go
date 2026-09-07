package visit

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the Visit list and its four writes. None of the four
// declares a role of its own beyond attaching=true: reaching the
// Engagement is AttachingWrite's CanAccessEngagement check, and who a
// Visit may be assigned to is decided per act inside the handler (#268,
// see assignee in roles.go) rather than per endpoint. All four carry
// attaching=true: staffauth.AttachingWrite is ADR-0008's write-side seam,
// attaching the acting Doula to the Engagement once the write succeeds.
// CreateHandler was newly wrapped Replayable in the 2026 idempotency-stance
// review -- an unconditional INSERT with a fresh id and no uniqueness
// guard, so a double-click logged a Visit twice. ScheduleHandler (#250)
// and NotesHandler (#251) join ReassignHandler as Exempt for the same
// reason: each is a plain "set this field to the given value" UPDATE, so
// re-sending an identical body is already a no-op.
//
// NotesHandler and ScheduleHandler both edit a field on a Visit that
// already exists, and neither asserts a role: the only gate on either is
// AttachingWrite's own CanAccessEngagement check, the same one ADR-0008's
// read table already applies. See notes.go's own doc comment for the
// argument, and schedule.go's for why #268 brought Schedule in line with
// it -- gating a reschedule on the Doula role refused the Admin whose job
// scheduling is.
//
// CreateHandler's own Replayable classification re-checked for #250's
// scheduledAt field and again for #268's staffId: idempotency.Wrap keys
// purely on the Idempotency-Key header plus practiceID/staffID
// (idempotency.go's lookup/save), and replays whatever status/body the
// first call produced without ever re-reading the second call's body.
// Adding a field to CreateRequest changes nothing that decision depends
// on -- and because the stored body is replayed verbatim, a retry of a
// create that named a colleague answers with that same colleague rather
// than re-deciding who the Visit is for.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router) {
	// The Practice-wide schedule (#263): every scheduled Visit at the
	// Practice in one read, soonest first, rather than one Engagement page
	// per Client. AnyStaff, not OwnerAndAdmin -- unlike the money and
	// Contract roll-ups, ADR-0008's read table does have a row for this
	// noun, and the handler applies its contractor half itself as a row
	// filter rather than as a refusal. See PracticeScheduleHandler's own
	// doc comment.
	g.Get("/api/practices/{practiceId}/visits", staffauth.AnyStaff, PracticeScheduleHandler())
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/visits", staffauth.AnyStaff, ListHandler())
	ir.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/visits", true, CreateHandler())
	ir.Exempt("PATCH /api/practices/{practiceId}/engagements/{engagementId}/visits/{visitId}",
		"plain UPDATE staff_id = $1 WHERE id = $2; sets the assignment to the given value, so re-sending the same body is a no-op",
		true, ReassignHandler())
	ir.Exempt("PATCH /api/practices/{practiceId}/engagements/{engagementId}/visits/{visitId}/schedule",
		"plain UPDATE scheduled_at = $1 WHERE id = $2; sets the scheduled instant to the given value (or clears it), so re-sending the same body is a no-op",
		true, ScheduleHandler())
	ir.Exempt("PATCH /api/practices/{practiceId}/engagements/{engagementId}/visits/{visitId}/notes",
		"plain UPDATE notes = $1 WHERE id = $2; sets the notes to the given value (or clears them to empty), so re-sending the same body is a no-op",
		true, NotesHandler())
}
