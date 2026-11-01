package webhookarrival_test

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/webhookarrival"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}

var arrivalNow = time.Date(2026, 11, 1, 9, 30, 0, 0, time.UTC)

func post(t *testing.T, db *sql.DB, route webhookarrival.Route, body string, next http.Handler) int {
	t.Helper()
	srv := httptest.NewServer(clockAt(webhookarrival.Record(db, route, next)))
	t.Cleanup(srv.Close)
	resp, err := http.Post(srv.URL, "application/json", strings.NewReader(body)) //nolint:noctx // test request
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func clockAt(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(clock.Into(r.Context(), func() time.Time { return arrivalNow })))
	})
}

func TestRecord_WritesOneRowWithTheRouteOutcomeEventAndClockTime(t *testing.T) {
	db := testdb.New(t)
	var seen string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = string(b)
		w.WriteHeader(http.StatusOK)
	})

	body := `{"id":"evt_1","type":"invoice.paid","data":{"object":{"customer_email":"hannah@example.com"}}}`
	post(t, db.App, webhookarrival.StripeConnect, body, next)

	if seen != body {
		t.Fatalf("handler read %q, want the whole body", seen)
	}
	got := testdb.WebhookArrivals(t, db)
	want := []testdb.WebhookArrival{{Route: "stripe_connect", Outcome: string(webhookarrival.Accepted), EventID: "evt_1", EventType: "invoice.paid", ArrivedAt: arrivalNow}}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %+v, want %+v", got, want)
	}
}

func TestRecord_TheHandlersMarkWinsOverTheStatus(t *testing.T) {
	db := testdb.New(t)
	for _, outcome := range []webhookarrival.Outcome{
		webhookarrival.Dropped, webhookarrival.BodyRefused, webhookarrival.SignatureRefused, webhookarrival.LivemodeRefused,
	} {
		post(t, db.App, webhookarrival.StripePlatform, `{"id":"evt_`+string(outcome)+`","type":"x"}`, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			webhookarrival.Mark(r.Context(), outcome)
			w.WriteHeader(http.StatusBadRequest)
		}))
	}
	got := outcomes(testdb.WebhookArrivals(t, db))
	want := []string{"body_refused", "dropped", "livemode_refused", "signature_refused"}
	if !slices.Equal(got, want) {
		t.Fatalf("outcomes = %v, want %v", got, want)
	}
}

