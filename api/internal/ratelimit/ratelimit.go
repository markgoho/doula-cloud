// Package ratelimit is docs/api-design.md section 6's defensive rate
// limiting, applied as a decorator a handler wraps in -- the same shape
// idempotency.Wrap and staffauth/clientauth.Middleware already use.
//
// Counters live in Postgres (rate_limit_buckets, 00060) rather than in
// process memory: Cloud Run scales the BFF to more than one instance, so
// an in-process counter would not actually limit anything (ADR-0004
// already made this call for sessions and idempotency keys). Wrap runs
// on its own short query against db, not the caller's own transaction --
// every endpoint this package fronts (staff signup, login, staff/portal
// invitation acceptance, the pre-account Offer routes) opens its own
// transaction after Wrap has already decided whether to let the request
// through, so there is no shared transaction to join.
package ratelimit

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"doula-cloud/api/internal/apierr"
)

// msgInternalError is the response body for a failure the caller can't
// act on -- deliberately vague, this package's own copy per this repo's
// convention (see portalinvite/errors.go).
const msgInternalError = "internal error"

// Rule is one dimension a request is checked and counted against,
// independently of every other Rule passed to Wrap alongside it. An
// endpoint combines more than one so that evading any single dimension --
// rotating IP addresses, or replaying a stolen credential from a fresh
// address -- still runs into another (docs/api-design.md ties this to
// "at least the subject and the client IP").
type Rule struct {
	// Dimension names this Rule in the bucket key and in a refusal's
	// logged row -- "ip", "token", "offerId", etc.
	Dimension string
	// Key extracts this Rule's value for the current request. ok is
	// false when the dimension does not apply to this request (no
	// Bearer token present, say), in which case this Rule is skipped
	// entirely rather than counted against an empty key.
	Key func(r *http.Request) (key string, ok bool)
	// Max is how many requests within Window this key may make before
	// Wrap refuses the next one.
	Max int
	// Window is the rolling period Max applies over.
	Window time.Duration
}

// Wrap enforces every rule in rules against db, keyed per rule by
// endpoint plus the rule's own Dimension and extracted key. A request
// that exceeds any single rule is refused with 429, a Retry-After header,
// and docs/api-design.md section 7's structured error body; the refusal
// is also recorded in rate_limit_refusals (00060) so repeated refusals
// against one address can be seen after the fact. Rules are checked in
// order and short-circuit on the first breach -- a rule after the
// breaching one is not touched for this request, which only means it
// under-counts a request that was refused anyway on another dimension.
func Wrap(db *sql.DB, endpoint string, rules []Rule) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// tightestLimit/tightestRemaining track the rule closest to its
			// own cap, across every rule this request is checked against --
			// docs/api-design.md section 6's RateLimit-Limit/-Remaining
			// headers report that one, the same way a multi-dimension limit
			// is only as generous as its tightest dimension.
			tightestLimit, tightestRemaining := 0, -1

			for _, rule := range rules {
				key, ok := rule.Key(r)
				if !ok {
					continue
				}

				count, windowStart, err := touch(r.Context(), db, bucketKey(endpoint, rule.Dimension, key), rule.Window)
				if err != nil {
					// coverage:ignore reason: DB query failure, not exercised by unit tests
					apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, msgInternalError, nil)
					return
				}

				if count > rule.Max {
					retryAfter := retryAfterSeconds(windowStart, rule.Window)
					// coverage:ignore reason: DB query failure, not exercised by unit tests
					if err := recordRefusal(r.Context(), db, endpoint, rule.Dimension, key); err != nil {
						// A failed refusal log must not itself turn a refusal
						// into a 500 -- the caller already isn't getting
						// through, and the log is for investigation, not
						// enforcement. Logged, not surfaced.
						log.Printf("ratelimit: record refusal: %v", err)
					}
					w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
					w.Header().Set("RateLimit-Limit", strconv.Itoa(rule.Max))
					w.Header().Set("RateLimit-Remaining", "0")
					apierr.Write(w, http.StatusTooManyRequests, apierr.CodeRateLimited,
						fmt.Sprintf("too many requests -- try again in %d seconds", retryAfter), nil)
					return
				}

				remaining := rule.Max - count
				if tightestRemaining < 0 || remaining < tightestRemaining {
					tightestLimit, tightestRemaining = rule.Max, remaining
				}
			}

			if tightestRemaining >= 0 {
				w.Header().Set("RateLimit-Limit", strconv.Itoa(tightestLimit))
				w.Header().Set("RateLimit-Remaining", strconv.Itoa(tightestRemaining))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// bucketKey namespaces a rate_limit_buckets row by the endpoint and rule
// it belongs to, so the same email or IP is counted separately per
// endpoint -- hammering signup does not spend login's budget.
func bucketKey(endpoint, dimension, key string) string {
	return endpoint + ":" + dimension + ":" + key
}

// touch atomically increments the bucket named key for the current
// window, starting a fresh window (count reset to 1) if the stored one
// has rolled off. The whole read-modify-write happens in one statement so
// two requests racing on the same key -- across Cloud Run instances, not
// just goroutines -- still serialize on the row rather than both reading
// a stale count.
//
// No reaper deletes an old bucket: a key nobody re-touches is a few idle
// bytes, and the CASE above resets it in place the moment anyone does,
// the same trade idempotency.go's TTL comment documents for
// idempotency_keys.
func touch(ctx context.Context, db *sql.DB, key string, window time.Duration) (count int, windowStart time.Time, err error) {
	err = db.QueryRowContext(ctx,
		`INSERT INTO rate_limit_buckets (key, window_start, count)
		 VALUES ($1, now(), 1)
		 ON CONFLICT (key) DO UPDATE SET
		     count = CASE WHEN rate_limit_buckets.window_start <= now() - make_interval(secs => $2)
		                  THEN 1 ELSE rate_limit_buckets.count + 1 END,
		     window_start = CASE WHEN rate_limit_buckets.window_start <= now() - make_interval(secs => $2)
		                  THEN now() ELSE rate_limit_buckets.window_start END
		 RETURNING count, window_start`,
		key, window.Seconds(),
	).Scan(&count, &windowStart)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, time.Time{}, fmt.Errorf("ratelimit: touch bucket %q: %w", key, err)
	}
	return count, windowStart, nil
}

// recordRefusal appends one rate_limit_refusals row (00060) for a request
// this rule just turned away.
func recordRefusal(ctx context.Context, db *sql.DB, endpoint, dimension, key string) error {
	if _, err := db.ExecContext(ctx,
		`INSERT INTO rate_limit_refusals (endpoint, dimension, key_value) VALUES ($1, $2, $3)`,
		endpoint, dimension, key,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("ratelimit: record refusal: %w", err)
	}
	return nil
}

// retryAfterSeconds is how long the caller must wait for windowStart's
// bucket to roll off, rounded up so a caller retrying at exactly this
// value never arrives a moment too early.
func retryAfterSeconds(windowStart time.Time, window time.Duration) int {
	remaining := int(math.Ceil(time.Until(windowStart.Add(window)).Seconds()))
	if remaining < 1 {
		// coverage:ignore reason: only reachable on clock skew between this
		// process and Postgres, not exercised by unit tests
		return 1
	}
	return remaining
}
