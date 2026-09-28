package export_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"doula-cloud/api/internal/testdb"
)

// TestHandler_NoCredentialOrKeyMaterialAnywhere is issue #288's own
// "not a judgment call" list: a seeded token, digest and access code
// must never reach the archive, in any file.
func TestHandler_NoCredentialOrKeyMaterialAnywhere(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-exclusion"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	const knownTokenDigest = "digest-should-never-leak-abc123"
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_invitations (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, 'invited@example.com', ARRAY['doula']::practice_role[], 'employee', $2, $3, now() + interval '7 days')`,
		practiceID, knownTokenDigest, staffID,
	); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/export")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	files := readZIP(t, resp)

	for name, content := range files {
		if strings.Contains(content, knownTokenDigest) {
			t.Fatalf("%s leaked the invitation's token_digest", name)
		}
	}
	inviteCSV := files["staff_invitation.csv"]
	if !strings.Contains(inviteCSV, "invited@example.com") {
		t.Fatalf("staff_invitation.csv should still carry the invitation's other columns: %s", inviteCSV)
	}
}

// TestHandler_ExportLeavesOutFeedback is #1523's own AC: charting Q7
// (#1498) settled that a piece of Feedback goes to DoulaCloud only and
// is never the Practice's record, so entities() must never name it --
// CONTEXT.md's Feedback entry restates the same rule ("a Practice's
// export leaves it out"). There is no feedback.csv to assert an absence
// of columns from, so this proves the omission the way #288's own
// "excluded, not a judgment call" list already does elsewhere: seed a
// piece naming this Practice and its Owner, then confirm nothing in the
// archive names it, in any file.
func TestHandler_ExportLeavesOutFeedback(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-feedback-exclusion"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	const distinctiveFeedbackText = "feedback-text-should-never-leave-the-database-x7q9"
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback
		    (id, kind, text, page_url, route_id, app_build, screen_width, browser,
		     staff_id, practice_id, roles, sent_at)
		 VALUES ($1, 'idea_or_request', $2, '/practices/x/clients/y', '/practices/[practiceId]/clients/[clientId]',
		         'abc1234', 390, 'Chrome 129', $3, $4, '{owner}'::practice_role[], now())`,
		uuid.NewString(), distinctiveFeedbackText, staffID, practiceID,
	); err != nil {
		t.Fatalf("seed feedback: %v", err)
	}

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/export")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	files := readZIP(t, resp)

	if _, ok := files["feedback.csv"]; ok {
		t.Fatal("archive carries a feedback.csv -- a piece of Feedback is never the Practice's record")
	}
	for name, content := range files {
		if strings.Contains(content, distinctiveFeedbackText) {
			t.Fatalf("%s leaked a piece of Feedback's free text", name)
		}
	}
}
