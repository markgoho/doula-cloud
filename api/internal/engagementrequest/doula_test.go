package engagementrequest_test

import (
	"encoding/json/jsontext"
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// The tests in this file are #1596: the Request names the Doula, or says
// there is none yet, and approval attaches her (ADR-0017 and ADR-0008,
// both amended on #1515).

const unknownStaffID = "11111111-2222-3333-4444-555555555555"

// grantedOrigin is the attachment_origin of an Attachment that somebody
// decided, the only origin approval writes.
const grantedOrigin = "granted"

// namedDoula reads who a Request names straight from the database.
func namedDoula(t *testing.T, db *testdb.DB, requestID string) *string {
	t.Helper()
	return testdb.Scalar[*string](t, db.Admin, `SELECT doula_staff_id::text FROM engagement_requests WHERE id = $1`, requestID)
}

// nameDoula sets who an already-seeded Request names, so pending does not
// grow a parameter every one of its other callers would pass.
func (e *env) nameDoula(requestID, doulaStaffID string) {
	e.T.Helper()
	e.Exec(`UPDATE engagement_requests SET doula_staff_id = $1 WHERE id = $2`, doulaStaffID, requestID)
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

// requireAttached fails unless the Engagement holds exactly one open,
// granted, fee-free Attachment for doulaID attached by attachedByID --
// a fee is only ever copied from an Offer -- and exactly one
// doula_attached activity row by attachedByID that names doulaID.
func (e *env) requireAttached(engagementID, doulaID, attachedByID string) {
	e.T.Helper()
	got := attachmentsOn(e.T, e.DB, engagementID)
	if len(got) != 1 || got[0].staffID != doulaID || got[0].origin != grantedOrigin || got[0].attachedBy != attachedByID || !got[0].open {
		e.T.Fatalf("attachments = %+v, want one open granted one for %s, attached by %s", got, doulaID, attachedByID)
	}
	if got[0].feeAmountCents != nil || got[0].feeTerms != nil {
		e.T.Fatalf("attachment fee = %v / %v, want none: a fee is only ever copied from an Offer", got[0].feeAmountCents, got[0].feeTerms)
	}
	actors, doulas := doulaAttachedRows(e.T, e.DB, engagementID)
	if len(actors) != 1 || actors[0] != attachedByID || doulas[0] != doulaID {
		e.T.Fatalf("doula_attached rows = actors %v doulas %v, want one by %s naming %s", actors, doulas, attachedByID, doulaID)
	}
}

// TestRequestHandler_APlainDoulaNamesHerself proves the ordinary path:
// the Request stores who it names, stays pending, and the approval
// screen's read carries her id and her name.
func TestRequestHandler_APlainDoulaNamesHerself(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.StaffName("Hana Kim")))
	owner := e.Colleague(fixture.Owner())

	out := e.ask(engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &e.Staff.ID})
	if out.State != testStatePending {
		t.Fatalf("state = %q, want pending", out.State)
	}
	if got := namedDoula(t, e.DB, out.RequestID); got == nil || *got != e.Staff.ID {
		t.Fatalf("stored doula = %v, want %s", got, e.Staff.ID)
	}

	detail := reqtest.Expect[engagementrequest.DetailResponse](t, http.MethodGet, e.requestURL(out.RequestID, ""), http.StatusOK, e.As(owner))
	if detail.DoulaStaffID == nil || *detail.DoulaStaffID != e.Staff.ID || detail.DoulaName == nil || *detail.DoulaName != "Hana Kim" {
		t.Fatalf("detail doula = %v / %v, want %s / Hana Kim", detail.DoulaStaffID, detail.DoulaName, e.Staff.ID)
	}
}

// TestRequestHandler_NoDoulaYetNamesNobody proves the other answer: an
// absent doulaStaffId and a blank one (spaces alone) both store no Doula, and the
// approval screen's read carries neither field.
func TestRequestHandler_NoDoulaYetNamesNobody(t *testing.T) {
	e := newEnv(t)
	owner := e.Colleague(fixture.Owner())

	blank := "  "
	absent := e.ask(birthAsk)
	blanked := e.ask(engagementrequest.RequestBody{Kind: testKindPostpartum, DoulaStaffID: &blank})
	for _, requestID := range []string{absent.RequestID, blanked.RequestID} {
		if got := namedDoula(t, e.DB, requestID); got != nil {
			t.Fatalf("stored doula = %v, want none", *got)
		}
	}

	raw := reqtest.Expect[map[string]jsontext.Value](t, http.MethodGet, e.requestURL(absent.RequestID, ""), http.StatusOK, e.As(owner))
	if _, has := raw["doulaStaffId"]; has {
		t.Fatalf("detail carries doulaStaffId %s, want it absent for No Doula yet", raw["doulaStaffId"])
	}
	if _, has := raw["doulaName"]; has {
		t.Fatalf("detail carries doulaName %s, want it absent for No Doula yet", raw["doulaName"])
	}
}
