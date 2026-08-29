// Package website holds the website a Practice declares to Stripe (#440):
// either a URL of her own, or a page Doula Cloud publishes for her at
// doula.cloud/p/<slug>.
//
// Stripe's hosted onboarding demands a website from every connected
// account and #421 walked what happens when it does not get one -- the
// field accepts empty, she completes every remaining step, submits, and
// returns to us "done" with charges_enabled false and nothing on screen
// saying why. A social profile satisfies the field, so "my own website"
// has to accept one; what it must not accept is something malformed,
// which Stripe's own field refuses ("Not a valid URL") and which would
// otherwise be discovered halfway through her onboarding.
//
// This package owns the answer and its audit trail, and stops there.
// Generating the hosted page is #441, gating the Account Link on an
// answer existing is #442, and rebuilding the site when she publishes is
// #443. All three read what is written here.
//
// Errors follow docs/api-design.md section 7's structured shape rather
// than the plain-text http.Error most of the BFF still writes. The
// documented rule is the one to follow on a new pair of endpoints with
// no consumers to break; the split it lands in is #462's to close, and
// the app reads either through apiErrorMessage.
package website

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// The two answers a Practice can give. There is no third stored value:
// a Practice that has not answered has no row (00045), so "undeclared"
// is a shape the API reports and not a mode the database holds.
const (
	ModeOwn        = "own"
	ModeHosted     = "hosted"
	ModeUndeclared = "undeclared"
)

// MaxFactLength is the character budget on each of the two facts a
// Practice supplies. One number rather than two: the screen counts down
// against it, the handler refuses past it, and 00045's CHECK constraints
// make a longer value impossible whatever either does. Counted in runes,
// not bytes, because the budget the screen shows her has to be the one
// the server enforces and a browser counts characters.
const MaxFactLength = 500

// MaxURLLength is the longest declared URL accepted -- the practical
// ceiling every mainstream browser handles, and far longer than any
// social profile.
const MaxURLLength = 2048

// The response bodies for a failure the caller can act on. Named
// constants because the screen renders them verbatim and the tests
// assert them.
const (
	MsgInvalidBody       = "invalid request body"
	MsgInvalidMode       = `mode must be "own" or "hosted"`
	MsgURLRequired       = "Enter the web address of your website or social profile"
	MsgURLMalformed      = "Enter a web address in the correct format, like https://example.com/your-practice"
	MsgDescriptionNeeded = "Enter a description of what your Practice offers"
	MsgPolicyNeeded      = "Enter your cancellation or refund policy"
	MsgTooLong           = "Shorten this to 500 characters or fewer"
	MsgInternalError     = "internal error"
)

// The machine-readable codes this package returns. docs/api-design.md
// section 7 names the first three; INTERNAL_ERROR follows portalinvite,
// the section's first adopter.
const (
	codeInvalidArgument = "INVALID_ARGUMENT"
	codeInternalError   = "INTERNAL_ERROR"
)

// APIError is docs/api-design.md section 7's structured error shape.
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// writeAPIError writes status with a {code, message, details?} JSON body.
func writeAPIError(w http.ResponseWriter, status int, code, message string, details map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	_ = json.NewEncoder(w).Encode(APIError{Code: code, Message: message, Details: details})
}

// Request is the whole body of a website declaration. A PUT rather than
// a POST, and a full replacement rather than a patch: there is one
// answer per Practice and re-sending the same one has to be safe, which
// is what makes it idempotent without an Idempotency-Key
// (docs/api-design.md section 3).
//
// The two hosted facts travel on every request, including a mode "own"
// one, so a Practice who switches to her own site and back does not have
// to write her cancellation policy a second time. Which fields are
// required is decided by mode -- see Validate.
type Request struct {
	Mode               string `json:"mode"`
	OwnURL             string `json:"ownUrl"`
	ServiceDescription string `json:"serviceDescription"`
	CancellationPolicy string `json:"cancellationPolicy"`
}

