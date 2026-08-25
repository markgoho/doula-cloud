package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/staffinvite"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// testWorkerFrom is every routes() test worker's stand-in From identity.
const testWorkerFrom = "Doula Cloud <notifications@mg.example.test>"

// testWorkerReplyTo is every Platform-voice routes() test worker's
// stand-in ReplyTo (ADR-0011's support@ inbox), shared by
// testLowCreditWorker, testPayoutOutboxWorker, and testPaymentOutboxWorker
// below.
const testWorkerReplyTo = "support@mg.example.test"

// testWorker is every routes() test's stand-in outbox worker -- its
// mail.FakeSender is never asserted on here, per #219's own package
// covering send/retry/dead-letter behavior; these tests only need routes()
// to compile and mount the route.
var testWorker = portalinvite.Worker{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: "noreply@mg.example.test"}

// testLowCreditWorker is every routes() test's stand-in for the
// out-of-Credits outbox worker (#342), the billing package's counterpart
// to testWorker above.
var testLowCreditWorker = billing.Worker{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo}

// testPayoutOutboxWorker is every routes() test's stand-in for the
// payout-account-incomplete outbox worker (#343), the payments package's
// counterpart to testLowCreditWorker above.
var testPayoutOutboxWorker = payments.Worker{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo}

// testPaymentOutboxWorker is every routes() test's stand-in for the
// payment-received outbox worker (#344), the payments package's
// counterpart to testPayoutOutboxWorker above.
var testPaymentOutboxWorker = payments.PaymentReceivedWorker{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo}

// testSessionNoticeOutboxWorker is every routes() test's stand-in for the
// new-sign-in/session-revoked outbox worker (#345), the sessionnotice
// package's counterpart to testPaymentOutboxWorker above. No AppBaseURL:
// sessionnotice.Worker has none, since neither notice's body links
// anywhere.
var testSessionNoticeOutboxWorker = sessionnotice.Worker{Sender: &mail.FakeSender{}, Now: time.Now, From: testWorkerFrom, ReplyTo: testWorkerReplyTo}

// testStaffInviteOutboxWorker is every routes() test's stand-in for the
// Staff invitation outbox worker (#339), the staffinvite package's
// counterpart to testWorker above.
var testStaffInviteOutboxWorker = staffinvite.Worker{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo}

// testOfferOutboxWorker is every routes() test's stand-in for the Offer
// outbox worker (#317), the offer package's counterpart to testWorker
// above.
var testOfferOutboxWorker = offer.Worker{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo}

// testNudgeEnqueuer is every routes() test's stand-in for ADR-0013's
// Cloud Tasks nudge -- its calls are never asserted on here, per
// tasknudge's own package covering the registry/Fire behavior; these
// tests only need routes() to compile and mount the route.
var testNudgeEnqueuer = &tasknudge.FakeEnqueuer{}

const testWorkerSecret = "worker-secret-test"

// testExpectedOrigin is the routes() tests' stand-in for whichever origin
// EXPECTED_ORIGINS would resolve to in a real environment (see
// resolveExpectedOrigins). None of the pre-existing tests below set an
// Origin header, so its value only matters to the origin-check tests
// added alongside it. Reused above as the outbox workers' AppBaseURL too
// -- in a real environment they're the same host, and goconst flags a
// second literal for the identical value.
const testExpectedOrigin = "https://app.example.test"

func TestHelloHandler(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/hello", nil)
	rec := httptest.NewRecorder()

	helloHandler(rec, req)

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := rec.Body.String(); got != `{"message":"hello world"}`+"\n" {
		t.Fatalf("body = %q", got)
	}
}

