// Package staffauth is the Staff-population auth middleware: it verifies
// the caller's Identity Platform ID token, resolves it against the staff
// table, and confirms the resulting Staff person holds a
// practice_memberships row for the :practiceId in the URL -- setting
// app.current_practice_id on a request-scoped transaction only once all
// of that has passed, so Postgres RLS enforces it for the rest of the
// request. The same query also reads that row's roles and employment
// type, and Middleware builds this request's Reader from them and places
// it on the context (ReaderFrom) -- the one practice_memberships read a
// request needs for its own Membership, rather than a query per handler.
package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/tasknudge"
)

type contextKey string

const (
	staffIDKey    contextKey = "staffauth.staffID"
	practiceIDKey contextKey = "staffauth.practiceID"
	txKey         contextKey = "staffauth.tx"
	readerKey     contextKey = "staffauth.reader"
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

// ReaderFrom returns the Reader Middleware resolved for the current
// request -- the caller's roles and employment type at the Practice in
// the URL, loaded by the one practice_memberships query Middleware runs.
// RequireOwner, RequireOwnerOrAdmin, visit.requireDoula, and every
// handler that used to call the now-removed ResolveReader read it from
// here instead, so a request resolves its Membership once rather than
// once per handler. ok is false only if Middleware never ran ahead of
// this handler.
func ReaderFrom(ctx context.Context) (Reader, bool) {
	reader, ok := ctx.Value(readerKey).(Reader)
	return reader, ok
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
		apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
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
		apierr.WriteError(w, "invalid "+label+" id", http.StatusBadRequest)
		return false
	}
	return true
}

