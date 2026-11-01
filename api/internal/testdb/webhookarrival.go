package testdb

import (
	"testing"
	"time"
)

// WebhookArrival is one webhook_arrivals row as a test reads it, an
// absent event id or type read as "".
type WebhookArrival struct {
	Route     string
	Outcome   string
	EventID   string
	EventType string
	ArrivedAt time.Time
}

// WebhookArrivals reads every webhook_arrivals row through the
// superuser, oldest first: app_runtime may only insert one.
func WebhookArrivals(t *testing.T, db *DB) []WebhookArrival {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT route, outcome, coalesce(event_id, ''), coalesce(event_type, ''), arrived_at
		   FROM webhook_arrivals ORDER BY id`)
	if err != nil {
		// coverage:ignore reason: fixture read failure, not exercised by the happy-path test
		t.Fatalf("testdb: read webhook arrivals: %v", err)
	}
	defer func() { _ = rows.Close() }()
	out := []WebhookArrival{}
	for rows.Next() {
		var a WebhookArrival
		if err := rows.Scan(&a.Route, &a.Outcome, &a.EventID, &a.EventType, &a.ArrivedAt); err != nil {
			// coverage:ignore reason: fixture scan failure, not exercised by the happy-path test
			t.Fatalf("testdb: scan webhook arrival: %v", err)
		}
		a.ArrivedAt = a.ArrivedAt.UTC()
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: fixture iteration failure, not exercised by the happy-path test
		t.Fatalf("testdb: iterate webhook arrivals: %v", err)
	}
	return out
}
