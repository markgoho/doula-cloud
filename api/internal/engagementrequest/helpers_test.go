package engagementrequest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
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
// GatedRouter and idempotency.Router, and seeds a live session for who.
func newServer(t *testing.T, db *testdb.DB, who fixture.StaffRecord, enq tasknudge.Enqueuer) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	engagementrequest.Mount(g, ir, db.App, enq)
	return httptest.NewServer(mux), fixture.Session(t, db, who)
}

// pendingRequest inserts a pending engagement_requests row directly,
// bypassing the endpoint, for tests exercising approve/refuse/withdraw in
// isolation.
func pendingRequest(t *testing.T, db *testdb.DB, p fixture.PracticeRecord, c fixture.ClientRecord, kind string, requestedBy fixture.StaffRecord) (requestID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, due_date, requested_by)
		 VALUES ($1, $2, $3::engagement_kind, $4, $5) RETURNING id`,
		p.ID, c.ID, kind, testDueDate, requestedBy.ID,
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