// RequireConfirmed guards the four undoable actions #473 found with no
// confirmation at all. It checks for the one signal ConfirmDialog sends
// -- the X-Confirmed header -- and writes its own 400 if it is missing.
// This is honest about what a server can verify: the header's presence
// proves the client attempted to signal confirmation, not that a person
// actually meant it; the real enforcement is client-side, ConfirmDialog
// never sending the request until its own confirm button is pressed.
// Shared across staffauth and offer, both of which already import this
// package.
func RequireConfirmed(w http.ResponseWriter, r *http.Request) (ok bool) {
	if r.Header.Get("X-Confirmed") != "true" {
		apierr.WriteError(w, "this action requires confirmation", http.StatusBadRequest)
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
			tx, uid, secondFactor, ok := authn.Begin(w, r, db)
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
				apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
			if !found {
				apierr.WriteError(w, MsgNoMatchingStaffAccount, http.StatusForbidden)
				return
			}

			isMember, roles, employmentType, requireMFA, err := setPracticeAndCheckMembership(r.Context(), tx, staffID, practiceID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
			if !isMember {
				apierr.WriteError(w, "no membership at this practice", http.StatusForbidden)
				return
			}
			reader := NewReader(staffID, roles, employmentType)

			// #606: refuse a Practice-scoped request when the caller's
			// Membership there holds Owner, or the Practice's own switch
			// is on, and her session shows no second factor. Signing in
			// stays open to everyone (authn.Begin above never gates on
			// this) -- only entering a Practice that demands it does.
			// This is not an ended session (401 would send the browser to
			// the login screen, per credentialed-fetch's own contract),
			// so it is a distinct, machine-readable 403 the app branches
			// on to route into enrolment instead.
			if !secondFactor && (reader.Has(roleOwner) || requireMFA) {
				writeMFARequired(w)
				return
			}

			// last_active_at rides along with last_practice_id because
			// both are the same act -- somebody from this Practice was
			// here -- and neither is worth a second round trip.
			//
			// It is the durable record that a Practice made contact. New
			// York escheats her unspent Credit balance at three years'
			// dormancy (APL 1315(1-b)), and 2 NYCRR 125.1 accepts "a
			// verifiable login by the owner" as what stops that clock --
			// so the evidence has to outlive a rotated request log and a
			// swept sessions row, both long gone before three years
			// (#420). It is written here, and not at sign-in, because
			// this is the first point in a request where
			// app.current_practice_id is set, and that is the variable
			// staff_practice_visibility reads -- the only policy
			// admitting an UPDATE of a staff row.
			//
			// Stamped at most once a day: the question it answers is
			// three years wide, so a fresher timestamp buys nothing.
			if _, err := tx.ExecContext(r.Context(),
				`UPDATE staff
				 SET last_practice_id = $1,
				     last_active_at = CASE
				         WHEN last_active_at IS NULL OR last_active_at < now() - interval '1 day'
				         THEN now() ELSE last_active_at END
				 WHERE id = $2`, practiceID, staffID); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), staffIDKey, staffID)
			ctx = context.WithValue(ctx, practiceIDKey, practiceID)
			ctx = context.WithValue(ctx, txKey, tx)
			ctx = context.WithValue(ctx, readerKey, reader)
			ctx = tasknudge.Begin(ctx)

			next.ServeHTTP(w, r.WithContext(ctx))

			// ADR-0013: a handler behind this Middleware that queued an
			// outbox row registers a nudge closure on ctx rather than
			// firing it itself, because the response idempotency.Wrap
			// (or the handler itself) already wrote is cached below,
			// before this Commit runs. Draining only on a successful
			// commit is what keeps a nudge from firing for a write that
			// never actually landed.
			if err := tx.Commit(); err == nil {
				committed = true
				tasknudge.Drain(ctx)
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
// reads for the rest of the request -- then checks whether staffID holds
// a practice_memberships row for practiceID, and if so returns the roles
// and employment type that row holds and whether practiceID has thrown
// its "require MFA for all staff" switch (#606) -- one query, so this is
// also the one practice_memberships read a request needs: Middleware
// builds this request's Reader from what it returns, rather than a
// downstream handler querying the table again. Both session vars are set
// only here, after identity resolution has already succeeded, so no RLS
// policy that reads them can be satisfied before that check has passed.
func setPracticeAndCheckMembership(ctx context.Context, tx *sql.Tx, staffID, practiceID string) (isMember bool, roles []string, employmentType string, requireMFA bool, err error) {
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		return false, nil, "", false, fmt.Errorf("staffauth: set current practice id: %w", err)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_staff_id', $1, true)`, staffID); err != nil {
		return false, nil, "", false, fmt.Errorf("staffauth: set current staff id: %w", err)
	}

	var rolesText, employmentTypeText sql.NullString
	scanErr := tx.QueryRowContext(ctx,
		`SELECT array_to_string(pm.roles, ','), pm.employment_type::text, p.require_mfa_for_all_staff
		 FROM practices p
		 LEFT JOIN practice_memberships pm ON pm.practice_id = p.id AND pm.staff_id = $2
		 WHERE p.id = $1`,
		practiceID, staffID,
	).Scan(&rolesText, &employmentTypeText, &requireMFA)
	if errors.Is(scanErr, sql.ErrNoRows) {
		// No such Practice at all -- same outward result as no membership.
		return false, nil, "", false, nil
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if scanErr != nil {
		return false, nil, "", false, fmt.Errorf("staffauth: check practice membership: %w", scanErr)
	}
	if !rolesText.Valid {
		// The Practice exists but staffID holds no membership row there.
		return false, nil, "", requireMFA, nil
	}
	return true, splitRoles(rolesText.String), employmentTypeText.String, requireMFA, nil
}

// codeMFARequired is the machine-readable APIError.Code the app's
// credentialed fetch reads to route into enrolment rather than treating
// this as an ended session (#606's AC: "distinguishable from an ended
// session ... does not send the browser to the login screen").
const codeMFARequired = "MFA_REQUIRED"

// APIError is docs/api-design.md section 7's structured error shape,
// this package's own copy per this repo's convention (see
// portalinvite/errors.go, ratelimit.go).
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeMFARequired writes the Practice-scoped boundary's MFA refusal: a
// live, valid session that may not enter this Practice without a second
// factor. 403, not 401 -- the session itself is fine.
func writeMFARequired(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(APIError{
		Code:    codeMFARequired,
		Message: "this Practice requires a second sign-in factor",
	})
}
