// Package idempotency implements docs/api-design.md section 3's
// Idempotency-Key contract as a single reusable decorator: replay a
// previously stored response for a key already seen in the caller's own
// scope, or run the handler and persist its result when a key is present.
//
// A caller is one of two populations, and each has its own decorator:
// Wrap for a Staff member, mounted inside staffauth.Middleware, and
// WrapClient for a Client, mounted inside clientauth.Middleware (#1924).
// Each reads the request-scoped tx and caller id its Middleware sets, and
// relies on idempotency_keys' RLS policy
// (00129_idempotency_keys_client_scope.sql) to enforce that scope at the
// database layer too.
package idempotency

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/staffauth"
)

const headerName = "Idempotency-Key"

// maxKeyLength bounds the caller-chosen key so an unbounded header value
// can't be used to bloat the table. A key over this length is treated the
// same as no key at all -- the request still runs, just without
// idempotency protection -- since the header is optional and a caller
// sending an oversized key shouldn't have their request rejected outright.
const maxKeyLength = 255

// ttl is docs/api-design.md section 3's "appropriate TTL (typically 24-48
// hours)" for a stored key. A row past this age is treated as not found,
// and Sweeper deletes it on the outbox drain's next tick (#1987), so a
// stored response does not outlive the window it exists for. Between the
// two, lookup's own created_at check is what honors the TTL, and save
// refreshes a row the sweep has not reached yet.
//
// All three read the instant from clock.Now(ctx), never SQL now(), and
// draw the line through expiredBefore (#1991): a row is live when
// created_at >= the cutoff and swept when created_at < it, so a moved
// clock moves the lookup, the save and the sweep together.
const ttl = 48 * time.Hour

// expiredBefore is the cutoff a row created before is past ttl at now --
// the one line lookup, save and Sweeper all draw.
func expiredBefore(now time.Time) time.Time {
	return now.Add(-ttl)
}

// scope is whose key a request carries: a Staff member at a Practice, or a
// Client, whose Practice is the one her own clients row belongs to.
// Exactly one of staffID and clientID is set, the same rule
// idempotency_keys_one_caller holds the table to.
type scope struct {
	practiceID string
	staffID    sql.NullString
	clientID   sql.NullString
}

// Wrap runs next unmodified when no usable Idempotency-Key header is
// present. When a key is present, it replays a stored response for that
// key scoped to the caller's own Practice and Staff member, or runs next
// and persists the result (when the response status is not a server
// error) for the next request carrying the same key. Must be mounted
// inside staffauth.Middleware.
func Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, ok := usableKey(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		serve(w, r, next, tx, key, scope{practiceID: practiceID, staffID: sql.NullString{String: staffID, Valid: true}})
	})
}

// WrapClient is Wrap for the Client-portal population (#1924): the same
// replay-or-run, with the key scoped to the calling Client and her own
// Practice. Must be mounted inside clientauth.Middleware, which sets the
// tx and the Client id this reads. clientauth.Middleware sets no Practice
// id, so the Practice is read from her own clients row on that tx --
// clients_self_visibility (00009) lets her session read it, and
// idempotency_keys_own_scope checks the same pairing again on every read
// and write.
func WrapClient(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, ok := usableKey(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		tx, hasTx := clientauth.Tx(r.Context())
		clientID, hasClient := clientauth.ClientID(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx and a Client id before this handler runs
		if !hasTx || !hasClient {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var practiceID string
		if err := tx.QueryRowContext(r.Context(), `SELECT practice_id FROM clients WHERE id = $1`, clientID).Scan(&practiceID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		serve(w, r, next, tx, key, scope{practiceID: practiceID, clientID: sql.NullString{String: clientID, Valid: true}})
	})
}

// usableKey returns the request's Idempotency-Key, and false when there is
// none or it is too long to store.
func usableKey(r *http.Request) (string, bool) {
	key := r.Header.Get(headerName)
	if key == "" || len(key) > maxKeyLength {
		return "", false
	}
	return key, true
}

// serve is the replay-or-run both Wrap and WrapClient share once each has
// resolved its caller's scope.
func serve(w http.ResponseWriter, r *http.Request, next http.Handler, tx *sql.Tx, key string, sc scope) {
	found, status, body, err := lookup(r.Context(), tx, key, sc)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	if found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		//nolint:gosec // replaying a body this same request scope already wrote as JSON on the initial call, not attacker-controlled markup
		_, _ = w.Write(body)
		return
	}

	rec := &recorder{ResponseWriter: w, status: http.StatusOK}
	next.ServeHTTP(rec, r)

	// A 5xx means the initial execution failed -- caching it would
	// permanently wedge this key, since the caller would never be able to
	// retry past a transient failure. Only successful and
	// deterministic-error (4xx) outcomes are worth replaying.
	if rec.status < 500 {
		// Copied onto a non-nil slice: a handler that writes no body (a
		// 204) leaves Bytes() nil, which the driver sends as NULL into
		// response_body's NOT NULL -- so an empty response was never
		// stored, and its retry re-ran instead of replaying.
		stored := append([]byte{}, rec.body.Bytes()...)
		if err := save(r.Context(), tx, key, sc, rec.status, stored); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			//
			// The response has already been written to the caller at this
			// point, so a storage failure can't change what they receive --
			// it just means the next retry with this key re-runs the
			// handler instead of replaying. Logged, not surfaced.
			log.Printf("idempotency: store response for key: %v", err)
		}
	}
}

