package idempotency_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// TestEndToEnd_PortalInviteReplaysOnRetry wires portalinvite.InviteHandler
// behind staffauth.Middleware(...)(idempotency.Wrap(...)) exactly as
// main.go does, proving the helper is genuinely usable against a real
// mutating endpoint (#126's AC), not just the synthetic handler in
// wrap_test.go.
func TestEndToEnd_PortalInviteReplaysOnRetry(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "e2e-portal-invite-staff"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "E2E Client", "e2e@example.com")

	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/portal-invite",
		staffauth.Middleware(db.App)(idempotency.Wrap(portalinvite.InviteHandler(&tasknudge.FakeEnqueuer{}))))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	session := authntest.SeedSession(t, db.App, identityUID)

	postInvite := func(key string) *http.Response {
		t.Helper()
		url := srv.URL + "/practices/" + practiceID + "/engagements/" + engagementID + "/portal-invite"
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		authntest.AddSessionCookie(req, session)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		return resp
	}
	decode := func(resp *http.Response) portalinvite.InviteResponse {
		t.Helper()
		var out portalinvite.InviteResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return out
	}

	first := postInvite("portal-invite-retry-key")
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first call status = %d, want %d", first.StatusCode, http.StatusCreated)
	}
	firstOut := decode(first)

	retried := postInvite("portal-invite-retry-key")
	defer retried.Body.Close()
	if retried.StatusCode != http.StatusCreated {
		t.Fatalf("retried call status = %d, want %d", retried.StatusCode, http.StatusCreated)
	}
	retriedOut := decode(retried)

	if retriedOut.InviteToken != firstOut.InviteToken {
		t.Fatalf("retried inviteToken = %q, want identical stored token %q -- business logic must not re-run on replay",
			retriedOut.InviteToken, firstOut.InviteToken)
	}

	noKey := postInvite("")
	defer noKey.Body.Close()
	noKeyOut := decode(noKey)
	if noKeyOut.InviteToken == firstOut.InviteToken {
		t.Fatalf("call with no Idempotency-Key returned the same token as the earlier call -- expected the handler to re-run and rotate it")
	}
}

// TestEndToEnd_PortalInviteReplaysOnRetryEvenWhenNudgeEnqueueFails is
// ADR-0013's correctness constraint proven at InviteHandler: a Cloud
// Tasks enqueue failure must never surface as a 500, because
// idempotency.Wrap only skips caching a response below status 500 --
// staffauth.Middleware drains the registered nudge (tasknudge.Register/
// Drain) only after its own tx.Commit() succeeds, by which point
// idempotency.Wrap has already cached the 201 this test asserts survives
// unchanged.
func TestEndToEnd_PortalInviteReplaysOnRetryEvenWhenNudgeEnqueueFails(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "e2e-portal-invite-nudge-fail-staff"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "E2E Client", "e2e-nudge-fail@example.com")

	failingEnq := &tasknudge.FakeEnqueuer{Err: errors.New("cloud tasks unavailable")}
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/portal-invite",
		staffauth.Middleware(db.App)(idempotency.Wrap(portalinvite.InviteHandler(failingEnq))))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	session := authntest.SeedSession(t, db.App, identityUID)

	postInvite := func(key string) *http.Response {
		t.Helper()
		url := srv.URL + "/practices/" + practiceID + "/engagements/" + engagementID + "/portal-invite"
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		authntest.AddSessionCookie(req, session)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		return resp
	}
	decode := func(resp *http.Response) portalinvite.InviteResponse {
		t.Helper()
		var out portalinvite.InviteResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return out
	}

	first := postInvite("portal-invite-nudge-fail-retry-key")
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first call status = %d, want %d -- an enqueue failure must never surface as a non-2xx", first.StatusCode, http.StatusCreated)
	}
	firstOut := decode(first)

	retried := postInvite("portal-invite-nudge-fail-retry-key")
	defer retried.Body.Close()
	if retried.StatusCode != http.StatusCreated {
		t.Fatalf("retried call status = %d, want %d", retried.StatusCode, http.StatusCreated)
	}
	retriedOut := decode(retried)
	if retriedOut.InviteToken != firstOut.InviteToken {
		t.Fatalf("retried inviteToken = %q, want identical cached token %q -- an enqueue failure must not have produced a 500 that skipped caching",
			retriedOut.InviteToken, firstOut.InviteToken)
	}
}

