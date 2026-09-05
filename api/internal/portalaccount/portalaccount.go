// Package portalaccount mints the identifier a Portal Account is known
// by, per ADR-0026's "The Portal Account becomes a table": Doula Cloud
// issues this identity itself now that a Client has no Identity Platform
// account behind her, and the identifier is what portal_accounts.identifier
// and client_portal_users.identity_uid both hold.
package portalaccount

import "github.com/google/uuid"

// Prefix marks an identifier as one Doula Cloud minted itself, never one
// Identity Platform issued. It is the namespace the ADR names as the
// sanctioned way to tell the two populations' identity_uid values apart
// -- not a hint read alongside the value, but the value's own proof of
// which issuer made it.
//
// The claim that it cannot collide with an Identity Platform uid is
// total, not probable: Identity Platform documents its auto-generated
// uids as 28 characters drawn from [A-Za-z0-9] only
// (https://cloud.google.com/identity-platform/docs/reference/rest/v1/UserInfo),
// and this codebase never calls CreateUser with a custom uid of its own
// -- so every Identity Platform uid this product ever sees is drawn from
// that alphabet. Prefix contains "_", a character outside it, so no
// string built from Prefix can ever equal one.
const Prefix = "portal_"

// NewIdentifier mints a fresh Portal Account identifier: Prefix followed
// by a random UUID. Called once, at accept time -- the one place a
// Portal Account comes into being (ADR-0026: "the invitation *is* the
// first magic link").
func NewIdentifier() string {
	return Prefix + uuid.NewString()
}