// The deployed app reaches the BFF through a Firebase Hosting rewrite that
// forwards /api/** to Cloud Run with the path unchanged, so the health probe
// has to be registered on the same /api prefix the browser uses. Body shape is
// TestHelloHandler's job; this one only pins where the route hangs.
func TestRoutes_HelloUnderAPIPrefix(t *testing.T) {
	mux, _ := routes(authntest.Verifier{}, nil, objectstore.NewMemoryStore(), push.NewFakePusher(), billing.NewFakeStripeClient(), "whsec_test", payments.NewFakeClient(), "whsec_connect_test", "whsec_account_test", testWorker, testWorkerSecret, "mailgun_webhook_test_key", testLowCreditWorker, testPayoutOutboxWorker, testPaymentOutboxWorker, testSessionNoticeOutboxWorker, testStaffInviteOutboxWorker, testOfferOutboxWorker, testNudgeEnqueuer, []string{testExpectedOrigin})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/hello", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/hello status = %d, want 200", resp.StatusCode)
	}
}

// TestRoutes_CreateAndEndSession drives #144's two new endpoints through
// the real route table: a valid Bearer token gets a __session cookie
// from create-session, and end-session clears it -- proving both are
// wired under /api and reachable with no Practice/Engagement in the
// path.
func TestRoutes_CreateAndEndSession(t *testing.T) {
	db := testdb.New(t)
	mux, _ := routes(authntest.Verifier{UID: "uid-1"}, db.App, objectstore.NewMemoryStore(), push.NewFakePusher(), billing.NewFakeStripeClient(), "whsec_test", payments.NewFakeClient(), "whsec_connect_test", "whsec_account_test", testWorker, testWorkerSecret, "mailgun_webhook_test_key", testLowCreditWorker, testPayoutOutboxWorker, testPaymentOutboxWorker, testSessionNoticeOutboxWorker, testStaffInviteOutboxWorker, testOfferOutboxWorker, testNudgeEnqueuer, []string{testExpectedOrigin})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	createReq, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/session", nil)
	if err != nil {
		t.Fatalf("build create request: %v", err)
	}
	createReq.Header.Set("Authorization", "Bearer good-token")
	createResp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/session status = %d, want %d", createResp.StatusCode, http.StatusOK)
	}
	var createdCookie *http.Cookie
	for _, c := range createResp.Cookies() {
		if c.Name == session.CookieName {
			createdCookie = c
		}
	}
	if createdCookie == nil {
		t.Fatal("no __session cookie set by POST /api/session")
	}

	endReq, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, srv.URL+"/api/session", nil)
	if err != nil {
		t.Fatalf("build end request: %v", err)
	}
	authntest.AddSessionCookie(endReq, createdCookie.Value)
	endResp, err := http.DefaultClient.Do(endReq)
	if err != nil {
		t.Fatalf("end request: %v", err)
	}
	defer endResp.Body.Close()
	if endResp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE /api/session status = %d, want %d", endResp.StatusCode, http.StatusOK)
	}
	var clearedCookie *http.Cookie
	for _, c := range endResp.Cookies() {
		if c.Name == session.CookieName {
			clearedCookie = c
		}
	}
	if clearedCookie == nil || clearedCookie.MaxAge >= 0 {
		t.Fatalf("DELETE /api/session did not clear the cookie: %+v", clearedCookie)
	}
	// Both endpoints are wired to the same session store, so the row the
	// create endpoint added is the one the end endpoint removed.
	if got := authntest.CountFor(t, db.App, "uid-1"); got != 0 {
		t.Fatalf("session rows for uid-1 = %d, want 0", got)
	}
}

// sessionCookieValue returns the token the __session cookie on resp
// carries, failing the test if the response set no such cookie. It is a
// credential to resend, never something to assert on.
func sessionCookieValue(t *testing.T, resp *http.Response) string {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == session.CookieName {
			return c.Value
		}
	}
	t.Fatalf("no %s cookie on the response", session.CookieName)
	return ""
}

func TestResolvePort(t *testing.T) {
	t.Setenv("PORT", "")
	if got := resolvePort(); got != "8080" {
		t.Fatalf("resolvePort() = %q, want 8080", got)
	}

	t.Setenv("PORT", "9090")
	if got := resolvePort(); got != "9090" {
		t.Fatalf("resolvePort() = %q, want 9090", got)
	}
}

