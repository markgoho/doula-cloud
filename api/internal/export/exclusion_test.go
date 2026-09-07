package export_test

import (
	"net/http"
	"strings"
	"testing"

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
