# Testing infrastructure

## Pre-commit hook: `gofmt` and `app/` typecheck/lint (enabled repo-wide, enforced in CI)

`core.hooksPath` is set to an absolute path (`scripts/hooks`) in this repo's shared `.git/config`, so the hook below is already active for the main checkout and every `.claude/worktrees/*` worktree — there is nothing to opt into per clone. Because the path is absolute, every worktree runs the *main* checkout's `scripts/hooks/pre-commit`, not its own branch's copy; a worktree mid-refactor of the hook script itself won't see its own changes take effect until they land on the branch checked out in main. Re-run `git config core.hooksPath scripts/hooks` only if setting up a fresh clone.

`scripts/hooks/pre-commit` runs:
1. **`api/` (Go)**: blocks any commit that stages an unformatted `.go` file, prompting to run `gofmt -w <file>` on it.
2. **`app/` (SvelteKit)**: if any `app/*` files are staged, runs `bun run --cwd app check` (`svelte-check`) and `bun run --cwd app lint` (`eslint`), blocking commits with broken imports, type errors, or lint failures.
3. **`app/` unit suite and coverage gate**: still only when `app/*` files are staged, runs `bun run --cwd app test:unit:coverage`. This is where the design brief's smoothness gates live (see below), and the brief's own argument is that a commitment nobody measures decays — so the cheapest place to measure is before the commit exists. Roughly 6s on top of the ~7s for steps 1-2. The Playwright e2e suite deliberately stays out: it builds the app and starts Postgres, the BFF and the Auth emulator.

The CI jobs are the actual enforcement backstop regardless of whether the local hook is enabled — required PR status checks reject a push that would have failed it (see `docs/agents/worktree-flow.md`).

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
- `app/src/lib/components/organisms/DataTable.performance.svelte.spec.ts` —
  a row costs at most six elements, and mount cost stays linear in the row
  count. A ratio, never a millisecond budget.
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

