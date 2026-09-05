# Environment variables

Every variable `api/main.go` reads, what it is for, and what it holds in
each of the three places the BFF runs. The Hugo site's own build reads two
of its own — see [The Hugo site's build](#the-hugo-sites-build). `app/.env.example` is the local
template; the Stripe half of it is filled in by hand, from the Sandbox
keys and the `stripe listen` secret (see [Stripe](#stripe)).

## Where a value lives

| Place | Mechanism |
| --- | --- |
| Local (`bun run dev:full`) | `app/.env.local`, auto-loaded by bun, spread into the BFF's own environment by `app/e2e/stack.ts` |
| CI (`bun run test:e2e`) | `app/e2e/stack.ts` only. No Stripe values: the e2e run stays on the fakes — see [CI stays on the fakes](#ci-stays-on-the-fakes) |
| Deployed (Cloud Run `doula-api`) | Secret Manager for credentials, plain env vars for the rest |

A **simulation run** ([#763](https://github.com/markgoho/doula-cloud/issues/763)) is the local row with three additions, and it does not add a variable: it keeps the Postgres volume instead of destroying it, installs a `sim.now()` shim before goose runs, and drains the `process-*` endpoints itself. `APP_BASE_URL` keeps its local default and `EXPECTED_ORIGINS` is left alone, because `scripts/stripe-listen.sh` is how Stripe reaches a run and no tunnel is involved. See [Where a run runs](simulation/environment.md).

`app/.gitignore` and the repo root `.gitignore` both exclude `.env*`
except `.env.example`. A Sandbox key is still a key.

## The variables

| Variable | Local | CI | Deployed |
| --- | --- | --- | --- |
| `DATABASE_URL` | `app_e2e` role on the compose Postgres | same | Secret Manager `doula-cloud-pg-app-runtime-dsn`, set in `ci.yml` |
| `EXPECTED_ORIGINS` | `http://localhost:5173` | `http://localhost:4173` | `https://doula-cloud-app.web.app`, set in `ci.yml` |
| `APP_BASE_URL` | defaults to `EXPECTED_ORIGINS` | same | `https://doula-cloud-app.web.app` |
| `STRIPE_API_KEY` | `sk_test_…` in `.env.local` | unset | Secret Manager `doula-cloud-stripe-api-key` |
| `STRIPE_CREDIT_PRICE_ID` | `price_…` in `.env.local` | unset | plain env var |
| `STRIPE_WEBHOOK_SECRET` | the `stripe listen` secret | unset | Secret Manager `doula-cloud-stripe-webhook-secret` |
| `STRIPE_CONNECT_WEBHOOK_SECRET` | the same `stripe listen` secret | unset | Secret Manager `doula-cloud-stripe-connect-webhook-secret` |
| `STRIPE_ACCOUNT_WEBHOOK_SECRET` | the same `stripe listen` secret | unset | Secret Manager `doula-cloud-stripe-account-webhook-secret` |
| `GCP_PROJECT_ID` | `doula-cloud` | same | ambient |
| `STORAGE_EMULATOR_HOST` | the compose `gcs` service | same | unset (real GCS) |
| `GCS_ATTACHMENTS_BUCKET` | `doula-cloud-e2e-attachments` | same | `doula-cloud-attachments`, a real bucket in `us-central1`, uniform access, public access prevention enforced |
| `VAPID_PUBLIC_KEY` / `VAPID_SUBSCRIBER` | throwaway keypair in `stack.ts` | same | `VAPID_PUBLIC_KEY` plain env var, `VAPID_SUBSCRIBER=mailto:admin@doula.cloud` (not `mg.doula.cloud`: that domain has no inbound MX, see [Mailgun](#mailgun)) |
| `VAPID_PRIVATE_KEY` | throwaway keypair in `stack.ts` | same | Secret Manager `doula-cloud-vapid-private-key` |
| `MAILGUN_API_KEY` | `e2e-mailgun-key`, set by `stack.ts` -- the sandbox mailbox does not check it | same | Secret Manager `doula-cloud-mailgun-api-key` |
| `MAILGUN_DOMAIN` | `sim.doula.cloud`, set by `stack.ts` -- the domain every persona's and every fixture's address sits under | same | `mg.doula.cloud`, plain env var |
| `MAILGUN_API_BASE` | the sandbox mailbox's origin, set by `stack.ts` (#764) | same | unset -- `mail.NewMailgunSender` keeps its `https://api.mailgun.net` default |
| `NOTIFICATION_WORKER_SECRET` | `e2e-worker-secret`, set by `stack.ts`; matched against the `X-Internal-Secret` header on calls to any of the thirteen `/api/internal/**/process-*` endpoints and to `/api/internal/outboxes/drain` | same | Secret Manager `doula-cloud-notification-worker-secret`, also set as the `process-outbox-drain` Scheduler job's header |
| `MAILGUN_WEBHOOK_SIGNING_KEY` | `e2e-mailgun-signing-key`, set by `stack.ts` on both the BFF and the mailbox, which signs its bounce and complaint webhooks with it | same | Secret Manager `doula-cloud-mailgun-webhook-signing-key`, set (#743) -- Mailgun's `permanent_fail` and `complained` webhooks point at the deployed endpoint |
| `NOTIFICATION_TASKS_QUEUE` | unset (no real `tasknudge.CloudTasksEnqueuer` is constructed in `routes()` tests, which inject `tasknudge.FakeEnqueuer` instead) | unset | the Cloud Tasks queue's full resource name, `projects/doula-cloud/locations/us-central1/queues/doula-cloud-notification-nudge`, plain env var |
| `NOTIFICATION_TASKS_TARGET_BASE_URL` | unset | unset | the same raw Cloud Run URL `gcloud run services describe` reports (see [Deployed webhook endpoints](#deployed-webhook-endpoints)), plain env var |
| `GITHUB_DISPATCH_TOKEN` | unset (nothing local fires a real deploy) | unset | Secret Manager `doula-cloud-github-dispatch-token`, a fine-grained personal access token scoped to `markgoho/doula-cloud` with **Contents: write** |

## The Hugo site's build

`bun run build` runs `scripts/sync-practice-pages.ts` before `hugo`, which writes a page into `hugo/content/p/<slug>/` for every Practice that published one (#441). It reads two variables, and neither belongs to the BFF.

| Variable | Local | PR preview | Merge deploy |
| --- | --- | --- | --- |
| `SYNC_PRACTICE_PAGES` | unset | unset | `required`, set in `firebase-hosting-merge.yml` |
| `DATABASE_URL` | unset | unset | built from Secret Manager `doula-cloud-pg-site-builder-dsn`, dialled through the Cloud SQL Auth Proxy on `127.0.0.1:5432` |

**Unset means "touch nothing", not "connect if you can".** The script prunes `hugo/content/p` before it writes, because that is the only way a Practice who switches back to her own website loses her page. So an unreachable database and an empty result set would produce the same output — every live page deleted, against a Stripe review #382 established is ongoing. `SYNC_PRACTICE_PAGES=required` makes an unreachable database fail the build instead, and the workflow's build/deploy split means a failed build uploads no artifact and the live site stays exactly as it was.

The DSN belongs to `site_builder_login`, a Cloud SQL user created for this and granted `site_builder` — 00046's role, which holds `SELECT` on five tables and no write grant at all. The build job's credential can read what is about to be published and change nothing. The name mirrors `app_runtime_login`, which is granted `app_runtime` the same way.

### `APP_BASE_URL` is not `EXPECTED_ORIGINS`

They hold the same string in all three environments today, and they are
still two variables. `EXPECTED_ORIGINS` is a list the CSRF check compares
an inbound `Origin` header against. `APP_BASE_URL` is one string the BFF
concatenates redirect targets onto: Checkout returns to
`/practices/{id}/billing?checkout=success|cancelled`, and the Connect
Account Link returns to `/practices/{id}/settings/payments?connect=return|refresh`.
A tunnelled local walk needs `APP_BASE_URL` overridden and
`EXPECTED_ORIGINS` left alone.

## Mailgun

Provisioned for #218 (map #213). One account, one API key, no per-domain
Sending Key: Mailgun's API key is account-wide, so the same value works
against both the sandbox domain and the verified one — only `MAILGUN_DOMAIN`
changes between Local and Deployed.

**No local stack reaches Mailgun any more.** `app/e2e/stack.ts` sets `MAILGUN_API_BASE` to the sandbox mailbox (`app/e2e/mailbox.ts`, #764) on every BFF it starts, which is both the Playwright e2e run and `bun run dev:full`, and it sets `MAILGUN_DOMAIN`/`MAILGUN_API_KEY` explicitly alongside it rather than inheriting them. That is deliberate: `.env.local` holds a real account-level key, `bun` loads it before `stack.ts` runs, and without those three lines a local run would post real mail to real Mailgun. The mailbox answers the same `POST /v3/<domain>/messages` endpoint, holds what it is sent, and serves it back as JSON and as a browsable inbox. A walk that genuinely needs a real Mailgun send now runs the BFF by hand with the real values in its environment; nothing that goes through `startStack` will do it for you.

`mg.doula.cloud` is verified: SPF (TXT) and both DKIM records (CNAME, under
Mailgun's automatic sender security, which rotates the key every 120 days)
are live in Squarespace DNS. DMARC is a plain self-published TXT at
`_dmarc.mg.doula.cloud` (`v=DMARC1; p=none;`) rather than Mailgun's
suggested record, whose `rua`/`ruf` route reports to Red Sift's
`inbox.ondmarc.com` — declined to avoid the third-party data share.
Receiving records (MX) and tracking records (open/click CNAME) were not
installed: inbound email is out of scope for the whole map, and tracking
was left undecided rather than defaulted on.

Proved end to end: a real send from `notifications@mg.doula.cloud` landed
in a real Gmail inbox, not spam, with `dkim=pass`, `spf=pass`,
`dmarc=pass (p=NONE)`.

The commands that produced the current state, so it can be rebuilt:

```
gcloud secrets create doula-cloud-mailgun-api-key --replication-policy=automatic --data-file=-
gcloud secrets add-iam-policy-binding doula-cloud-mailgun-api-key \
  --member serviceAccount:850855848778-compute@developer.gserviceaccount.com \
  --role roles/secretmanager.secretAccessor
```

#219 lands the code that reads both variables (the outbox worker, the
Client portal invite's real sender) plus a third,
`NOTIFICATION_WORKER_SECRET`, guarding the Cloud-Scheduler-triggered
`/api/internal/notifications/process-outbox` endpoint. #341 ran the
deploy step: `MAILGUN_API_KEY` and `NOTIFICATION_WORKER_SECRET` are set
as Cloud Run secrets on `doula-api`, `MAILGUN_DOMAIN=mg.doula.cloud` is
a plain env var, and a Cloud Scheduler job (`process-portal-invite-outbox`, `us-central1`, every 5 minutes) drove the endpoint on a fixed cadence. That job was deleted by #481, which replaced one job per outbox with a single `process-outbox-drain` — see [The outbox backstop](#the-outbox-backstop-one-drain-job). Proved
end to end: a real Client portal invite, added and sent through the
deployed app, arrived in a real inbox via the deployed pipeline (not a
local one-off).

While provisioning this, Identity Platform's Email/Password sign-in
provider turned out to be disabled in the production project (`GET
.../admin/v2/projects/doula-cloud/config` had no `signIn.email` block)
-- unrelated to Mailgun, but it meant nobody could sign up or log in to
the deployed app at all. Enabled via `PATCH
.../admin/v2/projects/doula-cloud/config?updateMask=signIn.email` with
`{"signIn":{"email":{"enabled":true,"passwordRequired":true}}}`.

#342 (the out-of-Credits Platform Notification) reuses `NOTIFICATION_WORKER_SECRET` for a second endpoint, `/api/internal/notifications/process-low-credit-outbox` -- same secret, same header, but a separate outbox table (ADR-0010).

#343 (the payout-account-incomplete Platform Notification) reuses `NOTIFICATION_WORKER_SECRET` for a third endpoint, `/api/internal/notifications/process-payout-outbox` -- same secret, same header, its own outbox table (`payout_outbox`).

#344 (the payment-arrived Platform Notification) reuses `NOTIFICATION_WORKER_SECRET` for a fourth endpoint, `/api/internal/notifications/process-payment-outbox` -- same secret, same header, its own outbox table (`payment_received_outbox`).

#345 (the new-sign-in/session-revoked Platform Notifications) reuses `NOTIFICATION_WORKER_SECRET` for a fifth endpoint, `/api/internal/notifications/process-session-notice-outbox` -- same secret, same header, one outbox table (`session_notice_outbox`) shared by both notices since #345 bundled them into a single ticket.

#339 (the Staff invitation Notification, RA-G1) reuses `NOTIFICATION_WORKER_SECRET` for a sixth endpoint, `/api/internal/notifications/process-staff-invite-outbox` -- same secret, same header, its own outbox table (`staff_invite_outbox`). `staffauth.InviteHandler` (#316) is its write site.

#317 (the Offer Notification, ADR-0008) reuses `NOTIFICATION_WORKER_SECRET` for a seventh endpoint, `/api/internal/notifications/process-offer-outbox` -- same secret, same header, its own outbox table (`engagement_offer_outbox`). Its write site is `offer.CreateHandler`'s email-target path, which mails one link that both joins the Practice and opens the Offer, plus the six-digit access code the pre-account read asks for. 

#398 (the Engagement Request Notification, ADR-0017) reuses `NOTIFICATION_WORKER_SECRET` for an eighth endpoint, `/api/internal/notifications/process-engagement-request-outbox` -- same secret, same header, its own outbox table (`engagement_request_outbox`, one row per Owner/Admin recipient rather than one per Request). Its write site is `engagementrequest.RequestHandler`, whenever the requester does not already hold approval authority herself. 

#394 (Client erasure, ADR-0027) reuses `NOTIFICATION_WORKER_SECRET` for a ninth endpoint, `/api/internal/clients/process-erasure-outbox` -- same secret, same header, its own outbox table (`client_erasure_outbox`). It is the first `process-*` endpoint that mails nobody, which is why it sits under `/api/internal/clients` rather than `/api/internal/notifications`: it deletes a Stripe Customer, runs its Redaction Job once Stripe's 90-day floor has passed, and deletes an Identity Platform account. Its write site is `client.EraseHandler`. Note that the redaction leg needs one thing no environment variable can supply: Stripe's Redaction Jobs API is in public preview and is **not enabled on the Doula Cloud account** -- `POST /v1/privacy/redaction_jobs` answers "Unrecognized request URL" today, verified against the Sandbox. Until Stripe enables it, that act dead-letters with the API's own error while every other leg of an erasure completes; enabling it is a request to Stripe, not a code change.

#340 (the Mailgun bounce/complaint webhook, ADR-0010) adds a sixth variable, `MAILGUN_WEBHOOK_SIGNING_KEY` -- Mailgun's HTTP webhook signing key, a separate value from `MAILGUN_API_KEY`, verifying `POST /api/mailgun/webhook`'s HMAC-SHA256 signature rather than a shared-secret header. **Provisioned on #743**, so it is no longer unset on the deployed service. The key does not need Mailgun's dashboard: it comes back from the account-level API, `GET https://api.mailgun.net/v5/accounts/http_signing_key` with the same `api:<MAILGUN_API_KEY>` basic auth every other call uses. It was stored as Secret Manager `doula-cloud-mailgun-webhook-signing-key`, granted to `850855848778-compute@developer.gserviceaccount.com`, and attached to `doula-api` out of band the same way the Stripe and VAPID values are -- `gcloud run services update doula-api --region us-central1 --update-secrets MAILGUN_WEBHOOK_SIGNING_KEY=doula-cloud-mailgun-webhook-signing-key:latest`. That survives every later trunk deploy, because `deploy-cloudrun`'s `secrets:` input merges rather than replaces (see the comment on the deploy step in `ci.yml`).

Both webhooks are registered against `mg.doula.cloud` through Mailgun's API, not its dashboard, and point at the canonical Cloud Run URL rather than the Firebase Hosting rewrite -- the endpoint is signature-verified and carries no cookie, so it has no reason to depend on the `/api/**` rewrite:

```bash
for id in permanent_fail complained; do
  curl -s --user "api:$MAILGUN_API_KEY" -X POST \
    https://api.mailgun.net/v3/domains/mg.doula.cloud/webhooks \
    -F id=$id \
    -F url=https://doula-api-850855848778.us-central1.run.app/api/mailgun/webhook
done
```

Proved end to end on #743: a real message to `bounce-743@bounce-test.doula.cloud` (an NXDOMAIN subdomain of our own domain, so a genuine first-time permanent failure with no ISP reputation cost -- Mailgun answered `498 No MX for bounce-test.doula.cloud`, reason `generic`, not a `suppress-*` reason) produced Mailgun event `ebsa_POBSEipJK4ci4LETg`, which the deployed endpoint turned into an `email_suppressions` row with cause `bounce`. A Practice signed up on the deployed app with that same address then had its verification email dead-lettered by `mailsuppress.Sender` -- `mail: address is suppressed: bounce-743@bounce-test.doula.cloud`, with no Mailgun request made -- while two unrelated rows drained in the same worker run reached Mailgun normally.

#348 (ADR-0013's Cloud Tasks nudge) adds `NOTIFICATION_TASKS_QUEUE` and `NOTIFICATION_TASKS_TARGET_BASE_URL`, both consumed only by `tasknudge.NewCloudTasksEnqueuer` at startup. No new secret: the enqueued task's `X-Internal-Secret` header is `NOTIFICATION_WORKER_SECRET`, the same value the thirteen `process-*` endpoints already check. One queue serves all nine outbox types (`main.go` builds one `CloudTasksEnqueuer` and passes it into `routes()` as `nudgeEnqueuer`), provisioned once:

```bash
gcloud tasks queues create doula-cloud-notification-nudge \
  --location=us-central1
gcloud tasks queues add-iam-policy-binding doula-cloud-notification-nudge \
  --location=us-central1 \
  --member=serviceAccount:<doula-api's runtime service account> \
  --role=roles/cloudtasks.enqueuer
gcloud run services update doula-api --region us-central1 \
  --update-env-vars NOTIFICATION_TASKS_QUEUE=projects/doula-cloud/locations/us-central1/queues/doula-cloud-notification-nudge,NOTIFICATION_TASKS_TARGET_BASE_URL=https://doula-api-850855848778.us-central1.run.app
```

`doula-api`'s own runtime service account also needs `roles/cloudtasks.enqueuer` on the queue (the binding above) to call `CreateTask` -- it already reaches Secret Manager and GCS under its existing identity, so no new service account is created for this.

### #443's site rebuild and page probe

Two more endpoints on the same `X-Internal-Secret` shape, under `/api/internal/site` rather than `/notifications` because neither of them notifies anybody. `NOTIFICATION_WORKER_SECRET` again, not a second credential.

`POST /api/internal/site/process-build-outbox` turns queued rebuilds into one `repository_dispatch`, and is the ninth type on ADR-0013's shared Cloud Tasks queue. It is the one type whose nudge is **delayed** -- 90 seconds, `tasknudge.Delay` -- because the worker collapses every pending row into a single deploy and can only collapse rows that have had a moment to gather. The drain job is its durability backstop: a dispatch that fails leaves the rows pending, and the next tick retries.

`POST /api/internal/site/verify-pages` fetches every published page from `doula.cloud` and records whether it resolved. Two callers, and deliberately identical behavior for both: the last step of `firebase-hosting-merge.yml`, which reads the same secret out of Secret Manager over Workload Identity Federation and posts to the raw Cloud Run URL; and a Cloud Scheduler job every fifteen minutes. The sweep is what covers the case the workflow cannot -- a build that fails produces no deploy and no callback at all, so only something that runs anyway can notice a page that never went live.

`GITHUB_DISPATCH_TOKEN` is the one new credential. **Contents: write** is the narrowest permission GitHub's dispatch endpoint accepts, and it is the same level a GitHub App would need, which is why #443 chose the simpler thing. Give it a real expiry rather than "no expiration": a lapsed token is not silent here -- the dispatch fails, the page never leaves `pending`, and the Practice's website settings screen says her page is not confirmed.

**Provisioned.** `verify-practice-pages` runs every fifteen minutes in `us-central1`, carrying `X-Internal-Secret`. #443's own `process-site-build-outbox` job existed too, and was deleted by #481 once the drain covered it -- see [The outbox backstop](#the-outbox-backstop-one-drain-job). The deploy workflow's service account (`github-action-733741680@doula-cloud.iam.gserviceaccount.com`) has been granted `roles/secretmanager.secretAccessor` on `doula-cloud-notification-worker-secret`, which it needs to read the secret its last step posts.

**Still outstanding: the token itself.** It is created by hand at <https://github.com/settings/personal-access-tokens> -- resource owner `markgoho`, repository access *"Only select repositories"* → `doula-cloud`, Repository permissions → **Contents: Read and write**, expiry 366 days. Until it is in Secret Manager and on the service, `process-site-build-outbox` runs green with nothing to do and a real publish would sit at `pending` with the outbox retrying.

```bash
printf %s "<the token>" | gcloud secrets create doula-cloud-github-dispatch-token \
  --project=doula-cloud --data-file=- --replication-policy=automatic
gcloud secrets add-iam-policy-binding doula-cloud-github-dispatch-token \
  --project=doula-cloud \
  --member=serviceAccount:850855848778-compute@developer.gserviceaccount.com \
  --role=roles/secretmanager.secretAccessor
gcloud run services update doula-api --region us-central1 --project=doula-cloud \
  --update-secrets GITHUB_DISPATCH_TOKEN=doula-cloud-github-dispatch-token:latest
```

### The outbox backstop: one drain job

**Which Cloud Scheduler jobs exist.** Two, both in `us-central1`, both carrying `X-Internal-Secret`:

| Job | Cadence | Calls |
| --- | --- | --- |
| `process-outbox-drain` | `*/5 * * * *` | `POST /api/internal/outboxes/drain` |
| `verify-practice-pages` | `*/15 * * * *` | `POST /api/internal/site/verify-pages` |

That is the whole list. No `process-*` endpoint has a job of its own any more, and none needs one: the drain runs every registration in `api/outboxes.go` in turn, each in its own transaction behind its own RLS door, so a fourteenth outbox is backstopped by being registered. Adding an outbox is no longer a console change. See [ADR-0010](adr/0010-notification-email-outbox.md)'s amendment and [#481](https://github.com/markgoho/doula-cloud/issues/481).

The drain answers `200` only when every outbox succeeded, and `500` naming the ones that failed, so a single broken outbox turns the job red rather than hiding inside a green tick. Each failure is also logged as `outbox: drain <path>: <error>`.

**Two outboxes have no nudge at all**, so the drain is the only thing that ever runs them: `process-staff-token-mail-outbox` and `process-staff-email-change-outbox` (#613 accepted ADR-0010's plain delay for both rather than wiring a nudge). Before #481 they had no Scheduler job either, which means they did not run outside a test or a hand invocation.

The three jobs that predated this — `process-portal-invite-outbox`, `process-site-build-outbox`, and #443's `verify-practice-pages` — are down to one: the first two were deleted as redundant once the drain covered them, and `verify-practice-pages` stays because it is not an outbox at all.

## Attachments bucket and VAPID push

#245 provisioned the two variables `ci.yml`'s own comment had wrongly
claimed were already set out of band: `GCS_ATTACHMENTS_BUCKET` (Contract
PDFs and Message attachments, `objectstore.NewGCSStore` in `main.go`) and
the VAPID keypair (`push.NewVAPIDPusher`, Web Push delivery).

Proved end to end, not inferred from the code path: a real Practice, Client,
Engagement and Contract were created against the deployed service, the
Contract was sent and signed through the Client portal, and the signed PDF
read back as `200 application/pdf` from the Owner-only endpoint.

The bucket carries no public access -- every read goes through
`GetSignedContractPDFHandler` and the equivalent attachment handler,
never a direct bucket URL. The runtime service account holds
`roles/storage.objectUser` on the bucket only, not a project-wide role,
matching the per-secret grants below.

The VAPID keypair is a real production pair, generated once with
`webpush.GenerateVAPIDKeys()` from a throwaway `main.go` in `api/` that
piped the private key straight into `gcloud secrets create` and printed
only the public key -- deleted afterward, so the private key never sat
in a file, a shell history, or a transcript longer than that one pipe.
The local/CI throwaway pair in `stack.ts` is deliberately not this one.

The public key also has to reach the browser: `deploy-app`'s "Build app"
step in `ci.yml` sets `VITE_VAPID_PUBLIC_KEY` at build time, read by
`app/src/lib/pushRegistration.ts`. It is hardcoded in `ci.yml` rather
than held as a GitHub secret -- a VAPID public key is meant to travel to
the browser, so there is nothing to protect. Before #245, this variable
was never set at all, so every browser push subscription on the live app
signed against an empty key; `objectstore` and `push` failing closed on
missing values is what kept that from ever reaching a real push service.

The commands that produced the current state, so it can be rebuilt:

```
gcloud storage buckets create gs://doula-cloud-attachments \
  --location=us-central1 \
  --uniform-bucket-level-access \
  --public-access-prevention
gcloud storage buckets add-iam-policy-binding gs://doula-cloud-attachments \
  --member=serviceAccount:850855848778-compute@developer.gserviceaccount.com \
  --role=roles/storage.objectUser
gcloud secrets create doula-cloud-vapid-private-key --replication-policy=automatic --data-file=-
gcloud secrets add-iam-policy-binding doula-cloud-vapid-private-key \
  --member=serviceAccount:850855848778-compute@developer.gserviceaccount.com \
  --role=roles/secretmanager.secretAccessor
gcloud run services update doula-api --region us-central1 \
  --update-env-vars GCS_ATTACHMENTS_BUCKET=doula-cloud-attachments,VAPID_PUBLIC_KEY=<public key>,VAPID_SUBSCRIBER=mailto:admin@doula.cloud \
  --update-secrets VAPID_PRIVATE_KEY=doula-cloud-vapid-private-key:latest
```

## Say Sandbox, not test mode

Stripe renamed it. What #242 and older tickets call test mode, the
dashboard now calls a **Sandbox** (`CONTEXT.md`): a separate environment with its own data and its own
API keys, rather than a toggle over one account. Keys from it still start
`sk_test_`, so nothing the code expects changes.

What does change is where a key comes from. A key copied while the
dashboard's environment switcher is on some other environment is a valid
key for the wrong place, and it fails later as a signature error or a
"no such price" — both of which read like code bugs and are not.

Whether a sandbox carries every Connect capability is not something to
take on trust. Create a throwaway v2 Account over the API and read its
`configuration.merchant.capabilities` back, so a missing capability
surfaces there rather than an hour later in a walk.

## Stripe: two surfaces, one key

**Platform billing.** The Practice pays us. A Stripe Customer plus a
Checkout Session over one flat Price. One event matters:
`checkout.session.completed`. The `credit_ledger` is credited by
`api/internal/billing/webhook.go` and by nothing else — never by the
browser redirect.

**Connect.** The Client pays the Practice. An **Accounts v2** connected
account per Practice carrying the `merchant` configuration, onboarded
through a hosted v2 Account Link, with Invoices raised on-behalf-of using
the `Stripe-Account` header. Three events matter, and they no longer
arrive the same way:

| Event | Kind | Route |
| --- | --- | --- |
| `v2.core.account[configuration.merchant].capability_status_updated` | thin | `/api/stripe/account-webhook` |
| `invoice.paid` | snapshot | `/api/stripe/connect-webhook` |
| `invoice.payment_failed` | snapshot | `/api/stripe/connect-webhook` |

`account.updated` is no longer what we act on. A v2 account **does** still
emit v1 snapshot `account.updated` on the connected account — an earlier
#247 note claimed otherwise, from a check that listed the *platform's*
`/v1/events` where connected-account events never appear. What is true is
that the v1 payload carries the v1 model (three booleans), which cannot
express the four-valued capability statuses the `practices` columns now
hold. The authoritative v2 state comes as a thin event.

One Stripe event destination also carries one `event_payload` —
subscribing a single destination to both a thin and a snapshot event type
is rejected outright. Hence two Connect routes, not one.

All three surfaces share `STRIPE_API_KEY` and nothing else. **Three
endpoints, three secrets**: `STRIPE_WEBHOOK_SECRET`,
`STRIPE_CONNECT_WEBHOOK_SECRET`, `STRIPE_ACCOUNT_WEBHOOK_SECRET`.

Connect is configured for **direct charges** — Stripe's "your merchants
collect payments directly", the Customer → Merchant → You shape. Every
Invoice is created with `Params.StripeAccount` set to the Practice's
connected account (`payments/stripe_api_client.go`), so the Client's money
lands in the Practice's balance and never passes through ours. The
alternative — the platform collecting and then paying recipients — would
put Client funds on our balance sheet and make us a money transmitter.

Accounts v2 makes that explicit where v1 left it implied. Account creation
sets `defaults.responsibilities.fees_collector` and `losses_collector` to
`stripe` — both are mandatory for a merchant configuration and have no
default — meaning the Practice's own account is billed Stripe's processing
fee and absorbs a disputed charge. `identity.country` is likewise required
at create time and is hardcoded `us`, matching the USD credit Price and the
USD invoice currency. See
[ADR-0007](adr/0007-connect-account-state-is-two-capabilities-and-a-requirements-list.md).

Doula Cloud takes **no per-transaction cut**: there is no
`ApplicationFeeAmount` anywhere. Practices pay for credits in a separate
transaction, which is what surface A is.

### One credit costs $20.00

Sandbox Price, USD, one-time. One credit is one Engagement, which is one
birth. $20.00 was settled on #439 against a $500–$2,000 birth engagement:
4% at the bottom of that range and 1% at the top, and about $30 a month
for a doula carrying roughly 1.5 births a month, which is ordinary
practice-management pricing. A joining Practice is granted three Credits
for each Staff member it has on the day it joins, so nobody rides the
whole pilot free and nobody hits a wall in week one.

**The Price object is the only authority for that number.** Nothing the
software reads holds a second copy: `STRIPE_CREDIT_PRICE_ID` carries an
id, not an amount, and the BFF reads the unit amount back off the Price
at purchase time (`0d492c8`) because an apportioned Session has to state
cent amounts itself. Stripe cannot edit `unit_amount` on an existing
Price, so changing the price means a new Price on the same Product, the
old one archived, and this variable moved everywhere it is set. #448 did
exactly that: `price_1U7NKZ…` at $5.00 is archived, `price_1U9yTw…` at
$20.00 replaced it. A replacement must carry `tax_behavior: exclusive`,
or `automatic_tax` refuses to compute at all.

### No PHI reaches Stripe

Settled by #30, enforced in #78. The credit flow sends a Practice id and
a count. The invoice flow sends a Client's name and email. Any new Stripe
field is subject to the same rule.

## Local webhook delivery

Stripe cannot reach a laptop, so the Stripe CLI holds an outbound
connection open and replays events to a local URL.

```
cd app && bun run dev:full     # terminal 1
bash scripts/stripe-listen.sh  # terminal 2
```

`stripe-listen.sh` forwards straight to the BFF on `127.0.0.1:18080`, not
through the vite proxy: a signature check reads the exact request bytes,
and one hop fewer is one fewer thing to have rewritten them.

**All three local secrets are the same value, on purpose.** A single
`stripe listen` session signs everything it forwards with its own one
secret, whichever of `--forward-to`, `--forward-connect-to` or
`--forward-thin-connect-to` the event goes to
([CLI reference](https://docs.stripe.com/cli/listen)). The secret does not
change between restarts. Deployed, they really are three different secrets,
because they come from three separately created destinations.

Thin events need naming. `stripe listen` forwards nothing to
`--forward-thin-connect-to` unless the event type is also listed in
`--thin-events`; `stripe-listen.sh` already passes the one type the account
route handles.

Read the current one back at any time with `stripe listen --print-secret`.

## Deployed webhook endpoints

They point at the raw Cloud Run URL — the one `gcloud run services describe`
reports for itself, `https://doula-api-850855848778.us-central1.run.app`, not
the legacy `doula-api-yrg7ybdc2q-uc.a.run.app` form. Both reach the same
service; the reported one is the one to trust. Not
`https://doula-cloud-app.web.app/api/…`. The Firebase Hosting rewrite
would put a CDN proxy in front of a signature check over the exact
request body, for no gain — a webhook carries no cookie and no session,
so the rewrite buys nothing here.

`csrf.Wrap` lets them through: a Stripe POST carries no `Origin` header,
and the rule is "no Origin, no rejection" (`api/internal/csrf/wrap.go:26`,
enforced at `wrap.go:44`; the origin list is resolved in
`api/main.go:47-70`).

| Endpoint | Events | Payload | `events_from` | State |
| --- | --- | --- | --- | --- |
| `/api/stripe/webhook` | `checkout.session.completed` | snapshot | `@self` | `we_1U7NT01rKoVEA79vnOcBFqtV`, enabled |
| `/api/stripe/account-webhook` | `v2.core.account[configuration.merchant].capability_status_updated` | thin | `@self` | `ed_test_61VGn5QffuUmiONhX16VGl100QSQI7KSJpC2tKXrs4xU`, enabled |
| `/api/stripe/connect-webhook` | `invoice.paid`, `invoice.payment_failed` | snapshot | `@accounts` | `we_1U7Ocp1rKoVEA79vT9AXETKU`, enabled |

The two Connect rows are one feature, split by Stripe's own constraint: an
event destination has one `event_payload`, and the account events are thin
while the Invoice events are snapshot. The snapshot destination pins
`snapshot_api_version` to `2026-07-29.dahlia`, the version
`stripe-go v86.3.0` sends on every request (`api_version.go`), so a
delivered object always deserializes into the structs the SDK ships.

Both are created over `/v2/core/event_destinations`, not the v1
`/v1/webhook_endpoints` surface, and neither carries a `connect=true`
flag — v2 replaced that with `events_from`.

### `events_from` is not the same for the two Connect destinations

The account destination is **`@self`**, not `@accounts`, and getting this
wrong is silent — the destination stays `enabled`, `status_details` stays
`null`, and nothing is delivered. It cost a debugging round on #247 and
`events_from` cannot be patched, so a wrong one means delete, recreate, and
roll the signing secret.

The rule: a v2 Account is an object the **platform** owns, so events about
it are emitted on the platform. An Invoice raised with the `Stripe-Account`
header belongs to the **connected account**, so those are `@accounts`.

Proved rather than reasoned. With the account destination on `@accounts`, a
freshly created v2 Account emitted `capability_status_updated` and no request
reached Cloud Run at all. A second destination on `@self`, pointed at the same
route, delivered on the next account create. Recreated on `@self`, the route
now answers `200` and logs the handler dropping an account no Practice claims.

The Invoice destination's `@accounts` is **not** verified this way — that needs
a Practice that has finished onboarding and raised a real Invoice, which is the
walk #247 still owes. If `invoice.paid` turns out not to arrive, this is the
first thing to check.

Both Connect secrets are in Secret Manager and wired to the service. Two
things that were not obvious when setting them up:

- **A new secret needs its own IAM binding.** Cloud Run's runtime service
  account (`850855848778-compute@developer.gserviceaccount.com`) is granted
  `roles/secretmanager.secretAccessor` **per secret**, not at the project
  level, so a freshly created secret is unreadable and the deploy fails with
  `Permission denied on secret`. Grant it with
  `gcloud secrets add-iam-policy-binding <name> --member=serviceAccount:… --role=roles/secretmanager.secretAccessor`.
- **`:latest` re-resolves per revision, not per version.** Adding a new
  secret version changes nothing on a running service; a new revision has to
  be created for it to be picked up.

Set out of band on purpose, for the reason the next section gives.

A signing secret is returned **once**, in the create response — for an
event destination, only if the request asks for it by name via
`include: ["webhook_endpoint.signing_secret"]`. Neither retrieve nor list
ever shows it again. If it is lost, roll it in the dashboard and add a new
Secret Manager version.

## Why the deployed Stripe values are set out of band

`ci.yml`'s deploy step carries `env_vars_update_strategy: merge`, and
`secrets` merges by default too. The Stripe env vars and secret references were
set directly with `gcloud run services update`, and every later push merges on
top of them rather than replacing them. **Verified, not assumed**: after the
update, `DATABASE_URL` and `EXPECTED_ORIGINS` were both still on the service.

The alternative — naming the Secret Manager secrets in `ci.yml`'s `secrets:`
line beside `DATABASE_URL` — is the tidier record, but it makes a green deploy
depend on secrets that only exist after a human has opened the Stripe account.
That ordering is why it stays out of band.

The commands that produced the current state, so it can be rebuilt:

```
gcloud secrets create doula-cloud-stripe-api-key --replication-policy=automatic --data-file=-
gcloud secrets create doula-cloud-stripe-webhook-secret --replication-policy=automatic --data-file=-
gcloud secrets add-iam-policy-binding <secret> \
  --member serviceAccount:850855848778-compute@developer.gserviceaccount.com \
  --role roles/secretmanager.secretAccessor
gcloud run services update doula-api --region us-central1 \
  --update-env-vars APP_BASE_URL=…,STRIPE_CREDIT_PRICE_ID=… \
  --update-secrets STRIPE_API_KEY=…,STRIPE_WEBHOOK_SECRET=…
```

## CI stays on the fakes

`bun run test:e2e` sets no Stripe variables, so `api/internal/billing`
and `api/internal/payments` run against their injected fakes
(`stripe_fake.go` in each). That is deliberate:

- A GitHub-hosted runner has no public URL, so Stripe could not deliver a
  webhook to it without a tunnel process inside the job.
- The Sandbox is one shared, stateful environment. Parallel CI runs would create
  Customers, Checkout Sessions and connected accounts in the same place
  and read each other's rows.
- It would make a green suite depend on a third party being up.

The fakes are the thing under test in CI. The Sandbox is the thing a
human walks, once, when the code changes.

## No Playwright spec drives a Stripe page

Decided with the above. Checkout, the Account Link and the hosted invoice
are Stripe's own pages: their DOM is not ours, it changes without notice,
and a spec over it would assert Stripe's markup rather than our product.
The parts we own — the Billing screen, the Payments settings screen, the
Invoice section — are covered up to the redirect and from the redirect
back. What happens in between is walked by hand, per the run logs in
`docs/test-plans/`.

## Account posture: platform, not merchant of record

Stripe's onboarding offers Managed Payments, where Stripe becomes the
merchant of record: legally the seller, registering for and remitting
sales tax and VAT, absorbing chargebacks, for 3.5% on top of the card
fee. Doula Cloud does not take it.

It suits a business that only ever sells its own product. Doula Cloud has
a second surface where the Practice is the merchant and we are the
platform, and every merged Stripe code path assumes the platform posture:
our own Price, our own Checkout Session, our own Customer, our own
webhook, and Standard connected accounts underneath. Stripe Tax and Radar
for Fraud Teams are both left off for the same reason — neither is needed
in the Sandbox, and SaaS sales tax is a launch question, not a Sandbox
one.

Stripe then offers four products, and two stay on: **Send invoices** and
**Build a platform**. The second is Connect, described in plain words —
"let your customers accept payments and receive payouts through your
product". It appears only inside the Sandbox, so the same screen shown
before entering one offers three options and hides it.
**Create subscriptions** is off because #45 settled that there is no
subscription and no tiers — a Practice prepays per Engagement, and the
purchase is a one-time Checkout Session in `mode=payment`. **Collect
tax** is off because Stripe Tax wants an origin address and a tax code
per Price before it does anything, and SaaS sales tax is a launch
question rather than a Sandbox one.

The same screen offers **"Connect to a platform instead"**. That link
reads backwards: it makes this account a *connected account* under
someone else's platform, which is the role a Practice plays. Connect is
enabled separately, on its own dashboard page.

Stripe says all of this is changeable later, and it is.

## First-time setup

There is no wizard. `scripts/stripe-setup.sh` was retired in `e1ff4eb`
once the deployed service was pointed at Stripe; only
`scripts/stripe-listen.sh` remains. Copy `app/.env.example` to
`app/.env.local` and fill it from the sections above: the Sandbox API key
and Price id from the Stripe dashboard, the three webhook secrets from
one `stripe listen --print-secret`, and the Mailgun key paired with the
account's sandbox domain.
