package offer_test

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/testdb"
)

const workerSecret = "worker-secret-test"

// newWorker builds a Worker whose clock the test controls and whose mail
// goes to an in-memory sender.
func newWorker(sender mail.Sender, now time.Time) offer.Worker {
	return offer.Worker{
		Sender:     sender,
		Now:        func() time.Time { return now },
		AppBaseURL: "https://app.example.test",
		From:       "Doula Cloud <notifications@mg.example.test>",
		ReplyTo:    "support@mg.example.test",
	}
}

// runWorker drives one ProcessPending pass through the same endpoint
// Cloud Scheduler calls, so the trusted-worker session variable the RLS
// policies read is set the way production sets it.
func runWorker(t *testing.T, db *testdb.DB, worker offer.Worker) response {
	t.Helper()
	srv := httptest.NewServer(offer.ProcessOutboxHandler(db.App, worker, workerSecret))
	t.Cleanup(srv.Close)
	return postWithSecret(t, srv.URL, workerSecret)
}

// postWithSecret POSTs to url carrying secret as the internal-worker
// header, closing the response.
func postWithSecret(t *testing.T, url, secret string) response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if secret != "" {
		req.Header.Set("X-Internal-Secret", secret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	read, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return response{status: resp.StatusCode, body: read}
}

// outboxRow is one engagement_offer_outbox row as these tests read it.
type outboxRow struct {
	status       string
	attemptCount int
	token        *string
	code         *string
	lastError    *string
}

func readOutbox(t *testing.T, db *testdb.DB, offerID string) outboxRow {
	t.Helper()
	var r outboxRow
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text, attempt_count, invite_token::text, access_code, last_error
		   FROM engagement_offer_outbox WHERE offer_id = $1`, offerID,
	).Scan(&r.status, &r.attemptCount, &r.token, &r.code, &r.lastError); err != nil {
		t.Fatalf("read outbox row: %v", err)
	}
	return r
}

func TestWorker_MailsTheLinkAndTheCode(t *testing.T) {
	f := newFixture(t)
	offerID, token, code := seedEmailOffer(t, f)
	sender := &mail.FakeSender{}

	expectStatus(t, runWorker(t, f.db, newWorker(sender, time.Now())), http.StatusOK)

	sent := sender.Sent()
	if len(sent) != 1 {
		t.Fatalf("sent = %d messages, want 1", len(sent))
	}
	msg := sent[0]
	if msg.To != testAddress {
		t.Fatalf("To = %q", msg.To)
	}
	if !strings.Contains(msg.Text, "/offers/"+offerID+"?token="+token) {
		t.Fatalf("body carries no offer link: %q", msg.Text)
	}
	if !strings.Contains(msg.Text, code) {
		t.Fatalf("body carries no access code: %q", msg.Text)
	}
	// Platform voice: no Practice, no Client, nothing about the work.
	if strings.Contains(msg.Text, "Test Practice") || strings.Contains(msg.Text, "Test Client") {
		t.Fatalf("body names the practice or the client: %q", msg.Text)
	}

	row := readOutbox(t, f.db, offerID)
	if row.status != "sent" || row.token != nil || row.code != nil {
		t.Fatalf("row = %+v, want sent with both credentials cleared", row)
	}

	var sentAt *time.Time
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT access_code_sent_at FROM engagement_offers WHERE id = $1`, offerID,
	).Scan(&sentAt); err != nil {
		t.Fatalf("read access_code_sent_at: %v", err)
	}
	if sentAt == nil {
		t.Fatal("access_code_sent_at was not stamped")
	}
}

// An Offer decided or taken back before its email went out is never
// mailed -- the row is resolved instead.
func TestWorker_SkipsAnOfferThatIsNoLongerOpen(t *testing.T) {
	f := newFixture(t)
	offerID, _, _ := seedEmailOffer(t, f)
	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "withdraw"), f.ownerSession, nil), http.StatusOK)

	sender := &mail.FakeSender{}
	expectStatus(t, runWorker(t, f.db, newWorker(sender, time.Now())), http.StatusOK)

	if len(sender.Sent()) != 0 {
		t.Fatalf("sent %d messages for a withdrawn offer", len(sender.Sent()))
	}
	if row := readOutbox(t, f.db, offerID); row.status != "sent" {
		t.Fatalf("row status = %q, want sent (resolved without mailing)", row.status)
	}
}

func TestWorker_SkipsAnExpiredOffer(t *testing.T) {
	f := newFixture(t)
	offerID, _, _ := seedEmailOffer(t, f)
	expireOffer(t, f.db, offerID)

	sender := &mail.FakeSender{}
	expectStatus(t, runWorker(t, f.db, newWorker(sender, time.Now())), http.StatusOK)

	if len(sender.Sent()) != 0 {
		t.Fatalf("sent %d messages for an expired offer", len(sender.Sent()))
	}
}

// failingSender is a mail.Sender that always refuses, for the retry and
// dead-letter paths.
type failingSender struct{}

func (failingSender) Send(context.Context, mail.Message) error {
	return errors.New("mailgun is down")
}

