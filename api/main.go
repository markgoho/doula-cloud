// Command doula-cloud-api runs the Go BFF.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	// Registers the "pgx" driver with database/sql; never referenced by name.
	_ "github.com/jackc/pgx/v5/stdlib"

	"cloud.google.com/go/storage"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/csrf"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/pushsub"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/visit"
)

type helloResponse struct {
	Message string `json:"message"`
}

func helloHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(helloResponse{Message: "hello world"}); err != nil {
		log.Printf("helloHandler: encode response: %v", err)
	}
}

func resolvePort() string {
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	return "8080"
}

// resolveExpectedOrigins reads the comma-separated EXPECTED_ORIGINS env
// var into the origins csrf.Wrap treats as same-site. A list, not a
// single value, because the local stack (app/e2e/stack.ts) runs one BFF
// process behind two different front-end origins depending on which
// caller started it -- `vite dev` (local development) and `vite
// preview` (the Playwright stack) -- and both need to pass. Production
// sets exactly one, the Firebase Hosting site's own origin (see #139).
// Unset resolves to an empty list, which rejects every state-changing
// request that carries an Origin header -- fail closed rather than
// silently accepting any origin if this is ever left unconfigured.
func resolveExpectedOrigins() []string {
	raw := os.Getenv("EXPECTED_ORIGINS")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if origin := strings.TrimSpace(part); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

// practiceSessionResponse confirms to the frontend which Practice the
// caller landed on -- and, as a side effect of running through
// staffauth.Middleware, records it as the Staff member's last-used
// Practice for their next login.
type practiceSessionResponse struct {
	PracticeID   string   `json:"practiceId"`
	PracticeName string   `json:"practiceName"`
	Roles        []string `json:"roles"`
}

func practiceSessionHandler(w http.ResponseWriter, r *http.Request) {
	tx, _ := staffauth.Tx(r.Context())
	staffID, _ := staffauth.StaffID(r.Context())
	practiceID, _ := staffauth.PracticeID(r.Context())

	var name string
	if err := tx.QueryRowContext(r.Context(), `SELECT name FROM practices WHERE id = $1`, practiceID).Scan(&name); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	roles, err := staffauth.Roles(r.Context(), tx, practiceID, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(practiceSessionResponse{PracticeID: practiceID, PracticeName: name, Roles: roles}); err != nil {
		log.Printf("practiceSessionHandler: encode response: %v", err)
	}
}

// ownerAndAdmin is the role declaration for every GatedRouter route
// ADR-0008's read table admits to Owner and Admin only (Staff roster,
// Credit balance and ledger, Contract's money-bearing Signed PDF and
// Invoice history) -- named once so golangci-lint's package-wide goconst
// check doesn't see four independent literals to flag.
var ownerAndAdmin = []string{"owner", "admin"}

// routes builds the BFF's route table. verifier, db, store, pusher,
// stripeClient, and paymentsClient are threaded through so tests can
// substitute a fake Identity Platform verifier, a test Postgres instance,
// an in-memory ObjectStore, an in-memory Pusher, and in-memory billing.StripeClient
// / payments.Client doubles instead of the real ones main() wires up.
//
// The whole mux is wrapped in csrf.Wrap, rather than only the
// authenticated routes: the bootstrap endpoints (signup, invitation
// acceptance) are state-changing too, and both they and the two Stripe
// webhook routes rely on the same "no Origin header, no rejection" rule
// -- there is no separate carve-out for either.
//
// routes' second return value is GatedRouter's registry of every GET it
// mounted -- main() never looks at it, but the guardrail tests in
// main_test.go walk it (and cross-check it against this function's own
// source for a route that bypassed the gate entirely) without needing a
// live server.
func routes(verifier authn.Verifier, db *sql.DB, store objectstore.ObjectStore, pusher push.Pusher, stripeClient billing.StripeClient, stripeWebhookSecret string, paymentsClient payments.Client, paymentsWebhookSecret, paymentsAccountWebhookSecret string, outboxWorker portalinvite.Worker, outboxWorkerSecret string, mailgunWebhookSigningKey string, lowCreditOutboxWorker billing.Worker, payoutOutboxWorker payments.Worker, paymentOutboxWorker payments.PaymentReceivedWorker, sessionNoticeOutboxWorker sessionnotice.Worker, expectedOrigins []string) (http.Handler, []staffauth.GatedRoute) {
	mux := http.NewServeMux()
	// GatedRouter (staffauth/gate.go) is the only door for a GET behind
	// staffauth.Middleware: Get panics at startup if a route has no role
	// declaration, so a forgotten one fails the binary rather than
	// silently opening to every Staff member (#231, #315). AnyStaff
	// declares an endpoint open to every role on purpose; a bare role
	// list names exactly who ADR-0008's read table admits.
	g := staffauth.NewGatedRouter(mux, db)
	// Under /api like every other route: Firebase Hosting rewrites /api/** to
	// this service with the path unchanged, so a bare /hello would be
	// unreachable from the browser. CI's two smoke tests curl this same path
	// against the container and against the raw Cloud Run URL.
	mux.HandleFunc("GET /api/hello", helloHandler)
	mux.Handle("POST /api/session", session.CreateHandler(verifier, db))
	mux.Handle("DELETE /api/session", session.EndHandler(db))
	mux.Handle("POST /api/staff/signup", staffauth.SignupHandler(verifier, db))
	mux.Handle("GET /api/staff/session", staffauth.SessionHandler(db))
	g.Get("/api/practices/{practiceId}/session", staffauth.AnyStaff, http.HandlerFunc(practiceSessionHandler))
	mux.Handle("PATCH /api/practices/{practiceId}/staff/{staffId}/roles",
		staffauth.Middleware(db)(staffauth.AssignRolesHandler()))
	// Staff roster: Owner and Admin only (ADR-0008's read table) -- a
	// Doula has no reason to see the full roster.
	g.Get("/api/practices/{practiceId}/staff", ownerAndAdmin, staffauth.ListStaffHandler())
	mux.Handle("DELETE /api/practices/{practiceId}/staff/{staffId}/sessions",
		staffauth.Middleware(db)(staffauth.EndSessionsHandler()))
	// Credit balance and ledger: Owner and Admin only (ADR-0008).
	g.Get("/api/practices/{practiceId}/billing", ownerAndAdmin, billing.GetBalanceHandler())
	mux.Handle("POST /api/practices/{practiceId}/billing/purchases",
		staffauth.Middleware(db)(billing.PostPurchaseHandler(stripeClient)))
	mux.Handle("POST /api/stripe/webhook", billing.PostPurchaseWebhookHandler(db, stripeWebhookSecret))
	mux.Handle("POST /api/practices/{practiceId}/payments/connect",
		staffauth.Middleware(db)(payments.PostConnectHandler(paymentsClient)))
	// ADR-0008's read table has no row for Stripe Connect state; mirroring
	// the write side's Owner-only gate (PostConnectHandler,
	// staffauth.RequireOwner) is the narrowest defensible default until a
	// real rule lands (#267 stays open for that rule).
	g.Get("/api/practices/{practiceId}/payments/connect", []string{"owner"}, payments.GetConnectStatusHandler(paymentsClient))
	mux.Handle("POST /api/stripe/connect-webhook", payments.PostConnectWebhookHandler(db, paymentsClient, paymentsWebhookSecret))
	// A second Connect route, not a second feature: Stripe's v2 account
	// events are thin and a destination carries one payload type, so they
	// cannot share connect-webhook's endpoint or its secret (#247).
	mux.Handle("POST /api/stripe/account-webhook", payments.PostAccountWebhookHandler(db, paymentsClient, paymentsAccountWebhookSecret))
	// Engagements, Visits, Messages, Plan Instances, and Contract scope
	// are open to every Staff role at the mount; the employee/contractor
	// split ADR-0008's read table draws inside that column is
	// attachment-narrowing the handler itself enforces via
	// staffauth.Reader.CanAccessEngagement, not a role declaration.
	g.Get("/api/practices/{practiceId}/clients", staffauth.AnyStaff, engagement.ListHandler())
	mux.Handle("POST /api/practices/{practiceId}/clients",
		staffauth.Middleware(db)(idempotency.Wrap(engagement.CreateHandler(db))))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}", staffauth.AnyStaff, engagement.DetailHandler())
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/visits", staffauth.AnyStaff, visit.ListHandler())
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/visits",
		staffauth.Middleware(db)(visit.CreateHandler()))
	mux.Handle("PATCH /api/practices/{practiceId}/engagements/{engagementId}/visits/{visitId}",
		staffauth.Middleware(db)(visit.ReassignHandler()))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/messages", staffauth.AnyStaff, message.ListHandler())
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/messages",
		staffauth.Middleware(db)(idempotency.Wrap(message.CreateHandler(store, pusher))))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/messages/{messageId}/attachment", staffauth.AnyStaff, message.AttachmentHandler(store))
	// Plan Template and Contract Template: every Staff role (ADR-0008),
	// no attachment narrowing -- a Template isn't Engagement-scoped.
	g.Get("/api/practices/{practiceId}/plan-templates/{planType}", staffauth.AnyStaff, plans.GetTemplateHandler())
	mux.Handle("PUT /api/practices/{practiceId}/plan-templates/{planType}",
		staffauth.Middleware(db)(plans.PutTemplateHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		staffauth.Middleware(db)(plans.PostInstanceHandler()))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}", staffauth.AnyStaff, plans.GetInstanceHandler())
	mux.Handle("PUT /api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		staffauth.Middleware(db)(plans.PutInstanceHandler()))
	g.Get("/api/practices/{practiceId}/contract-template", staffauth.AnyStaff, contracts.GetTemplateHandler())
	mux.Handle("PUT /api/practices/{practiceId}/contract-template",
		staffauth.Middleware(db)(contracts.PutTemplateHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/contract",
		staffauth.Middleware(db)(contracts.PostContractHandler()))
	// Contract read is the sharpest #231 case: scope reaches every role
	// (narrowed by attachment for a contractor, same as above), but money
	// -- and Invoice history -- is Owner/Admin only, never a Doula's,
	// employee or contractor (ADR-0008: "her own agreed fee only ...
	// never the Practice's price"). GetContractHandler does the
	// scope-vs-money split itself via staffauth.Reader +
	// contracts.ContractScope/ContractFull; the mount stays AnyStaff so
	// scope-only Doulas still reach it.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract", staffauth.AnyStaff, contracts.GetContractHandler())
	mux.Handle("PUT /api/practices/{practiceId}/engagements/{engagementId}/contract",
		staffauth.Middleware(db)(contracts.PutContractHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/send",
		staffauth.Middleware(db)(contracts.PostSendContractHandler(pusher)))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/void",
		staffauth.Middleware(db)(contracts.PostVoidContractHandler()))
	// The Signed PDF is a rendered, unredactable document -- it can't be
	// split into scope/money views the way the JSON Contract read can, so
	// it follows the money row: Owner/Admin only.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract/pdf", ownerAndAdmin, contracts.GetSignedContractPDFHandler(store))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/invoices",
		staffauth.Middleware(db)(payments.PostInvoiceHandler(paymentsClient)))
	// Invoice history rides the same money row as Contract money -- see
	// above. A contractor's own-fee narrowing (rather than an outright
	// no) is #317's to build once the Offer/Attachment flow exists.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract/invoices", ownerAndAdmin, payments.GetInvoicesHandler())
	mux.Handle("POST /api/practices/{practiceId}/push-subscriptions",
		staffauth.Middleware(db)(pushsub.RegisterHandler()))
	mux.Handle("DELETE /api/practices/{practiceId}/push-subscriptions",
		staffauth.Middleware(db)(pushsub.UnregisterHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/portal-invite",
		staffauth.Middleware(db)(idempotency.Wrap(portalinvite.InviteHandler())))
	mux.Handle("POST /api/portal/accept-invite", portalinvite.AcceptInviteHandler(verifier, db))
	// Cloud-Scheduler-triggered, not Staff/Client facing (ADR-0010):
	// authenticated by outboxWorkerSecret, not a session, so it sits
	// outside staffauth.Middleware/GatedRouter like the Stripe webhooks
	// above.
	mux.Handle("POST /api/internal/notifications/process-outbox", portalinvite.ProcessOutboxHandler(db, outboxWorker, outboxWorkerSecret))
	// #340/ADR-0010: Mailgun's bounce/complaint delivery-event webhook,
	// same no-staffauth shape as the Stripe webhooks above -- signature
	// verified instead of a session.
	mux.Handle("POST /api/mailgun/webhook", portalinvite.PostBounceWebhookHandler(db, mailgunWebhookSigningKey))
	// Same X-Internal-Secret guard, same Cloud Scheduler cadence, a
	// separate endpoint because the two workers process unrelated
	// outbox tables (ADR-0010, #342).
	mux.Handle("POST /api/internal/notifications/process-low-credit-outbox", billing.ProcessOutboxHandler(db, lowCreditOutboxWorker, outboxWorkerSecret))
	// Same shape again for #343's payout-incomplete outbox.
	mux.Handle("POST /api/internal/notifications/process-payout-outbox", payments.ProcessOutboxHandler(db, payoutOutboxWorker, outboxWorkerSecret))
	// Same shape again for #344's payment-received outbox.
	mux.Handle("POST /api/internal/notifications/process-payment-outbox", payments.ProcessPaymentOutboxHandler(db, paymentOutboxWorker, outboxWorkerSecret))
	// Same shape again for #345's session-notice outbox (new sign-in,
	// session revoked).
	mux.Handle("POST /api/internal/notifications/process-session-notice-outbox", sessionnotice.ProcessOutboxHandler(db, sessionNoticeOutboxWorker, outboxWorkerSecret))
	mux.Handle("GET /api/portal/session", clientauth.SessionHandler(db))
	mux.Handle("GET /api/portal/engagements/{engagementId}",
		clientauth.Middleware(db)(portal.DetailHandler()))
	mux.Handle("GET /api/portal/engagements/{engagementId}/birth-plan",
		clientauth.Middleware(db)(plans.ClientGetBirthPlanHandler()))
	mux.Handle("GET /api/portal/engagements/{engagementId}/contract",
		clientauth.Middleware(db)(contracts.ClientGetContractHandler()))
	mux.Handle("POST /api/portal/engagements/{engagementId}/contract/sign",
		clientauth.Middleware(db)(contracts.ClientPostSignContractHandler(store)))
	mux.Handle("GET /api/portal/engagements/{engagementId}/contract/pdf",
		clientauth.Middleware(db)(contracts.ClientGetSignedContractPDFHandler(store)))
	mux.Handle("GET /api/portal/engagements/{engagementId}/messages",
		clientauth.Middleware(db)(message.ClientListHandler()))
	mux.Handle("POST /api/portal/engagements/{engagementId}/messages",
		clientauth.Middleware(db)(message.ClientCreateHandler(store, pusher)))
	mux.Handle("GET /api/portal/engagements/{engagementId}/messages/{messageId}/attachment",
		clientauth.Middleware(db)(message.ClientAttachmentHandler(store)))
	mux.Handle("POST /api/portal/engagements/{engagementId}/push-subscriptions",
		clientauth.Middleware(db)(pushsub.ClientRegisterHandler()))
	mux.Handle("DELETE /api/portal/engagements/{engagementId}/push-subscriptions",
		clientauth.Middleware(db)(pushsub.ClientUnregisterHandler()))
	return csrf.Wrap(expectedOrigins, mux), g.Routes()
}

func main() {
	port := resolvePort()

	// coverage:ignore reason: requires a real DATABASE_URL and network access, not exercised by unit tests
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		// coverage:ignore reason: requires a real DATABASE_URL and network access, not exercised by unit tests
		log.Fatalf("open db: %v", err)
	}

	// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
	verifier, err := authn.NewFirebaseVerifier(context.Background(), os.Getenv("GCP_PROJECT_ID"))
	if err != nil {
		// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
		log.Fatalf("init verifier: %v", err)
	}

	// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
	gcsClient, err := storage.NewClient(context.Background())
	if err != nil {
		// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
		log.Fatalf("init GCS client: %v", err)
	}
	// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
	store := objectstore.NewGCSStore(gcsClient, os.Getenv("GCS_ATTACHMENTS_BUCKET"))

	// coverage:ignore reason: constructs the real Web Push client, not exercised by unit tests
	pusher := push.NewVAPIDPusher(os.Getenv("VAPID_PUBLIC_KEY"), os.Getenv("VAPID_PRIVATE_KEY"), os.Getenv("VAPID_SUBSCRIBER"))

	// coverage:ignore reason: constructs the real Stripe client, not exercised by unit tests
	stripeClient := billing.NewStripeAPIClient(os.Getenv("STRIPE_API_KEY"), os.Getenv("STRIPE_CREDIT_PRICE_ID"), os.Getenv("APP_BASE_URL"))

	// coverage:ignore reason: constructs the real Stripe client, not exercised by unit tests
	paymentsClient := payments.NewStripeAPIClient(os.Getenv("STRIPE_API_KEY"), os.Getenv("APP_BASE_URL"))

	// coverage:ignore reason: constructs the real Mailgun-backed sender, not exercised by unit tests
	mailgunDomain := os.Getenv("MAILGUN_DOMAIN")
	// coverage:ignore reason: constructs the real Mailgun-backed sender, not exercised by unit tests
	outboxWorker := portalinvite.Worker{
		Sender:     mail.NewMailgunSender(os.Getenv("MAILGUN_API_KEY"), mailgunDomain),
		Now:        time.Now,
		AppBaseURL: os.Getenv("APP_BASE_URL"),
		From:       "Doula Cloud <notifications@" + mailgunDomain + ">",
		ReplyTo:    "noreply@" + mailgunDomain,
	}

	// coverage:ignore reason: constructs the real Mailgun-backed sender, not exercised by unit tests
	lowCreditOutboxWorker := billing.Worker{
		Sender:     mail.NewMailgunSender(os.Getenv("MAILGUN_API_KEY"), mailgunDomain),
		Now:        time.Now,
		AppBaseURL: os.Getenv("APP_BASE_URL"),
		From:       "Doula Cloud <notifications@" + mailgunDomain + ">",
		// Platform voice (ADR-0011): a monitored inbox, not noreply --
		// unlike the Client portal invite's Practice voice above.
		ReplyTo: "support@" + mailgunDomain,
	}

	// coverage:ignore reason: constructs the real Mailgun-backed sender, not exercised by unit tests
	payoutOutboxWorker := payments.Worker{
		Sender:     mail.NewMailgunSender(os.Getenv("MAILGUN_API_KEY"), mailgunDomain),
		Now:        time.Now,
		AppBaseURL: os.Getenv("APP_BASE_URL"),
		From:       "Doula Cloud <notifications@" + mailgunDomain + ">",
		// Platform voice (ADR-0011), same as lowCreditOutboxWorker above.
		ReplyTo: "support@" + mailgunDomain,
	}

	// coverage:ignore reason: constructs the real Mailgun-backed sender, not exercised by unit tests
	paymentOutboxWorker := payments.PaymentReceivedWorker{
		Sender:     mail.NewMailgunSender(os.Getenv("MAILGUN_API_KEY"), mailgunDomain),
		Now:        time.Now,
		AppBaseURL: os.Getenv("APP_BASE_URL"),
		From:       "Doula Cloud <notifications@" + mailgunDomain + ">",
		// Platform voice (ADR-0011), same as lowCreditOutboxWorker above.
		ReplyTo: "support@" + mailgunDomain,
	}

	// coverage:ignore reason: constructs the real Mailgun-backed sender, not exercised by unit tests
	sessionNoticeOutboxWorker := sessionnotice.Worker{
		Sender: mail.NewMailgunSender(os.Getenv("MAILGUN_API_KEY"), mailgunDomain),
		Now:    time.Now,
		From:   "Doula Cloud <notifications@" + mailgunDomain + ">",
		// Platform voice (ADR-0011), same as lowCreditOutboxWorker above.
		ReplyTo: "support@" + mailgunDomain,
	}

	handler, _ := routes(verifier, db, store, pusher, stripeClient, os.Getenv("STRIPE_WEBHOOK_SECRET"), paymentsClient, os.Getenv("STRIPE_CONNECT_WEBHOOK_SECRET"), os.Getenv("STRIPE_ACCOUNT_WEBHOOK_SECRET"), outboxWorker, os.Getenv("NOTIFICATION_WORKER_SECRET"), os.Getenv("MAILGUN_WEBHOOK_SIGNING_KEY"), lowCreditOutboxWorker, payoutOutboxWorker, paymentOutboxWorker, sessionNoticeOutboxWorker, resolveExpectedOrigins())
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("listening on port %s", port)
	// coverage:ignore reason: listener startup, not exercised by unit tests
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
