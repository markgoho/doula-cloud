// Package apierr is the one place docs/api-design.md section 7's
// structured error shape is written from. #529 replaces six independent
// reinventions of the same {code, message, details} envelope
// (portalinvite, payments, contracts, website, ratelimit, each with its
// own writeAPIError) and several hundred handlers that wrote a bare
// string via http.Error -- every one of those call sites now calls
// Write or WriteError here instead.
package apierr

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Code is a machine-readable APIError.Code value, drawn from the
// enumerated set below rather than invented per call site.
type Code string

// The codes section 7 names, plus CodePayloadTooLarge and
// CodePaymentRequired -- added because a call site already returned 413
// and 402 respectively and neither status fits any of section 7's
// original eight.
const (
	CodeInvalidArgument    Code = "INVALID_ARGUMENT"
	CodeUnauthorized       Code = "UNAUTHORIZED"
	CodeForbidden          Code = "FORBIDDEN"
	CodeNotFound           Code = "NOT_FOUND"
	CodeConflict           Code = "CONFLICT"
	CodeFailedPrecondition Code = "FAILED_PRECONDITION"
	CodeRateLimited        Code = "RATE_LIMITED"
	CodeInternal           Code = "INTERNAL_ERROR"
	CodePayloadTooLarge    Code = "PAYLOAD_TOO_LARGE"
	CodePaymentRequired    Code = "PAYMENT_REQUIRED"
	// CodeSessionEvictionUnconfirmed is #610's press-through: the caller
	// holds a live session in the other population, minting would end it,
	// and the same request repeated with X-Confirmed goes through. Its
	// own code rather than CodeFailedPrecondition, which three unrelated
	// 409s already carry (payments/connect, client/erase): the frontend
	// switches on this one to render a warning with a confirm button
	// instead of an error, and matching a code three refusals share would
	// make any of them look like a press-through.
	CodeSessionEvictionUnconfirmed Code = "SESSION_EVICTION_UNCONFIRMED"
	// CodeMFARequired is #606's Practice-scoped boundary refusal: a live,
	// valid session that may not enter this Practice without a second
	// factor. Its own code, not CodeForbidden, so the app's credentialed
	// fetch can route into enrolment rather than treating this as an
	// ended session (#606's AC: "distinguishable from an ended session
	// ... does not send the browser to the login screen"). #842 moves it
	// here from staffauth's own local APIError copy.
	CodeMFARequired Code = "MFA_REQUIRED"
)

// APIError is docs/api-design.md section 7's structured error shape.
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// MsgInternalError is the body a caller sees for a failure that carries no
// more specific detail -- a DB error, an encoding error, anything the
// caller can't act on. #842 collapses thirteen per-package copies of this
// same literal (staffauth, clientauth, session, contracts, portalinvite,
// sitebuild, outbox, website, sessionmint, idempotency, ratelimit,
// sessionevict, authn) into this one, since every copy already agreed on
// the wording.
const MsgInternalError = "internal error"

// MaxRequestBodyBytes bounds how much of a request body DecodeJSON reads
// before giving up, via http.MaxBytesReader. #842 picks 1 MiB: it's what
// every already-guarded site (payments/billing webhook verification,
// portalinvite's Mailgun webhook) agreed on before this package existed,
// and no plain JSON decode site needs more -- the one body that
// legitimately runs larger, message's attachment upload, is multipart
// and bypasses DecodeJSON entirely (see its own package for why).
const MaxRequestBodyBytes = 1 << 20 // 1 MiB

// WriteJSON sends status with body v as JSON, the one success-body writer
// for every handler in api/internal -- the counterpart Write is for a
// refusal. #842 replaces four packages' own writeJSON (engagementrequest,
// offer, payments/invoice, website), each identical but for whether it
// also tried, uselessly, to answer an encode failure with a second
// response after the header was already sent.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	_ = json.NewEncoder(w).Encode(v)
}

// Write sends status with body {code, message, details} as JSON. details
// is nil for the large majority of call sites, which have nothing
// field-specific to say; #529 does not build new plumbing to populate it
// everywhere, only where a caller already has field-level information on
// hand.
func Write(w http.ResponseWriter, status int, code Code, message string, details map[string]string) {
	WriteJSON(w, status, APIError{Code: string(code), Message: message, Details: details})
}

// DecodeJSON decodes r.Body into v, first wrapping it in
// http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes) so an oversized
// body can't be read at all. A body over the cap gets its own refusal
// (413, CodePayloadTooLarge) rather than being folded into the generic
// 400 -- message/create.go's multipart path already drew this line for
// the same *http.MaxBytesError, and the app can only tell "too much" from
// "garbage" if the two arrive under different codes. Any other decode
// failure (malformed JSON, wrong shape) writes the section 7 refusal
// (400, "invalid request body" -- the one message every site that
// hand-wrote this already agreed on). Either way it returns false; the
// caller's only remaining job is to return when DecodeJSON does.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			Write(w, http.StatusRequestEntityTooLarge, CodePayloadTooLarge, "request body exceeds 1 MiB", nil)
			return false
		}
		WriteError(w, "invalid request body", http.StatusBadRequest)
		return false
	}
	return true
}

// WriteError is Write with its code chosen from status via CodeForStatus,
// for the handlers that only ever decided a status and a message --
// http.Error's own two arguments, in http.Error's own order, so a call
// site converts by renaming the call rather than reordering it.
func WriteError(w http.ResponseWriter, message string, status int) {
	Write(w, status, CodeForStatus(status), message, nil)
}

// CodeForStatus is the default status-to-code mapping for a handler that
// has no more specific reason to choose one -- which is most handlers,
// since most only ever decided a status. A handler that already has a
// more specific code in mind (payments.connect's FAILED_PRECONDITION on a
// 409 that is not a resource conflict, say) calls Write directly instead
// of going through this default.
func CodeForStatus(status int) Code {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return CodeInvalidArgument
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusPaymentRequired:
		return CodePaymentRequired
	case http.StatusConflict:
		return CodeConflict
	case http.StatusRequestEntityTooLarge:
		return CodePayloadTooLarge
	case http.StatusTooManyRequests:
		return CodeRateLimited
	default:
		return CodeInternal
	}
}