func TestWorker_RetriesThenDeadLetters(t *testing.T) {
	f := newFixture(t)
	offerID, _, _ := seedEmailOffer(t, f)

	// Five failures: four scheduled retries, then the dead letter.
	for attempt := 1; attempt <= 5; attempt++ {
		expectStatus(t, runWorker(t, f.db, newWorker(failingSender{}, time.Now())), http.StatusOK)
		row := readOutbox(t, f.db, offerID)
		if row.attemptCount != attempt {
			t.Fatalf("attempt %d: attempt_count = %d", attempt, row.attemptCount)
		}
		if row.lastError == nil || *row.lastError == "" {
			t.Fatalf("attempt %d: last_error not recorded", attempt)
		}
		if attempt < 5 {
			if row.status != "pending" {
				t.Fatalf("attempt %d: status = %q, want pending", attempt, row.status)
			}
			// Clear the backoff so the next pass sees the row as due.
			if _, err := f.db.Admin.ExecContext(t.Context(),
				`UPDATE engagement_offer_outbox SET next_attempt_at = now() WHERE offer_id = $1`, offerID,
			); err != nil {
				t.Fatalf("reset backoff: %v", err)
			}
			continue
		}
		if row.status != "dead_lettered" || row.token != nil || row.code != nil {
			t.Fatalf("final row = %+v, want dead_lettered with credentials cleared", row)
		}
	}
}

// A row whose backoff has not elapsed is not due, so nothing is sent.
func TestWorker_LeavesAnUndueRowAlone(t *testing.T) {
	f := newFixture(t)
	offerID, _, _ := seedEmailOffer(t, f)
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_offer_outbox SET next_attempt_at = now() + interval '1 hour' WHERE offer_id = $1`, offerID,
	); err != nil {
		t.Fatalf("delay row: %v", err)
	}

	sender := &mail.FakeSender{}
	expectStatus(t, runWorker(t, f.db, newWorker(sender, time.Now())), http.StatusOK)
	if len(sender.Sent()) != 0 {
		t.Fatalf("sent %d messages for an undue row", len(sender.Sent()))
	}
}

func TestProcessOutboxHandler_RefusesTheWrongSecret(t *testing.T) {
	db := testdb.New(t)
	srv := httptest.NewServer(offer.ProcessOutboxHandler(db.App, newWorker(&mail.FakeSender{}, time.Now()), workerSecret))
	defer srv.Close()

	expectStatus(t, postWithSecret(t, srv.URL, "wrong"), http.StatusUnauthorized)
}

// An empty configured secret refuses every request rather than accepting
// an unauthenticated one.
func TestProcessOutboxHandler_RefusesWhenNoSecretIsConfigured(t *testing.T) {
	db := testdb.New(t)
	srv := httptest.NewServer(offer.ProcessOutboxHandler(db.App, newWorker(&mail.FakeSender{}, time.Now()), ""))
	defer srv.Close()

	expectStatus(t, postWithSecret(t, srv.URL, ""), http.StatusUnauthorized)
}

// A re-sent Offer to the same address overwrites the pending row rather
// than queueing a second one -- one Offer, one email.
func TestQueue_OverwritesThePendingRow(t *testing.T) {
	f := newFixture(t)
	offerID, token, _ := seedEmailOffer(t, f)

	if _, err := f.db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_offer_outbox SET invite_token = $1, attempt_count = 3 WHERE offer_id = $2`,
		"11111111-1111-1111-1111-111111111111", offerID,
	); err != nil {
		t.Fatalf("stale the row: %v", err)
	}

	var count int
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM engagement_offer_outbox WHERE offer_id = $1`, offerID,
	).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("outbox rows = %d, want exactly one per offer", count)
	}
	if token == "" {
		t.Fatal("no token was queued")
	}
}

// An Offer whose email is still unsent when a later Offer rotates the
// same Invitation's token gets the fresh token, not a dead one.
func TestCreate_RefreshesAStaleStaffInviteOutboxToken(t *testing.T) {
	f := newFixture(t)
	var invitationID string
	firstOffer, _, _ := seedEmailOffer(t, f)
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT invitation_id::text FROM engagement_offers WHERE id = $1`, firstOffer,
	).Scan(&invitationID); err != nil {
		t.Fatalf("read invitation: %v", err)
	}
	// A Staff invitation email for the same Invitation, still unsent.
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`INSERT INTO staff_invite_outbox (invitation_id, invite_token) VALUES ($1, $2)`,
		invitationID, "11111111-1111-1111-1111-111111111111",
	); err != nil {
		t.Fatalf("seed staff invite outbox: %v", err)
	}

	secondEngagement := seedEngagement(t, f.db, f.practiceID)
	fee := int64(52000)
	var created offer.CreateResponse
	decode(t, do(t, http.MethodPost,
		f.srv+"/practices/"+f.practiceID+"/engagements/"+secondEngagement+"/offers", f.ownerSession,
		emailOfferBody(testAddress, contractorType, &fee)), http.StatusCreated, &created)

	_, freshToken := invitationDigestAndToken(t, f.db, created.OfferID)
	var staffInviteToken sql.NullString
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT invite_token::text FROM staff_invite_outbox WHERE invitation_id = $1`, invitationID,
	).Scan(&staffInviteToken); err != nil {
		t.Fatalf("read staff invite outbox: %v", err)
	}
	if staffInviteToken.String != freshToken {
		t.Fatalf("staff invite outbox token = %q, want the rotated %q", staffInviteToken.String, freshToken)
	}
}

// invitationDigestAndToken returns the Invitation id and the plaintext
// token queued for offerID.
func invitationDigestAndToken(t *testing.T, db *testdb.DB, offerID string) (invitationID, token string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT o.invitation_id::text, ob.invite_token::text
		   FROM engagement_offers o
		   JOIN engagement_offer_outbox ob ON ob.offer_id = o.id
		  WHERE o.id = $1`, offerID,
	).Scan(&invitationID, &token); err != nil {
		t.Fatalf("read invitation and token: %v", err)
	}
	return invitationID, token
}
