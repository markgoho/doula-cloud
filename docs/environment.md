# Environment variables

Every variable `api/main.go` reads, what it is for, and what it holds in
each of the three places the BFF runs. `app/.env.example` is the local
template; `scripts/stripe-setup.sh` fills the Stripe half of it in.

## Where a value lives

| Place | Mechanism |
| --- | --- |
| Local (`bun run dev:full`) | `app/.env.local`, auto-loaded by bun, spread into the BFF's own environment by `app/e2e/stack.ts` |
| CI (`bun run test:e2e`) | `app/e2e/stack.ts` only. No Stripe values: the e2e run stays on the fakes — see [CI stays on the fakes](#ci-stays-on-the-fakes) |
| Deployed (Cloud Run `doula-api`) | Secret Manager for credentials, plain env vars for the rest |

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
| `GCS_ATTACHMENTS_BUCKET` | `doula-cloud-e2e-attachments` | same | the real bucket |
| `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` / `VAPID_SUBSCRIBER` | throwaway keypair in `stack.ts` | same | real keys |

### `APP_BASE_URL` is not `EXPECTED_ORIGINS`

They hold the same string in all three environments today, and they are
still two variables. `EXPECTED_ORIGINS` is a list the CSRF check compares
an inbound `Origin` header against. `APP_BASE_URL` is one string the BFF
concatenates redirect targets onto: Checkout returns to
`/practices/{id}/billing?checkout=success|cancelled`, and the Connect
Account Link returns to `/practices/{id}/settings/payments?connect=return|refresh`.
A tunnelled local walk needs `APP_BASE_URL` overridden and
`EXPECTED_ORIGINS` left alone.

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

`account.updated` is gone. A v2 account emits no v1 snapshot event at all
(#247 verified this in the Sandbox: creating one produced six `v2.core.*`
thin events and nothing on `/v1/events`), and one Stripe event destination
carries one `event_payload` — subscribing a single destination to both a
thin and a snapshot event type is rejected outright. Hence two Connect
routes, not one.

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

### One credit costs $5.00

Sandbox Price, USD, one-time. One credit is one Engagement, which is
one birth; the first three per Practice are free forever (#45). $5 is a
deliberate pilot number: low enough that a 14-doula agency can try the
platform without a procurement conversation, and a real amount rather
than a `$1` placeholder nobody could show a customer.

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
and the rule is "no Origin, no rejection" (`api/main.go:125-129`).

| Endpoint | Events | Payload | `events_from` | State |
| --- | --- | --- | --- | --- |
| `/api/stripe/webhook` | `checkout.session.completed` | snapshot | `@self` | `we_1U7NT01rKoVEA79vnOcBFqtV`, enabled |
| `/api/stripe/account-webhook` | `v2.core.account[configuration.merchant].capability_status_updated` | thin | `@accounts` | `ed_test_61VGma0wEKcK6gNt916VGl100QSQI7KSJpC2tKXrsA8e`, enabled |
| `/api/stripe/connect-webhook` | `invoice.paid`, `invoice.payment_failed` | snapshot | `@accounts` | `we_1U7Ocp1rKoVEA79vT9AXETKU`, enabled |

The two Connect rows are one feature, split by Stripe's own constraint: an
event destination has one `event_payload`, and the account events are thin
while the Invoice events are snapshot. The snapshot destination pins
`snapshot_api_version` to `2026-07-29.dahlia`, the version
`stripe-go v86.3.0` sends on every request (`api_version.go`), so a
delivered object always deserializes into the structs the SDK ships.

Both are created over `/v2/core/event_destinations`, not the v1
`/v1/webhook_endpoints` surface, and neither carries a `connect=true`
flag — v2 replaced that with `events_from: ["@accounts"]`.

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

```
bash scripts/stripe-setup.sh
```

Twelve stages, from the Google Workspace mailbox that owns the account
through both end-to-end walks. It is re-runnable: values already in
`app/.env.local` come back as defaults.