// Response is what a Practice's website reads as. Never an error for a
// Practice that has not answered: Mode is "undeclared" and the rest is
// empty, because #442's payments screen asks this endpoint whether she
// may be sent to Stripe at all and a 404 there would be a state to
// special-case rather than an answer.
//
// UpdatedBy and UpdatedAt come from the newest audit row rather than
// from the current one, so the screen can print "Published by Maya Chen
// on 28 August 2026" -- the whole answer to "how did this page come to
// say that?" for a Practice with more than one Owner. Both are empty
// when nobody has answered.
type Response struct {
	Mode               string `json:"mode"`
	OwnURL             string `json:"ownUrl"`
	ServiceDescription string `json:"serviceDescription"`
	CancellationPolicy string `json:"cancellationPolicy"`
	// The name of the Staff member who last wrote the answer.
	UpdatedBy string `json:"updatedBy"`
	// RFC 3339, or empty when nobody has answered.
	UpdatedAt string `json:"updatedAt"`
	// PageState is whether the hosted page has been confirmed to
	// resolve on the live site (#443): "pending", "live" or "failed".
	// Empty for a Practice using her own website, who has no page here.
	//
	// "pending" is not a failure and never blocks her -- it is the
	// ordinary couple of minutes between publishing and the deploy
	// finishing. It is also what a page stays at when the build failed
	// and nothing ever reported, which is why the screen says "not
	// confirmed yet" rather than nothing at all: absence of a report is
	// never a pass.
	PageState string `json:"pageState"`
	// PageCheckedAt is when a probe last ran, RFC 3339, or empty when
	// none has.
	PageCheckedAt string `json:"pageCheckedAt"`
	// PageCheckDetail is why the last probe failed, in a few words for
	// her to read. Empty when it did not fail.
	PageCheckDetail string `json:"pageCheckDetail"`
	// PageURL is the public address of the page published for her.
	// Empty for a Practice on her own website. It is the same URL
	// Stripe is handed (ReadStripeProfile), so a screen that shows it to
	// her is showing her what Stripe will look at.
	PageURL string `json:"pageUrl"`
}

// The values PageState takes, matching 00049's practice_page_state.
const (
	PageStatePending = "pending"
	PageStateLive    = "live"
	PageStateFailed  = "failed"
)

// Validated is a Request that has been through Validate: trimmed,
// normalized, and known to satisfy the mode it declares.
type Validated struct {
	Mode               string
	OwnURL             string
	ServiceDescription string
	CancellationPolicy string
}

// Validate trims, normalizes and checks a declaration, returning the
// field-level detail docs/api-design.md section 7 asks for so the screen
// can put each message beside the input it is about rather than in one
// heap at the top.
//
// The rules are per-mode on purpose. A Practice declaring her own site
// is not asked for a service description, and a Practice publishing a
// page here is not asked for a URL -- demanding both would be demanding
// the thing the choice exists to avoid.
func Validate(req Request) (Validated, map[string]string) {
	details := map[string]string{}

	v := Validated{
		Mode:               strings.TrimSpace(req.Mode),
		ServiceDescription: strings.TrimSpace(req.ServiceDescription),
		CancellationPolicy: strings.TrimSpace(req.CancellationPolicy),
	}

	// The budget applies to whatever was sent, in either mode: a mode
	// "own" request still carries the two facts forward, and carrying
	// forward something the database will refuse is worse than refusing
	// it here.
	if utf8.RuneCountInString(v.ServiceDescription) > MaxFactLength {
		details["serviceDescription"] = MsgTooLong
	}
	if utf8.RuneCountInString(v.CancellationPolicy) > MaxFactLength {
		details["cancellationPolicy"] = MsgTooLong
	}

	switch v.Mode {
	case ModeOwn:
		normalized, ok := NormalizeURL(req.OwnURL)
		switch {
		case strings.TrimSpace(req.OwnURL) == "":
			details["ownUrl"] = MsgURLRequired
		case !ok:
			details["ownUrl"] = MsgURLMalformed
		default:
			v.OwnURL = normalized
		}
	case ModeHosted:
		if v.ServiceDescription == "" {
			details["serviceDescription"] = MsgDescriptionNeeded
		}
		if v.CancellationPolicy == "" {
			details["cancellationPolicy"] = MsgPolicyNeeded
		}
		// A URL she declared earlier survives a switch to the hosted
		// page, the same way the two facts survive a switch away from
		// it, so it is normalized rather than dropped when present.
		if strings.TrimSpace(req.OwnURL) != "" {
			if normalized, ok := NormalizeURL(req.OwnURL); ok {
				v.OwnURL = normalized
			}
		}
	default:
		details["mode"] = MsgInvalidMode
	}

	if len(details) > 0 {
		return Validated{}, details
	}
	return v, nil
}

