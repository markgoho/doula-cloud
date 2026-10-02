package staffauth

import (
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/pagecursor"
)

// unopenedFilter is the one value GET /api/staff/feedback's `issue`
// query parameter takes: only the pieces whose issue open has failed.
const unopenedFilter = "unopened"

// ListFeedbackHandler serves the founder's list (#1526): every piece of
// Feedback, newest first, a page at a time. `?issue=unopened` narrows it
// to the pieces whose issue open has dead-lettered or is retrying -- the
// read page asks for that list first and prints it under its own
// heading, which is where a lapsed GITHUB_FEEDBACK_TOKEN shows (#1500).
// One route and one DTO for both lists, so each has its own cursor and
// neither is unbounded.
//
// Must be mounted behind FounderOnly. It writes nothing, so it leaves
// the transaction for FounderOnly to roll back.
func ListFeedbackHandler(repo string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, _, ok := RequireTx(w, r)
		// coverage:ignore reason: FounderOnly always sets a tx before this handler runs
		if !ok {
			return
		}

		query := r.URL.Query()
		filter := query.Get("issue")
		if filter != "" && filter != unopenedFilter {
			apierr.WriteError(w, "invalid issue filter", http.StatusBadRequest)
			return
		}

		var after *pagecursor.Cursor
		if raw := query.Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil || uuid.Validate(c.ID) != nil {
				apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		page, err := feedback.List(r.Context(), tx, repo, filter == unopenedFilter, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, page)
	})
}

// ReadFeedbackHandler serves one piece of Feedback with every stored
// field (#1526), and records the read: one feedback_reads row per open,
// naming the founder and the instant (clock.Now, #773).
//
// A GET that writes, deliberately. The record is of the read itself, so
// it cannot be a second request the page might not make; it is written
// and committed before the piece is sent, and a read that cannot be
// recorded is not served.
//
// An id that names no piece -- never existed, erased, past retention, or
// not an id at all -- is the same 404 FounderOnly gives a non-founder.
//
// Must be mounted behind FounderOnly.
func ReadFeedbackHandler(repo string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, _, ok := RequireTx(w, r)
		// coverage:ignore reason: FounderOnly always sets a tx before this handler runs
		if !ok {
			return
		}
		staffID, _ := StaffID(r.Context())

		id := r.PathValue("feedbackId")
		if uuid.Validate(id) != nil {
			apierr.WriteError(w, MsgFounderRouteNotFound, http.StatusNotFound)
			return
		}

		piece, found, err := feedback.Get(r.Context(), tx, repo, id)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !found {
			apierr.WriteError(w, MsgFounderRouteNotFound, http.StatusNotFound)
			return
		}

		if err := feedback.RecordRead(r.Context(), tx, piece.ID, staffID, clock.Now(r.Context())); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, piece)
	})
}
