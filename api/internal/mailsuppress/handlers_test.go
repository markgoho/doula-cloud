package mailsuppress_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// newServer mounts this package's whole surface through mailsuppress.Mount,
// the same call main.go makes on the real GatedRouter and
// idempotency.Router.
func newServer(t *testing.T, db *testdb.DB, uid string, clearer mailsuppress.BounceClearer) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	mailsuppress.Mount(g, ir, clearer)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func request(t *testing.T, session, method, url string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, bytes.NewReader(body))
	if err != nil {
		// coverage:ignore reason: fixture failure, not exercised by the happy-path test
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// coverage:ignore reason: fixture failure, not exercised by the happy-path test
		t.Fatalf("request: %v", err)
	}
	return resp
}

func clearBody(t *testing.T, address string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]string{"address": address})
	if err != nil {
		// coverage:ignore reason: fixture failure, not exercised by the happy-path test
		t.Fatalf("marshal: %v", err)
	}
	return body
}

func TestListHandler_ShowsThisPracticesSuppressedAddresses(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Listing Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "lister", []string{roleOwner}, "employee")
	seedClientAt(t, db, practiceID, "bounced@example.test")
	seedClientAt(t, db, practiceID, "spammed@example.test")
	if err := mailsuppress.Record(t.Context(), db.App, "bounced@example.test", mailsuppress.CauseBounce, "evt-1"); err != nil {
		t.Fatalf("Record bounce: %v", err)
	}
	if err := mailsuppress.Record(t.Context(), db.App, "spammed@example.test", mailsuppress.CauseComplaint, "evt-2"); err != nil {
		t.Fatalf("Record complaint: %v", err)
	}
	srv, session := newServer(t, db, "lister", &mail.FakeSender{})
	defer srv.Close()

	resp := request(t, session, http.MethodGet, srv.URL+"/api/practices/"+practiceID+"/email-suppressions", nil)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got struct {
		Suppressions []struct {
			Address   string `json:"address"`
			Cause     string `json:"cause"`
			Clearable bool   `json:"clearable"`
		} `json:"suppressions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Suppressions) != 2 {
		t.Fatalf("listed %d suppressions, want 2", len(got.Suppressions))
	}
	// Clearable is the screen's affordance and ADR-0029's rule in one
	// field: a complaint row is shown, so Staff know why the mail stops,
	// but it offers nothing to press.
	for _, s := range got.Suppressions {
		if s.Cause == mailsuppress.CauseBounce && !s.Clearable {
			t.Fatal("a bounce is not offered as clearable")
		}
		if s.Cause == mailsuppress.CauseComplaint && s.Clearable {
			t.Fatal("a complaint is offered as clearable")
		}
	}
}

func TestClearHandler_ClearsABounce(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Clearing Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "clearer", []string{roleAdmin}, "employee")
	seedClientAt(t, db, practiceID, testAddress)
	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseBounce, "evt-1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	clearer := &mail.FakeSender{}
	srv, session := newServer(t, db, "clearer", clearer)
	defer srv.Close()

	resp := request(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/email-suppressions/clear", clearBody(t, testAddress))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if len(clearer.Deleted()) != 1 {
		t.Fatal("the clear never reached Mailgun's own bounce list")
	}
	active, err := mailsuppress.Active(t.Context(), db.App, testAddress)
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if active {
		t.Fatal("the address is still suppressed after a 204")
	}
}

// ADR-0029's rule has to hold at the endpoint, not only in the screen:
// the shared domain's reputation is what a second complaint costs.
func TestClearHandler_RefusesAComplaint(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Complaint Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "clearer", []string{roleOwner}, "employee")
	seedClientAt(t, db, practiceID, testAddress)
	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseComplaint, "evt-1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	clearer := &mail.FakeSender{}
	srv, session := newServer(t, db, "clearer", clearer)
	defer srv.Close()

	resp := request(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/email-suppressions/clear", clearBody(t, testAddress))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	if len(clearer.Deleted()) != 0 {
		t.Fatal("a refused complaint still reached Mailgun")
	}
}

// email_suppressions carries no practice_id and no RLS policy, so the
// handler is the only thing standing between one Practice and another's
// addresses.
func TestClearHandler_RefusesAnotherPracticesAddress(t *testing.T) {
	db := testdb.New(t)
	mine := testdb.SeedPractice(t, db, "Mine")
	theirs := testdb.SeedPractice(t, db, "Theirs")
	testdb.SeedStaffAtPractice(t, db, mine, "outsider", []string{roleOwner}, "employee")
	seedClientAt(t, db, theirs, testAddress)
	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseBounce, "evt-1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	clearer := &mail.FakeSender{}
	srv, session := newServer(t, db, "outsider", clearer)
	defer srv.Close()

	resp := request(t, session, http.MethodPost, srv.URL+"/api/practices/"+mine+"/email-suppressions/clear", clearBody(t, testAddress))
	defer func() { _ = resp.Body.Close() }()
	// 404, not 403: a Practice must not learn from this endpoint that
	// another Practice's Client is suppressed.
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if len(clearer.Deleted()) != 0 {
		t.Fatal("another Practice's address was cleared at Mailgun")
	}
	active, err := mailsuppress.Active(t.Context(), db.App, testAddress)
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if !active {
		t.Fatal("another Practice's suppression was lifted")
	}
}

func TestClearHandler_UnsuppressedAddressIs404(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Quiet Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "clearer", []string{roleOwner}, "employee")
	seedClientAt(t, db, practiceID, testAddress)
	srv, session := newServer(t, db, "clearer", &mail.FakeSender{})
	defer srv.Close()

	resp := request(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/email-suppressions/clear", clearBody(t, testAddress))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// A Doula is neither Owner nor Admin: ADR-0008 keeps the roster's own
// addresses in the same hands as the roster.
func TestClearHandler_RefusesADoula(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Doula Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "doula-only", []string{roleDoula}, "employee")
	seedClientAt(t, db, practiceID, testAddress)
	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseBounce, "evt-1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	srv, session := newServer(t, db, "doula-only", &mail.FakeSender{})
	defer srv.Close()

	resp := request(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/email-suppressions/clear", clearBody(t, testAddress))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestClearHandler_RejectsAMalformedBody(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Malformed Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "clearer", []string{roleOwner}, "employee")
	srv, session := newServer(t, db, "clearer", &mail.FakeSender{})
	defer srv.Close()

	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"not JSON", []byte("{")},
		{"no address", clearBody(t, "")},
		{"blank address", clearBody(t, "   ")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := request(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/email-suppressions/clear", tc.body)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

// Mailgun is the side that can refuse, and the local row must survive it
// unchanged -- otherwise the list says an address is usable that Mailgun
// still blocks.
func TestClearHandler_MailgunFailureIs502AndChangesNothing(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Unreachable Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "clearer", []string{roleOwner}, "employee")
	seedClientAt(t, db, practiceID, testAddress)
	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseBounce, "evt-1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	srv, session := newServer(t, db, "clearer", &mail.FakeSender{DeleteErr: errBoom})
	defer srv.Close()

	resp := request(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/email-suppressions/clear", clearBody(t, testAddress))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	active, err := mailsuppress.Active(t.Context(), db.App, testAddress)
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if !active {
		t.Fatal("a Mailgun failure cleared the local row anyway")
	}
}
