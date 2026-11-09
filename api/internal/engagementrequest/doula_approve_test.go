package engagementrequest_test

import (
	"encoding/json/v2"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
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
// attached her. #1625 holds it again when the Request names an Owner or an
// Admin who is a contractor Doula: the check runs again when the decision
// is made, and she is attached the same way.
func TestApproveHandler_AttachesTheNamedDoula(t *testing.T) {
	for _, tc := range []struct {
		name  string
		named []fixture.StaffOption
	}{
		{"a Doula", nil},
		{"an Owner who is a contractor Doula", []fixture.StaffOption{fixture.Roles(ownerRole, doulaRole), fixture.Contractor()}},
		{"an Admin who is a contractor Doula", []fixture.StaffOption{fixture.Roles(adminRole, doulaRole), fixture.Contractor()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Admin()), funded)
			named := e.Colleague(tc.named...)
			requestID := e.pending(named)
			e.nameDoula(requestID, named.ID)

			out := e.approve(requestID)

			e.requireAttached(out.EngagementID, named.ID, e.Staff.ID)
		})
	}
}

// TestApproveHandler_NoDoulaYetWritesNoAttachment proves the other
// answer at approval: the Engagement exists with nobody on it.
func TestApproveHandler_NoDoulaYetWritesNoAttachment(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()), funded)

	out := e.approve(e.pending(e.Colleague()))

	if got := attachmentsOn(t, e.DB, out.EngagementID); len(got) != 0 {
		t.Fatalf("attachments = %+v, want none for No Doula yet", got)
	}
	if actors, _ := doulaAttachedRows(t, e.DB, out.EngagementID); len(actors) != 0 {
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
		change func(e *env, doula fixture.StaffRecord)
		reason string
	}{
		{"her Membership ended", func(e *env, doula fixture.StaffRecord) {
			testdb.EndMembership(e.T, e.DB, e.Practice.ID, doula.ID, e.Staff.ID)
		}, "no longer Staff at this Practice"},
		{"she is now a contractor", func(e *env, doula fixture.StaffRecord) {
			setMembership(e, doula, "{doula}", contractorType)
		}, "Hana Kim is now a contractor"},
		{"she no longer holds the Doula role", func(e *env, doula fixture.StaffRecord) {
			setMembership(e, doula, "{admin}", employeeType)
		}, "Hana Kim no longer holds the Doula role"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Admin()), funded)
			doula := e.Colleague(fixture.StaffName("Hana Kim"))
			requestID := e.pending(e.Colleague())
			e.nameDoula(requestID, doula.ID)
			tc.change(e, doula)

			got := reqtest.Expect[apierr.APIError](t, http.MethodPost, e.requestURL(requestID, "/approve"), http.StatusConflict, e.SignedIn)
			if !strings.Contains(got.Message, tc.reason) {
				t.Fatalf("refusal message = %q, want it to name the reason %q", got.Message, tc.reason)
			}
			if len(got.Details) != 0 {
				t.Fatalf("refusal details = %v, want none: the approver amends no field", got.Details)
			}
			if row := readRequest(t, e.DB, requestID); row.state != testStatePending || row.decidedBy != nil {
				t.Fatalf("request row = %+v, want it still pending", row)
			}
			if n := countRows(t, e.DB, "engagements", e.Practice.ID); n != 0 {
				t.Fatalf("engagements rows = %d, want 0", n)
			}
			if n := creditLedgerCount(t, e.DB, e.Practice.ID); n != 1 {
				t.Fatalf("credit_ledger rows = %d, want 1 (the signup bonus, nothing spent)", n)
			}
		})
	}
}

// setMembership rewrites one Membership's roles and Employment type, the
// two facts an Owner changes on the Staff screen.
func setMembership(e *env, who fixture.StaffRecord, roles, employmentType string) {
	e.T.Helper()
	e.Exec(`UPDATE practice_memberships SET roles = $1::practice_role[], employment_type = $2::employment_type
	         WHERE practice_id = $3 AND staff_id = $4`,
		roles, employmentType, e.Practice.ID, who.ID)
}
