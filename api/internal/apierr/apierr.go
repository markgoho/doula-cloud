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
	"io"
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
	// CodePracticePendingDeletion is #871's Practice-scoped boundary
	// refusal: a live, valid session, and a Practice whose deletion is
	// pending. staffauth.Middleware writes it for every route under that
	// Practice except two: an Owner's own reach into
	// GET/DELETE .../deletion, and the plain identity read
	// GET .../session, open to every role. The app never actually
	// branches on this code -- .../session's own successful response now
	// carries a `pendingDeletion` flag instead, which
	// practices/[practiceId]/+layout.ts reads on every navigation to
	// route an Owner to the restore screen and everyone else to that
	// same screen's own locked notice. This code exists for the routes
	// still refused: it distinguishes "this Practice is locked" from an
	// ordinary CodeForbidden, the same reasoning CodeMFARequired's own
	// comment gives, for any caller (a stale tab, a direct API client)
	// that reaches one of them anyway.
	CodePracticePendingDeletion Code = "PRACTICE_PENDING_DELETION"
	// CodeBirthOutcomeFrozen is #293's press-through, the same shape
	// CodeSessionEvictionUnconfirmed names: the Engagement already
	// carries a birth outcome, ADR-0015 freezes it, and the same request
	// re-sent by a Practice Owner with correction: true goes through.
	// Nothing is written on the refusal. Its own code rather than the
	// generic CodeConflict, because the endpoint's other 409 -- a
	// correction offered where nothing is recorded -- is not
	// press-through at all, and a caller that told the two apart by
	// their prose would be doing what #692 forbids.
	CodeBirthOutcomeFrozen Code = "BIRTH_OUTCOME_FROZEN"
	// CodeBirthOutcomeRequired is #940's constraint said out loud: a
	// 'completed' Engagement carries a birth outcome
	// (engagements_completed_is_explained, 00094), and the two endpoints
	// that could otherwise break that rule say so by name rather than
	// letting the caller meet a raw constraint violation as a 500. The
	// status transition answers it when completion is asked for on a row
	// with no outcome; the birth-outcome endpoint answers it when a
	// correction would clear the outcome off a row already completed.
	// Nothing is written on either refusal, and neither is a
	// press-through: re-sending the same request changes nothing, which
	// is what makes this a different thing from CodeBirthOutcomeFrozen.
	//
	// Its own code rather than the generic CodeConflict, because both
	// endpoints already answer some *other* 409 -- the frozen
	// press-through, and a correction offered where nothing is recorded
	// -- and a caller that told them apart by their prose would be doing
	// what #692 forbids. No screen branches on it today: the app shows
	// the BFF's own sentence in the completion form's error summary,
	// since the control that answers it is a section of the same page
	// rather than a field of that form. The code exists so a caller can
	// branch, the same reasoning CodePracticePendingDeletion records.
	CodeBirthOutcomeRequired Code = "BIRTH_OUTCOME_REQUIRED"
	// CodeOfferCodeExhausted is #846's own 429: the pre-account Offer
	// read has spent all of maxAccessCodeAttempts' guesses against one
	// Offer's six-digit code (00041), and the Offer stays shut until the
	// Practice sends it again. The other 429 on that same route is
	// ratelimit.Wrap's per-Offer cap, which is CodeRateLimited and means
	// the opposite thing -- wait, and the same request works later. A
	// caller that told those two apart by their prose, or by a
	// Retry-After header only one of them carries, would be doing what
	// #692 forbids, so the permanent one gets its own code.
	CodeOfferCodeExhausted Code = "OFFER_CODE_EXHAUSTED"
)

// ForbiddenCodes is the recorded set of reason codes a 403 may carry
// (#918), and the reason the app's error page can tell one 403 from
// another at all.
//
// Before this set existed, 403 was the only signal a refusal carried
// across the wire, so the error page asserted the one cause a status
// code cannot actually name -- "your role does not have permission" --
// for every 403 alike, including two that are not about a role at all.
// Three codes, one per kind of thing a reader can do about it:
//
//   - CodeForbidden -- a role refusal. CodeForStatus hands this to every
//     403 that names no more specific reason, so it is also the default,
//     and the app reads an absent or unrecognized code the same way.
//   - CodePracticePendingDeletion -- a Practice-level condition. Nothing
//     about the reader; a different Practice-level fact has to change.
//   - CodeMFARequired -- a step the reader can take. Trying again does
//     nothing, but enrolling a second factor and then trying again does.
//
// A fourth kind of 403 needs a fourth code here and a fourth state on
// the app's error page, not a fourth shade of the role-refusal copy.
// TestForbiddenCodesAreTheRecordedSet holds the set closed: an
// apierr.Write that pairs a literal http.StatusForbidden with a code
// outside it fails the build.
var ForbiddenCodes = map[Code]bool{
	CodeForbidden:               true,
	CodePracticePendingDeletion: true,
	CodeMFARequired:             true,
}

// APIError is docs/api-design.md section 7's structured error shape.
//
// Code is the enumerated Code type, not a bare string: #811 retyped it so
// a reader compares against the constants above directly rather than
// writing string(CodeConflict) at every assertion, and so a code that
// travels between packages stays typed instead of decaying to a string
// on the way. It does not stop a bare literal being compared against --
// an untyped string constant still converts -- so the enumeration is a
// convenience for readers, not a closed set the compiler enforces.
// Code's underlying type is string, so the JSON is unchanged either way
// -- see TestAPIError_CodeSerializesAsAPlainJSONString, which holds that.
type APIError struct {
	Code    Code              `json:"code"`
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
	WriteJSON(w, status, APIError{Code: code, Message: message, Details: details})
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
	return decodeJSON(w, r, v, false)
}

// DecodeJSONOptional is DecodeJSON for a write whose body itself is
// optional -- visit.CreateHandler's own case (#250): a Visit may be
// created with no scheduled instant at all, and that request has always
// sent no body, so demanding one now would break every caller that still
// sends none. An empty body (io.EOF on the first token) leaves v at its
// zero value and reports success; any other decode failure -- a
// malformed body, one that trips the 1 MiB cap -- refuses exactly the
// way DecodeJSON does, since a body that *was* sent is still held to the
// same shape.
func DecodeJSONOptional(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeJSON(w, r, v, true)
}

// decodeJSON is the one place the two public decoders' shared body lives,
// so DecodeJSONOptional's empty-body allowance can't drift from
// DecodeJSON's own decode/error handling by being a second hand-copy of
// it.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any, allowEmpty bool) bool {
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		if allowEmpty && errors.Is(err, io.EOF) {
			return true
		}
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
