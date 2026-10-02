package engagementrequest_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/engagementrequest"
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
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	adminID := testdb.SeedStaffAtPractice(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	seedCredits(t, db, practiceID)
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "client.com")
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)
	nameDoula(t, db, requestID, doulaID)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	var out engagementrequest.ApproveResponse
	decode(t, do(t, approveURL(srv.URL, practiceID, requestID), session, nil), http.StatusOK, &out)

	got := attachmentsOn(t, db, out.EngagementID)
	if len(got) != 1 {
		t.Fatalf("attachments = %d, want exactly 1", len(got))
	}
	a := got[0]
	if a.staffID != doulaID || a.origin != "granted" || a.attachedBy != adminID || !a.open {
		t.Fatalf("attachment = %+v, want an open granted one for the Doula, attached by the approver", a)
	}
	if a.feeAmountCents != nil || a.feeTerms != nil {
		t.Fatalf("attachment fee = %v / %v, want none: a fee is only ever copied from an Offer", a.feeAmountCents, a.feeTerms)
	}

	actors, doulas := doulaAttachedRows(t, db, out.EngagementID)
	if len(actors) != 1 || actors[0] != adminID || doulas[0] != doulaID {
		t.Fatalf("doula_attached rows = actors %v doulas %v, want one by the approver naming the Doula", actors, doulas)
	}
}

// TestApproveHandler_NoDoulaYetWritesNoAttachment proves the other
// answer at approval: the Engagement exists with nobody on it.
func TestApproveHandler_NoDoulaYetWritesNoAttachment(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	testdb.SeedStaffAtPractice(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	seedCredits(t, db, practiceID)
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "client.com")
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	var out engagementrequest.ApproveResponse
	decode(t, do(t, approveURL(srv.URL, practiceID, requestID), session, nil), http.StatusOK, &out)

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
		change func(t *testing.T, db *testdb.DB, practiceID, doulaID, adminID string)
		reason string
	}{
		{"her Membership ended", func(t *testing.T, db *testdb.DB, practiceID, doulaID, adminID string) {
			t.Helper()
			testdb.EndMembership(t, db, practiceID, doulaID, adminID)
		}, "no longer Staff at this Practice"},
		{"she is now a contractor", func(t *testing.T, db *testdb.DB, practiceID, doulaID, _ string) {
			t.Helper()
			setMembership(t, db, practiceID, doulaID, "{doula}", contractorType)
		}, "Hana Kim is now a contractor"},
		{"she no longer holds the Doula role", func(t *testing.T, db *testdb.DB, practiceID, doulaID, _ string) {
			t.Helper()
			setMembership(t, db, practiceID, doulaID, "{admin}", employeeType)
		}, "Hana Kim no longer holds the Doula role"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			practiceID := testdb.SeedPractice(t, db, "Test Practice")
			askerID := testdb.SeedStaffAtPractice(t, db, practiceID, "asker-1", []string{doulaRole}, employeeType)
			doulaID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-1", "Hana Kim", []string{doulaRole}, employeeType)
			adminID := testdb.SeedStaffAtPractice(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
			seedCredits(t, db, practiceID)
			clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "client.com")
			requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, askerID)
			nameDoula(t, db, requestID, doulaID)
			tc.change(t, db, practiceID, doulaID, adminID)

			srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
			defer srv.Close()

			got := decodeRefusal(t, do(t, approveURL(srv.URL, practiceID, requestID), session, nil), http.StatusConflict)
			if !strings.Contains(got.Message, tc.reason) {
				t.Fatalf("refusal message = %q, want it to name the reason %q", got.Message, tc.reason)
			}
			if len(got.Details) != 0 {
				t.Fatalf("refusal details = %v, want none: the approver amends no field", got.Details)
			}
			if row := readRequest(t, db, requestID); row.state != testStatePending || row.decidedBy != nil {
				t.Fatalf("request row = %+v, want it still pending", row)
			}
			if n := countRows(t, db, "engagements", practiceID); n != 0 {
				t.Fatalf("engagements rows = %d, want 0", n)
			}
			if n := creditLedgerCount(t, db, practiceID); n != 1 {
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
