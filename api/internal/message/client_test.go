package message_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/testdb"
)

// newPortalServer mounts the same routes main.go wires up for the
// Client-portal side of this package, behind clientauth.Middleware,
// backed by a fresh in-memory ObjectStore and a fresh in-memory Pusher --
// no real GCS bucket or VAPID/push service reachable from api/ tests, per
// docs/testing.md.
func newPortalServer(t *testing.T, db *testdb.DB, uid string) (*httptest.Server, string) {
	t.Helper()
	return newPortalServerWithPusher(t, db, uid, push.NewFakePusher())
}

// newPortalServerWithPusher mirrors newPortalServer but lets the caller
// inject pusher directly, so a test can inspect what Message creation
// sent to it.
func newPortalServerWithPusher(t *testing.T, db *testdb.DB, uid string, pusher push.Pusher) (*httptest.Server, string) {
	t.Helper()
	store := objectstore.NewMemoryStore()
	mux := http.NewServeMux()
	mux.Handle("GET /portal/engagements/{engagementId}/messages",
		clientauth.Middleware(db.App)(message.ClientListHandler()))
	mux.Handle("POST /portal/engagements/{engagementId}/messages",
		clientauth.Middleware(db.App)(message.ClientCreateHandler(store, pusher)))
	mux.Handle("GET /portal/engagements/{engagementId}/messages/{messageId}/attachment",
		clientauth.Middleware(db.App)(message.ClientAttachmentHandler(store)))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func TestClientCreateHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-creating"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(message.CreateRequest{Body: "Question about my next visit."})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPost(t, session, srv.URL+"/portal/engagements/"+engagementID+"/messages", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out message.Message
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.MessageID == "" || out.SenderType != "client" || out.SenderID != clientID ||
		out.SenderName != "Jordan Client" || out.Body != "Question about my next visit." || out.CreatedAt.IsZero() {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestClientCreateHandler_EmptyBodyRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-empty-body"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	body, _ := json.Marshal(message.CreateRequest{Body: "   "})
	resp := authedPost(t, session, srv.URL+"/portal/engagements/"+engagementID+"/messages", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestClientCreateHandler_InvalidJSONBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-bad-json"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/portal/engagements/"+engagementID+"/messages", []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestClientCreateHandler_NotLinkedToClientForbidden proves the ticket's
// client-tier isolation AC through the full mounted route: a Client not
// linked to the Engagement gets 403, not the thread. This comes from
// clientauth.Middleware's ownership check (which runs before
// ClientCreateHandler), the same protection portal.DetailHandler relies
// on -- messages' own RLS policy is exercised directly, bypassing any
// handler, by rls_test.go's TestRLS_MessagesClientTierScopedToOwnEngagement.
func TestClientCreateHandler_NotLinkedToClientForbidden(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Practice")
	_, engagementID := seedClientEngagement(t, db, practiceID, "Owner Client", "owner@example.com")

	unrelatedClientID, _ := seedClientEngagement(t, db, practiceID, "Unrelated Client", "unrelated@example.com")
	seedPortalUser(t, db, "client-elsewhere", unrelatedClientID)

	srv, session := newPortalServer(t, db, "client-elsewhere")
	defer srv.Close()

	body, _ := json.Marshal(message.CreateRequest{Body: "trying to post anyway"})
	resp := authedPost(t, session, srv.URL+"/portal/engagements/"+engagementID+"/messages", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestClientListHandler_EmptyThread(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-empty-thread"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/portal/engagements/"+engagementID+"/messages")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out message.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Items) != 0 || out.HasMore || out.NextCursor != nil {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestClientListHandler_InvalidCursorRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-bad-cursor"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/portal/engagements/"+engagementID+"/messages?cursor=not!valid!base64!")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestClientListHandler_PaginatesNewestFirst mirrors
// TestListHandler_PaginatesNewestFirst for the Client-portal population,
// proving ClientListHandler's cursor/hasMore path is reachable the same
// way the Staff-side one is.
func TestClientListHandler_PaginatesNewestFirst(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-paging"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	const total = 35 // pageSize (30) + 5, to force a second page
	for i := range total {
		seedMessage(t, db, engagementID, "client", clientID, "message "+string(rune('A'+i%26)))
	}

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	firstResp := authedGet(t, session, srv.URL+"/portal/engagements/"+engagementID+"/messages")
	defer firstResp.Body.Close()
	if firstResp.StatusCode != http.StatusOK {
		t.Fatalf("first page status = %d, want %d", firstResp.StatusCode, http.StatusOK)
	}
	var first message.ListResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	secondResp := authedGet(t, session, srv.URL+"/portal/engagements/"+engagementID+"/messages?cursor="+*first.NextCursor)
	defer secondResp.Body.Close()
	if secondResp.StatusCode != http.StatusOK {
		t.Fatalf("second page status = %d, want %d", secondResp.StatusCode, http.StatusOK)
	}
	var second message.ListResponse
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if len(second.Items) != 5 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 5/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
}

// TestClientListHandler_NotLinkedToClientForbidden mirrors
// TestClientCreateHandler_NotLinkedToClientForbidden for reads.
func TestClientListHandler_NotLinkedToClientForbidden(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Practice")
	ownerClientID, engagementID := seedClientEngagement(t, db, practiceID, "Owner Client", "owner@example.com")
	seedMessage(t, db, engagementID, "client", ownerClientID, "not visible to other clients")

	unrelatedClientID, _ := seedClientEngagement(t, db, practiceID, "Unrelated Client", "unrelated@example.com")
	seedPortalUser(t, db, "client-listing-elsewhere", unrelatedClientID)

	srv, session := newPortalServer(t, db, "client-listing-elsewhere")
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/portal/engagements/"+engagementID+"/messages")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestSharedThread_StaffAndClientSeeOneContinuousThread proves the
// ticket's AC that a message sent by either side appears in the other
// side's thread -- one continuous conversation, not split by sender. Runs
// both a Staff-side server (staffauth) and a Client-portal server
// (clientauth) against the same testdb.DB, has each side post, then lists
// from both sides and asserts each sees both messages, in the same
// newest-first order, with the correct senderType/senderName per row.
func TestSharedThread_StaffAndClientSeeOneContinuousThread(t *testing.T) {
	const identityUIDStaff = "shared-thread-staff"
	const identityUIDClient = "shared-thread-client"
	const bodyFromStaff = "Hi, checking in before your visit."
	const bodyFromClient = "Thanks, see you then!"

	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Shared Practice")
	seedStaffAtPracticeNamed(t, db, practiceID, identityUIDStaff, "Jamie Doula")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUIDClient, clientID)

	staffSrv, staffSession := newServer(t, db, identityUIDStaff)
	defer staffSrv.Close()
	portalSrv, portalSession := newPortalServer(t, db, identityUIDClient)
	defer portalSrv.Close()

	staffBody, _ := json.Marshal(message.CreateRequest{Body: bodyFromStaff})
	staffResp := authedPost(t, staffSession, staffSrv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages", staffBody)
	defer staffResp.Body.Close()
	if staffResp.StatusCode != http.StatusCreated {
		t.Fatalf("staff create status = %d, want %d", staffResp.StatusCode, http.StatusCreated)
	}

	clientBody, _ := json.Marshal(message.CreateRequest{Body: bodyFromClient})
	clientResp := authedPost(t, portalSession, portalSrv.URL+"/portal/engagements/"+engagementID+"/messages", clientBody)
	defer clientResp.Body.Close()
	if clientResp.StatusCode != http.StatusCreated {
		t.Fatalf("client create status = %d, want %d", clientResp.StatusCode, http.StatusCreated)
	}

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

	staffListResp := authedGet(t, staffSession, staffSrv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages")
	defer staffListResp.Body.Close()
	var staffView message.ListResponse
	if err := json.NewDecoder(staffListResp.Body).Decode(&staffView); err != nil {
		t.Fatalf("decode staff-side list: %v", err)
	}
	assertBothMessagesInOrder(t, staffView.Items)

	clientListResp := authedGet(t, portalSession, portalSrv.URL+"/portal/engagements/"+engagementID+"/messages")
	defer clientListResp.Body.Close()
	var clientView message.ListResponse
	if err := json.NewDecoder(clientListResp.Body).Decode(&clientView); err != nil {
		t.Fatalf("decode client-side list: %v", err)
	}
	assertBothMessagesInOrder(t, clientView.Items)
}

// TestClientCreateHandler_AttachmentUploadAndDownloadRoundTrip mirrors
// TestCreateHandler_AttachmentUploadAndDownloadRoundTrip for the
// Client-portal side.
func TestClientCreateHandler_AttachmentUploadAndDownloadRoundTrip(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-attachment"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := authedMultipartPost(t, session, srv.URL+"/portal/engagements/"+engagementID+"/messages",
		"Here's a photo", "photo.png", pngBytes)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created message.Message
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.AttachmentContentType != pngContentType {
		t.Fatalf("attachmentContentType = %q, want image/png", created.AttachmentContentType)
	}

	dlResp := authedGet(t, session, srv.URL+"/portal/engagements/"+engagementID+"/messages/"+created.MessageID+"/attachment")
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d, want %d", dlResp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(dlResp.Body)
	if err != nil {
		t.Fatalf("read download body: %v", err)
	}
	if !bytes.Equal(body, pngBytes) {
		t.Fatalf("downloaded bytes don't match uploaded bytes")
	}
}

// TestClientAttachmentHandler_InvalidMessageID mirrors
// TestAttachmentHandler_InvalidMessageID for the Client-portal side.
func TestClientAttachmentHandler_InvalidMessageID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-bad-message-id"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/portal/engagements/"+engagementID+"/messages/not-a-uuid/attachment")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestClientAttachmentHandler_NotLinkedToClientForbidden mirrors
// TestClientCreateHandler_NotLinkedToClientForbidden for the attachment
// download route.
func TestClientAttachmentHandler_NotLinkedToClientForbidden(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Practice")
	ownerClientID, engagementID := seedClientEngagement(t, db, practiceID, "Owner Client", "owner@example.com")
	seedPortalUser(t, db, "client-owner-of-thread", ownerClientID)

	ownerSrv, ownerSession := newPortalServer(t, db, "client-owner-of-thread")
	defer ownerSrv.Close()
	createResp := authedMultipartPost(t, ownerSession, ownerSrv.URL+"/portal/engagements/"+engagementID+"/messages",
		"", "photo.png", pngBytes)
	defer createResp.Body.Close()
	var created message.Message
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	unrelatedClientID, _ := seedClientEngagement(t, db, practiceID, "Unrelated Client", "unrelated@example.com")
	seedPortalUser(t, db, "client-elsewhere-dl", unrelatedClientID)

	unrelatedSrv, unrelatedSession := newPortalServer(t, db, "client-elsewhere-dl")
	defer unrelatedSrv.Close()

	resp := authedGet(t, unrelatedSession, unrelatedSrv.URL+"/portal/engagements/"+engagementID+"/messages/"+created.MessageID+"/attachment")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