// lookup returns the stored status code and response body for key, if one
// was already persisted for the caller's own scope within ttl of
// clock.Now(ctx). A row older than ttl is treated as not found, honoring
// docs/api-design.md section 3's TTL until Sweeper deletes it. IS NOT
// DISTINCT FROM compares the caller column a scope leaves NULL as equal
// to the row's NULL, the same reading idempotency_keys_scope's NULLS NOT
// DISTINCT gives it.
func lookup(ctx context.Context, tx *sql.Tx, key string, sc scope) (found bool, status int, body []byte, err error) {
	err = tx.QueryRowContext(ctx,
		`SELECT status_code, response_body FROM idempotency_keys
		 WHERE key = $1 AND practice_id = $2
		   AND staff_id IS NOT DISTINCT FROM $3::uuid AND client_id IS NOT DISTINCT FROM $4::uuid
		   AND created_at >= $5`,
		key, sc.practiceID, sc.staffID, sc.clientID, expiredBefore(clock.Now(ctx)),
	).Scan(&status, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return false, 0, nil, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, 0, nil, fmt.Errorf("idempotency: look up key: %w", err)
	}
	return true, status, body, nil
}

// save persists status and body against key, in the caller's scope. On
// conflict with an existing row still within ttl, the insert no-ops -- a
// concurrent retry racing this same insert loses harmlessly, whichever
// write lands first is what future replays return. On conflict with a row
// past ttl (lookup already treats it as not-found, so the handler just
// re-ran), the row is refreshed in place instead of staying stale
// forever, since ON CONFLICT DO NOTHING would otherwise silently drop
// every save attempt against that key going forward.
//
// created_at is written from clock.Now(ctx), on the insert as well as the
// refresh, rather than left to the column's DEFAULT now(): the row's age
// is read against the same clock lookup and Sweeper read.
func save(ctx context.Context, tx *sql.Tx, key string, sc scope, status int, body []byte) error {
	now := clock.Now(ctx)
	_, err := tx.ExecContext(ctx,
		`INSERT INTO idempotency_keys (key, practice_id, staff_id, client_id, status_code, response_body, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (key, practice_id, staff_id, client_id) DO UPDATE
		 SET status_code = EXCLUDED.status_code,
		     response_body = EXCLUDED.response_body,
		     created_at = EXCLUDED.created_at
		 WHERE idempotency_keys.created_at < $8`,
		key, sc.practiceID, sc.staffID, sc.clientID, status, body, now, expiredBefore(now),
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("idempotency: save response for key: %w", err)
	}
	return nil
}

// recorder captures the status code and body next writes while still
// passing both through to the real ResponseWriter, so the caller's first
// request gets the real response and Wrap gets a copy to persist.
type recorder struct {
	http.ResponseWriter
	status      int
	body        bytes.Buffer
	wroteHeader bool
}

func (rec *recorder) WriteHeader(status int) {
	rec.status = status
	rec.wroteHeader = true
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *recorder) Write(b []byte) (int, error) {
	if !rec.wroteHeader {
		rec.WriteHeader(http.StatusOK)
	}
	rec.body.Write(b)
	return rec.ResponseWriter.Write(b) //nolint:wrapcheck // implements http.ResponseWriter; callers expect the raw net/http error, not a wrapped one
}
