package offer_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	doulaRole      = "doula"
	ownerRole      = "owner"
	contractorType = "contractor"
	employeeType   = "employee"

	// The offer_state values these tests assert on, and the two decidable
	// facts they seed every Offer with.
	stateExpired   = "expired"
	stateDeclined  = "declined"
	stateWithdrawn = "withdrawn"
	testClientArea = "North side"
	testDueDate    = "2027-01-01"
	testAddress    = "renata@example.test"
)

// newServer mounts the same routes main.go wires up for this package,
// behind staffauth.Middleware, and seeds a live session for uid --
// returning the token its __session cookie carries.
func newServer(t *testing.T, db *testdb.DB, uid string, enq tasknudge.Enqueuer) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/offers",
		staffauth.Middleware(db.App)(offer.CreateHandler(enq)))
	mux.Handle("GET /practices/{practiceId}/engagements/{engagementId}/offers",
		staffauth.Middleware(db.App)(offer.EngagementListHandler()))
	mux.Handle("GET /practices/{practiceId}/offers",
		staffauth.Middleware(db.App)(offer.InboxHandler()))
	mux.Handle("POST /practices/{practiceId}/offers/{offerId}/accept",
		staffauth.Middleware(db.App)(offer.AcceptHandler()))
	mux.Handle("POST /practices/{practiceId}/offers/{offerId}/decline",
		staffauth.Middleware(db.App)(offer.DeclineHandler()))
	mux.Handle("POST /practices/{practiceId}/offers/{offerId}/withdraw",
		staffauth.Middleware(db.App)(offer.WithdrawHandler()))
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/complete",
		staffauth.Middleware(db.App)(engagement.CompleteHandler()))
	mux.Handle("GET /offers/{offerId}", offer.ReadHandler(db.App))
	mux.Handle("POST /offers/{offerId}/decline", offer.DeclineByTokenHandler(db.App))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// seedSessionFor mints a live session for a second identity, so one test
// can act as two different people against the same server.
func seedSessionFor(t *testing.T, db *testdb.DB, uid string) string {
	t.Helper()
	return authntest.SeedSession(t, db.App, uid)
}

// response is one finished exchange -- status and body, with the
// http.Response already closed. Tests read this rather than a live
// *http.Response so a leaked body is not something a call site can
// forget.
type response struct {
	status int
	body   []byte
}

// do sends a request carrying session's __session cookie, reads the
// whole response, and closes it.
func do(t *testing.T, method, url, session string, body any) response {
	t.Helper()
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		payload = encoded
	}
	req, err := http.NewRequestWithContext(t.Context(), method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if session != "" {
		authntest.AddSessionCookie(req, session)
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

// decode unmarshals a response body into out, failing the test if the
// status is not want.
func decode(t *testing.T, resp response, want int, out any) {
	t.Helper()
	expectStatus(t, resp, want)
	if out == nil {
		return
	}
	if err := json.Unmarshal(resp.body, out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
}

// expectStatus fails unless resp's status is want.
func expectStatus(t *testing.T, resp response, want int) {
	t.Helper()
	if resp.status != want {
		t.Fatalf("status = %d, want %d: %s", resp.status, want, resp.body)
	}
}

// seedPractice inserts a bare Practice row.
func seedPractice(t *testing.T, db *testdb.DB) (practiceID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ('Test Practice') RETURNING id`,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	return practiceID
}

// seedMember inserts a Staff row bound to identityUID plus a Membership
// at practiceID with the given roles and employment type, using the
// superuser Admin connection so fixture setup isn't gated by the policies
// under test.
func seedMember(t *testing.T, db *testdb.DB, practiceID, identityUID string, roles []string, employmentType string) (staffID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, $2, $3) RETURNING id`,
		identityUID, "Staff "+identityUID, identityUID+"@example.com",
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type)
		 VALUES ($1, $2, $3::practice_role[], $4::employment_type)`,
		practiceID, staffID, "{"+strings.Join(roles, ",")+"}", employmentType,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return staffID
}

// seedEngagement inserts a Client and an Engagement at practiceID.
func seedEngagement(t *testing.T, db *testdb.DB, practiceID string) (engagementID string) {
	t.Helper()
	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (name, email) VALUES ('Test Client', 'client@example.com') RETURNING id`,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id) VALUES ($1, $2) RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return engagementID
}

// offerBody is a valid contractor-target CreateRequest for staffID, with
// the fee the fee CHECK requires.
func offerBody(staffID string, amountCents int64) offer.CreateRequest {
	return offer.CreateRequest{
		StaffID:            staffID,
		AmountCents:        &amountCents,
		Terms:              "Two prenatal visits, on call from 38 weeks.",
		ClientFirstInitial: "R",
		ClientArea:         testClientArea,
		DueDate:            time.Now().AddDate(0, 3, 0).Format(time.DateOnly),
	}
}

// emailOfferBody is a valid email-target CreateRequest. An emailed
// Invitation always joins her as a contractor, so a fee is always
// required on this path.
func emailOfferBody(email string, amountCents *int64) offer.CreateRequest {
	return offer.CreateRequest{
		Email:              email,
		AmountCents:        amountCents,
		Terms:              "Two prenatal visits.",
		ClientFirstInitial: "R",
		ClientArea:         testClientArea,
		DueDate:            time.Now().AddDate(0, 3, 0).Format(time.DateOnly),
	}
}

// offerState reads one Offer's state and decided_by straight from the
// database, bypassing RLS -- the audit facts the API deliberately does
// not serve.
func offerState(t *testing.T, db *testdb.DB, offerID string) (state string, decidedBy *string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT state::text, decided_by::text FROM engagement_offers WHERE id = $1`, offerID,
	).Scan(&state, &decidedBy); err != nil {
		t.Fatalf("read offer state: %v", err)
	}
	return state, decidedBy
}

// outboxCredentials reads the plaintext token and code the worker would
// mail for offerID -- the only place either exists in the clear.
func outboxCredentials(t *testing.T, db *testdb.DB, offerID string) (token, code string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT invite_token::text, access_code FROM engagement_offer_outbox WHERE offer_id = $1`, offerID,
	).Scan(&token, &code); err != nil {
		t.Fatalf("read outbox credentials: %v", err)
	}
	return token, code
}

// expireOffer backdates an Offer's expires_at so an expiry path can be
// exercised without waiting seven days.
func expireOffer(t *testing.T, db *testdb.DB, offerID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_offers SET expires_at = now() - interval '1 hour' WHERE id = $1`, offerID,
	); err != nil {
		t.Fatalf("expire offer: %v", err)
	}
}
