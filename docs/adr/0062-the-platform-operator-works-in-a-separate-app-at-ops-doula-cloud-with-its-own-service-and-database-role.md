# The Platform operator works in a separate app at ops.doula.cloud, with its own service and database role

Status: accepted, 2026-10-31. Charted on [Wayfinder: the Platform operator app at ops.doula.cloud (#2409)](https://github.com/markgoho/doula-cloud/issues/2409) and decided on its six grilling tickets, each cited below. Written on [#2419](https://github.com/markgoho/doula-cloud/issues/2419).

The founder decided on 2026-10-25 ([#1996](https://github.com/markgoho/doula-cloud/issues/1996)) that DoulaCloud gets an operations-only view across every Practice that never shows a Client record. Today the Platform operator (`GLOSSARY.md`) has two ways in: eight by-hand endpoints on `doula-api`'s `/api/internal`, called with a minted OIDC token, and `app/`'s `/feedback` pages, gated by `FOUNDER_STAFF_ID`. The second needs the founder signed up into a Practice, which the GLOSSARY rules out: a Platform operator is Staff, Owner, Admin or Doula at no Practice. Once the view also takes acts and raises alerts, the boundary worth having is one the Practice BFF cannot cross at all. This records that boundary and everything the operator's app reads, does, alerts on and records.

## The app and the service

- **The app is `ops/`, its own SvelteKit app, served at `ops.doula.cloud`.** It is not a route group in `app/`. It takes the design tokens from `app/src/lib/styles/app.css` by relative import, as `site/` does.
- **Hosting:** a third Firebase Hosting site, `doula-cloud-ops`, with `ops.doula.cloud` connected and `/api/**` rewritten to `doula-ops` in `us-central1`. The site and the domain are made by hand and listed in `docs/infrastructure.md` under "What stays by hand". There is no PR preview for `ops/`, because a preview would sign in against the live operator tenant.
- **The service is `api/cmd/ops`, a second binary in the same Go module, deployed as Cloud Run `doula-ops`.** Being in the same module puts it under `go test ./...` and `covcheck` with no gate change. It has its own wiring and does not import `doula-api`'s `package main`. Anything both need moves into `api/internal/`. Runtime service account `doula-ops-runtime@`, `min-instances=0`, ADR-0034's `ignore_changes` split, and its own `*sql.DB` pool capped low, since it shares `db-f1-micro`'s `max_connections` with `doula-api`. Ingress is `all` with an `allUsers` invoker, because the Hosting rewrite needs both ([ADR-0056](0056-the-hosting-rewrite-stays-and-a-google-run-caller-on-the-direct-path-is-an-accepted-residual.md)'s residual, accepted again).

Decided on [What is the ops backend service, and where is its database boundary? (#2415)](https://github.com/markgoho/doula-cloud/issues/2415).

## Signing in

Decided on [How does a Platform operator sign in to ops.doula.cloud? (#2414)](https://github.com/markgoho/doula-cloud/issues/2414).

- **An operator tenant in the `doula-cloud` Identity Platform project.** Operator logins live apart from Staff. The ops service verifies tokens with a tenant-scoped client, so it refuses every Staff token. Login accounts are made by hand in the tenant; there is no signup route ([ADR-0049](0049-doula-cloud-is-one-person-by-design-and-no-night-needs-him.md)).
- **`doula-api` refuses any token whose `firebase.tenant` is set**, at session create and at reauth. Without that, it would mint a Staff session from an operator's token.
- **TOTP is required, never SMS**, as for Staff ([ADR-0026](0026-two-populations-two-sign-in-methods-and-one-token-table.md)). It is enforced twice: the tenant's `mfaConfig` is `MANDATORY` with TOTP, and the ops service refuses any session without a TOTP second factor.
- **No `staff` row and no Practice.** An `operators` row, keyed by the tenant uid and holding name and email, is made at first sign-in.
- **Session:** 1 hour idle, 8 hours absolute, in its own `operator_sessions` table. It is not `sessions`, because `app_runtime` can insert there and could mint an operator session.

## The database boundary

- **A Postgres role `ops_runtime`, which `doula-api` never holds.** Its login is a password user made by hand, with the DSN in a Secret Manager secret only `doula-ops-runtime@` can read.
- **`ops_runtime` has no table grants and no policies.** Every read and act is a `SECURITY DEFINER` function called from a handler, with `search_path` set and `pg_temp` last, `EXECUTE` revoked from `PUBLIC` and granted to `ops_runtime` alone, in the same migration transaction. Each returns only the columns a screen prints.
- **No function body reads `clients`.** That makes "never a Client record" something a reviewer can check. The one exception is below, under Feedback.
- **Two direct grants**, on tables with no Practice in them: `operator_sessions`, and `operator_acts` with `operators` (see "The record").
- **No existing policy changes.** RLS stays whole for every other reader. A `TO ops_runtime USING (true)` policy is not used anywhere.
- **Two guards fail the build.** A Go test asserts `app_runtime` and `app_runtime_login` are not members of `ops_runtime`. A Go test over `pg_proc` lists every `SECURITY DEFINER` function each of `app_runtime` and `ops_runtime` may execute, and fails on any function that is not on its list. That catches a forgotten `REVOKE ... FROM PUBLIC`.
- **What it does not fence.** `practices` has no RLS, and `doula-api` already reads any Practice's ledger by setting `app.current_practice_id` itself. So `ops_runtime` fences the operator's own functions and acts: Feedback, every outbox, the cross-Practice reads, and the eight acts below. It does not fence Practice data in general, and this ADR does not claim it does.

**On [ADR-0037](0037-the-internal-boundary-is-a-caller-identity-not-a-shared-secret.md).** That ADR rejected a second Cloud Run service because it bought only "what one token check buys". Here the second service buys what no token check can: a database role the Practice BFF does not hold, and operator tokens verified in a process that serves no Practice route. ADR-0037 is not amended. Its reasoning still holds for the Scheduler and Tasks callers that stay on `doula-api`.

## What ops shows

Decided on [What does the ops app show? (#2413)](https://github.com/markgoho/doula-cloud/issues/2413), which holds the full field list and the source of each field.

- **The Practice list:** name and signup date, the Owner's name and email, Staff count, Stripe Connect status, Credits held, Engagements started (lifetime, from the ledger's `consumption` rows, in place of a Client count), the founding grant and who gave it, Deletion state, source once [#1393](https://github.com/markgoho/doula-cloud/issues/1393) lands, and open Feedback count. A finalized Deletion moves the Practice to a separate Deleted list.
- **The monthly read:** [#1394](https://github.com/markgoho/doula-cloud/issues/1394)'s Credits bought and Credits spent, per Practice with totals, chosen by month and defaulting to the last closed month. There is no running total for the current month anywhere, because a closed month never changes and a figure that moves daily becomes a goal ([ADR-0046](0046-the-product-counts-what-a-practice-did-and-never-who-she-is.md)). This supersedes ADR-0046's one sentence calling the read an internal endpoint "called by hand", and that ADR carries a note saying so.
- **Feedback**, as [#1526](https://github.com/markgoho/doula-cloud/issues/1526) shows it today: the list and each piece, with the sender's name and email.
- **The System section:** Stripe mode and platform account id; per webhook route (Stripe platform, connect and account, Mailgun), the last arrival and the last 7 days' refusals; the last arrival of each handled Stripe event type, with a type never seen marked as such; the Stripe events Stripe reports undelivered in the last 7 days; per outbox, pending and dead-lettered counts and the oldest pending age; and the pilot gate and allowlist, read only.
- **The two billing reads** that are endpoints today: dormant Practices (the yearly escheatment list) and the monthly read above.
- **The Operator log:** every row of `operator_acts`, newest first.

**What no screen shows:** no Client name, count, due date, note, Visit, Message, Contract, Invoice or payment; no named Staff use, so no last sign-in and nothing per Staff member, and the Owner appears only as a contact; no subscription state or billing mode, since nothing renews; no money moved through a Practice's Connect account.

**The Feedback exception.** The only data in ops that can name a Client is what that person sent to DoulaCloud herself, through Feedback. It is a message to DoulaCloud, not a record a Practice holds. It is the one `ops_runtime` function allowed to return a person's words, and reading one piece is recorded.

## What ops does

Decided on [Which actions can the ops app take? (#2416)](https://github.com/markgoho/doula-cloud/issues/2416).

**Ops takes the eight acts that move off `doula-api`, and no others.** Their SQL becomes `ops_runtime`-only functions and the grants they used are revoked from `app_runtime`. `doula-api`'s `/api/internal/**` keeps only what Cloud Scheduler and Cloud Tasks call. No act has two ways in.

| Act | Gate | Check page states |
|---|---|---|
| Refund | session + fresh TOTP | Practice, Credits, amount back; the requesting Owner and the date her request arrived, both required |
| Dormant Practices | session | (read only) |
| Monthly Credits | session | (read only) |
| Founding grant | session | Practice, and the amount sized from its roster, never typed in |
| MFA support-clear | session + fresh TOTP | the runbook's conditions: a live video call and an ID match |
| Pilot allowlist add | session | the address; add only |
| Pilot gate open | session + fresh TOTP | the gate opens for everyone, permanently ([ADR-0043](0043-a-date-is-a-trigger-only-when-someone-set-it.md)) |
| Legal publish | session + fresh TOTP | document, version, material or not, how many Owners are emailed, effective date |

- **Every act is confirmed on a check-your-answers page** ([ADR-0021](0021-govuk-is-the-reference-for-service-patterns.md)) that states its exact effect.
- **A fresh TOTP code** is asked for on that page for the four acts that move money, change a person's sign-in, or cannot be undone for everyone. A session cookie alone cannot take them.
- **The actor comes from the session**, never from the request body. Today's `grantedBy`, `addedBy`, `operator` and `publishedBy` body fields go away.
- **Every operator account may take every act.** Adding a person to the tenant is the deliberate, scoped act.
- **No act reads or changes a Client record.**
- `doula-ops-runtime@` becomes the most privileged identity in the project. It reads the Stripe platform secret and holds Identity Platform admin on the Staff pool. That fits it being the operator's service.

**Rejected acts:** resending an invitation (a Practice's own act); replaying a refused webhook (Stripe's Dashboard resends, and replay would mean storing payloads); starting or canceling a Practice's Deletion (the Owner's act, [ADR-0031](0031-practice-deletion-is-a-thirty-day-window-then-the-erasure-cascade.md)); retrying dead-lettered outbox rows ([ADR-0010](0010-notification-email-delivery-is-an-outbox-not-in-request.md) marks them failed for good); and support access to a Practice, which would be a new way into Client records and is out of scope ([#2435](https://github.com/markgoho/doula-cloud/issues/2435)). Ops gains more acts as a real need for each one shows up, each by its own decision.

## What ops alerts on

Decided on [What does ops alert on, and how does an alert reach the founder? (#2417)](https://github.com/markgoho/doula-cloud/issues/2417).

Every alert goes to the founder's phone and keeps the waking-hours window of [#1403](https://github.com/markgoho/doula-cloud/issues/1403), which lands first and whose channel and `terraform/` files these reuse. A failure found at night stays open and reaches the phone when the window opens. Each alert closes itself when its condition clears.

| Signal | Threshold | Read by |
|---|---|---|
| Stripe events Stripe still reports undelivered | 1 or more, older than 15 minutes | the `doula-ops` check |
| Livemode refusal on a Stripe webhook route | any 1 | the check |
| An outbox row due and not claimed | due more than 30 minutes ago | the check |
| An outbox row dead-lettered after its retries ran out | any new one | the check |
| `doula-api` 5xx | 5 or more in 15 minutes | Cloud Monitoring |
| The check's heartbeat missing | 1 hour | Cloud Monitoring |

- **`doula-ops` gets its own Cloud Scheduler caller.** Every 15 minutes Scheduler calls a check on `doula-ops` with OIDC. The check reads the same `ops_runtime` functions as the System section, so the screen and the alert cannot disagree. It writes one structured log line per failing signal, and a Cloud Monitoring log alert on that line goes to the phone. Each run also writes a heartbeat line.
- **On screen only:** silence per route or event type, signature refusals, Mailgun refusals, handler drops, and rows dead-lettered at once because of their own data.
- **Thresholds start low on purpose**, to see every failure and trace its cause, and are loosened by an ordinary PR as causes are found. There is no digest and no other channel.
- **To make the outbox alert possible,** a dead-lettered row records why: retries ran out, or its own data could not be sent. Today only `attempt_count` tells them apart.

## The record

Decided on [How is each Platform operator read and act recorded? (#2418)](https://github.com/markgoho/doula-cloud/issues/2418).

- **`operator_acts` is the operator's one record.** It is append-only: `ops_runtime` alone holds `SELECT` and `INSERT`, and no role holds `UPDATE` or `DELETE`. Each row holds the operator, what was done, the subject (a Practice, a Staff member, a Feedback piece, or none), when, and the act's details. `step_up_at` is set for the four acts that need a fresh TOTP code. A refund also holds the requesting Owner's `staff_id` and the date her request arrived.
- **Sign-in, sign-out and a refused step-up are rows too.** `operator_sessions` rows disappear when they expire, so they cannot be the record.
- **A read is recorded only when the page shows a person:** an Owner's contact, or a Feedback piece. The row holds ids and a time, never content. Aggregate screens record nothing.
- **Today's free-text actor columns become a key to the act row:** `credit_ledger.granted_by`, `pilot_allowlist.added_by`, `pilot_gate.opened_by`, `legal_version_publications.published_by` and `staff_auth_events.actor_operator`. A refund's ledger row gets the same key, so a refund records who took it for the first time.
- **A refund, a founding grant and a support-clear also write the Practice's own Activity**, under the fourth actor kind `operator`, shown as the person's name with DoulaCloud. That is ADR-0022's amendment of 2026-10-31.
- **Kept forever**, through Practice Deletion, like `credit_ledger` and `activity`.
- **Webhook arrivals and refusals are a separate table**, written by `doula-api` as `app_runtime`: route, event id and type where it parses, outcome, and time, with no payload. They are facts about the service, not acts by a person. A daily sweep deletes rows older than 90 days.

## Retired

`FOUNDER_STAFF_ID`, `staffauth.FounderOnly` and `app/`'s `/feedback` routes go once Feedback works in ops. Until then each Feedback piece is read on its private GitHub issue. [#1607](https://github.com/markgoho/doula-cloud/issues/1607) was closed as not planned. The runbooks that give the `gcloud auth print-identity-token` form change in the build that moves each endpoint.

## Considered and rejected

- **A route group in `app/`, gated by `FOUNDER_STAFF_ID`.** It puts the founder inside a Practice, and the process holds every Practice grant.
- **Routes on `doula-api`, served only on the ops host.** The `run.app` URL answers any path and a caller sets its own headers (ADR-0056), so a host check is routing, not a boundary.
- **A separate Go module in `ops/`.** It cannot import `api/internal/*`, needs a second coverage gate and CI job, and buys no boundary the second binary lacks.
- **One image deployed as two services with a mode flag** (ADR-0037's rejected design). Most of the cost, and the ops service would carry every Practice route.
- **Sign-in: the Staff pool with an allowlisted uid** (`doula-api` would still mint a Staff session for the operator); **a separate GCP project** (a second BAA, a second stack, for isolation the tenant already gives); **IAP with a Google account** (cannot sit behind a Hosting rewrite, has no local emulator, and needs a custom OAuth client). The accepted cost of the tenant is that TOTP can be phished. A passkey for the operator is an experiment for later ([#2426](https://github.com/markgoho/doula-cloud/issues/2426)).
- **`TO ops_runtime USING (true)` policies.** A door onto every row of every table they cover.
- **Showing a last Staff sign-in, a billing mode or a subscription state.** ADR-0046 keeps `staff.last_active_at` for the dormancy sweep and export only, and nothing renews.

## Consequences

- The build is filed as tickets under #2409. The skeleton (role, guards, service, app, sign-in) comes first. The System section can ship ahead of the rest for go-live. Each of the eight endpoints moves in its own ticket, with its screen, its actor key and its Activity row.
- Two services now share one small Postgres instance's connections, so both pools are capped ([#2432](https://github.com/markgoho/doula-cloud/issues/2432) for `doula-api`).
- The founder's monthly spreadsheet stays the monthly page. Ops is where its counts are read.
