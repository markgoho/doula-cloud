package engagementrequest_test

import (
	"encoding/json/v2"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// doulaAttachedRows reads the doula_attached activity rows on one
// Engagement: who decided it, and the diff that names the Doula.
func doulaAttachedRows(t *testing.T, db *testdb.DB, engagementID string) (actors, doulas []string) {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT actor_staff_id::text, diff FROM activity
		  WHERE subject_kind = $1 AND subject_id = $2 AND action = $3`,
		activity.SubjectEngagement, engagementID, string(activity.ActionDoulaAttached))
	if err != nil {
		t.Fatalf("read activity: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var actor string
		var raw []byte
		if err := rows.Scan(&actor, &raw); err != nil {
			t.Fatalf("scan activity: %v", err)
		}
		var diff map[string]string
		if err := json.Unmarshal(raw, &diff); err != nil {
			t.Fatalf("decode diff: %v", err)
		}
		actors = append(actors, actor)
		doulas = append(doulas, diff[activity.DiffKeyAttachedStaffID])
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate activity: %v", err)
	}
	return actors, doulas
}

// TestApproveHandler_AttachesTheNamedDoula proves approval's new half:
// one granted Attachment for the named Doula, with no fee, attached_by
// the approver and not the asker, and one activity row that says who
// attached her.
func TestApproveHandler_AttachesTheNamedDoula(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.Credits(3))
	doula := fixture.Staff(t, db, p)
	admin := fixture.Staff(t, db, p, fixture.Admin())
	client := fixture.Client(t, db, p)
	requestID := pendingRequest(t, db, p, client, testKindBirth, doula)
	nameDoula(t, db, requestID, doula.ID)

	srv, session := newServer(t, db, admin, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	outResp := reqtest.Post(t, approveURL(srv.URL, p.ID, requestID), reqtest.Session(session))
	defer outResp.Body.Close()
	out := reqtest.DecodeStatus[engagementrequest.ApproveResponse](t, outResp, http.StatusOK)

	got := attachmentsOn(t, db, out.EngagementID)
	if len(got) != 1 {
		t.Fatalf("attachments = %d, want exactly 1", len(got))
	}
	a := got[0]
	if a.staffID != doula.ID || a.origin != grantedOrigin || a.attachedBy != admin.ID || !a.open {
		t.Fatalf("attachment = %+v, want an open granted one for the Doula, attached by the approver", a)
	}
	if a.feeAmountCents != nil || a.feeTerms != nil {
		t.Fatalf("attachment fee = %v / %v, want none: a fee is only ever copied from an Offer", a.feeAmountCents, a.feeTerms)
	}

	actors, doulas := doulaAttachedRows(t, db, out.EngagementID)
	if len(actors) != 1 || actors[0] != admin.ID || doulas[0] != doula.ID {
		t.Fatalf("doula_attached rows = actors %v doulas %v, want one by the approver naming the Doula", actors, doulas)
	}
}

// TestApproveHandler_AttachesAnOwnerOrAdminWhoIsAContractorDoula is
// #1625 at approval, the check that runs again when the decision is
// made: a Request that names an Owner or an Admin who is a contractor
// Doula is approved, and she is attached with no fee, attached_by the
// approver, and one activity row that says who attached her.
func TestApproveHandler_AttachesAnOwnerOrAdminWhoIsAContractorDoula(t *testing.T) {
	for _, role := range []string{ownerRole, adminRole} {
		t.Run(role, func(t *testing.T) {
			db := testdb.New(t)
			p := fixture.Practice(t, db, fixture.Credits(3))
			named := fixture.Staff(t, db, p, fixture.Roles(role, doulaRole), fixture.Contractor())
			approver := fixture.Staff(t, db, p, fixture.Admin())
			client := fixture.Client(t, db, p)
			requestID := pendingRequest(t, db, p, client, testKindBirth, named)
			nameDoula(t, db, requestID, named.ID)

			srv, session := newServer(t, db, approver, &tasknudge.FakeEnqueuer{})
			defer srv.Close()

			outResp := reqtest.Post(t, approveURL(srv.URL, p.ID, requestID), reqtest.Session(session))
			defer outResp.Body.Close()
			out := reqtest.DecodeStatus[engagementrequest.ApproveResponse](t, outResp, http.StatusOK)

			got := attachmentsOn(t, db, out.EngagementID)
			if len(got) != 1 || got[0].staffID != named.ID || got[0].origin != grantedOrigin || got[0].attachedBy != approver.ID || !got[0].open {
				t.Fatalf("attachments = %+v, want one open granted one for the named Doula, attached by the approver", got)
			}
			if got[0].feeAmountCents != nil || got[0].feeTerms != nil {
				t.Fatalf("attachment fee = %v / %v, want none: a fee is only ever copied from an Offer", got[0].feeAmountCents, got[0].feeTerms)
			}
			actors, doulas := doulaAttachedRows(t, db, out.EngagementID)
			if len(actors) != 1 || actors[0] != approver.ID || doulas[0] != named.ID {
				t.Fatalf("doula_attached rows = actors %v doulas %v, want one by the approver naming the Doula", actors, doulas)
			}
		})
	}
}

// TestApproveHandler_NoDoulaYetWritesNoAttachment proves the other
// answer at approval: the Engagement exists with nobody on it.
func TestApproveHandler_NoDoulaYetWritesNoAttachment(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.Credits(3))
	doula := fixture.Staff(t, db, p)
	admin := fixture.Staff(t, db, p, fixture.Admin())
	client := fixture.Client(t, db, p)
	requestID := pendingRequest(t, db, p, client, testKindBirth, doula)

	srv, session := newServer(t, db, admin, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	outResp := reqtest.Post(t, approveURL(srv.URL, p.ID, requestID), reqtest.Session(session))
	defer outResp.Body.Close()
	out := reqtest.DecodeStatus[engagementrequest.ApproveResponse](t, outResp, http.StatusOK)

	if got := attachmentsOn(t, db, out.EngagementID); len(got) != 0 {
		t.Fatalf("attachments = %+v, want none for No Doula yet", got)
	}
	if actors, _ := doulaAttachedRows(t, db, out.EngagementID); len(actors) != 0 {
		t.Fatalf("doula_attached rows = %d, want none", len(actors))
	}
}

// TestApproveHandler_RefusesADoulaWhoCanNoLongerBeAttached is ADR-0017's
// "approval is refused with that reason and the Request stays pending":
// between the ask and the decision her Membership ended, her Employment
// type became contractor, or she stopped holding the Doula role. Each
// refusal is a 409 whose message names its own reason, and nothing is
// written -- no Engagement, no Credit spent, no Attachment.
func TestApproveHandler_RefusesADoulaWhoCanNoLongerBeAttached(t *testing.T) {
	cases := []struct {
		name   string
		change func(t *testing.T, db *testdb.DB, p fixture.PracticeRecord, doula, admin fixture.StaffRecord)
		reason string
	}{
		{"her Membership ended", func(t *testing.T, db *testdb.DB, p fixture.PracticeRecord, doula, admin fixture.StaffRecord) {
			t.Helper()
			testdb.EndMembership(t, db, p.ID, doula.ID, admin.ID)
		}, "no longer Staff at this Practice"},
		{"she is now a contractor", func(t *testing.T, db *testdb.DB, p fixture.PracticeRecord, doula, _ fixture.StaffRecord) {
			t.Helper()
			setMembership(t, db, p.ID, doula.ID, "{doula}", contractorType)
		}, "Hana Kim is now a contractor"},
		{"she no longer holds the Doula role", func(t *testing.T, db *testdb.DB, p fixture.PracticeRecord, doula, _ fixture.StaffRecord) {
			t.Helper()
			setMembership(t, db, p.ID, doula.ID, "{admin}", employeeType)
		}, "Hana Kim no longer holds the Doula role"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			p := fixture.Practice(t, db, fixture.Credits(3))
			asker := fixture.Staff(t, db, p)
			doula := fixture.Staff(t, db, p, fixture.StaffName("Hana Kim"))
			admin := fixture.Staff(t, db, p, fixture.Admin())
			client := fixture.Client(t, db, p)
			requestID := pendingRequest(t, db, p, client, testKindBirth, asker)
			nameDoula(t, db, requestID, doula.ID)
			tc.change(t, db, p, doula, admin)

			srv, session := newServer(t, db, admin, &tasknudge.FakeEnqueuer{})
			defer srv.Close()

			gotResp := reqtest.Post(t, approveURL(srv.URL, p.ID, requestID), reqtest.Session(session))
			defer gotResp.Body.Close()
			got := reqtest.DecodeStatus[apierr.APIError](t, gotResp, http.StatusConflict)
			if !strings.Contains(got.Message, tc.reason) {
				t.Fatalf("refusal message = %q, want it to name the reason %q", got.Message, tc.reason)
			}
			if len(got.Details) != 0 {
				t.Fatalf("refusal details = %v, want none: the approver amends no field", got.Details)
			}
			if row := readRequest(t, db, requestID); row.state != testStatePending || row.decidedBy != nil {
				t.Fatalf("request row = %+v, want it still pending", row)
			}
			if n := countRows(t, db, "engagements", p.ID); n != 0 {
				t.Fatalf("engagements rows = %d, want 0", n)
			}
			if n := creditLedgerCount(t, db, p.ID); n != 1 {
				t.Fatalf("credit_ledger rows = %d, want 1 (the signup bonus, nothing spent)", n)
			}
		})
	}
}

// setMembership rewrites one Membership's roles and Employment type, the
// two facts an Owner changes on the Staff screen.
func setMembership(t *testing.T, db *testdb.DB, practiceID, staffID, roles, employmentType string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practice_memberships SET roles = $1::practice_role[], employment_type = $2::employment_type
		  WHERE practice_id = $3 AND staff_id = $4`,
		roles, employmentType, practiceID, staffID,
	); err != nil {
		t.Fatalf("set membership: %v", err)
	}
}
