// Package staffauth is the Staff-population auth middleware: it verifies
// the caller's Identity Platform ID token, resolves it against the staff
// table, and confirms the resulting Staff person holds a
// practice_memberships row for the :practiceId in the URL -- setting
// app.current_practice_id on a request-scoped transaction only once all
// of that has passed, so Postgres RLS enforces it for the rest of the
// request.
package staffauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"doula-cloud/api/internal/authn"
)

type contextKey string

const (
	staffIDKey    contextKey = "staffauth.staffID"
	practiceIDKey contextKey = "staffauth.practiceID"
	txKey         contextKey = "staffauth.tx"
)

// StaffID returns the resolved Staff id for the current request.
func StaffID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(staffIDKey).(string)
	return id, ok
}

// PracticeID returns the :practiceId the current request is scoped to.
func PracticeID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(practiceIDKey).(string)
	return id, ok
}

// Tx returns the request-scoped database transaction, with
// app.current_practice_id already set for the life of the transaction.
// Handlers must run any query touching a practice-tier-RLS table through
// this transaction, not a fresh connection, or RLS will have nothing to
// scope against.
func Tx(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txKey).(*sql.Tx)
	return tx, ok
}

// Middleware wraps an http.Handler with Staff population resolution and
// practice-membership authorization. db must be a connection using the
// low-privilege app_runtime role -- the role the RLS policies in
// 00002_practice_staff_tenancy.sql apply to.
func Middleware(verifier authn.Verifier, db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			idToken, ok := bearerToken(r)
			if !ok {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}

			verified, err := verifier.VerifyIDToken(r.Context(), idToken)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			practiceID := r.PathValue("practiceId")
			if _, err := uuid.Parse(practiceID); err != nil {
				http.Error(w, "invalid practice id", http.StatusBadRequest)
				return
			}

			tx, err := db.BeginTx(r.Context(), nil)
			if err != nil {
				// coverage:ignore reason: DB connection failure, not exercised by unit tests
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			committed := false
			defer func() {
				if !committed {
					_ = tx.Rollback()
				}
			}()

			staffID, found, err := setIdentityAndResolveStaff(r.Context(), tx, verified.UID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if !found {
				http.Error(w, "no matching staff account", http.StatusForbidden)
				return
			}

			isMember, err := setPracticeAndCheckMembership(r.Context(), tx, staffID, practiceID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if !isMember {
				http.Error(w, "no membership at this practice", http.StatusForbidden)
				return
			}

			if _, err := tx.ExecContext(r.Context(), `UPDATE staff SET last_practice_id = $1 WHERE id = $2`, practiceID, staffID); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), staffIDKey, staffID)
			ctx = context.WithValue(ctx, practiceIDKey, practiceID)
			ctx = context.WithValue(ctx, txKey, tx)

			next.ServeHTTP(w, r.WithContext(ctx))

			if err := tx.Commit(); err == nil {
				committed = true
			}
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimPrefix(header, prefix)
	if token == "" {
		return "", false
	}
	return token, true
}

// setIdentityAndResolveStaff sets app.current_identity_uid -- the session
// variable staff's self-visibility RLS policy reads, since
// app.current_practice_id isn't known yet -- then looks up identityUID in
// staff.
func setIdentityAndResolveStaff(ctx context.Context, tx *sql.Tx, identityUID string) (string, bool, error) {
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		return "", false, fmt.Errorf("staffauth: set current identity uid: %w", err)
	}

	var staffID string
	err := tx.QueryRowContext(ctx, `SELECT id FROM staff WHERE identity_uid = $1`, identityUID).Scan(&staffID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return "", false, fmt.Errorf("staffauth: resolve staff: %w", err)
	}
	return staffID, true, nil
}

// setPracticeAndCheckMembership sets app.current_practice_id -- the
// session variable practice-tier RLS reads for the rest of the request --
// then checks whether staffID holds a practice_memberships row for
// practiceID.
func setPracticeAndCheckMembership(ctx context.Context, tx *sql.Tx, staffID, practiceID string) (bool, error) {
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		return false, fmt.Errorf("staffauth: set current practice id: %w", err)
	}

	var exists bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2)`,
		practiceID, staffID,
	).Scan(&exists)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return false, fmt.Errorf("staffauth: check practice membership: %w", err)
	}
	return exists, nil
}
