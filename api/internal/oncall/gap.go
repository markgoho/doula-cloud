package oncall

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/staffauth"
)

// maxReasonLength is 00114's engagement_coverage_gaps_reason_length.
const maxReasonLength = 500

// The field names a gap refusal is keyed under -- GapRequest's own json
// tags, per docs/api-design.md section 7 rule 4.
const (
	fieldStaffID         = "staffId"
	fieldStartsAt        = "startsAt"
	fieldEndsAt          = "endsAt"
	fieldReason          = "reason"
	fieldCoveringStaffID = "coveringStaffId"
)

// The sentences a gap refusal says, written for the person at the form.
const (
	// The five sentences a gap write meets when the birth has no window
	// to record one in. One per reason DeriveWindow can give, because
	// each is fixed by something different and "add a due date" is wrong
	// advice for four of them -- the same rule the roster follows when
	// it states why a birth has no window rather than guessing a date.
	MsgNoWindowNoDueDate  = "This birth has no on-call window yet, so there is no time to record a gap in. Add a due date first."
	MsgNoWindowPostpartum = "Postpartum care is not on-call work, so there is no on-call window to record a gap in."
	MsgNoWindowNotActive  = "Care has not started on this birth, or it has ended, so there is no on-call window to record a gap in."
	MsgNoWindowNobodyOnIt = "Nobody is on this birth yet, so there is nobody to record a gap for."
	MsgNoWindowEnded      = "The baby arrived before on call would have opened, so there is no window to record a gap in."
	MsgNotOnCall          = "Choose a doula who is on this birth."
	MsgStartsAtRequired   = "Enter when the gap starts."
	MsgEndsAtRequired     = "Enter when the gap ends."
	MsgEndsBeforeStarts   = "The gap must end after it starts."
	MsgOutsideWindow      = "The gap must fall inside this birth's on-call window."
	MsgReasonTooLong      = "Keep the reason to 500 characters or fewer."
	MsgCoverIsTheSame     = "Choose someone other than the doula who cannot be reached."
	MsgOnlyYourOwnGap     = "You can record a gap only for yourself. An owner or an admin can record one for a colleague."
	MsgGapNotFound        = "This coverage gap was not found. It may already have been cleared."
)

// GapRequest is the body of a gap create and a gap edit. An edit is a
// full replace, so both carry every field.
type GapRequest struct {
	StaffID         string     `json:"staffId"`
	StartsAt        *time.Time `json:"startsAt"`
	EndsAt          *time.Time `json:"endsAt"`
	Reason          *string    `json:"reason"`
	CoveringStaffID *string    `json:"coveringStaffId"`
}

// gapWrite is the per-request state every gap write needs, resolved once.
type gapWrite struct {
	practiceID   string
	engagementID string
	actorID      string
	isOwnerAdmin bool
	engagement   engagementOnCall
	zone         *time.Location
}

// beginGapWrite resolves the request's state and the Engagement's live
// window, writing the refusal itself when there is none. Reaching the
// Engagement at all is AttachingWrite's refusal at the mount; this only
// adds the on-call facts.
func beginGapWrite(w http.ResponseWriter, r *http.Request) (gapWrite, bool) {
	tx, practiceID, ok := staffauth.RequireTx(w, r)
	// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
	if !ok {
		return gapWrite{}, false
	}
	engagementID := r.PathValue("engagementId")
	if !staffauth.ParseUUID(w, "engagement", engagementID) {
		return gapWrite{}, false
	}
	reader, _ := staffauth.ReaderFrom(r.Context())
	actorID, _ := staffauth.StaffID(r.Context())

	settings, err := loadPracticeSettings(r.Context(), tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return gapWrite{}, false
	}
	engagements, err := loadEngagements(r.Context(), tx, practiceID, settings, engagementsFilter{engagementID: engagementID})
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return gapWrite{}, false
	}
	if len(engagements) == 0 || engagements[0].window == nil {
		var reason NoWindowReason
		if len(engagements) == 0 {
			// The Engagement is not a live birth with anybody granted on
			// it, so loadEngagements never returned it and cannot say
			// which of the three that is. DeriveWindow can, from the two
			// columns that decide it.
			reason = whyNotOnCall(r.Context(), tx, practiceID, engagementID)
		} else {
			reason = engagements[0].reason
		}
		apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, noWindowRefusal(reason), nil)
		return gapWrite{}, false
	}
	return gapWrite{
		practiceID:   practiceID,
		engagementID: engagementID,
		actorID:      actorID,
		isOwnerAdmin: reader.IsOwnerOrAdmin(),
		engagement:   engagements[0],
		zone:         settings.zone,
	}, true
}

