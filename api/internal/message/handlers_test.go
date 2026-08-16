package message_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// fakeVerifier is a test double for authn.Verifier -- see staffauth's own
// middleware_test.go for why: real Identity Platform tokens can't be
// minted without a live GCP project.
type fakeVerifier struct {
	uid string
}

func (f fakeVerifier) VerifyIDToken(_ context.Context, _ string) (*authn.VerifiedToken, error) {
	return &authn.VerifiedToken{UID: f.uid}, nil
}

// newServer mounts the same routes main.go wires up for this package,
// behind staffauth.Middleware, backed by a fresh in-memory ObjectStore and
// a fresh in-memory Pusher -- no real GCS bucket or VAPID/push service
// reachable from api/ tests, per docs/testing.md.
func newServer(verifier authn.Verifier, db *testdb.DB) *httptest.Server {
	return newServerWithStoreAndPusher(verifier, db, objectstore.NewMemoryStore(), push.NewFakePusher())
}

// newServerWithStore mirrors newServer but lets the caller inject store
// directly, so a test can exercise the handler's error-handling around a
// store failure (e.g. a GCS outage) -- a path MemoryStore's happy path
// never reaches.
func newServerWithStore(verifier authn.Verifier, db *testdb.DB, store objectstore.ObjectStore) *httptest.Server {
	return newServerWithStoreAndPusher(verifier, db, store, push.NewFakePusher())
}

// newServerWithPusher mirrors newServer but lets the caller inject pusher
// directly, so a test can inspect what Message creation sent to it.
func newServerWithPusher(verifier authn.Verifier, db *testdb.DB, pusher push.Pusher) *httptest.Server {
	return newServerWithStoreAndPusher(verifier, db, objectstore.NewMemoryStore(), pusher)
}

func newServerWithStoreAndPusher(verifier authn.Verifier, db *testdb.DB, store objectstore.ObjectStore, pusher push.Pusher) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /practices/{practiceId}/engagements/{engagementId}/messages",
		staffauth.Middleware(verifier, db.App)(message.ListHandler()))
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/messages",
		staffauth.Middleware(verifier, db.App)(message.CreateHandler(store, pusher)))
	mux.Handle("GET /practices/{practiceId}/engagements/{engagementId}/messages/{messageId}/attachment",
		staffauth.Middleware(verifier, db.App)(message.AttachmentHandler(store)))
	return httptest.NewServer(mux)
}

// failingStore is an objectstore.ObjectStore whose Put and Get always
// fail, standing in for a real GCS outage.
type failingStore struct{}

func (failingStore) Put(_ context.Context, _, _ string, _ io.Reader) error {
	return errors.New("simulated store failure")
}

func (failingStore) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, errors.New("simulated store failure")
}

