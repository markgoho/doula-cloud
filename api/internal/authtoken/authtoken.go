// Package authtoken owns auth_tokens (00061), the one BFF-minted token
// table #613's ticket asks for: a purpose column and a per-purpose
// expiry, not a table per purpose. #166's Client magic link and #605's
// MFA-recovery decision are expected to widen Purpose rather than build
// a table of their own.
//
// Mirroring authn's sessions table (00028): only a SHA-256 digest of the
// token is ever stored, so a leaked read of this table hands nobody a
// usable credential.
package authtoken

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// Purpose names what a token authorizes. Widen this, and auth_token_purpose
// (00061) together, rather than adding a table when a new purpose arrives.
type Purpose string

const (
	// PurposeStaffEmailVerification proves control of the address an
	// Identity Platform account already holds -- spending it sets
	// emailVerified via the Admin SDK. 24-hour expiry (#613).
	PurposeStaffEmailVerification Purpose = "staff_email_verification"
	// PurposeStaffPasswordReset authorizes one password change and ends
	// every session for the identity it names. 1-hour expiry (#613).
	PurposeStaffPasswordReset Purpose = "staff_password_reset"
	// PurposeStaffMFARecovery is the Owner-vouched issued recovery code
	// (#615, #605 §4.2.1.3): spending it clears the named identity's TOTP
	// enrolment and mints no session. 24-hour expiry, minted by MintCode
	// rather than Mint -- a short decimal code read aloud over the phone,
	// not a link.
	PurposeStaffMFARecovery Purpose = "staff_mfa_recovery"
	// PurposeClientMagicLink is a Client's sign-in link (#617, ADR-0026):
	// spending it mints a session for the Portal Account identifier it
	// names. 15-minute expiry -- Mailgun's message logs (ADR-0012) hold
	// the token longer than her inbox does, so the window stays short.
	PurposeClientMagicLink Purpose = "client_magic_link"
	// PurposeClientSignInAddressChange proves control of the *new*
	// address a Client asked to move her Portal Account to (#619,
	// ADR-0026): spending it moves portal_accounts.sign_in_address and
	// mints no session -- the old address keeps signing her in until
	// this is spent. 24-hour expiry: it grants no session and no
	// credential on its own, the same reasoning
	// PurposeStaffEmailVerification carries, and the mailbox it is
	// proving may be one she only reads once a day.
	PurposeClientSignInAddressChange Purpose = "client_sign_in_address_change"
)

// ErrInvalid is what Spend returns for a token that never existed, was
// already spent, or has expired -- deliberately one outcome rather than
// three: none of them is a caller's business to distinguish, the same
// reasoning authn's errNoSession gives for a session token.
var ErrInvalid = errors.New("authtoken: token invalid, spent, or expired")

// Querier is the subset of *sql.DB and *sql.Tx this package uses,
// mirroring authn.Querier -- a caller mints or spends a token on its own
// request-scoped transaction so a later failure rolls the mint/spend
// back with everything else.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Mint issues a fresh token for identityUID under purpose, expiring
// ttl after now, and returns its plaintext value -- only its digest is
// stored, so this is the one moment that value exists.
//
// Any unspent token this identity already holds for purpose is deleted
// first, so a re-request kills the previous link outright (#613's
// re-request AC: a link delivered on a late outbox retry must not
// coexist with a fresher one).
func Mint(ctx context.Context, q Querier, identityUID string, purpose Purpose, ttl time.Duration, now time.Time) (string, error) {
	if _, err := q.ExecContext(ctx,
		`DELETE FROM auth_tokens WHERE identity_uid = $1 AND purpose = $2 AND used_at IS NULL`,
		identityUID, purpose,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("authtoken: invalidate prior tokens: %w", err)
	}

	// rand.Text returns 128+ bits of cryptographic randomness as text and
	// cannot fail, so minting needs no error path of its own for it --
	// sessions' MintSession makes the same call.
	token := rand.Text()

	if _, err := q.ExecContext(ctx,
		`INSERT INTO auth_tokens (token_hash, purpose, identity_uid, expires_at) VALUES ($1, $2, $3, $4)`,
		digest(token), purpose, identityUID, now.Add(ttl),
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("authtoken: insert: %w", err)
	}
	return token, nil
}