// isAttached reports whether staffID holds an open, granted Attachment
// on the Engagement -- the only people a gap may name, so a gap is never
// a back door to attaching someone.
func (c gapWrite) isAttached(staffID string) bool {
	for _, d := range c.engagement.doulas {
		if d.staffID == staffID {
			return true
		}
	}
	return false
}

// mayWriteFor is the self-versus-colleague rule visit.resolveAssignee
// draws for a Visit: an Owner or an Admin may record a gap for any
// Doula on the birth, and a Doula only for herself.
func (c gapWrite) mayWriteFor(staffID string) bool {
	return c.isOwnerAdmin || staffID == c.actorID
}

// validate checks req against the Engagement's window and Attachments,
// returning the facts to store or writing the refusal itself.
func (c gapWrite) validate(w http.ResponseWriter, req GapRequest) (gapFacts, bool) {
	details := map[string]string{}
	if !c.isAttached(req.StaffID) {
		details[fieldStaffID] = MsgNotOnCall
	}
	if req.StartsAt == nil {
		details[fieldStartsAt] = MsgStartsAtRequired
	}
	if req.EndsAt == nil {
		details[fieldEndsAt] = MsgEndsAtRequired
	}
	if req.StartsAt != nil && req.EndsAt != nil {
		windowFrom, windowTo := c.engagement.window.Bounds(c.zone)
		switch {
		case !req.EndsAt.After(*req.StartsAt):
			details[fieldEndsAt] = MsgEndsBeforeStarts
		case req.StartsAt.Before(windowFrom) || req.EndsAt.After(windowTo):
			details[fieldStartsAt] = MsgOutsideWindow
		}
	}

	var reason *string
	if req.Reason != nil {
		if trimmed := strings.TrimSpace(*req.Reason); trimmed != "" {
			reason = &trimmed
		}
	}
	if reason != nil && len([]rune(*reason)) > maxReasonLength {
		details[fieldReason] = MsgReasonTooLong
	}

	covering := req.CoveringStaffID
	if covering != nil && *covering == "" {
		covering = nil
	}
	switch {
	case covering == nil:
	case *covering == req.StaffID:
		details[fieldCoveringStaffID] = MsgCoverIsTheSame
	case !c.isAttached(*covering):
		details[fieldCoveringStaffID] = MsgNotOnCall
	}

	if len(details) > 0 {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "The coverage gap could not be saved.", details)
		return gapFacts{}, false
	}
	return gapFacts{
		StaffID:         req.StaffID,
		StartsAt:        req.StartsAt.UTC(),
		EndsAt:          req.EndsAt.UTC(),
		CoveringStaffID: covering,
		reason:          reason,
	}, true
}

// whyNotOnCall says why an Engagement loadEngagements did not return has
// no window: it is not a birth, care is not under way, or nobody holds a
// granted Attachment on it. Answered through DeriveWindow rather than
// here, so one function decides it for every caller.
func whyNotOnCall(ctx context.Context, tx *sql.Tx, practiceID, engagementID string) NoWindowReason {
	var kind, status string
	var granted bool
	if err := tx.QueryRowContext(ctx,
		`SELECT e.kind::text, e.status::text,
		        EXISTS (SELECT 1 FROM engagement_attachments ea
		                 WHERE ea.engagement_id = e.id AND ea.origin = 'granted' AND ea.ended_at IS NULL)
		   FROM engagements e WHERE e.id = $1 AND e.practice_id = $2`,
		engagementID, practiceID,
	).Scan(&kind, &status, &granted); err != nil {
		// coverage:ignore reason: AttachingWrite already refused an Engagement this Practice cannot reach, so the row is always here
		return NoWindowNoDueDate
	}
	// The grant day itself does not matter to the three reasons this
	// answers; what matters is whether anybody holds one at all.
	var grantedOn *string
	if granted {
		today := clock.Now(ctx).Format(dateLayout)
		grantedOn = &today
	}
	_, reason := DeriveWindow(WindowInput{Kind: kind, Status: status, FirstGrantedOn: grantedOn})
	return reason
}

// noWindowRefusal is the sentence a person reads for each reason a birth
// has no window to record a gap in. One per reason, because each is
// fixed by something different.
func noWindowRefusal(reason NoWindowReason) string {
	switch reason {
	case NoWindowPostpartum:
		return MsgNoWindowPostpartum
	case NoWindowNotActive:
		return MsgNoWindowNotActive
	case NoWindowNobodyAttached:
		return MsgNoWindowNobodyOnIt
	case NoWindowEndedBeforeStart:
		return MsgNoWindowEnded
	case NoWindowNoDueDate:
		return MsgNoWindowNoDueDate
	default:
		// coverage:ignore reason: DeriveWindow gives no sixth reason, and a window that exists never reaches here -- the fallback exists so a refusal always carries a sentence
		return MsgNoWindowNoDueDate
	}
}
