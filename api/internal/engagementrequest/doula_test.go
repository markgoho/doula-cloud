package engagementrequest_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// The tests in this file are #1596: the Request names the Doula, or says
// there is none yet, and approval attaches her (ADR-0017 and ADR-0008,
// both amended on #1515).

const unknownStaffID = "11111111-2222-3333-4444-555555555555"

func requestsURL(base, practiceID, clientID string) string {
	return base + "/api/practices/" + practiceID + "/clients/" + clientID + "/engagement-requests"
}

func approveURL(base, practiceID, requestID string) string {
	return base + "/api/practices/" + practiceID + "/engagement-requests/" + requestID + "/approve"
}

// refusal is the error body docs/api-design.md section 7 gives every
// refusal.
type refusal struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details"`
}

func decodeRefusal(t *testing.T, resp response, want int) refusal {
	t.Helper()
	var out refusal
	decode(t, resp, want, &out)
	return out
}

// namedDoula reads who a Request names straight from the database.
func namedDoula(t *testing.T, db *testdb.DB, requestID string) *string {
	t.Helper()
	var doula *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT doula_staff_id::text FROM engagement_requests WHERE id = $1`, requestID,
	).Scan(&doula); err != nil {
		t.Fatalf("read named doula: %v", err)
	}
	return doula
}

// nameDoula sets who an already-seeded Request names, so pendingRequest
// does not grow a parameter every one of its other callers would pass.
func nameDoula(t *testing.T, db *testdb.DB, requestID, doulaStaffID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_requests SET doula_staff_id = $1 WHERE id = $2`, doulaStaffID, requestID,
	); err != nil {
		t.Fatalf("name doula on request: %v", err)
	}
}

// countRows counts table's rows at practiceID. table is a literal this
// file passes, never input.
func countRows(t *testing.T, db *testdb.DB, table, practiceID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM `+table+` WHERE practice_id = $1`, practiceID,
	).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

// attachment is one engagement_attachments row as ADR-0008 reads it.
type attachment struct {
	staffID, origin, attachedBy string
	feeAmountCents              *int64
	feeTerms                    *string
	open                        bool
}

func attachmentsOn(t *testing.T, db *testdb.DB, engagementID string) []attachment {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT staff_id::text, origin::text, attached_by::text, fee_amount_cents, fee_terms, ended_at IS NULL
		   FROM engagement_attachments WHERE engagement_id = $1`, engagementID)
	if err != nil {
		t.Fatalf("read attachments: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []attachment
	for rows.Next() {
		var a attachment
		if err := rows.Scan(&a.staffID, &a.origin, &a.attachedBy, &a.feeAmountCents, &a.feeTerms, &a.open); err != nil {
			t.Fatalf("scan attachment: %v", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate attachments: %v", err)
	}
	return out
}

// TestRequestHandler_APlainDoulaNamesHerself proves the ordinary path:
// the Request stores who it names, stays pending, and the approval
// screen's read carries her id and her name.
func TestRequestHandler_APlainDoulaNamesHerself(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	doulaID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-1", "Hana Kim", []string{doulaRole}, employeeType)
	testdb.SeedStaffAtPractice(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "client.com")

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	var out engagementrequest.RequestResponse
	decode(t, do(t, requestsURL(srv.URL, practiceID, clientID), session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &doulaID}),
		http.StatusCreated, &out)
	if out.State != testStatePending {
		t.Fatalf("state = %q, want pending", out.State)
	}
	if got := namedDoula(t, db, out.RequestID); got == nil || *got != doulaID {
		t.Fatalf("stored doula = %v, want %s", got, doulaID)
	}

	ownerSrv, ownerSession := newServer(t, db, "owner-1", &tasknudge.FakeEnqueuer{})
	defer ownerSrv.Close()
	var detail engagementrequest.DetailResponse
	decode(t, get(t, detailURL(ownerSrv.URL, practiceID, out.RequestID), ownerSession), http.StatusOK, &detail)
	if detail.DoulaStaffID == nil || *detail.DoulaStaffID != doulaID || detail.DoulaName == nil || *detail.DoulaName != "Hana Kim" {
		t.Fatalf("detail doula = %v / %v, want %s / Hana Kim", detail.DoulaStaffID, detail.DoulaName, doulaID)
	}
}

// TestRequestHandler_NoDoulaYetNamesNobody proves the other answer: an
// absent doulaStaffId and a blank one (spaces alone) both store no Doula, and the
// approval screen's read carries neither field.
func TestRequestHandler_NoDoulaYetNamesNobody(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	testdb.SeedStaffAtPractice(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "client.com")

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	blank := "  "
	var absent, blanked engagementrequest.RequestResponse
	decode(t, do(t, requestsURL(srv.URL, practiceID, clientID), session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate}), http.StatusCreated, &absent)
	decode(t, do(t, requestsURL(srv.URL, practiceID, clientID), session,
		engagementrequest.RequestBody{Kind: testKindPostpartum, DoulaStaffID: &blank}), http.StatusCreated, &blanked)
	for _, requestID := range []string{absent.RequestID, blanked.RequestID} {
		if got := namedDoula(t, db, requestID); got != nil {
			t.Fatalf("stored doula = %v, want none", *got)
		}
	}

	ownerSrv, ownerSession := newServer(t, db, "owner-1", &tasknudge.FakeEnqueuer{})
	defer ownerSrv.Close()
	resp := get(t, detailURL(ownerSrv.URL, practiceID, absent.RequestID), ownerSession)
	expectStatus(t, resp, http.StatusOK)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(resp.body, &raw); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if _, has := raw["doulaStaffId"]; has {
		t.Fatalf("detail carries doulaStaffId %s, want it absent for No Doula yet", raw["doulaStaffId"])
	}
	if _, has := raw["doulaName"]; has {
		t.Fatalf("detail carries doulaName %s, want it absent for No Doula yet", raw["doulaName"])
	}
}
