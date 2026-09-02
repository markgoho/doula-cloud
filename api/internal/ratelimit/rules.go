package ratelimit

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/clientip"
)

// IPRule limits by clientip.From(r), Cloud Run's caller address. Applies
// to every request -- there is always an IP -- so it is the dimension
// that still catches an attacker who evades every other rule by varying
// what that rule keys on (a fresh Bearer token per request, a new
// invitation token).
func IPRule(maxRequests int, window time.Duration) Rule {
	return Rule{
		Dimension: "ip",
		Key:       func(r *http.Request) (string, bool) { return clientip.From(r), true },
		Max:       maxRequests,
		Window:    window,
	}
}

// BearerTokenRule limits by a SHA-256 digest of the caller's presented
// Bearer ID token -- signup, login, and both invitation-acceptance
// endpoints all read one via authn.BearerToken before anything else,
// since none of them has a session yet to key on. This bounds how many
// times *one* credential may be replayed against the endpoint; it says
// nothing about an attacker minting many fresh credentials, which is
// IPRule's job. The digest, not the raw token, is what a refusal logs --
// it identifies the credential without the log itself becoming a usable
// bearer token.
//
// The rule is skipped (Key returns ok=false) when no Bearer token is
// present at all, since the handler behind Wrap will reject that request
// on its own account before this dimension has anything to say.
func BearerTokenRule(maxRequests int, window time.Duration) Rule {
	return Rule{
		Dimension: "token",
		Key: func(r *http.Request) (string, bool) {
			token, ok := authn.BearerToken(r)
			if !ok {
				return "", false
			}
			sum := sha256.Sum256([]byte(token))
			return hex.EncodeToString(sum[:]), true
		},
		Max:    maxRequests,
		Window: window,
	}
}

// PathValueRule limits by the named path parameter -- offerId for the
// pre-account Offer routes, which carry no Bearer token and no email to
// key on before their own token+code check runs. Naming the specific
// Offer being probed is this endpoint's natural "subject" dimension,
// alongside IPRule for the IP one.
func PathValueRule(name string, maxRequests int, window time.Duration) Rule {
	return Rule{
		Dimension: name,
		Key: func(r *http.Request) (string, bool) {
			v := r.PathValue(name)
			if v == "" {
				// coverage:ignore reason: unreachable once mounted on a route
				// pattern naming {name} -- ServeMux guarantees a match, not
				// exercised by unit tests
				return "", false
			}
			return v, true
		},
		Max:    maxRequests,
		Window: window,
	}
}
