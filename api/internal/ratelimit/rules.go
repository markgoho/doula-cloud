package ratelimit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
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

// SessionCookieRule limits by a SHA-256 digest of the caller's
// __session cookie -- for a route Wrap fronts that runs behind
// authn.Begin rather than authn.BeginBootstrap (#613's signed-in
// "request a fresh verification link"), where there is no Bearer token
// to key BearerTokenRule on. Skipped when no session cookie is present,
// the same shape BearerTokenRule uses: the handler behind Wrap rejects
// that request on its own account.
func SessionCookieRule(maxRequests int, window time.Duration) Rule {
	return Rule{
		Dimension: "session",
		Key: func(r *http.Request) (string, bool) {
			cookie, err := r.Cookie(authn.SessionCookieName)
			if err != nil {
				return "", false
			}
			sum := sha256.Sum256([]byte(cookie.Value))
			return hex.EncodeToString(sum[:]), true
		},
		Max:    maxRequests,
		Window: window,
	}
}

// JSONFieldRule limits by the string value of field in the request's
// JSON body -- #613's password-reset request, which carries no Bearer
// token, session, or path parameter to key on before its own lookup
// runs, only the address the caller typed. Mirrors the plan
// docs/api-design.md already records for #166's still-unbuilt magic-link
// request: key the request's own email address field directly, since
// unlike a Bearer token or a reset token an email address is not itself
// a usable credential. Reads and restores r.Body, so the handler behind
// Wrap still sees the full request. Skipped when the body is not JSON,
// or field is missing, empty, or not a string.
func JSONFieldRule(field string, maxRequests int, window time.Duration) Rule {
	return Rule{
		Dimension: field,
		Key:       jsonFieldKey(field, false),
		Max:       maxRequests,
		Window:    window,
	}
}

// HashedJSONFieldRule is JSONFieldRule for a field that is itself a
// usable credential -- #613's verification and reset tokens, posted in
// the body rather than a header or path parameter. The digest, not the
// raw value, is what a refusal logs, the same reason BearerTokenRule
// hashes a Bearer token instead of storing it.
func HashedJSONFieldRule(field string, maxRequests int, window time.Duration) Rule {
	return Rule{
		Dimension: field,
		Key:       jsonFieldKey(field, true),
		Max:       maxRequests,
		Window:    window,
	}
}

// jsonFieldKey builds the Key function JSONFieldRule and
// HashedJSONFieldRule share: read the body, restore it for the handler
// behind Wrap, and extract field as a normalized (trimmed, lower-cased)
// string -- optionally digesting it before it becomes a bucket key or a
// logged refusal.
func jsonFieldKey(field string, hash bool) func(*http.Request) (string, bool) {
	return func(r *http.Request) (string, bool) {
		if r.Body == nil {
			return "", false
		}
		body, err := io.ReadAll(r.Body)
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))
		if err != nil {
			// coverage:ignore reason: only fails on a body already consumed or closed, unreachable this early in the request lifecycle
			return "", false
		}

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", false
		}
		raw, ok := payload[field].(string)
		value := strings.ToLower(strings.TrimSpace(raw))
		if !ok || value == "" {
			return "", false
		}

		if !hash {
			return value, true
		}
		sum := sha256.Sum256([]byte(value))
		return hex.EncodeToString(sum[:]), true
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
