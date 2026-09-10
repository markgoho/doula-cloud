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
	// CountWithoutSecondFactor reports how many of uids currently hold no
	// enrolled MFA factor -- #606's switch confirmation, "how many Staff
	// it will affect before she throws it". A session's second_factor
	// column (see store.go) describes a past sign-in, not current
	// enrolment, so this is the one place the count has to ask Identity
	// Platform directly rather than read Postgres. Batched (100 uids per
	// Admin SDK call, GetUsers' own limit) rather than one round trip per
	// Staff member -- CLAUDE.md's performance expectation -- so a
	// Practice larger than 100 costs a handful of calls, not one per
	// person.
	CountWithoutSecondFactor(ctx context.Context, uids []string) (int, error)
	// DeleteAccount destroys uid's Identity Platform account outright --
	// #892's Staff login deletion, the one act in the product that ends a
	// person's ability to authenticate rather than narrowing her reach.
	// This existed once before, for Client erasure (#394), and was
	// retired by #796 when Clients stopped holding Identity Platform
	// accounts at all (00075); it is restored here for the population
	// that still does.
	//
	// An account Identity Platform already reports absent is a success,
	// not a failure: the act is irreversible and idempotent, and a retry
	// after a half-applied attempt must be able to finish rather than
	// dead-end on the part that already ran. Callers get nil, never
	// ErrAccountNotFound.
	//
	// Deleting the account does not end a session: a __session cookie is
	// verified against Postgres (ADR-0004), not against Identity
	// Platform, so the caller must delete her sessions rows itself.
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

// DeleteAccount destroys uid's account via the Admin SDK, treating an
// already-absent account as done. auth.IsUserNotFound is the same
// not-found test GetAccountByEmail above uses.
func (v *FirebaseVerifier) DeleteAccount(ctx context.Context, uid string) error {
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	err := v.client.DeleteUser(ctx, uid)
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	if err == nil || auth.IsUserNotFound(err) {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		return nil
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	return fmt.Errorf("authn: delete account: %w", err)
}

// getUsersBatchSize is the Admin SDK's own cap on one GetUsers call
// (auth.maxGetAccountsBatchSize, unexported in firebase.google.com/go) --
// CountWithoutSecondFactor chunks to it rather than assuming any one
// Practice's roster stays under it.
const getUsersBatchSize = 100

// CountWithoutSecondFactor implements AccountManager, via one batched
// Admin SDK accounts:lookup call (auth.Client.GetUsers) per
// getUsersBatchSize-sized chunk of uids.
func (v *FirebaseVerifier) CountWithoutSecondFactor(ctx context.Context, uids []string) (int, error) {
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	count := 0
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	for chunkStart := 0; chunkStart < len(uids); chunkStart += getUsersBatchSize {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		chunkEnd := min(chunkStart+getUsersBatchSize, len(uids))
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		chunkCount, err := v.countChunkWithoutSecondFactor(ctx, uids[chunkStart:chunkEnd])
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		if err != nil {
			// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
			return 0, err
		}
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		count += chunkCount
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	return count, nil
}

// countChunkWithoutSecondFactor is one GetUsers call's worth of
// CountWithoutSecondFactor -- uids must not exceed getUsersBatchSize.
func (v *FirebaseVerifier) countChunkWithoutSecondFactor(ctx context.Context, uids []string) (int, error) {
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	identifiers := make([]auth.UserIdentifier, len(uids))
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	for i, uid := range uids {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		identifiers[i] = auth.UIDIdentifier{UID: uid}
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	result, err := v.client.GetUsers(ctx, identifiers)
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		return 0, fmt.Errorf("authn: get users: %w", err)
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	count := len(result.NotFound)
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	for _, u := range result.Users {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		if u.MultiFactor == nil || len(u.MultiFactor.EnrolledFactors) == 0 {
			// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
			count++
		}
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	return count, nil
}

// ClearSecondFactors removes every MFA factor uid holds via the Admin
// SDK. MFASettings is a whole-list replace -- an empty
// MultiFactorSettings{} is the documented way to clear every enrolled
// factor at once (#605's mechanism note), and it is the only admin route
// there is: accounts.mfaEnrollment:withdraw needs the end user's own ID
// token.
//
// The body this puts on the wire is
// `{"localId":"...","mfa":{"enrollments":null}}` -- the SDK's
// validateAndFormatMfaSettings leaves its slice nil however the call is
// written, so no caller can turn that null into an array. #1128 probed
// it against the real doula-cloud Identity Platform project on
// 2026-09-10, on a throwaway account holding a real TOTP enrollment:
// 200, and the enrollment was gone on the read-back. Production reads
// the null as an empty repeated field, which is proto3-JSON's rule. The
// Firebase Auth emulator does not (`400 ... /mfa/enrollments must be
// array`), which is why no e2e spec walks a successful recovery spend --
// see app/e2e/mfa.ts. Do not rewrite this to send an array for the
// emulator's sake.
func (v *FirebaseVerifier) ClearSecondFactors(ctx context.Context, uid string) error {
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	if _, err := v.client.UpdateUser(ctx, uid, (&auth.UserToUpdate{}).MFASettings(auth.MultiFactorSettings{})); err != nil {
		// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
		return fmt.Errorf("authn: clear second factors: %w", err)
	}
	// coverage:ignore reason: requires a real GCP Identity Platform project, not exercised by unit tests
	return nil
}
