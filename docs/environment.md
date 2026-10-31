# Environment variables

Every variable `api/main.go` reads, what it is for, and what it holds in each of the three places the BFF runs. The marketing site's own build reads two of its own — see [The marketing site's build](#the-marketing-sites-build). `app/.env.example` is the local template; the Stripe half of it is filled in by hand, from the Sandbox keys and the `stripe listen` secret (see [Stripe](#stripe)).

**Before you provision anything by hand, read [`docs/infrastructure.md`](infrastructure.md).** It is the boundary [#797](https://github.com/markgoho/doula-cloud/issues/797) drew between what Terraform owns and what stays a console visit, and it names every resource in the `doula-cloud` project on both sides with a reason. This page still describes what each value *is* and how it gets set; that page says whether setting it by hand is the right thing to do at all. The short version: secret values, Cloud SQL logins, Identity Platform and everything outside GCP stay by hand, and nearly everything else — the Scheduler jobs, the Tasks queue, `doula-api`'s configuration, the secret shells, IAM, the bucket and the database instance — becomes a change to a configuration file and a `terraform apply`. [ADR-0034](adr/0034-terraform-owns-the-shape-not-the-image-and-apply-stays-off-ci.md) records the decision.

## Where a value lives

| Place | Mechanism |
| --- | --- |
| Local (`bun run dev:full`) | `app/.env.local`, auto-loaded by bun, spread into the BFF's own environment by `app/e2e/stack.ts` |
| CI (`bun run test:e2e`) | `app/e2e/stack.ts` only. No Stripe values: the e2e run stays on the fakes — see [CI stays on the fakes](#ci-stays-on-the-fakes) |
| Deployed (Cloud Run `doula-api`) | Secret Manager for credentials, plain env vars for the rest |

A **simulation run** ([#763](https://github.com/markgoho/doula-cloud/issues/763)) is the local row with three additions, and it does not add a variable: it keeps the Postgres volume instead of destroying it, installs a `sim.now()` shim before goose runs, and drains the `process-*` endpoints itself. `APP_BASE_URL` keeps its local default and `EXPECTED_ORIGINS` is left alone, because `scripts/stripe-listen.sh` is how Stripe reaches a run and no tunnel is involved. See [Where a run runs](simulation/environment.md).

`app/.gitignore` and the repo root `.gitignore` both exclude `.env*` except `.env.example`. A Sandbox key is still a key.

**Which identity reads the deployed row: `doula-api-runtime@doula-cloud.iam.gserviceaccount.com`.** Every Secret Manager reference on `doula-api` is resolved by this account, not by whoever deployed the revision, so a secret it cannot read is a container that will not start. It was the project's default compute account, `850855848778-compute@developer.gserviceaccount.com`, until [#1051](https://github.com/markgoho/doula-cloud/issues/1051) — an account holding project `roles/editor`. That did **not** make the per-secret bindings on this page redundant: `roles/editor` deliberately excludes `secretmanager.versions.access`, which is why #743 still hit `Permission denied on secret` on a freshly created secret. It made everything *else* redundant — the container could change any Cloud Run service, alter the Cloud SQL instance, and write to any bucket in the project.

The account now holds, and holds nothing beyond: `roles/cloudsql.client` and a three-permission custom role for Identity Platform accounts (`firebaseauth.users.get/update/delete`) at the project level, because neither service has resource-level IAM; a `secretAccessor` grant on each secret it reads; `roles/cloudtasks.enqueuer` on `doula-cloud-notification-nudge`; and `roles/storage.objectUser` on `doula-cloud-attachments`. `terraform/` owns every one of them, each next to the resource it is granted on, and `ci.yml`'s `deploy-api` passes `--service-account` so a deploy cannot put the default account back. **A new secret therefore needs its binding for this account, and the binding belongs in `terraform/secrets.tf`, not only in a `gcloud` command.**

**The other identity that reads a secret payload is `github-action-733741680@doula-cloud.iam.gserviceaccount.com`, and since [#1078](https://github.com/markgoho/doula-cloud/issues/1078) the same rule binds it.** It reads two, both DSNs, both inside a GitHub Actions job rather than at container start: `ci.yml`'s `migrate` job fetches `doula-cloud-pg-migrate-dsn`, and `firebase-hosting-merge.yml`'s `build` job fetches `doula-cloud-pg-site-builder-dsn`. Until #1078 it held `roles/secretmanager.secretAccessor` at the *project* level — every secret on this page, the Stripe key included — so a new `get-secretmanager-secrets` step needed no grant and got one for free. It now holds that role on those two secrets alone, as `*_deploy_accessor` resources in `terraform/secrets.tf`. So a workflow step that fetches a secret this account has no grant on fails with `Permission denied`, and the fix is a binding beside the secret, the same as for the runtime account above.

## The variables

| Variable | Local | CI | Deployed |
| --- | --- | --- | --- |
| `DATABASE_URL` | `app_e2e` role on the compose Postgres | same | Secret Manager `doula-cloud-pg-app-runtime-dsn`, set in `ci.yml` |
| `EXPECTED_ORIGINS` | `http://localhost:5173` | `http://localhost:4173` | `https://doula-cloud-app.web.app`, set in `ci.yml` |
| `APP_BASE_URL` | defaults to `EXPECTED_ORIGINS` | same | `https://doula-cloud-app.web.app` |
| `STRIPE_API_KEY` | `sk_test_…` in `.env.local` | unset | Secret Manager `doula-cloud-stripe-api-key` |
| `STRIPE_PUBLISHABLE_KEY` | `pk_test_…` in `.env.local` (`stripe config --list` shows it as `test_mode_pub_key`). Optional locally: without it the portal pay route answers `409` "Your Practice cannot take this payment yet." | unset | Required: on Cloud Run (`K_SERVICE` is set) with `STRIPE_API_KEY` set, the BFF refuses to start without it ([#2437](https://github.com/markgoho/doula-cloud/issues/2437)). The Sandbox `pk_test_…`, a literal in `terraform/cloud_run.tf`, since the value is public by design ([#1572](https://github.com/markgoho/doula-cloud/issues/1572)). It must come from the account that holds `STRIPE_API_KEY`, and the cutover to the live account swaps it for `pk_live_…` in the same change that swaps the secret key ([#2384](https://github.com/markgoho/doula-cloud/issues/2384)) |
| `STRIPE_MODE` | unset, or `test`. Optional locally, but the BFF refuses to start on a live key whenever `K_SERVICE` is unset, so a laptop never runs on the live account | unset | `test` until the cutover to the live account, then `live` in the same change that swaps the keys. Required: on Cloud Run (`K_SERVICE` is set) the BFF refuses to start without it. Plain env var, set out of band first, `gcloud run services update doula-api --update-env-vars STRIPE_MODE=test`, and declared in `terraform/cloud_run.tf` with the same value ([#2361](https://github.com/markgoho/doula-cloud/issues/2361)). Whenever `STRIPE_API_KEY` is set, the BFF reads its mode from the prefix (`sk_test_`/`rk_test_` or `sk_live_`/`rk_live_`; any other prefix refuses to start), refuses a `STRIPE_PUBLISHABLE_KEY` in the other mode and a `STRIPE_MODE` that disagrees, and every Stripe webhook route answers `400` to an event whose `livemode` is the other mode |
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
| `NOTIFICATION_WORKER_SECRET` | `e2e-worker-secret`, set by `stack.ts`; matched against the `X-Internal-Secret` header on calls to any of the nineteen `/api/internal/**/process-*` endpoints and to `/api/internal/outboxes/drain` | same | **unset, deliberately ([#1183](https://github.com/markgoho/doula-cloud/issues/1183))** — the local and end-to-end mechanism only, since ADR-0037 made the deployed boundary a caller identity. Unset refuses the `X-Internal-Secret` header outright, so the deployed service has no shared-string path into `/api/internal/**` at all. The Secret Manager secret `doula-cloud-notification-worker-secret` is being removed from the project ([#1192](https://github.com/markgoho/doula-cloud/issues/1192)): nothing has read it since #1183, Terraform's configuration no longer declares it, and a human `apply` — outstanding — deletes the shell |
| `INTERNAL_OIDC_AUDIENCE` | unset (the end-to-end stack mints no tokens) | unset | `https://doula-api-850855848778.us-central1.run.app` — the raw Cloud Run **base** URL, the `aud` every internal caller's OIDC token must carry (ADR-0037), plain env var. Both Scheduler jobs and every Cloud Tasks nudge set their `audience` to exactly this string; leaving it to either service's default gives the full target URI, path included, and a 401 (#1183) |
| `INTERNAL_OIDC_CALLERS` | unset | unset | `internal-caller@`, `doula-api-runtime@` and `github-action-733741680@` — comma-separated service account emails whose ID tokens `/api/internal/**` accepts (#1183): the Scheduler jobs' caller, `doula-api`'s own runtime account, which is what its Cloud Tasks nudges arrive as, and the deploy identity, which `firebase-hosting-merge.yml` presents a token for at `verify-pages`. Plain env var; an empty list accepts nobody |
| `INTERNAL_OIDC_SERVICE_ACCOUNT` | unset (nothing local enqueues a real task) | unset | `doula-api-runtime@doula-cloud.iam.gserviceaccount.com` (#1183) — the account Cloud Tasks mints each nudge's OIDC token for, which is the account the service itself runs as. Unset falls back to sending `X-Internal-Secret`, which is what the local stack does |
| `MAILGUN_WEBHOOK_SIGNING_KEY` | `e2e-mailgun-signing-key`, set by `stack.ts` on both the BFF and the mailbox, which signs its bounce and complaint webhooks with it | same | Secret Manager `doula-cloud-mailgun-webhook-signing-key`, set (#743) -- Mailgun's `permanent_fail` and `complained` webhooks point at the deployed endpoint |
| `NOTIFICATION_TASKS_QUEUE` | unset (no real `tasknudge.CloudTasksEnqueuer` is constructed in `routes()` tests, which inject `tasknudge.FakeEnqueuer` instead) | unset | the Cloud Tasks queue's full resource name, `projects/doula-cloud/locations/us-central1/queues/doula-cloud-notification-nudge`, plain env var |
| `NOTIFICATION_TASKS_TARGET_BASE_URL` | unset | unset | the same raw Cloud Run URL `gcloud run services describe` reports (see [Deployed webhook endpoints](#deployed-webhook-endpoints)), plain env var |
| `GITHUB_DISPATCH_TOKEN` | unset (nothing local fires a real deploy) | unset | Secret Manager `doula-cloud-github-dispatch-token`, a fine-grained personal access token scoped to `markgoho/doula-cloud` with **Contents: write** |
| `GITHUB_FEEDBACK_TOKEN` | unset -- `main.go` wires `feedback.NewFakeIssueCreator()` instead (#1524, #1500's resolution) | unset, same fake as local | Secret Manager `doula-cloud-github-feedback-token`, a second fine-grained personal access token scoped to `markgoho/doula-cloud-feedback` alone with **Issues: write**. Created 2026-10-01, **expires 2027-10-01** ([#1529](https://github.com/markgoho/doula-cloud/issues/1529)): before then, regenerate it at <https://github.com/settings/personal-access-tokens>, add the new value as a new version of the secret, and deploy a new revision so `latest` resolves again. Unset falls back to the same fake -- a lapsed or missing token dead-letters the row rather than failing a deploy |
| `GITHUB_FEEDBACK_REPO` | unset | unset | `markgoho/doula-cloud-feedback`, plain env var. The founder read page builds its link to a piece's issue from it too (#1526); unset, the page prints the issue number with no link |
| `FOUNDER_STAFF_ID` | `0f0a1d5e-5a17-4c0b-9d0f-f0de15260001`, a fixed Staff id set by `stack.ts` (#1526). No Staff row has that id until one is seeded: `bun run --cwd app seed:founder-session` seeds it and prints the founder's session cookie | same; the spec that needs the founder seeds the row with `seedFounderSession` (`app/e2e/founder.ts`) | the founder's own `staff.id` in the deployed database, plain env var. **Not set yet** ([#1607](https://github.com/markgoho/doula-cloud/issues/1607)): it goes on the live service first, with `gcloud run services update doula-api --update-env-vars FOUNDER_STAFF_ID=<id>`, and into `terraform/cloud_run.tf` with the same value after, because the `plan` check fails on a declared variable the live service does not carry. Unset, or naming nobody, the two founder routes (`GET /api/staff/feedback`, `GET /api/staff/feedback/{feedbackId}`) answer `404` to everybody, so a service that was never told who the founder is serves the page to nobody |
| `E2E_CLOCK_FOLLOWS_SIM_OFFSET` | `1`, set by `stack.ts` (#1833): the BFF's clock is real time plus `sim.offset_row`'s delta, read again on each call, so a simulation jump moves `clock.Now(ctx)` as it moves the database's `now()` | same | **never set.** `api/internal/stackswitch` refuses to start the process when it is set unless `FIREBASE_AUTH_EMULATOR_HOST` is set and the database carries the `sim` schema. Cloud Run has neither: the emulator does not exist there, and `simclock` installs the schema only into a database that has never seen a migration |
| `E2E_LEGAL_VERSIONS` | unset at start. `terms-agreement.e2e.ts` restarts the BFF with it (`restartAPI`), a JSON list of versions to append to the shipped Terms and Privacy Policy history (`document`, `effective`, `material`, `change`), so a spec can publish a material version without a change to `api/internal/legal/legal.go` | same | **never set**, behind the same gate as the row above |
| `E2E_ADDRESS_RATE_LIMIT_SCALE` | `10`, set by `stack.ts` (#1858): every per-address rate-limit cap (`ratelimit.IPRule`) is multiplied by it, since every request the suite makes arrives from `127.0.0.1` and one pass makes more than 50 signups. No other dimension is scaled | same | **never set**, behind the same gate as the clock row above, so the deployed caps are the ones `api/internal/staffauth/mount.go` and its neighbors state |

## The marketing site's build

`bun run build` runs `scripts/sync-practice-pages.ts` before the `site/` build, which writes `site/practice-pages/<slug>.json` for every Practice that published one (#441); the build prerenders each file into `p/<slug>.html`. It reads two variables, and neither belongs to the BFF.

| Variable | Local | PR preview | Merge deploy |
| --- | --- | --- | --- |
| `SYNC_PRACTICE_PAGES` | unset | unset | `required`, set in `firebase-hosting-merge.yml` |
| `DATABASE_URL` | unset | unset | built from Secret Manager `doula-cloud-pg-site-builder-dsn`, dialed through the Cloud SQL Auth Proxy on `127.0.0.1:5432` |

**Unset means "touch nothing", not "connect if you can".** The script prunes `site/practice-pages` before it writes, because that is the only way a Practice who switches back to her own website loses her page. So an unreachable database and an empty result set would produce the same output — every live page deleted, against a Stripe review #382 established is ongoing. `SYNC_PRACTICE_PAGES=required` makes an unreachable database fail the build instead, and the workflow's build/deploy split means a failed build uploads no artifact and the live site stays exactly as it was.

The DSN belongs to `site_builder_login`, a Cloud SQL user created for this and granted `site_builder` — 00046's role, which holds `SELECT` on five tables and no write grant at all. The build job's credential can read what is about to be published and change nothing. The name mirrors `app_runtime_login`, which is granted `app_runtime` the same way.

### `APP_BASE_URL` is not `EXPECTED_ORIGINS`

They hold the same string in all three environments today, and they are still two variables. `EXPECTED_ORIGINS` is a list the CSRF check compares an inbound `Origin` header against. `APP_BASE_URL` is one string the BFF concatenates redirect targets onto: Checkout returns to `/practices/{id}/billing?checkout=success|canceled`, and the Connect Account Link returns to `/practices/{id}/settings/payments?connect=return|refresh`. A tunneled local walk needs `APP_BASE_URL` overridden and `EXPECTED_ORIGINS` left alone.

## The Staff password policy (Identity Platform)

A Staff password is 15 characters or more, with no composition rule, and Identity Platform enforces it ([#1538](https://github.com/markgoho/doula-cloud/issues/1538), [ADR-0026's amendment](adr/0026-two-populations-two-sign-in-methods-and-one-token-table.md)). The number is written once in `app/src/lib/passwordRule.ts` (`MIN_PASSWORD_LENGTH`) and once in `api/internal/staffauth/reset.go` (`minPasswordLength`), which refuses a short password on the reset path before the Admin SDK sees it.

**Terraform does not own the policy.** The `google` provider's `google_identity_platform_config` has no password-policy block, and [`docs/infrastructure.md`](infrastructure.md) already keeps Identity Platform's configuration out of Terraform ([ADR-0034](adr/0034-terraform-owns-the-shape-not-the-image-and-apply-stays-off-ci.md)). The policy is a one-time `PATCH` on the project's admin config. Before #1538 the live project returned `passwordPolicyConfig: null`, which is Identity Platform's default of six characters. `ENFORCE` makes the provider refuse a shorter password at account creation and at a password change; `forceUpgradeOnSignin` is left off, so no existing account is forced to change anything.

```sh
curl -X PATCH "https://identitytoolkit.googleapis.com/admin/v2/projects/doula-cloud/config?updateMask=passwordPolicyConfig" -H "Authorization: Bearer $(gcloud auth print-access-token)" -H "X-Goog-User-Project: doula-cloud" -H "Content-Type: application/json" -d '{"passwordPolicyConfig":{"passwordPolicyEnforcementState":"ENFORCE","passwordPolicyVersions":[{"customStrengthOptions":{"minPasswordLength":15,"maxPasswordLength":4096}}]}}'
```

Read it back; the response must show `ENFORCE` and `minPasswordLength` of 15, with no `containsLowercaseCharacter`, `containsUppercaseCharacter`, `containsNumericCharacter` or `containsNonAlphanumericCharacter`:

```sh
curl "https://identitytoolkit.googleapis.com/admin/v2/projects/doula-cloud/config" -H "Authorization: Bearer $(gcloud auth print-access-token)" -H "X-Goog-User-Project: doula-cloud"
```

**The Auth emulator cannot enforce it.** The e2e stack's and the local stack's emulator (firebase-tools 15) hardcodes a six-character minimum in `operations.js` and its `PATCH` on the project config accepts no `passwordPolicyConfig` (the v2 admin route answers 404; probed for #1538). So the rule is covered in the browser and on the BFF instead: `passwordRule.spec.ts` for the number, the three screens' specs (`signup`, `accept-invite`, `reset-password`) for the hint and the refusal before a request leaves, `formErrors.spec.ts` for the provider's own refusal in the same words, and `TestSpendResetHandler_PasswordTooShort` for the BFF. The e2e fixtures use passwords that pass the rule, so a run against a stricter emulator would not break.

## The common-password list

A Staff password on a list of commonly used passwords is refused on the three screens that set one ([#1539](https://github.com/markgoho/doula-cloud/issues/1539), ADR-0026's amendment for #1493). Identity Platform's password policy has no such list, so the check is the product's own.

**Where it comes from.** [SecLists](https://github.com/danielmiessler/SecLists) (MIT), pinned to one commit, from three files under `Passwords/Common-Credentials/`: `100k-most-used-passwords-NCSC.txt`, `xato-net-10-million-passwords-1000000.txt` and `Pwdb_top-1000000.txt`. Only an entry of 15 characters or more is kept, because a shorter one is already refused for its length; entries are lowercased, kept to printable ASCII, and a cracking tool's `$HEX[...]` encoding is dropped. The result is about 16,000 entries and 280 KB of text, about 130 KB compressed.

**Where it lives.** Two copies of the same bytes: `app/src/lib/commonPasswords.txt`, which the browser loads as a chunk of its own on `/signup`, `/accept-invite` and `/reset-password` only (`passwordRule.usage.spec.ts` names those three), and `api/internal/staffauth/commonpasswords.txt`, which the BFF embeds for the reset spend. `commonPasswords.spec.ts` fails the build if the two differ. Neither file is edited by hand.

**How it is updated.** Move `SECLISTS_COMMIT` in `app/scripts/sync-common-passwords.ts` to a newer commit (or change `SOURCES`), then run `bun run --cwd app sync-common-passwords`. The script writes both copies. Commit both.

**What is compared.** The whole password, lowercased, against whole entries: never a part of it. The comparison runs in the browser, so neither the password nor a hash of it goes anywhere to be checked. A range query on a hash prefix (k-anonymity, as Have I Been Pwned's Pwned Passwords offers) was the other choice and was not taken: it sends a part of a hash of the secret to a third party at signup, with the address of a person signing up for a pregnancy-related service, and makes signup depend on that party being up. The cost of the choice is coverage: this is a list of commonly used passwords, not of every breached one.

**Where it is enforced.** On `/signup` and `/accept-invite` the browser gives the password to Identity Platform directly, so the browser's check is the only one. On `/reset-password` the request already carries the password to the BFF, which hands it to the Admin SDK (ADR-0026, #613), so `staffauth/reset.go` refuses a common password too, before it spends the link.

**If the list cannot be loaded** (a dropped connection, or a deploy that replaced the chunk under an open tab), the screen refuses the submit and says "There is a problem with the service. Try again in a few minutes." An unchecked password is never let through: block over warn. The next submit loads the list again.

## Mailgun

Provisioned for #218 (map #213). One account, one API key, no per-domain Sending Key: Mailgun's API key is account-wide, so the same value works against both the sandbox domain and the verified one — only `MAILGUN_DOMAIN` changes between Local and Deployed.

**No local stack reaches Mailgun any more.** `app/e2e/stack.ts` sets `MAILGUN_API_BASE` to the sandbox mailbox (`app/e2e/mailbox.ts`, #764) on every BFF it starts, which is both the Playwright e2e run and `bun run dev:full`, and it sets `MAILGUN_DOMAIN`/`MAILGUN_API_KEY` explicitly alongside it rather than inheriting them. That is deliberate: `.env.local` holds a real account-level key, `bun` loads it before `stack.ts` runs, and without those three lines a local run would post real mail to real Mailgun. The mailbox answers the same `POST /v3/<domain>/messages` endpoint, holds what it is sent, and serves it back as JSON and as a browsable inbox. A walk that genuinely needs a real Mailgun send now runs the BFF by hand with the real values in its environment; nothing that goes through `startStack` will do it for you.

`mg.doula.cloud` is verified: SPF (TXT) and both DKIM records (CNAME, under Mailgun's automatic sender security, which rotates the key every 120 days) are live in Squarespace DNS. DMARC is a plain self-published TXT at `_dmarc.mg.doula.cloud` (`v=DMARC1; p=none;`) rather than Mailgun's suggested record, whose `rua`/`ruf` route reports to Red Sift's `inbox.ondmarc.com` — declined to avoid the third-party data share. Receiving records (MX) and tracking records (open/click CNAME) were not installed: inbound email is out of scope for the whole map, and tracking was left undecided rather than defaulted on.

Proved end to end: a real send from `notifications@mg.doula.cloud` landed in a real Gmail inbox, not spam, with `dkim=pass`, `spf=pass`, `dmarc=pass (p=NONE)`.

The commands that produced the current state, so it can be rebuilt:

```
gcloud secrets create doula-cloud-mailgun-api-key --replication-policy=automatic --data-file=-
gcloud secrets add-iam-policy-binding doula-cloud-mailgun-api-key \
  --member serviceAccount:doula-api-runtime@doula-cloud.iam.gserviceaccount.com \
  --role roles/secretmanager.secretAccessor
```

#219 lands the code that reads both variables (the outbox worker, the Client portal invite's real sender) plus a third, `NOTIFICATION_WORKER_SECRET`, guarding the Cloud-Scheduler-triggered `/api/internal/notifications/process-outbox` endpoint. #341 ran the deploy step: `MAILGUN_API_KEY` and `NOTIFICATION_WORKER_SECRET` were set as Cloud Run secrets on `doula-api` (the second of those was removed from the service by [#1183](https://github.com/markgoho/doula-cloud/issues/1183), which made the deployed boundary a caller identity), `MAILGUN_DOMAIN=mg.doula.cloud` is a plain env var, and a Cloud Scheduler job (`process-portal-invite-outbox`, `us-central1`, every 5 minutes) drove the endpoint on a fixed cadence. That job was deleted by #481, which replaced one job per outbox with a single `process-outbox-drain` — see [The outbox backstop](#the-outbox-backstop-one-drain-job). Proved end to end: a real Client portal invite, added and sent through the deployed app, arrived in a real inbox via the deployed pipeline (not a local one-off).

While provisioning this, Identity Platform's Email/Password sign-in provider turned out to be disabled in the production project (`GET .../admin/v2/projects/doula-cloud/config` had no `signIn.email` block) -- unrelated to Mailgun, but it meant nobody could sign up or log in to the deployed app at all. Enabled via `PATCH .../admin/v2/projects/doula-cloud/config?updateMask=signIn.email` with `{"signIn":{"email":{"enabled":true,"passwordRequired":true}}}`.

#342 (the out-of-Credits Platform Notification) reuses the same `/api/internal/**` boundary for a second endpoint, `/api/internal/notifications/process-low-credit-outbox` -- same guard, no credential of its own, but a separate outbox table (ADR-0010).

#343 (the payout-account-incomplete Platform Notification) reuses the same `/api/internal/**` boundary for a third endpoint, `/api/internal/notifications/process-payout-outbox` -- same guard, no credential of its own, its own outbox table (`payout_outbox`).

#344 (the payment-arrived Platform Notification) reuses the same `/api/internal/**` boundary for a fourth endpoint, `/api/internal/notifications/process-payment-outbox` -- same guard, no credential of its own, its own outbox table (`payment_received_outbox`).

#345 (the new-sign-in/session-revoked Platform Notifications) reuses the same `/api/internal/**` boundary for a fifth endpoint, `/api/internal/notifications/process-session-notice-outbox` -- same guard, no credential of its own, one outbox table (`session_notice_outbox`) shared by both notices since #345 bundled them into a single ticket.

#339 (the Staff invitation Notification, RA-G1) reuses the same `/api/internal/**` boundary for a sixth endpoint, `/api/internal/notifications/process-staff-invite-outbox` -- same guard, no credential of its own, its own outbox table (`staff_invite_outbox`). `staffauth.InviteHandler` (#316) is its write site.

#317 (the Offer Notification, ADR-0008) reuses the same `/api/internal/**` boundary for a seventh endpoint, `/api/internal/notifications/process-offer-outbox` -- same guard, no credential of its own, its own outbox table (`engagement_offer_outbox`). Its write site is `offer.CreateHandler`'s email-target path, which mails one link that both joins the Practice and opens the Offer, plus the six-digit access code the pre-account read asks for.

#398 (the Engagement Request Notification, ADR-0017) reuses the same `/api/internal/**` boundary for an eighth endpoint, `/api/internal/notifications/process-engagement-request-outbox` -- same guard, no credential of its own, its own outbox table (`engagement_request_outbox`, one row per Owner/Admin recipient rather than one per Request). Its write site is `engagementrequest.RequestHandler`, whenever the requester does not already hold approval authority herself.

#394 (Client erasure, ADR-0027) reuses the same `/api/internal/**` boundary for a ninth endpoint, `/api/internal/clients/process-erasure-outbox` -- same guard, no credential of its own, its own outbox table (`client_erasure_outbox`). It is the first `process-*` endpoint that mails nobody, which is why it sits under `/api/internal/clients` rather than `/api/internal/notifications`: it deletes a Stripe Customer, runs its Redaction Job once Stripe's 90-day floor has passed, and deletes an Identity Platform account. Its write site is `client.EraseHandler`. Note that the redaction leg needs one thing no environment variable can supply: Stripe's Redaction Jobs API is in public preview and is **not enabled on the DoulaCloud account** -- `POST /v1/privacy/redaction_jobs` answers "Unrecognized request URL" today, verified against the Sandbox. Until Stripe enables it, that act dead-letters with the API's own error while every other leg of an erasure completes; enabling it is a request to Stripe, not a code change.

#917 (the connect-nudge Platform Notification, ADR-0035) reuses the same `/api/internal/**` boundary for `/api/internal/notifications/process-connect-nudge-outbox` -- same guard, no credential of its own, its own outbox table (`connect_nudge_outbox`). It is the one outbox whose write site is a person pressing a control rather than a webhook or a background transition: `payments.PostConnectNudgeHandler`, offered on the Payments settings screen to a reader who may read Stripe Connect state and may not act on it, and only while the Practice has no Stripe account at all.

#1093 (the "a birth needs on-call cover" Platform Notification) reuses the same `/api/internal/**` boundary for `/api/internal/notifications/process-coverage-gap-outbox` -- same guard, no credential of its own, its own outbox table (`coverage_gap_outbox`). Its write site is a person saving a coverage gap that nobody is covering (`oncall.CreateGapHandler`, `oncall.UpdateGapHandler`); it mails every current Owner and Admin, resolved at send time, and its worker rechecks the gap first, so a gap covered, cleared or ended before the send mails nobody.

#1524 (the private-GitHub-issue outbox for a piece of Feedback, #1500's resolution) reuses the same `/api/internal/**` boundary for `/api/internal/feedback/process-issue-outbox` -- same guard, its own outbox table (`feedback_issue_outbox`), and its own credential: `GITHUB_FEEDBACK_TOKEN`/`GITHUB_FEEDBACK_REPO` above. It is the second `process-*` endpoint that mails nobody: it opens an issue in a private repo, carrying only the facts #1501's resolution names as safe (never the free text, the full URL, the Practice name, or the sender), and writes the issue number back onto the `feedback` row. Its write sites are `staffauth.FeedbackHandler` and `clientauth.FeedbackHandler` (#1523).

#1525 gives that same endpoint two more duties, with no new endpoint, credential or Scheduler job. The outbox carries a second act, `close_erased`: a Client's Erasure (`client.Erase`, and a Practice's Deletion through it) deletes her `feedback` rows and queues one such job for each row that has an issue, and the worker then adds the `erased` label and closes that issue. The job carries the issue number itself, because the row that held it is gone. And at the start of each run the worker deletes every piece of Feedback sent more than 24 months ago (`feedback.RetentionMonths`), reading the instant from its injected clock; that delete leaves the GitHub issue as it is.

#340 (the Mailgun bounce/complaint webhook, ADR-0010) adds a sixth variable, `MAILGUN_WEBHOOK_SIGNING_KEY` -- Mailgun's HTTP webhook signing key, a separate value from `MAILGUN_API_KEY`, verifying `POST /api/mailgun/webhook`'s HMAC-SHA256 signature rather than a shared-secret header. **Provisioned on #743**, so it is no longer unset on the deployed service. The key does not need Mailgun's dashboard: it comes back from the account-level API, `GET https://api.mailgun.net/v5/accounts/http_signing_key` with the same `api:<MAILGUN_API_KEY>` basic auth every other call uses. It was stored as Secret Manager `doula-cloud-mailgun-webhook-signing-key`, granted to the runtime service account (`850855848778-compute@` at the time; `doula-api-runtime@` since #1051), and attached to `doula-api` out of band the same way the Stripe and VAPID values are -- `gcloud run services update doula-api --region us-central1 --update-secrets MAILGUN_WEBHOOK_SIGNING_KEY=doula-cloud-mailgun-webhook-signing-key:latest`. That survives every later trunk deploy, because `deploy-cloudrun`'s `secrets:` input merges rather than replaces (see the comment on the deploy step in `ci.yml`).

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

#348 (ADR-0013's Cloud Tasks nudge) adds `NOTIFICATION_TASKS_QUEUE` and `NOTIFICATION_TASKS_TARGET_BASE_URL`, both consumed only by `tasknudge.NewCloudTasksEnqueuer` at startup. No new secret: the enqueued task identifies itself the same way Cloud Scheduler does — an OIDC token where `INTERNAL_OIDC_SERVICE_ACCOUNT` names an account, and `X-Internal-Secret` where it does not (ADR-0037). One queue serves all nine outbox types (`main.go` builds one `CloudTasksEnqueuer` and passes it into `routes()` as `nudgeEnqueuer`), provisioned once:

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

### The boundary on `/api/internal/**`

Thirty-two addresses sit under `/api/internal/**` — the drain, twenty-one `process-*` outbox endpoints, #443's page probe, the idempotency-key sweep, and eight operator endpoints. `api/routes_internal_guardrail_test.go` holds the list, read from the router rather than written from memory, and asserts that every one of them answers `401` to a request carrying nothing.

`doula-api` is `allUsers` → `roles/run.invoker` and stays that way — it serves the app and three webhooks that arrive with no Google credential — and Cloud Run's IAM is per-service, so the boundary can only be inside the process. [ADR-0037](adr/0037-the-internal-boundary-is-a-caller-identity-not-a-shared-secret.md) makes it a caller identity: `internalauth.Guard` verifies a Google-signed OIDC ID token against `INTERNAL_OIDC_AUDIENCE` and then the token's own `email` claim against `INTERNAL_OIDC_CALLERS`.

`X-Internal-Secret` remains the mechanism for the local stack and the end-to-end suite, which run the BFF with no metadata server to mint a token with. It is refused outright wherever `NOTIFICATION_WORKER_SECRET` is unset, so a deployment that carries no secret accepts no header.

The deployed service accepts a token and nothing else, since [#1183](https://github.com/markgoho/doula-cloud/issues/1183) set the three `INTERNAL_OIDC_*` variables on Cloud Run and removed `NOTIFICATION_WORKER_SECRET` from it. An operator endpoint is reached by minting a token rather than pasting a secret (`docs/runbooks/mfa-recovery-support.md`):

```sh
gcloud auth print-identity-token \
  --impersonate-service-account=internal-caller@doula-cloud.iam.gserviceaccount.com \
  --audiences=https://doula-api-850855848778.us-central1.run.app \
  --include-email
```

Both flags are load-bearing. `--audiences` is the service's base URL and not the endpoint's own URL, because one audience has to serve all thirty-two addresses — and it is the same trap in the infrastructure, where Cloud Scheduler and Cloud Tasks each default an unset `audience` to the full target URI. `--include-email` is what puts the `email` claim on the token; the guard refuses a token that carries none, so its absence reads as a 401 from an otherwise valid token.

### The pilot gate: who can create a Practice before January

Until January 2027, `POST /api/staff/signup` creates a Practice only for an address on the pilot allowlist ([#1790](https://github.com/markgoho/doula-cloud/issues/1790)). Any other address gets a `403` with the code `PILOT_NOT_OPEN`, nothing is written, and the signup screen says DoulaCloud opens in January 2027. Accepting an Invitation is not gated: an Owner already chose that person. Migration `00125` creates the gate closed. The local stack and the end-to-end suite open it as they start (`openPilotGate` in `app/e2e/stack.ts`), and so does every Go test database (`testdb`'s template).

Both operator acts below take a token minted with the command above. Each one records who did it and when, and neither can be undone through the API.

**Let a doula into the pilot.** Do this after you have spoken with the doula. `address` is the address the doula will sign up with; case and spaces do not matter. `addedBy` is your own name. A second add of the same address answers `200` and keeps the first record. If the doula was refused before you added the address, the same form sent again now goes through.

```sh
TOKEN=$(gcloud auth print-identity-token \
  --impersonate-service-account=internal-caller@doula-cloud.iam.gserviceaccount.com \
  --audiences=https://doula-api-850855848778.us-central1.run.app \
  --include-email)

curl -s -X POST 'https://doula-api-850855848778.us-central1.run.app/api/internal/staffauth/pilot-allowlist' \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"address": "<the doula'\''s email address>", "addedBy": "<your name>"}'
```

The answer is the stored row: `{"address", "addedBy", "addedAt"}`.

**Open the pilot gate in January.** The one deliberate act that opens signup to everyone ([ADR-0043](adr/0043-a-date-is-a-trigger-only-when-someone-set-it.md)). From then on, signup does not read the allowlist. Opening it a second time answers `200` and keeps the first record. There is no endpoint that closes it again.

```sh
curl -s -X POST 'https://doula-api-850855848778.us-central1.run.app/api/internal/staffauth/pilot-gate/open' \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"openedBy": "<your name>"}'
```

The answer is the gate's record: `{"openedBy", "openedAt"}`.

### The monthly Credits read

Once a month, the founder reads two counts per Practice from the Credit ledger and copies them into the private spreadsheet in the DoulaCloud Google account ([ADR-0046](adr/0046-the-product-counts-what-a-practice-did-and-never-who-she-is.md), [#1394](https://github.com/markgoho/doula-cloud/issues/1394)). Mint a token with the command above, then ask for the month just ended:

```sh
curl -s 'https://doula-api-850855848778.us-central1.run.app/api/internal/billing/monthly-credits?month=<YYYY-MM>' \
  -H "Authorization: Bearer $TOKEN"
```

The answer is one row per Practice per calendar month, from the month the Practice was created through the month asked for, or through the month the Practice was deleted: `{"practiceId", "practiceName", "practiceCreatedMonth", "month", "creditsBought", "creditsSpent", "firstPurchaseMonth"}`. Months are UTC. `creditsBought` is the sum of purchase lots that month. `creditsSpent` is the count of Credits spent, which is the number of Engagements started. A signup bonus, a founding grant, a refund and a forfeit count as neither. `firstPurchaseMonth` is `null` until the Practice buys. A month in the future is refused with `400`.

The rows fill three of the spreadsheet's nine columns, which ADR-0046 names: people interviewed, Practices on real Clients, Practices who bought a Credit, Credits bought, Credits spent, Practices who bought again, revenue, cost, the pool's remainder. For the month asked for:

- **Credits bought** is the sum of `creditsBought`.
- **Credits spent** is the sum of `creditsSpent`.
- **Practices who bought again** is the number of rows with `creditsBought` above zero and a `month` later than `firstPurchaseMonth`.

Revenue and cost come from the bank, and the other columns from the founder's own notes. The totals are the spreadsheet's arithmetic, not the endpoint's. Below fifty in the denominator, write each figure as a count with its denominator beside it, never as a rate.

The response names no Staff member, no Client, no login and no feature, and a spec fixes its field list. The four signup-source fields join each row with [#1393](https://github.com/markgoho/doula-cloud/issues/1393); a count by source is a floor, never a total.

### #443's site rebuild and page probe

Two more endpoints behind the same internal guard, under `/api/internal/site` rather than `/notifications` because neither of them notifies anybody. The same guard again, not a second credential.

`POST /api/internal/site/process-build-outbox` turns queued rebuilds into one `repository_dispatch`, and is the ninth type on ADR-0013's shared Cloud Tasks queue. It is the one type whose nudge is **delayed** -- 90 seconds, `tasknudge.Delay` -- because the worker collapses every pending row into a single deploy and can only collapse rows that have had a moment to gather. The drain job is its durability backstop: a dispatch that fails leaves the rows pending, and the next tick retries.

`POST /api/internal/site/verify-pages` fetches every published page from `doula.cloud` and records whether it resolved. Two callers, and deliberately identical behavior for both: the last step of `firebase-hosting-merge.yml`, which posts to the raw Cloud Run URL carrying an ID token minted for the identity it already authenticates as (`github-action-733741680@`, on the allowlist for that reason); and a Cloud Scheduler job every fifteen minutes. The sweep is what covers the case the workflow cannot -- a build that fails produces no deploy and no callback at all, so only something that runs anyway can notice a page that never went live.

`GITHUB_DISPATCH_TOKEN` is the one new credential. **Contents: write** is the narrowest permission GitHub's dispatch endpoint accepts, and it is the same level a GitHub App would need, which is why #443 chose the simpler thing. Give it a real expiry rather than "no expiration": a lapsed token is not silent here -- the dispatch fails, the page never leaves `pending`, and the Practice's website settings screen says her page is not confirmed.

**Provisioned.** `verify-practice-pages` runs every fifteen minutes in `us-central1`, carrying an OIDC token minted for `internal-caller@` since [#1183](https://github.com/markgoho/doula-cloud/issues/1183). #443's own `process-site-build-outbox` job existed too, and was deleted by #481 once the drain covered it -- see [The outbox backstop](#the-outbox-backstop-one-drain-job). The deploy workflow's service account (`github-action-733741680@doula-cloud.iam.gserviceaccount.com`) held `roles/secretmanager.secretAccessor` on `doula-cloud-notification-worker-secret` for the same step; that grant went with the secret it read, and the workflow now mints its own token through `google-github-actions/auth`.

**Still outstanding: the token itself.** It is created by hand at <https://github.com/settings/personal-access-tokens> -- resource owner `markgoho`, repository access *"Only select repositories"* → `doula-cloud`, Repository permissions → **Contents: Read and write**, expiry 366 days. Until it is in Secret Manager and on the service, `process-site-build-outbox` runs green with nothing to do and a real publish would sit at `pending` with the outbox retrying.

```bash
printf %s "<the token>" | gcloud secrets create doula-cloud-github-dispatch-token \
  --project=doula-cloud --data-file=- --replication-policy=automatic
gcloud secrets add-iam-policy-binding doula-cloud-github-dispatch-token \
  --project=doula-cloud \
  --member=serviceAccount:doula-api-runtime@doula-cloud.iam.gserviceaccount.com \
  --role=roles/secretmanager.secretAccessor
gcloud run services update doula-api --region us-central1 --project=doula-cloud \
  --update-secrets GITHUB_DISPATCH_TOKEN=doula-cloud-github-dispatch-token:latest
```

### The outbox backstop: one drain job

**Which Cloud Scheduler jobs exist.** Two, both in `us-central1`. Both authenticate with an `oidc_token` minted for `internal-caller@`, with `audience` set explicitly to the service's base URL ([ADR-0037](adr/0037-the-internal-boundary-is-a-caller-identity-not-a-shared-secret.md), [#1183](https://github.com/markgoho/doula-cloud/issues/1183)); neither carries a header, and neither job spec holds a secret:

| Job | Cadence | Calls |
| --- | --- | --- |
| `process-outbox-drain` | `*/5 * * * *` | `POST /api/internal/outboxes/drain` |
| `verify-practice-pages` | `*/15 * * * *` | `POST /api/internal/site/verify-pages` |

That is the whole list. No `process-*` endpoint has a job of its own any more, and none needs one: the drain runs every registration in `api/outboxes.go` in turn, each in its own transaction behind its own RLS door, so a fourteenth outbox is backstopped by being registered. Adding an outbox is no longer a console change. See [ADR-0010](adr/0010-notification-email-outbox.md)'s amendment and [#481](https://github.com/markgoho/doula-cloud/issues/481).

Locally there is no Scheduler and no nudge, so `bun run dev:full` calls the same `POST /api/internal/outboxes/drain` itself, every 3 seconds, with the `X-Internal-Secret` header, while it runs ([#2010](https://github.com/markgoho/doula-cloud/issues/2010), [`testing/walk-a-screen-locally.md`](testing/walk-a-screen-locally.md) step 6). The e2e suite does not: each spec drains the outbox it reads.

The drain answers `200` only when every outbox succeeded, and `500` naming the ones that failed, so a single broken outbox turns the job red rather than hiding inside a green tick. Each failure is also logged as `outbox: drain <path>: <error>`.

**Four outboxes have no nudge at all**, so the drain is the only thing that ever runs them: `process-staff-token-mail-outbox` and `process-staff-email-change-outbox` (#613 accepted ADR-0010's plain delay for both rather than wiring a nudge), plus `process-portal-magic-link-outbox` (#617) and `process-portal-address-change-outbox` (#619), which follow the same acceptance — ADR-0026 carves no ADR-0010 exception for either mail. Before #481 the first two had no Scheduler job either, which means they did not run outside a test or a hand invocation.

The three jobs that predated this — `process-portal-invite-outbox`, `process-site-build-outbox`, and #443's `verify-practice-pages` — are down to one: the first two were deleted as redundant once the drain covered them, and `verify-practice-pages` stays because it is not an outbox at all.

## Attachments bucket and VAPID push

#245 provisioned the two variables `ci.yml`'s own comment had wrongly claimed were already set out of band: `GCS_ATTACHMENTS_BUCKET` (Contract PDFs and Message attachments, `objectstore.NewGCSStore` in `main.go`) and the VAPID keypair (`push.NewVAPIDPusher`, Web Push delivery).

Proved end to end, not inferred from the code path: a real Practice, Client, Engagement and Contract were created against the deployed service, the Contract was sent and signed through the Client portal, and the signed PDF read back as `200 application/pdf` from the Owner-only endpoint.

The bucket carries no public access -- every read goes through `GetSignedContractPDFHandler` and the equivalent attachment handler, never a direct bucket URL. The runtime service account holds `roles/storage.objectUser` on this bucket only, never a project-wide role, matching the per-secret grants below. Narrowing it further to `objectCreator` + `objectViewer` was tried and rejected in #1051: `objectstore.GCSStore` exposes only `Put` and `Get`, but `Put` is documented to overwrite, and overwriting an object in a bucket with no versioning needs `storage.objects.delete` as well as `.create`. The grant is a `google_storage_bucket_iam_member` in `terraform/storage.tf`, so a hand-widened bucket policy is a red `terraform plan`.

The VAPID keypair is a real production pair, generated once with `webpush.GenerateVAPIDKeys()` from a throwaway `main.go` in `api/` that piped the private key straight into `gcloud secrets create` and printed only the public key -- deleted afterward, so the private key never sat in a file, a shell history, or a transcript longer than that one pipe. The local/CI throwaway pair in `stack.ts` is deliberately not this one.

The public key also has to reach the browser: `deploy-app`'s "Build app" step in `ci.yml` sets `VITE_VAPID_PUBLIC_KEY` at build time, read by `app/src/lib/pushRegistration.ts`. It is hardcoded in `ci.yml` rather than held as a GitHub secret -- a VAPID public key is meant to travel to the browser, so there is nothing to protect. Before #245, this variable was never set at all, so every browser push subscription on the live app signed against an empty key; `objectstore` and `push` failing closed on missing values is what kept that from ever reaching a real push service.

The commands that produced the current state, so it can be rebuilt:

```
gcloud storage buckets create gs://doula-cloud-attachments \
  --location=us-central1 \
  --uniform-bucket-level-access \
  --public-access-prevention
gcloud storage buckets add-iam-policy-binding gs://doula-cloud-attachments \
  --member=serviceAccount:doula-api-runtime@doula-cloud.iam.gserviceaccount.com \
  --role=roles/storage.objectUser
gcloud secrets create doula-cloud-vapid-private-key --replication-policy=automatic --data-file=-
gcloud secrets add-iam-policy-binding doula-cloud-vapid-private-key \
  --member=serviceAccount:doula-api-runtime@doula-cloud.iam.gserviceaccount.com \
  --role=roles/secretmanager.secretAccessor
gcloud run services update doula-api --region us-central1 \
  --update-env-vars GCS_ATTACHMENTS_BUCKET=doula-cloud-attachments,VAPID_PUBLIC_KEY=<public key>,VAPID_SUBSCRIBER=mailto:admin@doula.cloud \
  --update-secrets VAPID_PRIVATE_KEY=doula-cloud-vapid-private-key:1
```

## Say Sandbox, not test mode

Stripe renamed it. What #242 and older tickets call test mode, the dashboard now calls a **Sandbox** (`GLOSSARY.md`): a separate environment with its own data and its own API keys, rather than a toggle over one account. Keys from it still start `sk_test_`, so nothing the code expects changes.

What does change is where a key comes from. A key copied while the dashboard's environment switcher is on some other environment is a valid key for the wrong place, and it fails later as a signature error or a "no such price" — both of which read like code bugs and are not.

Whether a sandbox carries every Connect capability is not something to take on trust. Create a throwaway v2 Account over the API and read its `configuration.merchant.capabilities` back, so a missing capability surfaces there rather than an hour later in a walk.

## Stripe: two surfaces, one key

**Platform billing.** The Practice pays us. A Stripe Customer plus a Checkout Session over one flat Price. One event matters: `checkout.session.completed`. The `credit_ledger` is credited by `api/internal/billing/webhook.go` and by nothing else — never by the browser redirect.

**Connect.** The Client pays the Practice. An **Accounts v2** connected account per Practice carrying the `merchant` configuration, onboarded through a hosted v2 Account Link, with Invoices raised on-behalf-of using the `Stripe-Account` header. Seven events matter, and they do not all arrive the same way:

| Event | Kind | Route |
| --- | --- | --- |
| `v2.core.account[configuration.merchant].capability_status_updated` | thin | `/api/stripe/account-webhook` |
| `invoice.paid` | snapshot | `/api/stripe/connect-webhook` |
| `invoice.payment_failed` | snapshot | `/api/stripe/connect-webhook` |
| `invoice.marked_uncollectible` | snapshot | `/api/stripe/connect-webhook` |
| `payment_intent.processing` | snapshot | `/api/stripe/connect-webhook` |
| `credit_note.created` | snapshot | `/api/stripe/connect-webhook` |
| `refund.created` | snapshot | `/api/stripe/connect-webhook` |

`invoice.payment_failed` changes no status ([#2224](https://github.com/markgoho/doula-cloud/issues/2224)). Stripe sends it on every failed attempt, a declined card or a bank debit that bounces days later, and keeps the Invoice `open` for the next try, so ours stays open and the attempt is recorded on the Engagement's activity with the method tried. A Stripe-backed Invoice becomes `uncollectible` only on `invoice.marked_uncollectible`, which the Practice causes in her own Stripe Dashboard. Without that subscription a write-off there never reaches DoulaCloud. Stripe also sends `invoice.payment_failed` beside `invoice.payment_action_required` the moment a redirect method (Klarna) sends the Client away to approve it, before she has done anything (seen in the Sandbox on [#2225](https://github.com/markgoho/doula-cloud/issues/2225)), so the handler asks Stripe first and records nothing while the newest PaymentIntent is `requires_action`.

`payment_intent.processing` marks a payment on its way ([#2225](https://github.com/markgoho/doula-cloud/issues/2225)): a US bank debit stays `processing` for up to 4 business days, and Stripe keeps its Invoice `open` meanwhile, so ours stays open and `invoices.payment_processing_since` holds the fact beside it. Every Staff and Client read of the Invoice says "Payment on its way" while it is set, and `invoice.paid` or `invoice.payment_failed` clears it. Under API version `2026-07-29.dahlia` a PaymentIntent names no Invoice, so the handler asks Stripe which one it pays: `GET /v1/invoice_payments` filtered by `payment[type]=payment_intent` and `payment[payment_intent]`, with `data.payment.payment_intent.payment_method` expanded, which also gives the PaymentIntent's status now (a late delivery for a payment that already finished marks nothing) and its method. Verified read-only against a paid Invoice on a Sandbox connected account. Without the subscription a deployed bank payment reads "Not yet paid" until it clears.

The two Refund events (#1009) are both needed, and dropping either one is silent. A credit note a Practice issues from the Invoice page of her Stripe Dashboard fires `credit_note.created`; a refund she makes from the Payments page, against the charge, fires only `refund.created` and never creates a credit note. A card refund fires both, and `payments.refundReference` keys each on the same Refund object's id, so the second arrival finds the first's row. Verified in the Sandbox on #1009's issue comment.

`account.updated` is no longer what we act on. A v2 account **does** still emit v1 snapshot `account.updated` on the connected account — an earlier #247 note claimed otherwise, from a check that listed the *platform's* `/v1/events` where connected-account events never appear. What is true is that the v1 payload carries the v1 model (three booleans), which cannot express the four-valued capability statuses the `practices` columns now hold. The authoritative v2 state comes as a thin event.

One Stripe event destination also carries one `event_payload` — subscribing a single destination to both a thin and a snapshot event type is rejected outright. Hence two Connect routes, not one.

All three surfaces share `STRIPE_API_KEY` and nothing else. **Three endpoints, three secrets**: `STRIPE_WEBHOOK_SECRET`, `STRIPE_CONNECT_WEBHOOK_SECRET`, `STRIPE_ACCOUNT_WEBHOOK_SECRET`.

Connect is configured for **direct charges** — Stripe's "your merchants collect payments directly", the Customer → Merchant → You shape. Every Invoice is created with `Params.StripeAccount` set to the Practice's connected account (`payments/stripe_api_client.go`), so the Client's money lands in the Practice's balance and never passes through ours. The alternative — the platform collecting and then paying recipients — would put Client funds on our balance sheet and make us a money transmitter.

Accounts v2 makes that explicit where v1 left it implied. Account creation sets `defaults.responsibilities.fees_collector` and `losses_collector` to `stripe` — both are mandatory for a merchant configuration and have no default — meaning the Practice's own account is billed Stripe's processing fee and absorbs a disputed charge. `identity.country` is likewise required at create time and is hardcoded `us`, matching the USD credit Price and the USD invoice currency. See [ADR-0007](adr/0007-connect-account-state-is-two-capabilities-and-a-requirements-list.md).

DoulaCloud takes **no per-transaction cut**: there is no `ApplicationFeeAmount` anywhere. Practices pay for credits in a separate transaction, which is what surface A is.

### The price of a Credit

Sandbox Price, USD, one-time. One credit is one Engagement, which is one birth. The first price, $20.00, was settled on #439 against a $500–$2,000 birth engagement: 4% at the bottom of that range and 1% at the top, and about $30 a month for a doula carrying roughly 1.5 births a month, which is ordinary practice-management pricing. A joining Practice is granted three Credits for each Staff member it has on the day it joins, so nobody rides the whole pilot free and nobody hits a wall in week one.

**The Terms are the authority for that number, not the Price** ([#1834](https://github.com/markgoho/doula-cloud/issues/1834)). Each version of the Terms of Service states the price in its text (the version of 2026-10-03 says "A Credit costs $20.00"), and each version of the Terms in `api/internal/legal/legal.go` carries that amount as `CreditPriceCents`. Checkout charges the amount of the version in force at `clock.Now(ctx)`, and the Billing screen shows the same amount, read the same way. `STRIPE_CREDIT_PRICE_ID` only names the credit Product, through one of its Prices: the BFF reads the Product off that Price, then uses the active, one-time, USD, `tax_behavior: exclusive` Price on that Product whose `unit_amount` is the amount the Terms state. If no such Price exists, the purchase is refused; it is never charged at some other amount. So the variable never moves when the price does. Before #1834 it did: #448 archived `price_1U7NKZ…` at $5.00 and moved the variable to `price_1U9yTw…` at $20.00.

A version that charges more per Credit than the version before it counts only once it has been published through `POST /api/internal/legal/publish`. A rise in the history that was never published emailed nobody, so checkout keeps the old price after its date. A version that lowers the price needs no publication. `api/internal/legal/legal_test.go` holds that every version of the Terms states a price, that a version that raises it carries a `PriceRise` reason, and that a version with a reason raises it.

### A rise in the price of a Credit

The Terms promise: "If the price of a Credit ever rises, we email each Owner at least 60 days before, with the reason. A rise never changes a Credit you already bought: it keeps the price you paid for it, and until the new price starts you can buy as many Credits as you like at the old one." A rise is carried out in this order:

1. **A new Price.** Add the new amount to `credit_prices` in both `terraform/stripe-sandbox` and `terraform/stripe-live`, beside the old one (`{ "2000" = true, "2500" = true }`), and `apply` each, at any time before the date. The module creates it on the same Product with `tax_behavior: exclusive`. Stripe cannot edit `unit_amount` on an existing Price, and `automatic_tax` refuses to compute on a Price without `tax_behavior: exclusive`. Keep the old amount in the list: checkout charges it until the date.
2. **A new version of the Terms.** One commit adds the new text (a new `TermsOfService<date>.svelte` in `site/src/lib/legalTexts/`, stating the new price), a `legal.Version` with `Material: true`, the new `CreditPriceCents`, and the `PriceRise` reason, and its mirror in `site/src/lib/legal.ts`. The version's date is the day the new price starts, at midnight UTC. The same commit changes every other place that states the price by hand, since nothing can print it there:
   - `docs/copy/terms-of-service-page.md`, the copy of the newest text, which this commit rewrites for the new version anyway.
   - `docs/copy/pilot-terms-page.md`, the copy `/pilot-terms` was built from. The page itself prints the price of the version in force at each build, so it changes on the date with no edit.

   `site/src/lib/creditPriceCopies.spec.ts` fails when either document states a price other than the newest version's, and when this list leaves one out. `site/src/lib/legalTexts.svelte.spec.ts` fails when a version's text states a price other than its own `CreditPriceCents`. Every page that states the price reads it and needs no edit: `/pilot-terms` prints it with `creditPrice` from `site/src/lib/legal.ts` at each build, so it changes at the first daily build on or after the date, the Billing screen gets it from the BFF, and the marketing site's pricing copy is to print it with `creditPrice` when [#868](https://github.com/markgoho/doula-cloud/issues/868) and [#284](https://github.com/markgoho/doula-cloud/issues/284) build it. `GLOSSARY.md` (**Credit**) and step 2.1-a of `docs/test-plans/evaluator-doula.md` name the Terms as the price, not a number. The records of what was decided or found on a date keep the price they were written with: the ADRs, `docs/research/`, `docs/journeys/`, and a closed gap's row in a test plan.
3. **Publication 60 days ahead.** `POST /api/internal/legal/publish` with `{"document": "terms", "version": "<date>", "publishedBy": "<your name>"}`. It refuses a rise fewer than 60 days before its date, and refuses any version of the Terms whose price no active Price from step 1 charges, so a forgotten Price is found now and not on the date. It queues the email to each Owner with the reason, and its `legal_version_publications` row records who published the rise and when. From then on the Billing screen shows the new price and its date under today's price.
4. **The switch on the date.** Nobody acts. At midnight UTC on the date, checkout charges the new amount and so picks the new Price. Archive the old Price after that, not before, by setting its entry in `credit_prices` to `false` in both roots and applying; `STRIPE_CREDIT_PRICE_ID` can keep naming it, since an archived Price still names its Product.

A Credit already bought keeps its price: each lot stores the `unit_price_cents` it was paid at (migration 00052), and each Checkout Session carries the `terms_version` that set its price in its metadata.

### No PHI reaches Stripe

Settled by #30, enforced in #78. The credit flow sends a Practice id and a count. The invoice flow sends a Client's name and email. Any new Stripe field is subject to the same rule.

## Local webhook delivery

Stripe cannot reach a laptop, so the Stripe CLI holds an outbound connection open and replays events to a local URL.

```
cd app && bun run dev:full     # terminal 1
bash scripts/stripe-listen.sh  # terminal 2
```

`stripe-listen.sh` forwards straight to the BFF, not through the vite proxy: a signature check reads the exact request bytes, and one hop fewer is one fewer thing to have rewritten them. The BFF it forwards to is the worktree's own, on `127.0.0.1:<18080 + 100 × offset>`, where the offset is the number in the nearest `.port-offset` above the script, read the way `app/e2e/ports.ts` reads it ([#2201](https://github.com/markgoho/doula-cloud/issues/2201)). The main checkout has no file, so it is offset 0 and `18080`; offset 1 is `18180`. The script prints the address it chose. Run it from the same worktree as the `dev:full` it feeds.

**It runs two listeners.** One takes every snapshot event (`--all-snapshot`): the platform's go to `/api/stripe/webhook` (`--forward-to`) and the connected accounts' to `/api/stripe/connect-webhook` (`--forward-connect-to`). The other takes the one Accounts v2 thin event the account route handles, `v2.core.account[configuration.merchant].capability_status_updated`, to `/api/stripe/account-webhook`. It names that event in `--events` and forwards it with `--forward-to`. Since Stripe CLI 1.55, one session cannot send snapshot and thin events to different places. `--thin-events` and `--forward-thin-to` are deprecated, and the CLI says "Snapshot and thin events cannot share one destination". A `--forward-to` with none of `--events`, `--all-snapshot` or `--all-thin` is refused, and a thin event that no listener names is never delivered.

**All three local secrets are the same value, on purpose.** The CLI signs everything it forwards with one secret, which belongs to its login and not to a session. Both listeners print the same secret, and it does not change between restarts. Deployed, they really are three different secrets, because they come from three separately created destinations.

Read the current one back at any time with `stripe listen --print-secret`.

## Deployed webhook endpoints

They point at the raw Cloud Run URL — the one `gcloud run services describe` reports for itself, `https://doula-api-850855848778.us-central1.run.app`, not the legacy `doula-api-yrg7ybdc2q-uc.a.run.app` form. Both reach the same service; the reported one is the one to trust. Not `https://doula-cloud-app.web.app/api/…`. The Firebase Hosting rewrite would put a CDN proxy in front of a signature check over the exact request body, for no gain — a webhook carries no cookie and no session, so the rewrite buys nothing here.

`csrf.Wrap` lets them through: a Stripe POST carries no `Origin` header, and the rule is "no Origin, no rejection" (`api/internal/csrf/wrap.go:26`, enforced at `wrap.go:44`; the origin list is resolved in `api/main.go:47-70`).

**`terraform/stripe` is the authority for this table** ([#2390](https://github.com/markgoho/doula-cloud/issues/2390), [ADR-0034](adr/0034-terraform-owns-the-shape-not-the-image-and-apply-stays-off-ci.md)'s 2026-10-31 amendment). Each route's events, payload, `events_from` and `snapshot_api_version` are in `terraform/stripe/destinations.json`, and the module applies them to both accounts. `api/internal/payments/stripe_destinations_test.go` and `api/internal/billing/stripe_destinations_test.go` fail when a handler takes an event its route is not subscribed to, or the route is subscribed to one no handler takes, and when a snapshot route pins any version but the one `stripe-go` sends. `api/stripe_destinations_test.go` holds the routes to the ones `routes_webhook.go` mounts. So adding a handled event is one line in `destinations.json` and an `apply` in each account, never a by-hand `POST`. `terraform/stripe-sandbox` and `terraform/stripe-live` differ only in the key and the mode; [docs/infrastructure.md](infrastructure.md#stripe-state) has the state and the keys.

| Endpoint | Events | Payload | `events_from` | Sandbox | Live |
| --- | --- | --- | --- | --- | --- |
| `/api/stripe/webhook` | `checkout.session.completed` | snapshot | `@self` | `we_1U7NT01rKoVEA79vnOcBFqtV`, enabled, imported | created by [#2463](https://github.com/markgoho/doula-cloud/issues/2463) |
| `/api/stripe/account-webhook` | `v2.core.account[configuration.merchant].capability_status_updated` | thin | `@self` | `ed_test_61VGn5QffuUmiONhX16VGl100QSQI7KSJpC2tKXrs4xU`, enabled, imported | created by #2463 |
| `/api/stripe/connect-webhook` | `invoice.paid`, `invoice.payment_failed`, `invoice.marked_uncollectible`, `credit_note.created`, `refund.created`, `payment_intent.processing` | snapshot | `@accounts` | `we_1U7Ocp1rKoVEA79vT9AXETKU`, enabled, imported | created by #2463 |

The two `we_…` rows were created as v1 webhook endpoints, and Stripe lists them as v2 event destinations too, so the module imports them as it imports the `ed_test_` row. Their signing secrets do not change.

`invoice.marked_uncollectible` (#2224) and `payment_intent.processing` (#2225) were added to the Sandbox destination by hand on 2026-10-31, with `POST /v2/core/event_destinations/{id}`. That gap, code a manual command ahead of the Sandbox and silently ahead of live, is what #2390 closes.

The two Connect rows are one feature, split by Stripe's own constraint: an event destination has one `event_payload`, and the account events are thin while the Invoice events are snapshot. The Connect snapshot destination pins `snapshot_api_version` to `2026-07-29.dahlia`, the version `stripe-go v86.3.0` sends on every request (`api_version.go`), so a delivered object always deserializes into the structs the SDK ships. The platform destination has no `snapshot_api_version`, so it renders at the account's default API version. Provider `0.3.0` can only set one by replacing the destination, which would roll its signing secret, and `PostPurchaseWebhookHandler` accepts any version (`stripe.WithIgnoreAPIVersionMismatch`). `unpinnedSnapshotRoutes` in `api/internal/payments/stripe_destinations_test.go` names this one exception.

Both are created over `/v2/core/event_destinations`, not the v1 `/v1/webhook_endpoints` surface, and neither carries a `connect=true` flag — v2 replaced that with `events_from`.

### What `terraform/stripe` cannot hold

Provider `0.3.0` manages Products, Prices and event destinations, and not the account itself, so these stay by hand in each account and nothing turns red when one changes. Sandbox state read with `stripe get /v1/account` on 2026-10-31; a row it cannot show says so.

| Setting | Where | Sandbox | Live |
| --- | --- | --- | --- |
| Connect settings: Accounts v2, the `merchant` configuration, direct charges, hosted onboarding | Dashboard, Connect settings | Set up for #247; not readable through the API | By hand before the switch, [#2384](https://github.com/markgoho/doula-cloud/issues/2384) |
| Platform profile: name, website, statement descriptor | Dashboard, Business details | `business_profile.name` "Doula Cloud sandbox", `url` `https://doula.cloud`, statement descriptor `DOULA.CLOUD` | [#387](https://github.com/markgoho/doula-cloud/issues/387) | <!-- brand:ignore: the name Stripe has stored on the Sandbox account, quoted as read -->
| Business details: legal entity, address, bank account | Dashboard, activation | Not required in a Sandbox | #387 |
| Tax settings: origin address, Stripe Tax registrations | Dashboard, Tax | Off; SaaS sales tax is a launch question (below) | #387 |
| `support_address` | Dashboard only | `null` | #387 |

### `events_from` is not the same for the two Connect destinations

The account destination is **`@self`**, not `@accounts`, and getting this wrong is silent — the destination stays `enabled`, `status_details` stays `null`, and nothing is delivered. It cost a debugging round on #247 and `events_from` cannot be patched, so a wrong one means delete, recreate, and roll the signing secret.

The rule: a v2 Account is an object the **platform** owns, so events about it are emitted on the platform. An Invoice raised with the `Stripe-Account` header belongs to the **connected account**, so those are `@accounts`.

Proved rather than reasoned. With the account destination on `@accounts`, a freshly created v2 Account emitted `capability_status_updated` and no request reached Cloud Run at all. A second destination on `@self`, pointed at the same route, delivered on the next account create. Recreated on `@self`, the route now answers `200` and logs the handler dropping an account no Practice claims.

The Invoice destination's `@accounts` is **not** verified this way — that needs a Practice that has finished onboarding and raised a real Invoice, which is the walk #247 still owes. If `invoice.paid` turns out not to arrive, this is the first thing to check.

Both Connect secrets are in Secret Manager and wired to the service. Two things that were not obvious when setting them up:

- **A new secret needs its own IAM binding.** Cloud Run's runtime service account (`doula-api-runtime@doula-cloud.iam.gserviceaccount.com`) is granted `roles/secretmanager.secretAccessor` **per secret**, not at the project level, so a freshly created secret is unreadable and the deploy fails with `Permission denied on secret`. Grant it with `gcloud secrets add-iam-policy-binding <name> --member=serviceAccount:… --role=roles/secretmanager.secretAccessor`.
- **`:latest` resolves at every instance start.** A secret exposed as an environment variable is read when Cloud Run starts an instance, not when the revision is created ([Cloud Run: configure secrets](https://cloud.google.com/run/docs/configuring/services/secrets)), so a new version under `latest` reaches the next instance that scales up, with no deploy. That is why the Stripe secrets are pinned; see the next section.

Set out of band on purpose, for the reason the section after next gives.

A signing secret is returned **once**, in the create response — for an event destination, only if the request asks for it by name via `include: ["webhook_endpoint.signing_secret"]`. Neither retrieve nor list ever shows it again. If it is lost, roll it in the dashboard and add a new Secret Manager version, then name that version in `terraform/cloud_run.tf` (next section). A destination the module creates, which is every live one ([#2463](https://github.com/markgoho/doula-cloud/issues/2463)), has its first secret in Terraform state, because provider `0.3.0` always asks for it on create. That one is rolled at once and only the rolled secret goes to Secret Manager ([docs/infrastructure.md](infrastructure.md#stripe-state)).

## The Stripe secrets are pinned to a version

Since [#2406](https://github.com/markgoho/doula-cloud/issues/2406), `terraform/cloud_run.tf` attaches each of the four Stripe secrets at a version number, not `latest`:

| Secret | Version |
| --- | --- |
| `doula-cloud-stripe-api-key` | 1 |
| `doula-cloud-stripe-webhook-secret` | 1 |
| `doula-cloud-stripe-connect-webhook-secret` | 1 |
| `doula-cloud-stripe-account-webhook-secret` | 2 (version 1 is still enabled, and unused) |

**Adding a version to one of them has no effect until a PR names it.** The new value sits in Secret Manager unused; the service changes only when a PR changes the number in `cloud_run.tf` and the founder runs `terraform apply` (ADR-0034). That matters because of the #2361 guard: `STRIPE_MODE` is a plain value in the same file, and an `sk_live_` key under `latest` with `STRIPE_MODE=test` would make every new instance refuse to start. Pinned, the live secrets can be added ahead of the go-live switch, and the switch is one PR changing four version numbers and `STRIPE_MODE` together. A rollback is the same PR reverted.

`VAPID_PRIVATE_KEY` is pinned for the same reason: its pair, `VAPID_PUBLIC_KEY`, is a plain value in the same file. The other secrets stay at `latest`. `DATABASE_URL` is named `:latest` by `ci.yml`'s deploy step on every push, so a pin here would be undone by the next deploy. `MAILGUN_API_KEY`, `MAILGUN_WEBHOOK_SIGNING_KEY`, `GITHUB_DISPATCH_TOKEN` and `GITHUB_FEEDBACK_TOKEN` each stand alone, with nothing in this repo they must agree with, and a new version of one is a rotation meant to take effect.

## Why the deployed Stripe values are set out of band

`ci.yml`'s deploy step carries `env_vars_update_strategy: merge`, and `secrets` merges by default too. The Stripe env vars and secret references were set directly with `gcloud run services update`, and every later push merges on top of them rather than replacing them. **Verified, not assumed**: after the update, `DATABASE_URL` and `EXPECTED_ORIGINS` were both still on the service.

The alternative — naming the Secret Manager secrets in `ci.yml`'s `secrets:` line beside `DATABASE_URL` — is the tidier record, but it makes a green deploy depend on secrets that only exist after a human has opened the Stripe account. That ordering is why it stays out of band.

The commands that produced the current state, so it can be rebuilt:

```
gcloud secrets create doula-cloud-stripe-api-key --replication-policy=automatic --data-file=-
gcloud secrets create doula-cloud-stripe-webhook-secret --replication-policy=automatic --data-file=-
gcloud secrets add-iam-policy-binding <secret> \
  --member serviceAccount:doula-api-runtime@doula-cloud.iam.gserviceaccount.com \
  --role roles/secretmanager.secretAccessor
gcloud run services update doula-api --region us-central1 \
  --update-env-vars APP_BASE_URL=…,STRIPE_CREDIT_PRICE_ID=… \
  --update-secrets STRIPE_API_KEY=…,STRIPE_WEBHOOK_SECRET=…
```

## CI stays on the fakes

`bun run test:e2e` sets no Stripe variables, so `api/internal/billing` and `api/internal/payments` run against their injected fakes (`stripe_fake.go` in each). That is deliberate:

- A GitHub-hosted runner has no public URL, so Stripe could not deliver a webhook to it without a tunnel process inside the job.
- The Sandbox is one shared, stateful environment. Parallel CI runs would create Customers, Checkout Sessions and connected accounts in the same place and read each other's rows.
- It would make a green suite depend on a third party being up.

The fakes are the thing under test in CI. The Sandbox is the thing a human walks, once, when the code changes.

## No Playwright spec drives a Stripe page

Decided with the above. Checkout, the Account Link and the hosted invoice are Stripe's own pages: their DOM is not ours, it changes without notice, and a spec over it would assert Stripe's markup rather than our product. The parts we own — the Credits screen, the Getting paid settings screen, the Invoice section — are covered up to the redirect and from the redirect back. What happens in between is walked by hand, per the run logs in `docs/test-plans/`.

## Account posture: platform, not merchant of record

Stripe's onboarding offers Managed Payments, where Stripe becomes the merchant of record: legally the seller, registering for and remitting sales tax and VAT, absorbing chargebacks, for 3.5% on top of the card fee. DoulaCloud does not take it.

It suits a business that only ever sells its own product. DoulaCloud has a second surface where the Practice is the merchant and we are the platform, and every merged Stripe code path assumes the platform posture: our own Price, our own Checkout Session, our own Customer, our own webhook, and Standard connected accounts underneath. Stripe Tax and Radar for Fraud Teams are both left off for the same reason — neither is needed in the Sandbox, and SaaS sales tax is a launch question, not a Sandbox one.

Stripe then offers four products, and two stay on: **Send invoices** and **Build a platform**. The second is Connect, described in plain words — "let your customers accept payments and receive payouts through your product". It appears only inside the Sandbox, so the same screen shown before entering one offers three options and hides it. **Create subscriptions** is off because #45 settled that there is no subscription and no tiers — a Practice prepays per Engagement, and the purchase is a one-time Checkout Session in `mode=payment`. **Collect tax** is off because Stripe Tax wants an origin address and a tax code per Price before it does anything, and SaaS sales tax is a launch question rather than a Sandbox one.

The same screen offers **"Connect to a platform instead"**. That link reads backwards: it makes this account a *connected account* under someone else's platform, which is the role a Practice plays. Connect is enabled separately, on its own dashboard page.

Stripe says all of this is changeable later, and it is.

## First-time setup

There is no wizard. `scripts/stripe-setup.sh` was retired in `e1ff4eb` once the deployed service was pointed at Stripe; only `scripts/stripe-listen.sh` remains. Copy `app/.env.example` to `app/.env.local` and fill it from the sections above: the Sandbox API key and Price id from the Stripe dashboard, the three webhook secrets from one `stripe listen --print-secret`, and the Mailgun key paired with the account's sandbox domain.
