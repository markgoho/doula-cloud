package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

// fakeStripeEraser records the two Stripe acts the worker performs, and
// can be made to fail either of them.
type fakeStripeEraser struct {
	deleted    [][2]string
	redactions [][2]string
	deleteErr  error
	redactErr  error
}

func (f *fakeStripeEraser) DeleteCustomer(_ context.Context, accountID, customerID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, [2]string{accountID, customerID})
	return nil
}

func (f *fakeStripeEraser) CreateRedactionJob(_ context.Context, accountID, customerID string) (string, error) {
	if f.redactErr != nil {
		return "", f.redactErr
	}
	f.redactions = append(f.redactions, [2]string{accountID, customerID})
	return "rdj_test", nil
}

const testConnectAccount = "acct_test_erasure"

// seedInvoicedClient seeds a Client who has been billed: a connected
// account on her Practice, an Engagement, a Contract, and one paid
// invoice carrying a Stripe Customer id, created `age` ago.
func seedInvoicedClient(t *testing.T, db *testdb.DB, practiceID, clientID, customerID, status string, age time.Duration) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_account_id = $2 WHERE id = $1`, practiceID, testConnectAccount,
	); err != nil {
		t.Fatalf("seed connect account: %v", err)
	}
	var engagementID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	var contractID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO contracts (engagement_id, prose, merge_field_values)
		 VALUES ($1, 'Ada Lovelace agrees to...', '{"client_name":"Ada Lovelace"}'::jsonb) RETURNING id`,
		engagementID,
	).Scan(&contractID); err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, stripe_customer_id, status, amount_cents, created_at)
		 VALUES ($1, $2, $3, $4, $5::invoice_status, 150000, now() - $6::interval)`,
		practiceID, contractID, "in_"+customerID, customerID, status, age.String(),
	); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}
}

// TestEraseHandler_QueuesStripeWorkAndReportsWhenItCanRun covers the
// Stripe criterion: the Customer delete is due at once, the Redaction
// Job is deferred to Stripe's 90-day floor past her newest invoice, and
// that date is visible to the Practice rather than implied.
func TestEraseHandler_QueuesStripeWorkAndReportsWhenItCanRun(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-stripe"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)
	seedInvoicedClient(t, db, practiceID, clientID, "cus_recent", "paid", 10*24*time.Hour)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	resp := postErasure(t, session, srv, practiceID, clientID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	acts := readOutbox(t, db, clientID)
	if len(acts) != 2 {
		t.Fatalf("outbox rows = %+v, want a delete and a redaction", acts)
	}
	del, ok := acts["stripe_customer_delete|cus_recent"]
	if !ok {
		t.Fatalf("no customer-delete row queued: %+v", acts)
	}
	if del.After(time.Now().Add(time.Minute)) {
		t.Fatalf("customer delete due at %v, want it due now", del)
	}
	redact, ok := acts["stripe_redaction_job|cus_recent"]
	if !ok {
		t.Fatalf("no redaction row queued: %+v", acts)
	}
	// 10 days old, so 80 days to wait.
	if wait := time.Until(redact); wait < 79*24*time.Hour || wait > 81*24*time.Hour {
		t.Fatalf("redaction due in %v, want about 80 days -- Stripe's 90-day floor past a 10-day-old invoice", wait)
	}

	detail := readDetail(t, session, srv, practiceID, clientID)
	if detail.StripeRedactionEligibleAt == nil {
		t.Fatal("detail carries no stripeRedactionEligibleAt, want the Practice to see the Stripe half is scheduled")
	}
	if detail.ErasedAt == nil {
		t.Fatal("detail carries no erasedAt")
	}

	// Her Contract's captured merge fields went with her.
	var mergeFields string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT ct.merge_field_values::text FROM contracts ct
		   JOIN engagements e ON e.id = ct.engagement_id WHERE e.client_id = $1`, clientID,
	).Scan(&mergeFields); err != nil {
		t.Fatalf("read merge fields: %v", err)
	}
	if mergeFields != "{}" {
		t.Fatalf("contract merge_field_values = %s, want {}", mergeFields)
	}
}

