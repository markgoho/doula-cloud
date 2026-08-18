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
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/pushsub"
	"doula-cloud/api/internal/session"
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
func routes(verifier authn.Verifier, db *sql.DB, store objectstore.ObjectStore, pusher push.Pusher, stripeClient billing.StripeClient, stripeWebhookSecret string, paymentsClient payments.Client, paymentsWebhookSecret string, expectedOrigins []string) http.Handler {
	mux := http.NewServeMux()
	// Under /api like every other route: Firebase Hosting rewrites /api/** to
	// this service with the path unchanged, so a bare /hello would be
	// unreachable from the browser. CI's two smoke tests curl this same path
	// against the container and against the raw Cloud Run URL.
	mux.HandleFunc("GET /api/hello", helloHandler)
	mux.Handle("POST /api/session", session.CreateHandler(verifier))
	mux.Handle("DELETE /api/session", session.EndHandler())
	mux.Handle("POST /api/staff/signup", staffauth.SignupHandler(verifier, db))
	mux.Handle("GET /api/staff/session", staffauth.SessionHandler(verifier, db))
	mux.Handle("POST /api/staff/accept-invite", staffauth.AcceptInviteHandler(verifier, db))
	mux.Handle("GET /api/practices/{practiceId}/session",
		staffauth.Middleware(verifier, db)(http.HandlerFunc(practiceSessionHandler)))
	mux.Handle("POST /api/practices/{practiceId}/invitations",
		staffauth.Middleware(verifier, db)(idempotency.Wrap(staffauth.InviteHandler())))
	mux.Handle("PATCH /api/practices/{practiceId}/staff/{staffId}/roles",
		staffauth.Middleware(verifier, db)(staffauth.AssignRolesHandler()))
	mux.Handle("GET /api/practices/{practiceId}/billing",
		staffauth.Middleware(verifier, db)(billing.GetBalanceHandler()))
	mux.Handle("POST /api/practices/{practiceId}/billing/purchases",
		staffauth.Middleware(verifier, db)(billing.PostPurchaseHandler(stripeClient)))
	mux.Handle("POST /api/stripe/webhook", billing.PostPurchaseWebhookHandler(db, stripeWebhookSecret))
	mux.Handle("POST /api/practices/{practiceId}/payments/connect",
		staffauth.Middleware(verifier, db)(payments.PostConnectHandler(paymentsClient)))
	mux.Handle("GET /api/practices/{practiceId}/payments/connect",
		staffauth.Middleware(verifier, db)(payments.GetConnectStatusHandler(paymentsClient)))
	mux.Handle("POST /api/stripe/connect-webhook", payments.PostConnectWebhookHandler(db, paymentsClient, paymentsWebhookSecret))
	mux.Handle("GET /api/practices/{practiceId}/clients",
		staffauth.Middleware(verifier, db)(engagement.ListHandler()))
	mux.Handle("POST /api/practices/{practiceId}/clients",
		staffauth.Middleware(verifier, db)(idempotency.Wrap(engagement.CreateHandler())))
	mux.Handle("GET /api/practices/{practiceId}/engagements/{engagementId}",
		staffauth.Middleware(verifier, db)(engagement.DetailHandler()))
	mux.Handle("GET /api/practices/{practiceId}/engagements/{engagementId}/visits",
		staffauth.Middleware(verifier, db)(visit.ListHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/visits",
		staffauth.Middleware(verifier, db)(visit.CreateHandler()))
	mux.Handle("PATCH /api/practices/{practiceId}/engagements/{engagementId}/visits/{visitId}",
		staffauth.Middleware(verifier, db)(visit.ReassignHandler()))
	mux.Handle("GET /api/practices/{practiceId}/engagements/{engagementId}/messages",
		staffauth.Middleware(verifier, db)(message.ListHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/messages",
		staffauth.Middleware(verifier, db)(idempotency.Wrap(message.CreateHandler(store, pusher))))
	mux.Handle("GET /api/practices/{practiceId}/engagements/{engagementId}/messages/{messageId}/attachment",
		staffauth.Middleware(verifier, db)(message.AttachmentHandler(store)))
	mux.Handle("GET /api/practices/{practiceId}/plan-templates/{planType}",
		staffauth.Middleware(verifier, db)(plans.GetTemplateHandler()))
	mux.Handle("PUT /api/practices/{practiceId}/plan-templates/{planType}",
		staffauth.Middleware(verifier, db)(plans.PutTemplateHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		staffauth.Middleware(verifier, db)(plans.PostInstanceHandler()))
	mux.Handle("GET /api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		staffauth.Middleware(verifier, db)(plans.GetInstanceHandler()))
	mux.Handle("PUT /api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		staffauth.Middleware(verifier, db)(plans.PutInstanceHandler()))
	mux.Handle("GET /api/practices/{practiceId}/contract-template",
		staffauth.Middleware(verifier, db)(contracts.GetTemplateHandler()))
	mux.Handle("PUT /api/practices/{practiceId}/contract-template",
		staffauth.Middleware(verifier, db)(contracts.PutTemplateHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/contract",
		staffauth.Middleware(verifier, db)(contracts.PostContractHandler()))
	mux.Handle("GET /api/practices/{practiceId}/engagements/{engagementId}/contract",
		staffauth.Middleware(verifier, db)(contracts.GetContractHandler()))
	mux.Handle("PUT /api/practices/{practiceId}/engagements/{engagementId}/contract",
		staffauth.Middleware(verifier, db)(contracts.PutContractHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/send",
		staffauth.Middleware(verifier, db)(contracts.PostSendContractHandler(pusher)))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/void",
		staffauth.Middleware(verifier, db)(contracts.PostVoidContractHandler()))
	mux.Handle("GET /api/practices/{practiceId}/engagements/{engagementId}/contract/pdf",
		staffauth.Middleware(verifier, db)(contracts.GetSignedContractPDFHandler(store)))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/invoices",
		staffauth.Middleware(verifier, db)(payments.PostInvoiceHandler(paymentsClient)))
	mux.Handle("GET /api/practices/{practiceId}/engagements/{engagementId}/contract/invoices",
		staffauth.Middleware(verifier, db)(payments.GetInvoicesHandler()))
	mux.Handle("POST /api/practices/{practiceId}/push-subscriptions",
		staffauth.Middleware(verifier, db)(pushsub.RegisterHandler()))
	mux.Handle("DELETE /api/practices/{practiceId}/push-subscriptions",
		staffauth.Middleware(verifier, db)(pushsub.UnregisterHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/portal-invite",
		staffauth.Middleware(verifier, db)(idempotency.Wrap(portalinvite.InviteHandler())))
	mux.Handle("POST /api/portal/accept-invite", portalinvite.AcceptInviteHandler(verifier, db))
	mux.Handle("GET /api/portal/session", clientauth.SessionHandler(verifier, db))
	mux.Handle("GET /api/portal/engagements/{engagementId}",
		clientauth.Middleware(verifier, db)(portal.DetailHandler()))
	mux.Handle("GET /api/portal/engagements/{engagementId}/birth-plan",
		clientauth.Middleware(verifier, db)(plans.ClientGetBirthPlanHandler()))
	mux.Handle("GET /api/portal/engagements/{engagementId}/contract",
		clientauth.Middleware(verifier, db)(contracts.ClientGetContractHandler()))
	mux.Handle("POST /api/portal/engagements/{engagementId}/contract/sign",
		clientauth.Middleware(verifier, db)(contracts.ClientPostSignContractHandler(store)))
	mux.Handle("GET /api/portal/engagements/{engagementId}/contract/pdf",
		clientauth.Middleware(verifier, db)(contracts.ClientGetSignedContractPDFHandler(store)))
	mux.Handle("GET /api/portal/engagements/{engagementId}/messages",
		clientauth.Middleware(verifier, db)(message.ClientListHandler()))
	mux.Handle("POST /api/portal/engagements/{engagementId}/messages",
		clientauth.Middleware(verifier, db)(message.ClientCreateHandler(store, pusher)))
	mux.Handle("GET /api/portal/engagements/{engagementId}/messages/{messageId}/attachment",
		clientauth.Middleware(verifier, db)(message.ClientAttachmentHandler(store)))
	mux.Handle("POST /api/portal/engagements/{engagementId}/push-subscriptions",
		clientauth.Middleware(verifier, db)(pushsub.ClientRegisterHandler()))
	mux.Handle("DELETE /api/portal/engagements/{engagementId}/push-subscriptions",
		clientauth.Middleware(verifier, db)(pushsub.ClientUnregisterHandler()))
	return csrf.Wrap(expectedOrigins, mux)
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

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           routes(verifier, db, store, pusher, stripeClient, os.Getenv("STRIPE_WEBHOOK_SECRET"), paymentsClient, os.Getenv("STRIPE_CONNECT_WEBHOOK_SECRET"), resolveExpectedOrigins()),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("listening on port %s", port)
	// coverage:ignore reason: listener startup, not exercised by unit tests
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
