package legalnotice_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/legalnotice"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

const (
	testSecret  = "legal-notice-test-secret" // #nosec G101 -- test fixture, not a credential
	publishURL  = "/api/internal/legal/publish"
	processURL  = "/api/internal/notifications/process-legal-notice-outbox"
	testAppBase = "https://app.example.test"
	founder     = "Mark"
	anaAddress  = "ana@example.com"
	statusSent  = "sent"
	statusDead  = "dead_lettered"
	employee    = "employee"
)

// owner is the role list of every Owner these tests seed.
var owner = []string{"owner"}

// publishedAt is the instant the clock reads in every test: the operator
// publishes the version on this day.
var publishedAt = time.Date(2026, time.October, 23, 15, 0, 0, 0, time.UTC)

// The fixture histories. The real ones in api/internal/legal hold no
// material version yet, and must not be given a made-up one, so the
// handler and the worker take their documents as a dependency.
//
// 2026-12-01 is 38 days after publishedAt, so it may be published.
// 2026-11-15 is 23 days after it, too soon for a material version.
var (
	fixtureTerms = legal.Document{
		Name: "Terms of Service",
		Path: "/terms",
		Versions: []legal.Version{
			{Effective: "2026-09-29", Material: false, Change: "First version."},
			{Effective: "2026-11-15", Material: true, Change: "Says what a Credit costs from November."},
			{Effective: "2026-12-01", Material: true, Change: "The price of a Credit changes to $40."},
		},
	}
	fixturePrivacy = legal.Document{
		Name: "Privacy Policy",
		Path: "/privacy",
		Versions: []legal.Version{
			{Effective: "2026-09-29", Material: false, Change: "First version."},
			{Effective: "2026-11-01", Material: false, Change: "Corrects a typing error."},
		},
	}
	fixtureDocs = []legal.Document{fixtureTerms, fixturePrivacy}
)

// newPublishServer mounts the publish endpoint behind clock.Middleware,
// so the instant it records is publishedAt.
func newPublishServer(t *testing.T, db *testdb.DB) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("POST "+publishURL, legalnotice.PublishHandler(db.App, internalauth.FromSecret(testSecret), fixtureDocs))
	srv := httptest.NewServer(clock.Middleware(func() time.Time { return publishedAt })(mux))
	t.Cleanup(srv.Close)
	return srv
}

// publish posts body to the publish endpoint with secret, and returns
// the status and the body it answered with.
func publish(t *testing.T, srv *httptest.Server, secret, body string) (status int, answer []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+publishURL, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Internal-Secret", secret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	answer, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read answer: %v", err)
	}
	return resp.StatusCode, answer
}

// publishOK publishes document's version and returns the record the
// endpoint answered with.
func publishOK(t *testing.T, srv *httptest.Server, document, version string) legalnotice.Publication {
	t.Helper()
	body, err := json.Marshal(legalnotice.PublishRequest{Document: document, Version: version, PublishedBy: founder})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	status, answer := publish(t, srv, testSecret, string(body))
	if status != http.StatusOK {
		t.Fatalf("publish %s %s: status = %d, want 200", document, version, status)
	}
	var p legalnotice.Publication
	if err := json.Unmarshal(answer, &p); err != nil {
		t.Fatalf("decode publication: %v", err)
	}
	return p
}

// runWorker drives one pass of the worker through the same endpoint the
// drain calls, with the notification door it opens in production.
func runWorker(t *testing.T, db *testdb.DB, sender mail.Sender, docs []legal.Document) {
	t.Helper()
	worker := legalnotice.NewWorker(outbox.Mailer{
		Sender: sender, Now: func() time.Time { return publishedAt.Add(time.Minute) },
		AppBaseURL: testAppBase, From: "DoulaCloud <notifications@example.test>", ReplyTo: "support@example.test",
	}, docs)
	srv := httptest.NewServer(outbox.ProcessHandler(db.App, worker, internalauth.FromSecret(testSecret), outbox.NotificationDoor))
	defer srv.Close()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+processURL, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Internal-Secret", testSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("process: status = %d", resp.StatusCode)
	}
}

// outboxRow is one legal_notice_outbox row as a test reads it.
type outboxRow struct {
	practiceID string
	staffID    string
	document   string
	version    string
	status     string
	createdAt  time.Time
	lastError  string
}

// outboxRows reads every legal_notice_outbox row, oldest first.
func outboxRows(t *testing.T, db *testdb.DB) []outboxRow {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT practice_id, staff_id, document, version::text, status::text, created_at, coalesce(last_error, '')
		   FROM legal_notice_outbox ORDER BY practice_id, staff_id`)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var got []outboxRow
	for rows.Next() {
		var r outboxRow
		if err := rows.Scan(&r.practiceID, &r.staffID, &r.document, &r.version, &r.status, &r.createdAt, &r.lastError); err != nil {
			t.Fatalf("scan outbox: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate outbox: %v", err)
	}
	return got
}

// statusOf is the status of the one row addressed to staffID at
// practiceID, and its last_error.
func statusOf(t *testing.T, db *testdb.DB, practiceID, staffID string) (status, lastError string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text, coalesce(last_error, '') FROM legal_notice_outbox WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, staffID,
	).Scan(&status, &lastError); err != nil {
		t.Fatalf("read row for %s at %s: %v", staffID, practiceID, err)
	}
	return status, lastError
}

// addOwner makes an existing Staff member an Owner of a second Practice.
func addOwner(t *testing.T, db *testdb.DB, practiceID, staffID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{owner}', $3)`,
		practiceID, staffID, employee,
	); err != nil {
		t.Fatalf("add owner: %v", err)
	}
}

// startDeletion puts practiceID in its 30-day deletion window.
func startDeletion(t *testing.T, db *testdb.DB, practiceID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET deletion_requested_at = now(), deletion_finalize_at = now() + interval '30 days' WHERE id = $1`,
		practiceID,
	); err != nil {
		t.Fatalf("start deletion: %v", err)
	}
}

// finishDeletion marks practiceID deleted.
func finishDeletion(t *testing.T, db *testdb.DB, practiceID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET deletion_requested_at = now(), deletion_finalize_at = now(), deleted_at = now() WHERE id = $1`,
		practiceID,
	); err != nil {
		t.Fatalf("finish deletion: %v", err)
	}
}
