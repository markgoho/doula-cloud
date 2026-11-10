package portalinvite_test

import (
	"database/sql"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// invite posts the portal invite of engagementID as the Staff member at
// the screen and requires the status want.
func invite(s *fixture.Scene, engagementID string, want int) {
	s.T.Helper()
	reqtest.ExpectStatus(s.T, http.MethodPost, s.PracticeURL(invitePath(engagementID)), want, s.SignedIn)
}

// inviteSucceeding posts an invite that succeeds with status want and
// returns its body.
func inviteSucceeding(s *fixture.Scene, engagementID string, want int) portalinvite.InviteResponse {
	s.T.Helper()
	return reqtest.Expect[portalinvite.InviteResponse](s.T, http.MethodPost, s.PracticeURL(invitePath(engagementID)), want, s.SignedIn)
}

// inviteTokenExpiresAt reads back the current invite_token_expires_at for
// portalUserID, so a test can prove a re-invite moves it forward rather
// than just rotating the token.
func inviteTokenExpiresAt(t *testing.T, db *testdb.DB, portalUserID string) time.Time {
	t.Helper()
	expiresAt := testdb.Scalar[time.Time](t, db.Admin, `SELECT invite_token_expires_at FROM client_portal_users WHERE id = $1`, portalUserID)
	return expiresAt
}

func TestInviteHandler_RefusesAnEngagementItCannotUse(t *testing.T) {
	for name, c := range map[string]struct {
		engagementID string
		want         int
	}{
		"one that does not exist":  {unknownUUID, http.StatusNotFound},
		"an id that is not a uuid": {notAUUID, http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			s := newInviteScene(t)

			invite(s, c.engagementID, c.want)
		})
	}
}

func TestInviteHandler_Success(t *testing.T) {
	s := newInviteScene(t, invitedClient("New Client", "new@example.com"))

	out := inviteSucceeding(s, s.Engagement.ID, http.StatusCreated)

	if out.ClientPortalUserID == "" || out.InviteToken == "" {
		t.Fatalf("expected non-empty ids, got %+v", out)
	}
	var gotClientID string
	var identityUIDCol sql.NullString
	if err := s.DB.Admin.QueryRowContext(t.Context(),
		`SELECT client_id, identity_uid FROM client_portal_users WHERE id = $1`, out.ClientPortalUserID,
	).Scan(&gotClientID, &identityUIDCol); err != nil {
		t.Fatalf("query pending row: %v", err)
	}
	if gotClientID != s.Engagement.Client.ID {
		t.Fatalf("client_id = %q, want %q", gotClientID, s.Engagement.Client.ID)
	}
	if identityUIDCol.Valid {
		t.Fatalf("expected pending row to have no identity_uid yet, got %q", identityUIDCol.String)
	}
	outboxStatus := testdb.Scalar[string](t, s.DB.Admin, `SELECT status FROM portal_invite_outbox WHERE client_portal_user_id = $1`, out.ClientPortalUserID)
	if outboxStatus != testOutboxStatusPending {
		t.Fatalf("outbox status = %q, want %s", outboxStatus, testOutboxStatusPending)
	}
}

