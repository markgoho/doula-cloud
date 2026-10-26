package payments

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/staffauth"
)

// actionConnectFormReturned records an Owner coming back from Stripe's
// hosted onboarding form to the Payments settings screen (#1812) --
// Practice-scoped and plain-string, the same shape
// actionConnectNudgeRequested uses. No diff: the row says who came back
// and when, and nothing else.
//
// Stripe sends her to return_url when she leaves the form, not only when
// she finishes it, so one of these rows does not prove she finished. It
// is the moment closest to what she lived through, and #1591 reads it
// beside stripe_connect_status_history's first row with nothing
// outstanding, which is the moment the data can prove.
const actionConnectFormReturned = string(activity.ActionStripeConnectFormReturned)

// MsgConnectReturnNoAccount is the refusal for a return with no Stripe
// account behind it. Only a hand-typed address reaches it: Stripe sends
// an Owner back only from a form PostConnectHandler opened, and that
// handler stores the account first.
const MsgConnectReturnNoAccount = "You have not started connecting Stripe yet."

// PostConnectReturnHandler records that the signed-in Owner came back from
// Stripe's form (`?connect=return`). The browser is the only place that
// sees the landing, so the Payments settings screen calls this once on
// arrival. Each call records one landing; a reload records another, and
// the reader picks the one it needs by time (#1591's query takes the
// first landing after Stripe last had something outstanding).
//
// Owner-only is declared at the mount, for the reason PostConnectHandler
// gives: only an Owner can open the form, so only an Owner comes back
// from it. Must be mounted behind staffauth.Middleware.
func PostConnectReturnHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		var accountID sql.NullString
		if err := tx.QueryRowContext(r.Context(),
			`SELECT stripe_connect_account_id FROM practices WHERE id = $1`, practiceID,
		).Scan(&accountID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !accountID.Valid {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, MsgConnectReturnNoAccount, nil)
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectPractice,
			SubjectID:   practiceID,
			Action:      actionConnectFormReturned,
			Actor:       activity.StaffActor(staffID),
			// Evidence of when a person did something, so the server's
			// clock names it (activity.Entry.CreatedAt).
			CreatedAt: clock.Now(r.Context()),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