// schemePrefix matches RFC 3986's scheme production at the head of a
// string: a letter followed by letters, digits, "+", "-" or "." and then
// a colon.
var schemePrefix = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:`)

// NormalizeURL trims what she typed, supplies the scheme she almost
// certainly omitted, and reports whether the result is a web address at
// all.
//
// Deliberately not an allowlist. #421 established that a social profile
// satisfies Stripe's website field -- a Facebook page URL cleared the
// requirement on the probe account -- so a rule that only admitted a
// Practice's own domain would refuse the answer most solo doulas have.
// What is checked is only what Stripe's own field checks: that it is a
// well-formed http(s) address with a host that could resolve.
//
// "facebook.com/rochester-doulas" becomes
// "https://facebook.com/rochester-doulas" rather than being refused.
// Someone typing a web address without a scheme has answered the
// question, and supplying https is normalization rather than a guess
// about what she meant.
func NormalizeURL(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > MaxURLLength {
		return "", false
	}
	// A bare host has no scheme for url.Parse to find, so it parses as a
	// path and comes back with an empty Host. Prefixing before parsing
	// rather than after means one parse and one set of rules.
	//
	// The test is for a scheme and not for "://", because "mailto:" and
	// "javascript:" have no slashes: prefixing one of those would turn
	// "mailto:hello@rochesterdoulas.com" into a perfectly well-formed
	// https URL with "mailto" as the username, which is how a scheme
	// nobody can visit would have slipped through.
	if !schemePrefix.MatchString(trimmed) {
		trimmed = "https://" + trimmed
	}
	// url.Parse is lenient -- it accepts a space in the host and calls
	// the whole thing an opaque path -- so parsing is the start of the
	// check and not the end of it.
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	host := parsed.Hostname()
	// A host with no dot in it is a machine name on somebody's local
	// network, not an address a Client or a Stripe reviewer can reach.
	if host == "" || !strings.Contains(host, ".") || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return "", false
	}
	// Measured on the normalized form, not on what she typed: the https
	// prefix this function supplies is what can push an otherwise
	// in-budget address past the ceiling. Whitespace needs no check of
	// its own -- url.Parse refuses it in a host and percent-escapes it
	// everywhere else, so none of it survives to be stored.
	if utf8.RuneCountInString(parsed.String()) > MaxURLLength {
		return "", false
	}
	return parsed.String(), true
}

// FormatUpdatedAt renders an audit timestamp for Response.UpdatedAt, or
// the empty string when there is nothing to render -- a Practice that
// has never answered has no event row and no date to print.
func FormatUpdatedAt(t time.Time, valid bool) string {
	if !valid {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// MaxSlugLength caps the path segment of doula.cloud/p/<slug>. Sixty
// characters is longer than any Practice name anyone has typed and short
// enough that the URL a Client reads off a card statement or an invoice
// stays a URL.
const MaxSlugLength = 60

// slugSeparators matches every run of characters that is not an
// unreserved ASCII alphanumeric. Everything in a run collapses to one
// hyphen, so "Rochester  Doulas, LLC" and "Rochester-Doulas-LLC" reach
// the same shape.
var slugSeparators = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns a Practice name into the path segment its page is
// published under, falling back to the Practice id when the name leaves
// nothing usable behind.
//
// Called once, at a Practice's first publish, and the result is stored
// (00046). It is deliberately not called again: practices.name is an
// Owner's to edit, Stripe holds the declared URL for the life of the
// connected account, and #382 established that Stripe's review of that
// URL is ongoing -- so a slug recomputed on a rename would point a live
// review at a 404.
//
// Non-ASCII characters collapse to a separator rather than being
// transliterated. A name written in another script therefore falls
// through to the id-derived form, which is ugly and stable, rather than
// to a guess at what its letters sound like in English.
func Slugify(name, practiceID string) string {
	slug := slugSeparators.ReplaceAllString(strings.ToLower(name), "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > MaxSlugLength {
		// Cut on a byte boundary and then re-trim: every character that
		// survives slugSeparators is one ASCII byte, so the cut can only
		// land mid-word, never mid-rune.
		slug = strings.Trim(slug[:MaxSlugLength], "-")
	}
	if slug == "" {
		// A uuid's first block is eight hex characters -- enough to tell
		// two unnamed Practices apart, and the unique index is what
		// actually guarantees it.
		id := strings.ReplaceAll(practiceID, "-", "")
		if len(id) > 8 {
			id = id[:8]
		}
		return "practice-" + id
	}
	return slug
}

// SlugCandidate returns the nth slug to try for a name, counting from
// zero. The first attempt is the plain slug; a collision with another
// Practice's page moves to "-2", then "-3", and so on.
//
// A suffix rather than a random string, because the slug is a public URL
// a Practice reads and repeats. "rochester-doulas-2" is a Practice whose
// name someone else took first; "rochester-doulas-x7f2" is a support
// question.
func SlugCandidate(name, practiceID string, attempt int) string {
	base := Slugify(name, practiceID)
	if attempt == 0 {
		return base
	}
	suffix := "-" + strconv.Itoa(attempt+1)
	// The suffix has to fit inside the same ceiling the column holds, so
	// the base gives way to it rather than the other way round.
	if len(base)+len(suffix) > MaxSlugLength {
		base = strings.Trim(base[:MaxSlugLength-len(suffix)], "-")
	}
	return base + suffix
}
