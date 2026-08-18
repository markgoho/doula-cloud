// Package authntest provides a shared test double for authn.Verifier, so
// the test packages that exercise authenticated HTTP handlers do not each
// carry their own copy of it. Production code must not import it.
package authntest

import (
	"context"

	"doula-cloud/api/internal/authn"
)

// Verifier is a test double for authn.Verifier. Real Identity Platform
// token verification needs a live GCP project, so tests substitute this
// and state the outcome they want directly.
type Verifier struct {
	// UID is the caller identity VerifyIDToken reports on success.
	UID string
	// Err, when non-nil, is returned instead of a verified token.
	Err error
}

// Compile-time proof that Verifier still satisfies the interface it doubles.
var _ authn.Verifier = Verifier{}

// VerifyIDToken returns v.Err if it is set, and a token carrying v.UID
// otherwise. The token argument is ignored — what a test wants back is
// stated on the Verifier, not encoded in the token it passes.
func (v Verifier) VerifyIDToken(_ context.Context, _ string) (*authn.VerifiedToken, error) {
	if v.Err != nil {
		return nil, v.Err
	}
	return &authn.VerifiedToken{UID: v.UID}, nil
}
