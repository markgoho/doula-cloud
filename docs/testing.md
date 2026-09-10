# Testing infrastructure

## Pre-commit hook: `gofmt` and `app/` typecheck/lint (enabled repo-wide, enforced in CI)

`core.hooksPath` is set to an absolute path (`scripts/hooks`) in this repo's shared `.git/config`, so the hook below is already active for the main checkout and every `.claude/worktrees/*` worktree — there is nothing to opt into per clone. Because the path is absolute, every worktree runs the *main* checkout's `scripts/hooks/pre-commit`, not its own branch's copy; a worktree mid-refactor of the hook script itself won't see its own changes take effect until they land on the branch checked out in main. Re-run `git config core.hooksPath scripts/hooks` only if setting up a fresh clone.

`scripts/hooks/pre-commit` runs:
1. **`api/` (Go)**: blocks any commit that stages an unformatted `.go` file, prompting to run `gofmt -w <file>` on it.
2. **`app/` (SvelteKit)**: if any `app/*` files are staged, runs `bun run --cwd app check` (`svelte-check`) and `bun run --cwd app lint` (`eslint`), blocking commits with broken imports, type errors, or lint failures.
3. **`app/` unit suite and coverage gate**: still only when `app/*` files are staged, runs `bun run --cwd app test:unit:coverage`. This is where the design brief's smoothness gates live (see below), and the brief's own argument is that a commitment nobody measures decays — so the cheapest place to measure is before the commit exists. Measured on an idle 14-CPU machine, this step is ~16s and peaks around 4.9 GB for 2689 tests at 100% coverage, on top of the ~7s for steps 1-2. The Playwright e2e suite deliberately stays out: it builds the app and starts Postgres, the object store, the BFF and the Auth emulator — it is costed in "What the e2e suite costs, and why its workers are capped" below, which is where to look before running it beside anything else. See "The memory this gate costs, and why the browser pool is capped" below for where that 4.9 GB goes, and "Only one session runs this step at a time" for the lock that keeps two sessions from paying it simultaneously.

The CI jobs are the actual enforcement backstop regardless of whether the local hook is enabled — required PR status checks reject a push that would have failed it (see `docs/agents/worktree-flow.md`).

## The memory this gate costs, and why the browser pool is capped

The gate above is the heaviest thing this repo runs locally, and the reason is not obvious from its name: `app/vite.config.ts` runs its `client` project in **browser mode**, so `test:unit:coverage` starts headless Chromium. The browsers belong to the pre-commit gate, not to the e2e suite the gate deliberately excludes.

