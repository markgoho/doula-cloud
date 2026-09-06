// Package clientauth is the Client-portal-population auth middleware: it
// verifies the caller's Identity Platform ID token, resolves it against
// the client_portal_users table, and confirms the resulting Client holds
// the Engagement named by :engagementId in the URL -- setting
// app.current_client_id on a request-scoped transaction only once all of
// that has passed, so Postgres RLS enforces it for the rest of the
// request. Mirrors staffauth.Middleware's shape for the Staff population.
package clientauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
)

type contextKey string

const (
	clientIDKey     contextKey = "clientauth.clientID"
	engagementIDKey contextKey = "clientauth.engagementID"
	identityUIDKey  contextKey = "clientauth.identityUID"
	txKey           contextKey = "clientauth.tx"
)

// MsgInternalError is the response body for any failure the caller can't
// act on (a DB error, an encoding failure) -- deliberately vague so it
// never leaks internals. Defined here rather than reused from staffauth
// so the Client-portal population doesn't depend on the Staff-auth
// package for something population-agnostic; portal reuses this one
// rather than defining a third copy.
const MsgInternalError = "internal error"

// PortalPopulation is the shared OpenGet reason for every Client-portal
// read: ADR-0008's role table describes Staff at a Practice, and a Client
// holds no Membership to check against, so there is nothing for a role
// declaration to be about. Every package that mounts a portal GET reaches
// for this one string rather than defining its own copy, so a reader
// scanning any of them recognizes the same reason.
const PortalPopulation = "clientauth.Middleware, not staffauth -- a Client holds no Membership, so ADR-0008's read table has nothing to say about this route"

// ClientID returns the resolved Client id for the current request.
func ClientID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(clientIDKey).(string)
	return id, ok
}

// EngagementID returns the :engagementId the current request is scoped
// to.
func EngagementID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(engagementIDKey).(string)
	return id, ok
}

// IdentityUID returns the caller's verified Identity Platform uid -- the
// Portal Account itself (ADR-0015: "the person lives in the login"),
// distinct from ClientID, which is this one Practice's record of her.
// notificationpref keys #303's preference store on this rather than
// ClientID so the row names the login, not the Practice-scoped Client
// record it happens to be read through.
func IdentityUID(ctx context.Context) (string, bool) {
	uid, ok := ctx.Value(identityUIDKey).(string)
	return uid, ok
}

// Tx returns the request-scoped database transaction, with
// app.current_client_id already set for the life of the transaction.
// Handlers must run any query touching a client-tier-RLS table through
// this transaction, not a fresh connection, or RLS will have nothing to
// scope against.
func Tx(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txKey).(*sql.Tx)
	return tx, ok
}

// Middleware wraps an http.Handler with Client-portal population
// resolution and Engagement-ownership authorization. db must be a
// connection using the low-privilege app_runtime role -- the role the RLS
// policies in 00006_client_portal_users.sql apply to.
func Middleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tx, uid, _, ok := authn.Begin(w, r, db)
			if !ok {
				return
			}
			committed := false
			defer func() {
				if !committed {
					_ = tx.Rollback()
				}
			}()

			engagementID := r.PathValue("engagementId")
			if _, err := uuid.Parse(engagementID); err != nil {
				apierr.WriteError(w, "invalid engagement id", http.StatusBadRequest)
				return
			}

			clientID, owns, err := resolveOwningClient(r.Context(), tx, uid, engagementID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
			if !owns {
				apierr.WriteError(w, "engagement not linked to this client", http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), clientIDKey, clientID)
			ctx = context.WithValue(ctx, engagementIDKey, engagementID)
			ctx = context.WithValue(ctx, identityUIDKey, uid)
			ctx = context.WithValue(ctx, txKey, tx)

			next.ServeHTTP(w, r.WithContext(ctx))

			if err := tx.Commit(); err == nil {
				committed = true
			}
		})
	}
}

// resolveOwningClient sets app.current_identity_uid, then finds which of
// identityUID's client_portal_users rows owns engagementID. Before #309
// there was exactly one row per identity_uid, so resolving the identity
// alone was enough; #309 lifted that constraint (ADR-0015: a Portal
// Account reaches many Clients, one per Practice), so a Client-portal
// caller who holds access at more than one Practice can now have more
// than one candidate row here. Trying each until one owns the named
// Engagement, rather than picking the first arbitrarily, is what keeps a
// request for Practice B's Engagement from being refused because the
// first row Postgres happened to return was Practice A's.
func resolveOwningClient(ctx context.Context, tx *sql.Tx, identityUID, engagementID string) (clientID string, owns bool, err error) {
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		return "", false, fmt.Errorf("clientauth: set current identity uid: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `SELECT client_id FROM client_portal_users WHERE identity_uid = $1`, identityUID)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return "", false, fmt.Errorf("clientauth: list clients for identity: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var candidates []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return "", false, fmt.Errorf("clientauth: scan client id: %w", err)
		}
		candidates = append(candidates, id)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return "", false, fmt.Errorf("clientauth: iterate client rows: %w", err)
	}

	for _, candidate := range candidates {
		owns, err := setClientAndCheckEngagement(ctx, tx, candidate, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return "", false, err
		}
		if owns {
			return candidate, true, nil
		}
	}
	return "", false, nil
}

// setIdentityAndResolveClient sets app.current_identity_uid -- the
// session variable client_portal_users' self-visibility RLS policy
// reads, since app.current_client_id isn't known yet -- then looks up
// identityUID in client_portal_users.
func setIdentityAndResolveClient(ctx context.Context, tx *sql.Tx, identityUID string) (string, bool, error) {
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		return "", false, fmt.Errorf("clientauth: set current identity uid: %w", err)
	}

	var clientID string
	err := tx.QueryRowContext(ctx, `SELECT client_id FROM client_portal_users WHERE identity_uid = $1`, identityUID).Scan(&clientID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return "", false, fmt.Errorf("clientauth: resolve client: %w", err)
	}
	return clientID, true, nil
}

// setClientAndCheckEngagement sets app.current_client_id -- the session
// variable client-tier RLS reads for the rest of the request -- then
// checks whether engagementID belongs to clientID.
func setClientAndCheckEngagement(ctx context.Context, tx *sql.Tx, clientID, engagementID string) (bool, error) {
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		return false, fmt.Errorf("clientauth: set current client id: %w", err)
	}

	var exists bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM engagements WHERE id = $1 AND client_id = $2)`,
		engagementID, clientID,
	).Scan(&exists)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return false, fmt.Errorf("clientauth: check engagement ownership: %w", err)
	}
	return exists, nil
}
