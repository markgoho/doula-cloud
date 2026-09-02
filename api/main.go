// Command doula-cloud-api runs the Go BFF.
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	// Registers the "pgx" driver with database/sql; never referenced by name.
	_ "github.com/jackc/pgx/v5/stdlib"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/storage"

	"doula-cloud/api/internal/authmail"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/sitebuild"
	"doula-cloud/api/internal/staffinvite"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/website"
)

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

	// All eight mail kinds share one Mailgun domain/credential and one
	// APP_BASE_URL, so they share one Sender construction and the two
	// From/ReplyTo identities ADR-0011 defines -- minting eight otherwise
	// identical Mailgun senders bought nothing.
	//
	// coverage:ignore reason: constructs the real Mailgun-backed sender, not exercised by unit tests
	mailgunDomain := os.Getenv("MAILGUN_DOMAIN")
	// coverage:ignore reason: constructs the real Mailgun-backed sender, not exercised by unit tests
	mailgunSender := mail.NewMailgunSender(os.Getenv("MAILGUN_API_KEY"), mailgunDomain)
	appBaseURL := os.Getenv("APP_BASE_URL")
	notificationsFrom := "Doula Cloud <notifications@" + mailgunDomain + ">"
	// Platform voice (ADR-0011): a monitored inbox, not noreply -- every
	// kind except the Client portal invite below, which is Practice
	// voice.
	supportReplyTo := "support@" + mailgunDomain

	outboxWorker := portalinvite.Worker{
		Sender: mailgunSender, Now: time.Now, AppBaseURL: appBaseURL,
		From: notificationsFrom, ReplyTo: "noreply@" + mailgunDomain,
	}
	lowCreditOutboxWorker := billing.Worker{
		Sender: mailgunSender, Now: time.Now, AppBaseURL: appBaseURL,
		From: notificationsFrom, ReplyTo: supportReplyTo,
	}
	payoutOutboxWorker := payments.Worker{
		Sender: mailgunSender, Now: time.Now, AppBaseURL: appBaseURL,
		From: notificationsFrom, ReplyTo: supportReplyTo,
	}
	paymentOutboxWorker := payments.PaymentReceivedWorker{
		Sender: mailgunSender, Now: time.Now, AppBaseURL: appBaseURL,
		From: notificationsFrom, ReplyTo: supportReplyTo,
	}
	sessionNoticeOutboxWorker := sessionnotice.Worker{
		Sender: mailgunSender, Now: time.Now,
		From: notificationsFrom, ReplyTo: supportReplyTo,
	}
	staffInviteOutboxWorker := staffinvite.Worker{
		Sender: mailgunSender, Now: time.Now, AppBaseURL: appBaseURL,
		From: notificationsFrom, ReplyTo: supportReplyTo,
	}
	offerOutboxWorker := offer.Worker{
		Sender: mailgunSender, Now: time.Now, AppBaseURL: appBaseURL,
		From: notificationsFrom, ReplyTo: supportReplyTo,
	}
	engagementRequestOutboxWorker := engagementrequest.Worker{
		Sender: mailgunSender, Now: time.Now, AppBaseURL: appBaseURL,
		From: notificationsFrom, ReplyTo: supportReplyTo,
	}
	// #613: verification/reset mail resolves its recipient live via the
	// Admin SDK (verifier also satisfies authn.AccountManager) rather
	// than joining `staff`, per authmail's own package doc.
	staffTokenMailOutboxWorker := authmail.TokenMailWorker{
		Sender: mailgunSender, Accounts: verifier, Now: time.Now, AppBaseURL: appBaseURL,
		From: notificationsFrom, ReplyTo: supportReplyTo,
	}
	staffEmailChangeOutboxWorker := authmail.EmailChangeWorker{
		Sender: mailgunSender, Now: time.Now,
		From: notificationsFrom, ReplyTo: supportReplyTo,
	}

	// ADR-0013: one shared queue nudging all eight outbox process-*
	// endpoints, reusing NOTIFICATION_WORKER_SECRET rather than a second
	// credential. NOTIFICATION_TASKS_QUEUE is unset in local dev, CI's
	// boot smoke test, and the e2e stack (see docs/environment.md) --
	// none of those have GCP credentials for a real *cloudtasks.Client,
	// so this only constructs one when a queue is actually configured,
	// falling back to NoOpEnqueuer otherwise. Every outbox still gets
	// Cloud Scheduler's cadence regardless of which Enqueuer is wired up.
	var nudgeEnqueuer tasknudge.Enqueuer = tasknudge.NoOpEnqueuer{}
	if queue := os.Getenv("NOTIFICATION_TASKS_QUEUE"); queue != "" {
		// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
		cloudTasksClient, err := cloudtasks.NewClient(context.Background())
		if err != nil {
			// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
			log.Fatalf("init Cloud Tasks client: %v", err)
		}
		// coverage:ignore reason: constructs the real Cloud-Tasks-backed enqueuer, not exercised by unit tests
		nudgeEnqueuer = tasknudge.NewCloudTasksEnqueuer(cloudTasksClient, queue, os.Getenv("NOTIFICATION_TASKS_TARGET_BASE_URL"), os.Getenv("NOTIFICATION_WORKER_SECRET"))
	}

	// #443. The HTTP client is shared by both and is deliberately
	// short-timeout: a probe is a CDN fetch of a static file, and a
	// dispatch is one small POST -- neither has any business waiting
	// long enough to hold the sweep open.
	siteHTTP := &http.Client{Timeout: 10 * time.Second}
	siteBuildWorker := sitebuild.Worker{
		Dispatcher: sitebuild.GitHubDispatcher{
			Client: siteHTTP,
			// Trimmed, because this arrives from Secret Manager and a
			// secret written with a here-string or an editor carries a
			// trailing newline nobody can see. Go refuses a header value
			// containing one outright -- so the deploy would stop firing
			// on a credential that is otherwise perfectly good, and the
			// only symptom would be Practice pages that never leave
			// pending. Cost of trimming: nothing. A token has no
			// meaningful leading or trailing whitespace.
			Token: strings.TrimSpace(os.Getenv("GITHUB_DISPATCH_TOKEN")),
		},
		Now: time.Now,
	}
	pageVerifier := sitebuild.Verifier{
		// The same constant #442 hands Stripe, so the address probed is
		// by construction the address Stripe was told about.
		Prober: sitebuild.HTTPProber{Client: siteHTTP, BaseURL: website.SiteBaseURL},
		Now:    time.Now,
	}

	handler, _, _ := routes(Deps{
		Verifier:       verifier,
		AccountManager: verifier,
		DB:             db,
		Store:          store,
		Pusher:         pusher,

		StripeClient:        stripeClient,
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),

		PaymentsClient:               paymentsClient,
		PaymentsWebhookSecret:        os.Getenv("STRIPE_CONNECT_WEBHOOK_SECRET"),
		PaymentsAccountWebhookSecret: os.Getenv("STRIPE_ACCOUNT_WEBHOOK_SECRET"),

		MailgunWebhookSigningKey: os.Getenv("MAILGUN_WEBHOOK_SIGNING_KEY"),
		WorkerSecret:             os.Getenv("NOTIFICATION_WORKER_SECRET"),

		PortalInviteWorker:      outboxWorker,
		LowCreditWorker:         lowCreditOutboxWorker,
		PayoutWorker:            payoutOutboxWorker,
		PaymentReceivedWorker:   paymentOutboxWorker,
		SessionNoticeWorker:     sessionNoticeOutboxWorker,
		StaffInviteWorker:       staffInviteOutboxWorker,
		OfferWorker:             offerOutboxWorker,
		EngagementRequestWorker: engagementRequestOutboxWorker,
		SiteBuildWorker:         siteBuildWorker,
		PageVerifier:            pageVerifier,
		StaffTokenMailWorker:    staffTokenMailOutboxWorker,
		StaffEmailChangeWorker:  staffEmailChangeOutboxWorker,

		NudgeEnqueuer:   nudgeEnqueuer,
		ExpectedOrigins: resolveExpectedOrigins(),
	})
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