`@axe-core/playwright` scans thirty-one routes, covering every screen in the A–G layout-archetype table on [#405](https://github.com/markgoho/doula-cloud/issues/405) plus the ones that postdate it — the two settings screens #452's hub added, the portal's Messages page, the three under `clients/{clientId}` that [#516](https://github.com/markgoho/doula-cloud/issues/516) brought in (the Client detail hub, her edit screen, and the Engagement Request form), the contractor Doula's branch of `clients/search` that [#525](https://github.com/markgoho/doula-cloud/issues/525) added, and the Practice-wide Invoice list from [#265](https://github.com/markgoho/doula-cloud/issues/265). The route inventory is in the spec itself, typed and grouped by archetype; that table *is* the documented set, so adding a route means adding a row rather than remembering a convention. Four tests, one per session the routes need — signed out, Staff, Client portal, contractor Doula — because signup and login cost the same few seconds however many routes follow them, and the contractor branch renders under a session none of the other three can produce.

The ruleset is `wcag2a`, `wcag2aa`, `wcag21a`, `wcag21aa`, `wcag22aa`: WCAG 2.2 AA, the bar GDS holds its own services to, and this repo already takes the GOV.UK Design System as its reference for service patterns ([ADR-0021](adr/0021-govuk-is-the-reference-for-service-patterns.md)). axe's `best-practice` tag is deliberately off — those are opinions, not conformance failures, and a gate that blocks on an opinion is a gate people learn to route around. Light theme only, following the same scope call the design map made: dark is derived later and rendering both would double the run for nothing.

Every route waits on its own `<h1>` before axe runs. This matters more than it looks: the app is a client-rendered SPA, `goto` resolves long before the data lands, and axe against a half-painted page finds a different set of violations every time — which CI's `retries: 2` would then quietly launder into green, the exact failure mode [ADR-0020](adr/0020-smoothness-is-gated-on-causes-because-the-outcome-is-not-measurable-where-the-gate-lives.md) documents.

**A violation fails the build.** It was a real choice, and the argument against it is in [#447](https://github.com/markgoho/doula-cloud/issues/447): a first adoption against twenty-three unaudited routes will be red on day one, and a red check people learn to ignore is worse than no check. What settles it is that the ticket also required every violation the first run surfaced to be fixed or filed — so day one is green, and red afterwards means a regression somebody just introduced, not a backlog. `CLAUDE.md` carries accessibility as a standing expectation on every feature; a check that only reports does not enforce an expectation.

What is filed rather than fixed lives in the spec's `KNOWN` list, one entry per allowance, each naming the issue that owns it. The list is **self-emptying**, the same shape as `DataTable.usage.spec.ts`'s pagination waiting list: an entry whose rule no longer fires on that route fails the scan until it is narrowed or deleted, so finishing the work is what removes it and a partial fix says so out loud. The list is empty today: #487 gave every scanned route a real `<title>` through the shared `PageTitle` primitive, which retired the one entry that stood, and no route added since has needed a new one.

Routes deliberately not scanned, so the gap is a decision rather than an oversight: `style-guide/*` (component demos, not archetypes — sixty pages for no additional archetype coverage), `demo/*` (SvelteKit scaffolding), and `/` and `/account`, which render with no shell at all and are already filed as [#484](https://github.com/markgoho/doula-cloud/issues/484).

### `app/e2e/keyboard.e2e.ts` — the half axe cannot see

axe reads one rendered page. It can tell you a control has an accessible name; it cannot tell you the control is reachable, reachable in a sane order, or that pressing Enter on it does what clicking it does. So the journeys there are walked with `page.keyboard` and nothing else — no `.click()`, no `.fill()`, no `.focus()`. `.fill()` is banned there specifically: it sets a value without the field ever being focused, which is the one step a keyboard user cannot skip.

The first walk is Stages 1 and 2 of [`docs/journeys/practice-owner.md`](journeys/practice-owner.md) — Renata signs in and invites a Doula — chosen because one task crosses the signed-out shell, the Staff shell's nav, a record list and a form that writes. The second ([#516](https://github.com/markgoho/doula-cloud/issues/516)) walks the Client detail hub to her edit form and on to an Engagement Request, and exists for the two controls the first has no example of: a modal confirmation, operated inside the focus trap `showModal()` gives it, and a radio group, whose selection moves on an arrow key rather than a Tab. Order is asserted only where order is a real obligation: the skip link is the first stop, and the password field follows the email. Everything else is asserted as *reachable within a budget of Tab presses*, because freezing the exact count would turn every nav change into a failing test for no accessibility reason.

### What this owns, and what it does not

Focus visibility and keyboard reachability are asserted **here and nowhere else** — the smoothness gate above hands its requirements 4 and 5 to these two specs on purpose, so do not add a second focus or keyboard assertion under another name.

Three things are outside both specs, and each has somewhere else to be met:

- **`:focus-visible` rendering.** Neither spec can honestly read whether a focus ring is *perceptible*. axe checks that a focus style is not suppressed; it cannot judge contrast against whatever is behind it. That is a human check on a real display, and the token floors it depends on are proved in `app/src/lib/styles/tokens.spec.ts`.
- **Focus return.** Whether closing a dialog puts focus back on the control that opened it is a sequence, not a snapshot, and axe never sees it. The shell's own menus and its narrow-viewport sheet get this from the platform — they are a native `popover` and a `<dialog>` opened with `showModal()`, and the browser owns the top layer, light dismiss, Escape and the focus return. **Anything hand-rolled does not, and the obligation lands on the first `Dialog` component ([#473](https://github.com/markgoho/doula-cloud/issues/473)): its own spec must assert that dismissing it returns focus to the trigger.** Prefer the platform element, and inherit the behaviour instead of testing for it.
- **Assistive-technology output.** No automated check hears what a screen reader says. axe covers roughly a third of WCAG by rule count; a passing scan is a floor, not a pass.

## `api/`: lint with golangci-lint, matching CI exactly

CI runs `golangci-lint` (config: `api/.golangci.yml`) as its own gating step,
separate from `go vet`/`go build` -- a change can compile and pass `go test`
while still failing CI on `golangci-lint` alone (goconst, noctx, unparam,
wrapcheck, and the rest of the curated set in that config). `go vet` is not a
substitute for it. Before considering `api/` work done, run the same command
CI runs:

```sh
cd api
golangci-lint run
```

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

## `api/`: migrations via goose

Migrations live in `api/db/migrations`. In dev/CI, `internal/testdb`
applies them programmatically (see above). At deploy time,
`scripts/migrate.sh` applies them through the Cloud SQL Auth Proxy as a
blocking pre-deploy step — it must exit non-zero, and stop the deploy, if
migration fails. It's not yet wired to a real instance (none is
provisioned in the `doula-cloud` GCP project); see the script's header for
the required env vars.

## `app/`: e2e stack — Postgres and the object store in compose, migrate/BFF/emulator as host processes

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
the goose migration step, the `app_e2e` login role, the Go BFF, and the
Firebase Auth emulator — runs as a plain host process, started/stopped by
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
in [docs/environment.md](environment.md). First-time setup is `bash
scripts/stripe-setup.sh`.