func TestInviteHandler_ReinviteRotatesToken(t *testing.T) {
	s := newInviteScene(t, invitedClient("Reinvited Client", "reinvited@example.com"))

	firstOut := inviteSucceeding(s, s.Engagement.ID, http.StatusCreated)
	firstExpiresAt := inviteTokenExpiresAt(t, s.DB, firstOut.ClientPortalUserID)
	// #2522: a refused accept on the first token is a fact about that
	// token; rotating it asks again.
	s.Exec(`UPDATE client_portal_users SET state = 'refused', access_refused_at = now() WHERE id = $1`, firstOut.ClientPortalUserID)

	secondOut := inviteSucceeding(s, s.Engagement.ID, http.StatusOK)

	if secondOut.ClientPortalUserID != firstOut.ClientPortalUserID {
		t.Fatalf("re-invite created a second row: %q != %q", secondOut.ClientPortalUserID, firstOut.ClientPortalUserID)
	}
	if secondOut.InviteToken == firstOut.InviteToken {
		t.Fatalf("expected invite_token to rotate, got the same token twice")
	}
	secondExpiresAt := inviteTokenExpiresAt(t, s.DB, firstOut.ClientPortalUserID)
	if !secondExpiresAt.After(firstExpiresAt) {
		t.Fatalf("expected invite_token_expires_at to move forward on re-invite, first=%v second=%v", firstExpiresAt, secondExpiresAt)
	}
	stillRefused := testdb.Scalar[bool](t, s.DB.Admin, `SELECT access_refused_at IS NOT NULL FROM client_portal_users WHERE id = $1`, firstOut.ClientPortalUserID)
	if stillRefused {
		t.Fatal("access_refused_at survived the re-invite, want it cleared with the rotated token")
	}
	count := testdb.Scalar[int](t, s.DB.Admin, `SELECT count(*) FROM client_portal_users WHERE client_id = $1`, s.Engagement.Client.ID)
	if count != 1 {
		t.Fatalf("expected exactly one row for the client after re-invite, got %d", count)
	}

	// The re-invite reuses the same pending outbox row (queueOutboxSend's
	// ON CONFLICT), rather than forking a second one racing to send the
	// same portal user's invite.
	var outboxCount, outboxAttemptCount int
	if err := s.DB.Admin.QueryRowContext(t.Context(),
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
	s := newInviteScene(t, invitedClient("Accepted Client", "accepted@example.com"))
	testdb.SeedPortalAccount(t, s.DB, "already-accepted-uid", "already-accepted-uid@example.com")
	s.Exec(`INSERT INTO client_portal_users (identity_uid, client_id, state) VALUES ('already-accepted-uid', $1, 'accepted')`, s.Engagement.Client.ID)

	out := reqtest.Expect[apierr.APIError](t, http.MethodPost, s.PracticeURL(invitePath(s.Engagement.ID)), http.StatusConflict, s.SignedIn)

	if out.Code != apierr.CodeConflict {
		t.Fatalf("code = %q, want %q", out.Code, apierr.CodeConflict)
	}
}

// TestInviteHandler_SuppressedAddress is #789's ACs: a portal invite to a
// currently-suppressed address is refused at the endpoint, names Blocked
// email addresses, and queues no outbox row; a suppression that has been
// cleared (#744) lets the same invite through unchanged; and a Client with
// no address on file (ADR-0017) has nothing to check, so the invite
// proceeds exactly as it did before -- the outbox dead-letters it later
// for its own, separate reason ("client has no email on file").
func TestInviteHandler_SuppressedAddress(t *testing.T) {
	const email = "suppressed@example.com"
	suppress := func(t *testing.T, db *testdb.DB) {
		t.Helper()
		if err := mailsuppress.Record(t.Context(), db.Admin, email, mailsuppress.CauseBounce, "evt-789"); err != nil {
			t.Fatalf("seed suppression: %v", err)
		}
	}
	cases := []struct {
		name  string
		email string
		setUp func(t *testing.T, db *testdb.DB)
		want  int
	}{
		{name: "a suppressed address is refused", email: email, setUp: suppress, want: http.StatusConflict},
		{
			name: "a cleared suppression lets the invite through", email: email, want: http.StatusCreated,
			setUp: func(t *testing.T, db *testdb.DB) {
				suppress(t, db)
				if _, err := db.Admin.ExecContext(t.Context(),
					`UPDATE email_suppressions SET cleared_at = now() WHERE address = $1`, mailsuppress.Normalize(email),
				); err != nil {
					t.Fatalf("clear suppression: %v", err)
				}
			},
		},
		{name: "no email on file skips the check", want: http.StatusCreated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newInviteScene(t, invitedClient("Suppression Client", c.email))
			if c.setUp != nil {
				c.setUp(t, s.DB)
			}

			if c.want != http.StatusConflict {
				invite(s, s.Engagement.ID, c.want)
				return
			}
			out := reqtest.Expect[apierr.APIError](t, http.MethodPost, s.PracticeURL(invitePath(s.Engagement.ID)), c.want, s.SignedIn)
			if out.Code != apierr.CodeFailedPrecondition {
				t.Fatalf("code = %q, want %q", out.Code, apierr.CodeFailedPrecondition)
			}
			if !strings.Contains(out.Message, "Blocked email addresses") {
				t.Fatalf("message = %q, want it to name Blocked email addresses", out.Message)
			}
			if out.Details["email"] == "" {
				t.Fatalf("expected a field-keyed details entry for email, got %+v", out.Details)
			}
			portalUserCount := testdb.Scalar[int](t, s.DB.Admin, `SELECT count(*) FROM client_portal_users WHERE client_id = $1`, s.Engagement.Client.ID)
			if portalUserCount != 0 {
				t.Fatalf("expected no client_portal_users row for a refused invite, got %d", portalUserCount)
			}
			outboxCount := testdb.Scalar[int](t, s.DB.Admin, `SELECT count(*) FROM portal_invite_outbox o
				 JOIN client_portal_users pu ON pu.id = o.client_portal_user_id WHERE pu.client_id = $1`, s.Engagement.Client.ID)
			if outboxCount != 0 {
				t.Fatalf("expected no portal_invite_outbox row for a refused invite, got %d", outboxCount)
			}
		})
	}
}
