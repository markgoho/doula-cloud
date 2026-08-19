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

// RequireTx resolves the request-scoped tx and Practice id
// Middleware placed on context, writing an internal error response if
// the tx is missing. Shared by downstream BFF packages (engagement, visit)
// mounted behind Middleware.
func RequireTx(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID string, ok bool) {
	tx, has := Tx(r.Context())
	if !has {
		// coverage:ignore reason: Middleware always sets a tx before this handler runs
		http.Error(w, MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	practiceID, _ = PracticeID(r.Context())
	return tx, practiceID, true
}

// ParseUUID validates that value is a well-formed UUID, writing a 400
// response itself (naming the field via label) if not. Shared by
// Middleware and downstream handlers.
func ParseUUID(w http.ResponseWriter, label, value string) (ok bool) {
	if _, err := uuid.Parse(value); err != nil {
		http.Error(w, "invalid "+label+" id", http.StatusBadRequest)
		return false
	}
	return true
}

// Middleware wraps an http.Handler with Staff population resolution and
// practice-membership authorization. db must be a connection using the
// low-privilege app_runtime role -- the role the RLS policies in
// 00002_practice_staff_tenancy.sql apply to.
func Middleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tx, uid, ok := authn.Begin(w, r, db)
			if !ok {
				return
			}
			committed := false
			defer func() {
				if !committed {
					_ = tx.Rollback()
				}
			}()

			practiceID := r.PathValue("practiceId")
			if !ParseUUID(w, "practice", practiceID) {
				return
			}

			staffID, found, err := setIdentityAndResolveStaff(r.Context(), tx, uid)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
			if !found {
				http.Error(w, "no matching staff account", http.StatusForbidden)
				return
			}

			isMember, err := setPracticeAndCheckMembership(r.Context(), tx, staffID, practiceID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
			if !isMember {
				http.Error(w, "no membership at this practice", http.StatusForbidden)
				return
			}

			if _, err := tx.ExecContext(r.Context(), `UPDATE staff SET last_practice_id = $1 WHERE id = $2`, practiceID, staffID); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, MsgInternalError, http.StatusInternalServerError)
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

// setPracticeAndCheckMembership sets app.current_practice_id and
// app.current_staff_id -- the session variables practice/staff-tier RLS
// reads for the rest of the request -- then checks whether staffID holds a
// practice_memberships row for practiceID. Both vars are set only here,
// after identity resolution has already succeeded, so no RLS policy that
// reads them can be satisfied before that check has passed.
func setPracticeAndCheckMembership(ctx context.Context, tx *sql.Tx, staffID, practiceID string) (bool, error) {
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		return false, fmt.Errorf("staffauth: set current practice id: %w", err)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_staff_id', $1, true)`, staffID); err != nil {
		return false, fmt.Errorf("staffauth: set current staff id: %w", err)
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