func TestRecord_AnUnmarkedResponseIsNamedByItsStatus(t *testing.T) {
	db := testdb.New(t)
	for _, status := range []int{http.StatusOK, http.StatusBadRequest, http.StatusInternalServerError} {
		post(t, db.App, webhookarrival.StripeAccount, `{}`, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
	}
	post(t, db.App, webhookarrival.StripeAccount, `{}`, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	got := outcomes(testdb.WebhookArrivals(t, db))
	want := []string{"accepted", "accepted", "failed", "refused"}
	if !slices.Equal(got, want) {
		t.Fatalf("outcomes = %v, want %v", got, want)
	}
}

func TestRecord_MailgunReadsTheEventFromEventData(t *testing.T) {
	db := testdb.New(t)
	post(t, db.App, webhookarrival.Mailgun,
		`{"signature":{"token":"t"},"event-data":{"id":"mg_1","event":"failed","recipient":"hannah@example.com"}}`,
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	got := testdb.WebhookArrivals(t, db)
	if len(got) != 1 || got[0].Route != "mailgun" || got[0].EventID != "mg_1" || got[0].EventType != "failed" {
		t.Fatalf("rows = %+v, want one mailgun row for mg_1 failed", got)
	}
}

func TestRecord_ABodyThatDoesNotParseStillRecordsTheArrival(t *testing.T) {
	db := testdb.New(t)
	long := strings.Repeat("e", 256)
	for _, body := range []string{`not json`, `{"id":"` + long + `","type":"` + long + `"}`} {
		post(t, db.App, webhookarrival.StripePlatform, body, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
	}
	post(t, db.App, webhookarrival.Mailgun, `not json`, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	oversized := `{"id":"evt_big","type":"x","pad":"` + strings.Repeat("p", 1<<20) + `"}`
	var seen int
	post(t, db.App, webhookarrival.StripePlatform, oversized, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = len(b)
		w.WriteHeader(http.StatusBadRequest)
	}))
	if seen != len(oversized) {
		t.Fatalf("handler read %d bytes of an oversized body, want all %d", seen, len(oversized))
	}
	got := testdb.WebhookArrivals(t, db)
	if len(got) != 4 {
		t.Fatalf("rows = %+v, want four", got)
	}
	for _, row := range got {
		if row.EventID != "" || row.EventType != "" {
			t.Fatalf("row %+v carries an event id or type from a body that does not parse to one", row)
		}
	}
}

func TestRecord_AFailedWriteLeavesTheResponseAlone(t *testing.T) {
	db := testdb.New(t)
	closed, err := sql.Open("pgx", db.AppURL)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = closed.Close()

	status := post(t, closed, webhookarrival.Mailgun, `{}`, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
}

func outcomes(rows []testdb.WebhookArrival) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Outcome)
	}
	slices.Sort(out)
	return out
}

func seedArrival(t *testing.T, db *testdb.DB, eventID string, at time.Time) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO webhook_arrivals (route, outcome, event_id, arrived_at) VALUES ('stripe_platform', 'accepted', $1, $2)`,
		eventID, at,
	); err != nil {
		t.Fatalf("seed arrival: %v", err)
	}
}

func runSweep(t *testing.T, db *testdb.DB, door bool) error {
	t.Helper()
	ctx := clock.Into(t.Context(), func() time.Time { return arrivalNow })
	tx, err := db.App.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if door {
		if _, err := tx.ExecContext(ctx, `SELECT set_config($1, 'true', true)`, outbox.NotificationDoor); err != nil {
			t.Fatalf("set door: %v", err)
		}
	}
	if err := (webhookarrival.Sweeper{}).ProcessPending(ctx, tx); err != nil {
		return err //nolint:wrapcheck // test helper; the caller only checks nil-ness
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return nil
}

func eventIDs(rows []testdb.WebhookArrival) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.EventID)
	}
	slices.Sort(out)
	return out
}

func TestSweeper_DeletesRowsOlderThan90DaysAndKeepsTheBoundary(t *testing.T) {
	db := testdb.New(t)
	ninety := 90 * 24 * time.Hour
	seedArrival(t, db, "past", arrivalNow.Add(-ninety-time.Second))
	seedArrival(t, db, "boundary", arrivalNow.Add(-ninety))
	seedArrival(t, db, "inside", arrivalNow.Add(-ninety+time.Second))

	if err := runSweep(t, db, true); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got, want := eventIDs(testdb.WebhookArrivals(t, db)), []string{"boundary", "inside"}; !slices.Equal(got, want) {
		t.Fatalf("rows after the sweep = %v, want %v", got, want)
	}
}

func TestSweeper_RefusedOutsideTheWorkerDoor(t *testing.T) {
	db := testdb.New(t)
	seedArrival(t, db, "past", arrivalNow.Add(-100*24*time.Hour))

	if err := runSweep(t, db, false); err == nil {
		t.Fatal("the sweep ran without the worker door, want it refused")
	}
	if got := eventIDs(testdb.WebhookArrivals(t, db)); !slices.Equal(got, []string{"past"}) {
		t.Fatalf("rows after a refused sweep = %v, want [past]", got)
	}
}

func TestAppRuntimeCannotReadOrDeleteArrivals(t *testing.T) {
	db := testdb.New(t)
	seedArrival(t, db, "kept", arrivalNow)
	for _, statement := range []string{`SELECT count(*) FROM webhook_arrivals`, `DELETE FROM webhook_arrivals`, `UPDATE webhook_arrivals SET outcome = 'failed'`} {
		if _, err := db.App.ExecContext(t.Context(), statement); err == nil {
			t.Errorf("app_runtime ran %q, want it refused", statement)
		}
	}
}
