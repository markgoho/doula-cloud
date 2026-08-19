// Package authntest provides a shared test double for authn.Verifier, so
// the test packages that exercise authenticated HTTP handlers do not each
// carry their own copy of it. Production code must not import it.
package authntest

import (
	"context"
	"errors"
	"time"

	"doula-cloud/api/internal/authn"
)

// ErrRevoked stands in for Identity Platform's own revocation error, so
// a test can report a credential as revoked by setting it as a
// Verifier's Err -- for example VerifySessionCookie -- without a live
// project.
var ErrRevoked = errors.New("authntest: session revoked")

// Verifier is a test double for authn.Verifier. Real Identity Platform
// token verification needs a live GCP project, so tests substitute this
// and state the outcome they want directly.
type Verifier struct {
	// UID is the caller identity VerifyIDToken and VerifySessionCookie
	// report on success.
	UID string
	// Err, when non-nil, is returned instead of a verified token by
	// VerifyIDToken and VerifySessionCookie. Set it to ErrRevoked to
	// simulate a revoked credential.
	Err error
	// MintErr, when non-nil, is returned instead of a session cookie by
	// MintSessionCookie.
	MintErr error
	// IssuedAt and Expires, when non-zero, are reported on the token
	// VerifySessionCookie returns, so a test can drive authn.Begin's
	// renewal (#147) by placing the cookie before or after half its
	// life relative to time.Now().
	IssuedAt time.Time
	Expires  time.Time
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

// MintSessionCookie returns v.MintErr if it is set, and a fake cookie
// value carrying v.UID otherwise. The token and expiry arguments are
// ignored — what a test wants back is stated on the Verifier.
func (v Verifier) MintSessionCookie(_ context.Context, _ string, _ time.Duration) (string, error) {
	if v.MintErr != nil {
		return "", v.MintErr
	}
	return "fake-session-cookie:" + v.UID, nil
}

// VerifySessionCookie returns v.Err if it is set, and a token carrying
// v.UID, v.IssuedAt, and v.Expires otherwise. The cookie argument is
// ignored — what a test wants back is stated on the Verifier.
func (v Verifier) VerifySessionCookie(_ context.Context, _ string) (*authn.VerifiedToken, error) {
	if v.Err != nil {
		return nil, v.Err
	}
	return &authn.VerifiedToken{UID: v.UID, IssuedAt: v.IssuedAt, Expires: v.Expires}, nil
}

// RevokeRefreshTokens always succeeds. Tests that need a request made
// after revocation to fail should construct a Verifier with Err set to
// ErrRevoked, rather than relying on this call to have a stateful
// effect.
func (v Verifier) RevokeRefreshTokens(_ context.Context, _ string) error {
	return nil
}
