package message_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// newPortalServerWithPusher mirrors newPortalServer but lets the caller
// inject pusher directly, so a test can inspect what Message creation
// sent to it.
func newPortalServerWithPusher(t *testing.T, db *testdb.DB, who fixture.Signer, pusher push.Pusher) (srv *httptest.Server, session string) {
	t.Helper()
	store := objectstore.NewMemoryStore()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	message.Mount(g, ir, db.App, store, pusher)
	return httptest.NewServer(mux), fixture.Session(t, db, who)
}

// withPortalClient seeds the env's Engagement for a Client who has a portal
// login, so AsClient signs her in.
func withPortalClient(name string) envOption {
	return fixture.WithClient(fixture.WithPortalLogin(), fixture.ClientName(name))
}

// portalMessagesURL is the Client-portal thread of engagement, plus tail.
func (e *env) portalMessagesURL(engagementID, tail string) string {
	return e.URL("/api/portal/engagements/" + engagementID + "/messages" + tail)
}

// staffMessagesURL is the Staff-side thread of the env's Engagement.
func (e *env) staffMessagesURL() string {
	return e.PracticeURL("/engagements/" + e.Engagement.ID + "/messages")
}

// portalThread reads the env's Engagement's thread as its Client.
func (e *env) portalThread(query string) message.ListResponse {
	e.T.Helper()
	return reqtest.Expect[message.ListResponse](e.T, http.MethodGet, e.portalMessagesURL(e.Engagement.ID, "")+query, http.StatusOK, e.AsClient)
}