Vitest sizes that pool as `Math.min(12, ncpu - 1)` — a guard against the main thread choking, with nothing in it that asks what memory is free, and nothing that knows how many other sessions are doing the same thing. Several Claude sessions run against this repo at once (`docs/agents/worktree-flow.md`), each free to commit whenever it likes. On a 24 GB machine whose non-repo residents (a browser, the `claude` processes, the Podman VM) already come to ~10 GB, two uncapped gates at ~7 GB each do not fit, and the harness kills the commit with "system is running low on memory". That was [#935](https://github.com/markgoho/doula-cloud/issues/935).

So the `client` project pins `maxWorkers` to `Math.min(6, availableParallelism() - 1)`. Measured on an idle 14-CPU machine:

| Renderers | Peak (browsers + Vitest) | Wall time |
| --- | --- | --- |
| 12 (Vitest's default here) | ~7.0 GB | 17.4s |
| **6 (what we pin)** | **4.9 GB** | **16.3s** |
| 4 | 4.5 GB | 18.0s |

Compare warm runs only. The first run of a session pays a cold Vite transform (`transform 38s, import 87s` against ~4s and ~22s once warm) and takes about 20s at any worker count; a cold run measured against a warm one will credit the cap with roughly 4s it did not earn.

Memory is the reason for the cap, and it is the column that moves: browser RSS falls from ~6.0 GB to ~4.0 GB. Wall time is roughly a wash — about a second, plus a real drop in CPU time (111s to 78s) from no longer oversubscribing 14 cores. Six is chosen because it costs nothing in speed, not because it buys any.

It is clamped rather than a bare constant so CI's worker count is untouched: a 4-vCPU `ubuntu-latest` runner already resolves to 3, and a constant 6 would have raised the parallelism there. The `groupOrder` split below *does* apply in CI, where the two projects previously overlapped — measured at no cost, with the `app` job at 4m20s against 4m11s and 4m39s for the two preceding trunk runs.

Two things to know before you change it:

- **`--maxWorkers` on the command line will not override this.** The browser pool reads the project's own `maxWorkers`, not the root config's. A run with `--maxWorkers=4` still spawns 12 renderers. Edit the `client` project in `app/vite.config.ts`.
- **The `server` project carries `sequence: { groupOrder: 1 }` because of this cap.** Vitest refuses two projects that share a `groupOrder` but disagree on `maxWorkers`. Splitting them is a second memory win, not a formality: the Node forks no longer overlap the Chromium renderers, so the run has one peak instead of two stacked together.

### Only one session runs this step at a time

Capping one run does not coordinate several. Two concurrent gates fit (~10 GB of headroom used); three do not, because roughly 3.5 GB per gate is fixed cost that no worker count removes — so lowering the cap further buys no third run, and only admission control does. Up to 9 worktrees run at once (`docs/agents/worktree-flow.md`), each free to commit whenever it likes, so three at once is the ordinary case rather than the edge one. That was [#936](https://github.com/markgoho/doula-cloud/issues/936).

`scripts/gate-lock.ts` wraps the `test:unit:coverage` step in a machine-wide exclusive lock: `bun scripts/gate-lock.ts -- <command>`. A second session waits and says so — one line naming the holding worktree, its pid, and how long it has been running, repeated every 15 seconds so the commit reads as queued rather than hung. `gofmt`, `check` and `lint` are cheap and stay unlocked.

The lock is a directory created with a non-recursive `mkdir`, which is the atomic test-and-set; macOS ships no `flock` binary, which is why this is a bun script and not a shell one-liner. It lives in the **git common directory** (`$(git rev-parse --git-common-dir)/pre-commit-gate.lock`), which every worktree of this checkout resolves to the same path — a per-worktree `.git` would give each session a private lock and coordinate nothing.

**Every path through it fails open**, because a gate that wedges every commit is worse than the memory pressure it prevents:

- **The owner is gone.** The holder's pid is checked with signal 0 on every poll; a killed session's lock is reclaimed at once, with no waiting on any threshold. `EPERM` counts as alive — only `ESRCH` proves a process dead.
- **The lock has gone quiet.** A holder touches the lock directory every 5 seconds for as long as its command runs, so `STALE_LOCK_MS` (5 minutes) measures *abandonment*, not duration — 60 consecutive missed beats. Without the heartbeat this would be a run-time limit, and a gate queued behind two others on a pressured machine would have its lock taken while it was demonstrably still working. It only ever fires on a lock this script did not finish writing; a holder that was killed is caught by the liveness check above, which needs no threshold.
- **Reclaiming is single-winner.** A stale lock is taken by `rename`, not `rm -rf`. Two waiters that both judge the same lock stale would otherwise both end up running — the first removes it and takes a fresh one, the second deletes *that* and takes one too. Only one rename can succeed; the loser gets `ENOENT` and goes round the loop. For the same reason a holder releases its lock only on positive proof that the lock is still the one it took (an owner file naming its own pid), never on the absence of proof. The directory's inode is the obvious cheaper receipt and does not work: APFS reuses the inode a removed directory just gave up.
- **The wrapper itself is optional.** `scripts/hooks/pre-commit` resolves it from `$0` (the hook's absolute path in the main checkout, since `core.hooksPath` is absolute) rather than the committing worktree's cwd, and runs the step unwrapped if the file is not there. A worktree branched before this landed still commits.
- **Anything unexpected.** No git directory, an unwritable parent, a lock that cannot be reasoned about: the wrapper prints `gate-lock: running without the lock (…)` and runs the command.

**Landing this is not the same as switching it on.** `core.hooksPath` is absolute into the main checkout, so every worktree runs *main's* `scripts/hooks/pre-commit` and therefore *main's* `scripts/gate-lock.ts`. Until main's own tree carries both, the hook's `-f` fallback runs the step unwrapped everywhere — by design, but it means the lock starts working only once main's `trunk` has fast-forwarded past the merge. `.claude/hooks/sync-trunk.ts` does that on `SessionStart`, so in practice it is live from each session's next start.

Two escape hatches, for when you do not want to wait:

```sh
# Run this commit's gate with no lock at all.
SKIP_GATE_LOCK=1 git commit -m "…"

# Clear a lock by hand. Almost never needed -- a dead owner is reclaimed
# on the next poll and a stale one after 5 minutes -- but this is the
# command if you want it gone now.
rm -rf "$(git rev-parse --git-common-dir)/pre-commit-gate.lock"
```

Measured on the same 14-CPU / 24 GB machine as the table above, warm, sampling every 500 ms. The full record, including how the run was driven, is on [#936](https://github.com/markgoho/doula-cloud/issues/936).

| | Peak whole gate | Peak Chromium | Renderer processes | Free-memory floor |
| --- | --- | --- | --- | --- |
| One gate | 6.00 GB | 4.78 GB | 12 | 34% |
| **Three concurrent gates, locked** | **7.33 GB** | **6.22 GB** | **12** | **34%** |
| Three concurrent gates, unlocked | ~18 GB (projected) | ~14 GB | 36 | — |

Three sessions committing at once cost about a fifth more than one, not three times — they run one after another, finishing at +21s, +43s and +78s, each waiting on the previous one's lock. The overshoot above a single gate is the tail of one run's browsers exiting while the next starts, and the number that matters is the free-memory floor: identical at one gate and three.

The unlocked row was deliberately **not** reproduced. Doing so means intentionally exhausting memory on a machine with other live agent sessions mid-commit, and the figure is already known from the per-gate one: ~18 GB against the ~10.5 GB of non-repo residents #936 measured is well past 24 GB, the same arithmetic that killed a commit at two *uncapped* gates in #935.

**Do not read the renderer count as a worker count.** A single capped gate was observed at both 6 and 12 `--type=renderer` processes across runs of the same command, so the number does not map one-to-one onto the `maxWorkers` cap of 6 and the reason for the spread was not chased down. Compare it against the one-gate row above, never against the cap. [#937](https://github.com/markgoho/doula-cloud/issues/937) sized Playwright's workers without it for exactly this reason — it took the worker count from Playwright's own `Running N tests using M workers` line and the cost from sampled RSS, and the next section is what that measured.

### What the e2e suite costs, and why its workers are capped

The e2e suite is not in the pre-commit gate — it builds the app and starts Postgres, the object store, the Go BFF and the Auth emulator — so it costs nothing until someone runs `bun run --cwd app test:e2e`. It is costed here anyway, so both heavy local runs are in one place, because it had the same shape of memory-blind default the gate did. That was [#937](https://github.com/markgoho/doula-cloud/issues/937).

Playwright's `workers` defaults to the string `"50%"`, which `resolveWorkers` turns into `Math.max(1, Math.floor(os.cpus().length / 2))` — 7 on the 14-CPU machine this repo is developed on. Each Playwright worker gets its **own browser**, unlike Vitest's browser pool where many renderers share one, so the arrangement is more expensive per worker than the gate's. So `app/playwright.config.ts` pins `workers` to `Math.min(4, Math.max(Math.floor(cpus().length / 2), 1))` — the `Math.max(…, 1)` is Playwright's own guard for a single-core box, kept so the clamp is the default with a ceiling on it rather than a second formula.

Measured on the same 14-CPU / 24 GB machine, warm, sampling every 500 ms, with 68 tests across 30 files. The first two rows are two runs each and the third is one; the starting conditions were a 65-73% free-memory reading and zero live `ms-playwright` processes, not an idle machine — other agent sessions were live on the box, as they were for #936.

| Workers | Peak whole run | of which browsers | Wall time | Free-memory floor |
| --- | --- | --- | --- | --- |
| 7 (the default here) | 4.58 / 4.63 GB | 2.51 / 2.53 GB | 40.9s / 41.8s | 60% / 58% |
| **4 (what we pin)** | **2.88 / 3.03 GB** | **1.69 / 1.67 GB** | **42.2s / 40.3s** | **62% / 64%** |
| 3 | 2.42 GB | 1.31 GB | 47.0s | 64% |

Memory falls roughly linearly with the count — about 0.55 GB a worker, browsers and their Node hosts together — so there is no knee to find. Wall time is what picks the number: it is flat from 7 down to 4, where the two runs at four straddle the two at seven, and only starts rising at 3. Four is the lowest count that costs nothing in speed, which is the same rule that picked six for the unit gate above.

**The run has a floor no worker count reaches**, and it is a separate peak in time rather than a component of the one in the table: the `vite build` in `webServer.command` runs to completion before the first browser starts, peaking at 1.1-1.2 GB warm and around 2.4 GB on a session's first, cold build. This is not the concurrent fixed cost #936 found under the unit gate, where the 3.5 GB overlapped the browsers and was what made a third gate impossible. Here the two peaks never coincide, so the floor only says how little a very low worker count could ever buy.

Read the worker count from Playwright's own `Running N tests using M workers` line and never from a process count. The renderer counts sampled here were 9 and 8 at seven workers, 6 and 6 at four, and 4 at three — never once equal to the worker count, and not stable between runs of the same command, exactly as the paragraph above this section warns.

It is clamped rather than a bare constant so CI is untouched: `ubuntu-latest` is a 4-vCPU runner where `"50%"` already resolves to 2, and a constant 4 would have *raised* the parallelism there. The clamp restates Playwright's own formula (`os.cpus().length`, not `availableParallelism()`) rather than the one `app/vite.config.ts` mirrors for Vitest, because the two products read different numbers and a clamp that must never raise parallelism has to be written against the default it is clamping.

### When a commit is killed for memory

The failure looks like a hung or killed commit, not a test failure, so check the machine rather than the diff:

```sh
# What the test browsers are costing right now. `grep -i chrome` is no use
# here -- it also matches your own browser. ms-playwright is the giveaway.
ps -Ao pid,rss,command | grep ms-playwright | grep -v grep

# How many renderers are live -- this is the number the cap controls.
ps -Ao command= | grep ms-playwright | grep -v grep | grep -c -- --type=renderer

# Current pressure. Prefer this to `sysctl vm.swapusage`: macOS never
# shrinks its swap-used counter, so a high figure there is a record of the
# worst moment since boot, not a reading of now.
memory_pressure | tail -3
```

If renderers from another session are live, wait rather than kill them — they exit on their own and the memory comes back. A fix that races another session is worse than the wait.

## Smoothness: gated on causes, not on frame rate

[ADR-0020](adr/0020-smoothness-is-gated-on-causes-because-the-outcome-is-not-measurable-where-the-gate-lives.md)
records why, with the measurements behind it. The short version: headless
Chromium reports a fixed ~8.3ms frame no matter what it renders, and CI's
`app` job runs Postgres, the Auth emulator, the Go BFF and Chromium on one
shared runner with `retries: 2`. Neither frame rate nor interaction latency
can be read honestly in either place. Facts about *space* can, so those are
what is asserted.

Four specs carry it, all in the unit suite, all blocking:

- `app/src/lib/styles/motion.spec.ts` — parses every `.svelte` and `.css`
  file under `app/src` for raw durations and easing keywords, ungated
  transforms, unjustified `@keyframes`, motion tokens with no consumer, and
  `<img>` without intrinsic dimensions. Break a rule deliberately by putting
  `motion:ignore` plus the reason in a comment attached to the declaration
  or to the rule that encloses it — the same shape as `coverage:ignore` in
  `api/`.
- `app/src/lib/components/organisms/DataTable.usage.spec.ts` — no route may
  hand `DataTable` an unbounded list. Four routes are on a justified waiting
  list until [#446](https://github.com/markgoho/doula-cloud/issues/446)
  gives their endpoints a cursor; the spec fails if one is left on that list
  after it starts paginating.
- `app/src/lib/components/organisms/DataTable.performance.svelte.spec.ts` — a row costs at most six elements, and reads the row array a fixed, small number of times rather than reading the rest of the list per row. A read count, never a millisecond budget — [#489](https://github.com/markgoho/doula-cloud/issues/489) replaced the last wall-clock assertion in this file after it flaked on CI.
- `app/src/lib/components/atoms/Skeleton.layoutShift.svelte.spec.ts` — a
  skeleton reserves the space the content it stands in for will occupy.

**What is not checked** is listed in ADR-0020 rather than left to be
discovered: frame rate, the 100ms and 400ms latency budgets, route-level
Cumulative Layout Shift, and the blank first frame an SPA paints before its
JavaScript boots. Scroll feel is a human check on a real display when a
ticket touches a list. Focus visibility and keyboard reachability belong to
accessibility — the next section is what that sentence points at — not to
this gate, so nothing is asserted twice under two names.

## Accessibility: axe on every archetype, and a keyboard walk beside it

Two e2e specs carry it, both blocking, both in the Playwright suite rather than the unit one — they need the real production build the suite already starts (`bun run build && bun run preview`), so a scan sees what a person sees rather than what a component renders in isolation.

### `app/e2e/accessibility.e2e.ts` — the automated half

`@axe-core/playwright` scans forty-six routes, covering every screen in the A–G layout-archetype table on [#405](https://github.com/markgoho/doula-cloud/issues/405) plus the ones that postdate it — the two settings screens #452's hub added, the Practice-deletion screen [#871](https://github.com/markgoho/doula-cloud/issues/871) built, the MFA-requirement screen enrolled here by [#894](https://github.com/markgoho/doula-cloud/issues/894), the two MFA-recovery screens [#694](https://github.com/markgoho/doula-cloud/issues/694) built, the portal's Messages page, the three under `clients/{clientId}` that [#516](https://github.com/markgoho/doula-cloud/issues/516) brought in (the Client detail hub, her edit screen, and the Engagement Request form), the contractor Doula's branch of `clients/search` that [#525](https://github.com/markgoho/doula-cloud/issues/525) added, the Practice-wide Invoice list from [#265](https://github.com/markgoho/doula-cloud/issues/265), the engagement-request approval screen that [#502](https://github.com/markgoho/doula-cloud/issues/502) built and [#528](https://github.com/markgoho/doula-cloud/issues/528) enrolled here, `/account`, given the same shell every other authenticated Staff route already has by [#484](https://github.com/markgoho/doula-cloud/issues/484), and `/no-practice` ([#745](https://github.com/markgoho/doula-cloud/issues/745), enrolled here by [#749](https://github.com/markgoho/doula-cloud/issues/749)), the one archetype-A route that needs a live session — a signed-in identity exchanged straight for a session with no `POST /api/staff/signup` in between, so the staff-session lookup the screen itself calls resolves to no Practice rather than to no session at all, which is what every other route in that batch gets scanned under. The route inventory is in the spec itself, typed and grouped by archetype; that table *is* the documented set, so adding a route means adding a row rather than remembering a convention. Five tests, one per session the routes need — signed out, no-Practice, Staff, Client portal, contractor Doula — because signup and login cost the same few seconds however many routes follow them, and the contractor branch renders under a session none of the other four can produce.

The ruleset is `wcag2a`, `wcag2aa`, `wcag21a`, `wcag21aa`, `wcag22aa`: WCAG 2.2 AA, the bar GDS holds its own services to, and this repo already takes the GOV.UK Design System as its reference for service patterns ([ADR-0021](adr/0021-govuk-is-the-reference-for-service-patterns.md)). axe's `best-practice` tag is deliberately off — those are opinions, not conformance failures, and a gate that blocks on an opinion is a gate people learn to route around. Light theme only, following the same scope call the design map made: dark is derived later and rendering both would double the run for nothing.

Every route waits on its own `<h1>` before axe runs. This matters more than it looks: the app is a client-rendered SPA, `goto` resolves long before the data lands, and axe against a half-painted page finds a different set of violations every time — which CI's `retries: 2` would then quietly launder into green, the exact failure mode [ADR-0020](adr/0020-smoothness-is-gated-on-causes-because-the-outcome-is-not-measurable-where-the-gate-lives.md) documents.

**A violation fails the build.** It was a real choice, and the argument against it is in [#447](https://github.com/markgoho/doula-cloud/issues/447): a first adoption against twenty-three unaudited routes will be red on day one, and a red check people learn to ignore is worse than no check. What settles it is that the ticket also required every violation the first run surfaced to be fixed or filed — so day one is green, and red afterwards means a regression somebody just introduced, not a backlog. `CLAUDE.md` carries accessibility as a standing expectation on every feature; a check that only reports does not enforce an expectation.

What is filed rather than fixed lives in the spec's `KNOWN` list, one entry per allowance, each naming the issue that owns it. The list is **self-emptying**, the same shape as `DataTable.usage.spec.ts`'s pagination waiting list: an entry whose rule no longer fires on that route fails the scan until it is narrowed or deleted, so finishing the work is what removes it and a partial fix says so out loud. The list is empty today: #487 gave every scanned route a real `<title>` through the shared `PageTitle` primitive, which retired the one entry that stood, and no route added since has needed a new one.

Routes deliberately not scanned, so the gap is a decision rather than an oversight: `style-guide/*` (component demos, not archetypes — sixty pages for no additional archetype coverage), `demo/*` (SvelteKit scaffolding), and the two picker states of `/` — the Staff picker and the portal picker — which render the same nodes as the signed-out state of `/` that *is* scanned (an `EntryPage`, an `<h1>`, and a list of links) while costing a Staff member with two Memberships and a Portal Account with two Engagements to provision. The one node they add that the scanned state has not got is the portal picker's status paragraph, joined to its link by `aria-describedby`; that join is asserted in `Link.svelte.spec.ts` rather than here, since axe scans the DOM instances it is pointed at and not the pattern behind them. `/` itself scans as an archetype A route: [#357](https://github.com/markgoho/doula-cloud/issues/357) decided and built what `/` shows, and [#678](https://github.com/markgoho/doula-cloud/issues/678) decided that it carries a shell, moving it into the `(signed-out)` route group so it gets the reduced bar, the `<main>` landmark and the skip link that group already supplies. `/account` used to share the same shell-less gap, until [#484](https://github.com/markgoho/doula-cloud/issues/484) gave it the same shell every other authenticated Staff route already has; it now scans, as an archetype F route, in the spec's own route inventory above.

### `app/e2e/keyboard.e2e.ts` — the half axe cannot see

axe reads one rendered page. It can tell you a control has an accessible name; it cannot tell you the control is reachable, reachable in a sane order, or that pressing Enter on it does what clicking it does. So the journeys there are walked with `page.keyboard` and nothing else — no `.click()`, no `.fill()`, no `.focus()`. `.fill()` is banned there specifically: it sets a value without the field ever being focused, which is the one step a keyboard user cannot skip.

The first walk is Stages 1 and 2 of [`docs/journeys/practice-owner.md`](journeys/practice-owner.md) — Renata signs in and invites a Doula — chosen because one task crosses the signed-out shell, the Staff shell's nav, a record list and a form that writes. The second ([#516](https://github.com/markgoho/doula-cloud/issues/516)) walks the Client detail hub to her edit form and on to an Engagement Request, and exists for the two controls the first has no example of: a modal confirmation, operated inside the focus trap `showModal()` gives it, and a radio group, whose selection moves on an arrow key rather than a Tab. The third ([#525](https://github.com/markgoho/doula-cloud/issues/525)) is the contractor Doula's one-control door onto `clients/search` — the "Set up a Practice" link her empty-state explainer offers in place of the search form an Owner sees. The fourth ([#528](https://github.com/markgoho/doula-cloud/issues/528)) reaches the engagement-request approval screen from the Overview hub's own "Review requests" link, through the pending-Requests inbox's row link, and demonstrates both of the screen's decisions: Approve is reached and held (asserted focused, never pressed — the seeded Request is decided exactly once), and Refuse is completed, typing a reason with `page.keyboard.type` and pressing Enter, which lands the walk back on the Client's own record. Order is asserted only where order is a real obligation: the skip link is the first stop, and the password field follows the email. Everything else is asserted as *reachable within a budget of Tab presses*, because freezing the exact count would turn every nav change into a failing test for no accessibility reason.

### What this owns, and what it does not

Focus visibility and keyboard reachability are asserted **here and nowhere else** — the smoothness gate above hands its requirements 4 and 5 to these two specs on purpose, so do not add a second focus or keyboard assertion under another name.

Three things are outside both specs, and each has somewhere else to be met:

- **`:focus-visible` rendering.** Neither spec can honestly read whether a focus ring is *perceptible*. axe checks that a focus style is not suppressed; it cannot judge contrast against whatever is behind it. That is a human check on a real display, and the token floors it depends on are proved in `app/src/lib/styles/tokens.spec.ts`.
- **Focus return.** Whether closing a dialog puts focus back on the control that opened it is a sequence, not a snapshot, and axe never sees it. The shell's own menus and its narrow-viewport sheet get this from the platform — they are a native `popover` and a `<dialog>` opened with `showModal()`, and the browser owns the top layer, light dismiss, Escape and the focus return. **Anything hand-rolled does not, and the obligation lands on the first `Dialog` component ([#473](https://github.com/markgoho/doula-cloud/issues/473)): its own spec must assert that dismissing it returns focus to the trigger.** Prefer the platform element, and inherit the behavior instead of testing for it.
- **Assistive-technology output.** No automated check hears what a screen reader says. axe covers roughly a third of WCAG by rule count; a passing scan is a floor, not a pass.

## Layout: the continuum check, and what a fixture owes it

Layout is verified by sweeping a subject across the space it can be given rather than by asserting at chosen widths — `app/src/routes/style-guide/continuum.svelte.spec.ts` over every component demo, `app/src/routes/route-continuum.svelte.spec.ts` over every route, and the drag surface at `/style-guide/drag-surface` as the same fixtures seen by a person. `docs/adr/0025-layout-is-verified-across-the-continuum.md` is the decision and its Fixtures section is where a fixture's rules live: hostile values rather than polite ones, a row set realizing every state a field renders differently, and every session a route renders differently under.

`.claude/rules/svelte-tests.md` is the operational form of those rules and the file to read before writing a fixture or a route spec — where a route's content is declared, what a spec still owns, the two-row shape, and the `variants` list a role-varying route declares its other sessions in ([#913](https://github.com/markgoho/doula-cloud/issues/913)).

## `api/`: lint with golangci-lint, matching CI exactly

CI runs `golangci-lint` (config: `api/.golangci.yml`) as its own gating step,
separate from `go vet`/`go build` -- a change can compile and pass `go test`
while still failing CI on `golangci-lint` alone (goconst, noctx, unparam,
wrapcheck, and the rest of the curated set in that config). `go vet` is not a
substitute for it. Before considering `api/` work done, run the same command
CI runs:

```sh
cd api
GOLANGCI_LINT_CACHE="$(git rev-parse --show-toplevel)/api/.golangci-cache" golangci-lint run
```

**Always set `GOLANGCI_LINT_CACHE` to a path under `--show-toplevel`, never bare `golangci-lint run`.** Without it, golangci-lint's results cache defaults to one location shared by every worktree on the machine (`~/.cache/golangci-lint` / `~/Library/Caches/golangci-lint`), keyed in a way that does not account for a worktree's path being reused or removed -- a session linting after another worktree was pruned can see findings that point at files that no longer exist on disk, or, worse, a stale "clean" entry that masks a real issue in the current worktree's own changed file (#587). `--show-toplevel` resolves to the current worktree's own root, so the cache lives and dies with that worktree and never leaks into another one; `.golangci-cache` is gitignored, and `.claude/hooks/gate-golangci-lint-cache.sh` blocks a bare `golangci-lint run` in a Claude Code session. If you ever see findings in files that don't exist in your working tree, that's this problem: run `golangci-lint cache clean` with the same `GOLANGCI_LINT_CACHE` set, then rerun.

Two linters in this set are package-wide, not per-file, so a change to one
file can newly flag lines you didn't touch in other files in the same
package -- `goconst` (a literal crosses its repetition threshold once new
call sites are added elsewhere) and `unparam` (a return value becomes
"never used" once it's whole-package, not just per-caller). Don't skip
fixing those on the grounds that "that file isn't part of this change" --
if `golangci-lint run` at the repo's current state reports it, CI will too.

## Coverage: 100% line coverage, with justified exceptions

Both `api/` and `app/` are gated at 100% line coverage in CI. A line that
genuinely can't be exercised by a test (e.g. `log.Fatal` on listener
startup failure) needs an inline comment justifying the exception — it is
not left to ad-hoc PR discussion.

**`api/` (Go):** mark the line, or the `if` guarding it, with a comment
containing `coverage:ignore`:

```go
// coverage:ignore reason: listener startup, not exercised by unit tests
if err := http.ListenAndServe(":"+port, nil); err != nil {
	log.Fatal(err)
}
```

`api/tools/covcheck` parses the `go test -coverprofile` output and fails
the build on any zero-coverage line that has no `coverage:ignore` comment
directly above it or within the uncovered block. Run it locally:

```sh
cd api
go test ./... -coverprofile=coverage.out
go run ./tools/covcheck -profile=coverage.out -module=doula-cloud/api -skip=doula-cloud/api/tools/
```

`tools/covcheck` itself is still tested (`go test ./...` runs its unit
tests same as any other package) but excluded from the coverage
*requirement* via `-skip` — it's dev tooling, not shipped application
code.

**`app/` (SvelteKit + Vitest):** Vitest's `v8` coverage provider has this
built in — use `/* v8 ignore next */` (or `/* v8 ignore start */` /
`/* v8 ignore stop */` for a range), with a trailing reason comment. The
100% threshold is set in `app/vite.config.ts` under `test.coverage`,
scoped to `src/lib/**` — the code Vitest unit-tests. `src/routes/**` is
exercised by the Playwright e2e suite instead, but that suite does not
currently collect coverage, so route code has no coverage gate yet. As
route code accumulates real logic (beyond markup), instrumenting the e2e
run's coverage and merging it into the same threshold is the follow-up;
until then the 100% gate is honest about covering `src/lib/**` only, not
all of `app/`.

## `api/`: real Postgres for tests, container-engine-agnostic

`api/internal/testdb` uses testcontainers-go to start **one** real,
disposable Postgres container per test *process* (`go test` forks one
process per package), applies the goose migrations (`api/db/migrations`)
once into a template database, and then hands each call to `testdb.New(t)`
a fresh database cloned from that template — a file copy, not a migration
replay. It hands back a `*testdb.DB` with two connections: `Admin` (the
superuser the migrations ran as, for fixture setup) and `App` (a
low-privilege `app_runtime`-derived role, the one the running application
actually connects as). Postgres superusers and table owners always bypass
Row-Level Security, so tests that need to observe RLS in effect — not just
assume it — must query through `App`, not `Admin`.

Every package that calls `testdb.New` must define a `TestMain` that hands
off to `testdb.Main`, so the shared container is terminated once at
process exit rather than leaked or torn down mid-run:

```go
func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}
```

CI runs this against Docker (preinstalled on the runner, no setup needed).
Locally, testcontainers-go reads `DOCKER_HOST` from the environment, so
pointing that at a Podman socket runs the same tests against Podman
instead, with no code change:

```sh
# macOS: podman machine start, then export the socket it prints, e.g.
export DOCKER_HOST='unix:///path/to/podman-machine-default-api.sock'
# Linux: podman ships with a rootless socket, e.g.
export DOCKER_HOST="unix:///run/user/$(id -u)/podman/podman.sock"

export TESTCONTAINERS_RYUK_DISABLED=true # Ryuk is unreliable under rootless Podman
cd api
go test ./...
```

Ryuk being disabled locally is why `testdb.Main` exists: without an
explicit `container.Terminate` at process exit, a full local `go test
./...` would leave one Postgres container running per package that calls
`testdb.New`. CI leaves Ryuk enabled as a backstop, but relies on
`testdb.Main` too, since Ryuk only reaps containers after they're already
orphaned.

Every package that calls `testdb.New` must wire up its own `TestMain` --
four didn't (`internal/mfarecoverymail`, `internal/outbox`,
`internal/sessionmint`, `internal/sessionnotice`), which meant those
packages leaked their container on every run, clean or not, until #889
gave each one the same three-line `TestMain` every other package already
had.

### A due-time fixture must not compare two clocks

The host and the database run on different clocks whenever the container engine is VM-backed — Podman on macOS, Docker Desktop on macOS. The VM keeps its own time, drifts against the host's, and does not resync when the Mac wakes. CI never sees this: on `ubuntu-latest` the container shares the runner's kernel clock, so the skew is structurally zero.

That makes a coin flip out of any fixture that inserts `next_attempt_at` from a host-side `time.Now()` and then claims the row back through a predicate like `WHERE next_attempt_at <= now()` — a coin flip on the sign of the drift. With the VM's clock even milliseconds behind the host's, the row is not yet due, the worker claims nothing, and the assertion reads the row back exactly as inserted — which looks like the worker silently did nothing rather than like a clock problem. [#987](https://github.com/markgoho/doula-cloud/issues/987) was eight `internal/outbox` tests failing this way.

Seed a due-time from the database's own clock, never the host's. `insertTestRow` in `api/internal/outbox/outbox_test.go` is the pattern: it takes an offset (`0` for "due now", `-time.Minute` for "overdue", `time.Hour` for "not yet due") and computes the timestamp in SQL as `now() + $3 * interval '1 microsecond'`, so one clock decides. Where a fixture must pass a host-side `time.Time`, give it a margin far larger than any plausible skew, and say in a comment that the margin is what absorbs it.

## Reaping orphaned testcontainers

`testdb.Main`'s teardown above only runs on a clean process exit. A
killed test process -- an interrupted agent, a timeout, a cut-short TDD
loop -- never reaches it, and Ryuk being disabled locally (previous
section) means nothing else reaps that container either. With several
parallel Claude Code agent sessions on one machine, these orphans
accumulate without bound: 116 running `postgres:16-alpine` containers,
146 total, 133MB free of a 4GB Podman machine, observed live (#889). That
starvation made `podman compose up` for `dev:full` time out with
`ETIMEDOUT` and made unrelated `api/internal/visit` tests flake locally
while staying green in CI.

`.claude/hooks/testdb-reap.ts` is a `SessionStart` hook (registered in
`.claude/settings.json`) that clears these out at the start of every
session. It removes any container labeled `org.testcontainers=true`
(testcontainers-go's own label, present on every container `testdb.New`
starts and nothing else on the machine) that is older than **15
minutes**. That threshold is deliberately generous, not tight: a
container backs one `go test` process for one package, so a live one
lives minutes at most, and the slowest single package observed on this
machine (`internal/payments`, idle machine) finished in 90.4s. 15 minutes
is roughly 10x that, enough headroom for several sessions competing for
the same 4GB Podman machine -- which is exactly the condition that causes
the leak -- without ever mistaking a live run for an orphan. The
reasoning lives as a comment on `REAP_THRESHOLD_MS` in the hook itself,
not only here.

The hook points the engine at `DOCKER_HOST` when the variable is set, the same way `testdb.go` does (previous section) — but **an unset `DOCKER_HOST` is not a reason to skip the run**, and treating it as one made this hook inert for months. The variable is exported by hand into the shell that runs the tests, as the block above shows; it is never set from a login profile, and a `SessionStart` hook inherits the login environment rather than that shell's. So the hook never saw it and always returned early: 55 containers, the oldest 38 hours old, on a machine that had started dozens of sessions since ([#1066](https://github.com/markgoho/doula-cloud/issues/1066)). `podman ps` with no `--url` reaches the same engine through its default `podman machine` connection, so `.claude/hooks/container-engine.ts` — the seam both reapers share — adds `--url` only when there is a socket to name, and otherwise just invokes the engine. If the engine isn't reachable at all, the hook does nothing and exits 0. It **fails open on every error path** -- unlike `gate-worktree-edit.ts`/`gate-bash-write.ts`, which fail closed because they're `PreToolUse` gates deciding whether to allow a tool call. This hook runs on `SessionStart`, makes no such decision, and exists purely to tidy up; a reaper that errors must never block or slow a session from starting. `gate-shared-index.sh` is the existing fail-open precedent, for the same class of reason. It's quiet when there's nothing to reap; when it does reap, it logs the count and the reason.

If containers pile up faster than a session boundary clears them, the
same manual command #889 was diagnosed with still works as an escape
hatch:

```sh
podman rm -f -t 2 $(podman ps -aq --filter 'label=org.testcontainers=true')
```

## Reaping orphaned e2e stacks

The reaper above only knows about containers labeled `org.testcontainers=true`, which is the one thing a compose stack is not. `app/e2e/stack.ts` brings `app/compose.e2e.yaml` up under a per-worktree compose project name — `doula-cloud-e2e-<PORT_OFFSET>` — and takes it down again in `stopStack`. A killed worktree session never reaches that teardown, so its `db` and `gcs` containers, the project's `<project>_default` network and its named `<project>_db-data` volume sit on the same shared Podman machine indefinitely, feeding exactly the starvation the previous section describes ([#1066](https://github.com/markgoho/doula-cloud/issues/1066), and most likely the cause behind [#782](https://github.com/markgoho/doula-cloud/issues/782)).

`.claude/hooks/e2e-stack-reap.ts` is the second `SessionStart` hook (registered beside `testdb-reap.ts` in `.claude/settings.json`). It reads every container's `com.docker.compose.project` label — the key podman-compose 1.6.0 and Docker Compose v2 both write — groups them into projects, and runs `compose -p <project> down -v` on each one it judges orphaned. `down -v` rather than `rm -f` is the point: containers are the smaller half of what a stack leaves behind, and only a project-scoped `down -v` also collects the network and the volume.

**A stack is orphaned when both clocks say so, never one.** Unlike a testcontainers Postgres, an e2e stack legitimately outlives any fixed budget — `bun run dev:full` shares it and runs for as long as somebody is working — so age alone must never authorize a reap:

- **The worktree behind it has gone quiet.** This is the real signal. Each worktree's `.port-offset` file says which project name it owns — the same scan `worktree-provision.ts` runs to refuse an offset another worktree holds — so an offset that no `.claude/worktrees/*` directory claims, or whose directory has not been touched in **30 minutes**, has nothing live behind it. Thirty minutes, read the same three ways (worktree directory, its git dir, that dir's index), is `worktree-prune.ts`'s own measure of "nobody is standing here", written because a session that has just landed its PR sits in a finished worktree for as long as it takes to write a summary. Only the quiet test is borrowed, not the bar: prune removes a directory only when it is quiet *and* clean *and* merged, because deleting a worktree can destroy unlanded work. Nothing here can — a reaped stack is a database the next `up -d` rebuilds, holding fixtures — which is why quiet plus the age check below is the whole bar. Both readers fail closed: a worktree that is there but whose timestamps cannot be read counts as touched, and only a directory that is *gone* answers "nothing live".
- **The stack itself is older than 15 minutes.** The weaker of the two, and there only to cover a stack brought up seconds ago inside a worktree whose files happen not to have been touched since — comfortably longer than `compose up -d` plus the migrate (≤90s) and `go build` (≤120s) steps that follow it.

Two things it deliberately does not do. It never touches the main checkout's own unsuffixed `doula-cloud-e2e` project: offset 0 has no `.port-offset` file to go quiet, it is where a long interactive `dev:full` runs, and it is the one place a person is standing when they would notice their own database vanish. And it does not treat the stack's host BFF being up as proof of life — `startAPI` spawns it `detached` and `unref`s it, so an orphaned stack's BFF is still listening on its port, which would make every orphan look alive and this hook a no-op.

It fails open on every error path, for the same reason `testdb-reap.ts` does, and one project's teardown failing never stops the others. The manual escape hatch, for a project name `podman ps` shows:

```sh
podman compose -p doula-cloud-e2e-<offset> -f app/compose.e2e.yaml down -v
```

## `api/`: migrations via goose

Migrations live in `api/db/migrations`. In dev/CI, `internal/testdb`
applies them programmatically (see above). At deploy time,
`scripts/migrate.sh` applies them through the Cloud SQL Auth Proxy as a
blocking pre-deploy step — it must exit non-zero, and stop the deploy, if
migration fails. It's not yet wired to a real instance (none is
provisioned in the `doula-cloud` GCP project); see the script's header for
the required env vars.

## `app/`: e2e stack — Postgres and the object store in compose, migrate/BFF/emulator as host processes

What a run of this stack costs in memory, and why `workers` is pinned rather than left at Playwright's CPU-derived default, is in "What the e2e suite costs, and why its workers are capped" above.

`app/compose.e2e.yaml` defines two backing services: a pinned
`postgres:16-alpine`, and a pinned `fsouza/fake-gcs-server` on
`127.0.0.1:14443` standing in for the GCS bucket the BFF writes signed
Contract PDFs and message attachments to. `stack.ts` creates the one
bucket (`seedGCSBucket`) the same way it creates the one login role, and
points the BFF's `STORAGE_EMULATOR_HOST`/`GCS_ATTACHMENTS_BUCKET` at it.
The store used to be aimed at an unreachable host on the grounds that no
spec touches the attachment endpoints — which also made Contract signing
answer a bare 500, since it puts the PDF in the store before it writes the
status (`api/internal/contracts/sign.go`). Everything else Playwright e2e
tests run against —
the goose migration step, the `app_e2e` login role, the Go BFF, the
Firebase Auth emulator, and the sandbox mailbox (`app/e2e/mailbox.ts`,
#764 — the Mailgun-shaped sink `MAILGUN_API_BASE` points the BFF at, so
no local stack can post real mail to real Mailgun and a spec can read
what a person would have received) — runs as a plain host process, started/stopped by
`app/e2e/stack.ts` (`startStack`/`stopStack`), which `app/playwright.config.ts`
wires up via `globalSetup`/`globalTeardown`
(`app/e2e/global-setup.ts`/`app/e2e/global-teardown.ts`). `stack.ts` runs
`api/cmd/migrate` with `go run` and the BFF with `go build` + a tracked
PID (mirroring how it already managed the Firebase emulator), against
`DATABASE_URL`s pointed at the compose Postgres over `127.0.0.1:15432`.
Building the BFF and migrate binaries as compose images used to cost the
e2e run a cold `go mod download`/build on every CI run with no cache
sharing between them; running them as host processes instead lets CI's
`app` job share a single warm Go build cache (via `actions/setup-go`,
keyed on `api/go.sum`) with everything else that touches `api/`.

Postgres and the object store stay in `compose.e2e.yaml` (image pinning
matters more than build cost there) and are brought up/down via `$CONTAINER_ENGINE
compose` (`docker compose` and `podman compose` share the same v2 CLI
syntax) — CI sets `CONTAINER_ENGINE=docker` (preinstalled, no
rootless-socket setup needed on `ubuntu-latest`); it defaults to `podman`
for local dev, which needs `podman-compose` on `PATH` (`brew install
podman-compose`). Since every process now reaches Postgres, the emulator,
and the BFF's own listener over loopback directly, the old
`E2E_HOST_GATEWAY`/`host.containers.internal`/`host.docker.internal`
container-to-host routing machinery is gone — it was only ever needed to
get a *container* to reach a host-bound service, and nothing runs the BFF
in a container anymore.

**Running this from a worktree on macOS with Podman** needs nothing beyond what the rest of this doc already says: `DOCKER_HOST` pointed at the Podman machine's socket and `TESTCONTAINERS_RYUK_DISABLED=true` (see "Reaping orphaned testcontainers" above), exported in the same shell that runs `bun run test:e2e` or `bun run dev:full`. `startDatabase` (`app/e2e/stack.ts`) scopes every `podman compose` call to a per-worktree project name (`doula-cloud-e2e-<PORT_OFFSET>`) and host port pair already, so two worktrees' stacks never collide — no extra setup is needed per worktree beyond what `EnterWorktree` already provisions.

[#782](https://github.com/markgoho/doula-cloud/issues/782) reported `startDatabase` failing with a bare `exit status 1` and no podman-compose output at all, from a worktree at a non-zero `PORT_OFFSET`, while the exact same `podman compose … up -d` command run by hand in the same directory succeeded. `execFileSync` was run with `stdio: 'inherit'` — which hands the child the *caller's own* file descriptors, so Node never fills in `error.stdout`/`error.stderr` on a non-zero exit; the bare exit status was all that reached the console. `startDatabase` now runs with the default (captured) stdio instead and prints podman-compose's own stdout/stderr on failure before re-throwing, so the next occurrence is diagnosable from the console output alone.

The failure itself did not reproduce under repeated attempts here — by hand, via a direct `execFileSync` call matching `startDatabase`'s own arguments, and via a full `bunx playwright test` run through `global-setup.ts` — with both the original and the fixed code, at a non-zero offset, with the Podman VM under real concurrent load from other worktree sessions (`podman ps -a` showed 16 containers from other sessions' `go test` runs at the time, some several hours old; load average over 9 on the VM's 7 CPUs). `COMPOSE_ENV` and `PATH` both checked out correctly under `execFileSync` in every attempt. The best-supported explanation, given the repo already diagnosed the identical shared resource once before, is transient contention on the single 4GB Podman machine every worktree's containers share — the same VM #889 found running on 133MB free under concurrent `go test` load, which made `podman compose up` for `dev:full` fail a different way (`ETIMEDOUT` rather than a bare exit status). That reaper only clears `org.testcontainers=true`-labeled containers from `go test`, not a `podman-compose` e2e stack a killed worktree session left running — see [#1066](https://github.com/markgoho/doula-cloud/issues/1066) for closing that gap.

Because that removes the only thing that proved `api/Dockerfile` still
builds and boots (the runtime image is distroless
`gcr.io/distroless/static-debian12`, where a missing CA bundle or tzdata
would break a real deploy even though `go build`/`go test` pass cleanly),
CI's `api-image` job (`.github/workflows/ci.yml`) now builds that image
with `docker/build-push-action` and runs a boot smoke test against it —
container stays running and answers on its port — in parallel with `app`,
off the critical path (see PR #108 for measured before/after timings).

## Stripe: fakes in CI, the Sandbox by hand

`bun run test:e2e` sets no Stripe variables, so both Stripe clients run
against their injected fakes (`api/internal/billing/stripe_fake.go`,
`api/internal/payments/stripe_fake.go`). That is the deliberate choice,
not a gap: a GitHub-hosted runner has no public URL for Stripe to deliver
a webhook to, and the Stripe Sandbox is one shared, stateful environment
that parallel runs would trample.

Driving the real thing — a real Checkout Session, real Connect
onboarding, real `invoice.paid` — is a local, by-hand job. `bun run
dev:full` picks up `app/.env.local`, and `bash scripts/stripe-listen.sh`
forwards Stripe's events to the local BFF beside it. Everything about
that setup, including which variable holds what in each environment, is
in [docs/environment.md](environment.md). First-time setup is copying
`app/.env.example` and filling it in by hand.

## Logging in as Staff locally

The vendored Firebase Auth emulator (`app/node_modules`, firebase-tools 15.28.1) has no TOTP multi-factor enrollment path at all — its `mfaEnrollment:start`/`:finalize` endpoints hardcode a `phoneEnrollmentInfo` request shape and reject anything else, including the `totpEnrollmentInfo` shape the Firebase JS SDK's `TotpMultiFactorGenerator.generateSecret()` sends. That call is exactly what `app/src/routes/mfa/enroll/+page.svelte` makes to start enrollment, so walking that screen against the local stack 400s with `INVALID_ARGUMENT : ((Missing phoneEnrollmentInfo.))` every time — confirmed by reproducing the request directly against a standalone emulator instance, and tracked upstream as [firebase/firebase-tools#6224](https://github.com/firebase/firebase-tools/issues/6224), still open. Every Owner needs a second factor unconditionally (`api/internal/staffauth/middleware.go`), so this is a hard wall in front of every Practice screen, not just the enrollment screen — see [#900](https://github.com/markgoho/doula-cloud/issues/900).

The same emulator has a second MFA limitation, at the other end of a second factor's life: **it refuses the body that clears one.** `authn.FirebaseVerifier.ClearSecondFactors` — the single call under every MFA-recovery path — sends `mfa.enrollments` as JSON null, because the Admin SDK's `validateAndFormatMfaSettings` leaves its slice nil however the call is written. The emulator's schema types that field `array` and answers `400 Invalid JSON payload received. /mfa/enrollments must be array`. Production Identity Platform accepts the same body and clears the factor: [#1128](https://github.com/markgoho/doula-cloud/issues/1128) probed it against the real project on 2026-09-10, on a throwaway account holding a real TOTP enrollment, and read the enrollment back gone. So this is emulator strictness over a proto3-JSON null, the shipped call is right as written, and the consequence for tests is that `app/e2e/mfa-recovery.e2e.ts` cannot walk a *successful* recovery spend — only the refusals. The schema was read in both firebase-tools 15.27.0 (what `app/node_modules` holds) and 15.28.1 (what `package.json` pins) — `apiSpec.js`'s `GoogleCloudIdentitytoolkitV1MfaInfo` types `enrollments` as `array` with no null allowed in each — against `firebase.google.com/go/v4` v4.21.0, the SDK's newest release. `ClearSecondFactors`' own doc comment is the canonical record.

There is no local fix for the TOTP screen itself: deriving a valid code from a known secret needs a TOTP-capable server, and the local emulator is not one. That recipe works against the *deployed* app's real Identity Platform project instead — enroll by hand once through <https://doula-cloud-app.web.app> and keep the secret for future runs. What the emulator *does* support is PHONE_SMS enrollment, and the second-factor gate in `staffauth.Middleware` only checks the `firebase.sign_in_second_factor` claim — it does not care which provider produced it. `app/e2e/mfa.ts` already enrolls a phone factor as a stand-in and has since #606; that is the pattern this reuses rather than a new bypass.

**To reach an authenticated Practice screen locally as Staff, without walking the login form or the MFA screens:**

1. Start the stack: `bun run --cwd app dev:full`.
2. In a second terminal, run `bun run --cwd app seed:staff-session`. It calls `seedFoundingOwner` (`app/e2e/staffSignup.ts`) and `signInEnrolled` (`app/e2e/mfa.ts`) directly — the same helpers the e2e suite uses, invoked outside the Playwright test runner via `request.newContext()` — to sign up a founding Owner, verify her email, enroll a phone second factor, and sign in. It prints JSON with the session cookie value and the Practice URL to open.
3. Drive a real browser (the `playwriter` skill, or any Playwright-based tool) with `context.addCookies([{ name: '__session', value: cookieValue, url: origin, httpOnly: true, secure: false, sameSite: 'Lax' }])`, then `page.goto(practiceUrl)` from the script's output — the same cookie-injection pattern `app/e2e/mfa.ts`'s `enterPracticeAsEnrolled` uses inside the test suite.

**What this does and does not exercise.** Everything downstream of authentication runs for real: the real session cookie, the real `staffauth.Middleware` check, every Practice-scoped screen and API route. It does not exercise the login form or the TOTP QR-code/secret/code-entry screens — those need a TOTP-capable Identity Platform, which only the deployed app has locally reachable. A UI change to the login or MFA-enrollment screens still needs a walk of the deployed app (or a real GCP Identity Platform project) to verify visually; every other UI change can use this path.