// TestEraseHandler_RedactionIsDueAtOnceForAnOldInvoice is the other side
// of the floor: an invoice already past 90 days waits for nothing, and
// there is no future date to show.
func TestEraseHandler_RedactionIsDueAtOnceForAnOldInvoice(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-old-invoice"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)
	seedInvoicedClient(t, db, practiceID, clientID, "cus_old", "paid", 200*24*time.Hour)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	resp := postErasure(t, session, srv, practiceID, clientID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	acts := readOutbox(t, db, clientID)
	redact, ok := acts["stripe_redaction_job|cus_old"]
	if !ok {
		t.Fatalf("no redaction row queued: %+v", acts)
	}
	if redact.After(time.Now().Add(time.Minute)) {
		t.Fatalf("redaction due at %v, want it due now for a 200-day-old invoice", redact)
	}
	if detail := readDetail(t, session, srv, practiceID, clientID); detail.StripeRedactionEligibleAt != nil {
		t.Fatalf("stripeRedactionEligibleAt = %v, want absent when there is nothing to wait for", detail.StripeRedactionEligibleAt)
	}
}

// TestEraseHandler_RefusesWhileAnInvoiceIsUnsettled -- Stripe cannot
// redact a non-terminal transaction, and the Practice cannot collect on
// one whose Customer has been deleted, so the whole act refuses.
func TestEraseHandler_RefusesWhileAnInvoiceIsUnsettled(t *testing.T) {
	for _, status := range []string{"draft", "open"} {
		t.Run(status, func(t *testing.T) {
			db := testdb.New(t)
			uid := "owner-erase-unsettled-" + status
			practiceID, staffID := seedOwner(t, db, uid)
			clientID := seedFullClient(t, db, practiceID, staffID)
			seedInvoicedClient(t, db, practiceID, clientID, "cus_"+status, status, time.Hour)

			srv, session := newErasureServer(t, db, uid)
			defer srv.Close()

			resp := postErasure(t, session, srv, practiceID, clientID)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
			}
			if rows := readOutbox(t, db, clientID); len(rows) != 0 {
				t.Fatalf("outbox rows = %+v, want none after a refusal", rows)
			}
		})
	}
}

