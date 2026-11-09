package clientauth

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/dbtx"
)

// EndAllSessionsHandler lets a Client sign herself out of every device
// from inside the portal (#618, ADR-0026) -- distinct from ordinary
// sign-out (session.EndHandler, DELETE /api/session), which ends only
// the browser making the request. It reuses authn.EndAllSessions, the
// same delete-by-identity_uid query staffauth.EndSessionsHandler already
// drives for an Owner revoking a Staff member's sessions; this is that
// query's self-service counterpart for the Client population -- ADR-0026
// calls both "nearly free" once the query already existed.
//
// Authenticated the same way RequestAddressChangeHandler is: a live
// portal session, checked against portal_accounts rather than run
// through clientauth.Middleware, because this acts on the whole Portal
// Account (ADR-0015: "the person lives in the login"), not one
// Engagement.
func EndAllSessionsHandler(db *sql.DB) http.Handler {
	return portalAccountHandler(db, func(w http.ResponseWriter, r *http.Request, tx *sql.Tx, uid string) {
		if err := authn.EndAllSessions(r.Context(), tx, uid); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// client_portal_users' self-visibility policy, which
		// listPortalClientIDs (inside recordForEachClient) reads through,
		// matches on app.current_identity_uid -- the same set_config
		// applyAddressChange makes before its own call into this loop.
		if _, err := tx.ExecContext(r.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, uid); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := recordForEachClient(r.Context(), tx, uid, activity.ActionPortalSessionsEnded); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if !dbtx.Commit(w, tx) {
			// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
			return
		}

		// This request's own session is one of the ones EndAllSessions
		// just deleted -- it matches every row for uid, this one
		// included -- so the browser is told to drop it too, the same
		// clearing session.EndHandler sends on ordinary sign-out.
		http.SetCookie(w, authn.ExpiredSessionCookie())
		w.WriteHeader(http.StatusNoContent)
	})
}
