// Package webhookarrival records one webhook_arrivals row for every
// request to doula-api's four webhook routes, refused ones included, and
// sweeps the rows after 90 days (ADR-0062, #2473). No payload is stored.
//
// The row is written after the handler returns, so any transaction the
// handler opened has already released its pool connection (#2432).
package webhookarrival

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"doula-cloud/api/internal/clock"
)

// Route names one of the four webhook routes.
type Route string

// The four routes, as webhook_arrivals.route stores them.
const (
	StripePlatform Route = "stripe_platform"
	StripeConnect  Route = "stripe_connect"
	StripeAccount  Route = "stripe_account"
	Mailgun        Route = "mailgun"
)

// Outcome is what became of one request.
type Outcome string

// The outcomes, as webhook_arrivals.outcome stores them.
const (
	Accepted         Outcome = "accepted"
	Dropped          Outcome = "dropped"
	BodyRefused      Outcome = "body_refused"
	SignatureRefused Outcome = "signature_refused"
	LivemodeRefused  Outcome = "livemode_refused"
	Refused          Outcome = "refused"
	Failed           Outcome = "failed"
)

// Retention is how long a row is kept before the sweep deletes it.
const Retention = 90 * 24 * time.Hour

const (
	maxPeekBytes      = 1 << 20
	maxEventIDTypeLen = 255
)

type markKey struct{}

// Mark names the outcome of the request ctx belongs to, overriding the
// one Record would read off the status. Outside Record it does nothing.
func Mark(ctx context.Context, outcome Outcome) {
	if m, ok := ctx.Value(markKey{}).(*Outcome); ok {
		*m = outcome
	}
}

// Record wraps a webhook route's handler so each request writes one
// arrival row for route.
func Record(db *sql.DB, route Route, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrivedAt := clock.Now(r.Context())
		peek, _ := io.ReadAll(io.LimitReader(r.Body, maxPeekBytes+1))
		r.Body = readCloser{io.MultiReader(bytes.NewReader(peek), r.Body), r.Body}
		eventID, eventType := parseEvent(route, peek)

		var marked Outcome
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), markKey{}, &marked)))

		outcome := marked
		if outcome == "" {
			outcome = outcomeOf(rec.status)
		}
		if _, err := db.ExecContext(context.WithoutCancel(r.Context()),
			`INSERT INTO webhook_arrivals (route, outcome, event_id, event_type, arrived_at)
			 VALUES ($1, $2, $3, $4, $5)`,
			string(route), string(outcome), nullable(eventID), nullable(eventType), arrivedAt,
		); err != nil {
			log.Printf("webhookarrival: record %s arrival (%s): %v", route, outcome, err)
		}
	})
}

func outcomeOf(status int) Outcome {
	switch {
	case status == 0 || status < http.StatusBadRequest:
		return Accepted
	case status < http.StatusInternalServerError:
		return Refused
	default:
		return Failed
	}
}

func parseEvent(route Route, body []byte) (id, typ string) {
	if len(body) > maxPeekBytes {
		return "", ""
	}
	if route == Mailgun {
		var p struct {
			EventData struct {
				ID    string `json:"id"`
				Event string `json:"event"`
			} `json:"event-data"`
		}
		if json.Unmarshal(body, &p) != nil {
			return "", ""
		}
		return bounded(p.EventData.ID), bounded(p.EventData.Event)
	}
	var p struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if json.Unmarshal(body, &p) != nil {
		return "", ""
	}
	return bounded(p.ID), bounded(p.Type)
}

func bounded(s string) string {
	if len(s) > maxEventIDTypeLen {
		return ""
	}
	return s
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

type readCloser struct {
	io.Reader
	io.Closer
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	if s.status == 0 {
		s.status = status
	}
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b) //nolint:wrapcheck // a ResponseWriter passes its own error through
}

// Sweeper deletes every arrival older than Retention. It is an
// outbox.Processor so the drain's one Cloud Scheduler tick runs it, as it
// runs idempotency.Sweeper; sweep_webhook_arrivals refuses a transaction
// without the drain's worker door.
type Sweeper struct{}

// ProcessPending deletes every row that arrived before clock.Now(ctx)
// less Retention.
func (Sweeper) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `SELECT sweep_webhook_arrivals($1)`, clock.Now(ctx).Add(-Retention)); err != nil {
		return fmt.Errorf("webhookarrival: sweep: %w", err)
	}
	return nil
}
