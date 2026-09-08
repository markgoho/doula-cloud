package staffauth

import (
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// PracticeSessionResponse confirms to the frontend which Practice the
// caller landed on -- and, as a side effect of running through
// Middleware, records it as the Staff member's last-used Practice for
// their next login.
//
// IsContractor carries ADR-0008's employment-type axis alongside Roles.
// It exists because #501's contractor Add-a-Client door needs to branch
// on employment type before ever calling client.SearchHandler -- the same
// UX-only mirror of a BFF role gate this endpoint's Roles field already
// is for the Owner/Admin screens that read it.
type PracticeSessionResponse struct {
	PracticeID string `json:"practiceId"`
	// StaffID is the caller's own Staff id at this Practice (#909).
	// `/api/staff/session` already hands the app the same fact -- #437
	// widened SessionInfo with it on the argument that a second round
	// trip to learn what the first could have carried is a round trip
	// nobody needs -- so this is not a new disclosure. It is that same
	// fact carried here, on the one response every route under this
	// Practice already resolves in a `load` before first paint, so a
	// route reads it without a second call. The screen that needs it is
	// the Add-a-Visit picker, which has to know which roster entry is
	// the caller: the same UX-only mirror of a BFF rule Roles and
	// IsContractor already are, the rule here being visit.assignee's --
	// an absent assignee means the caller.
	StaffID      string   `json:"staffId"`
	PracticeName string   `json:"practiceName"`
	Roles        []string `json:"roles"`
	IsContractor bool     `json:"isContractor"`
	// PendingDeletion is #871's own addition: Middleware still lets this
	// one route through while the Practice is mid-deletion (see
	// isPracticeSessionRoute), so app/'s +layout.ts load needs this flag
	// on the response itself to route an Owner to the restore screen and
	// everyone else to an accurate locked message, rather than every
	// other route under this Practice quietly refusing one at a time.
	PendingDeletion bool `json:"pendingDeletion"`
}

// PracticeSessionHandler answers GET .../session: which Practice, which
// roles, which employment type. Moved from routes_practice.go by #836 --
// the handler already used nothing but this package's own exports
// (Tx, PracticeID, ReaderFrom), so it belongs here rather than in main.
func PracticeSessionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, _ := Tx(r.Context())
		practiceID, _ := PracticeID(r.Context())

		var name string
		if err := tx.QueryRowContext(r.Context(), `SELECT name FROM practices WHERE id = $1`, practiceID).Scan(&name); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		reader, has := ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		staffID, _ := StaffID(r.Context())

		resp := PracticeSessionResponse{
			PracticeID:      practiceID,
			StaffID:         staffID,
			PracticeName:    name,
			Roles:           reader.Roles(),
			IsContractor:    reader.IsContractor(),
			PendingDeletion: pendingDeletionFrom(r.Context()),
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}
