package main

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/authmail"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/csrf"
	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/mfarecoverymail"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/sitebuild"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/staffinvite"
	"doula-cloud/api/internal/tasknudge"
)

// Deps is everything the route table needs to build itself.
//
// A struct rather than a parameter list, and that is the whole point of
// it (#482). routes() used to take twenty-three positional arguments, so
// adding one dependency meant editing every call site -- nine of them
// across main_test.go and gate_guardrail_test.go, in files that had
// nothing to do with the feature adding it. Two sessions each adding a
// worker then conflicted in three files on lines neither cared about.
//
// As a struct, a new dependency is a new field: nothing that does not
// mention it has to change, and a test that does not exercise a worker
// simply leaves it at its zero value.
//
// Flat rather than nested. Grouping the outbox workers under a Workers
// field was considered and dropped: it would say "these are the workers"
// in the type, which nothing needs to be told, at the cost of a second
// name at every use.
//
// The workers themselves stay flat fields here for exactly that reason.
// What did move is the *route* half of an outbox -- its path, its
// Postgres door, and whether ADR-0013 nudges it -- which now lives in
// outboxes.go as one list. That is not the nesting rejected above: a
// worker is still named directly, the list adds no second name at any use
// site, and it replaces six restatements of an outbox's identity that
// previously had to agree with each other by hand.
type Deps struct {
	// Verifier checks an Identity Platform token. Tests substitute
	// authntest.Verifier.
	Verifier authn.Verifier
	// AccountManager is the Admin SDK surface #613 widens ADR-0004 into --
	// reading and writing the account records Identity Platform still
	// owns as credential store. Tests substitute authntest.FakeAccountManager.
	AccountManager authn.AccountManager
	DB             *sql.DB
	Store          objectstore.ObjectStore
	Pusher         push.Pusher

	StripeClient        billing.StripeClient
	StripeWebhookSecret string

	PaymentsClient               payments.Client
	PaymentsWebhookSecret        string
	PaymentsAccountWebhookSecret string

	MailgunWebhookSigningKey string

	// WorkerSecret is NOTIFICATION_WORKER_SECRET, which every
	// process-* endpoint and #443's two site endpoints check the
	// X-Internal-Secret header against.
	WorkerSecret string

	// The outbox workers, one per table (ADR-0010). Named for what each
	// one processes rather than for being an outbox worker, which all of
	// them are.
	PortalInviteWorker      portalinvite.Worker
	LowCreditWorker         billing.Worker
	PayoutWorker            payments.Worker
	PaymentReceivedWorker   payments.PaymentReceivedWorker
	SessionNoticeWorker     sessionnotice.Worker
	StaffInviteWorker       staffinvite.Worker
	OfferWorker             offer.Worker
	EngagementRequestWorker engagementrequest.Worker
	SiteBuildWorker         sitebuild.Worker
	PageVerifier            sitebuild.Verifier
	StaffTokenMailWorker    authmail.TokenMailWorker
	StaffEmailChangeWorker  authmail.EmailChangeWorker
	MFARecoveryMailWorker   mfarecoverymail.Worker
	// ClientErasureWorker is the one worker here that sends no mail: it
	// carries out the Stripe and Identity Platform half of a Client
	// erasure (#394, ADR-0027).
	ClientErasureWorker client.ErasureWorker

	NudgeEnqueuer   tasknudge.Enqueuer
	ExpectedOrigins []string
}

// routes builds the BFF's route table.
//
// The registrations themselves live in routes_session.go,
// routes_practice.go, routes_portal.go, routes_internal.go and
// routes_webhook.go -- one file per area, each carrying only the imports
// its own routes need. That split is not cosmetic: a single file meant
// every feature ticket appended to the same import block and the same
// route list, so unrelated work collided by construction (#482).
//
// The whole mux is wrapped in csrf.Wrap, rather than only the
// authenticated routes: the bootstrap endpoints (signup, invitation
// acceptance) are state-changing too, and both they and the Stripe
// webhook routes rely on the same "no Origin header, no rejection" rule
// -- there is no separate carve-out for either.
//
// The second return value is GatedRouter's registry of every GET it
// mounted -- main() never looks at it, but the guardrail tests in
// gate_guardrail_test.go walk it, and cross-check it against the route
// files' own source for a route that bypassed the gate entirely. The
// third is idempotency.Router's registry of every mutating route
// registerPracticeRoutes declared -- idempotency_guardrail_test.go's
// mirror of the same check, scoped to that file's routes.
func routes(d Deps) (http.Handler, []staffauth.GatedRoute, []idempotency.Route) {
	mux := http.NewServeMux()
	// GatedRouter (staffauth/gate.go) is the only door for a GET behind
	// staffauth.Middleware: Get panics at startup if a route has no role
	// declaration, so a forgotten one fails the binary rather than
	// silently opening to every Staff member (#231, #315). AnyStaff
	// declares an endpoint open to every role on purpose; a bare role
	// list names exactly who ADR-0008's read table admits.
	// The mux is handed to g here and named nowhere else. That is what
	// makes the gate the only door: a route file has no mux to reach for,
	// so bypassing the registry is a compile error rather than something a
	// test has to find by regexing this package's own source.
	g := staffauth.NewGatedRouter(mux, d.DB)
	// idempotency.Router is registerPracticeRoutes' mirror of g for its
	// mutating routes: Replayable or Exempt is the only way to register
	// one, and Exempt refuses to register without a reason.
	ir := idempotency.NewRouter(g)

	// Every file takes g and nothing takes the mux. The last three mount
	// outside staffauth.Middleware entirely -- a Client session, a
	// scheduler, a webhook signature -- so there is no Membership for a
	// role declaration to be about; their reads say so at the mount
	// through g.OpenGet, and their writes go through g.Write, which asks
	// for no declaration but does put them in the registry. Only
	// registerPracticeRoutes takes ir: the review that added it (2026)
	// scoped the idempotency-stance requirement to that file's mutating
	// routes.
	registerSessionRoutes(g, d)
	registerPracticeRoutes(g, ir, d)
	registerPortalRoutes(g, d)
	registerInternalRoutes(g, d)
	registerWebhookRoutes(g, d)

	return csrf.Wrap(d.ExpectedOrigins, mux), g.Routes(), ir.Routes()
}
