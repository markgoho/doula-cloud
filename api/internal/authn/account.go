package authn

import (
	"context"
	"errors"
	"fmt"

	"firebase.google.com/go/v4/auth"
)

// Account is the Identity Platform fact set #613's mail flows ever read
// back: the account's current address, and whether Identity Platform
// considers it verified. It is deliberately smaller than auth.UserRecord
// -- callers outside this package have no business reading anything else
// Identity Platform stores.
type Account struct {
	Email         string
	EmailVerified bool
}

// ErrAccountNotFound is what GetAccountByEmail returns for an address no
// Identity Platform account holds -- password reset's request endpoint
// reads this and answers the caller identically either way (#168's
// account-enumeration rule, restated by #613).
var ErrAccountNotFound = errors.New("authn: no account for that identifier")

// AccountManager is the Admin SDK surface #613 widens ADR-0004's
// single-method Verifier into: reading and writing the account records
// Identity Platform owns as credential store, never sending mail --
// #169's decision moves the post office to Doula Cloud's own outbox and
// leaves Identity Platform exactly these four calls.
type AccountManager interface {
	// GetAccount reads uid's current address and verified flag. Used at
	// outbox send time, so the address mailed is always the account's
	// current one rather than whatever staff.email held when the row was
	// queued (#614 -- the two can drift).
	GetAccount(ctx context.Context, uid string) (Account, error)
	// GetAccountByEmail resolves an address to the account holding it,
	// or ErrAccountNotFound.
	GetAccountByEmail(ctx context.Context, email string) (uid string, err error)
	// SetEmailVerified flips uid's emailVerified flag to true -- spending
	// a verification link, or accepting a Staff invitation, calls this.
	SetEmailVerified(ctx context.Context, uid string) error
	// SetPassword sets uid's password credential. Spending a reset link
	// calls this; it does not itself mint a session (#613: a reset must
	// not walk past #167's enforced MFA).
	SetPassword(ctx context.Context, uid, password string) error
	// SetEmail changes uid's account address and clears its verified
	// flag -- the new address has not been proven, whatever the old one's
	// state was. A Staff member who changes address goes back through
	// verification the same way self-signup does.
	SetEmail(ctx context.Context, uid, email string) error
	// ClearSecondFactors removes every MFA factor uid has enrolled --
	// #615's mechanism note: MFASettings(MultiFactorSettings{}) is a
	// whole-list replace, there is no per-factor delete, and a Staff
	// member is never expected to hold more than the one TOTP enrolment
	// this exists to clear. Every recovery path's spend calls this, never
	// accounts.mfaEnrollment:withdraw, which #605 ruled out -- it demands
	// the end user's own ID token and cannot be driven by this service
	// account.
	ClearSecondFactors(ctx context.Context, uid string) error
	// DeleteAccount removes uid's Identity Platform account outright --
	// the credential and the address it holds, gone. #394's Client
	// erasure is the only caller and the first account deletion in this
	// codebase: everything else in this interface changes an account,
	// none of it ends one. It is deliberately not offered for Staff --
	// a Staff member's Membership ends, which is a different act
	// entirely (ADR-0027: ending is not erasure).
	//
	// Deleting the account does not invalidate a session she already
	// holds: sessions are rows in Postgres, verified against Postgres.
	// The caller deletes those itself, in the same transaction.
	//
	// Reports no error for a uid Identity Platform does not know, so a
	// retried erasure is a no-op rather than a failure.
	DeleteAccount(ctx context.Context, uid string) error
}

var _ AccountManager = (*FirebaseVerifier)(nil)

// GetAccount reads uid's account via the Admin SDK.
func (v *FirebaseVerifier) GetAccount(ctx context.Context, uid string) (Account, error) {
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	rec, err := v.client.GetUser(ctx, uid)
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		return Account{}, fmt.Errorf("authn: get account: %w", err)
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	return Account{Email: rec.Email, EmailVerified: rec.EmailVerified}, nil
}

// GetAccountByEmail resolves email to its account via the Admin SDK.
func (v *FirebaseVerifier) GetAccountByEmail(ctx context.Context, email string) (string, error) {
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	rec, err := v.client.GetUserByEmail(ctx, email)
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	if auth.IsUserNotFound(err) {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		return "", ErrAccountNotFound
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		return "", fmt.Errorf("authn: get account by email: %w", err)
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	return rec.UID, nil
}

// SetEmailVerified sets uid's emailVerified flag via the Admin SDK.
func (v *FirebaseVerifier) SetEmailVerified(ctx context.Context, uid string) error {
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	if _, err := v.client.UpdateUser(ctx, uid, (&auth.UserToUpdate{}).EmailVerified(true)); err != nil {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		return fmt.Errorf("authn: set email verified: %w", err)
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	return nil
}

// SetPassword sets uid's password via the Admin SDK.
func (v *FirebaseVerifier) SetPassword(ctx context.Context, uid, password string) error {
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	if _, err := v.client.UpdateUser(ctx, uid, (&auth.UserToUpdate{}).Password(password)); err != nil {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		return fmt.Errorf("authn: set password: %w", err)
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	return nil
}

// SetEmail changes uid's account address via the Admin SDK, clearing
// emailVerified in the same call -- the new address has not been proven.
func (v *FirebaseVerifier) SetEmail(ctx context.Context, uid, email string) error {
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	if _, err := v.client.UpdateUser(ctx, uid, (&auth.UserToUpdate{}).Email(email).EmailVerified(false)); err != nil {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		return fmt.Errorf("authn: set email: %w", err)
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	return nil
}

// DeleteAccount deletes uid's account via the Admin SDK, treating "no
// such user" as success -- a retried erasure must not fail on work it
// already did.
func (v *FirebaseVerifier) DeleteAccount(ctx context.Context, uid string) error {
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	err := v.client.DeleteUser(ctx, uid)
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	if err != nil && !auth.IsUserNotFound(err) {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		return fmt.Errorf("authn: delete account: %w", err)
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	return nil
}

// ClearSecondFactors removes every MFA factor uid holds via the Admin
// SDK. MFASettings is a whole-list replace -- an empty
// MultiFactorSettings{} is the documented way to clear every enrolled
// factor at once (#605's mechanism note), and it is the only admin route
// there is: accounts.mfaEnrollment:withdraw needs the end user's own ID
// token.
func (v *FirebaseVerifier) ClearSecondFactors(ctx context.Context, uid string) error {
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	if _, err := v.client.UpdateUser(ctx, uid, (&auth.UserToUpdate{}).MFASettings(auth.MultiFactorSettings{})); err != nil {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		return fmt.Errorf("authn: clear second factors: %w", err)
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	return nil
}
