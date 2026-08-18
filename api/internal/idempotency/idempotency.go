// Package idempotency implements docs/api-design.md section 3's
// Idempotency-Key contract as a single reusable decorator: replay a
// previously stored response for a key already seen in the caller's own
// scope, or run the handler and persist its result when a key is present.
// Must be mounted inside staffauth.Middleware -- it reads the
// request-scoped tx, Practice id, and Staff id Middleware sets, and relies
// on idempotency_keys' RLS policy (00027_idempotency_keys.sql) to enforce
// that scope at the database layer too.
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
// hours)" for a stored key. A row past this age is treated as not found --
// the request runs normally and overwrites nothing, since the original row
// is still there for save's ON CONFLICT to no-op against -- so the
// created_at column alone is enough to honor the TTL for v1 without a
// reaper/sweep job actually deleting anything.
const ttl = 48 * time.Hour

// msgInternalError is this package's own copy of the vague internal-error
// message, per this repo's convention of small per-package copies (see
// portalinvite/errors.go).
const msgInternalError = "internal error"

// Wrap runs next unmodified when no usable Idempotency-Key header is
// present. When a key is present, it replays a stored response for that
// key scoped to the caller's own Practice and Staff member, or runs next
// and persists the result (when the response status is not a server
// error) for the next request carrying the same key.
func Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(headerName)
		if key == "" || len(key) > maxKeyLength {
			next.ServeHTTP(w, r)
			return
		}

		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		found, status, body, err := lookup(r.Context(), tx, key, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, msgInternalError, http.StatusInternalServerError)
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
		// permanently wedge this key, since the caller would never be
		// able to retry past a transient failure. Only successful and
		// deterministic-error (4xx) outcomes are worth replaying.
		if rec.status < 500 {
			if err := save(r.Context(), tx, key, practiceID, staffID, rec.status, rec.body.Bytes()); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				//
				// The response has already been written to the caller at
				// this point, so a storage failure can't change what they
				// receive -- it just means the next retry with this key
				// re-runs the handler instead of replaying. Logged, not
				// surfaced.
				log.Printf("idempotency: store response for key: %v", err)
			}
		}
	})
}

// lookup returns the stored status code and response body for key, if one
// was already persisted for the caller's own practiceID/staffID scope
// within ttl. A row older than ttl is treated as not found, honoring
// docs/api-design.md section 3's TTL without a reaper job actually
// deleting it.
func lookup(ctx context.Context, tx *sql.Tx, key, practiceID, staffID string) (found bool, status int, body []byte, err error) {
	err = tx.QueryRowContext(ctx,
		`SELECT status_code, response_body FROM idempotency_keys
		 WHERE key = $1 AND practice_id = $2 AND staff_id = $3
		   AND created_at >= now() - make_interval(secs => $4)`,
		key, practiceID, staffID, ttl.Seconds(),
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

// save persists status and body against key, scoped to practiceID and
// staffID. On conflict with an existing row still within ttl, the insert
// no-ops -- a concurrent retry racing this same insert loses harmlessly,
// whichever write lands first is what future replays return. On conflict
// with a row past ttl (lookup already treats it as not-found, so the
// handler just re-ran), the row is refreshed in place instead of staying
// stale forever, since ON CONFLICT DO NOTHING would otherwise silently
// drop every save attempt against that key going forward.
func save(ctx context.Context, tx *sql.Tx, key, practiceID, staffID string, status int, body []byte) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO idempotency_keys (key, practice_id, staff_id, status_code, response_body)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (key, practice_id, staff_id) DO UPDATE
		 SET status_code = EXCLUDED.status_code,
		     response_body = EXCLUDED.response_body,
		     created_at = now()
		 WHERE idempotency_keys.created_at < now() - make_interval(secs => $6)`,
		key, practiceID, staffID, status, body, ttl.Seconds(),
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
