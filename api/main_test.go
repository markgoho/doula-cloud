package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/staffinvite"
	"doula-cloud/api/internal/staffnotice"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// testWorkerFrom is every routes() test worker's stand-in From identity.
const testWorkerFrom = "DoulaCloud <notifications@mg.example.test>"

// roleDoula names the practice_role every routes() test that seeds a
// plain roster member (not an Owner) uses, so goconst has one spelling
// to point at rather than three string literals.
const (
	roleDoula = "doula"
	roleOwner = "owner"
)

// testWorkerReplyTo is every Platform-voice routes() test worker's
// stand-in ReplyTo (ADR-0011's support@ inbox), shared by
// testLowCreditWorker, testPayoutOutboxWorker, and testPaymentOutboxWorker
// below.
const testWorkerReplyTo = "support@mg.example.test"

// testWorker is every routes() test's stand-in outbox worker -- its
// mail.FakeSender is never asserted on here, per #219's own package
// covering send/retry/dead-letter behavior; these tests only need routes()
// to compile and mount the route.
var testWorker = portalinvite.NewWorker(outbox.Mailer{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: "noreply@mg.example.test"})

// testLowCreditWorker is every routes() test's stand-in for the
// out-of-Credits outbox worker (#342), the billing package's counterpart
// to testWorker above.
var testLowCreditWorker = billing.Worker{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo}

// testPayoutOutboxWorker is every routes() test's stand-in for the
// payout-account-incomplete outbox worker (#343), the payments package's
// counterpart to testLowCreditWorker above.
var testPayoutOutboxWorker = payments.Worker{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo}

// testConnectNudgeOutboxWorker is every routes() test's stand-in for the
// connect-nudge outbox worker (#917), the payments package's third
// outbox worker beside the two either side of it here.
var testConnectNudgeOutboxWorker = payments.ConnectNudgeWorker{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo}

// testGapNoticeOutboxWorker is every routes() test's stand-in for the
// coverage-gap outbox worker (#1093).
var testGapNoticeOutboxWorker = oncall.GapNoticeWorker{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo}

// testPaymentOutboxWorker is every routes() test's stand-in for the
// payment-received outbox worker (#344), the payments package's
// counterpart to testPayoutOutboxWorker above.
var testPaymentOutboxWorker = payments.PaymentReceivedWorker{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo}

// testSessionNoticeOutboxWorker is every routes() test's stand-in for the
// new-sign-in/session-revoked outbox worker (#345), the sessionnotice
// package's counterpart to testPaymentOutboxWorker above. No AppBaseURL:
// sessionnotice.Worker has none, since neither notice's body links
// anywhere.
var testSessionNoticeOutboxWorker = sessionnotice.NewWorker(outbox.Mailer{Sender: &mail.FakeSender{}, Now: time.Now, From: testWorkerFrom, ReplyTo: testWorkerReplyTo})

// testStaffInviteOutboxWorker is every routes() test's stand-in for the
// Staff invitation outbox worker (#339), the staffinvite package's
// counterpart to testWorker above.
var testStaffInviteOutboxWorker = staffinvite.NewWorker(outbox.Mailer{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo})

// testOfferOutboxWorker is every routes() test's stand-in for the Offer
// outbox worker (#317), the offer package's counterpart to testWorker
// above.
var testOfferOutboxWorker = offer.Worker{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo}

var testStaffNoticeOutboxWorker = staffnotice.NewWorker(outbox.Mailer{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo})

// testEngagementRequestOutboxWorker is every routes() test's stand-in for
// the Engagement Request outbox worker (#398), the engagementrequest
// package's counterpart to testOfferOutboxWorker above.
var testEngagementRequestOutboxWorker = engagementrequest.NewWorker(outbox.Mailer{Sender: &mail.FakeSender{}, Now: time.Now, AppBaseURL: testExpectedOrigin, From: testWorkerFrom, ReplyTo: testWorkerReplyTo})

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

const signupPath = "/api/staff/signup"

