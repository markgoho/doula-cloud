// Session storage. A session is an opaque random token in the __session
// cookie plus one row in Postgres (00028_sessions.sql, ADR-0004), so
// verification is a local read inside the transaction Begin already
// opens and renewal is an UPDATE that needs no Bearer ID token.

package authn

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Querier is the subset of *sql.DB and *sql.Tx the session store uses,
// so a caller can mint a session either on its own request-scoped
// transaction -- which is what lets a bootstrap endpoint roll the whole
// signup back if minting fails -- or straight on the pool.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// errNoSession is what lookupSession returns when a token matches no
// live session row -- it was never issued, it was ended, or it has
// expired. Unexported because the three are deliberately
// indistinguishable outside this package: all three mean 401, and no
// caller has a reason to tell them apart.
var errNoSession = errors.New("authn: no live session for this token")

// MintSession creates a session for identityUID that expires
// SessionLifetimeFor(identityUID) after now, and returns the cookie
// carrying its token. The token is generated here and only its digest is
// stored, so this is the one moment the plaintext token exists.
//
// secondFactor is recorded on the row as-is, from the ID token that was
// verified to reach this call (VerifiedToken.SecondFactor) -- #606's
// decision 3: the boundary that later gates on this reads the session
// row, never Identity Platform or the token again, so the fact has to be
// captured here, at the one moment both exist together.
//
// It also sweeps expired rows first. Minting is rare (a sign-in) and the
// sweep is one indexed DELETE, which is enough to keep the table from
// accumulating the sessions nobody ever returns to -- no reaper job, no
// background goroutine.
func MintSession(ctx context.Context, q Querier, identityUID string, secondFactor bool, now time.Time) (*http.Cookie, error) {
	if _, err := q.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= $1`, now); err != nil {
		return nil, fmt.Errorf("authn: sweep expired sessions: %w", err)
	}

	// rand.Text returns 128+ bits of cryptographic randomness as text and
	// cannot fail, so a session token needs no error path of its own.
	token := rand.Text()
	lifetime := SessionLifetimeFor(identityUID)

	if _, err := q.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, identity_uid, expires_at, second_factor) VALUES ($1, $2, $3, $4)`,
		tokenHash(token), identityUID, now.Add(lifetime), secondFactor,
	); err != nil {
		return nil, fmt.Errorf("authn: insert session: %w", err)
	}

	return NewSessionCookie(token, lifetime), nil
}

// EndSession deletes the session token names. It reports no error for a
// token that matches nothing: ending an already-ended session is the
// same outcome as ending a live one.
func EndSession(ctx context.Context, q Querier, token string) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash(token)); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("authn: delete session: %w", err)
	}
	return nil
}

// EndAllSessions deletes every session row identityUID holds -- every
// device that person is signed in from, not just one token. This is the
// administrative "end their sessions everywhere" action (#154); ordinary
// sign-out is EndSession, by token, above.
func EndAllSessions(ctx context.Context, q Querier, identityUID string) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM sessions WHERE identity_uid = $1`, identityUID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("authn: delete all sessions: %w", err)
	}
	return nil
}

// lookupSession resolves token to the identity holding it, that
// session's expiry, and whether it carries a second factor, or
// errNoSession if no row is live at now. Expired rows are filtered here
// rather than deleted, because this read runs inside a transaction that
// a failed authorization check later rolls back; MintSession's sweep is
// what actually removes them.
func lookupSession(ctx context.Context, q Querier, token string, now time.Time) (identityUID string, expiresAt time.Time, secondFactor bool, err error) {
	err = q.QueryRowContext(ctx,
		`SELECT identity_uid, expires_at, second_factor FROM sessions WHERE token_hash = $1 AND expires_at > $2`,
		tokenHash(token), now,
	).Scan(&identityUID, &expiresAt, &secondFactor)
	if errors.Is(err, sql.ErrNoRows) {
		return "", time.Time{}, false, errNoSession
	}
	if err != nil {
		return "", time.Time{}, false, fmt.Errorf("authn: look up session: %w", err)
	}
	return identityUID, expiresAt, secondFactor, nil
}

// renewSession pushes token's expiry out to a full lifetime from now.
// The token itself never changes, so the browser keeps the cookie value
// it already holds and only its MaxAge is refreshed.
func renewSession(ctx context.Context, q Querier, token string, lifetime time.Duration, now time.Time) error {
	if _, err := q.ExecContext(ctx,
		`UPDATE sessions SET expires_at = $1 WHERE token_hash = $2`,
		now.Add(lifetime), tokenHash(token),
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("authn: renew session: %w", err)
	}
	return nil
}

// tokenHash is what the sessions table stores in place of the token --
// see 00028_sessions.sql for why a plain digest is the right primitive
// for an input that is already all entropy.
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
