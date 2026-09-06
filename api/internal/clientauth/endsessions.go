package clientauth

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, _, ok := authn.Begin(w, r, db)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		// A __session cookie is one cookie for two populations
		// (ADR-0026), so a signed-in Staff member's session reaches this
		// route with a perfectly valid uid that names no Portal Account --
		// see RequestAddressChangeHandler's own doc comment for why this
		// check exists rather than letting EndAllSessions run against
		// whatever uid the cookie names.
		holds, err := isPortalAccount(r.Context(), tx, uid)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !holds {
			apierr.WriteError(w, msgNotAPortalAccount, http.StatusForbidden)
			return
		}

		if err := authn.EndAllSessions(r.Context(), tx, uid); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		// client_portal_users' self-visibility policy, which
		// listPortalClientIDs (inside recordForEachClient) reads through,
		// matches on app.current_identity_uid -- the same set_config
		// applyAddressChange makes before its own call into this loop.
		if _, err := tx.ExecContext(r.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, uid); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := recordForEachClient(r.Context(), tx, uid, activity.ActionPortalSessionsEnded); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		// This request's own session is one of the ones EndAllSessions
		// just deleted -- it matches every row for uid, this one
		// included -- so the browser is told to drop it too, the same
		// clearing session.EndHandler sends on ordinary sign-out.
		http.SetCookie(w, &http.Cookie{
			Name:     authn.SessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})
		w.WriteHeader(http.StatusNoContent)
	})
}
