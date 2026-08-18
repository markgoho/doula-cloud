package staffauth_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// seedOwnerMembership seeds a Practice and a Staff member holding the
// 'owner' role there -- the only role InviteHandler and AssignRolesHandler
// accept as authorization.
func seedOwnerMembership(t *testing.T, db *testdb.DB, identityUID string) (staffID, practiceID string) {
	t.Helper()
	staffID, practiceID = seedStaffWithMembership(t, db, identityUID)
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE practice_memberships SET roles = '{owner}' WHERE staff_id = $1`, staffID); err != nil {
		t.Fatalf("promote to owner: %v", err)
	}
	return staffID, practiceID
}

func newInviteServer(verifier authntest.Verifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/invitations",
		staffauth.Middleware(verifier, db.App)(staffauth.InviteHandler()))
	return httptest.NewServer(mux)
}

func postInvite(t *testing.T, srv *httptest.Server, practiceID string, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/practices/"+practiceID+"/invitations", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestInviteHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-not-owner"
	_, practiceID := seedStaffWithMembership(t, db, identityUID) // seedMembership grants '{doula}', not owner

	srv := newInviteServer(authntest.Verifier{UID: identityUID}, db)
	defer srv.Close()

	resp := postInvite(t, srv, practiceID, staffauth.InviteRequest{Email: "invitee@example.com", Name: inviteeName})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestInviteHandler_MissingFields(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-missing-fields"
	_, practiceID := seedOwnerMembership(t, db, identityUID)

	srv := newInviteServer(authntest.Verifier{UID: identityUID}, db)
	defer srv.Close()

	resp := postInvite(t, srv, practiceID, staffauth.InviteRequest{Email: "", Name: inviteeName})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestInviteHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-invalid-body"
	_, practiceID := seedOwnerMembership(t, db, identityUID)

	srv := newInviteServer(authntest.Verifier{UID: identityUID}, db)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/practices/"+practiceID+"/invitations", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestInviteHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-invites"
	_, practiceID := seedOwnerMembership(t, db, identityUID)

	srv := newInviteServer(authntest.Verifier{UID: identityUID}, db)
	defer srv.Close()

	resp := postInvite(t, srv, practiceID, staffauth.InviteRequest{Email: "invitee@example.com", Name: inviteeName})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var out staffauth.InviteResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.StaffID == "" || out.InviteToken == "" {
		t.Fatalf("expected non-empty ids, got %+v", out)
	}

	var roles string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT array_to_string(roles, ',') FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, out.StaffID,
	).Scan(&roles); err != nil {
		t.Fatalf("query membership roles: %v", err)
	}
	if roles != "" {
		t.Fatalf("expected zero roles on the new membership, got %q", roles)
	}

	var identityUIDCol sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT identity_uid FROM staff WHERE id = $1`, out.StaffID).Scan(&identityUIDCol); err != nil {
		t.Fatalf("query pending staff: %v", err)
	}
	if identityUIDCol.Valid {
		t.Fatalf("expected pending staff row to have no identity_uid yet, got %q", identityUIDCol.String)
	}
}