// TestEraseHandler_DeletesHerPortalLoginAndEndsHerSessions covers the
// Identity Platform criterion, and the session deletion it implies:
// deleting the account does not by itself sign her out.
func TestEraseHandler_DeletesHerPortalLoginAndEndsHerSessions(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-portal"
	const portalUID = "portal-uid-erasure"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, identity_uid) VALUES ($1, $2)`, clientID, portalUID,
	); err != nil {
		t.Fatalf("seed portal user: %v", err)
	}
	authntest.SeedSession(t, db.App, portalUID)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	resp := postErasure(t, session, srv, practiceID, clientID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var sessions int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM sessions WHERE identity_uid = $1`, portalUID,
	).Scan(&sessions); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessions != 0 {
		t.Fatalf("portal sessions = %d, want 0 -- she must not still be signed in", sessions)
	}

	var storedUID *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT identity_uid FROM client_portal_users WHERE client_id = $1`, clientID,
	).Scan(&storedUID); err != nil {
		t.Fatalf("read portal user: %v", err)
	}
	if storedUID != nil {
		t.Fatalf("identity_uid = %q, want NULL", *storedUID)
	}

	if _, ok := readOutbox(t, db, clientID)["identity_account_delete|"+portalUID]; !ok {
		t.Fatal("no identity-account-delete row queued")
	}
}

// TestEraseHandler_RevokesAnUnacceptedPortalInvite -- an invitation
// nobody accepted has no Identity Platform account behind it, so there
// is nothing to delete and nothing to queue; the pending token is
// revoked and that is the whole act.
func TestEraseHandler_RevokesAnUnacceptedPortalInvite(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-pending-invite"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid())`, clientID,
	); err != nil {
		t.Fatalf("seed portal invite: %v", err)
	}

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	resp := postErasure(t, session, srv, practiceID, clientID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var token *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT invite_token::text FROM client_portal_users WHERE client_id = $1`, clientID,
	).Scan(&token); err != nil {
		t.Fatalf("read portal user: %v", err)
	}
	if token != nil {
		t.Fatalf("invite_token = %q, want NULL", *token)
	}
	if rows := readOutbox(t, db, clientID); len(rows) != 0 {
		t.Fatalf("outbox rows = %+v, want none -- there is no account to delete", rows)
	}
}

// TestErasureWorker_PerformsEveryQueuedAct is the worker's happy path,
// all three acts at once.
func TestErasureWorker_PerformsEveryQueuedAct(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-worker-happy"
	const portalUID = "portal-uid-worker"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)
	seedInvoicedClient(t, db, practiceID, clientID, "cus_worker", "paid", 200*24*time.Hour)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, identity_uid) VALUES ($1, $2)`, clientID, portalUID,
	); err != nil {
		t.Fatalf("seed portal user: %v", err)
	}

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()
	resp := postErasure(t, session, srv, practiceID, clientID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("erase status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	stripe := &fakeStripeEraser{}
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(portalUID, "ada@example.com", true)
	runWorker(t, db, client.ErasureWorker{Stripe: stripe, Identity: accounts, Now: time.Now})

	if len(stripe.deleted) != 1 || stripe.deleted[0] != [2]string{testConnectAccount, "cus_worker"} {
		t.Fatalf("customer deletes = %v, want one on the Practice's own connected account", stripe.deleted)
	}
	if len(stripe.redactions) != 1 || stripe.redactions[0] != [2]string{testConnectAccount, "cus_worker"} {
		t.Fatalf("redaction jobs = %v, want one", stripe.redactions)
	}
	if accounts.Exists(portalUID) {
		t.Fatal("the Identity Platform account still exists, want it deleted")
	}
	for act, status := range readOutboxStatus(t, db, clientID) {
		if status != "sent" {
			t.Errorf("%s = %q, want sent", act, status)
		}
	}
}

// TestErasureWorker_RetriesAFailedAct -- a Stripe outage is not a reason
// to give up on an erasure, so the row goes back on the retry schedule
// rather than being marked done.
func TestErasureWorker_RetriesAFailedAct(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-worker-retry"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)
	seedInvoicedClient(t, db, practiceID, clientID, "cus_retry", "paid", 200*24*time.Hour)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()
	resp := postErasure(t, session, srv, practiceID, clientID)
	defer resp.Body.Close()

	stripe := &fakeStripeEraser{deleteErr: errors.New("stripe is down"), redactErr: errors.New("stripe is down")}
	runWorker(t, db, client.ErasureWorker{Stripe: stripe, Identity: authntest.NewFakeAccountManager(), Now: time.Now})

	for act, status := range readOutboxStatus(t, db, clientID) {
		if status != pendingStatus {
			t.Errorf("%s = %q, want pending -- a failed act is retried, not abandoned", act, status)
		}
	}
}

// TestErasureWorker_DeadLettersWhenThePracticeHasNoConnectedAccount --
// nothing to reach and nothing to wait for, so the row is dead-lettered
// rather than retried forever or falsely marked done.
func TestErasureWorker_DeadLettersWhenThePracticeHasNoConnectedAccount(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-worker-no-account"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)
	seedInvoicedClient(t, db, practiceID, clientID, "cus_orphan", "paid", 200*24*time.Hour)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_account_id = NULL WHERE id = $1`, practiceID,
	); err != nil {
		t.Fatalf("clear connect account: %v", err)
	}

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()
	resp := postErasure(t, session, srv, practiceID, clientID)
	defer resp.Body.Close()

	stripe := &fakeStripeEraser{}
	runWorker(t, db, client.ErasureWorker{Stripe: stripe, Identity: authntest.NewFakeAccountManager(), Now: time.Now})

	for act, status := range readOutboxStatus(t, db, clientID) {
		if status != "dead_lettered" {
			t.Errorf("%s = %q, want dead_lettered", act, status)
		}
	}
	if len(stripe.deleted) != 0 {
		t.Fatalf("customer deletes = %v, want none -- there is no account to call", stripe.deleted)
	}
}

// TestErasureWorker_RetriesAFailedIdentityDelete covers the third act's
// own failure branch.
func TestErasureWorker_RetriesAFailedIdentityDelete(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-worker-identity-fail"
	const portalUID = "portal-uid-worker-fail"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, identity_uid) VALUES ($1, $2)`, clientID, portalUID,
	); err != nil {
		t.Fatalf("seed portal user: %v", err)
	}

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()
	resp := postErasure(t, session, srv, practiceID, clientID)
	defer resp.Body.Close()

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(portalUID, "ada@example.com", true)
	accounts.DeleteAccountErr = errors.New("identity platform is down")
	runWorker(t, db, client.ErasureWorker{Stripe: &fakeStripeEraser{}, Identity: accounts, Now: time.Now})

	if status := readOutboxStatus(t, db, clientID)["identity_account_delete|"+portalUID]; status != pendingStatus {
		t.Fatalf("identity delete status = %q, want pending", status)
	}
	if !accounts.Exists(portalUID) {
		t.Fatal("the account was deleted despite the error")
	}
}