func TestClientCreateHandler_Success(t *testing.T) {
	e := newEnv(t, withPortalClient("Jordan Client"))

	out := reqtest.Expect[message.Message](t, http.MethodPost, e.portalMessagesURL(e.Engagement.ID, ""), http.StatusCreated, e.AsClient,
		reqtest.JSON(message.CreateRequest{Body: "Question about my next visit."}))

	if out.MessageID == "" || out.SenderType != "client" || out.SenderID != e.Engagement.Client.ID ||
		out.SenderName != "Jordan Client" || out.Body != "Question about my next visit." || out.CreatedAt.IsZero() {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestClientCreateHandler_EmptyBodyRejected(t *testing.T) {
	e := newEnv(t, withPortalClient("Client"))

	resp := reqtest.Post(t, e.portalMessagesURL(e.Engagement.ID, ""), e.AsClient, reqtest.JSON(message.CreateRequest{Body: "   "}))
	defer resp.Body.Close()

	// The JSON path is the one an empty compose form takes, since the app
	// only sends multipart with a file, so it says the same thing the
	// multipart path does, against the Message field (#1952).
	assertFieldRefusal(t, resp, http.StatusBadRequest, "body", message.MsgMessageOrFile)
}

func TestClientCreateHandler_InvalidJSONBody(t *testing.T) {
	e := newEnv(t, withPortalClient("Client"))

	reqtest.ExpectStatus(t, http.MethodPost, e.portalMessagesURL(e.Engagement.ID, ""), http.StatusBadRequest, e.AsClient, reqtest.RawJSON([]byte("not json")))
}

func TestClientListHandler_EmptyThread(t *testing.T) {
	e := newEnv(t, withPortalClient("Client"))

	out := e.portalThread("")
	if len(out.Items) != 0 || out.HasMore || out.NextCursor != nil {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestClientListHandler_InvalidCursorRejected(t *testing.T) {
	e := newEnv(t, withPortalClient("Client"))

	reqtest.ExpectStatus(t, http.MethodGet, e.portalMessagesURL(e.Engagement.ID, "?cursor=not!valid!base64!"), http.StatusBadRequest, e.AsClient)
}

// TestClientListHandler_PaginatesNewestFirst mirrors
// TestListHandler_PaginatesNewestFirst for the Client-portal population,
// proving ClientListHandler's cursor/hasMore path is reachable the same
// way the Staff-side one is.
func TestClientListHandler_PaginatesNewestFirst(t *testing.T) {
	e := newEnv(t, withPortalClient("Client"))

	const total = 35 // pageSize (30) + 5, to force a second page
	for i := range total {
		seedMessage(t, e.DB, e.Engagement.ID, "client", e.Engagement.Client.ID, "message "+string(rune('A'+i%26)))
	}

	first := e.portalThread("")
	reqtest.RequirePage(t, "first", len(first.Items), first.HasMore, first.NextCursor, 30, true)
	second := e.portalThread("?cursor=" + *first.NextCursor)
	reqtest.RequirePage(t, "second", len(second.Items), second.HasMore, second.NextCursor, 5, false)
}

// TestClientRoutes_NotLinkedToClientForbidden proves the ticket's
// client-tier isolation AC through the full mounted route: a Client not
// linked to the Engagement gets 403, not the thread, whether she posts,
// lists or downloads an attachment. This comes from clientauth.Middleware's
// ownership check (which runs before the handlers), the same protection
// portal.DetailHandler relies on -- messages' own RLS policy is exercised
// directly, bypassing any handler, by rls_test.go's
// TestRLS_MessagesClientTierScopedToOwnEngagement.
func TestClientRoutes_NotLinkedToClientForbidden(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		tail   func(messageID string) string
		opts   []reqtest.Option
	}{
		{"posting", http.MethodPost, func(string) string { return "" }, []reqtest.Option{reqtest.JSON(message.CreateRequest{Body: "trying to post anyway"})}},
		{"listing", http.MethodGet, func(string) string { return "" }, nil},
		{"downloading an attachment", http.MethodGet, func(id string) string { return "/" + id + "/attachment" }, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, withPortalClient("Owner Client"))
			messageID := seedMessageWithAttachment(t, e.DB, e.Engagement.ID, "client", e.Engagement.Client.ID, "objects/photo.png", pngContentType, "photo.png", int64(len(pngBytes)))
			unrelated := e.AnotherEngagement(fixture.NewClient(fixture.WithPortalLogin(), fixture.ClientName("Unrelated Client")))

			reqtest.ExpectStatus(t, tc.method, e.portalMessagesURL(e.Engagement.ID, tc.tail(messageID)), http.StatusForbidden,
				append([]reqtest.Option{e.As(unrelated.Client)}, tc.opts...)...)
		})
	}
}

// TestSharedThread_StaffAndClientSeeOneContinuousThread proves the
// ticket's AC that a message sent by either side appears in the other
// side's thread -- one continuous conversation, not split by sender. Has
// each side post, then lists from both sides and asserts each sees both
// messages, in the same newest-first order, with the correct
// senderType/senderName per row.
func TestSharedThread_StaffAndClientSeeOneContinuousThread(t *testing.T) {
	const bodyFromStaff = "Hi, checking in before your visit."
	const bodyFromClient = "Thanks, see you then!"

	e := newEnv(t, signedInAs(fixture.StaffName("Jamie Doula")), withPortalClient("Jordan Client"))

	reqtest.ExpectStatus(t, http.MethodPost, e.staffMessagesURL(), http.StatusCreated, e.SignedIn, reqtest.JSON(message.CreateRequest{Body: bodyFromStaff}))
	reqtest.ExpectStatus(t, http.MethodPost, e.portalMessagesURL(e.Engagement.ID, ""), http.StatusCreated, e.AsClient, reqtest.JSON(message.CreateRequest{Body: bodyFromClient}))

	assertBothMessagesInOrder := func(t *testing.T, items []message.Message) {
		t.Helper()
		if len(items) != 2 {
			t.Fatalf("thread = %+v, want both messages visible", items)
		}
		// Newest first: the Client's reply was sent after the Staff message.
		if items[0].Body != bodyFromClient || items[0].SenderType != "client" || items[0].SenderName != "Jordan Client" {
			t.Fatalf("newest item = %+v, want client's reply", items[0])
		}
		if items[1].Body != bodyFromStaff || items[1].SenderType != "staff" || items[1].SenderName != "Jamie Doula" {
			t.Fatalf("oldest item = %+v, want staff's message", items[1])
		}
	}

	assertBothMessagesInOrder(t, reqtest.Expect[message.ListResponse](t, http.MethodGet, e.staffMessagesURL(), http.StatusOK, e.SignedIn).Items)
	assertBothMessagesInOrder(t, e.portalThread("").Items)
}

// TestClientCreateHandler_AttachmentUploadAndDownloadRoundTrip mirrors
// TestCreateHandler_AttachmentUploadAndDownloadRoundTrip for the
// Client-portal side.
func TestClientCreateHandler_AttachmentUploadAndDownloadRoundTrip(t *testing.T) {
	e := newEnv(t, withPortalClient("Client"))

	resp := multipartPost(t, e.portalMessagesURL(e.Engagement.ID, ""), "Here's a photo", "photo.png", pngBytes, e.AsClient) //nolint:bodyclose // DecodeStatus closes the body
	created := reqtest.DecodeStatus[message.Message](t, resp, http.StatusCreated)                                           //nolint:bodyclose // DecodeStatus closes the body
	if created.AttachmentContentType != pngContentType {
		t.Fatalf("attachmentContentType = %q, want image/png", created.AttachmentContentType)
	}

	body := reqtest.ExpectBody(t, http.MethodGet, e.portalMessagesURL(e.Engagement.ID, "/"+created.MessageID+"/attachment"), http.StatusOK, e.AsClient)
	if !bytes.Equal(body, pngBytes) {
		t.Fatalf("downloaded bytes don't match uploaded bytes")
	}
}

// TestClientAttachmentHandler_InvalidMessageID mirrors
// TestAttachmentHandler_InvalidMessageID for the Client-portal side.
func TestClientAttachmentHandler_InvalidMessageID(t *testing.T) {
	e := newEnv(t, withPortalClient("Client"))

	reqtest.ExpectStatus(t, http.MethodGet, e.portalMessagesURL(e.Engagement.ID, "/not-a-uuid/attachment"), http.StatusBadRequest, e.AsClient)
}

// TestClientListHandler_NamesASenderWhoHasLeftThePractice is the Message
// thread's half of #1077. The portal thread resolves a Staff sender's
// name through the same staff row the Visits screen does, so a Doula who
// left used to take her name out of the conversation she was part of --
// a thread of unattributed messages. 00111's reach through the Message
// itself keeps it.
func TestClientListHandler_NamesASenderWhoHasLeftThePractice(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.StaffName("Maya Okonkwo")), withPortalClient("Nadia Client"))
	reqtest.ExpectStatus(t, http.MethodPost, e.staffMessagesURL(), http.StatusCreated, e.SignedIn, reqtest.JSON(message.CreateRequest{Body: "I'll see you on Thursday."}))

	testdb.RemoveMembership(t, e.DB, e.Staff.ID)

	thread := e.portalThread("")
	if len(thread.Items) != 1 {
		t.Fatalf("thread = %+v, want the one Message she was sent", thread.Items)
	}
	if thread.Items[0].SenderName != "Maya Okonkwo" {
		t.Fatalf("senderName = %q, want the name of the Doula who wrote it", thread.Items[0].SenderName)
	}
}

