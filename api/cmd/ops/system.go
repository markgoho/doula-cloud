package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/stripemode"
)

const (
	systemWindowDays = 7
	// Stripe's own list page size; a count that reaches it is a lower bound.
	undeliveredCountLimit = 100
	// The allowlist is a handful of addresses in the pilot; the cap keeps a
	// page bounded and the total says how many there are.
	allowlistShown = 200
)

// The four webhook routes, in the order the screen lists them.
const (
	routeStripePlatform = "stripe_platform"
	routeStripeConnect  = "stripe_connect"
	routeStripeAccount  = "stripe_account"
)

type undeliveredEvents struct {
	Count   int  `json:"count"`
	AtLeast bool `json:"atLeast"`
}

// PlatformAccountID and Undelivered are null when Stripe could not be read.
type stripeView struct {
	Mode              string             `json:"mode"`
	PlatformAccountID *string            `json:"platformAccountId"`
	Undelivered       *undeliveredEvents `json:"undelivered"`
}

type webhookRefusals struct {
	Signature int `json:"signature"`
	Livemode  int `json:"livemode"`
	Body      int `json:"body"`
	Payload   int `json:"payload"`
	Other     int `json:"other"`
	Failed    int `json:"failed"`
}

// LastArrivalAt is the last verified request; a refused one never moves it.
type webhookRoute struct {
	Route         string          `json:"route"`
	LastArrivalAt *time.Time      `json:"lastArrivalAt"`
	Refusals      webhookRefusals `json:"refusals"`
}

// LastArrivalAt is null for a handled type not seen in the 90 days the arrivals are kept.
type eventTypeView struct {
	Route         string     `json:"route"`
	Type          string     `json:"type"`
	LastArrivalAt *time.Time `json:"lastArrivalAt"`
}

// OldestDueAt is when the oldest pending row that is due now came due, null when none is.
type outboxView struct {
	Name         string     `json:"name"`
	Pending      int        `json:"pending"`
	DeadLettered int        `json:"deadLettered"`
	OldestDueAt  *time.Time `json:"oldestDueAt"`
}

// Both are null while the gate is closed; OpenedBy is also null for a gate opened before openings named an operator.
type gateView struct {
	OpenedAt *time.Time `json:"openedAt"`
	OpenedBy *string    `json:"openedBy"`
}

type allowlistEntry struct {
	Address string    `json:"address"`
	AddedAt time.Time `json:"addedAt"`
	AddedBy *string   `json:"addedBy"`
}

type allowlistView struct {
	Total int              `json:"total"`
	Items []allowlistEntry `json:"items"`
}

type pilotView struct {
	Gate      gateView      `json:"gate"`
	Allowlist allowlistView `json:"allowlist"`
}

type systemView struct {
	Stripe     stripeView      `json:"stripe"`
	Webhooks   []webhookRoute  `json:"webhooks"`
	EventTypes []eventTypeView `json:"eventTypes"`
	Outboxes   []outboxView    `json:"outboxes"`
	Pilot      pilotView       `json:"pilot"`
}

// It reads service facts and the pilot lists, no Practice's records and no Client's, so it writes no read row.
// Stripe is asked first, with no database connection held, so the one connection this takes is for the reads.
func systemRead(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := clock.Now(r.Context())
		since := now.AddDate(0, 0, -systemWindowDays)
		view := systemView{Stripe: readStripe(r.Context(), d.Stripe, d.StripeMode, since)}

		tx, err := d.DB.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true})
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		defer func() { _ = tx.Rollback() }()
		if err := readSystemDatabase(r.Context(), tx, now, since, &view); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, view)
	}
}

func readStripe(ctx context.Context, client billing.StripeClient, mode stripemode.Mode, since time.Time) stripeView {
	view := stripeView{Mode: string(mode)}
	if id, err := client.PlatformAccountID(ctx); err != nil {
		log.Printf("doula-ops: system: platform account: %v", err)
	} else {
		view.PlatformAccountID = &id
	}
	if n, err := client.UndeliveredEventCount(ctx, since, undeliveredCountLimit); err != nil {
		log.Printf("doula-ops: system: undelivered events: %v", err)
	} else {
		view.Undelivered = &undeliveredEvents{Count: n, AtLeast: n >= undeliveredCountLimit}
	}
	return view
}

// into names the columns of one row, in the order the query selects them.
func queryAll[T any](ctx context.Context, tx *sql.Tx, into func(*T) []any, query string, args ...any) ([]T, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return nil, fmt.Errorf("read system: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := []T{}
	for rows.Next() {
		var item T
		if err := rows.Scan(into(&item)...); err != nil {
			// coverage:ignore reason: scan failure on a well-typed query, not exercised by unit tests
			return nil, fmt.Errorf("scan system: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return nil, fmt.Errorf("read system: %w", err)
	}
	return items, nil
}

func readSystemDatabase(ctx context.Context, tx *sql.Tx, now, since time.Time, view *systemView) error {
	var err error
	view.Webhooks, err = queryAll(ctx, tx, func(r *webhookRoute) []any {
		return []any{&r.Route, &r.LastArrivalAt, &r.Refusals.Signature, &r.Refusals.Livemode, &r.Refusals.Body, &r.Refusals.Payload, &r.Refusals.Other, &r.Refusals.Failed}
	}, `SELECT route, last_arrival_at, signature_refused, livemode_refused, body_refused, payload_refused, refused, failed
	      FROM ops_webhook_routes($1)
	     ORDER BY array_position(ARRAY['stripe_platform', 'stripe_connect', 'stripe_account', 'mailgun'], route)`, since)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return err
	}

	seen, err := queryAll(ctx, tx, func(e *eventTypeView) []any { return []any{&e.Route, &e.Type, &e.LastArrivalAt} }, `SELECT route, event_type, last_arrival_at FROM ops_webhook_event_types()`)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return err
	}
	view.EventTypes = handledEventTypes(seen)

	view.Outboxes, err = queryAll(ctx, tx, func(o *outboxView) []any { return []any{&o.Name, &o.Pending, &o.DeadLettered, &o.OldestDueAt} }, `SELECT outbox, pending, dead_lettered, oldest_due_at FROM ops_outbox_status($1)`, now)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return err
	}

	gates, err := queryAll(ctx, tx, func(g *gateView) []any { return []any{&g.OpenedAt, &g.OpenedBy} }, `SELECT opened_at, opened_by FROM ops_pilot_gate()`)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return err
	}
	view.Pilot.Gate = gates[0]

	view.Pilot.Allowlist.Items, err = queryAll(ctx, tx, func(e *allowlistEntry) []any {
		return []any{&e.Address, &e.AddedAt, &e.AddedBy, &view.Pilot.Allowlist.Total}
	}, `SELECT address, added_at, added_by, total FROM ops_pilot_allowlist($1)`, allowlistShown)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return err
	}
	return nil
}

// The handled types of each Stripe route, each with its last verified arrival, or none.
func handledEventTypes(seen []eventTypeView) []eventTypeView {
	last := map[[2]string]*time.Time{}
	for _, e := range seen {
		last[[2]string{e.Route, e.Type}] = e.LastArrivalAt
	}
	handled := []struct {
		route string
		types []string
	}{
		{routeStripePlatform, billing.HandledEventTypes()},
		{routeStripeConnect, payments.ConnectEventTypes()},
		{routeStripeAccount, payments.AccountEventTypes()},
	}
	out := []eventTypeView{}
	for _, h := range handled {
		for _, typ := range h.types {
			out = append(out, eventTypeView{Route: h.route, Type: typ, LastArrivalAt: last[[2]string{h.route, typ}]})
		}
	}
	return out
}
