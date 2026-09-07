package visit

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// NotesRequest carries a Visit's new notes text. Notes is required and
// non-nil -- unlike ScheduleRequest.ScheduledAt, a nil Notes here has no
// meaning to fall back to (an absent scheduledAt means "leave it
// unscheduled"; an absent notes has no equivalent "leave it unwritten",
// since this route exists only to write it). A caller who wants to clear
// notes back to nothing sends the empty string, not null -- the column
// only ever travels NULL (never written) -> a string (written, possibly
// empty) in one direction; there is no write path back to NULL.
type NotesRequest struct {
	Notes *string `json:"notes"`
}

// NotesResponse confirms a Visit's notes after the write.
type NotesResponse struct {
	VisitID string `json:"visitId"`
	Notes   string `json:"notes"`
}

// NotesHandler writes a Visit's free-text notes (#251), on the rule
// ADR-0006/ADR-0008's Visits row already states: any Staff member who may
// read a Visit may also write its notes. ScheduleHandler now shares that
// rule (#268); only CreateHandler and ReassignHandler ask anything more,
// and what they ask is about whose name goes on the Visit rather than
// about who is writing (see assignee in roles.go).
//
// The narrowing this rule still needs -- a contractor Doula reaches only
// an Engagement she holds a granted attachment on -- comes from
// staffauth.AttachingWrite (mounted in mount.go with attaching=true), the
// exact seam ADR-0008's write table already reuses for this Engagement.
// Must be mounted behind staffauth.Middleware.
func NotesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		visitID := r.PathValue("visitId")
		if !staffauth.ParseUUID(w, "visit", visitID) {
			return
		}
		if err := requireEngagementAtPractice(r.Context(), tx, engagementID, practiceID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.WriteError(w, "engagement not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var req NotesRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if req.Notes == nil {
			apierr.WriteError(w, "notes is required", http.StatusBadRequest)
			return
		}

		// Read before writing, inside this same transaction, the same
		// shape ScheduleHandler's own comment gives -- and for the same
		// reason: a first write, an edit and a clear all edit the
		// identical column, and a Diff that reads the same for all three
		// answers nothing. previous.Valid is this route's own 404 (a
		// missing Visit scans no row at all), so there is no separate
		// RowsAffected check.
		var previous sql.NullString
		if err := tx.QueryRowContext(r.Context(),
			`SELECT notes FROM visits WHERE id = $1 AND engagement_id = $2`,
			visitID, engagementID,
		).Scan(&previous); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.WriteError(w, "visit not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE visits SET notes = $1 WHERE id = $2 AND engagement_id = $3`,
			*req.Notes, visitID, engagementID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// The notes text itself never rides the Diff -- CONTEXT.md's
		// Erasure entry already names a Visit's notes among the hand-typed
		// free text that is not redacted, and plans.PutInstanceHandler
		// sets the precedent for a staff-only free-text field's own edit
		// action: its Diff carries {"planType": planType}, not the
		// answers. Putting the content here too would give it a second,
		// unsealed home in a table Erasure's own redaction never reaches
		// (ADR-0027) -- worse for a bereavement Visit's notes than for
		// nowhere at all. hadNotes/hasNotes still make a first write, an
		// edit and a clear three distinguishable rows without the text.
		diff, err := json.Marshal(map[string]any{
			"visitId":  visitID,
			"hadNotes": previous.Valid,
			"hasNotes": *req.Notes != "",
		})
		if err != nil {
			// coverage:ignore reason: a map of a string, a bool and a bool always marshals cleanly, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionVisitNotesEdited),
			Diff:        diff,
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, NotesResponse{VisitID: visitID, Notes: *req.Notes})
	})
}