// TestClientListHandler_StandsInForASenderWhoDeletedHerLogin is #1198.
// 00111's client_portal_sees_staff policy deliberately refuses a Staff
// row once ADR-0033 has redacted it (deleted_at IS NOT NULL), so the
// portal thread's LEFT JOIN finds no row for that sender at all -- unlike
// TestClientListHandler_NamesASenderWhoHasLeftThePractice above, where
// the row is still reachable through the Message itself. The Visits
// screen already stands in with "Your practice" for this same gap
// (TestVisitsHandler_StandsInForADoulaWhoDeletedHerLogin); the Message
// thread must not disagree by showing an empty name instead.
func TestClientListHandler_StandsInForASenderWhoDeletedHerLogin(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.StaffName("Maya Okonkwo")), withPortalClient("Nadia Client"))
	reqtest.ExpectStatus(t, http.MethodPost, e.staffMessagesURL(), http.StatusCreated, e.SignedIn, reqtest.JSON(message.CreateRequest{Body: "See you Thursday."}))

	testdb.RemoveMembership(t, e.DB, e.Staff.ID)
	e.Exec(`UPDATE staff SET first_name = $1, last_name = $2, deleted_at = now() WHERE id = $3`,
		staffauth.DeletedStaffFirstName, staffauth.DeletedStaffLastName, e.Staff.ID)

	thread := e.portalThread("")
	if len(thread.Items) != 1 {
		t.Fatalf("thread = %+v, want the one Message she was sent", thread.Items)
	}
	if thread.Items[0].SenderName != "Your practice" {
		t.Fatalf("senderName = %q, want the Practice standing in for a name that no longer exists", thread.Items[0].SenderName)
	}
}
