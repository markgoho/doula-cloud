package engagementrequest_test

import (
	"database/sql"
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/reqtest"
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

	testStateRefused   = "refused"
	testStateWithdrawn = "withdrawn"

	notAUUID = "not-a-uuid"
	zeroUUID = "00000000-0000-0000-0000-000000000000"
)

// env is what a test in this package starts from: a Practice with this
// package's whole surface mounted on a server, one Staff member at the
// screen and a Client with no history. With no option she is an employee
// Doula and the Practice holds no Credits; a test says only what it changes,
// with signedInAs and withCredits.
type env struct {
	*fixture.Scene
	// Client is a Client at the Practice.
	Client fixture.ClientRecord
}

type envOption = fixture.SceneOption

var signedInAs = fixture.SignedInAs

// funded gives the Practice three Credits to approve with.
var funded = fixture.InPractice(fixture.Credits(3))

// newEnv mounts this package's whole surface through
// engagementrequest.Mount, the same call main.go makes on the real
// GatedRouter and idempotency.Router.
func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	mount := func(g *staffauth.GatedRouter, ir *idempotency.Router, app *sql.DB) {
		engagementrequest.Mount(g, ir, app, &tasknudge.FakeEnqueuer{})
	}
	s := fixture.NewScene(t, mount, opts...)
	return &env{Scene: s, Client: fixture.Client(t, s.DB, s.Practice)}
}

// requestURL is a Request's route plus tail, addressed the way the
// approval screen addresses it: a Request id and nothing else.
func (e *env) requestURL(requestID, tail string) string {
	return e.PracticeURL("/engagement-requests/" + requestID + tail)
}

func (e *env) listURL() string { return e.PracticeURL("/engagement-requests") }

func (e *env) askURL(clientID string) string {
	return e.PracticeURL("/clients/" + clientID + "/engagement-requests")
}

// pending inserts a pending birth Request for the env's Client, asked by
// requester, bypassing the endpoint, for tests exercising
// approve/refuse/withdraw in isolation.
func (e *env) pending(requester fixture.StaffRecord) (requestID string) {
	return e.pendingFor(e.Client, testKindBirth, requester)
}

func (e *env) pendingFor(c fixture.ClientRecord, kind string, requester fixture.StaffRecord) (requestID string) {
	e.T.Helper()
	return testdb.Scalar[string](e.T, e.DB.Admin,
		`INSERT INTO engagement_requests (practice_id, client_id, kind, due_date, requested_by)
		 VALUES ($1, $2, $3::engagement_kind, $4, $5) RETURNING id`,
		e.Practice.ID, c.ID, kind, testDueDate, requester.ID)
}

// approve approves a Request as the person at the screen.
func (e *env) approve(requestID string) engagementrequest.ApproveResponse {
	e.T.Helper()
	return reqtest.Expect[engagementrequest.ApproveResponse](e.T, http.MethodPost, e.requestURL(requestID, "/approve"), http.StatusOK, e.SignedIn)
}

// ask files the birth Request body for the env's Client as the person at the
// screen and returns the 201 answer.
func (e *env) ask(body engagementrequest.RequestBody) engagementrequest.RequestResponse {
	e.T.Helper()
	return reqtest.Expect[engagementrequest.RequestResponse](e.T, http.MethodPost, e.askURL(e.Client.ID), http.StatusCreated, e.SignedIn, reqtest.JSON(body))
}

// askRefused files the ask and requires the refusal want, without reading
// the answer.
func (e *env) askRefused(want int, body any) {
	e.T.Helper()
	e.askClientRefused(e.Client.ID, want, body)
}

func (e *env) askClientRefused(clientID string, want int, body any) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, http.MethodPost, e.askURL(clientID), want, e.SignedIn, reqtest.JSON(body))
}

var birthAsk = engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate}

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

// countRows counts table's rows at practiceID. table is a literal this
// package passes, never input.
func countRows(t *testing.T, db *testdb.DB, table, practiceID string) int {
	t.Helper()
	return testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM `+table+` WHERE practice_id = $1`, practiceID)
}

func creditLedgerCount(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	return countRows(t, db, "credit_ledger", practiceID)
}

func lowCreditOutboxCount(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	return countRows(t, db, "low_credit_outbox", practiceID)
}

// outboxCount counts engagement_request_outbox rows for requestID.
func outboxCount(t *testing.T, db *testdb.DB, requestID string) int {
	t.Helper()
	return testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM engagement_request_outbox WHERE request_id = $1`, requestID)
}

// setRequestNote writes a note onto an already-seeded Request, so a test
// can prove the note reaches the approver without pending growing a
// parameter every one of its other callers would have to pass.
func (e *env) setRequestNote(requestID, note string) {
	e.T.Helper()
	e.Exec(`UPDATE engagement_requests SET note = $1 WHERE id = $2`, note, requestID)
}

// clearDueDate removes a Request's due date after it is seeded, so a test
// can prove a postpartum-only ask travels without one -- ADR-0017's
// nullable due_date -- without pending growing a parameter every one of its
// other callers would have to pass.
func (e *env) clearDueDate(requestID string) {
	e.T.Helper()
	e.Exec(`UPDATE engagement_requests SET due_date = NULL WHERE id = $1`, requestID)
}