func authedGet(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func authedPost(t *testing.T, url string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(body))
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

// authedMultipartPost posts a multipart/form-data create-Message request:
// a "body" text field plus, when filename is non-empty, an "attachment"
// file field holding content.
func authedMultipartPost(t *testing.T, url, body, filename string, content []byte) *http.Response {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("body", body); err != nil {
		t.Fatalf("write body field: %v", err)
	}
	if filename != "" {
		part, err := writer.CreateFormFile("attachment", filename)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("write attachment content: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, &buf)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// pngContentType is what http.DetectContentType reports for pngBytes.
const pngContentType = "image/png"

// pngBytes is a minimal valid 1x1 PNG, enough for http.DetectContentType
// to recognize it as image/png.
var pngBytes = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func TestCreateHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-creating"
	practiceID := seedPractice(t, db, "Practice")
	staffID := seedStaffAtPracticeNamed(t, db, practiceID, identityUID, "Jamie Doula")
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, err := json.Marshal(message.CreateRequest{Body: "Hi, checking in on your appointment."})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out message.Message
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.MessageID == "" || out.SenderType != "staff" || out.SenderID != staffID ||
		out.SenderName != "Jamie Doula" || out.Body != "Hi, checking in on your appointment." || out.CreatedAt.IsZero() {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestCreateHandler_EmptyBodyRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-empty-body"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, _ := json.Marshal(message.CreateRequest{Body: "   "})
	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestCreateHandler_InvalidJSONBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-bad-json"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages", []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestCreateHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-bad-engagement-id"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, _ := json.Marshal(message.CreateRequest{Body: "hello"})
	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/engagements/not-a-uuid/messages", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestCreateHandler_NotFoundForEngagementAtDifferentPractice proves the
// ticket's practice-tier isolation AC from the HTTP layer: a Staff member
// at a different Practice can't post into an Engagement's thread, even
// knowing its id. This 404 comes from requireEngagementAtPractice's
// explicit practice_id check, not from messages' own RLS policy -- that
// policy is exercised directly (bypassing any handler) by rls_test.go's
// TestRLS_MessagesPracticeTierScopedToOwnPractice, added in #57.
func TestCreateHandler_NotFoundForEngagementAtDifferentPractice(t *testing.T) {
	db := testdb.New(t)
	homePracticeID := seedPractice(t, db, "Home Practice")
	seedStaffAtPractice(t, db, homePracticeID, "staff-elsewhere")

	otherPracticeID := seedPractice(t, db, "Other Practice")
	_, engagementID := seedClientEngagement(t, db, otherPracticeID, "Other Client", "other@example.com")

	srv := newServer(fakeVerifier{uid: "staff-elsewhere"}, db)
	defer srv.Close()

	body, _ := json.Marshal(message.CreateRequest{Body: "hello"})
	resp := authedPost(t, srv.URL+"/practices/"+homePracticeID+"/engagements/"+engagementID+"/messages", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestListHandler_EmptyThread(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-empty-thread"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages")
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

func TestListHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-list-bad-id"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/not-a-uuid/messages")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestListHandler_NotFoundForEngagementAtDifferentPractice mirrors
// TestCreateHandler_NotFoundForEngagementAtDifferentPractice for reads: a
// Staff member at a different Practice gets zero rows / 404, per the
// ticket's practice-tier isolation AC -- see that test's comment for why
// this is the app-layer check, not messages' own RLS policy.
func TestListHandler_NotFoundForEngagementAtDifferentPractice(t *testing.T) {
	db := testdb.New(t)
	homePracticeID := seedPractice(t, db, "Home Practice")
	seedStaffAtPractice(t, db, homePracticeID, "staff-listing-elsewhere")

	otherPracticeID := seedPractice(t, db, "Other Practice")
	otherStaffID := seedStaffAtPractice(t, db, otherPracticeID, "staff-owns-thread")
	_, engagementID := seedClientEngagement(t, db, otherPracticeID, "Other Client", "other@example.com")
	seedMessage(t, db, engagementID, "staff", otherStaffID, "not visible across practices")

	srv := newServer(fakeVerifier{uid: "staff-listing-elsewhere"}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+homePracticeID+"/engagements/"+engagementID+"/messages")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestListHandler_InvalidCursorRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-bad-cursor"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	// Three distinct ways a cursor can be malformed: not valid base64 at
	// all, valid base64 but missing the "created_at|id" separator, and
	// valid base64 with the separator but an unparseable timestamp.
	cases := []string{
		"not!valid!base64!",
		"not-a-cursor",
		"YmFkdGltZXxzb21lLWlk", // base64("badtime|some-id")
	}
	for _, cursor := range cases {
		resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages?cursor="+cursor)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("cursor %q: status = %d, want %d", cursor, resp.StatusCode, http.StatusBadRequest)
		}
	}
}

// TestListHandler_AnyStaffAtSamePracticeSeesAndCanReplyToSameThread proves
// the ticket's "no per-Staff-member sub-threads" AC: Staff A posts, Staff
// B at the same Practice -- who neither created the Engagement nor sent
// the first message -- lists the thread, sees A's message under A's own
// name, and can reply into the same thread.
func TestListHandler_AnyStaffAtSamePracticeSeesAndCanReplyToSameThread(t *testing.T) {
	const messageFromA = "message from A"

	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Shared Practice")
	seedStaffAtPracticeNamed(t, db, practiceID, "staff-a", "Staff A")
	seedStaffAtPracticeNamed(t, db, practiceID, "staff-b", "Staff B")
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srvA := newServer(fakeVerifier{uid: "staff-a"}, db)
	defer srvA.Close()
	bodyA, _ := json.Marshal(message.CreateRequest{Body: messageFromA})
	respA := authedPost(t, srvA.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages", bodyA)
	defer respA.Body.Close()
	if respA.StatusCode != http.StatusCreated {
		t.Fatalf("Staff A create status = %d, want %d", respA.StatusCode, http.StatusCreated)
	}

	srvB := newServer(fakeVerifier{uid: "staff-b"}, db)
	defer srvB.Close()

	listResp := authedGet(t, srvB.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages")
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("Staff B list status = %d, want %d", listResp.StatusCode, http.StatusOK)
	}
	var listed message.ListResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Body != messageFromA || listed.Items[0].SenderName != "Staff A" {
		t.Fatalf("Staff B's view of the thread = %+v, want A's message with A's name", listed.Items)
	}

	bodyB, _ := json.Marshal(message.CreateRequest{Body: "reply from B"})
	replyResp := authedPost(t, srvB.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages", bodyB)
	defer replyResp.Body.Close()
	if replyResp.StatusCode != http.StatusCreated {
		t.Fatalf("Staff B reply status = %d, want %d", replyResp.StatusCode, http.StatusCreated)
	}

	finalResp := authedGet(t, srvA.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages")
	defer finalResp.Body.Close()
	var final message.ListResponse
	if err := json.NewDecoder(finalResp.Body).Decode(&final); err != nil {
		t.Fatalf("decode final list response: %v", err)
	}
	if len(final.Items) != 2 {
		t.Fatalf("final thread = %+v, want both messages visible to Staff A", final.Items)
	}
	// Newest first: B's reply was sent after A's message.
	if final.Items[0].Body != "reply from B" || final.Items[1].Body != messageFromA {
		t.Fatalf("final thread order = %+v, want newest first", final.Items)
	}
}

// TestListHandler_PaginatesNewestFirst seeds more than one page of
// Messages and walks the cursor to prove: newest-first ordering, a
// truncated first page with hasMore/nextCursor set, and a second page
// that reaches the remainder with hasMore false.
func TestListHandler_PaginatesNewestFirst(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-paging"
	practiceID := seedPractice(t, db, "Practice")
	staffID := seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	const total = 35 // pageSize (30) + 5, to force a second page
	for i := range total {
		seedMessage(t, db, engagementID, "staff", staffID, "message "+string(rune('A'+i%26)))
	}

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	firstResp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages")
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

	secondResp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages?cursor="+*first.NextCursor)
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

	seen := map[string]bool{}
	for _, it := range append(first.Items, second.Items...) {
		if seen[it.MessageID] {
			t.Fatalf("message %q appeared on both pages", it.MessageID)
		}
		seen[it.MessageID] = true
	}
	if len(seen) != total {
		t.Fatalf("saw %d distinct messages across both pages, want %d", len(seen), total)
	}
}

// TestCreateHandler_AttachmentUploadAndDownloadRoundTrip proves #60's core
// AC end to end: a multipart create with a valid image attachment succeeds,
// the response DTO carries the attachment metadata, and the attachment
// download endpoint serves back the exact bytes with the sniffed (not
// client-declared) content type.
func TestCreateHandler_AttachmentUploadAndDownloadRoundTrip(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-attachment"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedMultipartPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages",
		"Here's the photo", "photo.png", pngBytes)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created message.Message
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.AttachmentContentType != pngContentType || created.AttachmentFilename != "photo.png" {
		t.Fatalf("attachment metadata = %+v, want image/png photo.png", created)
	}

	dlResp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages/"+created.MessageID+"/attachment")
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d, want %d", dlResp.StatusCode, http.StatusOK)
	}
	if got := dlResp.Header.Get("Content-Type"); got != pngContentType {
		t.Fatalf("Content-Type = %q, want image/png", got)
	}
	if got := dlResp.Header.Get("Content-Disposition"); got != `attachment; filename="photo.png"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	body, err := io.ReadAll(dlResp.Body)
	if err != nil {
		t.Fatalf("read download body: %v", err)
	}
	if !bytes.Equal(body, pngBytes) {
		t.Fatalf("downloaded bytes don't match uploaded bytes")
	}
}

// TestCreateHandler_MultipartTextOnlyNoAttachmentField proves a
// multipart request with a body and no "attachment" field at all
// succeeds as a text-only Message, the same as the JSON path.
func TestCreateHandler_MultipartTextOnlyNoAttachmentField(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-multipart-text-only"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedMultipartPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages",
		"text only, sent as multipart", "", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created message.Message
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.Body != "text only, sent as multipart" || created.AttachmentFilename != "" {
		t.Fatalf("unexpected response: %+v", created)
	}
}

// TestCreateHandler_AttachmentOnlyNoBody proves an attachment alone
// satisfies messages_has_content -- body is optional once an attachment
// is present.
func TestCreateHandler_AttachmentOnlyNoBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-attachment-only"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedMultipartPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages",
		"", "photo.png", pngBytes)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created message.Message
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.Body != "" || created.AttachmentFilename != "photo.png" {
		t.Fatalf("unexpected response: %+v", created)
	}
}

// TestCreateHandler_AttachmentPDFAccepted proves the second allowed
// content type -- image/* isn't the only kind #56's resolution allows.
func TestCreateHandler_AttachmentPDFAccepted(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-pdf"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	pdfBytes := []byte("%PDF-1.4\nfake pdf content for content-type sniffing\n")
	resp := authedMultipartPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages",
		"Here's the contract", "form.pdf", pdfBytes)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created message.Message
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.AttachmentContentType != "application/pdf" {
		t.Fatalf("attachmentContentType = %q, want application/pdf", created.AttachmentContentType)
	}
}

// TestCreateHandler_AttachmentWrongTypeRejected proves the content-type
// cap: a plain-text attachment is neither image/* nor application/pdf.
func TestCreateHandler_AttachmentWrongTypeRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-wrong-type"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedMultipartPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages",
		"trying to attach a text file", "notes.txt", []byte("just plain text, not an image or PDF"))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestCreateHandler_AttachmentOversizedRejected proves the ~10MB cap is
// enforced against the actual (post-parse) attachment size, before Put is
// ever called.
func TestCreateHandler_AttachmentOversizedRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-oversized"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	oversized := bytes.Repeat([]byte("a"), 10<<20+32<<10) // 10MB + 32KB: over the cap, under the request-body ceiling
	resp := authedMultipartPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages",
		"too big", "big.png", oversized)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}
}

// TestCreateHandler_RequestWayOversizedRejectedAtParse proves the
// http.MaxBytesReader wrap: a request so large it can't even be parsed is
// rejected while parsing, before FileHeader.Size is ever consulted --
// with the same 413 status as the post-parse size check, since both are
// the same "attachment too large" failure from the caller's perspective.
func TestCreateHandler_RequestWayOversizedRejectedAtParse(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-way-oversized"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	wayOversized := bytes.Repeat([]byte("a"), 11<<20) // past maxCreateRequestBytes entirely
	resp := authedMultipartPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages",
		"way too big", "big.png", wayOversized)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}
}

// TestCreateHandler_MalformedMultipartRejected proves a genuinely
// malformed multipart body (not merely oversized) still gets a distinct
// 400, not the 413 reserved for the size failure.
func TestCreateHandler_MalformedMultipartRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-malformed-multipart"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages", bytes.NewReader([]byte("not actually multipart")))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "multipart/form-data; boundary=doesnotmatch")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestCreateHandler_EmptyBodyAndNoAttachmentRejected proves
// messages_has_content's "one or the other" rule is enforced by the
// handler too, not just the DB constraint.
func TestCreateHandler_EmptyBodyAndNoAttachmentRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-empty-multipart"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedMultipartPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages", "", "", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestCreateHandler_StorePutFailureReturns500 exercises the handler's
// error handling around a failed ObjectStore.Put (e.g. a GCS outage) --
// injecting failingStore rather than relying on a coverage:ignore, since
// #60 built ObjectStore as a seam specifically so this path is testable.
func TestCreateHandler_StorePutFailureReturns500(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-store-put-fails"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServerWithStore(fakeVerifier{uid: identityUID}, db, failingStore{})
	defer srv.Close()

	resp := authedMultipartPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages",
		"", "photo.png", pngBytes)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

// TestAttachmentHandler_StoreGetFailureReturns500 mirrors
// TestCreateHandler_StorePutFailureReturns500 for the download side: a
// Message with attachment metadata already in the DB (seeded directly,
// bypassing Put) whose ObjectStore.Get fails.
func TestAttachmentHandler_StoreGetFailureReturns500(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-store-get-fails"
	practiceID := seedPractice(t, db, "Practice")
	staffID := seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")
	messageID := seedMessageWithAttachment(t, db, engagementID, "staff", staffID,
		"messages/whatever/path", pngContentType, "photo.png", int64(len(pngBytes)))

	srv := newServerWithStore(fakeVerifier{uid: identityUID}, db, failingStore{})
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages/"+messageID+"/attachment")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

// TestAttachmentHandler_NoAttachmentNotFound proves a text-only Message
// has no attachment to download.
func TestAttachmentHandler_NoAttachmentNotFound(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-no-attachment"
	practiceID := seedPractice(t, db, "Practice")
	staffID := seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")
	seedMessage(t, db, engagementID, "staff", staffID, "text only, no attachment")

	var messageID string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT id FROM messages WHERE engagement_id = $1`, engagementID).Scan(&messageID); err != nil {
		t.Fatalf("query message id: %v", err)
	}

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages/"+messageID+"/attachment")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestAttachmentHandler_InvalidMessageID proves the messageId path param
// is validated the same way engagementId is.
func TestAttachmentHandler_InvalidMessageID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-bad-message-id"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages/not-a-uuid/attachment")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestAttachmentHandler_InvalidEngagementID mirrors
// TestCreateHandler_InvalidEngagementID for the attachment download route.
func TestAttachmentHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-bad-engagement-attachment"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/not-a-uuid/messages/00000000-0000-0000-0000-000000000000/attachment")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestAttachmentHandler_EngagementNotFoundAtDifferentPractice mirrors
// TestCreateHandler_NotFoundForEngagementAtDifferentPractice for the
// attachment download route: a Staff member at a different Practice can't
// reach the Engagement at all, let alone its attachment.
func TestAttachmentHandler_EngagementNotFoundAtDifferentPractice(t *testing.T) {
	db := testdb.New(t)
	homePracticeID := seedPractice(t, db, "Home Practice")
	seedStaffAtPractice(t, db, homePracticeID, "staff-attachment-elsewhere")

	otherPracticeID := seedPractice(t, db, "Other Practice")
	_, engagementID := seedClientEngagement(t, db, otherPracticeID, "Other Client", "other@example.com")

	srv := newServer(fakeVerifier{uid: "staff-attachment-elsewhere"}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+homePracticeID+"/engagements/"+engagementID+"/messages/00000000-0000-0000-0000-000000000000/attachment")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestAttachmentHandler_WrongEngagementNotFound proves the messageId path
// param is checked against engagement_id explicitly (app-layer, on top of
// RLS): a Message's attachment can't be fetched through a different
// Engagement's URL, even one at the same Practice.
func TestAttachmentHandler_WrongEngagementNotFound(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-cross-engagement"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client One", "one@example.com")
	_, otherEngagementID := seedClientEngagement(t, db, practiceID, "Client Two", "two@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	createResp := authedMultipartPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages",
		"", "photo.png", pngBytes)
	defer createResp.Body.Close()
	var created message.Message
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+otherEngagementID+"/messages/"+created.MessageID+"/attachment")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestCreateHandler_JSONRequestStillTextOnly proves the original
// application/json contract is untouched by #60: it never carries an
// attachment, matching pre-#60 behavior exactly.
func TestCreateHandler_JSONRequestStillTextOnly(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-json-unchanged"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, _ := json.Marshal(message.CreateRequest{Body: "plain JSON, no attachment"})
	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created message.Message
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.AttachmentFilename != "" || created.AttachmentContentType != "" {
		t.Fatalf("unexpected attachment on JSON-only create: %+v", created)
	}
}
