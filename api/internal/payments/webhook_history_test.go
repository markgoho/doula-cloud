package payments_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

// historyRow is one stripe_connect_status_history row as a test reads it
// back (#1812).
type historyRow struct {
	eventID         string
	cardPayments    string
	payouts         string
	requirementsDue []string
	eventCreatedAt  time.Time
	processedAt     time.Time
}

// connectHistory reads every history row for practiceID in the order the
// webhook applied them, through the superuser Admin connection.
func connectHistory(t *testing.T, db *testdb.DB, practiceID string) []historyRow {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT stripe_event_id, card_payments_status, payouts_status,
			array_to_json(requirements_due)::text, stripe_event_created_at, processed_at
		FROM stripe_connect_status_history WHERE practice_id = $1 ORDER BY processed_at, id`, practiceID,
	)
	if err != nil {
		t.Fatalf("query history: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []historyRow
	for rows.Next() {
		var h historyRow
		var requirementsJSON string
		if err := rows.Scan(&h.eventID, &h.cardPayments, &h.payouts, &requirementsJSON, &h.eventCreatedAt, &h.processedAt); err != nil {
			t.Fatalf("scan history: %v", err)
		}
		if err := json.Unmarshal([]byte(requirementsJSON), &h.requirementsDue); err != nil {
			t.Fatalf("decode requirements: %v", err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate history: %v", err)
	}
	return out
}

func historyCount(t *testing.T, db *testdb.DB) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM stripe_connect_status_history`).Scan(&n); err != nil {
		t.Fatalf("count history: %v", err)
	}
	return n
}

func postCapabilityStatusAt(t *testing.T, db *testdb.DB, client *accountFetchClient, eventID, accountID string, created time.Time) {
	t.Helper()
	srv := newAccountWebhookServer(db, client)
	defer srv.Close()
	resp := postAccountWebhook(t, srv,
		accountEventPayloadAt(t, eventID, stripeEventTypeCapabilityStatus, accountID, created), stripeAccountWebhookSecret)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delivery %s status = %d, want %d", eventID, resp.StatusCode, http.StatusOK)
	}
}

