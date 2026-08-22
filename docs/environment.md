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
except `.env.example`. A test-mode key is still a key.

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

## Stripe: two surfaces, one key

**Platform billing.** The Practice pays us. A Stripe Customer plus a
Checkout Session over one flat Price. One event matters:
`checkout.session.completed`. The `credit_ledger` is credited by
`api/internal/billing/webhook.go` and by nothing else — never by the
browser redirect.

**Connect.** The Client pays the Practice. A Standard connected account
per Practice, onboarded through a hosted Account Link, with Invoices
raised on-behalf-of using the `Stripe-Account` header. Three events
matter: `account.updated`, `invoice.paid`, `invoice.payment_failed`.

They share `STRIPE_API_KEY` and nothing else. Two endpoints, two secrets.

### One credit costs $5.00

Test-mode Price, USD, one-time. One credit is one Engagement, which is
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

**Both local secrets are the same value, on purpose.** A single `stripe
listen` session signs everything it forwards with its own one secret,
whether the event goes to `--forward-to` or `--forward-connect-to`
([CLI reference](https://docs.stripe.com/cli/listen)). The secret does not
change between restarts. Deployed, they really are two different secrets,
because they come from two separately created endpoints.

Read the current one back at any time with `stripe listen --print-secret`.

## Deployed webhook endpoints

Both point at the raw Cloud Run URL
(`https://doula-api-yrg7ybdc2q-uc.a.run.app`), not at
`https://doula-cloud-app.web.app/api/…`. The Firebase Hosting rewrite
would put a CDN proxy in front of a signature check over the exact
request body, for no gain — a webhook carries no cookie and no session,
so the rewrite buys nothing here.

`csrf.Wrap` lets them through: a Stripe POST carries no `Origin` header,
and the rule is "no Origin, no rejection" (`api/main.go:125-129`).

| Endpoint | Events | `connect` |
| --- | --- | --- |
| `/api/stripe/webhook` | `checkout.session.completed` | false |
| `/api/stripe/connect-webhook` | `account.updated`, `invoice.paid`, `invoice.payment_failed` | true |

A webhook endpoint's signing secret is returned **once**, in the create
response. Neither retrieve nor list ever shows it again. If it is lost,
roll it in the dashboard and add a new Secret Manager version.

## Why the deployed Stripe values are set out of band

`ci.yml`'s deploy step carries `env_vars_update_strategy: merge`, and
`secrets` merges by default too. `scripts/stripe-setup.sh` sets the
Stripe env vars and secret references directly with
`gcloud run services update`, and every later push merges on top of them
rather than replacing them.

The alternative — naming the three Secret Manager secrets in `ci.yml`'s
`secrets:` line beside `DATABASE_URL` — is the tidier record, but it
makes a green deploy depend on secrets that only exist after a human has
run the wizard. That ordering is the reason it stays out of band. If the
secrets are ever recreated from scratch, `scripts/stripe-setup.sh` is the
reproducible path, not `ci.yml`.

## CI stays on the fakes

`bun run test:e2e` sets no Stripe variables, so `api/internal/billing`
and `api/internal/payments` run against their injected fakes
(`stripe_fake.go` in each). That is deliberate:

- A GitHub-hosted runner has no public URL, so Stripe could not deliver a
  webhook to it without a tunnel process inside the job.
- Test mode is a shared, stateful account. Parallel CI runs would create
  Customers, Checkout Sessions and connected accounts in the same place
  and read each other's rows.
- It would make a green suite depend on a third party being up.

The fakes are the thing under test in CI. Test mode is the thing a human
walks, once, when the code changes.

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
in test mode, and SaaS sales tax is a launch question, not a test-mode
one.

Stripe says this is changeable later, and it is.

## First-time setup

```
bash scripts/stripe-setup.sh
```

Twelve stages, from the Google Workspace mailbox that owns the account
through both end-to-end walks. It is re-runnable: values already in
`app/.env.local` come back as defaults.
