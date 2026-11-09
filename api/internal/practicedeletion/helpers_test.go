package practicedeletion_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/practicedeletion"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	ownerRole = "owner"
	adminRole = "admin"
)

// env is a Practice with this package's routes mounted beside
// client.Mount -- the latter only so a test can seed a Client through the
// real endpoint and, where a test needs it, put a real erasure through it
// to prove finalize does not double-erase -- and one Staff member of the
// given role at the screen.
type env struct {
	*fixture.Scene
}

func newEnv(t *testing.T, role fixture.StaffOption) *env {
	t.Helper()
	mount := func(g *staffauth.GatedRouter, ir *idempotency.Router, _ *sql.DB) {
		client.Mount(g, ir, tasknudge.NoOpEnqueuer{})
		practicedeletion.Mount(g, ir)
	}
	return &env{fixture.NewScene(t, mount, fixture.SignedInAs(role))}
}

// call issues method against the Practice's deletion route as the Staff
// member at the screen. Every route this package tests takes no request
// body.
func (e *env) call(method string, confirmed bool) *http.Response {
	e.T.Helper()
	return reqtest.Do(e.T, method, e.PracticeURL("/deletion"), e.SignedIn,
		reqtest.Header("Content-Type", "application/json"), reqtest.HeaderIf(confirmed, "X-Confirmed", "true"))
}

// expect issues method and requires want.
func (e *env) expect(method string, confirmed bool, want int) {
	e.T.Helper()
	resp := e.call(method, confirmed)
	defer func() { _ = resp.Body.Close() }()
	reqtest.RequireStatus(e.T, resp, want)
}

// initiate and restore are the two writes, each expected to succeed.
func (e *env) initiate() { e.T.Helper(); e.expect(http.MethodPost, true, http.StatusOK) }
func (e *env) restore()  { e.T.Helper(); e.expect(http.MethodDelete, false, http.StatusNoContent) }

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
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, status, amount_cents, reference, due_at)
		 VALUES ($1, $2, gen_random_uuid()::text, 'open', 10000, gen_random_uuid()::text, now() + interval '30 days')`,
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
