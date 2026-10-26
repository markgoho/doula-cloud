package payments_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// postConnectNudgeAt sends one nudge request as uid through a server whose
// clock is fixed at now (#1992), with a session seeded at that instant.
func postConnectNudgeAt(t *testing.T, db *testdb.DB, uid, practiceID string, now time.Time) int {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	payments.Mount(g, ir, payments.NewFakeClient(), &tasknudge.FakeEnqueuer{}, db.App, testPublishableKey)
	srv := httptest.NewServer(clock.Middleware(func() time.Time { return now })(mux))
	defer srv.Close()
	resp := postConnectNudge(t, srv, authntest.SeedSessionAt(t, db.App, uid, now), practiceID)
	_ = resp.Body.Close()
	return resp.StatusCode
}

// TestPostConnectNudgeHandler_CooldownEdgeOnTheRequestClock is #1992's
// edge for the cooldown: the first ask is stamped at the request clock's
// instant, a second ask one microsecond short of ConnectNudgeCooldownDays
// later is refused, and one at exactly that many days is accepted. Both
// the stamp and the window read clock.Now(ctx).
func TestPostConnectNudgeHandler_CooldownEdgeOnTheRequestClock(t *testing.T) {
	askedAt := time.Now().Add(-30 * 24 * time.Hour).Truncate(time.Microsecond).UTC()
	cooldown := time.Duration(payments.ConnectNudgeCooldownDays) * 24 * time.Hour
	cases := []struct {
		name       string
		secondAt   time.Time
		wantStatus int
	}{
		{"just-inside", askedAt.Add(cooldown - time.Microsecond), http.StatusConflict},
		{"at-cooldown", askedAt.Add(cooldown), http.StatusAccepted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := testdb.New(t)
			const uid = "connect-nudge-clock-edge"
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")

			if got := postConnectNudgeAt(t, db, uid, practiceID, askedAt); got != http.StatusAccepted {
				t.Fatalf("first status = %d, want %d", got, http.StatusAccepted)
			}
			var createdAt time.Time
			if err := db.Admin.QueryRowContext(t.Context(),
				`SELECT created_at FROM connect_nudge_outbox WHERE practice_id = $1`, practiceID,
			).Scan(&createdAt); err != nil {
				t.Fatalf("read created_at: %v", err)
			}
			if !createdAt.Equal(askedAt) {
				t.Fatalf("created_at = %v, want the request clock %v", createdAt, askedAt)
			}
			// Sent, so the one-pending index is not what answers the second ask.
			if _, err := db.Admin.ExecContext(t.Context(),
				`UPDATE connect_nudge_outbox SET status = 'sent' WHERE practice_id = $1`, practiceID,
			); err != nil {
				t.Fatalf("mark sent: %v", err)
			}

			if got := postConnectNudgeAt(t, db, uid, practiceID, c.secondAt); got != c.wantStatus {
				t.Fatalf("second status = %d, want %d", got, c.wantStatus)
			}
		})
	}
}
