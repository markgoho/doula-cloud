package testdb_test

import (
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

func TestWebhookArrivals(t *testing.T) {
	db := testdb.New(t)
	at := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO webhook_arrivals (route, outcome, event_id, event_type, arrived_at)
		 VALUES ('mailgun', 'accepted', 'mg_1', 'failed', $1), ('stripe_platform', 'refused', NULL, NULL, $1)`, at,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	want := []testdb.WebhookArrival{
		{Route: "mailgun", Outcome: "accepted", EventID: "mg_1", EventType: "failed", ArrivedAt: at},
		{Route: "stripe_platform", Outcome: "refused", ArrivedAt: at},
	}
	if got := testdb.WebhookArrivals(t, db); !slices.Equal(got, want) {
		t.Fatalf("arrivals = %+v, want %+v", got, want)
	}
}