// mfaCodeDigits is #605's §4.2.1.2 floor ("at least six decimal digits")
// plus a small margin, still short enough to read aloud over a phone
// call -- the shape #615's Owner-vouch path needs, unlike Mint's
// rand.Text() link tokens.
const mfaCodeDigits = 8

// mfaCodeSpace is the exclusive upper bound randomDigits draws from: 10^mfaCodeDigits.
var mfaCodeSpace = new(big.Int).Exp(big.NewInt(10), big.NewInt(mfaCodeDigits), nil)

// MintCode is Mint for a short, all-decimal code rather than a long
// link token -- #615's issued recovery code, meant to be read aloud over
// a phone call. Deletes any unspent code this identity already holds for
// purpose first, the same re-request rule Mint applies.
//
// A decimal code's keyspace (10^8 here) is small enough that
// token_hash's PRIMARY KEY can collide across two unrelated recoveries
// minted around the same time -- astronomically unlikely, but Mint's
// bare INSERT would surface it as an opaque constraint-violation error
// rather than just trying again, so this retries a fresh draw a few
// times instead.
func MintCode(ctx context.Context, q Querier, identityUID string, purpose Purpose, ttl time.Duration, now time.Time) (string, error) {
	if _, err := q.ExecContext(ctx,
		`DELETE FROM auth_tokens WHERE identity_uid = $1 AND purpose = $2 AND used_at IS NULL`,
		identityUID, purpose,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("authtoken: invalidate prior codes: %w", err)
	}

	const maxAttempts = 5
	for range maxAttempts {
		code, err := randomDigits(mfaCodeDigits)
		if err != nil {
			// coverage:ignore reason: crypto/rand.Int only errors when its bound is non-positive; mfaCodeSpace is a fixed positive constant, not exercised by unit tests
			return "", fmt.Errorf("authtoken: generate code: %w", err)
		}

		res, err := q.ExecContext(ctx,
			`INSERT INTO auth_tokens (token_hash, purpose, identity_uid, expires_at) VALUES ($1, $2, $3, $4)
			 ON CONFLICT (token_hash) DO NOTHING`,
			digest(code), purpose, identityUID, now.Add(ttl),
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return "", fmt.Errorf("authtoken: insert code: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return code, nil
		}
	}
	// coverage:ignore reason: requires forcing mfaCodeDigits collisions across maxAttempts draws, not exercised by unit tests
	return "", fmt.Errorf("authtoken: could not mint a unique code after %d attempts", maxAttempts)
}

// randomDigits draws a cryptographically random n-digit decimal string,
// zero-padded -- unlike rand.Text(), the result may start with '0'.
func randomDigits(n int) (string, error) {
	v, err := rand.Int(rand.Reader, mfaCodeSpace)
	if err != nil {
		// coverage:ignore reason: crypto/rand.Int only errors when its bound is non-positive; mfaCodeSpace is a fixed positive constant, not exercised by unit tests
		return "", fmt.Errorf("authtoken: draw random digits: %w", err)
	}
	return fmt.Sprintf("%0*d", n, v), nil
}

// Spend resolves token under purpose to the identity_uid holding it and
// marks it used, atomically: the UPDATE's own WHERE clause is the
// single-use check, so two concurrent spends of the same token can never
// both succeed. Returns ErrInvalid for a token that does not match a
// live, unspent row under this purpose -- wrong purpose, already spent,
// or past expires_at are all the same outcome.
func Spend(ctx context.Context, q Querier, token string, purpose Purpose, now time.Time) (identityUID string, err error) {
	err = q.QueryRowContext(ctx,
		`UPDATE auth_tokens SET used_at = $1
		 WHERE token_hash = $2 AND purpose = $3 AND used_at IS NULL AND expires_at > $1
		 RETURNING identity_uid`,
		now, digest(token), purpose,
	).Scan(&identityUID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrInvalid
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("authtoken: spend: %w", err)
	}
	return identityUID, nil
}

func digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Digest exposes token_hash's own digest -- SHA-256 of the plaintext,
// hex-encoded -- to a caller that needs to key a companion row off the
// same primary key auth_tokens uses, without storing the plaintext
// itself. staff_mfa_recovery_vouches (00062) is the one caller: it needs
// to look a mint back up by its token_hash at spend time, and computing
// a different digest there would silently never match.
func Digest(token string) string {
	return digest(token)
}
