package practicedeletion_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/practicedeletion"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	ownerRole = "owner"
	adminRole = "admin"
)

// newMux mounts practicedeletion.Mount alongside client.Mount -- the
// latter only so a test can seed a Client through the real endpoint and,
// where a test needs it, put a real erasure through it to prove finalize
// does not double-erase.
func newMux(db *testdb.DB) *http.ServeMux {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	client.Mount(g, ir, tasknudge.NoOpEnqueuer{})
	practicedeletion.Mount(g, ir)
	return mux
}

func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	return httptest.NewServer(newMux(db)), authntest.SeedSession(t, db.App, uid)
}

// authedRequest issues method against url with session's cookie -- every
// route this package tests takes no request body, so unlike
// client_test's own authedJSON there is no body parameter to marshal.
func authedRequest(t *testing.T, session, method, url string, confirmed bool) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	if confirmed {
		req.Header.Set("X-Confirmed", "true")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func authedGet(t *testing.T, session, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// decodeJSON reads and json-decodes resp's body into v, closing the body
// once done.
func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// seedUnsettledInvoice gives practiceID one draft Invoice on a fresh
// Client/Engagement/Contract chain -- the fact InitiateHandler's own
// precheck refuses on.
func seedUnsettledInvoice(t *testing.T, db *testdb.DB, practiceID string) {
	t.Helper()
	var clientID, engagementID, contractID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name) VALUES ($1, 'Unsettled') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents) VALUES ($1, 'signed', '', 15000) RETURNING id`,
		engagementID,
	).Scan(&contractID); err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, status, amount_cents, reference)
		 VALUES ($1, $2, gen_random_uuid()::text, 'open', 10000, gen_random_uuid()::text)`,
		practiceID, contractID,
	); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}
}

// practiceDeletionState reads back the columns 00088_practice_deletion.sql
// added to practices, for assertions the JSON response doesn't need to
// carry (e.g. deletion_requested_by).
func practiceDeletionState(t *testing.T, db *testdb.DB, practiceID string) (requestedAt, finalizeAt, deletedAt *time.Time, requestedBy *string) {
	t.Helper()
	var r, f, d sql.NullTime
	var by sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT deletion_requested_at, deletion_finalize_at, deleted_at, deletion_requested_by FROM practices WHERE id = $1`,
		practiceID,
	).Scan(&r, &f, &d, &by); err != nil {
		t.Fatalf("read practice deletion state: %v", err)
	}
	toTime := func(v sql.NullTime) *time.Time {
		if !v.Valid {
			return nil
		}
		t := v.Time
		return &t
	}
	toStr := func(v sql.NullString) *string {
		if !v.Valid {
			return nil
		}
		s := v.String
		return &s
	}
	return toTime(r), toTime(f), toTime(d), toStr(by)
}

// pendingOutboxRow reads back the single pending practice_deletion_outbox
// row for practiceID and act, failing the test if there isn't exactly
// one -- the shape enqueue()'s upsert guarantees.
func pendingOutboxRow(t *testing.T, db *testdb.DB, practiceID, act string) (nextAttemptAt time.Time, attemptCount int) {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT next_attempt_at, attempt_count FROM practice_deletion_outbox WHERE practice_id = $1 AND act = $2 AND status = 'pending'`,
		practiceID, act,
	)
	if err != nil {
		t.Fatalf("query practice_deletion_outbox: %v", err)
	}
	defer rows.Close()
	var got []struct {
		next    time.Time
		attempt int
	}
	for rows.Next() {
		var row struct {
			next    time.Time
			attempt int
		}
		if err := rows.Scan(&row.next, &row.attempt); err != nil {
			t.Fatalf("scan practice_deletion_outbox row: %v", err)
		}
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate practice_deletion_outbox rows: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("pending %s rows for practice %s = %d, want 1", act, practiceID, len(got))
	}
	return got[0].next, got[0].attempt
}