// TestEndToEnd_CreateClientNoDuplicateRowOnRetry wires client.CreateHandler
// behind staffauth.Middleware(...)(idempotency.Wrap(...)) exactly as main.go
// does, proving a retried create-Client request with the same
// Idempotency-Key returns the identical Client rather than a second row.
// #397 split the old combined create-Client-and-Engagement handler (#128) --
// saving a Client is free now, so there is no credit to double-spend here;
// that risk moves to the Engagement Request approval endpoint.
func TestEndToEnd_CreateClientNoDuplicateRowOnRetry(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "e2e-create-client-staff"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/clients",
		staffauth.Middleware(db.App)(idempotency.Wrap(client.CreateHandler())))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	session := authntest.SeedSession(t, db.App, identityUID)

	postClient := func(key string) *http.Response {
		t.Helper()
		// Override: true -- this test is about idempotency replay, not the
		// match-refusal path (covered in the client package's own tests), and
		// without it the no-key retry below would collide with the first
		// call's identical GivenName and 409 instead of creating its own row.
		body, err := json.Marshal(client.CreateRequest{Record: client.Record{GivenName: "E2E Client"}, Override: true})
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/practices/"+practiceID+"/clients", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		authntest.AddSessionCookie(req, session)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		return resp
	}
	decode := func(resp *http.Response) client.Record {
		t.Helper()
		var out client.Record
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return out
	}
	clientCount := func() int {
		t.Helper()
		var n int
		if err := db.Admin.QueryRowContext(t.Context(),
			`SELECT count(*) FROM clients WHERE practice_id = $1`,
			practiceID,
		).Scan(&n); err != nil {
			t.Fatalf("count client rows: %v", err)
		}
		return n
	}

	first := postClient("create-client-retry-key")
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first call status = %d, want %d", first.StatusCode, http.StatusCreated)
	}
	firstOut := decode(first)
	if clientCount() != 1 {
		t.Fatalf("client count after first call = %d, want 1", clientCount())
	}

	retried := postClient("create-client-retry-key")
	defer retried.Body.Close()
	if retried.StatusCode != http.StatusCreated {
		t.Fatalf("retried call status = %d, want %d", retried.StatusCode, http.StatusCreated)
	}
	retriedOut := decode(retried)
	if retriedOut.ID != firstOut.ID {
		t.Fatalf("retried response = %+v, want identical stored response %+v -- business logic must not re-run on replay", retriedOut, firstOut)
	}
	if n := clientCount(); n != 1 {
		t.Fatalf("client count after retry = %d, want 1 (no second row inserted)", n)
	}

	noKey := postClient("")
	defer noKey.Body.Close()
	if noKey.StatusCode != http.StatusCreated {
		t.Fatalf("no-key call status = %d, want %d", noKey.StatusCode, http.StatusCreated)
	}
	noKeyOut := decode(noKey)
	if noKeyOut.ID == firstOut.ID {
		t.Fatalf("call with no Idempotency-Key returned the same Client as the earlier call -- expected the handler to re-run")
	}
	if n := clientCount(); n != 2 {
		t.Fatalf("client count after no-key call = %d, want 2 (second call inserts its own row)", n)
	}
}

// TestEndToEnd_MessageCreateNoDuplicateRowOnRetry wires message.CreateHandler
// behind staffauth.Middleware(...)(idempotency.Wrap(...)) exactly as main.go
// does, proving a retried create-Message request with the same
// Idempotency-Key returns the identical stored Message and inserts exactly
// one messages row -- the concrete duplicate-post risk #129 exists to
// close.
func TestEndToEnd_MessageCreateNoDuplicateRowOnRetry(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "e2e-message-create-staff"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "E2E Client", "e2e-message@example.com")
	seedPushSubscription(t, db, "client", clientID, "https://push.example.com/message-create-retry")

	pusher := push.NewFakePusher()
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/messages",
		staffauth.Middleware(db.App)(
			idempotency.Wrap(message.CreateHandler(objectstore.NewMemoryStore(), pusher))))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	session := authntest.SeedSession(t, db.App, identityUID)

	postMessage := func(key string) *http.Response {
		t.Helper()
		body, err := json.Marshal(message.CreateRequest{Body: "hello from e2e"})
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		url := srv.URL + "/practices/" + practiceID + "/engagements/" + engagementID + "/messages"
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		authntest.AddSessionCookie(req, session)
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		return resp
	}
	decode := func(resp *http.Response) message.Message {
		t.Helper()
		var out message.Message
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return out
	}
	messageCount := func() int {
		t.Helper()
		var n int
		if err := db.Admin.QueryRowContext(t.Context(),
			`SELECT count(*) FROM messages WHERE engagement_id = $1`,
			engagementID,
		).Scan(&n); err != nil {
			t.Fatalf("count messages rows: %v", err)
		}
		return n
	}

	first := postMessage("message-create-retry-key")
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first call status = %d, want %d", first.StatusCode, http.StatusCreated)
	}
	firstOut := decode(first)
	if firstOut.SenderID == clientID {
		t.Fatalf("firstOut.SenderID = %q, want the calling Staff's id, not the seeded Client's -- staff-side create must not stamp the Client identity", firstOut.SenderID)
	}
	if messageCount() != 1 {
		t.Fatalf("message count after first call = %d, want 1", messageCount())
	}
	if n := len(pusher.Calls()); n != 1 {
		t.Fatalf("push calls after first call = %d, want 1", n)
	}

	retried := postMessage("message-create-retry-key")
	defer retried.Body.Close()
	if retried.StatusCode != http.StatusCreated {
		t.Fatalf("retried call status = %d, want %d", retried.StatusCode, http.StatusCreated)
	}
	retriedOut := decode(retried)
	if retriedOut.MessageID != firstOut.MessageID {
		t.Fatalf("retried response = %+v, want identical stored response %+v -- business logic must not re-run on replay", retriedOut, firstOut)
	}
	if n := messageCount(); n != 1 {
		t.Fatalf("message count after retry = %d, want 1 (no second row inserted)", n)
	}
	if n := len(pusher.Calls()); n != 1 {
		t.Fatalf("push calls after retry = %d, want 1 (replay must not re-notify)", n)
	}

	noKey := postMessage("")
	defer noKey.Body.Close()
	if noKey.StatusCode != http.StatusCreated {
		t.Fatalf("no-key call status = %d, want %d", noKey.StatusCode, http.StatusCreated)
	}
	noKeyOut := decode(noKey)
	if noKeyOut.MessageID == firstOut.MessageID {
		t.Fatalf("call with no Idempotency-Key returned the same Message as the earlier call -- expected the handler to re-run")
	}
	if n := messageCount(); n != 2 {
		t.Fatalf("message count after no-key call = %d, want 2 (second call inserts its own row)", n)
	}
	if n := len(pusher.Calls()); n != 2 {
		t.Fatalf("push calls after no-key call = %d, want 2", n)
	}
}

