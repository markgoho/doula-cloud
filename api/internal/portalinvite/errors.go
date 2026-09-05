// Package portalinvite lets a Staff member provision Client-portal access:
// invite a Client on one of the Practice's Engagements, and let the Client
// accept by verifying their identity -- the provisioning path
// 00006_client_portal_users.sql's RLS and clientauth.Middleware assumed
// but never got, per #90. Mirrors staffauth's invite/accept shape, applied
// to client_portal_users instead of staff.
package portalinvite

// MsgInternalError is the response body for any failure the caller can't
// act on -- deliberately vague so it never leaks internals. Matches
// clientauth.MsgInternalError's wording, kept as its own copy per this
// repo's convention of small per-package copies (see staffauth,
// clientauth, contracts).
const MsgInternalError = "internal error"
