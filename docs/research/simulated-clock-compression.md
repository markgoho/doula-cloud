# How six months of clock advances, and what a run costs in wall-clock time

Research for GitHub issue [#762](https://github.com/markgoho/doula-cloud/issues/762), the feasibility gate of map [Six months in a sandbox (#759)](https://github.com/markgoho/doula-cloud/issues/759). Question: by what mechanism does simulated time advance across the whole stack — Go, Postgres, the scheduled work, Stripe, and the session — and what does one six-month run cost in real minutes?

Every claim below was verified by running something against the code in this worktree, a live `postgres:16-alpine` container, a Linux container, or the Stripe Sandbox (`acct_1U9c2i1rKoEV0BlC`). Where a claim rests on reading rather than running, it says so.

Prose here is written unwrapped, per the repo's markdown rule, which differs from the older files in this directory.

## Recommendation

**A two-part injected clock that jumps: a Go `Clock` seam in the BFF, and a `sim.now()` shim ahead of `pg_catalog` on the database role. One offset value, held in a Postgres row, read by both. The harness advances it, then drains every `process-*` endpoint.**

Concretely, four pieces:

1. **An offset row in the database.** `sim.offset_row(delta interval)`, one row. Advancing six months is `UPDATE sim.offset_row SET delta = ...`, which measured **0.644 ms**.
2. **A `sim.now()` function that reads it**, installed into a `sim` schema that sits ahead of `pg_catalog` on the login role's `search_path`. This makes every one of the 72 `DEFAULT now()` columns and every `now()` in a handler's SQL read simulated time, with **zero schema changes and zero SQL rewrites** — provided the shim exists before goose runs. See Area 2 for the experiment that establishes each half of that sentence.
3. **A `Clock` seam in the BFF** that reads the same offset (or is told it by the harness) and is threaded through the 27 non-test `time.Now()` calls. Deliberately *not* a process-wide fake, because one caller — the Firebase Admin SDK's ID-token verifier — must stay on real time (Area 7).
4. **A harness drain.** After each jump, POST all 13 `process-*` outbox endpoints until no rows claim. This is exactly what Cloud Scheduler does in production; nothing else fires locally (Area 5).

Stripe advances alongside, on its own test clocks, one per connected account (Area 6).

### Rejected alternatives

**`libfaketime` (or any faked system clock under the container stack) — rejected on two independent grounds, both measured.**

First, **it does not move a Go binary.** In a `debian:bookworm-slim` container with `faketime` installed, the same `+180 days` invocation moved `date` and left a compiled Go program on real time:

```
REAL
2026-09-05T18:18:51Z   <- Go binary
2026-09-05T18:18:51Z   <- date
FAKE180
2026-09-05T18:18:51Z   <- Go binary under faketime '+180 days'
2027-03-04T18:18:51Z   <- date under faketime '+180 days'
```

The BFF is the component whose clock matters most, and it is the one component `libfaketime` cannot touch.

Second, **there is almost no container stack left to fake.** `app/compose.e2e.yaml` runs exactly two services, `db` and `gcs`; its own header comment says migrations, the `app_e2e` login role, and the Go BFF "all run as host processes now (see `app/e2e/stack.ts`)", and `app/e2e/stack.ts:302` starts the Firebase Auth emulator as a host process too (`bunx firebase-tools emulators:start`). So a container-level clock fake would reach Postgres — the one component the `sim.now()` shim already handles correctly — and nothing else. On the local host it would also have to work on macOS, where `DYLD_INSERT_LIBRARIES` is stripped for system binaries under SIP — that last clause is reasoned, not run, and it does not carry the rejection on its own.

**Backdating written rows — rejected, and the map already rejected it.** It shifts nothing: a row inserted with an old `created_at` still fails a `WHERE next_attempt_at <= now()` claim against a real Postgres clock, so no behavior fires. The map's settled note ("Time actually elapses") rules this out on its own; the mechanical reason is in Area 2.

**A Go-only clock seam, with Postgres left on real time — rejected as the named failure mode.** 12 of the 13 outbox claim queries compare `next_attempt_at <= now()` in SQL, and `api/internal/outbox/outbox.go:198` documents that this is deliberate: "The claim query compares against Postgres's own `now()`, not `w.Now()`". A Go seam alone moves the app's idea of the date and leaves every scheduled thing frozen — rows dated in the future relative to a database that will not claim them.

### Which one catches more bugs

The map's test. The injected seam wins on this too, not only on feasibility. `libfaketime` is an environment trick that cannot be asserted about in a unit test; the seam is the same object a Go test injects a fixed time into, so a time bug found in a run can be reproduced by a test in `api/`. And because the seam is *injected rather than global*, it exposes a class of bug a global fake would hide: a code path that reached for `time.Now()` directly instead of the clock shows up as a wrong date, rather than being silently corrected by the fake.

## Area 1 — There is no clock seam today

**Partly true, and the ticket's numbers are wrong in a way that changes the estimate.**

The ticket says `Clock` appears in two places against "roughly 295 raw `time.Now()` calls across `api/internal`". Measured:

| Measure | Count |
| --- | --- |
| `time.Now()` in `api/internal`, **including tests** | 284 |
| `time.Now()` in `api/internal`, **excluding tests** | 25 |
| `time.Now()` in the whole `api/` tree, excluding tests | 27 |
| `time.Since(` / `time.Until(` in `api/`, excluding tests | 2 |

The ~295 figure counted test files. Production code has **27** call sites, and they cluster: 13 in `internal/staffauth`, 3 in `internal/authntest`, 2 each in `internal/offer` and `internal/billing`, and one each in `tasknudge`, `session`, `portalinvite`, `client`, `authn`. This is a morning's work threading a `Clock`, not a cross-cutting refactor.

There are also **three** existing seams, not two. Alongside `api/internal/sitebuild` (three files) and `api/internal/push/vapid`, `api/internal/outbox/outbox.go:64` already carries `Now func() time.Time` on `Worker`, consumed at `outbox.go:213`. Every outbox worker therefore already takes an injectable clock — the seam the harness needs on the send side exists.

## Area 2 — A Go clock seam does not reach the database

**Confirmed, and the fix is cheaper than the ticket assumes.**

The counts, in `api/db/migrations`:

| Pattern | Count |
| --- | --- |
| `now()` | 74, across 37 files |
| of which `DEFAULT now()` | 72 |
| `CURRENT_TIMESTAMP` | **0** |
| `CURRENT_DATE`, `CURRENT_TIME`, `LOCALTIMESTAMP`, `LOCALTIME`, `clock_timestamp`, `statement_timestamp`, `transaction_timestamp` | **0** |

In non-test Go, `now()` appears **85 times** in handler SQL, concentrated in `staffauth` (12), `offer` (11), `ratelimit` (4), `engagementrequest` (4), `payments` (3), `idempotency` (3), `authmail` (3).

That every one of these is the *function* `now()` and none is the *keyword* `CURRENT_TIMESTAMP` is the load-bearing fact, because a function resolves through `search_path` and a reserved keyword does not. Verified against a live `postgres:16-alpine`:

```
--- CURRENT_TIMESTAMP under shim (keyword, not shadowable) ---
     current_timestamp_kw      |            now_fn
-------------------------------+-------------------------------
 2026-09-05 18:16:29.172479+00 | 2027-03-04 18:16:29.172479+00
```

Three further facts, each of which the mechanism depends on, each run rather than reasoned:

**`pg_catalog` is searched implicitly first, so the default `search_path` defeats the shim.** With the shipped default (`"$user", public`), `SELECT now()` returned real time even with `public.now()` defined. Only after `SET search_path = public, pg_catalog` — naming `pg_catalog` explicitly, and last — did it return the shifted value:

```
--- after shim, default search_path ---
 2026-09-05 18:16:29.169013+00
--- explicit search_path = public, pg_catalog ---
 2027-03-04 18:16:29.169236+00
```

**`DEFAULT now()` binds the function at `CREATE TABLE` time, so the shim must be installed before goose runs.** Table `a` was created before the shim existed, table `b` after. Postgres stored their defaults differently and inserted differently:

```
 relname |          pg_get_expr
---------+-------------------------------
 a       | pg_catalog.now()
 b       | now()

 t |              at
---+-------------------------------
 a | 2026-09-05 18:16:29.170772+00
 b | 2027-03-04 18:16:29.171077+00
```

This is the difference between rewriting 72 migration defaults and changing nothing: install the shim as a pre-migration step on a fresh database, and all 72 defaults resolve to it. It also means the shim **cannot** be retrofitted onto an already-migrated database without rewriting those defaults — a fact the harness's setup order has to respect.

**Role-level `search_path` survives the connection pool.** `ALTER ROLE app SET search_path = sim, public, pg_catalog` applied to a brand-new connection (`SHOW search_path` → `sim, public, pg_catalog`), a table created on that connection stored the unqualified `now()`, and an insert after `UPDATE sim.offset_row SET delta = '90 days'` landed 90 days out. Nothing in `api/` or `app/e2e` sets `search_path` on a connection that would override this; the only four occurrences are `SET search_path` clauses *inside* function bodies in migrations `00003`, `00010`, and `00067`, which pin `public` and would need `sim` prepended if those functions are to see simulated time.

The offset is read from a one-row table rather than a GUC on purpose: a GUC is per-connection and the pool would defeat it. The function is declared `STABLE`, so it is evaluated once per statement, matching `now()`'s own semantics.

Cost to advance: **0.644 ms** for the `UPDATE`, measured with `\timing on`.

## Area 3 — Alternatives to a seam

Answered in **Rejected alternatives** above: `libfaketime` measured not to move a Go binary and to have almost no container stack left to act on; backdated rows shift nothing; a Go-only seam is the documented failure mode. The seam plus the `sim.now()` shim is the recommendation, and it is also the one that catches more bugs, for the reason given there.

## Area 4 — Jump or scale

**Jump. Scaling is not on the table, and the 4:1 framing does not apply.**

Nothing in the recommended mechanism runs continuously: `sim.now()` returns `pg_catalog.now() + delta`, so between jumps simulated time advances at exactly 1:1 with real time, and a jump is a single `UPDATE`. A *scaling* clock would need the offset to grow as a function of real elapsed time, which is expressible (`delta` as a rate) but buys nothing — the product has no behavior that depends on time passing smoothly, only on comparisons against a current instant.

So the compression ratio is not a property of the clock, and "six months in ten minutes" is not the question. The cost model is additive:

```
wall_clock =  Σ (observed UI acts × seconds per act)          <- the volume ticket supplies the act count
            + Σ over jumps of:
                 0.001 s   (offset UPDATE)
                 + 0.4 s × (Stripe test clocks) + ~6 s   (advances are async: issue all, wait once)
                 + drain   (13 process-* POSTs, repeated until no rows claim)
                 + one sign-in per persona active after the jump   (Area 7)

where Stripe test clocks = Σ over connected accounts of ceil(Clients with a Stripe Customer ÷ 3)
```

Three numbers a later ticket multiplies out are measured here: the offset update at **0.644 ms**; a Stripe test-clock advance at **~6 s**, asynchronous, so clocks advanced together share one wait; and **three Customers per test clock**, which is what makes the client book — not the number of Practices — the driver of the Stripe term (Area 6). Everything else is act count and per-act latency, which this ticket does not own.

The practical consequence is that **the number of jumps, not the size of them, is the budget**. Advancing 180 days in one jump costs the same as advancing 1 day, so the run's granularity should be chosen by what the personas need to observe, not by clock cost. A run with a daily jump for six months pays 180 × (Stripe advances + drain + sign-ins); a run that jumps only between acts pays far less. The map's "total wall-clock time is an input of the system" is satisfiable directly: jump granularity is that input.

## Area 5 — Scheduled work does not run locally

**Confirmed. Nothing fires by itself, and the drain is honest.**

There are **13** `process-*` endpoints, all registered in one list at `api/outboxes.go:32`:

| Path | Nudged |
| --- | --- |
| `/api/internal/notifications/process-outbox` (portal invite) | yes |
| `.../process-low-credit-outbox` | yes |
| `.../process-payout-outbox` | yes |
| `.../process-payment-outbox` | yes |
| `.../process-session-notice-outbox` | yes |
| `.../process-staff-invite-outbox` | yes |
| `.../process-offer-outbox` | yes |
| `.../process-engagement-request-outbox` | yes |
| `.../process-staff-token-mail-outbox` | **no** — `outboxes.go:98` records the deliberate choice |
| `.../process-staff-email-change-outbox` | **no** |
| `.../process-mfa-recovery-outbox` | yes |
| `/api/internal/clients/process-erasure-outbox` | yes |
| `/api/internal/site/process-build-outbox` | yes (and the one with no `Door`) |

**What each gates on:** 12 of the 13 claim rows with `WHERE status = 'pending' AND next_attempt_at <= now()` — `offer/outbox.go:91`, `payments/payment_outbox.go:78`, `payments/outbox.go:72`, `engagementrequest/outbox.go:102`, `authmail/authmail.go:98` and `:205`, `portalinvite/outbox.go:57`, `mfarecoverymail/outbox.go:83`, `staffinvite/outbox.go:108`, `sessionnotice/outbox.go:209`, `client/erasure_outbox.go:67`, `billing/outbox.go:109`. That `now()` is Postgres's, per `outbox/outbox.go:198`. The shim moves all twelve at once; nothing else does.

The thirteenth is the exception that proves the two-part recommendation necessary. `sitebuild/outbox.go:72` passes `w.Now()` into its claim query as a **parameter**, and `:61` compares `w.Now().Sub(oldest)` against its coalesce window — so the site-rebuild outbox is gated by the *Go* clock, not Postgres's. Twelve endpoints need the shim, one needs the seam, and the harness needs both moved together or the thirteen fall out of step with each other.

**The nudge does not fire locally, and it is not a lie about the product — but only if the harness drains.** `api/internal/tasknudge/fake.go` shows `FakeEnqueuer.Enqueue` appends the `OutboxType` to a slice and returns. It records; it never issues an HTTP request. So a nudge scheduled at +24h fires *never* locally, not late. What must simulate it is the harness POSTing the endpoints itself.

That drain is sound rather than a lie, because in production Cloud Scheduler POSTs the same paths on a cadence and the Cloud Tasks nudge (ADR-0013) only shortens the wait — `api/internal/tasknudge/tasknudge.go:3` says the nudge "calls the same `process-*` endpoint Cloud Scheduler already" calls. A harness that drains every endpoint after each jump reproduces the scheduler exactly. What it does **not** reproduce is nudge *latency* — production delivers a nudged notification in seconds and an un-nudged one within the scheduler's cadence, and a drain flattens that distinction. A run therefore cannot find a bug about a notification arriving too late, and the friction log must not claim otherwise.

One time-dependent behavior deserves flagging because the drain will reach it: `client/erasure_outbox.go` runs its Redaction Job "once Stripe's 90-day floor has passed" (`api/outboxes.go:122`). Under a six-month simulated span with the shim, that floor genuinely passes — which is exactly the kind of behavior the map exists to exercise, and would never fire under backdated rows.

## Area 6 — Stripe keeps its own time

**Yes, Stripe can participate — conditionally.** Verified against the Sandbox on connected account `acct_1U9c2i1rKoEV0BlC` (`charges_enabled: true`, `details_submitted: true`), not from documentation.

**Test clocks apply to connected accounts.** Creating one with the `Stripe-Account` header succeeded:

```
$ stripe test_helpers test_clocks create --frozen-time 1767225600 --name clock-probe \
    --stripe-account acct_1U9c2i1rKoEV0BlC
{ "id": "clock_1UCO9D1rKoEV0BlCFJjnIGq8", "frozen_time": 1767225600, "status": "ready" }
```

**Invoices raised on the connected account inherit it.** A Customer created on that account with `test_clock` set, then an Invoice with `collection_method=send_invoice` and `days_until_due=30` — the same shape `api/internal/payments/stripe_api_client.go:254` and `:280` raise — came back dated by the clock and not by real time. Real time at the moment of the call was `1788632223` (2026-09-05); the invoice reported:

```
"created": 1767225600,      <- 2026-01-01, the clock's frozen_time
"due_date": 1769817600,     <- 2026-01-31, 30 days after it
"webhooks_delivered_at": 1767225600
```

**Advancing moves the invoice's own lifecycle timestamps.** After advancing the clock to `1771113600` (2026-02-15), finalizing that first (zero-total) invoice recorded `"finalized_at": 1771113600` and `"paid_at": 1771113600` — the simulated instant, not the real one. It was marked paid at finalization only because its total was $0; the invoice item had not attached to it. The second invoice below is the real-money case.

**Cost of an advance: about six seconds, and it is asynchronous.** Two advances were timed: 45 simulated days returned in `0.417 s` with `status: advancing` and reached `ready` **6 s** later (`1788632245` → `1788632251`); 61 simulated days returned immediately and reached `ready` **4 s** later (`1788632643` → `1788632647`). The wait is not proportional to the span. Because the `advance` call returns straight away, *n* clocks can be issued together and then polled once — *n* clocks cost roughly *n* × 0.4 s to issue plus **one** ~6 s wait, not *n* × 6 s.

**A due date passing changes nothing on Stripe's side. Observed, not assumed.** A second Customer on the same clock was invoiced for $50,000 (`collection_method=send_invoice`, `days_until_due=30`) at simulated 2026-02-15, finalized to `"status": "open"`, `"total": 50000`, `"due_date": 1773705600` (2026-03-17). The clock was then advanced to `1776384000` (2026-04-17), a month past the due date. The invoice re-read as:

```
"status": "open",  "due_date": 1773705600,  "total": 50000,  "attempt_count": 0
```

The events the advance emitted were `test_helpers.test_clock.advancing`, `test_helpers.test_clock.ready`, `invoice.finalized` and `invoice.updated` — **no overdue event of any kind**. Stripe has no `overdue` invoice status and takes no action when a `send_invoice` due date passes.

The app has none either. The word `overdue` (and `days_overdue`, `is_late`) appears **zero times** across `api/internal`, `api/db/migrations`, and `app/src` outside tests; `api/internal/payments/practice_invoices.go:33` surfaces Stripe's own `Status` string straight through. So "an invoice goes overdue" — the map's own headline example of elapsed time — is **not a behavior that exists anywhere in the system**. A run will observe an invoice sitting `open` past its due date and nothing happening. That is a legitimate finding, and it is worth filing now rather than discovering mid-run.

**The conditions.** Four, and each is real work for the build ticket:

1. **The app must attach the test clock at Customer creation.** `test_clock` is settable only when a Customer is created, so the product's Customer-creation path needs a sandbox-only seam that passes it. This is a test-only parameter in production code, which is the price of Stripe participating at all.
2. **A test clock holds at most three Customers.** Measured: the fourth `customers create` against `clock_1UCO9D…` was refused with *"You already have the maximum number (3) of customers allowed on this test clock."* This is the fact that sizes the Stripe cost, and it is not the account count — it is **`ceil(Clients with a Stripe Customer ÷ 3)` clocks per connected account**. A 14-doula agency with sixty clients on the books needs twenty clocks for that one Practice. All of them must be advanced in lockstep with `sim.offset_row`, and (per the timing above) they can be issued in parallel, so the wall-clock cost stays near one ~6 s wait plus ~0.4 s per clock issued.
3. **Drift between Stripe's clocks and the database offset is the failure mode** — an invoice `due_date` computed by Stripe against its clock, compared by the app against a differently-offset `now()`. With twenty-plus clocks per Practice, "advance them all, then verify every one reached `ready` before resuming" is a required harness step, not a nicety.
4. **A test clock is deleted after 30 real days** (`deletes_after` was exactly 2,592,000 s after `created`). A run must complete, and its Stripe state must be read, within that window. Generous for a run measured in hours; fatal for a "standing world" expected to persist across months of real calendar.

## Area 7 — Sessions expire on real time

**A persona can hold a session across simulated months — conditionally, and the condition is realistic rather than awkward: she signs in again after each jump longer than 12 hours.**

The mechanism, read from the code:

- **The ID token is verified exactly once, at sign-in.** `api/internal/session/session.go:59` calls `verifier.VerifyIDToken`. After that the browser carries `__session`, an opaque token checked against a database row — `api/internal/authn/store.go:101`, `SELECT ... FROM sessions WHERE token_hash = $1 AND expires_at > $2`. No later request touches Identity Platform. So the Auth emulator's lack of clock control governs only the sign-in moment, not the session's life.
- **The verifier is hard-wired to real time and is not injectable from our code.** In `firebase.google.com/go/v4@v4.21.0`, `auth/token_verifier.go:120` and `:139` set `clock: internal.SystemClock`; the checks at `:210` and `:218` compare `IssuedAt` and `Expires` against `tv.clock.Now()` with a `clockSkewSeconds = 300` allowance (`:46`). The field is unexported and the `Clock` on `AuthConfig` (`:375`) is an internal type.

This is why the seam must be **injected, not a process-wide fake**. Under an injected clock, `VerifyIDToken` keeps using real `time.Now()` — and so does the emulator that issued the token, whose `exp` is real-now + 1 h. The two agree, and sign-in works at any simulated offset. Under a process-wide fake (which `libfaketime` could not have produced anyway, per Area 3), a token issued at real time would look months expired and **every** sign-in would be rejected — the "rejects every session" failure the ticket anticipated.

The session row, meanwhile, runs on simulated time: `session.go:65` takes `now := time.Now()` (the seam's future call site) and hands it to `authn.MintSession`, which writes `expires_at = now + SessionLifetime` (`store.go:62`). `SessionLifetime` is **12 h** (`api/internal/authn/authn.go:41`), with sliding renewal past half-life (`authn.go:216`, `renewIfStale`). `MintSession` also runs `DELETE FROM sessions WHERE expires_at <= $1` (`store.go:52`) — so a jump of six months plus one sign-in garbage-collects every other persona's session at once, which the harness should expect.

So the answer, plainly: **a jump of more than 12 simulated hours expires every open session; the persona's first act after the jump is a sign-in, which succeeds.** That costs one sign-in per active persona per jump, it is what a real doula's morning looks like anyway, and it exercises `sessionnotice.QueueNewSignInIfDue` (`session.go:74`) roughly once per simulated day — itself a source of findings.

**The browser's own clock is not a problem.** Only one non-test file in `app/src` constructs a `Date`: `app/src/lib/dates.ts`, whose two exported functions (`formatInstant`, `formatCalendarDay`) format a server-supplied string. Neither calls `Date.now()`, so nothing in the UI renders a relative time against the browser clock, and no "3 days ago" label can disagree with the simulated date. Should that change, Playwright's `page.clock` is available — `app/package.json:25` pins `@playwright/test` at `^1.60.0`, well past the 1.45 that introduced it.

## Loose ends for the build ticket

- The four in-migration `SET search_path` clauses (`00003_staff_signup.sql:40`, `00010_...:29`, `00067_...:72` and `:116`) pin `public` (or `public, pg_temp`) inside function bodies. Any of those functions that calls `now()` will keep real time unless `sim` is prepended. Which of them call `now()` was not checked here.
- The 27 production `time.Now()` sites were counted, not read. Which of them are genuinely simulated time (a session expiry, a due date) and which are genuinely real time (a rate-limit window, a metric, a token-verification skew) is a judgment per call site, and `internal/ratelimit`'s four SQL `now()` uses raise the same question on the database side: a six-month jump would expire every rate-limit window instantly.
- **Settled, [#780](https://github.com/markgoho/doula-cloud/issues/780).** A clock is created per *connected account*: `api/internal/billing` raises no Invoice at all — it creates Checkout Sessions for credits, which carry no due date — so the platform account needs no clock. And there is no per-account ceiling on the number of clocks worth designing around: **80 test clocks were created on one connected account with no refusal**, measured against the Sandbox on 2026-09-05 and then deleted. That is well above every figure `docs/simulation/calendar.md` produces, which sizes run one at eight clocks. The three-Customers-per-clock ceiling stands as measured on [#762](https://github.com/markgoho/doula-cloud/issues/762).
- **The shim needs more than one role.** `compose.e2e.yaml` runs Postgres as `POSTGRES_USER: app`, while `app/e2e/stack.ts` provisions a separate `app_e2e` login role. Goose's role needs the `search_path` for the 72 `DEFAULT now()` columns to bind to `sim.now()` at migration time; the BFF's role needs it for the 85 query-side `now()` calls. And `sim.now()` as prototyped is not `SECURITY DEFINER`, so whichever role runs under RLS needs `SELECT` on `sim.offset_row`. Which roles exist and which need the grant was not enumerated.