// TestEndToEnd_MessageCreateMultipartNoDuplicateUploadOnRetry covers the
// multipart/form-data path (an attachment upload alongside the row insert)
// that makes message.CreateHandler different from #128's /clients and
// #126's portal-invite: a retried multipart create with the same
// Idempotency-Key must replay the stored response without a second
// ObjectStore.Put, not just without a second messages row.
func TestEndToEnd_MessageCreateMultipartNoDuplicateUploadOnRetry(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "e2e-message-multipart-staff"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "E2E Multipart Client", "e2e-multipart@example.com")

	store := newCountingPutStore()
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/messages",
		staffauth.Middleware(db.App)(
			idempotency.Wrap(message.CreateHandler(store, push.NewFakePusher()))))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	session := authntest.SeedSession(t, db.App, identityUID)

	postMultipart := func(key string) *http.Response {
		t.Helper()
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		if err := writer.WriteField("body", "photo attached"); err != nil {
			t.Fatalf("write body field: %v", err)
		}
		part, err := writer.CreateFormFile("attachment", "photo.png")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write(pngBytes); err != nil {
			t.Fatalf("write attachment content: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close multipart writer: %v", err)
		}

		url := srv.URL + "/practices/" + practiceID + "/engagements/" + engagementID + "/messages"
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, &buf)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		authntest.AddSessionCookie(req, session)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		return resp
	}
	decode := func(resp *http.Response) message.Message {
		t.Helper()
		var out message.Message
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return out
	}

	first := postMultipart("message-multipart-retry-key")
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first call status = %d, want %d", first.StatusCode, http.StatusCreated)
	}
	firstOut := decode(first)
	if firstOut.AttachmentFilename != "photo.png" {
		t.Fatalf("first response AttachmentFilename = %q, want %q", firstOut.AttachmentFilename, "photo.png")
	}
	if n := store.puts.Load(); n != 1 {
		t.Fatalf("store Put calls after first call = %d, want 1", n)
	}

	retried := postMultipart("message-multipart-retry-key")
	defer retried.Body.Close()
	if retried.StatusCode != http.StatusCreated {
		t.Fatalf("retried call status = %d, want %d", retried.StatusCode, http.StatusCreated)
	}
	retriedOut := decode(retried)
	if retriedOut.MessageID != firstOut.MessageID {
		t.Fatalf("retried response = %+v, want identical stored response %+v -- business logic must not re-run on replay", retriedOut, firstOut)
	}
	if n := store.puts.Load(); n != 1 {
		t.Fatalf("store Put calls after retry = %d, want 1 (no second upload)", n)
	}

	noKey := postMultipart("")
	defer noKey.Body.Close()
	if noKey.StatusCode != http.StatusCreated {
		t.Fatalf("no-key call status = %d, want %d", noKey.StatusCode, http.StatusCreated)
	}
	noKeyOut := decode(noKey)
	if noKeyOut.MessageID == firstOut.MessageID {
		t.Fatalf("call with no Idempotency-Key returned the same Message as the earlier call -- expected the handler to re-run")
	}
	if n := store.puts.Load(); n != 2 {
		t.Fatalf("store Put calls after no-key call = %d, want 2 (second call uploads its own attachment) -- without this control, the retry assertion above can't distinguish a real replay from the handler simply running once for an unrelated reason", n)
	}
}
