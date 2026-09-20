package oncall

import (
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
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
	MsgNoWindow         = "This birth has no on-call window yet, so there is no time to record a gap in. Add a due date first."
	MsgNotOnCall        = "Choose a doula who is on this birth."
	MsgStartsAtRequired = "Enter when the gap starts."
	MsgEndsAtRequired   = "Enter when the gap ends."
	MsgEndsBeforeStarts = "The gap must end after it starts."
	MsgOutsideWindow    = "The gap must fall inside this birth's on-call window."
	MsgReasonTooLong    = "Keep the reason to 500 characters or fewer."
	MsgCoverIsTheSame   = "Choose someone other than the doula who cannot be reached."
	MsgOnlyYourOwnGap   = "You can record a gap only for yourself. An owner or an admin can record one for a colleague."
	MsgGapNotFound      = "This coverage gap was not found. It may already have been cleared."
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
		apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, MsgNoWindow, nil)
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
