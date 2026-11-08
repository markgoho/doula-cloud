package engagementrequest_test

import (
	"encoding/json/jsontext"
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// The tests in this file are #1596: the Request names the Doula, or says
// there is none yet, and approval attaches her (ADR-0017 and ADR-0008,
// both amended on #1515).

const unknownStaffID = "11111111-2222-3333-4444-555555555555"

// grantedOrigin is the attachment_origin of an Attachment that somebody
// decided, the only origin approval writes.
const grantedOrigin = "granted"

func requestsURL(base, practiceID, clientID string) string {
	return base + "/api/practices/" + practiceID + "/clients/" + clientID + "/engagement-requests"
}

func approveURL(base, practiceID, requestID string) string {
	return base + "/api/practices/" + practiceID + "/engagement-requests/" + requestID + "/approve"
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
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p, fixture.StaffName("Hana Kim"))
	owner := fixture.Staff(t, db, p, fixture.Owner())
	client := fixture.Client(t, db, p)

	srv, session := newServer(t, db, doula, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	out := reqtest.DecodeStatus[engagementrequest.RequestResponse](t, reqtest.Post(t, requestsURL(srv.URL, p.ID, client.ID), reqtest.Session(session), reqtest.JSON(engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &doula.ID})), http.StatusCreated)
	if out.State != testStatePending {
		t.Fatalf("state = %q, want pending", out.State)
	}
	if got := namedDoula(t, db, out.RequestID); got == nil || *got != doula.ID {
		t.Fatalf("stored doula = %v, want %s", got, doula.ID)
	}

	ownerSrv, ownerSession := newServer(t, db, owner, &tasknudge.FakeEnqueuer{})
	defer ownerSrv.Close()
	detail := reqtest.DecodeStatus[engagementrequest.DetailResponse](t, reqtest.Get(t, detailURL(ownerSrv.URL, p.ID, out.RequestID), reqtest.Session(ownerSession)), http.StatusOK)
	if detail.DoulaStaffID == nil || *detail.DoulaStaffID != doula.ID || detail.DoulaName == nil || *detail.DoulaName != "Hana Kim" {
		t.Fatalf("detail doula = %v / %v, want %s / Hana Kim", detail.DoulaStaffID, detail.DoulaName, doula.ID)
	}
}

// TestRequestHandler_NoDoulaYetNamesNobody proves the other answer: an
// absent doulaStaffId and a blank one (spaces alone) both store no Doula, and the
// approval screen's read carries neither field.
func TestRequestHandler_NoDoulaYetNamesNobody(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)
	owner := fixture.Staff(t, db, p, fixture.Owner())
	client := fixture.Client(t, db, p)

	srv, session := newServer(t, db, doula, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	blank := "  "
	absent := reqtest.DecodeStatus[engagementrequest.RequestResponse](t, reqtest.Post(t, requestsURL(srv.URL, p.ID, client.ID), reqtest.Session(session), reqtest.JSON(engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate})), http.StatusCreated)
	blanked := reqtest.DecodeStatus[engagementrequest.RequestResponse](t, reqtest.Post(t, requestsURL(srv.URL, p.ID, client.ID), reqtest.Session(session), reqtest.JSON(engagementrequest.RequestBody{Kind: testKindPostpartum, DoulaStaffID: &blank})), http.StatusCreated)
	for _, requestID := range []string{absent.RequestID, blanked.RequestID} {
		if got := namedDoula(t, db, requestID); got != nil {
			t.Fatalf("stored doula = %v, want none", *got)
		}
	}

	ownerSrv, ownerSession := newServer(t, db, owner, &tasknudge.FakeEnqueuer{})
	defer ownerSrv.Close()
	resp := reqtest.Get(t, detailURL(ownerSrv.URL, p.ID, absent.RequestID), reqtest.Session(ownerSession))
	reqtest.RequireStatus(t, resp, http.StatusOK)
	raw := reqtest.Decode[map[string]jsontext.Value](t, resp)
	if _, has := raw["doulaStaffId"]; has {
		t.Fatalf("detail carries doulaStaffId %s, want it absent for No Doula yet", raw["doulaStaffId"])
	}
	if _, has := raw["doulaName"]; has {
		t.Fatalf("detail carries doulaName %s, want it absent for No Doula yet", raw["doulaName"])
	}
}
