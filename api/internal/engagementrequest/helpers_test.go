package engagementrequest_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	ownerRole      = "owner"
	adminRole      = "admin"
	doulaRole      = "doula"
	contractorType = "contractor"
	employeeType   = "employee"
	testDueDate    = "2027-01-01"

	testKindBirth      = "birth"
	testKindPostpartum = "postpartum"
	testStatePending   = "pending"
	testStateApproved  = "approved"
)

// newServer mounts this package's whole surface through
// engagementrequest.Mount, the same call main.go makes on the real
// GatedRouter and idempotency.Router, and seeds a live session for uid.
func newServer(t *testing.T, db *testdb.DB, uid string, enq tasknudge.Enqueuer) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	engagementrequest.Mount(g, ir, db.App, enq)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// response is one finished exchange -- status and body, with the
// http.Response already closed.
type response struct {
	status int
	body   []byte
}

// do POSTs body (or an empty body when nil) carrying session's __session
// cookie, reads the whole response, and closes it. Every endpoint in this
// package is a POST, so the method is fixed rather than a parameter.
func do(t *testing.T, url, session string, body any) response {
	t.Helper()
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		payload = encoded
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(payload))
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

// get GETs url carrying session's __session cookie -- the one read path
// in this package (DetailHandler), next to do's POST-only exchanges.
func get(t *testing.T, url, session string) response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
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
// at practiceID with the given roles and employment type.
func seedMember(t *testing.T, db *testdb.DB, practiceID, identityUID string, roles []string, employmentType string) (staffID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, $2, $3, 'NY') RETURNING id`,
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

// seedClient inserts a bare Client row at practiceID.
func seedClient(t *testing.T, db *testdb.DB, practiceID string) (clientID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Test Client', 'client@example.com') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	return clientID
}

// seedEngagement inserts an Engagement for clientID at practiceID with
// the given status ('intake', 'active', 'postpartum', or 'completed').
func seedEngagement(t *testing.T, db *testdb.DB, practiceID, clientID, status string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind, status) VALUES ($1, $2, 'birth', $3::engagement_status)`,
		clientID, practiceID, status,
	); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
}

// seedCredits appends a signup_bonus credit_ledger row giving practiceID
// a balance of 3 -- every test that needs a nonzero balance needs the
// same amount, so this takes no quantity parameter.
func seedCredits(t *testing.T, db *testdb.DB, practiceID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', 3)`,
		practiceID,
	); err != nil {
		t.Fatalf("seed credits: %v", err)
	}
}

// pendingRequest inserts a pending engagement_requests row directly,
// bypassing the endpoint, for tests exercising approve/refuse/withdraw in
// isolation.
func pendingRequest(t *testing.T, db *testdb.DB, practiceID, clientID, kind, requestedBy string) (requestID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, due_date, requested_by)
		 VALUES ($1, $2, $3::engagement_kind, $4, $5) RETURNING id`,
		practiceID, clientID, kind, testDueDate, requestedBy,
	).Scan(&requestID); err != nil {
		t.Fatalf("seed pending request: %v", err)
	}
	return requestID
}

// requestRow reads a Request's decision facts straight from the
// database, bypassing RLS.
type requestRow struct {
	state        string
	decidedBy    *string
	engagementID *string
	reason       *string
}

func readRequest(t *testing.T, db *testdb.DB, requestID string) requestRow {
	t.Helper()
	var row requestRow
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT state::text, decided_by::text, engagement_id::text, reason FROM engagement_requests WHERE id = $1`,
		requestID,
	).Scan(&row.state, &row.decidedBy, &row.engagementID, &row.reason); err != nil {
		t.Fatalf("read request: %v", err)
	}
	return row
}

// engagementRow reads an Engagement's kind and due_date straight from the
// database.
func engagementRow(t *testing.T, db *testdb.DB, engagementID string) (kind string, dueDate *string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT kind::text, due_date::text FROM engagements WHERE id = $1`, engagementID,
	).Scan(&kind, &dueDate); err != nil {
		t.Fatalf("read engagement: %v", err)
	}
	return kind, dueDate
}

// creditLedgerCount counts credit_ledger rows for practiceID.
func creditLedgerCount(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM credit_ledger WHERE practice_id = $1`, practiceID,
	).Scan(&count); err != nil {
		t.Fatalf("count credit_ledger: %v", err)
	}
	return count
}

// outboxCount counts engagement_request_outbox rows for requestID.
func outboxCount(t *testing.T, db *testdb.DB, requestID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM engagement_request_outbox WHERE request_id = $1`, requestID,
	).Scan(&count); err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	return count
}

// lowCreditOutboxCount counts low_credit_outbox rows for practiceID.
func lowCreditOutboxCount(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM low_credit_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&count); err != nil {
		t.Fatalf("count low_credit_outbox rows: %v", err)
	}
	return count
}

// setRequestNote writes a note onto an already-seeded Request, so a test
// can prove the note reaches the approver without pendingRequest growing
// a parameter every one of its other callers would have to pass.
func setRequestNote(t *testing.T, db *testdb.DB, requestID, note string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_requests SET note = $1 WHERE id = $2`, note, requestID,
	); err != nil {
		t.Fatalf("set request note: %v", err)
	}
}

// clearDueDate removes a Request's due date after it is seeded, so a test
// can prove a postpartum-only ask travels without one -- ADR-0017's
// nullable due_date -- without pendingRequest growing a parameter every
// one of its other callers would have to pass.
func clearDueDate(t *testing.T, db *testdb.DB, requestID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_requests SET due_date = NULL WHERE id = $1`, requestID,
	); err != nil {
		t.Fatalf("clear due date: %v", err)
	}
}
