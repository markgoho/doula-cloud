package portalinvite_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/testdb"
)

func TestInviteHandler_EngagementNotFound(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "invite-engagement-not-found"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv, session := newInviteServer(t, db, identityUID)
	defer srv.Close()

	resp := postInvite(t, srv, session, practiceID, "00000000-0000-0000-0000-000000000000")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestInviteHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "invite-invalid-engagement-id"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv, session := newInviteServer(t, db, identityUID)
	defer srv.Close()

	resp := postInvite(t, srv, session, practiceID, "not-a-uuid")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestInviteHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "invite-success-staff"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "New Client", "new@example.com")

	srv, session := newInviteServer(t, db, identityUID)
	defer srv.Close()

	resp := postInvite(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var out portalinvite.InviteResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ClientPortalUserID == "" || out.InviteToken == "" {
		t.Fatalf("expected non-empty ids, got %+v", out)
	}

	var gotClientID string
	var identityUIDCol sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT client_id, identity_uid FROM client_portal_users WHERE id = $1`, out.ClientPortalUserID,
	).Scan(&gotClientID, &identityUIDCol); err != nil {
		t.Fatalf("query pending row: %v", err)
	}
	if gotClientID != clientID {
		t.Fatalf("client_id = %q, want %q", gotClientID, clientID)
	}
	if identityUIDCol.Valid {
		t.Fatalf("expected pending row to have no identity_uid yet, got %q", identityUIDCol.String)
	}

	var outboxStatus string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status FROM portal_invite_outbox WHERE client_portal_user_id = $1`, out.ClientPortalUserID,
	).Scan(&outboxStatus); err != nil {
		t.Fatalf("query outbox row: %v", err)
	}
	if outboxStatus != testOutboxStatusPending {
		t.Fatalf("outbox status = %q, want %s", outboxStatus, testOutboxStatusPending)
	}
}

func TestInviteHandler_ReinviteRotatesToken(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "invite-reinvite-staff"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Reinvited Client", "reinvited@example.com")

	srv, session := newInviteServer(t, db, identityUID)
	defer srv.Close()

	first := postInvite(t, srv, session, practiceID, engagementID)
	var firstOut portalinvite.InviteResponse
	if err := json.NewDecoder(first.Body).Decode(&firstOut); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	_ = first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", first.StatusCode, http.StatusCreated)
	}

	second := postInvite(t, srv, session, practiceID, engagementID)
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want %d", second.StatusCode, http.StatusOK)
	}
	var secondOut portalinvite.InviteResponse
	if err := json.NewDecoder(second.Body).Decode(&secondOut); err != nil {
		t.Fatalf("decode second response: %v", err)
	}

	if secondOut.ClientPortalUserID != firstOut.ClientPortalUserID {
		t.Fatalf("re-invite created a second row: %q != %q", secondOut.ClientPortalUserID, firstOut.ClientPortalUserID)
	}
	if secondOut.InviteToken == firstOut.InviteToken {
		t.Fatalf("expected invite_token to rotate, got the same token twice")
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM client_portal_users WHERE client_id = (SELECT client_id FROM client_portal_users WHERE id = $1)`,
		firstOut.ClientPortalUserID,
	).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one row for the client after re-invite, got %d", count)
	}

	// The re-invite reuses the same pending outbox row (queueOutboxSend's
	// ON CONFLICT), rather than forking a second one racing to send the
	// same portal user's invite.
	var outboxCount, outboxAttemptCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(attempt_count) FROM portal_invite_outbox WHERE client_portal_user_id = $1`,
		firstOut.ClientPortalUserID,
	).Scan(&outboxCount, &outboxAttemptCount); err != nil {
		t.Fatalf("query outbox rows: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("expected exactly one outbox row after re-invite, got %d", outboxCount)
	}
	if outboxAttemptCount != 0 {
		t.Fatalf("expected the re-invite to reset attempt_count to 0, got %d", outboxAttemptCount)
	}
}

func TestInviteHandler_AlreadyAcceptedConflict(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "invite-already-accepted-staff"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Accepted Client", "accepted@example.com")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ('already-accepted-uid', $1)`,
		clientID,
	); err != nil {
		t.Fatalf("seed accepted portal user: %v", err)
	}

	srv, session := newInviteServer(t, db, identityUID)
	defer srv.Close()

	resp := postInvite(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}

	var out apierr.APIError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Code != "CONFLICT" {
		t.Fatalf("code = %q, want %q", out.Code, "CONFLICT")
	}
}