// testDeps is the dependency set every routes() test starts from: fakes
// throughout, no database, and the fixture workers declared above.
//
// A test that cares about one dependency takes a copy and overrides that
// field, so what it varies is the only thing it names -- which is what
// #482's struct bought. The nine call sites here used to repeat
// twenty-three positional arguments each, and adding a worker meant
// editing all nine to say nothing new.
func testDeps() Deps {
	// #nosec G101 -- test fixtures, not credentials
	return Deps{
		Verifier: authntest.Verifier{},
		Store:    objectstore.NewMemoryStore(),
		Pusher:   push.NewFakePusher(),

		StripeClient:        billing.NewFakeStripeClient(),
		StripeWebhookSecret: "whsec_test",

		PaymentsClient:               payments.NewFakeClient(),
		PaymentsWebhookSecret:        "whsec_connect_test",
		PaymentsAccountWebhookSecret: "whsec_account_test",

		MailgunWebhookSigningKey: "mailgun_webhook_test_key",
		BounceClearer:            &mail.FakeSender{},
		InternalAuth:             internalauth.FromSecret(testWorkerSecret),

		PortalInviteWorker:      testWorker,
		LowCreditWorker:         testLowCreditWorker,
		PayoutWorker:            testPayoutOutboxWorker,
		ConnectNudgeWorker:      testConnectNudgeOutboxWorker,
		GapNoticeWorker:         testGapNoticeOutboxWorker,
		PaymentReceivedWorker:   testPaymentOutboxWorker,
		SessionNoticeWorker:     testSessionNoticeOutboxWorker,
		StaffInviteWorker:       testStaffInviteOutboxWorker,
		OfferWorker:             testOfferOutboxWorker,
		StaffNoticeWorker:       testStaffNoticeOutboxWorker,
		EngagementRequestWorker: testEngagementRequestOutboxWorker,

		NudgeEnqueuer:   testNudgeEnqueuer,
		ExpectedOrigins: []string{testExpectedOrigin},
	}
}

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
	srv := routedServer(t, nil)

	reqtest.ExpectStatus(t, http.MethodGet, srv.URL+"/api/hello", http.StatusOK)
}

// routedServer serves routes() over testDeps on a server that closes with
// the test. With a db the routes read it as the app role; with none they
// have no database. A test that varies a dependency names it in vary.
func routedServer(t *testing.T, db *testdb.DB, vary ...func(*Deps)) *httptest.Server {
	t.Helper()
	deps := testDeps()
	if db != nil {
		deps.DB = db.App
	}
	for _, v := range vary {
		v(&deps)
	}
	mux, _, _ := routes(deps)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestRoutes_CreateAndEndSession drives #144's two new endpoints through
// the real route table: a valid Bearer token gets a __session cookie
// from create-session, and end-session clears it -- proving both are
// wired under /api and reachable with no Practice/Engagement in the
// path.
func TestRoutes_CreateAndEndSession(t *testing.T) {
	db := testdb.New(t)
	srv := routedServer(t, db, func(d *Deps) { d.Verifier = authntest.Verifier{UID: "uid-1"} })

	createResp := reqtest.Post(t, srv.URL+"/api/session", reqtest.Header("Authorization", "Bearer good-token"))
	defer createResp.Body.Close()
	reqtest.RequireStatus(t, createResp, http.StatusOK)
	createdToken := sessionCookieValue(t, createResp)

	endResp := reqtest.Delete(t, srv.URL+"/api/session", reqtest.Session(createdToken))
	defer endResp.Body.Close()
	reqtest.RequireStatus(t, endResp, http.StatusOK)
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
	srv := routedServer(t, testdb.New(t))

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, signupPath},
		{http.MethodGet, "/api/staff/session"},
		{http.MethodGet, "/api/practices/00000000-0000-0000-0000-000000000000/session"},
	}
	for _, c := range cases {
		reqtest.ExpectStatus(t, c.method, srv.URL+c.path, http.StatusUnauthorized)
	}
}

