package message_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestPostMessage_RefusesAMessageForAnErasedClient: neither side of the
// thread may add to it once the record is erased. A real erasure also
// removes the Client's portal link, so her side is the declared backstop;
// the fixture leaves the link in place to prove the backstop holds.
func TestPostMessage_RefusesAMessageForAnErasedClient(t *testing.T) {
	e := newEnv(t, withPortalClient("Jordan Client"))
	fixture.EraseClient(t, e.DB, e.Engagement.Client.ID)
	_, want := clientstanding.ActSendMessage.Check(clientstanding.Erased)

	for side, call := range map[string]func() string{
		"from the Staff side": func() string {
			return apierrtest.Expect(t, http.MethodPost, e.staffMessagesURL(), http.StatusConflict, e.SignedIn,
				reqtest.JSON(message.CreateRequest{Body: "Checking in."})).Message
		},
		"from the Client side": func() string {
			return apierrtest.Expect(t, http.MethodPost, e.portalMessagesURL(e.Engagement.ID, ""), http.StatusConflict, e.AsClient,
				reqtest.JSON(message.CreateRequest{Body: "Thanks."})).Message
		},
	} {
		if got := call(); got != want {
			t.Errorf("%s: message = %q, want %q", side, got, want)
		}
	}
	if n := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM messages WHERE engagement_id = $1`, e.Engagement.ID); n != 0 {
		t.Errorf("messages = %d, want 0", n)
	}
}
