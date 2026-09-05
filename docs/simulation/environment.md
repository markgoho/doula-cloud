# Where a run runs, and what it cannot show

Settled on [#763](https://github.com/markgoho/doula-cloud/issues/763), under the map [Six months in a sandbox](https://github.com/markgoho/doula-cloud/issues/759).

Four files describe a run. [README.md](README.md) is the **instrument** — what a friction log is and what an entry must carry. [worlds/rooted-birth-collective.md](worlds/rooted-birth-collective.md) is the **World** — who is walking and what they arrive from. [calendar.md](calendar.md) is the **calendar** — how much work there is, when it arrives, and what goes wrong. This file is the **sandbox**: where the run happens, and, just as load-bearing, the list of things that happen there differently from production, or not at all.

## The sandbox is the local stack, on the machine that has the real Chrome

A run stands up `app/compose.e2e.yaml` plus the host processes `app/e2e/stack.ts` sequences: Postgres 16 and `fake-gcs-server` in containers, and the Firebase Auth emulator, goose, the `app_e2e` login role, the Go BFF and the built Svelte app as host processes. Nothing is deployed, nothing touches GCP, and the whole world lives in one Podman volume.

`app/e2e/stack.ts` installs simulated time ([#762](https://github.com/markgoho/doula-cloud/issues/762), [#778](https://github.com/markgoho/doula-cloud/issues/778)) into that same Postgres before a single migration runs: `go run ./cmd/simclock install app` creates the `sim` schema, its one offset row, and `sim.now()`, and points the `app` superuser's `search_path` at it, so all 72 `DEFAULT now()` columns goose is about to create bind to it. `go run ./cmd/simclock grant app_e2e`, run right after the `app_e2e` role is created, points that role's `search_path` at `sim` too and grants it the `USAGE`/`SELECT` `sim.now()` needs to run under a non-superuser connection. Both are idempotent, so a resumed run against a kept volume calls them again for free; `api/internal/simclock` refuses outright if it's ever pointed at a database that already carries goose's migrations without the shim, which is also what keeps it from ever reaching a deployed one — a deployed database always has its migrations applied already.

The other three candidates lose on recorded decisions rather than on cost.

| Candidate | Why it lost |
| --- | --- |
| **A PR preview deployment** | There is no per-PR BFF. `.github/workflows/ci.yml:480` gates every Cloud Run deploy on a push to `trunk`, and a Hosting preview channel reuses `firebase.json`, whose `/api/**` rewrite points at the **production** `doula-api`. A run there would write six simulated months into production Cloud SQL. |
| **A dedicated long-lived sandbox deployment** | The clock. [#762](https://github.com/markgoho/doula-cloud/issues/762) puts a `sim` schema ahead of `pg_catalog` on the login role's `search_path`, which is the search-path shadowing pattern behind CVE-2018-1058; the map already rules it must never reach a deployed role or Cloud SQL. Its other half is a `Clock` **injected** into the BFF at start-up, and Cloud Run offers no such handle. Cost never entered it, so no cost was quoted. |
| **A cheap Linux VM running the same stack** | Only for run one. Stripe's hosted Connect forms CAPTCHA a Playwright-launched Chromium in both headless and headed mode and never CAPTCHA a real profiled Chrome, so `playwriter` against the user's own browser is required ([#236](https://github.com/markgoho/doula-cloud/issues/236), recipe in [`docs/test-plans/connect-onboarding.md`](../test-plans/connect-onboarding.md)) — and Tasha's Practice is created **mid-run**, so that need is not confined to a set-up phase. This is the natural answer for a later run if hosting a run on a laptop turns out to hurt.

## Stripe reaches it, with no tunnel and no variable overridden

`scripts/stripe-listen.sh` forwards all three webhook surfaces — `/api/stripe/webhook`, `/api/stripe/connect-webhook` and the thin-event `/api/stripe/account-webhook` — straight to `127.0.0.1:18080`, signed with the one session secret all three local secrets hold. Outbound API calls and test-clock advances are ordinary HTTPS out of the machine. Checkout and Account Link `return_url`s are browser redirects, so a `localhost` `APP_BASE_URL` is what the browser follows.

So **`APP_BASE_URL` keeps its local default and `EXPECTED_ORIGINS` is left alone**. `docs/environment.md`'s tunnel instruction applies to a walk that needs Stripe to reach a public URL; a run does not, because `stripe listen` is that reach.

The one Stripe cost that binds the sandbox is that a **test clock holds at most three Customers** and is **deleted 30 real days after creation** ([#762](https://github.com/markgoho/doula-cloud/issues/762)). The client book, not the Practice count, sizes the number of clocks, and 30 real days is the hard expiry on the Stripe half of a world.

## How a persona receives her email

Settled on [#764](https://github.com/markgoho/doula-cloud/issues/764). Eleven of the product's mail kinds are the only way something reaches a person, and several are the first thing that ever happens to her — a Staff invitation, a Client's portal invitation, a verification link. The nine walks got past this by reading the token off the pending outbox row (`readStaffInviteToken` in `app/e2e/stack.ts`), which proves a code path and skips the act.

**Mail goes to the sandbox mailbox** (`app/e2e/mailbox.ts`), a Bun host process `stack.ts` starts beside the BFF and the Auth emulator. It answers the one endpoint `mail.MailgunSender` posts to, `POST /v3/<domain>/messages`, so the message it holds is the message the product built — subject, From, Reply-To and body, rendered by the real worker. `MAILGUN_API_BASE` (`api/main.go`) is the whole seam; `MailgunSender.BaseURL` is the field its own httptest tests already use, and this only exposes it to the environment.

**Every address is `<name>@sim.doula.cloud`**, a subdomain of our own domain with no MX — the same shape #743 used for `bounce-test.doula.cloud`. The mailbox holds mail for any domain, so the convention is for the reader's benefit, not the harness's: an address in a log says at a glance that it was a simulated person. The mailbox is keyed on the bare address, case-insensitively, so a persona typing her own address into a form types it however she likes.

**She reads it in a browser.** `GET /inbox/<address>` is an inbox page and each message is a page of its own, with the URLs in the body rendered as links — so following an invitation is a click out of a message, and a broken link is found by clicking it. That is the observed act. `GET /api/messages?to=…` returns the same messages as JSON; that is the harness asserting, and it is never itself an act in a log.

**The inbox shows arrival order and the run's own clock label, never a wall-clock time.** The mailbox receives in real time while the run is six simulated months in, so a real timestamp would be a lie a persona could narrate confusion about. The harness POSTs `/api/clock` with a label on each jump ([#762](https://github.com/markgoho/doula-cloud/issues/762)) and every later message carries it.

**Mailgun's event side is played by the mailbox.** `POST /api/delivery-event` signs a `failed`/`permanent` or `complained` payload with `MAILGUN_WEBHOOK_SIGNING_KEY` and posts it to the BFF's `/api/mailgun/webhook`, which writes the `email_suppressions` row that makes `mailsuppress.Sender` refuse every later send to that address (ADR-0029). This is the half a missing CLI forwarder used to cost us, and it is the loop #743 proved live against the deployed service. **Run one uses both**, settled at [the calendar](calendar.md#what-goes-wrong-and-how-often) ([#765](https://github.com/markgoho/doula-cloud/issues/765)): a Client's address is typed wrong and hard-bounces in month 2, and a Client marks a notification as spam in month 5. Each suppresses her silently, and nobody at the Practice is told.

`app/e2e/mail-delivery.e2e.ts` walks the whole of it and runs in CI, so the mailbox cannot rot between runs.

## What the sandbox cannot show

Every environment loses something. A run that quietly cannot see a thing is worse than one that knows it cannot, so these are written down rather than remembered. Each is either accepted with its owner named, or it is somebody's ticket.

| Not observable | Why | Owner |
| --- | --- | --- |
| **When a notification arrived** | Nothing fires by itself locally; the harness drains the `process-*` endpoints after each jump. Already an inadmissible claim in [README.md](README.md#claims-that-are-never-admissible). | [#762](https://github.com/markgoho/doula-cloud/issues/762), accepted |
| **Hosting's `/api/**` cookie stripping** | Locally vite proxies `/api` same-origin, so the rewrite that strips every cookie but `__session` is never exercised. | [#138](https://github.com/markgoho/doula-cloud/issues/138), accepted |
| **Anything Firebase itself would email** | The Auth emulator sends no mail; verification and password-reset messages exist only in its logs. TOTP across the cast is already out of run one. | [#761](https://github.com/markgoho/doula-cloud/issues/761), accepted |
| **Mailgun's own delivery machinery** | Reputation, spam placement, greylisting, the real bounce a real bad address produces. Nothing in a run leaves the machine. The bounce and complaint *webhooks* are observable — the mailbox signs and posts them itself, which is what a missing CLI forwarder cost us. | [#764](https://github.com/markgoho/doula-cloud/issues/764), accepted |
| **Real GCS, Cloud Tasks, Cloud SQL, Secret Manager** | Faked, no-op, containerised, or read from the environment. A permissions or quota failure in any of them is invisible to a run. | Accepted |
| **Production-shaped timings** | See below. | Accepted |

### Every timing is a lower bound, and the bands do not move

The BFF, Postgres and the browser are all on one machine with no network between them, no CDN, no Cloud Run cold start and no Cloud SQL hop. So a local measurement is the **fastest** the product will ever be for that act.

The consequence is asymmetric, and it is a rule, not a caveat: **a run can prove an act too slow; it can never prove one fast enough.** An act inside the 1 s band locally is unproven in production, and no entry may say otherwise. An act over 10 s locally is a finding with no discretion, exactly as [README.md](README.md#performance-is-an-entry-with-a-number-on-it) says — a local machine being generous makes a slow local reading worse news, not better. The 1 s and 10 s bands stay where they are; moving one would silently reclassify every entry in every past run.

Every run README carries the line that its timings are local lower bounds.

## A run is resumable, and a world outlives it until the next run

Today `app/e2e/stack.ts:279` tears the stack down with `compose down -v`, which destroys the Postgres volume and the in-memory fake GCS bucket; the connected Stripe account made during a session does not survive it either. That makes a run one unbroken sitting, which twenty-two people walking six months will not reliably fit inside.

**A run can be stopped and resumed.** Everything stateful already survives if the volume is kept — the rows, `sim.offset_row`, and Stripe's test clocks, which live at Stripe. What a resume rebuilds is browser contexts and sign-ins, which the harness must do anyway: a persona holds her session until a jump longer than 12 h and then signs in again, because the Firebase ID-token verifier is hard-wired to real time. So resume costs a keep-the-volume path in `stack.ts` and a documented restart, and it buys back jump granularity as an observational choice rather than a scheduling constraint.

**A world is kept until the next run starts, and never longer than 30 real days.** A filed finding is read and re-read in the days after a run, and reproducing one against the world that produced it is worth a volume. Past 30 days Stripe has deleted the test clocks, so half the world silently no longer exists — a world older than its clocks is a trap. The run README records that the world was kept and when it expires.

## The ceiling

**Browsers are not the ceiling.** Measured on the machine a run will use (Apple M4 Pro, 14 cores, 24 GB): twenty-two Chrome contexts in one browser stand up in 2.9 s and hold 608 MB with a blank page each. The cast is twenty-two named people; real application pages cost more than a blank one, but not the order of magnitude that would make this tight.

**Personas are interleaved on one clock, not run as twenty-two independent sessions.** The map wants to see what breaks when other people are in the system at the same time, and that is satisfied by other people's data and other people's live sessions existing concurrently — which interleaving gives. Genuine simultaneity is reserved for **named probes**: two cast members acting on the same object at the same moment, written into the run as deliberate acts. [#203](https://github.com/markgoho/doula-cloud/issues/203) already puts synthetic concurrency and stress runs out of scope, and unbounded parallel personas is that by another name.

So the binding ceiling is not memory or CPU. It is **how many agent sessions a run is willing to spend**, which is a budget the harness owns, not a property of this environment.