// TestRoutes_SignupLoginLanding walks the full ticket flow through the
// real route table: sign up a new Practice, fetch the session to find
// where to land, then hit the practice-scoped landing route and confirm
// it recorded last_practice_id.
func TestRoutes_SignupLoginLanding(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "e2e-owner-uid"
	// SecondFactor: true -- #606 requires MFA for an Owner at the
	// Practice-scoped boundary this test's landing request crosses.
	// That gate itself is staffauth's own TestMiddleware_MFAGate; this
	// test is about signup/session/landing wiring, so its fixture
	// stands in for an already-enrolled Owner rather than exercising
	// the gate a second time.
	srv := routedServer(t, db, func(d *Deps) {
		d.Verifier = authntest.Verifier{UID: identityUID, Email: "jamie@example.com", SecondFactor: true}
	})

	signupResp := reqtest.Post(t, srv.URL+signupPath, reqtest.Header("Authorization", "Bearer tok"),
		reqtest.JSON(staffauth.SignupRequest{
			WorkState:    "NY",
			PracticeName: "Riverside Doulas",
			FirstName:    "Jamie", LastName: "Owner",
			Timezone: "America/New_York",
		}))
	defer signupResp.Body.Close()
	if signupResp.StatusCode != http.StatusCreated {
		t.Fatalf("signup status = %d, want %d", signupResp.StatusCode, http.StatusCreated)
	}
	var signedUp staffauth.SignupResponse
	if err := json.UnmarshalRead(signupResp.Body, &signedUp); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}

	// Signup runs before a session exists, so it takes a Bearer ID token
	// -- and hands back the session cookie (#145) that every route after
	// it reads, since #151 left them nothing else to read. Carrying that
	// cookie forward, rather than minting a second session, is what
	// proves the one signup issued actually authenticates the rest of the
	// flow.
	sessionCookie := sessionCookieValue(t, signupResp)

	as := reqtest.Session(sessionCookie)
	session := reqtest.Expect[staffauth.SessionResponse](t, http.MethodGet, srv.URL+"/api/staff/session", http.StatusOK, as)
	if len(session.Memberships) != 1 || session.Memberships[0].PracticeID != signedUp.PracticeID {
		t.Fatalf("memberships = %+v, want single membership at %q", session.Memberships, signedUp.PracticeID)
	}

	landing := reqtest.Expect[staffauth.PracticeSessionResponse](t, http.MethodGet, srv.URL+"/api/practices/"+signedUp.PracticeID+"/session", http.StatusOK, as)
	if landing.PracticeName != "Riverside Doulas" {
		t.Fatalf("practiceName = %q, want %q", landing.PracticeName, "Riverside Doulas")
	}
	if landing.IsContractor {
		t.Fatalf("isContractor = %v, want false for a signed-up Owner", landing.IsContractor)
	}

	var lastPracticeID string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT last_practice_id FROM staff WHERE id = $1`, signedUp.StaffID).Scan(&lastPracticeID); err != nil {
		t.Fatalf("query last_practice_id: %v", err)
	}
	if lastPracticeID != signedUp.PracticeID {
		t.Fatalf("last_practice_id = %q, want %q", lastPracticeID, signedUp.PracticeID)
	}
}

// TestRoutes_PracticeSessionContractorFlag confirms
// staffauth.PracticeSessionHandler's IsContractor field, which #501's contractor
// Add-a-Client door reads to branch before ever calling
// client.SearchHandler -- straight fixtures rather than a full
// signup/invite/accept round trip, since only the response's employment-
// type axis is under test here.
func TestRoutes_PracticeSessionContractorFlag(t *testing.T) {
	db := testdb.New(t)
	srv := routedServer(t, db)
	practice := fixture.Practice(t, db)
	contractor := fixture.Staff(t, db, practice, fixture.Doula(), fixture.Contractor())

	landing := reqtest.Expect[staffauth.PracticeSessionResponse](t, http.MethodGet, srv.URL+"/api/practices/"+practice.ID+"/session", http.StatusOK,
		reqtest.Session(fixture.Session(t, db, contractor)))
	if !landing.IsContractor {
		t.Fatalf("isContractor = %v, want true for a contractor Doula", landing.IsContractor)
	}
	if len(landing.Roles) != 1 || landing.Roles[0] != roleDoula {
		t.Fatalf("roles = %v, want [doula]", landing.Roles)
	}
}

// TestRoutes_StaffInviteSuppressedAddressRefused exercises #861's guard
// through the real route table rather than staffauth's own test server,
// so the closure routes_session.go wires staffauth.Mount's
// SuppressionChecker with -- routes() is the one place that adapts
// mailsuppress.Active to it, to avoid staffauth importing mailsuppress
// back (mailsuppress.Mount already takes a *staffauth.GatedRouter).
func TestRoutes_StaffInviteSuppressedAddressRefused(t *testing.T) {
	db := testdb.New(t)
	srv := routedServer(t, db)
	practice := fixture.Practice(t, db)
	owner := fixture.Staff(t, db, practice, fixture.Owner())
	const email = "blocked@example.com"
	if err := mailsuppress.Record(t.Context(), db.App, email, mailsuppress.CauseBounce, "evt-e2e-861"); err != nil {
		t.Fatalf("seed suppression: %v", err)
	}

	resp := reqtest.Post(t, srv.URL+"/api/practices/"+practice.ID+"/staff/invitations", reqtest.Session(fixture.Session(t, db, owner)),
		reqtest.JSON(staffauth.InviteRequest{Email: email, Roles: []string{roleDoula}, EmploymentType: "employee"}))
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
	refusal := apierrtest.Decode(t, resp)
	if refusal.Details["email"] != staffauth.MsgAddressBlocked {
		t.Fatalf("details = %v, want an email entry naming Blocked email addresses", refusal.Details)
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

// TestInternalCallerAuth_ReadsTheOIDCIdentityItNudgesAs covers the
// other side of ADR-0037's boundary: a Cloud Tasks nudge identifies
// itself as the service account named here, for the same audience the
// guard checks, so the two halves cannot be configured to disagree.
func TestInternalCallerAuth_ReadsTheOIDCIdentityItNudgesAs(t *testing.T) {
	env := map[string]string{
		"INTERNAL_OIDC_SERVICE_ACCOUNT": "doula-api@doula-cloud.iam.gserviceaccount.com",
		"INTERNAL_OIDC_AUDIENCE":        "https://doula-api.example",
		"NOTIFICATION_WORKER_SECRET":    "",
	}

	got := internalCallerAuth(func(name string) string { return env[name] })

	if got.ServiceAccount != env["INTERNAL_OIDC_SERVICE_ACCOUNT"] {
		t.Errorf("ServiceAccount = %q, want %q", got.ServiceAccount, env["INTERNAL_OIDC_SERVICE_ACCOUNT"])
	}
	if got.Audience != env["INTERNAL_OIDC_AUDIENCE"] {
		t.Errorf("Audience = %q, want %q", got.Audience, env["INTERNAL_OIDC_AUDIENCE"])
	}
	if got.Secret != "" {
		t.Errorf("Secret = %q, want empty -- production carries no shared secret", got.Secret)
	}
}

// TestInternalGuard_AcceptsNothingWhenNothingIsConfigured is the failure
// this wiring must not have: a service started with no internal
// configuration at all serving the outbox drain to whoever asks.
func TestInternalGuard_AcceptsNothingWhenNothingIsConfigured(t *testing.T) {
	guard := internalGuard(func(string) string { return "" })

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/internal/outboxes/drain", nil)
	r.Header.Set("Authorization", "Bearer anything")
	if guard.Allow(r) {
		t.Error("Allow() = true with nothing configured, want false")
	}
}

// TestRoutes_CrossOriginStateChangeRejected drives #142's core case
// through the real route table: a state-changing request carrying an
// Origin header that isn't the configured one is rejected before it
// reaches the handler -- proven by getting 403 rather than the 401 a
// missing bearer token would otherwise produce.
//
// The client search (#1829) moved from GET to POST to sit behind the same
// origin check, so a cross-site search is refused before it reaches
// staffauth. A matching Origin is not caught by the check: the request
// still reaches the handler, which rejects it for its own reason (401, no
// bearer token).
func TestRoutes_OriginCheck(t *testing.T) {
	for _, tc := range []struct {
		name   string
		path   string
		origin string
		withDB bool
		want   int
	}{
		{name: "a cross-origin state change is rejected", path: signupPath, origin: "https://evil.example", want: http.StatusForbidden},
		{name: "a cross-origin Client search is rejected", path: "/api/practices/00000000-0000-0000-0000-000000000001/clients/search", origin: "https://evil.example", want: http.StatusForbidden},
		{name: "a matching origin reaches the handler", path: signupPath, origin: testExpectedOrigin, withDB: true, want: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var db *testdb.DB
			if tc.withDB {
				db = testdb.New(t)
			}
			srv := routedServer(t, db)

			reqtest.ExpectStatus(t, http.MethodPost, srv.URL+tc.path, tc.want,
				reqtest.Header("Origin", tc.origin), reqtest.RawJSON([]byte(`{"name":"Ana"}`)))
		})
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
	db := testdb.New(t)
	srv := routedServer(t, db, func(d *Deps) { d.StripeWebhookSecret = stripeWebhookSecret })

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

	reqtest.ExpectStatus(t, http.MethodPost, srv.URL+"/api/stripe/webhook", http.StatusOK,
		reqtest.Header("Stripe-Signature", signed.Header), reqtest.Body("", payload))

	// payments.NewFakeClient's VerifyWebhookSignature accepts anything and
	// reports an unrecognized event type, which PostConnectWebhookHandler
	// also just acknowledges -- no signature header needed to prove the
	// route is reachable with no Origin header.
	reqtest.ExpectStatus(t, http.MethodPost, srv.URL+"/api/stripe/connect-webhook", http.StatusOK)

	// Same shape for the thin-event route: the fake's ParseAccountEvent
	// accepts anything and reports an unrecognized event type, which
	// PostAccountWebhookHandler acknowledges without touching the
	// database.
	reqtest.ExpectStatus(t, http.MethodPost, srv.URL+"/api/stripe/account-webhook", http.StatusOK)

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
	reqtest.ExpectStatus(t, http.MethodPost, srv.URL+"/api/mailgun/webhook", http.StatusOK, reqtest.Body("", mailgunPayload))

	var routesRecorded []string
	for _, a := range testdb.WebhookArrivals(t, db) {
		routesRecorded = append(routesRecorded, a.Route+" "+a.Outcome)
	}
	if want := []string{"stripe_platform dropped", "stripe_connect dropped", "stripe_account dropped", "mailgun dropped"}; !slices.Equal(routesRecorded, want) {
		t.Fatalf("webhook arrivals = %v, want %v", routesRecorded, want)
	}
}