func TestRoutes_MissingTokenPaths(t *testing.T) {
	db := testdb.New(t)
	mux, _ := routes(authntest.Verifier{}, db.App, objectstore.NewMemoryStore(), push.NewFakePusher(), billing.NewFakeStripeClient(), "whsec_test", payments.NewFakeClient(), "whsec_connect_test", "whsec_account_test", testWorker, testWorkerSecret, "mailgun_webhook_test_key", testLowCreditWorker, testPayoutOutboxWorker, testPaymentOutboxWorker, testSessionNoticeOutboxWorker, testStaffInviteOutboxWorker, testOfferOutboxWorker, testNudgeEnqueuer, []string{testExpectedOrigin})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/staff/signup"},
		{http.MethodGet, "/api/staff/session"},
		{http.MethodGet, "/api/practices/00000000-0000-0000-0000-000000000000/session"},
	}
	for _, c := range cases {
		req, err := http.NewRequestWithContext(t.Context(), c.method, srv.URL+c.path, nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request %s %s: %v", c.method, c.path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", c.method, c.path, resp.StatusCode, http.StatusUnauthorized)
		}
	}
}

// TestRoutes_SignupLoginLanding walks the full ticket flow through the
// real route table: sign up a new Practice, fetch the session to find
// where to land, then hit the practice-scoped landing route and confirm
// it recorded last_practice_id.
func TestRoutes_SignupLoginLanding(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "e2e-owner-uid"
	mux, _ := routes(authntest.Verifier{UID: identityUID}, db.App, objectstore.NewMemoryStore(), push.NewFakePusher(), billing.NewFakeStripeClient(), "whsec_test", payments.NewFakeClient(), "whsec_connect_test", "whsec_account_test", testWorker, testWorkerSecret, "mailgun_webhook_test_key", testLowCreditWorker, testPayoutOutboxWorker, testPaymentOutboxWorker, testSessionNoticeOutboxWorker, testStaffInviteOutboxWorker, testOfferOutboxWorker, testNudgeEnqueuer, []string{testExpectedOrigin})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	signupBody, _ := json.Marshal(staffauth.SignupRequest{
		PracticeName: "Riverside Doulas",
		StaffName:    "Jamie Owner",
		StaffEmail:   "jamie@example.com",
	})
	signupReq, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/signup", bytes.NewReader(signupBody))
	signupReq.Header.Set("Authorization", "Bearer tok")
	signupResp, err := http.DefaultClient.Do(signupReq)
	if err != nil {
		t.Fatalf("signup request: %v", err)
	}
	defer signupResp.Body.Close()
	if signupResp.StatusCode != http.StatusCreated {
		t.Fatalf("signup status = %d, want %d", signupResp.StatusCode, http.StatusCreated)
	}
	var signedUp staffauth.SignupResponse
	if err := json.NewDecoder(signupResp.Body).Decode(&signedUp); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}

	// Signup runs before a session exists, so it takes a Bearer ID token
	// -- and hands back the session cookie (#145) that every route after
	// it reads, since #151 left them nothing else to read. Carrying that
	// cookie forward, rather than minting a second session, is what
	// proves the one signup issued actually authenticates the rest of the
	// flow.
	sessionCookie := sessionCookieValue(t, signupResp)

	sessionReq, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/staff/session", nil)
	authntest.AddSessionCookie(sessionReq, sessionCookie)
	sessionResp, err := http.DefaultClient.Do(sessionReq)
	if err != nil {
		t.Fatalf("session request: %v", err)
	}
	defer sessionResp.Body.Close()
	var session staffauth.SessionResponse
	if err := json.NewDecoder(sessionResp.Body).Decode(&session); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if len(session.Memberships) != 1 || session.Memberships[0].PracticeID != signedUp.PracticeID {
		t.Fatalf("memberships = %+v, want single membership at %q", session.Memberships, signedUp.PracticeID)
	}

	landingReq, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/practices/"+signedUp.PracticeID+"/session", nil)
	authntest.AddSessionCookie(landingReq, sessionCookie)
	landingResp, err := http.DefaultClient.Do(landingReq)
	if err != nil {
		t.Fatalf("landing request: %v", err)
	}
	defer landingResp.Body.Close()
	if landingResp.StatusCode != http.StatusOK {
		t.Fatalf("landing status = %d, want %d", landingResp.StatusCode, http.StatusOK)
	}
	var landing practiceSessionResponse
	if err := json.NewDecoder(landingResp.Body).Decode(&landing); err != nil {
		t.Fatalf("decode landing response: %v", err)
	}
	if landing.PracticeName != "Riverside Doulas" {
		t.Fatalf("practiceName = %q, want %q", landing.PracticeName, "Riverside Doulas")
	}

	var lastPracticeID string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT last_practice_id FROM staff WHERE id = $1`, signedUp.StaffID).Scan(&lastPracticeID); err != nil {
		t.Fatalf("query last_practice_id: %v", err)
	}
	if lastPracticeID != signedUp.PracticeID {
		t.Fatalf("last_practice_id = %q, want %q", lastPracticeID, signedUp.PracticeID)
	}
}

func TestResolveExpectedOrigins(t *testing.T) {
	t.Setenv("EXPECTED_ORIGINS", "")
	if got := resolveExpectedOrigins(); got != nil {
		t.Fatalf("resolveExpectedOrigins() = %v, want nil", got)
	}

	t.Setenv("EXPECTED_ORIGINS", "https://a.example, https://b.example,,")
	got := resolveExpectedOrigins()
	want := []string{"https://a.example", "https://b.example"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("resolveExpectedOrigins() = %v, want %v", got, want)
	}
}

// TestRoutes_CrossOriginStateChangeRejected drives #142's core case
// through the real route table: a state-changing request carrying an
// Origin header that isn't the configured one is rejected before it
// reaches the handler -- proven by getting 403 rather than the 401 a
// missing bearer token would otherwise produce.
func TestRoutes_CrossOriginStateChangeRejected(t *testing.T) {
	mux, _ := routes(authntest.Verifier{}, nil, objectstore.NewMemoryStore(), push.NewFakePusher(), billing.NewFakeStripeClient(), "whsec_test", payments.NewFakeClient(), "whsec_connect_test", "whsec_account_test", testWorker, testWorkerSecret, "mailgun_webhook_test_key", testLowCreditWorker, testPayoutOutboxWorker, testPaymentOutboxWorker, testSessionNoticeOutboxWorker, testStaffInviteOutboxWorker, testOfferOutboxWorker, testNudgeEnqueuer, []string{testExpectedOrigin})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/signup", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Origin", "https://evil.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestRoutes_MatchingOriginAllowed proves a matching Origin header
// doesn't get caught by the check -- the request still reaches the
// handler, which rejects it for its own reason (401, no bearer token),
// not the origin check's 403.
func TestRoutes_MatchingOriginAllowed(t *testing.T) {
	db := testdb.New(t)
	mux, _ := routes(authntest.Verifier{}, db.App, objectstore.NewMemoryStore(), push.NewFakePusher(), billing.NewFakeStripeClient(), "whsec_test", payments.NewFakeClient(), "whsec_connect_test", "whsec_account_test", testWorker, testWorkerSecret, "mailgun_webhook_test_key", testLowCreditWorker, testPayoutOutboxWorker, testPaymentOutboxWorker, testSessionNoticeOutboxWorker, testStaffInviteOutboxWorker, testOfferOutboxWorker, testNudgeEnqueuer, []string{testExpectedOrigin})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/signup", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Origin", testExpectedOrigin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestRoutes_StripeWebhooksSucceedWithNoOrigin proves #142's explicit
// carve-out: all three server-to-server Stripe webhook routes, which send
// no Origin header, still succeed through the real route table rather than
// being caught by the origin check meant for browser callers. The third is
// the v2 account event destination (#247) -- a separate route from the
// Connect one because a Stripe destination carries one payload type, thin
// or snapshot, never both.
func TestRoutes_StripeWebhooksSucceedWithNoOrigin(t *testing.T) {
	const stripeWebhookSecret = "whsec_test"
	mux, _ := routes(authntest.Verifier{}, nil, objectstore.NewMemoryStore(), push.NewFakePusher(), billing.NewFakeStripeClient(), stripeWebhookSecret, payments.NewFakeClient(), "whsec_connect_test", "whsec_account_test", testWorker, testWorkerSecret, "mailgun_webhook_test_key", testLowCreditWorker, testPayoutOutboxWorker, testPaymentOutboxWorker, testSessionNoticeOutboxWorker, testStaffInviteOutboxWorker, testOfferOutboxWorker, testNudgeEnqueuer, []string{testExpectedOrigin})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// An event type PostPurchaseWebhookHandler doesn't act on: proves the
	// request reached the handler and was acknowledged, with no database
	// needed (db is nil above).
	const objectKey = "object"
	payload, err := json.Marshal(map[string]any{
		"id":      "evt_other",
		objectKey: "event",
		"type":    "payment_intent.succeeded",
		"data":    map[string]any{objectKey: map[string]any{"id": "pi_test", objectKey: "payment_intent"}},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: stripeWebhookSecret})

	billingReq, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/stripe/webhook", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	billingReq.Header.Set("Stripe-Signature", signed.Header)
	billingResp, err := http.DefaultClient.Do(billingReq)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer billingResp.Body.Close()
	if billingResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/stripe/webhook status = %d, want %d", billingResp.StatusCode, http.StatusOK)
	}

	// payments.NewFakeClient's VerifyWebhookSignature accepts anything and
	// reports an unrecognized event type, which PostConnectWebhookHandler
	// also just acknowledges -- no signature header needed to prove the
	// route is reachable with no Origin header.
	connectReq, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/stripe/connect-webhook", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	connectResp, err := http.DefaultClient.Do(connectReq)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer connectResp.Body.Close()
	if connectResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/stripe/connect-webhook status = %d, want %d", connectResp.StatusCode, http.StatusOK)
	}

	// Same shape for the thin-event route: the fake's ParseAccountEvent
	// accepts anything and reports an unrecognized event type, which
	// PostAccountWebhookHandler acknowledges without touching the
	// database.
	accountReq, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/stripe/account-webhook", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	accountResp, err := http.DefaultClient.Do(accountReq)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer accountResp.Body.Close()
	if accountResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/stripe/account-webhook status = %d, want %d", accountResp.StatusCode, http.StatusOK)
	}

	// Same shape for the Mailgun webhook (#340): an event type
	// PostBounceWebhookHandler doesn't act on, signed with the same key
	// threaded through routes() above, proves the route is mounted and
	// reachable with no Origin header and no database needed.
	const mailgunTimestamp, mailgunToken = "1700000000", "guardrail-test-token"
	mailgunMAC := hmac.New(sha256.New, []byte("mailgun_webhook_test_key"))
	mailgunMAC.Write([]byte(mailgunTimestamp + mailgunToken))
	mailgunPayload, err := json.Marshal(map[string]any{
		"signature": map[string]any{
			"timestamp": mailgunTimestamp,
			"token":     mailgunToken,
			"signature": hex.EncodeToString(mailgunMAC.Sum(nil)),
		},
		"event-data": map[string]any{"id": "evt_guardrail", "event": "delivered"},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	mailgunReq, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/mailgun/webhook", bytes.NewReader(mailgunPayload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	mailgunResp, err := http.DefaultClient.Do(mailgunReq)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer mailgunResp.Body.Close()
	if mailgunResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/mailgun/webhook status = %d, want %d", mailgunResp.StatusCode, http.StatusOK)
	}
}
