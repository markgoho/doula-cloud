package engagement

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// fieldDueDate is the dueDate json tag, which this endpoint's one field
// refusal keys its Details under.
const fieldDueDate = "dueDate"

// DueDateChangeRequest carries an Engagement's new due date (#2013): a
// calendar day, YYYY-MM-DD, never an instant -- ADR-0036's rule that a
// day is the Practice's day applies to an instant turned into a day, and
// a due date is a day from the start.
type DueDateChangeRequest struct {
	DueDate string `json:"dueDate"`
}

// DueDateChangeResponse confirms the due date the Engagement now holds.
type DueDateChangeResponse struct {
	EngagementID string `json:"engagementId"`
	DueDate      string `json:"dueDate"`
}

// ChangeDueDateHandler sets an Engagement's due date after its
// Engagement Request was approved (#2013). engagementrequest.Approve
// writes it once; a dating scan or a midwife's revised estimate moves it
// in real care, and ADR-0015 standing rule 2 ("an Engagement is editable
// in place") is why that is a change here rather than a new Engagement.
//
// Who may: refuseFactWrite, the gate every other ADR-0015 mutable fact
// uses (ChangeKindHandler, TransitionHandler, RecordBirthOutcomeHandler)
// -- an Owner, an Admin or an employee Doula; a contractor Doula is
// refused outright, and so is a member with none of the three roles.
//
// What it does not refuse: a date in the past (an overdue birth's
// revised estimate is real) and a change after the birth outcome is
// recorded. ADR-0015's freeze names client_id, birth_outcome and
// pregnancy_ended_on, and not the due date; once the pregnancy has ended
// the On-call window ends on pregnancy_ended_on, and the portal already
// hides a due date after a loss (portal.DetailHandler,
// HasLivingOrExpectedBaby), so a later correction changes no Client
// surface that presumes a baby. It sets a date and never clears one:
// intake demands a due date on birth work, and nothing yet asks for a
// way to remove one.
//
// What follows the new date needs no write of its own. The On-call
// window is derived on every read (oncall.DeriveWindow), the portal and
// the Staff detail read the column on every request, and the Offer
// copies the date only when an Offer is sent.
//
// PUT and no Idempotency-Key, the same naturally-idempotent shape
// ChangeKindHandler's own PUT uses: re-sending the date the Engagement
// already holds writes nothing and raises no second Activity row. The
// write is compare-and-set on the date this handler just read (IS NOT
// DISTINCT FROM, because the column is nullable), so a concurrent change
// is never silently overwritten.
//
// The Activity row is due_date_changed, with both sides of the fact
// (dueDateBefore, null for a first date, and dueDateAfter) and the Staff
// member who acted. It is in activity's staffingActions, beside
// kind_changed: the Practice's own record of how a mutable fact came to
// be.
//
// Must be mounted behind staffauth.Middleware.
func ChangeDueDateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		if refuseFactWrite(w, reader, "A contractor Doula cannot change an Engagement's due date") {
			return
		}

		var req DueDateChangeRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		day, err := time.Parse(time.DateOnly, strings.TrimSpace(req.DueDate))
		if err != nil {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument,
				"The due date must be a calendar day, YYYY-MM-DD",
				map[string]string{fieldDueDate: MsgDueDateMalformed})
			return
		}
		target := day.Format(time.DateOnly)

		var current *string
		err = tx.QueryRowContext(r.Context(),
			`SELECT due_date::text FROM engagements WHERE id = $1 AND practice_id = $2`,
			engagementID, practiceID,
		).Scan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "Engagement not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if current != nil && *current == target {
			apierr.WriteJSON(w, http.StatusOK, DueDateChangeResponse{EngagementID: engagementID, DueDate: target})
			return
		}

		result, err := tx.ExecContext(r.Context(),
			`UPDATE engagements SET due_date = $1::date
			  WHERE id = $2 AND practice_id = $3 AND due_date IS NOT DISTINCT FROM $4::date`,
			target, engagementID, practiceID, current,
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			// coverage:ignore reason: driver RowsAffected failure or a concurrent writer already changed this Engagement's due date, neither exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		actorStaffID, _ := staffauth.StaffID(r.Context())
		if err := recordFact(r.Context(), tx, practiceID, engagementID, actorStaffID, activity.ActionDueDateChanged,
			map[string]any{"dueDateBefore": current, "dueDateAfter": target}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, DueDateChangeResponse{EngagementID: engagementID, DueDate: target})
	})
}
