package portalinvite_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// TestInviteHandler_RefusesAnErasedClient: an erased record has no
// address and no portal link left (ADR-0027). Before the gate the invite
// wrote a pending row and queued a send nobody could receive.
func TestInviteHandler_RefusesAnErasedClient(t *testing.T) {
	s := newInviteScene(t, invitedClient("Gone Client", "gone@example.com"))
	fixture.EraseClient(t, s.DB, s.Engagement.Client.ID)

	got := apierrtest.Expect(t, http.MethodPost, s.PracticeURL(invitePath(s.Engagement.ID)), http.StatusConflict, s.SignedIn)

	if _, want := clientstanding.ActInviteToPortal.Check(clientstanding.Erased); got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}
	if n := testdb.Scalar[int](t, s.DB.Admin, `SELECT count(*) FROM client_portal_users WHERE client_id = $1`, s.Engagement.Client.ID); n != 0 {
		t.Errorf("client_portal_users = %d, want 0", n)
	}
}
