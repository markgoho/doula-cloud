// Package csrf rejects cross-site state-changing requests via an Origin
// check. SameSite=Lax on the session cookie (see #138) already withholds
// the cookie from a cross-site POST, which covers the classic
// form-submission attack; Lax was chosen over Strict deliberately, so an
// emailed portal-invitation link still works for an already-signed-in
// Client. This check closes the gap that choice leaves relative to
// Strict. No token-based CSRF scheme is introduced.
package csrf

import (
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// stateChanging is the set of HTTP methods this check applies to. GET,
// HEAD, and OPTIONS never change state, so a cross-site GET (e.g. an
// image tag or a top-level navigation) is never rejected here.
var stateChanging = map[string]bool{
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
}

// Wrap rejects a state-changing request whose Origin header is present
// and not one of allowedOrigins. A request with no Origin header is
// allowed through unconditionally -- server-to-server callers (the two
// Stripe webhook routes) send none, and this check exists to stop a
// browser acting cross-site, not to authenticate the caller.
//
// Mounted around the whole mux, not just the authenticated routes: the
// bootstrap endpoints (staff signup, invitation acceptance) are
// state-changing too, and a caller with no Origin header -- webhook or
// bootstrap alike -- passes through this check the same way.
func Wrap(allowedOrigins []string, next http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if stateChanging[r.Method] {
			if origin := r.Header.Get("Origin"); origin != "" && !allowed[origin] {
				apierr.WriteError(w, "cross-site request rejected", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
