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
)

// APIError is docs/api-design.md section 7's structured error shape.
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// Write sends status with body {code, message, details} as JSON. details
// is nil for the large majority of call sites, which have nothing
// field-specific to say; #529 does not build new plumbing to populate it
// everywhere, only where a caller already has field-level information on
// hand.
func Write(w http.ResponseWriter, status int, code Code, message string, details map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	_ = json.NewEncoder(w).Encode(APIError{Code: string(code), Message: message, Details: details})
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