// TestProcessErasureOutboxHandler_RunsDueActsBehindTheWorkerSecret is
// the endpoint Cloud Scheduler and the nudge both call: the secret gate
// is outbox.ProcessHandler's, tested there, so this proves the wiring --
// a correct secret runs the worker, a wrong one does not.
func TestProcessErasureOutboxHandler_RunsDueActsBehindTheWorkerSecret(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erasure-endpoint"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)
	seedInvoicedClient(t, db, practiceID, clientID, "cus_endpoint", "paid", 200*24*time.Hour)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()
	resp := postErasure(t, session, srv, practiceID, clientID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("erase status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	stripe := &fakeStripeEraser{}
	worker := client.ErasureWorker{Stripe: stripe, Identity: authntest.NewFakeAccountManager(), Now: time.Now}
	mux := http.NewServeMux()
	mux.Handle("POST /internal/clients/process-erasure-outbox",
		client.ProcessErasureOutboxHandler(db.App, worker, "correct-secret"))
	workerSrv := httptest.NewServer(mux)
	defer workerSrv.Close()

	wrong := postProcessErasure(t, workerSrv, "wrong-secret")
	defer wrong.Body.Close()
	if wrong.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong-secret status = %d, want %d", wrong.StatusCode, http.StatusUnauthorized)
	}
	if len(stripe.deleted) != 0 {
		t.Fatalf("customer deletes = %v, want none behind a rejected secret", stripe.deleted)
	}

	right := postProcessErasure(t, workerSrv, "correct-secret")
	defer right.Body.Close()
	if right.StatusCode != http.StatusOK {
		t.Fatalf("correct-secret status = %d, want %d", right.StatusCode, http.StatusOK)
	}
	if len(stripe.deleted) != 1 {
		t.Fatalf("customer deletes = %v, want one", stripe.deleted)
	}
}

func postProcessErasure(t *testing.T, srv *httptest.Server, secret string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/internal/clients/process-erasure-outbox", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Internal-Secret", secret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// runWorker runs one pass of the erasure worker in its own transaction,
// the way outbox.ProcessHandler does in production.
func runWorker(t *testing.T, db *testdb.DB, worker client.ErasureWorker) {
	t.Helper()
	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := worker.ProcessPending(t.Context(), tx); err != nil {
		t.Fatalf("process pending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// readOutbox reads every queued act for clientID as "act|target" ->
// when it is due.
func readOutbox(t *testing.T, db *testdb.DB, clientID string) map[string]time.Time {
	t.Helper()
	return scanOutbox[time.Time](t, db, clientID, `act::text || '|' || target, next_attempt_at`)
}

// readOutboxStatus reads every queued act for clientID as "act|target"
// -> its status.
func readOutboxStatus(t *testing.T, db *testdb.DB, clientID string) map[string]string {
	t.Helper()
	return scanOutbox[string](t, db, clientID, `act::text || '|' || target, status::text`)
}

func scanOutbox[V any](t *testing.T, db *testdb.DB, clientID, columns string) map[string]V {
	t.Helper()
	//nolint:gosec // columns is a compile-time constant from this file's two callers, never request data
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT `+columns+` FROM client_erasure_outbox WHERE client_id = $1`, clientID)
	if err != nil {
		t.Fatalf("read erasure outbox: %v", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]V{}
	for rows.Next() {
		var key string
		var value V
		if err := rows.Scan(&key, &value); err != nil {
			t.Fatalf("scan erasure outbox: %v", err)
		}
		out[key] = value
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate erasure outbox: %v", err)
	}
	return out
}
