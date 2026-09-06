// Cross-population session eviction (#610, decided on #168).
//
// A browser can hold exactly one Doula Cloud session: Firebase Hosting
// strips every cookie on the /api/** hop but __session (#139), so a
// second one cannot be issued. Signing into the second population
// therefore overwrites the first, and until #610 nothing in the code
// knew that -- a doula who is also a Client was signed out of her
// Practice without being told, on a laptop that may be shared by design.
//
// The eviction is not prevented, it is disclosed: every mint seam asks
// EvictionFor first, refuses once with RefuseUnconfirmed so the page can
// say what continuing costs, and mints on the retry that carries
// staffauth's own X-Confirmed header. The refusal says only that a
// session exists -- never whose, never which Practice or address. She
// proved she holds the cookie by presenting it, so naming anything
// behind it adds nothing she does not already have, and turns the
// endpoint into an oracle if the cookie were ever someone else's.

package authn

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/portalaccount"
)

// Tier names the population an identity_uid was issued in.
type Tier string

// The two populations ADR-0026 describes, and the only two there are:
// every identity_uid this product ever sees is minted by exactly one of
// two issuers.
const (
	// TierStaff is an Identity Platform uid -- a Staff member.
	TierStaff Tier = "staff"
	// TierPortal is a Doula Cloud-issued Portal Account identifier -- a
	// Client.
	TierPortal Tier = "portal"
)

// TierOf reports which population identityUID belongs to, read from its
// namespace prefix and nothing else.
//
// ADR-0026 ("The Portal Account becomes a table, and the prefix is the
// namespace") sanctions this rather than a tier column on `sessions`:
// the prefix is not a hint about the identifier, it is the namespace it
// was issued in, so the test is total and deterministic and a column
// would be a second place holding the same fact, free to disagree with
// the first. See portalaccount.Prefix for why no Identity Platform uid
// can ever carry it.
func TierOf(identityUID string) Tier {
	if strings.HasPrefix(identityUID, portalaccount.Prefix) {
		return TierPortal
	}
	return TierStaff
}

// Eviction is the live session in the other population that minting
// would replace: the token whose row has to be deleted, and the identity
// that will need telling.
type Eviction struct {
	// Token is the __session cookie value the browser presented -- the
	// argument EndSession takes.
	Token string
	// IdentityUID is who holds the session being evicted.
	IdentityUID string
	// Tier is the population IdentityUID belongs to, which is by
	// construction not the tier being minted.
	Tier Tier
}

// EvictionFor reports the live session r carries when it belongs to the
// population other than minting. found is false when there is nothing to
// evict: no cookie, a cookie naming no live session, or a session in the
// same population as the one being minted.
//
// A same-population re-sign-in is deliberately not an eviction. Signing
// in again as yourself replaces your own session with your own session,
// which is what a re-sign-in has always meant and carries nothing to
// warn about.
func EvictionFor(ctx context.Context, q Querier, r *http.Request, minting Tier, now time.Time) (ev Eviction, found bool, err error) {
	// http.ErrNoCookie is the only error r.Cookie returns, and it is an
	// ordinary outcome here rather than a failure to report: a caller
	// signing in with no session holds nothing to evict.
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		//nolint:nilerr // http.ErrNoCookie is "nothing to evict", not a failure
		return Eviction{}, false, nil
	}

	// A cookie naming no live session says the same thing: a stale
	// cookie evicts nothing, exactly as no cookie does, so errNoSession
	// becomes found=false rather than an error.
	uid, _, _, err := lookupSession(ctx, q, cookie.Value, now)
	if err != nil {
		if errors.Is(err, errNoSession) {
			return Eviction{}, false, nil
		}
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return Eviction{}, false, err
	}

	tier := TierOf(uid)
	if tier == minting {
		return Eviction{}, false, nil
	}
	return Eviction{Token: cookie.Value, IdentityUID: uid, Tier: tier}, true, nil
}

// EvictionWarning is what a caller is told about an eviction it has not
// confirmed. It names the population being left, because that is the
// whole point of telling her, and nothing else about the session -- see
// this file's own header for why.
func EvictionWarning(tier Tier) string {
	if tier == TierStaff {
		return "Continuing signs you out of your Practice in this browser."
	}
	return "Continuing signs you out of the client portal in this browser."
}

// EvictionUnconfirmed is the code an unconfirmed eviction is refused
// with, so a caller tells this refusal apart from every other one this
// endpoint writes by its code rather than by matching English prose
// (#692's rule). 409 rather than 400: nothing about the request is
// malformed, a condition outside it is unmet.
const EvictionUnconfirmed = apierr.CodeFailedPrecondition

// RefuseUnconfirmed writes the 409 that asks for a deliberate press and
// returns false, unless found is false or the caller already pressed it.
// The signal is staffauth.RequireConfirmed's own X-Confirmed
// header, read here rather than through that function because the
// confirmation is conditional -- most sign-ins evict nothing and must
// not be asked -- and because authn cannot import staffauth, which
// already imports authn.
//
// Honest about what a server can verify, the same way RequireConfirmed
// is: the header proves the client attempted to signal confirmation, not
// that a person meant it. What makes it a real disclosure is that the
// page does not send it until she has read the warning and pressed
// through.
func RefuseUnconfirmed(w http.ResponseWriter, r *http.Request, ev Eviction, found bool) (ok bool) {
	if !found || r.Header.Get("X-Confirmed") == "true" {
		return true
	}
	apierr.Write(w, http.StatusConflict, EvictionUnconfirmed, EvictionWarning(ev.Tier), nil)
	return false
}
