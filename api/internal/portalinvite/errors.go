// Package portalinvite lets a Staff member provision Client-portal access:
// invite a Client on one of the Practice's Engagements, and let the Client
// accept by verifying their identity -- the provisioning path
// 00006_client_portal_users.sql's RLS and clientauth.Middleware assumed
// but never got, per #90. Mirrors staffauth's invite/accept shape, applied
// to client_portal_users instead of staff.
package portalinvite

import (
	"encoding/json"
	"net/http"
)

// MsgInternalError is the response body for any failure the caller can't
// act on -- deliberately vague so it never leaks internals. Matches
// clientauth.MsgInternalError's wording, kept as its own copy per this
// repo's convention of small per-package copies (see staffauth,
// clientauth, contracts).
const MsgInternalError = "internal error"

// codeInternalError is the APIError.Code used for every 500 response in
// this package.
const codeInternalError = "INTERNAL_ERROR"

// APIError is docs/api-design.md section 7's structured error shape. This
// package is the first adopter: a brand-new pair of endpoints with no
// existing consumers to break, so there's no contract-stability cost to
// using it here (unlike retrofitting an existing handler's plain-text
// body).
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// writeAPIError writes status with body {code, message} as JSON.
func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	_ = json.NewEncoder(w).Encode(APIError{Code: code, Message: message})
}
