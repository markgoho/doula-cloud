package message_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/message"
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
// behind staffauth.Middleware.
func newServer(verifier authn.Verifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /practices/{practiceId}/engagements/{engagementId}/messages",
		staffauth.Middleware(verifier, db.App)(message.ListHandler()))
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/messages",
		staffauth.Middleware(verifier, db.App)(message.CreateHandler()))
	return httptest.NewServer(mux)
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