// TestPostAccountWebhookHandler_AppliedEventWritesOneHistoryRow is #1812's
// first criterion: the row names the Practice, the event, both statuses,
// what is outstanding, Stripe's time and ours -- and ours is the same
// instant the practices row records, because both are written in one
// transaction.
func TestPostAccountWebhookHandler_AppliedEventWritesOneHistoryRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "History Practice", "acct_history")
	client := activeAccountClient()
	client.status = payments.AccountStatus{
		CardPayments:    payments.CapabilityPending,
		Payouts:         payments.CapabilityRestricted,
		RequirementsDue: []string{requirementMCC},
	}
	created := time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)

	postCapabilityStatusAt(t, db, client, "evt_history_one", "acct_history", created)

	history := connectHistory(t, db, practiceID)
	if len(history) != 1 {
		t.Fatalf("history rows = %d, want 1", len(history))
	}
	h := history[0]
	if h.eventID != "evt_history_one" {
		t.Fatalf("event id = %q, want the delivering event", h.eventID)
	}
	if h.cardPayments != string(payments.CapabilityPending) || h.payouts != string(payments.CapabilityRestricted) {
		t.Fatalf("statuses = (%q, %q), want (pending, restricted)", h.cardPayments, h.payouts)
	}
	if len(h.requirementsDue) != 1 || h.requirementsDue[0] != requirementMCC {
		t.Fatalf("requirements = %v, want [%s]", h.requirementsDue, requirementMCC)
	}
	if !h.eventCreatedAt.Equal(created) {
		t.Fatalf("stripe_event_created_at = %v, want Stripe's own %v", h.eventCreatedAt, created)
	}

	var updatedAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT stripe_connect_status_updated_at FROM practices WHERE id = $1`, practiceID,
	).Scan(&updatedAt); err != nil {
		t.Fatalf("query practices: %v", err)
	}
	if !h.processedAt.Equal(updatedAt) {
		t.Fatalf("processed_at = %v, want the practices row's own %v", h.processedAt, updatedAt)
	}
}

// TestPostAccountWebhookHandler_HistoryKeepsEveryAppliedEvent is the gap
// #1812 closes: practices keeps only the last write, so the moment
// card_payments became active must survive a later event in the history.
func TestPostAccountWebhookHandler_HistoryKeepsEveryAppliedEvent(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "History Two Practice", "acct_history_two")
	client := activeAccountClient()
	client.status = payments.AccountStatus{CardPayments: payments.CapabilityActive, Payouts: payments.CapabilityPending}
	postCapabilityStatusAt(t, db, client, "evt_history_active", "acct_history_two", time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC))

	client.status = payments.AccountStatus{CardPayments: payments.CapabilityActive, Payouts: payments.CapabilityActive}
	postCapabilityStatusAt(t, db, client, "evt_history_payouts", "acct_history_two", time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC))

	history := connectHistory(t, db, practiceID)
	if len(history) != 2 {
		t.Fatalf("history rows = %d, want 2", len(history))
	}
	if history[0].eventID != "evt_history_active" || history[0].payouts != string(payments.CapabilityPending) {
		t.Fatalf("first row = %+v, want the earlier event and its pending payouts kept", history[0])
	}
	if history[1].eventID != "evt_history_payouts" || history[1].payouts != string(payments.CapabilityActive) {
		t.Fatalf("second row = %+v, want the later event", history[1])
	}
}

// TestPostAccountWebhookHandler_MissingEventTimeFallsBackToProcessedTime:
// a notification with no `created` must not be stored as year 1, where
// it would read as the earliest event of all.
func TestPostAccountWebhookHandler_MissingEventTimeFallsBackToProcessedTime(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "History No Time Practice", "acct_history_no_time")
	client := activeAccountClient()

	postCapabilityStatusAt(t, db, client, "evt_history_no_time", "acct_history_no_time", time.Time{})

	history := connectHistory(t, db, practiceID)
	if len(history) != 1 {
		t.Fatalf("history rows = %d, want 1", len(history))
	}
	if !history[0].eventCreatedAt.Equal(history[0].processedAt) {
		t.Fatalf("stripe_event_created_at = %v, want the processed time %v", history[0].eventCreatedAt, history[0].processedAt)
	}
}

// TestPostAccountWebhookHandler_ReplayWritesNoHistoryRow is the first half
// of #1812's second criterion: a delivery already in stripe_webhook_events
// changes nothing, so it records nothing.
func TestPostAccountWebhookHandler_ReplayWritesNoHistoryRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "History Replay Practice", "acct_history_replay")
	client := activeAccountClient()
	created := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)

	postCapabilityStatusAt(t, db, client, "evt_history_replayed", "acct_history_replay", created)
	postCapabilityStatusAt(t, db, client, "evt_history_replayed", "acct_history_replay", created)

	if got := len(connectHistory(t, db, practiceID)); got != 1 {
		t.Fatalf("history rows = %d after a replay, want 1", got)
	}
}

// TestPostAccountWebhookHandler_UnrecognizedAccountWritesNoHistoryRow is
// the second half: an account no Practice claims has no Practice to
// record against.
func TestPostAccountWebhookHandler_UnrecognizedAccountWritesNoHistoryRow(t *testing.T) {
	db := testdb.New(t)
	seedConnectedPractice(t, db, "History Known Practice", "acct_history_known")
	client := activeAccountClient()

	postCapabilityStatusAt(t, db, client, "evt_history_stranger", "acct_history_stranger", time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC))

	if got := historyCount(t, db); got != 0 {
		t.Fatalf("history rows = %d, want 0 for an unrecognized account", got)
	}
}
